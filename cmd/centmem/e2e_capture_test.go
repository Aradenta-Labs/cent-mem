package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

// TestE2E_CaptureRun verifies the complete end-to-end capture lifecycle:
// 1. Initializing the store
// 2. Ingesting a multi-turn transcript fixture containing decisions, facts, dependencies, code, and logs
// 3. Verifying recall retrieval of captured memories
// 4. Verifying get fact retrieval
// 5. Inspecting the capture summary output
func TestE2E_CaptureRun(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	// 1. Initialize centmem store
	_, _, initCode := runCLI(t, home, "init", "--non-interactive")
	if initCode != 0 {
		t.Fatalf("init failed with code %d", initCode)
	}

	// 2. Create fixture transcript file
	fixturePath := filepath.Join(t.TempDir(), "session_transcript.jsonl")
	rawMsgs := []capture.TranscriptMessage{
		{Role: "user", Content: "Let's decide on the database architecture for our project."},
		{Role: "assistant", Content: "We decided to use SQLite-vec for local vector embeddings."},
		{Role: "user", Content: "What is the API endpoint and version we are shipping?"},
		{Role: "assistant", Content: "API documentation is at https://api.centmem.io/v1 for release v1.3.0."},
		{Role: "user", Content: "How do we install the Go dependencies?"},
		{Role: "assistant", Content: "Run go get github.com/mattn/go-sqlite3 to install the SQLite driver."},
		{Role: "assistant", Content: "Here is the initialization snippet:\n```go\nfunc Init() bool {\n  return true\n}\n```"},
		{Role: "assistant", Content: "Completed core store schema migration."},
		{Role: "assistant", Content: "The root cause was missing foreign keys, resolved by enabling PRAGMA foreign_keys."},
	}
	var sb strings.Builder
	for _, m := range rawMsgs {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal fixture msg: %v", err)
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(fixturePath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write transcript fixture: %v", err)
	}

	// 3. Execute centmem capture run
	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", fixturePath, "--scope", "project:cent-mem-e2e", "--harness", "antigravity")
	if code != 0 {
		t.Fatalf("capture run failed (code %d): %s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("failed to parse capture run summary JSON: %v (stdout: %s)", err, stdout)
	}

	if !summary.OK {
		t.Errorf("summary.OK = false, want true")
	}
	if summary.TotalMessages != len(rawMsgs) {
		t.Errorf("summary.TotalMessages = %d, want %d", summary.TotalMessages, len(rawMsgs))
	}
	if summary.Captured < 4 {
		t.Errorf("summary.Captured = %d, want >= 4", summary.Captured)
	}
	if summary.Harness != "antigravity" {
		t.Errorf("summary.Harness = %q, want 'antigravity'", summary.Harness)
	}

	// 4. Verify recall retrieves the captured decision memory
	recallOut, _, recallCode := runCLI(t, home, "recall", "SQLite-vec", "--scope", "project:cent-mem-e2e", "--top", "5")
	if recallCode != 0 {
		t.Fatalf("recall failed (code %d)", recallCode)
	}
	var recallRes map[string]any
	if err := json.Unmarshal([]byte(recallOut), &recallRes); err != nil {
		t.Fatalf("unmarshal recall output: %v", err)
	}
	results, ok := recallRes["results"].([]any)
	if !ok || len(results) == 0 {
		t.Fatalf("expected recall results for SQLite-vec, got: %v", recallRes)
	}

	// 5. Verify get retrieves the captured URL fact
	getURLOut, _, getURLCode := runCLI(t, home, "get", "--scope", "project:cent-mem-e2e", "--key", "url.api.centmem.io.v1")
	if getURLCode != 0 {
		t.Fatalf("get url fact failed (code %d)", getURLCode)
	}
	var urlFact map[string]any
	if err := json.Unmarshal([]byte(getURLOut), &urlFact); err != nil {
		t.Fatalf("unmarshal get fact output: %v", err)
	}
	if urlFact["value"] != "https://api.centmem.io/v1" {
		t.Errorf("url fact value = %v, want https://api.centmem.io/v1", urlFact["value"])
	}

	// 6. Verify get retrieves the captured dependency fact
	getDepOut, _, getDepCode := runCLI(t, home, "get", "--scope", "project:cent-mem-e2e", "--key", "dep.github.com.mattn.go-sqlite3")
	if getDepCode != 0 {
		t.Fatalf("get dependency fact failed (code %d)", getDepCode)
	}
	var depFact map[string]any
	if err := json.Unmarshal([]byte(getDepOut), &depFact); err != nil {
		t.Fatalf("unmarshal get dep output: %v", err)
	}
	valStr, _ := depFact["value"].(string)
	if !strings.Contains(valStr, "github.com/mattn/go-sqlite3") {
		t.Errorf("dep fact value %q should contain github.com/mattn/go-sqlite3", valStr)
	}

	// 7. Verify capture summary command retrieves the stored summary
	sumOut, _, sumCode := runCLI(t, home, "capture", "summary")
	if sumCode != 0 {
		t.Fatalf("capture summary command failed (code %d)", sumCode)
	}
	var loadedSummary capture.CaptureSummary
	if err := json.Unmarshal([]byte(sumOut), &loadedSummary); err != nil {
		t.Fatalf("unmarshal capture summary command output: %v", err)
	}
	if loadedSummary.SessionID != summary.SessionID {
		t.Errorf("loaded summary session ID = %q, want %q", loadedSummary.SessionID, summary.SessionID)
	}
	if loadedSummary.Captured != summary.Captured {
		t.Errorf("loaded summary captured = %d, want %d", loadedSummary.Captured, summary.Captured)
	}
}

// TestE2E_CaptureRun_Deduplication verifies that repeating a capture run across sessions
// detects existing memories in the store and skips duplicates.
func TestE2E_CaptureRun_Deduplication(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init", "--non-interactive")

	fixturePath := filepath.Join(t.TempDir(), "dedup_transcript.jsonl")
	content := `{"role":"user","content":"We chose PostgreSQL for production analytics."}` + "\n"
	if err := os.WriteFile(fixturePath, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Pass 1: Fresh capture
	stdout1, _, code1 := runCLI(t, home, "capture", "run", "--transcript", fixturePath, "--scope", "project:dedup-e2e")
	if code1 != 0 {
		t.Fatalf("pass 1 failed: %d", code1)
	}
	var sum1 capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout1), &sum1)
	if sum1.Captured == 0 {
		t.Fatalf("pass 1 expected captured > 0, got 0")
	}

	// Pass 2: Repeat on same transcript in a new session
	stdout2, _, code2 := runCLI(t, home, "capture", "run", "--transcript", fixturePath, "--scope", "project:dedup-e2e")
	if code2 != 0 {
		t.Fatalf("pass 2 failed: %d", code2)
	}
	var sum2 capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout2), &sum2)
	if sum2.Captured != 0 {
		t.Errorf("pass 2 expected 0 captured memories (all duplicates), got %d", sum2.Captured)
	}
	if sum2.SkippedDuplicate == 0 {
		t.Errorf("pass 2 expected skipped_duplicate > 0, got %d", sum2.SkippedDuplicate)
	}
}

// TestE2E_CaptureRun_FallbackChain verifies that when the configured local LLM
// is offline, the classifier seamlessly falls back to the heuristic classifier.
func TestE2E_CaptureRun_FallbackChain(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init", "--non-interactive")

	runCLI(t, home, "config", "set", "capture.backend", "local-llm")
	runCLI(t, home, "config", "set", "capture.local_llm_endpoint", "http://127.0.0.1:54321/unreachable")
	runCLI(t, home, "config", "set", "capture.scope", "project:fallback-e2e")

	fixturePath := filepath.Join(t.TempDir(), "fallback_transcript.jsonl")
	content := `{"role":"user","content":"We decided to use Redis for session caching."}` + "\n"
	_ = os.WriteFile(fixturePath, []byte(content), 0644)

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", fixturePath)
	if code != 0 {
		t.Fatalf("capture run with fallback failed: code=%d stderr=%s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}

	if summary.Captured < 1 {
		t.Errorf("expected at least 1 captured memory via heuristic fallback, got %d", summary.Captured)
	}

	recallOut, _, recallCode := runCLI(t, home, "recall", "Redis", "--scope", "project:fallback-e2e")
	if recallCode != 0 {
		t.Fatalf("recall failed: %d", recallCode)
	}
	if !strings.Contains(recallOut, "Redis") {
		t.Errorf("expected recall to retrieve captured memory: %s", recallOut)
	}
}

// TestE2E_CaptureRun_OpenAICompatibleBackend verifies cloud inference via OpenAI-compatible endpoint.
func TestE2E_CaptureRun_OpenAICompatibleBackend(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init", "--non-interactive")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "[{\"category\":\"decision\",\"content\":\"Adopt strict TypeScript typing\",\"confidence\":0.95,\"tags\":[\"decision\",\"ts\"]}]"
				}
			}]
		}`))
	}))
	defer server.Close()

	t.Setenv("TEST_OPENAI_KEY_E2E", "sk-mock-key")
	runCLI(t, home, "config", "set", "capture.backend", "openai-compatible")
	runCLI(t, home, "config", "set", "capture.api_base_url", server.URL)
	runCLI(t, home, "config", "set", "capture.api_key_env", "TEST_OPENAI_KEY_E2E")
	runCLI(t, home, "config", "set", "capture.scope", "project:cloud-e2e")

	fixturePath := filepath.Join(t.TempDir(), "cloud_transcript.jsonl")
	_ = os.WriteFile(fixturePath, []byte(`{"role":"user","content":"Let's configure TypeScript settings."}`+"\n"), 0644)

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", fixturePath)
	if code != 0 {
		t.Fatalf("capture run failed (code %d): %s", code, stderr)
	}

	var summary capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout), &summary)
	if summary.Captured != 1 {
		t.Errorf("expected 1 captured memory, got %d", summary.Captured)
	}

	recallOut, _, _ := runCLI(t, home, "recall", "strict TypeScript", "--scope", "project:cloud-e2e")
	if !strings.Contains(recallOut, "Adopt strict TypeScript typing") {
		t.Errorf("expected recall to find captured memory, got: %s", recallOut)
	}
}

// TestE2E_Capture_WatcherIncremental verifies real-time tailing of an actively growing transcript.
func TestE2E_Capture_WatcherIncremental(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init", "--non-interactive")

	liveTranscript := filepath.Join(t.TempDir(), "live_transcript.jsonl")
	f, err := os.OpenFile(liveTranscript, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("open live transcript: %v", err)
	}
	defer f.Close()

	// Initial message
	_, _ = f.WriteString(`{"role":"user","content":"We decided to enable gzip compression."}` + "\n")
	_ = f.Sync()

	// Run capture on the initial file
	stdout, _, code := runCLI(t, home, "capture", "run", "--transcript", liveTranscript, "--scope", "project:watcher-e2e")
	if code != 0 {
		t.Fatalf("initial capture run failed: %d", code)
	}

	var sum1 capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout), &sum1)
	if sum1.Captured != 1 {
		t.Errorf("expected 1 captured memory initially, got %d", sum1.Captured)
	}

	// Append second message
	time.Sleep(20 * time.Millisecond)
	_, _ = f.WriteString(`{"role":"user","content":"We decided to set keepalive to 60s."}` + "\n")
	_ = f.Sync()

	// Run capture again
	stdout2, _, code2 := runCLI(t, home, "capture", "run", "--transcript", liveTranscript, "--scope", "project:watcher-e2e")
	if code2 != 0 {
		t.Fatalf("second capture run failed: %d", code2)
	}

	var sum2 capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout2), &sum2)
	// The first decision was skipped as duplicate, second decision is captured
	if sum2.Captured != 1 {
		t.Errorf("expected 1 newly captured memory, got %d", sum2.Captured)
	}
	if sum2.SkippedDuplicate < 1 {
		t.Errorf("expected at least 1 skipped duplicate, got %d", sum2.SkippedDuplicate)
	}
}
