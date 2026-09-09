package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/daemon"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestCentmemd_LifecycleInProcess(t *testing.T) {
	dir := t.TempDir()
	sockDir, err := os.MkdirTemp("/tmp", "cmd-life-")
	if err != nil {
		sockDir = dir
	}
	defer os.RemoveAll(sockDir)

	sockPath := filepath.Join(sockDir, "d.sock")
	pidPath := filepath.Join(dir, "centmemd.pid")

	// 1. Status when stopped
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := run([]string{"status", "--socket", sockPath, "--pid-file", pidPath})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("status returned exit code %d, want 0", exitCode)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var statusOut map[string]any
	_ = json.Unmarshal(buf.Bytes(), &statusOut)
	if statusOut["ok"] != false || statusOut["status"] != "stopped" {
		t.Fatalf("expected stopped status, got: %+v", statusOut)
	}

	// 2. Start daemon server
	cfg := config.Config{
		Home:   dir,
		DBPath: filepath.Join(dir, "centmem.db"),
		Daemon: config.DaemonConfig{
			SocketPath: sockPath,
			PIDPath:    pidPath,
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
	defer st.Close()

	emb := embed.NewStub(384)
	srv := daemon.NewServer(cfg, st, emb)
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	// 3. Status when running
	r, w, _ = os.Pipe()
	os.Stdout = w

	exitCode = run([]string{"status", "--socket", sockPath, "--pid-file", pidPath})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("status returned %d, want 0", exitCode)
	}
	buf.Reset()
	_, _ = buf.ReadFrom(r)
	statusOut = nil
	_ = json.Unmarshal(buf.Bytes(), &statusOut)
	if statusOut["ok"] != true || statusOut["status"] != "running" {
		t.Fatalf("expected running status, got: %+v", statusOut)
	}

	// 4. Stop daemon
	if err := srv.Stop(); err != nil {
		t.Fatalf("srv.Stop: %v", err)
	}

	// Verify status when stopped again
	r, w, _ = os.Pipe()
	os.Stdout = w

	exitCode = run([]string{"status", "--socket", sockPath, "--pid-file", pidPath})

	w.Close()
	os.Stdout = origStdout

	if exitCode != 0 {
		t.Fatalf("status returned %d, want 0", exitCode)
	}
	buf.Reset()
	_, _ = buf.ReadFrom(r)
	statusOut = nil
	_ = json.Unmarshal(buf.Bytes(), &statusOut)
	if statusOut["ok"] != false || statusOut["status"] != "stopped" {
		t.Fatalf("expected stopped status after srv.Stop, got: %+v", statusOut)
	}

	// Verify socket and PID files cleaned up
	if _, err := os.Stat(sockPath); err == nil {
		t.Errorf("socket file %s was not unlinked", sockPath)
	}
	if _, err := os.Stat(pidPath); err == nil {
		t.Errorf("pid file %s was not unlinked", pidPath)
	}
}

func TestCentmemd_BinaryLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary test in short mode")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "centmemd")

	// Compile binary
	buildCmd := exec.Command("go", "build", "-tags", "fts5", "-o", binPath, ".")
	buildCmd.Dir = "."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile centmemd: %v\nOutput: %s", err, string(out))
	}

	homeDir := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(homeDir, 0700)

	sockDir, err := os.MkdirTemp("/tmp", "cmd-binlife-")
	if err != nil {
		sockDir = homeDir
	}
	defer os.RemoveAll(sockDir)

	sockPath := filepath.Join(sockDir, "d.sock")
	pidPath := filepath.Join(homeDir, "centmemd.pid")

	// Start daemon
	startCmd := exec.Command(binPath, "start", "--home", homeDir, "--socket", sockPath, "--pid-file", pidPath)
	startOut, err := startCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("centmemd start failed: %v\nOutput: %s", err, string(startOut))
	}
	var startMap map[string]any
	if err := json.Unmarshal(startOut, &startMap); err != nil {
		t.Fatalf("unmarshal start output: %v, raw: %s", err, string(startOut))
	}
	if startMap["ok"] != true || startMap["status"] != "started" {
		t.Fatalf("unexpected startMap: %+v", startMap)
	}

	time.Sleep(100 * time.Millisecond)

	// Status daemon
	statusCmd := exec.Command(binPath, "status", "--home", homeDir, "--socket", sockPath, "--pid-file", pidPath)
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("centmemd status failed: %v\nOutput: %s", err, string(statusOut))
	}
	var statusMap map[string]any
	if err := json.Unmarshal(statusOut, &statusMap); err != nil {
		t.Fatalf("unmarshal status output: %v, raw: %s", err, string(statusOut))
	}
	if statusMap["ok"] != true || statusMap["status"] != "running" {
		t.Fatalf("unexpected statusMap: %+v", statusMap)
	}

	// Stop daemon
	stopCmd := exec.Command(binPath, "stop", "--home", homeDir, "--socket", sockPath, "--pid-file", pidPath)
	stopOut, err := stopCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("centmemd stop failed: %v\nOutput: %s", err, string(stopOut))
	}
	var stopMap map[string]any
	if err := json.Unmarshal(stopOut, &stopMap); err != nil {
		t.Fatalf("unmarshal stop output: %v, raw: %s", err, string(stopOut))
	}
	if stopMap["ok"] != true || stopMap["status"] != "stopped" {
		t.Fatalf("unexpected stopMap: %+v", stopMap)
	}

	// Verify unlinked
	if _, err := os.Stat(sockPath); err == nil {
		t.Errorf("socket file %s still exists", sockPath)
	}
	if _, err := os.Stat(pidPath); err == nil {
		t.Errorf("pid file %s still exists", pidPath)
	}
}
