package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/compact"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"google.golang.org/grpc"
)

// Server is the centmemd long-running daemon instance.
type Server struct {
	cfg          config.Config
	st           *store.Store
	emb          embed.Embedder
	grpcServer   *grpc.Server
	unixListener net.Listener
	tcpListener  net.Listener

	subscribers map[chan store.Event]struct{}
	subMu       sync.RWMutex

	writeCh   chan struct{}
	stopCh    chan struct{}
	stoppedCh chan struct{}

	startTime time.Time
	mu        sync.Mutex
	writeMu   sync.Mutex
	running   bool
}

// LockWrite acquires the exclusive write lock for modifying store state.
func (s *Server) LockWrite() func() {
	s.writeMu.Lock()
	return s.writeMu.Unlock
}

// NewServer initializes a new centmemd server.
func NewServer(cfg config.Config, st *store.Store, emb embed.Embedder) *Server {
	return &Server{
		cfg:         cfg,
		st:          st,
		emb:         emb,
		subscribers: make(map[chan store.Event]struct{}),
		writeCh:     make(chan struct{}, 10),
		stopCh:      make(chan struct{}),
		stoppedCh:   make(chan struct{}),
		startTime:   time.Now(),
	}
}

// Start begins listening on IPC socket/TCP and starts background workers.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("daemon is already running")
	}

	sockPath := s.cfg.Daemon.SocketPath
	if sockPath == "" {
		sockPath = filepath.Join(s.cfg.Home, "centmemd.sock")
	}

	// Probe existing socket
	if _, err := os.Stat(sockPath); err == nil {
		if conn, dialErr := net.DialTimeout("unix", sockPath, 100*time.Millisecond); dialErr == nil {
			conn.Close()
			return fmt.Errorf("daemon already running on socket %s", sockPath)
		}
		// Stale socket, remove it
		_ = os.Remove(sockPath)
	}

	// Ensure directory exists with owner-only permissions
	if err := os.MkdirAll(filepath.Dir(sockPath), 0700); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket %s: %w", sockPath, err)
	}
	_ = os.Chmod(sockPath, 0600)
	s.unixListener = l

	// Optional TCP listener
	if s.cfg.Daemon.Port > 0 {
		tcpAddr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Daemon.Port)
		tl, err := net.Listen("tcp", tcpAddr)
		if err != nil {
			s.unixListener.Close()
			_ = os.Remove(sockPath)
			return fmt.Errorf("listen on tcp %s: %w", tcpAddr, err)
		}
		s.tcpListener = tl
	}

	// Write PID file
	pidPath := s.cfg.Daemon.PIDPath
	if pidPath == "" {
		pidPath = filepath.Join(s.cfg.Home, "centmemd.pid")
	}
	_ = os.MkdirAll(filepath.Dir(pidPath), 0700)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		s.unixListener.Close()
		_ = os.Remove(sockPath)
		return fmt.Errorf("write pid file: %w", err)
	}

	s.grpcServer = grpc.NewServer()
	centmemv1.RegisterMemoryServiceServer(s.grpcServer, newMemoryServiceServer(s))
	centmemv1.RegisterSyncServiceServer(s.grpcServer, newSyncServiceServer(s))

	s.running = true

	// Serve Unix socket
	go func() {
		if err := s.grpcServer.Serve(s.unixListener); err != nil && s.running {
			slog.Error("daemon unix serve error", "err", err)
		}
	}()

	// Serve TCP if configured
	if s.tcpListener != nil {
		go func() {
			tcpGrpc := grpc.NewServer()
			centmemv1.RegisterMemoryServiceServer(tcpGrpc, newMemoryServiceServer(s))
			centmemv1.RegisterSyncServiceServer(tcpGrpc, newSyncServiceServer(s))
			if err := tcpGrpc.Serve(s.tcpListener); err != nil && s.running {
				slog.Error("daemon tcp serve error", "err", err)
			}
		}()
	}

	// Background embedding drainer
	go s.drainLoop()

	// Background compaction ticker
	go s.compactLoop()

	return nil
}

// Stop gracefully terminates the daemon server.
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	// Gracefully stop gRPC
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	if s.unixListener != nil {
		s.unixListener.Close()
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}

	// Final queue drain
	if s.emb != nil {
		q := embed.NewQueue(s.st, s.emb)
		q.MaxTime = 2 * time.Second
		_, _ = q.Drain(context.Background())
	}

	// Remove socket and PID files
	if s.cfg.Daemon.SocketPath != "" {
		_ = os.Remove(s.cfg.Daemon.SocketPath)
	} else {
		_ = os.Remove(filepath.Join(s.cfg.Home, "centmemd.sock"))
	}

	if s.cfg.Daemon.PIDPath != "" {
		_ = os.Remove(s.cfg.Daemon.PIDPath)
	} else {
		_ = os.Remove(filepath.Join(s.cfg.Home, "centmemd.pid"))
	}

	close(s.stoppedCh)
	return nil
}

// Wait blocks until the daemon stops.
func (s *Server) Wait() {
	<-s.stoppedCh
}

// SignalWrite informs the background drainer that new memories need embedding.
func (s *Server) SignalWrite() {
	select {
	case s.writeCh <- struct{}{}:
	default:
	}
}

// BroadcastEvent notifies all live event stream subscribers.
func (s *Server) BroadcastEvent(e store.Event) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- e:
		default:
			// Non-blocking drop if subscriber buffer is full
		}
	}
}

// SubscribeEvents registers a channel for live event updates.
func (s *Server) SubscribeEvents() (<-chan store.Event, func()) {
	ch := make(chan store.Event, 200)
	s.subMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subMu.Unlock()

	unsubscribe := func() {
		s.subMu.Lock()
		delete(s.subscribers, ch)
		s.subMu.Unlock()
	}
	return ch, unsubscribe
}

// drainLoop continuously processes pending items in the embed queue.
func (s *Server) drainLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-s.writeCh:
			s.drainOnce()
		case <-ticker.C:
			s.drainOnce()
		}
	}
}

func (s *Server) drainOnce() {
	if s.emb == nil {
		return
	}
	unlock := s.LockWrite()
	defer unlock()

	q := embed.NewQueue(s.st, s.emb)
	q.MaxTime = 300 * time.Millisecond
	_, _ = q.Drain(context.Background())
}

// compactLoop periodically runs compaction in the background.
func (s *Server) compactLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			policy := compact.Policy{
				FactKeepForever:        s.cfg.Retention.FactKeepDays == 0,
				NoteSummarizeAfterDays: s.cfg.Retention.NoteSummarizeAfterDays,
				LogSummarizeAfterDays:  s.cfg.Retention.LogSummarizeAfterDays,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			unlock := s.LockWrite()
			_, _ = compact.Compact(ctx, s.st, compact.Options{DryRun: false, Policy: &policy})
			unlock()
			cancel()
		}
	}
}
