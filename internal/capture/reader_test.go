package capture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestReader_JSONL(t *testing.T) {
	jsonlContent := `{"role":"user","content":"Let's decide on SQLite","timestamp":"2026-09-02T10:00:00Z"}
{"role":"assistant","content":"We chose SQLite with WAL mode.","timestamp":"2026-09-02T10:00:05Z"}
`
	tmpFile := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(tmpFile, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	msgs, err := capture.ReadTranscript(tmpFile)
	if err != nil {
		t.Fatalf("ReadTranscript error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Let's decide on SQLite" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "We chose SQLite with WAL mode." {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2026-09-02T10:00:00Z")
	if !msgs[0].Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, msgs[0].Timestamp)
	}
}

func TestReader_JSONArray(t *testing.T) {
	jsonContent := `[
		{"role":"user","message":"What database should we use?"},
		{"role":"assistant","message":"We decided to use SQLite-vec for embeddings."}
	]`
	r := strings.NewReader(jsonContent)
	msgs, err := capture.ReadTranscriptFromPipe(r)
	if err != nil {
		t.Fatalf("ReadTranscriptFromPipe error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].Role != "assistant" || !strings.Contains(msgs[1].Content, "SQLite-vec") {
		t.Errorf("unexpected message: %+v", msgs[1])
	}
}

func TestReader_AntigravityFormat(t *testing.T) {
	jsonlContent := `{"source":"USER_EXPLICIT","type":"USER_INPUT","content":"Please set api.url to https://api.centmem.io"}
{"source":"MODEL","type":"PLANNER_RESPONSE","content":"Done! Saved api.url."}
`
	r := strings.NewReader(jsonlContent)
	msgs, err := capture.ReadTranscriptFromPipe(r)
	if err != nil {
		t.Fatalf("ReadTranscriptFromPipe error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", msgs[1].Role)
	}
}

func TestReader_PlainText(t *testing.T) {
	text := `User: We agreed on using port 8080 for the local server.
Assistant: Understood. I will update the config to port 8080.
`
	r := strings.NewReader(text)
	msgs, err := capture.ReadTranscriptFromPipe(r)
	if err != nil {
		t.Fatalf("ReadTranscriptFromPipe error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || !strings.Contains(msgs[0].Content, "port 8080") {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || !strings.Contains(msgs[1].Content, "port 8080") {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestReader_Empty(t *testing.T) {
	r := strings.NewReader("   \n\t  ")
	msgs, err := capture.ReadTranscriptFromPipe(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for empty input, got %d", len(msgs))
	}
}
