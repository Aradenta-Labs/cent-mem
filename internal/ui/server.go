package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// ServerConfig configures the embedded UI web server.
type ServerConfig struct {
	Host    string
	Port    int
	NoOpen  bool
	Version string
}

// DefaultServerConfig returns the standard localhost:4231 config.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:    "127.0.0.1",
		Port:    4231,
		NoOpen:  false,
		Version: "1.4.0",
	}
}

// Server wraps http.Server with centmem UI routes.
type Server struct {
	cfg        ServerConfig
	httpServer *http.Server
	listener   net.Listener
	addr       string
}

// NewServer creates a new centmem UI server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 4231
	}
	if cfg.Version == "" {
		cfg.Version = "1.4.0"
	}

	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"status":  "healthy",
			"version": cfg.Version,
		})
	})

	// Embedded Static File Server with SPA Fallback
	staticHandler, err := FileServerHandler()
	if err != nil {
		return nil, fmt.Errorf("embedded static handler: %w", err)
	}
	mux.Handle("/", staticHandler)

	s := &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	return s, nil
}

// Start binds to the configured host/port and starts serving HTTP requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	s.listener = ln
	s.addr = ln.Addr().String()

	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	return nil
}

// URL returns the full HTTP address URL.
func (s *Server) URL() string {
	if s.addr == "" {
		return fmt.Sprintf("http://%s:%d", s.cfg.Host, s.cfg.Port)
	}
	return fmt.Sprintf("http://%s", s.addr)
}

// Addr returns the network listener address string.
func (s *Server) Addr() string {
	return s.addr
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// OpenBrowser opens the given URL in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// linux / bsd
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
