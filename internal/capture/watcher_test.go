package capture

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcher_InitialReadAndAppend(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	initialContent := `{"role": "user", "content": "Question 1", "timestamp": "2026-09-02T10:00:00Z"}
{"role": "assistant", "content": "Answer 1", "timestamp": "2026-09-02T10:01:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var processed []TranscriptMessage
	var mu sync.Mutex

	w, err := NewWatcher(WatcherConfig{
		TranscriptPath: transcriptPath,
		Home:           tmpDir,
		PollInterval:   50 * time.Millisecond,
	}, func(msgs []TranscriptMessage) error {
		mu.Lock()
		defer mu.Unlock()
		processed = append(processed, msgs...)
		return nil
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	// 1. Initial poll
	n, err := w.PollOnce()
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 messages on initial poll, got %d", n)
	}

	mu.Lock()
	if len(processed) != 2 {
		t.Fatalf("expected 2 processed messages, got %d", len(processed))
	}
	mu.Unlock()

	// 2. Poll without changes
	n, err = w.PollOnce()
	if err != nil {
		t.Fatalf("PollOnce second time: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new messages on unchanged file, got %d", n)
	}

	// 3. Append new content
	appendContent := `{"role": "user", "content": "Question 2", "timestamp": "2026-09-02T10:02:00Z"}
`
	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.WriteString(appendContent); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()

	n, err = w.PollOnce()
	if err != nil {
		t.Fatalf("PollOnce after append: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 new message after append, got %d", n)
	}

	mu.Lock()
	if len(processed) != 3 {
		t.Fatalf("expected 3 total processed messages, got %d", len(processed))
	}
	if processed[2].Content != "Question 2" {
		t.Errorf("expected 'Question 2', got %q", processed[2].Content)
	}
	mu.Unlock()
}

func TestWatcher_FileTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	content := `{"role": "user", "content": "Old content", "timestamp": "2026-09-02T10:00:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var count int
	w, err := NewWatcher(WatcherConfig{
		TranscriptPath: transcriptPath,
		Home:           tmpDir,
	}, func(msgs []TranscriptMessage) error {
		count += len(msgs)
		return nil
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	_, _ = w.PollOnce()
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Truncate file with smaller fresh content
	freshContent := `{"role": "user", "content": "New", "timestamp": "2026-09-02T10:05:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(freshContent), 0600); err != nil {
		t.Fatalf("truncate and write: %v", err)
	}

	n, err := w.PollOnce()
	if err != nil {
		t.Fatalf("PollOnce after truncation: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 message from truncated fresh file, got %d", n)
	}
}

func TestWatcher_StatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
	statePath := filepath.Join(tmpDir, "custom-state.json")

	content := `{"role": "user", "content": "Persist test", "timestamp": "2026-09-02T10:00:00Z"}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	w1, err := NewWatcher(WatcherConfig{
		TranscriptPath: transcriptPath,
		StateFilePath:  statePath,
	}, nil)
	if err != nil {
		t.Fatalf("NewWatcher 1: %v", err)
	}

	n, err := w1.PollOnce()
	if err != nil || n != 1 {
		t.Fatalf("w1 PollOnce n=%d, err=%v", n, err)
	}

	// Create new watcher instance with same state path
	w2, err := NewWatcher(WatcherConfig{
		TranscriptPath: transcriptPath,
		StateFilePath:  statePath,
	}, nil)
	if err != nil {
		t.Fatalf("NewWatcher 2: %v", err)
	}

	n2, err := w2.PollOnce()
	if err != nil {
		t.Fatalf("w2 PollOnce err: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("expected 0 new messages on reloaded watcher, got %d", n2)
	}
}

func TestWatcher_WatchContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
	_ = os.WriteFile(transcriptPath, []byte(""), 0600)

	w, err := NewWatcher(WatcherConfig{
		TranscriptPath: transcriptPath,
		PollInterval:   20 * time.Millisecond,
		Home:           tmpDir,
	}, nil)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = w.Watch(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}
