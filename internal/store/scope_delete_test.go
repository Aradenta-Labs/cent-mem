package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestStore_DeleteScopeTree(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// 1. Setup hierarchical scopes:
	//    project:alpha
	//    project:alpha/agent:agent-1
	//    project:alpha/agent:agent-1/session:sess-1
	//    project:beta (should NOT be deleted)
	pMem, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha",
		Type:    "note",
		Content: "Project alpha root memory",
	})
	if err != nil {
		t.Fatalf("put pMem: %v", err)
	}

	aMem, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha/agent:agent-1",
		Type:    "note",
		Content: "Agent memory in alpha",
	})
	if err != nil {
		t.Fatalf("put aMem: %v", err)
	}

	sMem, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha/agent:agent-1/session:sess-1",
		Type:    "log",
		Content: "Session log in alpha",
	})
	if err != nil {
		t.Fatalf("put sMem: %v", err)
	}

	bMem, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:beta",
		Type:    "note",
		Content: "Project beta memory",
	})
	if err != nil {
		t.Fatalf("put bMem: %v", err)
	}

	abMem, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha-beta",
		Type:    "note",
		Content: "Project alpha-beta memory (should NOT be deleted)",
	})
	if err != nil {
		t.Fatalf("put abMem: %v", err)
	}

	// 2. Add memory links and embeddings / vectors
	if _, err := s.CreateLink(ctx, pMem, aMem, "supersedes", false); err != nil {
		t.Fatalf("create link: %v", err)
	}
	if _, err := s.CreateLink(ctx, bMem, sMem, "depends-on", false); err != nil {
		t.Fatalf("create link to session: %v", err)
	}

	// Insert dummy vector in memories_vec
	dummyVec := make([]byte, 384*4)
	if _, err := s.DB().ExecContext(ctx, "INSERT OR REPLACE INTO memories_vec(memory_id, embedding) VALUES (?, ?)", pMem, dummyVec); err != nil {
		t.Fatalf("insert memories_vec: %v", err)
	}

	// Verify links exist
	outgoing, _, err := s.GetLinksForMemory(ctx, pMem, false)
	if err != nil || len(outgoing) == 0 {
		t.Fatalf("expected links on pMem: %v", err)
	}

	// 3. Test DeleteScopeTree on project:alpha
	summary, err := s.DeleteScopeTree(ctx, "project:alpha")
	if err != nil {
		t.Fatalf("DeleteScopeTree failed: %v", err)
	}

	if summary.ScopePath != "project:alpha" {
		t.Errorf("summary.ScopePath = %q, want 'project:alpha'", summary.ScopePath)
	}
	if summary.ScopesDeleted != 3 {
		t.Errorf("summary.ScopesDeleted = %d, want 3", summary.ScopesDeleted)
	}
	if summary.MemoriesDeleted != 3 {
		t.Errorf("summary.MemoriesDeleted = %d, want 3", summary.MemoriesDeleted)
	}

	// Verify project:alpha and its descendants are gone from scopes table
	var count int
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM scopes WHERE path = 'project:alpha' OR path LIKE 'project:alpha/%'").Scan(&count); err != nil {
		t.Fatalf("query scopes count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 scopes for project:alpha tree, found %d", count)
	}

	// Verify project:beta still exists
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM scopes WHERE path = 'project:beta'").Scan(&count); err != nil {
		t.Fatalf("query beta scope count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 scope for project:beta, found %d", count)
	}

	// Verify memories in alpha are deleted
	for _, id := range []int64{pMem, aMem, sMem} {
		m, err := s.GetMemory(ctx, id)
		if err != store.ErrNotFound && m != nil {
			t.Errorf("expected memory %d to be deleted, got %v", id, m)
		}
	}

	// Verify memory in beta still exists
	bGot, err := s.GetMemory(ctx, bMem)
	if err != nil || bGot == nil {
		t.Errorf("expected beta memory to exist, got err: %v", err)
	}

	// Verify project:alpha-beta scope and memory still exist (not deleted by prefix match)
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM scopes WHERE path = 'project:alpha-beta'").Scan(&count); err != nil {
		t.Fatalf("query alpha-beta scope count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 scope for project:alpha-beta, found %d", count)
	}
	abGot, err := s.GetMemory(ctx, abMem)
	if err != nil || abGot == nil {
		t.Errorf("expected alpha-beta memory to exist, got err: %v", err)
	}

	// Verify memories_vec cleaned up
	var vecCount int
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM memories_vec WHERE memory_id = ?", pMem).Scan(&vecCount); err != nil {
		t.Fatalf("query memories_vec count: %v", err)
	}
	if vecCount != 0 {
		t.Errorf("expected 0 vectors in memories_vec for deleted memory, got %d", vecCount)
	}

	// Verify memory_links cleaned up
	var linkCount int
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_links WHERE from_id IN (?, ?, ?) OR to_id IN (?, ?, ?)", pMem, aMem, sMem, pMem, aMem, sMem).Scan(&linkCount); err != nil {
		t.Fatalf("query memory_links count: %v", err)
	}
	if linkCount != 0 {
		t.Errorf("expected 0 links for deleted memories, got %d", linkCount)
	}

	// Verify delete event in events table
	var evCount int
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE op = 'delete' AND scope_path = 'project:alpha'").Scan(&evCount); err != nil {
		t.Fatalf("query events count: %v", err)
	}
	if evCount != 1 {
		t.Errorf("expected 1 delete event, found %d", evCount)
	}
}

func TestStore_DeleteScopeTree_RejectsGlobal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.DeleteScopeTree(ctx, "global")
	if err == nil {
		t.Fatal("expected error when deleting root scope 'global', got nil")
	}

	_, err = s.DeleteScopeTree(ctx, "  global  ")
	if err == nil {
		t.Fatal("expected error when deleting trimmed 'global', got nil")
	}
}

func TestStore_DeleteScopeTree_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.DeleteScopeTree(ctx, "project:does-not-exist")
	if err == nil {
		t.Fatal("expected error when deleting non-existent scope, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}
