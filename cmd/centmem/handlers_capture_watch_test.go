package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestCLI_CaptureRun_WatchMode(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcriptPath := filepath.Join(home, "watch-transcript.jsonl")

	initial := `{"role": "user", "content": "Decision: We use SQLite-vec for local storage.", "timestamp": "2026-09-02T10:00:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(initial), 0600); err != nil {
		t.Fatalf("write initial transcript: %v", err)
	}

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--watch", "--once", "--interval", "50ms", "--scope", "project:testapp", "--harness", "antigravity")
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr: %s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v\nstdout: %s", err, stdout)
	}

	if summary.Captured < 1 {
		t.Errorf("expected at least 1 captured item, got %d", summary.Captured)
	}
}

func TestCLI_CaptureRun_WatchMissingTranscript(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--watch")
	if code == 0 {
		t.Fatalf("expected non-zero exit code on missing transcript, got stdout: %s", stdout)
	}

	if !strings.Contains(stderr, "missing transcript") {
		t.Errorf("expected stderr to mention 'missing transcript', got: %s", stderr)
	}
}

func TestCLI_CaptureConvert_Command(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	inputPath := filepath.Join(home, "cursor.json")
	outputPath := filepath.Join(home, "cursor.centmem.jsonl")

	cursorContent := `{
		"messages": [
			{"role": "user", "text": "What database are we using?", "timestamp": "2026-09-02T10:00:00Z"},
			{"role": "assistant", "text": "We chose SQLite.", "timestamp": "2026-09-02T10:01:00Z"}
		]
	}`
	if err := os.WriteFile(inputPath, []byte(cursorContent), 0600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	stdout, stderr, code := runCLI(t, home, "capture", "convert", "--harness", "cursor", "--input", inputPath, "--output", outputPath)
	if code != 0 {
		t.Fatalf("convert failed: code=%d stderr=%s", code, stderr)
	}

	var res struct {
		OK       bool   `json:"ok"`
		Output   string `json:"output"`
		Messages int    `json:"messages"`
		Harness  string `json:"harness"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal convert output: %v\nstdout: %s", err, stdout)
	}

	if !res.OK || res.Messages != 2 || res.Harness != "cursor" {
		t.Errorf("unexpected convert response: %+v", res)
	}

	outData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read converted output: %v", err)
	}

	if !bytes.Contains(outData, []byte("What database are we using?")) {
		t.Errorf("output missing user message: %s", string(outData))
	}
}
