package store_test

import (
	"context"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestLinks_MigrationAndSchemaVersion(t *testing.T) {
	s := newTestStore(t)
	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != "5" {
		t.Fatalf("expected schema_version '5', got %q", v)
	}
}

func TestLinks_CreateConstraints(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "Decision 1",
	})
	if err != nil {
		t.Fatalf("PutMemory m1: %v", err)
	}

	m2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "Decision 2",
	})
	if err != nil {
		t.Fatalf("PutMemory m2: %v", err)
	}

	// 1. Self-link must fail
	if _, err := s.CreateLink(ctx, m1, m1, "supersedes", false); err == nil {
		t.Errorf("expected error linking memory to itself, got nil")
	}

	// 2. Invalid relation must fail
	if _, err := s.CreateLink(ctx, m1, m2, "invalid-relation", false); err == nil {
		t.Errorf("expected error with invalid relation, got nil")
	}

	// 3. Non-existent source memory must fail
	if _, err := s.CreateLink(ctx, 99999, m2, "supersedes", false); err == nil {
		t.Errorf("expected error with non-existent source memory, got nil")
	}

	// 4. Non-existent target memory must fail
	if _, err := s.CreateLink(ctx, m1, 99999, "supersedes", false); err == nil {
		t.Errorf("expected error with non-existent target memory, got nil")
	}
}

func TestLinks_CRUDAndLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "Deploy on Fly.io",
	})
	if err != nil {
		t.Fatalf("PutMemory m1: %v", err)
	}

	m2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "Switched from Fly.io to AWS ECS",
	})
	if err != nil {
		t.Fatalf("PutMemory m2: %v", err)
	}

	// Create suggested link m2 -> m1
	link1, err := s.CreateLink(ctx, m2, m1, "supersedes", true)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if !link1.Suggested {
		t.Errorf("expected link1.Suggested to be true")
	}
	if link1.Relation != "supersedes" {
		t.Errorf("expected relation 'supersedes', got %q", link1.Relation)
	}

	// Check GetLinksForMemory with includeSuggested = false
	out, in, err := s.GetLinksForMemory(ctx, m2, false)
	if err != nil {
		t.Fatalf("GetLinksForMemory: %v", err)
	}
	if len(out) != 0 || len(in) != 0 {
		t.Errorf("expected 0 confirmed links, got out=%d, in=%d", len(out), len(in))
	}

	// Check GetLinksForMemory with includeSuggested = true
	out, in, err = s.GetLinksForMemory(ctx, m2, true)
	if err != nil {
		t.Fatalf("GetLinksForMemory: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing link for m2, got %d", len(out))
	}
	if out[0].TargetContent != "Deploy on Fly.io" {
		t.Errorf("expected target content 'Deploy on Fly.io', got %q", out[0].TargetContent)
	}
	if out[0].SourceContent != "Switched from Fly.io to AWS ECS" {
		t.Errorf("expected source content 'Switched from Fly.io to AWS ECS', got %q", out[0].SourceContent)
	}

	// Incoming link for m1
	out1, in1, err := s.GetLinksForMemory(ctx, m1, true)
	if err != nil {
		t.Fatalf("GetLinksForMemory m1: %v", err)
	}
	if len(out1) != 0 || len(in1) != 1 {
		t.Fatalf("expected 0 outgoing, 1 incoming for m1, got out=%d, in=%d", len(out1), len(in1))
	}
	if in1[0].FromID != m2 {
		t.Errorf("expected in1[0].FromID == %d, got %d", m2, in1[0].FromID)
	}

	// Confirm link
	if err := s.ConfirmLink(ctx, link1.ID); err != nil {
		t.Fatalf("ConfirmLink: %v", err)
	}

	// Now includeSuggested = false should find the confirmed link
	out, in, err = s.GetLinksForMemory(ctx, m2, false)
	if err != nil {
		t.Fatalf("GetLinksForMemory after confirm: %v", err)
	}
	if len(out) != 1 || out[0].Suggested {
		t.Errorf("expected 1 confirmed outgoing link, got %v", out)
	}

	// Test Dismiss on confirmed link (should fail because suggested = 0)
	if err := s.DismissLink(ctx, link1.ID); err == nil {
		t.Errorf("expected DismissLink to fail on confirmed link, got nil")
	}

	// Create a new suggested link to test dismissal
	m3, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "ECS requires IAM roles",
	})
	if err != nil {
		t.Fatalf("PutMemory m3: %v", err)
	}

	link2, err := s.CreateLink(ctx, m3, m2, "depends-on", true)
	if err != nil {
		t.Fatalf("CreateLink link2: %v", err)
	}

	// Dismiss link2
	if err := s.DismissLink(ctx, link2.ID); err != nil {
		t.Fatalf("DismissLink link2: %v", err)
	}

	// Check link2 is gone
	if _, err := s.GetLinkByID(ctx, link2.ID); err == nil {
		t.Errorf("expected link2 to be gone after dismissal")
	}

	// Test DeleteLink with invalid relation
	if _, err := s.DeleteLink(ctx, m2, m1, "invalid_rel"); err == nil {
		t.Errorf("expected DeleteLink with invalid relation to fail, got nil")
	}

	// Test DeleteLink by endpoints
	deleted, err := s.DeleteLink(ctx, m2, m1, "supersedes")
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected deleted == 1, got %d", deleted)
	}
}

func TestLinks_CascadeOnMemoryForget(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "Alpha",
	})
	if err != nil {
		t.Fatalf("PutMemory m1: %v", err)
	}

	m2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:p1",
		Type:    "note",
		Content: "Beta",
	})
	if err != nil {
		t.Fatalf("PutMemory m2: %v", err)
	}

	link, err := s.CreateLink(ctx, m1, m2, "supports", false)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}

	// Forget memory m1
	if _, err := s.Forget(ctx, []int64{m1}, nil, nil, nil); err != nil {
		t.Fatalf("Forget m1: %v", err)
	}

	// Verify link was cascade-deleted
	if _, err := s.GetLinkByID(ctx, link.ID); err == nil {
		t.Errorf("expected link to be deleted by cascade when m1 was forgotten")
	}
}

func TestLinks_BatchFetch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "A"})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "B"})
	m3, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "C"})

	_, _ = s.CreateLink(ctx, m1, m2, "supports", false)
	_, _ = s.CreateLink(ctx, m2, m3, "refines", false)

	batch, err := s.GetLinksForMemories(ctx, []int64{m1, m2, m3}, false)
	if err != nil {
		t.Fatalf("GetLinksForMemories: %v", err)
	}

	if len(batch[m1]) != 1 {
		t.Errorf("expected 1 link for m1, got %d", len(batch[m1]))
	}
	if len(batch[m2]) != 2 {
		t.Errorf("expected 2 links for m2 (1 in, 1 out), got %d", len(batch[m2]))
	}
	if len(batch[m3]) != 1 {
		t.Errorf("expected 1 link for m3, got %d", len(batch[m3]))
	}
}
