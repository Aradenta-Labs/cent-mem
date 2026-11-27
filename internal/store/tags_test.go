package store_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestListTagStats_AndScopeIsolation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha",
		Type:    "note",
		Content: "Use SQLite for local persistence",
		Tags:    []string{"database", "sqlite"},
	})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha/agent:claude",
		Type:    "note",
		Content: "WAL mode enabled",
		Tags:    []string{"database", "db"},
	})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}

	mArchived, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:alpha",
		Type:    "note",
		Content: "Old archived note",
		Tags:    []string{"archived-only"},
	})
	if err != nil {
		t.Fatalf("put mArchived: %v", err)
	}
	if err := s.ArchiveMemory(ctx, mArchived); err != nil {
		t.Fatalf("archive: %v", err)
	}

	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:beta",
		Type:    "note",
		Content: "Other project memory",
		Tags:    []string{"beta-only"},
	})
	if err != nil {
		t.Fatalf("put beta: %v", err)
	}

	// Empty non-nil scopeIDs -> empty slice.
	emptyStats, err := s.ListTagStats(ctx, []int64{})
	if err != nil {
		t.Fatalf("ListTagStats empty: %v", err)
	}
	if len(emptyStats) != 0 {
		t.Fatalf("expected 0 stats for empty scopeIDs, got %d", len(emptyStats))
	}

	// Project alpha subtree includes child agent scope, excludes archived and beta.
	sc, err := scope.Parse("project:alpha")
	if err != nil {
		t.Fatalf("parse scope: %v", err)
	}
	scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
	if err != nil {
		t.Fatalf("resolve scopes: %v", err)
	}
	stats, err := s.ListTagStats(ctx, scopeIDs)
	if err != nil {
		t.Fatalf("ListTagStats alpha: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("expected 3 tags in project:alpha subtree, got %d: %+v", len(stats), stats)
	}
	if stats[0].Tag != "database" || stats[0].Count != 2 {
		t.Errorf("stats[0] = %+v, want tag=database count=2", stats[0])
	}
	if stats[1].Tag != "db" || stats[1].Count != 1 {
		t.Errorf("stats[1] = %+v, want tag=db count=1", stats[1])
	}
	if stats[2].Tag != "sqlite" || stats[2].Count != 1 {
		t.Errorf("stats[2] = %+v, want tag=sqlite count=1", stats[2])
	}
	if stats[0].LastUsed.IsZero() {
		t.Error("expected non-zero LastUsed timestamp")
	}

	// nil scopeIDs returns all active tags across projects.
	allStats, err := s.ListTagStats(ctx, nil)
	if err != nil {
		t.Fatalf("ListTagStats nil: %v", err)
	}
	if len(allStats) != 4 {
		t.Fatalf("expected 4 tags across all scopes, got %d: %+v", len(allStats), allStats)
	}
}

func TestBulkRenameTag_DeduplicatesAndPreservesUpdatedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:tags",
		Type:    "note",
		Content: "Memory with both alias and canonical",
		Tags:    []string{"db", "database", "storage"},
	})
	if err != nil {
		t.Fatalf("put id1: %v", err)
	}
	id2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:tags/agent:worker",
		Type:    "note",
		Content: "Memory with sqlite alias",
		Tags:    []string{"sqlite", "backend"},
	})
	if err != nil {
		t.Fatalf("put id2: %v", err)
	}
	idOther, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:other",
		Type:    "note",
		Content: "Unrelated project db tag",
		Tags:    []string{"db"},
	})
	if err != nil {
		t.Fatalf("put idOther: %v", err)
	}

	before1, _ := s.GetMemory(ctx, id1)
	before2, _ := s.GetMemory(ctx, id2)
	time.Sleep(5 * time.Millisecond)

	sc, _ := scope.Parse("project:tags")
	scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
	if err != nil {
		t.Fatalf("resolve scopes: %v", err)
	}

	count, err := s.CountMemoriesWithAnyTag(ctx, scopeIDs, []string{"db", "sqlite"}, "database")
	if err != nil {
		t.Fatalf("CountMemoriesWithAnyTag: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountMemoriesWithAnyTag = %d, want 2", count)
	}

	affected, err := s.BulkRenameTag(ctx, scopeIDs, []string{"db", "sqlite"}, "database")
	if err != nil {
		t.Fatalf("BulkRenameTag: %v", err)
	}
	if affected != 2 {
		t.Fatalf("affected = %d, want 2", affected)
	}

	after1, _ := s.GetMemory(ctx, id1)
	after2, _ := s.GetMemory(ctx, id2)
	afterOther, _ := s.GetMemory(ctx, idOther)

	if !slices.Equal(after1.Tags, []string{"database", "storage"}) {
		t.Errorf("id1 tags = %v, want [database storage]", after1.Tags)
	}
	if !slices.Equal(after2.Tags, []string{"backend", "database"}) {
		t.Errorf("id2 tags = %v, want [backend database]", after2.Tags)
	}
	if !slices.Equal(afterOther.Tags, []string{"db"}) {
		t.Errorf("other project tags modified: %v", afterOther.Tags)
	}

	// updated_at must be preserved.
	if !after1.UpdatedAt.Equal(before1.UpdatedAt) {
		t.Errorf("id1 updated_at changed: before=%v after=%v", before1.UpdatedAt, after1.UpdatedAt)
	}
	if !after2.UpdatedAt.Equal(before2.UpdatedAt) {
		t.Errorf("id2 updated_at changed: before=%v after=%v", before2.UpdatedAt, after2.UpdatedAt)
	}

	// Re-running is idempotent (0 affected).
	affected2, err := s.BulkRenameTag(ctx, scopeIDs, []string{"db", "sqlite"}, "database")
	if err != nil {
		t.Fatalf("second BulkRenameTag: %v", err)
	}
	if affected2 != 0 {
		t.Errorf("second BulkRenameTag affected = %d, want 0", affected2)
	}

	// Validation errors.
	if _, err := s.BulkRenameTag(ctx, scopeIDs, []string{"db"}, ""); err == nil {
		t.Error("expected error for empty target tag")
	}
	if _, err := s.BulkRenameTag(ctx, scopeIDs, []string{"db"}, "a,b"); err == nil {
		t.Error("expected error for target tag with comma")
	}
}

func TestApplyProposal_TagMerge(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:prop",
		Type:    "note",
		Content: "Auth middleware design",
		Tags:    []string{"authn"},
	})
	if err != nil {
		t.Fatalf("put id1: %v", err)
	}
	id2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:prop/agent:dev",
		Type:    "note",
		Content: "OAuth token flow",
		Tags:    []string{"authentication", "security"},
	})
	if err != nil {
		t.Fatalf("put id2: %v", err)
	}

	before1, _ := s.GetMemory(ctx, id1)
	time.Sleep(5 * time.Millisecond)

	payloadBytes, _ := json.Marshal(store.TagMergeProposalPayload{
		Kind:          "tag_merge",
		FromTags:      []string{"authn", "authentication"},
		ToTag:         "auth",
		AffectedCount: 2,
	})
	propID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:prop",
		ProposalType: "update",
		Title:        "Merge tags authn, authentication → auth",
		Reasoning:    "Tag cluster detected",
		PayloadJSON:  string(payloadBytes),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	if err := s.ApplyProposal(ctx, propID); err != nil {
		t.Fatalf("ApplyProposal tag_merge: %v", err)
	}

	prop, err := s.GetProposal(ctx, propID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if prop.Status != "applied" || prop.AppliedAt == nil {
		t.Errorf("proposal status = %q applied_at = %v, want applied", prop.Status, prop.AppliedAt)
	}

	after1, _ := s.GetMemory(ctx, id1)
	after2, _ := s.GetMemory(ctx, id2)
	if !slices.Equal(after1.Tags, []string{"auth"}) {
		t.Errorf("id1 tags = %v, want [auth]", after1.Tags)
	}
	if !slices.Equal(after2.Tags, []string{"auth", "security"}) {
		t.Errorf("id2 tags = %v, want [auth security]", after2.Tags)
	}
	if !after1.UpdatedAt.Equal(before1.UpdatedAt) {
		t.Errorf("id1 updated_at changed: before=%v after=%v", before1.UpdatedAt, after1.UpdatedAt)
	}

	// Applying again must conflict (already applied).
	if err := s.ApplyProposal(ctx, propID); err == nil {
		t.Error("expected conflict error re-applying already applied tag_merge proposal")
	}
}

func TestBulkRenameTag_CaseVariantNormalization(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:case-tags",
		Type:    "note",
		Content: "Legacy uppercase tag row",
		Tags:    []string{"database", "go"},
	})
	if err != nil {
		t.Fatalf("put id1: %v", err)
	}
	// Simulate legacy/imported mixed-case tag stored directly in SQLite.
	if _, err := s.DB().ExecContext(ctx, `UPDATE memories SET tags = 'Database,go' WHERE id = ?`, id1); err != nil {
		t.Fatalf("inject mixed-case tag: %v", err)
	}
	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:case-tags",
		Type:    "note",
		Content: "Already lowercase tag row",
		Tags:    []string{"database"},
	})
	if err != nil {
		t.Fatalf("put id2: %v", err)
	}

	before1, _ := s.GetMemory(ctx, id1)
	time.Sleep(5 * time.Millisecond)

	sc, _ := scope.Parse("project:case-tags")
	scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
	if err != nil {
		t.Fatalf("resolve scopes: %v", err)
	}

	cnt, err := s.CountMemoriesWithAnyTag(ctx, scopeIDs, []string{"Database"}, "database")
	if err != nil {
		t.Fatalf("CountMemoriesWithAnyTag: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("CountMemoriesWithAnyTag = %d, want 1", cnt)
	}

	affected, err := s.BulkRenameTag(ctx, scopeIDs, []string{"Database"}, "database")
	if err != nil {
		t.Fatalf("BulkRenameTag: %v", err)
	}
	if affected != 1 {
		t.Fatalf("BulkRenameTag affected = %d, want 1", affected)
	}

	after1, _ := s.GetMemory(ctx, id1)
	if !slices.Equal(after1.Tags, []string{"database", "go"}) {
		t.Errorf("after1 tags = %v, want [database go]", after1.Tags)
	}
	if !after1.UpdatedAt.Equal(before1.UpdatedAt) {
		t.Errorf("updated_at changed: before=%v after=%v", before1.UpdatedAt, after1.UpdatedAt)
	}
}
