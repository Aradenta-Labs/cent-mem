package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestE2E_ClaudeCode_SessionEndTrap(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	projDir := t.TempDir()
	transcriptPath := filepath.Join(projDir, "transcript.jsonl")
	transcriptContent := `{"role": "user", "content": "We need to choose a database architecture.", "timestamp": "2026-09-02T10:00:00Z"}
{"role": "assistant", "content": "Decision: We chose SQLite with FTS5 and sqlite-vec for local storage.", "timestamp": "2026-09-02T10:01:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	// 1. Run capture on-demand (simulating what the EXIT trap invokes)
	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--harness", "claude-code", "--scope", "project:claudeapp")
	if code != 0 {
		t.Fatalf("capture run failed: code=%d stderr=%s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v\nstdout: %s", err, stdout)
	}

	if summary.Captured < 1 {
		t.Errorf("expected at least 1 captured item, got %d", summary.Captured)
	}

	// 2. Query summary via CLI
	sumOut, sumErr, sumCode := runCLI(t, home, "capture", "summary")
	if sumCode != 0 {
		t.Fatalf("capture summary failed: code=%d stderr=%s", sumCode, sumErr)
	}
	var fetchedSummary capture.CaptureSummary
	if err := json.Unmarshal([]byte(sumOut), &fetchedSummary); err != nil {
		t.Fatalf("unmarshal fetched summary: %v", err)
	}
	if fetchedSummary.Captured != summary.Captured {
		t.Errorf("summary mismatch: %d vs %d", fetchedSummary.Captured, summary.Captured)
	}

	// 3. Verify memory is in the store and recallable
	recOut, recErr, recCode := runCLI(t, home, "recall", "database architecture SQLite", "--scope", "project:claudeapp")
	if recCode != 0 {
		t.Fatalf("recall failed: code=%d stderr=%s", recCode, recErr)
	}

	var recResult struct {
		OK      bool           `json:"ok"`
		Results []store.Memory `json:"results"`
	}
	if err := json.Unmarshal([]byte(recOut), &recResult); err != nil {
		t.Fatalf("unmarshal recall result: %v\nstdout: %s", err, recOut)
	}
	if len(recResult.Results) == 0 {
		t.Fatalf("expected recall results, got 0")
	}
}

func TestE2E_Antigravity_RealTimeWatch(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcriptPath := filepath.Join(home, "antigravity-transcript.jsonl")

	initial := `{"step_index": 1, "source": "USER_EXPLICIT", "type": "USER_INPUT", "content": "What caching approach should we use?", "created_at": "2026-09-02T10:00:00Z"}
{"step_index": 2, "source": "MODEL", "type": "PLANNER_RESPONSE", "content": "Decision: We will use an LRU memory cache for fast lookups.", "created_at": "2026-09-02T10:01:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(initial), 0600); err != nil {
		t.Fatalf("write initial transcript: %v", err)
	}

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--watch", "--once", "--interval", "50ms", "--harness", "antigravity", "--scope", "project:antigravityapp")
	if code != 0 {
		t.Fatalf("watch capture run failed: code=%d stderr=%s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v\nstdout: %s", err, stdout)
	}

	if summary.Captured < 1 {
		t.Errorf("expected captured >= 1, got %d", summary.Captured)
	}
}

func TestE2E_Codex_StreamingPipe(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	pipeContent := "User: Setup dependencies\nAssistant: Fact: api.endpoint is https://api.centmem.internal\n"

	stdout, stderr, code := runCLIWithStdin(t, home, pipeContent, "capture", "run", "--harness", "codex", "--scope", "project:codexapp")
	if code != 0 {
		t.Fatalf("streaming pipe capture failed: code=%d stderr=%s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v\nstdout: %s", err, stdout)
	}

	if summary.TotalMessages < 1 {
		t.Errorf("expected total messages >= 1, got %d", summary.TotalMessages)
	}
}
