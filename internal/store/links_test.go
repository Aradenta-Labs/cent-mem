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
	if v < "5" {
		t.Fatalf("expected schema_version >= '5', got %q", v)
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

func TestLinks_ListSuggestedLinks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	mGlobal, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "Global memory"})
	if err != nil {
		t.Fatalf("PutMemory mGlobal: %v", err)
	}
	mAlpha, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha", Type: "note", Content: "Alpha memory"})
	if err != nil {
		t.Fatalf("PutMemory mAlpha: %v", err)
	}
	mAlphaAgent, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha/agent:claude", Type: "note", Content: "Alpha agent memory"})
	if err != nil {
		t.Fatalf("PutMemory mAlphaAgent: %v", err)
	}
	mBeta, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:beta", Type: "note", Content: "Beta memory"})
	if err != nil {
		t.Fatalf("PutMemory mBeta: %v", err)
	}

	// Suggested links
	s1, err := s.CreateLink(ctx, mAlpha, mAlphaAgent, "supports", true)
	if err != nil {
		t.Fatalf("CreateLink s1: %v", err)
	}
	s2, err := s.CreateLink(ctx, mAlphaAgent, mGlobal, "refines", true)
	if err != nil {
		t.Fatalf("CreateLink s2: %v", err)
	}
	s3, err := s.CreateLink(ctx, mBeta, mGlobal, "depends-on", true)
	if err != nil {
		t.Fatalf("CreateLink s3: %v", err)
	}
	// Confirmed link (should never appear)
	if _, err := s.CreateLink(ctx, mAlpha, mBeta, "contradicts", false); err != nil {
		t.Fatalf("CreateLink confirmed: %v", err)
	}

	// 1. Unfiltered & "global" should return all 3 suggested links
	all, total, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{})
	if err != nil {
		t.Fatalf("ListSuggestedLinks all: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("expected total=3 len=3, got total=%d len=%d", total, len(all))
	}

	globalRes, globalTotal, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{ScopePath: "global"})
	if err != nil {
		t.Fatalf("ListSuggestedLinks global: %v", err)
	}
	if globalTotal != 3 || len(globalRes) != 3 {
		t.Fatalf("expected global total=3 len=3, got total=%d len=%d", globalTotal, len(globalRes))
	}

	// 2. Scope subtree filtering (project:alpha includes child project:alpha/agent:claude and cross-scope link to global, excludes project:beta-only)
	alphaRes, alphaTotal, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{ScopePath: "project:alpha"})
	if err != nil {
		t.Fatalf("ListSuggestedLinks alpha: %v", err)
	}
	if alphaTotal != 2 || len(alphaRes) != 2 {
		t.Fatalf("expected alpha total=2 len=2, got total=%d len=%d", alphaTotal, len(alphaRes))
	}
	seenIDs := map[int64]bool{}
	for _, item := range alphaRes {
		if seenIDs[item.ID] {
			t.Fatalf("duplicate link id %d in alphaRes", item.ID)
		}
		seenIDs[item.ID] = true
		if item.ID == s3.ID {
			t.Errorf("unexpected beta link %d in alphaRes", s3.ID)
		}
	}
	if !seenIDs[s1.ID] || !seenIDs[s2.ID] {
		t.Errorf("expected s1(%d) and s2(%d) in alphaRes, got %v", s1.ID, s2.ID, seenIDs)
	}

	// 3. Relation filtering & invalid relation error
	relRes, relTotal, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{Relation: "supports"})
	if err != nil {
		t.Fatalf("ListSuggestedLinks relation: %v", err)
	}
	if relTotal != 1 || len(relRes) != 1 || relRes[0].ID != s1.ID {
		t.Fatalf("expected only s1(%d), got total=%d items=%+v", s1.ID, relTotal, relRes)
	}
	if _, _, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{Relation: "bogus"}); err == nil {
		t.Errorf("expected error for invalid relation")
	}

	// 4. Unknown scope returns empty without error
	unknownRes, unknownTotal, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{ScopePath: "project:nonexistent"})
	if err != nil {
		t.Fatalf("ListSuggestedLinks nonexistent: %v", err)
	}
	if unknownTotal != 0 || len(unknownRes) != 0 {
		t.Fatalf("expected empty for unknown scope, got total=%d len=%d", unknownTotal, len(unknownRes))
	}

	// 5. Pagination preserves full total
	pageRes, pageTotal, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListSuggestedLinks page: %v", err)
	}
	if pageTotal != 3 || len(pageRes) != 1 {
		t.Fatalf("expected total=3 len=1, got total=%d len=%d", pageTotal, len(pageRes))
	}

	// 6. Confirm and dismiss remove items from suggested list
	if err := s.ConfirmLink(ctx, s1.ID); err != nil {
		t.Fatalf("ConfirmLink s1: %v", err)
	}
	if err := s.DismissLink(ctx, s2.ID); err != nil {
		t.Fatalf("DismissLink s2: %v", err)
	}
	afterRes, afterTotal, err := s.ListSuggestedLinks(ctx, store.SuggestedLinkListQuery{})
	if err != nil {
		t.Fatalf("ListSuggestedLinks after confirm/dismiss: %v", err)
	}
	if afterTotal != 1 || len(afterRes) != 1 || afterRes[0].ID != s3.ID {
		t.Fatalf("expected only s3(%d) remaining, got total=%d items=%+v", s3.ID, afterTotal, afterRes)
	}
}

func TestLinks_GetGraph(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	mGlobal, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "fact", Key: "env", Content: "env = prod", Tags: []string{"infra"}})
	if err != nil {
		t.Fatalf("PutMemory mGlobal: %v", err)
	}
	mAlpha1, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha", Type: "note", Content: "Alpha API design"})
	if err != nil {
		t.Fatalf("PutMemory mAlpha1: %v", err)
	}
	mAlphaAgent, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha/agent:claude", Type: "log", Content: "Alpha agent trace"})
	if err != nil {
		t.Fatalf("PutMemory mAlphaAgent: %v", err)
	}
	mAlphaIso, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha", Type: "note", Content: "Alpha isolated note"})
	if err != nil {
		t.Fatalf("PutMemory mAlphaIso: %v", err)
	}
	mBeta, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:beta", Type: "note", Content: "Beta unrelated note"})
	if err != nil {
		t.Fatalf("PutMemory mBeta: %v", err)
	}

	if _, err := s.CreateLink(ctx, mAlpha1, mAlphaAgent, "supports", false); err != nil {
		t.Fatalf("CreateLink confirmed: %v", err)
	}
	if _, err := s.CreateLink(ctx, mAlphaAgent, mGlobal, "depends-on", true); err != nil {
		t.Fatalf("CreateLink suggested cross-scope: %v", err)
	}

	// 1. All scopes with suggested links
	resAll, err := s.GetGraph(ctx, store.GraphQuery{IncludeSuggested: true})
	if err != nil {
		t.Fatalf("GetGraph all: %v", err)
	}
	if resAll.TotalNodes != 5 || len(resAll.Nodes) != 5 || len(resAll.Edges) != 2 || resAll.Truncated {
		t.Fatalf("unexpected all graph: total=%d nodes=%d edges=%d truncated=%v", resAll.TotalNodes, len(resAll.Nodes), len(resAll.Edges), resAll.Truncated)
	}

	// 2. Scoped query (project:alpha) includes child scope + external mGlobal endpoint, excludes mBeta
	resAlpha, err := s.GetGraph(ctx, store.GraphQuery{ScopePath: "project:alpha", IncludeSuggested: true})
	if err != nil {
		t.Fatalf("GetGraph alpha: %v", err)
	}
	if resAlpha.TotalNodes != 3 {
		t.Errorf("expected TotalNodes=3 in project:alpha subtree, got %d", resAlpha.TotalNodes)
	}
	if len(resAlpha.Nodes) != 4 || len(resAlpha.Edges) != 2 {
		t.Fatalf("expected 4 nodes (3 in-scope + 1 external global) and 2 edges, got nodes=%d edges=%d", len(resAlpha.Nodes), len(resAlpha.Edges))
	}
	foundGlobalExternal := false
	for _, n := range resAlpha.Nodes {
		if n.ID == mBeta {
			t.Errorf("unexpected beta node in alpha graph")
		}
		if n.ID == mGlobal {
			foundGlobalExternal = n.External
		}
	}
	if !foundGlobalExternal {
		t.Errorf("expected mGlobal to be included with External=true")
	}

	// 3. IncludeSuggested=false omits suggested edge and therefore omits external mGlobal
	resConfirmed, err := s.GetGraph(ctx, store.GraphQuery{ScopePath: "project:alpha", IncludeSuggested: false})
	if err != nil {
		t.Fatalf("GetGraph confirmed only: %v", err)
	}
	if len(resConfirmed.Edges) != 1 || len(resConfirmed.Nodes) != 3 {
		t.Fatalf("expected 3 nodes and 1 confirmed edge, got nodes=%d edges=%d", len(resConfirmed.Nodes), len(resConfirmed.Edges))
	}

	// 4. Limit prioritization keeps connected nodes ahead of isolated nodes
	resCapped, err := s.GetGraph(ctx, store.GraphQuery{ScopePath: "project:alpha", IncludeSuggested: false, Limit: 2})
	if err != nil {
		t.Fatalf("GetGraph capped: %v", err)
	}
	if !resCapped.Truncated || resCapped.TotalNodes != 3 || len(resCapped.Nodes) != 2 {
		t.Fatalf("expected Truncated=true TotalNodes=3 len=2, got %+v", resCapped)
	}
	for _, n := range resCapped.Nodes {
		if n.ID == mAlphaIso {
			t.Errorf("expected isolated node %d to be excluded when limit=2 prioritizes connected nodes", mAlphaIso)
		}
	}

	// 5. Unknown scope returns empty cleanly
	resEmpty, err := s.GetGraph(ctx, store.GraphQuery{ScopePath: "project:doesnotexist", IncludeSuggested: true})
	if err != nil {
		t.Fatalf("GetGraph unknown scope: %v", err)
	}
	if len(resEmpty.Nodes) != 0 || len(resEmpty.Edges) != 0 || resEmpty.TotalNodes != 0 {
		t.Fatalf("expected empty graph, got %+v", resEmpty)
	}
}
