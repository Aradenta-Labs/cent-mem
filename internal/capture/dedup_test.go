package capture_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestSessionDedup_InSession(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "test-session-123"

	dedup, err := capture.NewSessionDedup(tempDir, sessionID)
	if err != nil {
		t.Fatalf("NewSessionDedup: %v", err)
	}

	item := capture.CaptureItem{
		Category: "decision",
		Content:  "Use SQLite-vec",
	}

	if dedup.IsSessionDuplicate(item) {
		t.Errorf("item should not be duplicate initially")
	}

	if err := dedup.MarkSaved(item); err != nil {
		t.Fatalf("MarkSaved: %v", err)
	}

	if !dedup.IsSessionDuplicate(item) {
		t.Errorf("item should be detected as duplicate after MarkSaved")
	}

	// Test persistence across new instance for same session
	dedup2, err := capture.NewSessionDedup(tempDir, sessionID)
	if err != nil {
		t.Fatalf("NewSessionDedup instance 2: %v", err)
	}
	if !dedup2.IsSessionDuplicate(item) {
		t.Errorf("item should be detected as duplicate in restored SessionDedup instance")
	}

	// Test cleanup
	if err := dedup2.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	dedupPath := filepath.Join(tempDir, "session-"+sessionID+".dedup.json")
	if _, err := os.Stat(dedupPath); !os.IsNotExist(err) {
		t.Errorf("expected session dedup file to be deleted")
	}
}

func TestStoreDuplicate_Detection(t *testing.T) {
	tempHome := t.TempDir()
	cfg := config.Config{
		Home:   tempHome,
		DBPath: filepath.Join(tempHome, "test.db"),
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// Insert an existing memory directly into store
	_, _, err = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "Existing decision",
		Tags:    []string{"decision"},
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}

	itemDup := capture.CaptureItem{
		Category: "decision",
		Content:  "Existing decision",
	}
	itemNew := capture.CaptureItem{
		Category: "decision",
		Content:  "Brand new decision",
	}

	isDup, err := capture.IsStoreDuplicate(ctx, itemDup, "global", st)
	if err != nil {
		t.Fatalf("IsStoreDuplicate: %v", err)
	}
	if !isDup {
		t.Errorf("expected itemDup to be detected as store duplicate")
	}

	isNewDup, err := capture.IsStoreDuplicate(ctx, itemNew, "global", st)
	if err != nil {
		t.Fatalf("IsStoreDuplicate: %v", err)
	}
	if isNewDup {
		t.Errorf("expected itemNew to NOT be duplicate")
	}
}
