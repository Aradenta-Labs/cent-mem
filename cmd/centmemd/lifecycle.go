package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/daemon"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func cmdStart(args []string) int {
	fs := newDaemonFlagSet("start")
	if err := fs.Parse(args); err != nil {
		cli.WriteError(os.Stderr, cli.Invalidf("flag parse: %v", err))
		return cli.ExitError
	}
	cfg, err := loadDaemonConfig(fs)
	if err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("config: %v", err))
		return cli.ExitError
	}

	sockPath := cfg.Daemon.SocketPath
	pidPath := cfg.Daemon.PIDPath

	// Check if already running
	if daemon.ProbeDaemon(sockPath, 50*time.Millisecond) {
		pid, _ := readPID(pidPath)
		_ = prettyPrint(fs, map[string]any{
			"ok":     true,
			"status": "already_running",
			"pid":    pid,
			"socket": sockPath,
			"port":   cfg.Daemon.Port,
		})
		return cli.ExitOK
	}

	// Executable to spawn
	exe, err := os.Executable()
	if err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("os.Executable: %v", err))
		return cli.ExitError
	}

	runArgs := []string{"run"}
	if h := fs.Lookup("home"); h != nil && h.Value.String() != "" {
		runArgs = append(runArgs, "--home", h.Value.String())
	}
	if d := fs.Lookup("db"); d != nil && d.Value.String() != "" {
		runArgs = append(runArgs, "--db", d.Value.String())
	}
	if s := fs.Lookup("socket"); s != nil && s.Value.String() != "" {
		runArgs = append(runArgs, "--socket", s.Value.String())
	}
	if p := fs.Lookup("pid-file"); p != nil && p.Value.String() != "" {
		runArgs = append(runArgs, "--pid-file", p.Value.String())
	}
	if pt := fs.Lookup("port"); pt != nil && pt.Value.String() != "0" && pt.Value.String() != "" {
		runArgs = append(runArgs, "--port", pt.Value.String())
	}

	cmd := exec.Command(exe, runArgs...)
	_ = os.MkdirAll(cfg.Home, 0700)
	logFile, err := os.OpenFile(filepath.Join(cfg.Home, "centmemd.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("start daemon process: %v", err))
		return cli.ExitError
	}

	// Poll socket up to 3 seconds
	startTimeout := 3 * time.Second
	pollInterval := 50 * time.Millisecond
	deadline := time.Now().Add(startTimeout)

	active := false
	for time.Now().Before(deadline) {
		if daemon.ProbeDaemon(sockPath, 50*time.Millisecond) {
			active = true
			break
		}
		time.Sleep(pollInterval)
	}

	if !active {
		cli.WriteError(os.Stderr, cli.Internalf("daemon started (pid %d) but socket %s did not become active within timeout", cmd.Process.Pid, sockPath))
		return cli.ExitError
	}

	pid, _ := readPID(pidPath)
	if pid == 0 {
		pid = cmd.Process.Pid
	}

	_ = prettyPrint(fs, map[string]any{
		"ok":     true,
		"status": "started",
		"pid":    pid,
		"socket": sockPath,
		"port":   cfg.Daemon.Port,
	})
	return cli.ExitOK
}

func cmdRun(args []string) int {
	fs := newDaemonFlagSet("run")
	if err := fs.Parse(args); err != nil {
		cli.WriteError(os.Stderr, cli.Invalidf("flag parse: %v", err))
		return cli.ExitError
	}
	cfg, err := loadDaemonConfig(fs)
	if err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("config: %v", err))
		return cli.ExitError
	}

	st, err := store.Open(cfg)
	if err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("open store: %v", err))
		return cli.ExitError
	}
	defer st.Close()

	emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
	if emb != nil {
		defer emb.Close()
	}

	srv := daemon.NewServer(cfg, st, emb)
	if err := srv.Start(); err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("start server: %v", err))
		return cli.ExitError
	}

	// Trap signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
	}

	if err := srv.Stop(); err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("stop server: %v", err))
		return cli.ExitError
	}

	return cli.ExitOK
}

func cmdStatus(args []string) int {
	fs := newDaemonFlagSet("status")
	if err := fs.Parse(args); err != nil {
		cli.WriteError(os.Stderr, cli.Invalidf("flag parse: %v", err))
		return cli.ExitError
	}
	cfg, err := loadDaemonConfig(fs)
	if err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("config: %v", err))
		return cli.ExitError
	}

	sockPath := cfg.Daemon.SocketPath
	pidPath := cfg.Daemon.PIDPath

	if !daemon.ProbeDaemon(sockPath, 100*time.Millisecond) {
		_ = prettyPrint(fs, map[string]any{
			"ok":     false,
			"status": "stopped",
			"socket": sockPath,
		})
		return cli.ExitOK
	}

	pid, _ := readPID(pidPath)

	out := map[string]any{
		"ok":     true,
		"status": "running",
		"pid":    pid,
		"socket": sockPath,
		"port":   cfg.Daemon.Port,
	}

	// Fetch stats via gRPC client
	client, err := daemon.NewClient(sockPath)
	if err == nil {
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if statsResp, err := client.Stats(ctx, &centmemv1.StatsRequest{}); err == nil {
			out["stats"] = map[string]any{
				"db_path":           statsResp.DbPath,
				"db_size_mb":        statsResp.DbSizeMb,
				"total_memories":    statsResp.TotalMemories,
				"by_type":           statsResp.ByType,
				"by_scope":          statsResp.ByScope,
				"pending_embedding": statsResp.PendingEmbedding,
				"last_compact_at":   statsResp.LastCompactAt,
			}
		}
	}

	_ = prettyPrint(fs, out)
	return cli.ExitOK
}

func cmdStop(args []string) int {
	fs := newDaemonFlagSet("stop")
	if err := fs.Parse(args); err != nil {
		cli.WriteError(os.Stderr, cli.Invalidf("flag parse: %v", err))
		return cli.ExitError
	}
	cfg, err := loadDaemonConfig(fs)
	if err != nil {
		cli.WriteError(os.Stderr, cli.Internalf("config: %v", err))
		return cli.ExitError
	}

	sockPath := cfg.Daemon.SocketPath
	pidPath := cfg.Daemon.PIDPath

	pid, err := readPID(pidPath)
	if err != nil || pid <= 0 {
		// No PID file. Check if daemon is active.
		if !daemon.ProbeDaemon(sockPath, 50*time.Millisecond) {
			_ = os.Remove(sockPath)
			_ = os.Remove(pidPath)
			_ = prettyPrint(fs, map[string]any{
				"ok":     true,
				"status": "already_stopped",
			})
			return cli.ExitOK
		}
	}

	if pid > 0 {
		proc, err := os.FindProcess(pid)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)

			// Wait up to 5s for process to exit
			deadline := time.Now().Add(5 * time.Second)
			stopped := false
			for time.Now().Before(deadline) {
				if !daemon.ProbeDaemon(sockPath, 50*time.Millisecond) {
					stopped = true
					break
				}
				time.Sleep(50 * time.Millisecond)
			}

			if !stopped {
				_ = proc.Signal(syscall.SIGKILL)
			}
		}
	}

	_ = os.Remove(sockPath)
	_ = os.Remove(pidPath)

	_ = prettyPrint(fs, map[string]any{
		"ok":     true,
		"status": "stopped",
		"pid":    pid,
	})
	return cli.ExitOK
}

func readPID(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}
