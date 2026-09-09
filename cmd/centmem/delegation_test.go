package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/daemon"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestDelegation_ActiveDaemonAndFallback(t *testing.T) {
	dir := t.TempDir()
	sockDir, err := os.MkdirTemp("/tmp", "cmd-deleg-")
	if err != nil {
		sockDir = dir
	}
	defer os.RemoveAll(sockDir)

	sockPath := filepath.Join(sockDir, "d.sock")
	dbPath := filepath.Join(dir, "centmem.db")

	cfg := config.Config{
		Home:   dir,
		DBPath: dbPath,
		Daemon: config.DaemonConfig{
			SocketPath: sockPath,
			PIDPath:    filepath.Join(dir, "centmemd.pid"),
		},
		Retention: config.Retention{
			FactKeepDays:           0,
			NoteSummarizeAfterDays: 30,
			LogSummarizeAfterDays:  7,
		},
		Search: config.DefaultSearchConfig(),
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	emb := embed.NewStub(384)

	srv := daemon.NewServer(cfg, st, emb)
	if err := srv.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}

	t.Setenv("CENTMEM_DAEMON_SOCKET", sockPath)

	// 1. Run put command while daemon is active -> delegated
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := run([]string{
		"put",
		"--home", dir,
		"--scope", "project:deleg",
		"--type", "note",
		"--content", "delegated memory content via daemon",
		"--no-suggest",
	})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("put returned exitCode %d, want 0", exitCode)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var putOut map[string]any
	if err := json.Unmarshal(buf.Bytes(), &putOut); err != nil {
		t.Fatalf("unmarshal put output: %v, raw: %s", err, buf.String())
	}
	if putOut["ok"] != true || putOut["id"] == nil {
		t.Fatalf("unexpected put output: %+v", putOut)
	}

	// 2. Recall via daemon
	r, w, _ = os.Pipe()
	os.Stdout = w

	exitCode = run([]string{
		"recall",
		"delegated memory",
		"--home", dir,
		"--scope", "project:deleg",
	})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("recall returned exitCode %d, want 0", exitCode)
	}
	buf.Reset()
	_, _ = buf.ReadFrom(r)
	var recallOut map[string]any
	if err := json.Unmarshal(buf.Bytes(), &recallOut); err != nil {
		t.Fatalf("unmarshal recall output: %v, raw: %s", err, buf.String())
	}
	results, _ := recallOut["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("expected recall results, got none: %s", buf.String())
	}

	// 3. Test --direct escape hatch
	r, w, _ = os.Pipe()
	os.Stdout = w

	exitCode = run([]string{
		"recall",
		"delegated memory",
		"--home", dir,
		"--scope", "project:deleg",
		"--direct",
	})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("recall --direct returned exitCode %d, want 0", exitCode)
	}

	// 4. Test CENTMEM_DIRECT=1 env var
	t.Setenv("CENTMEM_DIRECT", "1")
	r, w, _ = os.Pipe()
	os.Stdout = w

	exitCode = run([]string{
		"put",
		"--home", dir,
		"--scope", "project:deleg",
		"--type", "note",
		"--content", "direct mode memory bypasses daemon",
		"--no-suggest",
	})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("put with CENTMEM_DIRECT=1 returned exitCode %d, want 0", exitCode)
	}
	t.Setenv("CENTMEM_DIRECT", "")

	// 5. Failover / Graceful Fallback: Stop daemon -> subsequent commands must succeed via direct SQLite mode
	srv.Stop()
	st.Close()

	r, w, _ = os.Pipe()
	os.Stdout = w

	exitCode = run([]string{
		"put",
		"--home", dir,
		"--scope", "project:deleg",
		"--type", "note",
		"--content", "fallback direct memory after daemon stop",
		"--no-suggest",
	})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("put after daemon stop returned exitCode %d, want 0", exitCode)
	}
	buf.Reset()
	_, _ = buf.ReadFrom(r)
	putOut = nil
	if err := json.Unmarshal(buf.Bytes(), &putOut); err != nil {
		t.Fatalf("unmarshal fallback put output: %v, raw: %s", err, buf.String())
	}
	if putOut["ok"] != true {
		t.Fatalf("expected fallback put to succeed, got: %+v", putOut)
	}
}
