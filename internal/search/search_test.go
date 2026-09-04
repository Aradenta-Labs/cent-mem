package search_test

import (
	"context"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func testSearcher(t *testing.T) (*search.Searcher, *store.Store) {
	t.Helper()
	cfg := config.Config{DBPath: t.TempDir() + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return search.New(s), s
}

func seed(t *testing.T, s *store.Store, scopePath, typ, content string, tags []string) int64 {
	t.Helper()
	id, _, err := s.PutMemory(context.Background(), store.MemoryInput{
		Scope:   scopePath,
		Type:    typ,
		Content: content,
		Tags:    tags,
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	return id
}

func TestKeywordRank_BM25(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "the quick brown fox jumps over the lazy dog", nil)
	seed(t, s, "global", "note", "fox fox fox fox", nil)

	ctx := context.Background()
	res, err := se.Keyword(ctx, search.Query{Text: "fox", Top: 5, Scope: "global", Inherit: true}, 15)
	if err != nil {
		t.Fatalf("Keyword: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one keyword result")
	}
	// Higher term frequency (4 foxes) should rank first.
	if res[0].Content != "fox fox fox fox" {
		t.Errorf("expected 'fox fox fox fox' first, got %q", res[0].Content)
	}
}

func TestFactsRank_Prefix(t *testing.T) {
	se, s := testSearcher(t)
	if _, _, err := s.SetFact(context.Background(), store.FactInput{
		Scope: "global", Key: "user.timezone", Value: `"Asia/Jakarta"`,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := se.Facts(context.Background(), search.Query{Text: "user", Top: 5, Scope: "global", Inherit: true}, 15)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(res))
	}
	if res[0].Type != "fact" {
		t.Errorf("type = %q, want fact", res[0].Type)
	}
}

func TestTimelineOrder(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "log", "first", nil)
	time.Sleep(10 * time.Millisecond)
	seed(t, s, "global", "log", "second", nil)

	res, err := se.Timeline(context.Background(), search.Query{Top: 5, Scope: "global", Inherit: true}, 15)
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if len(res) < 2 {
		t.Fatalf("expected >=2 timeline entries, got %d", len(res))
	}
	if res[0].Content != "second" {
		t.Errorf("expected most recent first, got %q", res[0].Content)
	}
}

func TestRecall_Dedup(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "we deploy via github actions to fly", nil)

	res, err := se.Recall(context.Background(), search.Query{Text: "github", Top: 5, Scope: "global", Inherit: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	// The single memory appears once even though timeline also matches.
	seen := map[int64]bool{}
	for _, r := range res {
		if seen[r.ID] {
			t.Errorf("duplicate id %d in results", r.ID)
		}
		seen[r.ID] = true
	}
}

func TestRecall_Filters(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "github actions deploy", []string{"ci", "deploy"})
	seed(t, s, "global", "log", "github raw log entry", []string{"log"})

	res, err := se.Recall(context.Background(), search.Query{
		Text: "github", Top: 5, Scope: "global", Inherit: true, Type: "note",
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	for _, r := range res {
		if r.Type != "note" {
			t.Errorf("expected only notes, got type %q", r.Type)
		}
	}
}

func TestRecall_InheritScope(t *testing.T) {
	se, s := testSearcher(t)
	// Memory in global scope.
	seed(t, s, "global", "note", "project convention: use tabs", []string{"convention"})

	// Query from project scope with inherit -> should see the global memory.
	res, err := se.Recall(context.Background(), search.Query{
		Text: "convention", Top: 5, Scope: "project:cent-mem", Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall inherit: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected inherited result from global, got none")
	}
	if res[0].Scope != "global" {
		t.Errorf("expected result scoped to global, got %q", res[0].Scope)
	}
}

func TestRecall_CandidatePool(t *testing.T) {
	// Candidate pool floor should be 50.
	// Since we can't easily spy on the exact `cand` passed, we can ensure that
	// if we insert 40 items that don't match, and 1 item that matches at the end,
	// that item is found, meaning `cand` was large enough.
	se, s := testSearcher(t)
	
	// Create 60 memories. The first 55 are noise. The 56th matches the query.
	for i := 0; i < 55; i++ {
		seed(t, s, "global", "note", "noise", nil)
	}
	seed(t, s, "global", "note", "needle in haystack", nil)
	
	res, err := se.Recall(context.Background(), search.Query{
		Text: "needle", Top: 5, Scope: "global", Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected to find needle in candidate pool")
	}
}

func TestKeyword_PrefixFallback(t *testing.T) {
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "architecture decisions", nil)

	ctx := context.Background()
	// "arch" should not match exact phrase "architecture" but will match via prefix fallback
	res, err := se.Keyword(ctx, search.Query{Text: "arch", Top: 5, Scope: "global", Inherit: true}, 15)
	if err != nil {
		t.Fatalf("Keyword: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one keyword result via prefix fallback")
	}
	if len(res[0].MatchedBy) == 0 || res[0].MatchedBy[0] != "keyword_prefix" {
		t.Errorf("expected matched_by keyword_prefix, got %v", res[0].MatchedBy)
	}
}

func TestRecall_WeightedRRF(t *testing.T) {
	se, s := testSearcher(t)
	// We want to verify timeline weight is 0.3 for text queries.
	// Insert an old memory that is highly relevant
	oldID := seed(t, s, "global", "note", "the absolute best architecture architecture", nil)
	// Make it older in DB
	s.DB().Exec("UPDATE memories SET created_at = ? WHERE id = ?", time.Now().Add(-24*time.Hour).UnixMicro(), oldID)
	
	// Insert a new memory that is barely relevant (only timeline will give it a high rank)
	seed(t, s, "global", "note", "recent meaningless update", nil)

	// Since timeline weight is 0.3, the highly relevant older one should score higher.
	res, err := se.Recall(context.Background(), search.Query{
		Text: "architecture", Top: 5, Scope: "global", Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected results")
	}
	// The most relevant result should be first
	if res[0].ID != oldID {
		t.Errorf("expected highly relevant older memory first, got ID %d with score %v", res[0].ID, res[0].Score)
	}
}
