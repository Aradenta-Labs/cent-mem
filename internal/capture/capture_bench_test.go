package capture_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func BenchmarkCapture_HeuristicClassify(b *testing.B) {
	cfg := capture.DefaultCaptureConfig()
	classifier := capture.NewHeuristicClassifier(cfg)
	ctx := context.Background()

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Let's decide on the database architecture.", Timestamp: time.Now()},
		{Role: "assistant", Content: "We decided to use SQLite-vec for local embeddings.", Timestamp: time.Now()},
		{Role: "user", Content: "What is the API endpoint?", Timestamp: time.Now()},
		{Role: "assistant", Content: "API docs are at https://api.centmem.io/v1 for version v1.3.0.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Run go get github.com/mattn/go-sqlite3 to install dependencies.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Here is code:\n```go\nfunc Init() {}\n```", Timestamp: time.Now()},
		{Role: "assistant", Content: "Completed store initialization.", Timestamp: time.Now()},
		{Role: "assistant", Content: "The root cause was foreign keys, resolved by enabling PRAGMA foreign_keys.", Timestamp: time.Now()},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := classifier.Classify(ctx, messages, nil)
		if err != nil || len(items) == 0 {
			b.Fatalf("Classify failed: %v", err)
		}
	}
}

func BenchmarkCapture_SessionDedup(b *testing.B) {
	tempHome := b.TempDir()
	sessionID := "bench-session-123"
	dedup, err := capture.NewSessionDedup(tempHome, sessionID)
	if err != nil {
		b.Fatalf("NewSessionDedup: %v", err)
	}

	item := capture.CaptureItem{
		Category: "decision",
		Content:  "We decided to use SQLite-vec for local embeddings.",
	}
	_ = dedup.MarkSaved(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dedup.IsSessionDuplicate(item)
	}
}

func BenchmarkCapture_WritePipeline(b *testing.B) {
	tempHome := b.TempDir()
	cfg := config.Config{
		Home:   tempHome,
		DBPath: filepath.Join(tempHome, "bench.db"),
	}

	st, err := store.Open(cfg)
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	captureCfg := capture.DefaultCaptureConfig()
	captureCfg.Scope = "project:bench"
	writer := capture.NewWriter(st, captureCfg)
	sessionID := "bench-sess-write"

	item := capture.CaptureItem{
		Category: "decision",
		Content:  "Adopt SQLite WAL mode for concurrency",
		Tags:     []string{"decision", "db"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, _, err := writer.Write(ctx, item, sessionID)
		if err != nil || id <= 0 {
			b.Fatalf("Write failed: %v", err)
		}
	}
}
