package store_test

import (
	"context"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestStore_ArchiveMemory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:       "project:test",
		Type:        "note",
		Content:     "memory to be archived",
		SourceAgent: "agent-1",
	})
	if err != nil {
		t.Fatalf("put memory: %v", err)
	}

	if err := s.ArchiveMemory(ctx, id); err != nil {
		t.Fatalf("archive memory: %v", err)
	}

	m, err := s.GetMemory(ctx, id)
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if m.Status != "archived" {
		t.Fatalf("expected status 'archived', got %q", m.Status)
	}

	// Archiving again is a no-op
	if err := s.ArchiveMemory(ctx, id); err != nil {
		t.Fatalf("archive already archived memory: %v", err)
	}
}

func TestStore_ArchiveMemoriesByTag(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	scope := "project:test"
	_, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:       scope,
		Type:        "note",
		Content:     "doc chunk 1",
		Tags:        []string{"docs", "README.md", "intro"},
		SourceAgent: "docs-capture",
	})
	if err != nil {
		t.Fatalf("put doc 1: %v", err)
	}

	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:       scope,
		Type:        "note",
		Content:     "doc chunk 2",
		Tags:        []string{"docs", "README.md", "usage"},
		SourceAgent: "docs-capture",
	})
	if err != nil {
		t.Fatalf("put doc 2: %v", err)
	}

	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:       scope,
		Type:        "note",
		Content:     "other doc chunk",
		Tags:        []string{"docs", "ARCH.md", "overview"},
		SourceAgent: "docs-capture",
	})
	if err != nil {
		t.Fatalf("put other doc: %v", err)
	}

	count, err := s.ArchiveMemoriesByTag(ctx, scope, "docs-capture", "README.md")
	if err != nil {
		t.Fatalf("archive by tag: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 archived memories, got %d", count)
	}

	active, err := s.List(ctx, store.ListQuery{
		ScopePath: scope,
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("list active memories: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active memory remaining, got %d", len(active))
	}
	if active[0].Content != "other doc chunk" {
		t.Fatalf("unexpected remaining memory: %s", active[0].Content)
	}
}

func TestStore_UpdateMemoryContent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:       "project:test",
		Type:        "note",
		Content:     "[file: a.go:10] [TODO] old line number",
		Tags:        []string{"comment", "todo", "a.go"},
		SourceAgent: "comment-capture",
	})
	if err != nil {
		t.Fatalf("put comment: %v", err)
	}

	newContent := "[file: a.go:25] [TODO] old line number"
	newTags := []string{"comment", "todo", "a.go", "shifted"}
	if err := s.UpdateMemoryContent(ctx, id, newContent, newTags); err != nil {
		t.Fatalf("update memory content: %v", err)
	}

	m, err := s.GetMemory(ctx, id)
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if m.Content != newContent {
		t.Fatalf("expected content %q, got %q", newContent, m.Content)
	}
	if len(m.Tags) != 4 {
		t.Fatalf("expected 4 tags, got %d", len(m.Tags))
	}
}

func TestStore_HasActiveMemory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	scope := "project:test"
	content := "active note content"
	has, err := s.HasActiveMemory(ctx, scope, "note", content)
	if err != nil {
		t.Fatalf("check has: %v", err)
	}
	if has {
		t.Fatal("expected false before insertion")
	}

	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   scope,
		Type:    "note",
		Content: content,
	})
	if err != nil {
		t.Fatalf("put memory: %v", err)
	}

	has, err = s.HasActiveMemory(ctx, scope, "note", content)
	if err != nil {
		t.Fatalf("check has after put: %v", err)
	}
	if !has {
		t.Fatal("expected true after insertion")
	}

	// Archive the memory
	if err := s.ArchiveMemory(ctx, id); err != nil {
		t.Fatalf("archive memory: %v", err)
	}

	has, err = s.HasActiveMemory(ctx, scope, "note", content)
	if err != nil {
		t.Fatalf("check has after archive: %v", err)
	}
	if has {
		t.Fatal("expected false after archive")
	}
}
