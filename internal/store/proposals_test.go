package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestProposals_MigrationAndSchemaVersion(t *testing.T) {
	s := newTestStore(t)
	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != "6" {
		t.Fatalf("expected schema_version '6', got %q", v)
	}
}

func TestProposals_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &store.Proposal{
		ScopePath:    "project:test-proposals",
		ProposalType: "link",
		Title:        "Link related decisions",
		Reasoning:    "Decision 10 supersedes 9",
		PayloadJSON:  `{"from_id":10,"to_id":9,"relation":"supersedes"}`,
	}

	id, err := s.CreateProposal(ctx, p)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected id > 0, got %d", id)
	}

	got, err := s.GetProposal(ctx, id)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if got.ID != id {
		t.Errorf("expected ID=%d, got %d", id, got.ID)
	}
	if got.ProposalType != "link" {
		t.Errorf("expected ProposalType=link, got %q", got.ProposalType)
	}
	if got.Status != "pending" {
		t.Errorf("expected Status=pending, got %q", got.Status)
	}
	if got.Title != p.Title {
		t.Errorf("expected Title=%q, got %q", p.Title, got.Title)
	}
	if got.Reasoning != p.Reasoning {
		t.Errorf("expected Reasoning=%q, got %q", p.Reasoning, got.Reasoning)
	}
	if got.AppliedAt != nil {
		t.Errorf("expected AppliedAt to be nil, got %v", got.AppliedAt)
	}

	// Invalid proposal type
	_, err = s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:test-proposals",
		ProposalType: "invalid_type",
		Title:        "Bad",
		Reasoning:    "Bad",
		PayloadJSON:  "{}",
	})
	if err == nil {
		t.Fatalf("expected error for invalid proposal_type, got nil")
	}

	// Non-existent proposal
	_, err = s.GetProposal(ctx, 999999)
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing proposal, got %v", err)
	}
}

func TestProposals_ListAndFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create 3 proposals in project:p1 and 1 in project:p2
	_, _ = s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:p1",
		ProposalType: "link",
		Title:        "P1 link",
		Reasoning:    "reason",
		PayloadJSON:  "{}",
	})
	p2ID, _ := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:p1",
		ProposalType: "merge",
		Title:        "P1 merge",
		Reasoning:    "reason",
		PayloadJSON:  "{}",
	})
	_, _ = s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:p1",
		ProposalType: "update",
		Title:        "P1 update",
		Reasoning:    "reason",
		PayloadJSON:  "{}",
	})
	_, _ = s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:p2",
		ProposalType: "archive",
		Title:        "P2 archive",
		Reasoning:    "reason",
		PayloadJSON:  "{}",
	})

	// Dismiss p2
	if err := s.DismissProposal(ctx, p2ID); err != nil {
		t.Fatalf("DismissProposal: %v", err)
	}

	// List all
	all, err := s.ListProposals(ctx, store.ProposalListQuery{})
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("expected 4 proposals, got %d", len(all))
	}

	// Filter by scope
	p1List, err := s.ListProposals(ctx, store.ProposalListQuery{ScopePath: "project:p1"})
	if err != nil {
		t.Fatalf("ListProposals p1: %v", err)
	}
	if len(p1List) != 3 {
		t.Errorf("expected 3 proposals in project:p1, got %d", len(p1List))
	}

	// Filter by status pending
	pending, err := s.ListProposals(ctx, store.ProposalListQuery{Status: "pending"})
	if err != nil {
		t.Fatalf("ListProposals pending: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("expected 3 pending proposals, got %d", len(pending))
	}

	// Filter by status dismissed
	dismissed, err := s.ListProposals(ctx, store.ProposalListQuery{Status: "dismissed"})
	if err != nil {
		t.Fatalf("ListProposals dismissed: %v", err)
	}
	if len(dismissed) != 1 || dismissed[0].ID != p2ID {
		t.Errorf("expected 1 dismissed proposal with ID %d", p2ID)
	}

	// Filter by proposal_type
	archives, err := s.ListProposals(ctx, store.ProposalListQuery{ProposalType: "archive"})
	if err != nil {
		t.Fatalf("ListProposals archive: %v", err)
	}
	if len(archives) != 1 {
		t.Errorf("expected 1 archive proposal, got %d", len(archives))
	}
}

func TestProposals_ApplyLink(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:app",
		Type:    "note",
		Content: "Use PostgreSQL",
	})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:app",
		Type:    "note",
		Content: "Use SQLite",
	})

	linkPayload, _ := json.Marshal(store.LinkProposalPayload{
		FromID:   m2,
		ToID:     m1,
		Relation: "supersedes",
	})

	propID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:app",
		ProposalType: "link",
		Title:        "Switch to SQLite",
		Reasoning:    "SQLite is simpler for local single-user use",
		PayloadJSON:  string(linkPayload),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	if err := s.ApplyProposal(ctx, propID); err != nil {
		t.Fatalf("ApplyProposal: %v", err)
	}

	// Verify proposal state
	p, err := s.GetProposal(ctx, propID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if p.Status != "applied" {
		t.Errorf("expected status 'applied', got %q", p.Status)
	}
	if p.AppliedAt == nil {
		t.Errorf("expected non-nil AppliedAt")
	}

	// Verify link was created in memory_links with suggested=0
	outgoing, incoming, err := s.GetLinksForMemory(ctx, m2, true)
	if err != nil {
		t.Fatalf("GetLinksForMemory: %v", err)
	}
	if len(outgoing) != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", len(outgoing))
	}
	if outgoing[0].ToID != m1 || outgoing[0].Relation != "supersedes" || outgoing[0].Suggested {
		t.Errorf("link details mismatch: %+v", outgoing[0])
	}
	_ = incoming

	// Applying again should error
	if err := s.ApplyProposal(ctx, propID); err == nil {
		t.Errorf("expected error applying already applied proposal, got nil")
	}
}

func TestProposals_ApplyMerge(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:auth",
		Type:    "note",
		Content: "Decision Part 1: JWT for tokens",
		Tags:    []string{"jwt", "auth"},
	})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:auth",
		Type:    "note",
		Content: "Decision Part 2: Refresh token in HttpOnly cookie",
		Tags:    []string{"security", "auth"},
	})

	mergePayload, _ := json.Marshal(store.MergeProposalPayload{
		SourceIDs:     []int64{m1, m2},
		TargetContent: "Consolidated Auth Architecture: JWT access token with HttpOnly refresh cookies",
		TargetTags:    []string{"consolidated"},
	})

	propID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:auth",
		ProposalType: "merge",
		Title:        "Consolidate Auth Decisions",
		Reasoning:    "Combine Part 1 and Part 2 into a single unified architecture note",
		PayloadJSON:  string(mergePayload),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	if err := s.ApplyProposal(ctx, propID); err != nil {
		t.Fatalf("ApplyProposal: %v", err)
	}

	// Verify sources are summarized
	src1, err := s.GetMemory(ctx, m1)
	if err != nil {
		t.Fatalf("GetMemory src1: %v", err)
	}
	if src1.Status != "summarized" {
		t.Errorf("expected src1 status 'summarized', got %q", src1.Status)
	}

	src2, err := s.GetMemory(ctx, m2)
	if err != nil {
		t.Fatalf("GetMemory src2: %v", err)
	}
	if src2.Status != "summarized" {
		t.Errorf("expected src2 status 'summarized', got %q", src2.Status)
	}

	// Verify new merged memory exists
	memories, err := s.List(ctx, store.ListQuery{
		ScopePath: "project:auth",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("List active memories: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 active merged memory, got %d", len(memories))
	}
	merged := memories[0]
	if merged.Content != "Consolidated Auth Architecture: JWT access token with HttpOnly refresh cookies" {
		t.Errorf("unexpected merged content: %q", merged.Content)
	}

	// Verify supersedes links exist from merged -> src1 and merged -> src2
	outgoing, _, err := s.GetLinksForMemory(ctx, merged.ID, true)
	if err != nil {
		t.Fatalf("GetLinksForMemory: %v", err)
	}
	if len(outgoing) != 2 {
		t.Fatalf("expected 2 supersedes links, got %d", len(outgoing))
	}
	for _, l := range outgoing {
		if l.Relation != "supersedes" {
			t.Errorf("expected relation 'supersedes', got %q", l.Relation)
		}
	}
}

func TestProposals_ApplyUpdateAndArchive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ops",
		Type:    "note",
		Content: "Deploy target is AWS EC2",
		Tags:    []string{"deploy"},
	})

	// Test Update
	updatePayload, _ := json.Marshal(store.UpdateProposalPayload{
		TargetID: m1,
		Content:  "Deploy target updated to Fly.io",
		Tags:     []string{"deploy", "cloud"},
	})
	updatePropID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ops",
		ProposalType: "update",
		Title:        "Update deployment target",
		Reasoning:    "Migrated from AWS to Fly.io",
		PayloadJSON:  string(updatePayload),
	})
	if err != nil {
		t.Fatalf("CreateProposal update: %v", err)
	}

	if err := s.ApplyProposal(ctx, updatePropID); err != nil {
		t.Fatalf("ApplyProposal update: %v", err)
	}

	mem1, err := s.GetMemory(ctx, m1)
	if err != nil {
		t.Fatalf("GetMemory m1: %v", err)
	}
	if mem1.Content != "Deploy target updated to Fly.io" {
		t.Errorf("expected updated content, got %q", mem1.Content)
	}

	// Test Archive
	archivePayload, _ := json.Marshal(store.ArchiveProposalPayload{
		TargetID: m1,
		Reason:   "Obsolete infrastructure note",
	})
	archivePropID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ops",
		ProposalType: "archive",
		Title:        "Archive deployment note",
		Reasoning:    "Deprecated stack",
		PayloadJSON:  string(archivePayload),
	})
	if err != nil {
		t.Fatalf("CreateProposal archive: %v", err)
	}

	if err := s.ApplyProposal(ctx, archivePropID); err != nil {
		t.Fatalf("ApplyProposal archive: %v", err)
	}

	mem1Archived, err := s.GetMemory(ctx, m1)
	if err != nil {
		t.Fatalf("GetMemory m1 archived: %v", err)
	}
	if mem1Archived.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", mem1Archived.Status)
	}
}

func TestProposals_RollbackOnFailure(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Proposal referencing non-existent memory ID
	badPayload, _ := json.Marshal(store.LinkProposalPayload{
		FromID:   99991,
		ToID:     99992,
		Relation: "supports",
	})
	propID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:err",
		ProposalType: "link",
		Title:        "Broken link",
		Reasoning:    "Should fail",
		PayloadJSON:  string(badPayload),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	err = s.ApplyProposal(ctx, propID)
	if err == nil {
		t.Fatalf("expected error applying broken proposal, got nil")
	}

	// Proposal status should still be pending (not applied)
	p, err := s.GetProposal(ctx, propID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if p.Status != "pending" {
		t.Errorf("expected status to remain 'pending' after rollback, got %q", p.Status)
	}
}

func TestProposals_EventsAndStateGuards(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 1. Create two memories
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ev",
		Type:    "note",
		Content: "Decision 1",
		Tags:    []string{"d1"},
	})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ev",
		Type:    "note",
		Content: "Decision 2",
		Tags:    []string{"d2"},
	})

	// 2. Test Merge proposal and verify events table audit entries
	mergePayload, _ := json.Marshal(store.MergeProposalPayload{
		SourceIDs:     []int64{m1, m2, m1}, // Contains duplicate to test deduplication
		TargetContent: "Consolidated Decisions",
		TargetTags:    []string{"unified"},
	})
	mergePropID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ev",
		ProposalType: "merge",
		Title:        "Merge decisions",
		Reasoning:    "Consolidate",
		PayloadJSON:  string(mergePayload),
	})
	if err != nil {
		t.Fatalf("CreateProposal merge: %v", err)
	}

	if err := s.ApplyProposal(ctx, mergePropID); err != nil {
		t.Fatalf("ApplyProposal merge: %v", err)
	}

	// Cannot dismiss already applied proposal
	if err := s.DismissProposal(ctx, mergePropID); err == nil {
		t.Fatalf("expected error dismissing applied proposal, got nil")
	}

	// Verify events table has op='insert' for new memory and op='summarize' for source memories
	events, err := s.EventsSince(ctx, 0, 50)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var insertCount, summarizeCount int
	for _, ev := range events {
		if ev.Op == "insert" {
			insertCount++
		}
		if ev.Op == "summarize" {
			summarizeCount++
		}
	}
	if summarizeCount != 2 {
		t.Errorf("expected 2 'summarize' events for source memories, got %d", summarizeCount)
	}
	if insertCount < 3 { // 2 initial puts + 1 merged insert
		t.Errorf("expected at least 3 'insert' events, got %d", insertCount)
	}

	// Cannot merge non-active memories again
	badMergePayload, _ := json.Marshal(store.MergeProposalPayload{
		SourceIDs:     []int64{m1},
		TargetContent: "Try merge already summarized",
	})
	badPropID, _ := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ev",
		ProposalType: "merge",
		Title:        "Bad merge",
		Reasoning:    "fail",
		PayloadJSON:  string(badMergePayload),
	})
	if err := s.ApplyProposal(ctx, badPropID); err == nil {
		t.Fatalf("expected error merging non-active memory, got nil")
	}

	// 3. Test Update proposal rejects non-active memory
	updateBadPayload, _ := json.Marshal(store.UpdateProposalPayload{
		TargetID: m1,
		Content:  "Updated content",
	})
	badUpdatePropID, _ := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ev",
		ProposalType: "update",
		Title:        "Bad update",
		Reasoning:    "fail",
		PayloadJSON:  string(updateBadPayload),
	})
	if err := s.ApplyProposal(ctx, badUpdatePropID); err == nil {
		t.Fatalf("expected error updating summarized memory, got nil")
	}
}
