package capture_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestCaptureDocs_HeadingsAndHierarchy(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	docPath := filepath.Join(dir, "architecture.md")
	docContent := `# Architecture Overview

This is the system overview.

## Storage Engine

SQLite is used for local durability.

### WAL Mode

Write-Ahead Logging provides concurrency.

## Search Engine

Hybrid search with RRF fusion.
`
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	res, err := capture.CaptureDocs(ctx, st, capture.DocsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture docs: %v", err)
	}
	if res.Scanned != 1 {
		t.Fatalf("expected 1 file scanned, got %d", res.Scanned)
	}
	if len(res.Items) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(res.Items))
	}

	// Verify breadcrumbs in items
	foundWAL := false
	for _, it := range res.Items {
		if strings.Contains(it.Content, "[architecture.md # Architecture Overview > Storage Engine > WAL Mode]") {
			foundWAL = true
		}
	}
	if !foundWAL {
		t.Fatal("expected breadcrumb for nested heading WAL Mode")
	}

	// Verify caching on second run: mtime hasn't changed
	secondRes, err := capture.CaptureDocs(ctx, st, capture.DocsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("second capture docs: %v", err)
	}
	if len(secondRes.Items) != 0 {
		t.Fatalf("expected 0 new items on unchanged file, got %d", len(secondRes.Items))
	}
}

func TestCaptureDocs_HeadlessDoc(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	docPath := filepath.Join(dir, "notes.txt")
	docContent := `First paragraph explaining project background without any markdown headings.

Second paragraph with more details about operational requirements.`
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	res, err := capture.CaptureDocs(ctx, st, capture.DocsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture docs: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 chunk for headless doc, got %d", len(res.Items))
	}
	if !strings.Contains(res.Items[0].Content, "[notes.txt # Overview]") {
		t.Fatalf("expected Overview breadcrumb in headless doc, got %s", res.Items[0].Content)
	}
}

func TestCaptureDocs_Tombstoning(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	file1 := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(file1, []byte("# Guide\n\nContent for guide.\n"), 0644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	res, err := capture.CaptureDocs(ctx, st, capture.DocsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture docs: %v", err)
	}
	if res.MemoriesCreated != 1 {
		t.Fatalf("expected 1 memory created, got %d", res.MemoriesCreated)
	}

	// Delete file
	if err := os.Remove(file1); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	// Sleep slightly to ensure distinct mtime/state
	time.Sleep(10 * time.Millisecond)

	tombRes, err := capture.CaptureDocs(ctx, st, capture.DocsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture docs with deleted file: %v", err)
	}
	if tombRes.MemoriesUpdated != 1 {
		t.Fatalf("expected 1 memory updated (tombstoned), got %d", tombRes.MemoriesUpdated)
	}

	// Verify store state: memory should now be archived
	active, err := st.List(ctx, store.ListQuery{
		ScopePath: capture.ResolveDefaultScope(dir),
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected 0 active memories after tombstoning, got %d", len(active))
	}
}

func TestCaptureDocs_CodeFencesPreserved(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	docPath := filepath.Join(dir, "setup.md")
	docContent := `# Setup Instructions

Run this script:

` + "```bash" + `
# This is a bash comment, not a heading
curl -sSL https://example.com/install.sh | bash
` + "```" + `

## Verification

Check that everything works.
`
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("write setup.md: %v", err)
	}

	res, err := capture.CaptureDocs(ctx, st, capture.DocsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture docs with code fence: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected exactly 2 chunks (Setup Instructions and Verification), got %d: %+v", len(res.Items), res.Items)
	}

	// Verify chunk 1 contains the bash comment and breadcrumb is Setup Instructions
	if !strings.Contains(res.Items[0].Content, "[setup.md # Setup Instructions]") {
		t.Errorf("unexpected chunk 1 header: %s", res.Items[0].Content)
	}
	if !strings.Contains(res.Items[0].Content, "# This is a bash comment, not a heading") {
		t.Errorf("expected bash comment inside code block in chunk 1: %s", res.Items[0].Content)
	}

	// Verify chunk 2 breadcrumb is Setup Instructions > Verification
	if !strings.Contains(res.Items[1].Content, "[setup.md # Setup Instructions > Verification]") {
		t.Errorf("unexpected chunk 2 header: %s", res.Items[1].Content)
	}
}
