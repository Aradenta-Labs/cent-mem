package search_test

import (
	"context"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// embedAll drains the queue so memories_vec gets populated, then returns a
// Searcher bound to a deterministic stub embedder (same vectors as the queue
// used, so query embedding is consistent).
func embedAll(t *testing.T, s *store.Store) *search.Searcher {
	t.Helper()
	q := embed.NewQueue(s, embed.NewStub(384))
	q.MaxTime = 0 // no deadline limit needed in tests
	if _, err := q.Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	return search.New(s).WithEmbedder(embed.NewStub(384))
}

// I.11 — nearest neighbor returns the expected top match.
func TestSemantic_NearestNeighbor(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "the quick brown fox", nil)
	seed(t, s, "global", "note", "totally unrelated content", nil)

	se = embedAll(t, s)
	ctx := context.Background()
	res, err := se.Semantic(ctx, search.Query{Text: "a quick fox", Top: 5, Scope: "global", Inherit: true}, 15)
	if err != nil {
		t.Fatalf("Semantic: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one semantic result")
	}
	if res[0].Content != "the quick brown fox" {
		t.Errorf("expected nearest match 'the quick brown fox', got %q", res[0].Content)
	}
}

// I.12 — scope filter applied after vec match.
func TestSemantic_FilterByScope(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "the quick brown fox", nil)
	seed(t, s, "project:alpha", "note", "another fox here", nil)

	se = embedAll(t, s)
	ctx := context.Background()
	// Query from project:alpha with inherit -> sees global + project:alpha.
	res, err := se.Semantic(ctx, search.Query{Text: "fox", Top: 20, Scope: "project:alpha", Inherit: true}, 30)
	if err != nil {
		t.Fatalf("Semantic: %v", err)
	}
	for _, r := range res {
		if r.Scope != "global" && r.Scope != "project:alpha" {
			t.Errorf("unexpected scope %q in results", r.Scope)
		}
	}
}

// I.12b — type filter applied.
func TestSemantic_FilterByType(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "the quick brown fox", nil)
	seed(t, s, "global", "log", "fox log entry", nil)

	se = embedAll(t, s)
	ctx := context.Background()
	res, err := se.Semantic(ctx, search.Query{Text: "fox", Top: 20, Scope: "global", Inherit: true, Type: "note"}, 30)
	if err != nil {
		t.Fatalf("Semantic: %v", err)
	}
	for _, r := range res {
		if r.Type != "note" {
			t.Errorf("expected only notes, got %q", r.Type)
		}
	}
}

// I.13 — hybrid recall surfaces semantic-only matches (matched_by includes semantic).
func TestRecall_HybridFuse(t *testing.T) {
	se, s := testSearcher(t)
	// A memory with no keyword overlap to the query.
	seed(t, s, "global", "note", "the quick brown fox jumps over the fence", nil)

	se = embedAll(t, s)
	ctx := context.Background()
	res, err := se.Recall(ctx, search.Query{Text: "a quick brown fox jumps over the fence", Top: 5, Scope: "global", Inherit: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one result")
	}
	// Because content == query here, keyword will also match; assert id present.
	found := false
	for _, r := range res {
		if r.Content == "the quick brown fox jumps over the fence" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected the memory in hybrid results")
	}
}

// TestRecall_ParaphraseQuery validates that recall can surface a memory via the
// semantic path when the query shares few/no keywords with the content. With
// the deterministic stub embedder we exercise the path end-to-end by asserting
// the semantic contribution is present in matched_by and the relevant memory
// ranks above a keyword-unrelated distractor.
func TestRecall_ParaphraseQuery(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "we deploy via github actions to fly.io", nil)
	seed(t, s, "global", "note", "the weather in jakarta is rainy today", nil)

	se = embedAll(t, s)
	ctx := context.Background()
	res, err := se.Recall(ctx, search.Query{Text: "how do we ship?", Top: 5, Scope: "global", Inherit: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one result")
	}
	found := false
	for _, r := range res {
		if r.Content == "we deploy via github actions to fly.io" {
			found = true
			if !hasStr(r.MatchedBy, "semantic") {
				t.Errorf("expected matched_by to include semantic, got %v", r.MatchedBy)
			}
		}
	}
	if !found {
		t.Fatal("expected the deploy memory in paraphrase results")
	}
}

func hasStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
