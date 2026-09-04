package store_test

import (
	"context"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// TestFormatEmbedText_TagsAndKey verifies that FormatEmbedText produces
// the canonical structured prefix format specified in Phase C.
func TestFormatEmbedText_TagsAndKey(t *testing.T) {
	// 1. Full metadata: key, tags, and content
	res := store.FormatEmbedText("fact", "db.pool_size", "Set max_open_conns to 25.", []string{"database", "performance", "config"})
	want := "Key: db.pool_size\nTags: database, performance, config\n\nContent: Set max_open_conns to 25."
	if res != want {
		t.Errorf("FormatEmbedText full metadata:\ngot:\n%q\nwant:\n%q", res, want)
	}

	// 2. Content only (no key, no tags)
	res = store.FormatEmbedText("note", "", "Just a regular note.", nil)
	want = "Content: Just a regular note."
	if res != want {
		t.Errorf("FormatEmbedText content only:\ngot:\n%q\nwant:\n%q", res, want)
	}

	// 3. Key and content, no tags
	res = store.FormatEmbedText("fact", "app.mode", "production", []string{})
	want = "Key: app.mode\nContent: production"
	if res != want {
		t.Errorf("FormatEmbedText key only:\ngot:\n%q\nwant:\n%q", res, want)
	}

	// 4. Tags and content, no key
	res = store.FormatEmbedText("note", "", "Important finding.", []string{"ai", "security"})
	want = "Tags: ai, security\n\nContent: Important finding."
	if res != want {
		t.Errorf("FormatEmbedText tags only:\ngot:\n%q\nwant:\n%q", res, want)
	}

	// 5. Empty tag filtering
	res = store.FormatEmbedText("note", "", "Filtered tags.", []string{"", "  ", "valid"})
	want = "Tags: valid\n\nContent: Filtered tags."
	if res != want {
		t.Errorf("FormatEmbedText whitespace tag filtering:\ngot:\n%q\nwant:\n%q", res, want)
	}
}

// TestMigration_M0003_Enqueue verifies that migration m0003 bumps the schema version to 3,
// records embedding_version = 2, and properly enqueues active memories.
func TestMigration_M0003_Enqueue(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Verify schema_version is 3
	version, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != "3" {
		t.Errorf("expected schema_version = '3', got %q", version)
	}

	// Verify embedding_version in meta
	var embVersion string
	err = s.DB().QueryRowContext(ctx, "SELECT value FROM meta WHERE key = 'embedding_version'").Scan(&embVersion)
	if err != nil {
		t.Fatalf("query embedding_version: %v", err)
	}
	if embVersion != "2" {
		t.Errorf("expected embedding_version = '2', got %q", embVersion)
	}

	// Put a new memory
	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "test active enqueue",
		Tags:    []string{"test"},
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}

	pending, err := s.PendingEmbeddings(ctx)
	if err != nil {
		t.Fatalf("PendingEmbeddings: %v", err)
	}
	if pending < 1 {
		t.Errorf("expected at least 1 pending embedding, got %d", pending)
	}

	// Clear queue to test EnqueueAllActive idempotency
	_, err = s.DB().ExecContext(ctx, "DELETE FROM embed_queue")
	if err != nil {
		t.Fatalf("clear embed_queue: %v", err)
	}

	enqueued, err := s.EnqueueAllActive(ctx)
	if err != nil {
		t.Fatalf("EnqueueAllActive: %v", err)
	}
	if enqueued < 1 {
		t.Errorf("expected at least 1 newly enqueued memory, got %d", enqueued)
	}

	// Enqueue again should be idempotent (0 new rows)
	enqueued2, err := s.EnqueueAllActive(ctx)
	if err != nil {
		t.Fatalf("EnqueueAllActive second pass: %v", err)
	}
	if enqueued2 != 0 {
		t.Errorf("expected 0 rows on second EnqueueAllActive pass, got %d", enqueued2)
	}

	// LoadMemoryEmbedTexts returns structured text
	texts, err := s.LoadMemoryEmbedTexts(ctx, []int64{id})
	if err != nil {
		t.Fatalf("LoadMemoryEmbedTexts: %v", err)
	}
	expectedText := store.FormatEmbedText("note", "", "test active enqueue", []string{"test"})
	if texts[id] != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, texts[id])
	}
}
