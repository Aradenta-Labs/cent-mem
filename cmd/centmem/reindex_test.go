package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// TestReindex_CLI_EndToEnd verifies that centmem reindex drains the queue,
// writes embeddings, returns valid JSON, and is idempotent.
func TestReindex_CLI_EndToEnd(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	// Initialize store
	_, _, code := runCLI(t, home, "init")
	if code != 0 {
		t.Fatalf("init failed with exit code %d", code)
	}

	// Insert memories
	_, _, code = runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "first memory for reindex test", "--tags", "test,v1")
	if code != 0 {
		t.Fatalf("put 1 failed with code %d", code)
	}
	_, _, code = runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "second memory for reindex test", "--tags", "test,v2")
	if code != 0 {
		t.Fatalf("put 2 failed with code %d", code)
	}

	// 1. Dry run with --all reports total active memories without processing
	stdout, stderr, code := runCLI(t, home, "reindex", "--all", "--dry-run")
	if code != 0 {
		t.Fatalf("reindex --all --dry-run failed with code %d: %s", code, stderr)
	}
	var dryResp map[string]any
	if err := json.Unmarshal([]byte(stdout), &dryResp); err != nil {
		t.Fatalf("unmarshal dry-run JSON: %v (stdout: %s)", err, stdout)
	}
	if dryResp["ok"] != true {
		t.Errorf("expected ok=true, got %v", dryResp["ok"])
	}
	if dryResp["reindexed"] != float64(0) {
		t.Errorf("expected reindexed=0 on dry-run, got %v", dryResp["reindexed"])
	}
	if dryResp["pending"].(float64) < 2 {
		t.Errorf("expected pending >= 2, got %v", dryResp["pending"])
	}

	// 2. Reindex with --all: re-enqueues active memories and re-embeds them
	stdout, stderr, code = runCLI(t, home, "reindex", "--all", "--batch", "16", "--max-time", "10s")
	if code != 0 {
		t.Fatalf("reindex --all failed with code %d: %s", code, stderr)
	}
	var allResp map[string]any
	if err := json.Unmarshal([]byte(stdout), &allResp); err != nil {
		t.Fatalf("unmarshal reindex --all JSON: %v", err)
	}
	if allResp["ok"] != true {
		t.Errorf("expected ok=true, got %v", allResp["ok"])
	}
	if allResp["reindexed"].(float64) < 2 {
		t.Errorf("expected reindexed >= 2 with --all, got %v", allResp["reindexed"])
	}
	if allResp["pending"] != float64(0) {
		t.Errorf("expected pending=0 after --all, got %v", allResp["pending"])
	}

	// 3. Idempotency: reindex again with empty queue
	stdout, stderr, code = runCLI(t, home, "reindex")
	if code != 0 {
		t.Fatalf("second reindex failed with code %d: %s", code, stderr)
	}
	var secondResp map[string]any
	if err := json.Unmarshal([]byte(stdout), &secondResp); err != nil {
		t.Fatalf("unmarshal second reindex JSON: %v", err)
	}
	if secondResp["reindexed"] != float64(0) {
		t.Errorf("expected reindexed=0 on clean queue, got %v", secondResp["reindexed"])
	}
	if secondResp["pending"] != float64(0) {
		t.Errorf("expected pending=0 on clean queue, got %v", secondResp["pending"])
	}

	// 4. Test reindex with manual queue entries
	t.Setenv("CENTMEM_HOME", home)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	ctx := context.Background()
	enqueued, err := s.EnqueueAllActive(ctx)
	s.Close()
	if err != nil {
		t.Fatalf("EnqueueAllActive: %v", err)
	}
	if enqueued < 2 {
		t.Fatalf("expected enqueued >= 2, got %d", enqueued)
	}

	// Dry run without --all now shows pending > 0
	stdout, stderr, code = runCLI(t, home, "reindex", "--dry-run")
	if code != 0 {
		t.Fatalf("reindex --dry-run failed with code %d: %s", code, stderr)
	}
	dryResp = nil
	if err := json.Unmarshal([]byte(stdout), &dryResp); err != nil {
		t.Fatalf("unmarshal dry-run JSON: %v", err)
	}
	if dryResp["pending"].(float64) < 2 {
		t.Errorf("expected pending >= 2, got %v", dryResp["pending"])
	}

	// Normal reindex drains these pending items
	stdout, stderr, code = runCLI(t, home, "reindex")
	if code != 0 {
		t.Fatalf("reindex failed with code %d: %s", code, stderr)
	}
	var drainResp map[string]any
	if err := json.Unmarshal([]byte(stdout), &drainResp); err != nil {
		t.Fatalf("unmarshal drain JSON: %v", err)
	}
	if drainResp["reindexed"].(float64) < 2 {
		t.Errorf("expected reindexed >= 2, got %v", drainResp["reindexed"])
	}
	if drainResp["pending"] != float64(0) {
		t.Errorf("expected pending=0 after drain, got %v", drainResp["pending"])
	}
}
