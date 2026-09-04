package search_test

import (
	"context"
	"strings"
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

func TestRecall_NearDupe(t *testing.T) {
	se, s := testSearcher(t)

	// 1. Seed first memory
	content := "production deploy guidelines to fly.io container infrastructure"
	id1 := seed(t, s, "global", "note", content, []string{"deploy", "infrastructure"})

	// 2. Backdate id1 to bypass PutMemory's 60s ingestion deduplication window
	past := time.Now().Add(-10 * time.Minute).UnixMicro()
	if _, err := s.DB().Exec("UPDATE memories SET updated_at = ?, created_at = ? WHERE id = ?", past, past, id1); err != nil {
		t.Fatalf("failed to backdate id1: %v", err)
	}

	// 3. Seed second memory with identical content
	id2 := seed(t, s, "global", "note", content, []string{"deploy", "infrastructure"})

	// 4. Seed a third, unique memory to verify top-N slot promotion
	id3 := seed(t, s, "global", "note", "production database replication setup postgres", []string{"database"})

	// Verify two distinct rows exist in DB with identical content_hash
	if id1 == id2 {
		t.Fatalf("expected distinct IDs, got id1 == id2 == %d", id1)
	}
	var hash1, hash2 string
	if err := s.DB().QueryRow("SELECT content_hash FROM memories WHERE id = ?", id1).Scan(&hash1); err != nil {
		t.Fatalf("scan hash1: %v", err)
	}
	if err := s.DB().QueryRow("SELECT content_hash FROM memories WHERE id = ?", id2).Scan(&hash2); err != nil {
		t.Fatalf("scan hash2: %v", err)
	}
	if hash1 != hash2 || hash1 == "" {
		t.Fatalf("expected identical non-empty content hashes, got %q vs %q", hash1, hash2)
	}

	// 5. Query with Top = 2.
	// Without near-dupe collapse, [id2, id1] would occupy both top-2 slots.
	// With near-dupe collapse, id1 is collapsed, allowing id3 to fill the 2nd slot.
	res, err := se.Recall(context.Background(), search.Query{
		Text: "production deploy", Top: 2, Scope: "global", Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("expected 2 results after collapsing near-duplicates, got %d", len(res))
	}

	var winnerID, loserID int64
	if res[0].ID == id1 {
		winnerID = id1
		loserID = id2
	} else if res[0].ID == id2 {
		winnerID = id2
		loserID = id1
	} else {
		t.Fatalf("expected one of duplicates (id1=%d, id2=%d) first, got ID %d", id1, id2, res[0].ID)
	}
	_ = winnerID

	if res[1].ID != id3 {
		t.Errorf("expected unique memory id3 (%d) second, got ID %d", id3, res[1].ID)
	}

	// Verify neither result is the collapsed duplicate
	for _, r := range res {
		if r.ID == loserID {
			t.Errorf("collapsed duplicate memory (%d) should not be present in results", loserID)
		}
	}

	// Verify ContentHash is populated on the result
	if res[0].ContentHash != hash1 {
		t.Errorf("expected ContentHash %q, got %q", hash1, res[0].ContentHash)
	}
}

func TestRecall_DecayOrdering(t *testing.T) {
	se, s := testSearcher(t)

	// 1. Seed an older memory with slightly higher keyword term frequency (2 matches)
	oldID := seed(t, s, "global", "note", "kubernetes autoscaling guidelines kubernetes autoscaling", []string{"k8s"})

	// 2. Seed a newer memory with standard keyword term frequency (1 match)
	newID := seed(t, s, "global", "note", "kubernetes autoscaling deployment guidelines", []string{"k8s"})

	// 3. Backdate old memory by 60 days (4 half-lives if half-life = 15 days)
	pastTime := time.Now().Add(-60 * 24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec("UPDATE memories SET created_at = ? WHERE id = ?", pastTime, oldID); err != nil {
		t.Fatalf("failed to backdate old memory: %v", err)
	}

	ctx := context.Background()
	q := search.Query{
		Text: "kubernetes autoscaling guidelines", Top: 5, Scope: "global", Inherit: true,
	}

	// 4. Test without recency decay (decay = 0)
	se = se.WithDecayDays(0)
	resNoDecay, err := se.Recall(ctx, q)
	if err != nil {
		t.Fatalf("Recall without decay: %v", err)
	}
	if len(resNoDecay) < 2 {
		t.Fatalf("expected at least 2 results without decay, got %d", len(resNoDecay))
	}

	// 5. Test with recency decay enabled: half-life = 15 days.
	// Age of old memory = 60 days (4 half-lives -> score * 0.5^4 = 0.0625)
	// Age of new memory = 0 days (0 half-lives -> score * 1.0)
	se = se.WithDecayDays(15)
	resDecay, err := se.Recall(ctx, q)
	if err != nil {
		t.Fatalf("Recall with decay: %v", err)
	}
	if len(resDecay) < 2 {
		t.Fatalf("expected at least 2 results with decay, got %d", len(resDecay))
	}

	// With decay active, the recent memory must rank #1
	if resDecay[0].ID != newID {
		t.Errorf("expected new memory (%d) first with decay, got ID %d (score %f vs %f)",
			newID, resDecay[0].ID, resDecay[0].Score, resDecay[1].Score)
	}
	if resDecay[1].ID != oldID {
		t.Errorf("expected old memory (%d) second with decay, got ID %d", oldID, resDecay[1].ID)
	}

	// The decayed old score should be significantly lower than the recent score
	if resDecay[0].Score < resDecay[1].Score*3.0 {
		t.Errorf("expected recent memory score (%f) to substantially exceed decayed old score (%f)",
			resDecay[0].Score, resDecay[1].Score)
	}
}

func TestEnrichQuery(t *testing.T) {
	se, s := testSearcher(t)

	// Seed memories with various tags
	seed(t, s, "global", "note", "golang concurrency patterns channel", []string{"go", "concurrency", "channel", "backend"})
	seed(t, s, "global", "note", "golang concurrency goroutine pool", []string{"go", "concurrency", "goroutine", "performance"})
	seed(t, s, "global", "note", "golang memory allocation gc", []string{"go", "memory", "gc"})
	seed(t, s, "global", "note", "python web framework django", []string{"python", "django"})

	ctx := context.Background()

	t.Run("basic tag enrichment appends top tags", func(t *testing.T) {
		q := search.Query{Text: "concurrency", Scope: "global", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}

		// Original query text preserved as prefix
		if !strings.HasPrefix(enriched.Text, "concurrency") {
			t.Errorf("expected enriched text to start with original query, got %q", enriched.Text)
		}

		// "go" appears in both concurrency memories and should be present
		if !strings.Contains(enriched.Text, "go") {
			t.Errorf("expected enriched text to contain tag 'go', got %q", enriched.Text)
		}

		// Tag already in query ("concurrency") should not be duplicated
		tokens := strings.Fields(enriched.Text)
		countConcurrency := 0
		for _, tok := range tokens {
			if strings.EqualFold(tok, "concurrency") {
				countConcurrency++
			}
		}
		if countConcurrency != 1 {
			t.Errorf("expected 'concurrency' to appear exactly once, appeared %d times in %q", countConcurrency, enriched.Text)
		}

		// At most 3 tags appended -> max 4 tokens total
		if len(tokens) > 4 {
			t.Errorf("expected at most 4 tokens (1 term + 3 tags), got %d in %q", len(tokens), enriched.Text)
		}
	})

	t.Run("empty query text returns unchanged", func(t *testing.T) {
		q := search.Query{Text: "", Scope: "global"}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if enriched.Text != "" {
			t.Errorf("expected empty text, got %q", enriched.Text)
		}
	})

	t.Run("no matching memories returns unchanged", func(t *testing.T) {
		q := search.Query{Text: "nonexistentunmatchedterm", Scope: "global"}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if enriched.Text != "nonexistentunmatchedterm" {
			t.Errorf("expected query text unchanged, got %q", enriched.Text)
		}
	})

	t.Run("matches with no tags returns unchanged", func(t *testing.T) {
		seed(t, s, "global", "note", "uniquetermwithouttags", nil)
		q := search.Query{Text: "uniquetermwithouttags", Scope: "global"}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if enriched.Text != "uniquetermwithouttags" {
			t.Errorf("expected query text unchanged when memories have no tags, got %q", enriched.Text)
		}
	})

	t.Run("scope filter restricts tags", func(t *testing.T) {
		seed(t, s, "project:isolated", "note", "classified secret algorithm details", []string{"classified", "secret"})
		// Query in a different scope with Inherit=false should not pick up tags from project:isolated
		q := search.Query{Text: "algorithm details", Scope: "project:other", Inherit: false}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if strings.Contains(enriched.Text, "classified") || strings.Contains(enriched.Text, "secret") {
			t.Errorf("expected scope filter to exclude tags from project:isolated, got %q", enriched.Text)
		}
	})
}

func TestRecall_EnrichQuery_FactsPrefixPreserved(t *testing.T) {
	ctx := context.Background()
	se, s := testSearcher(t)

	// Seed a fact with key "user.timezone"
	if _, _, err := s.SetFact(ctx, store.FactInput{
		Scope: "global",
		Key:   "user.timezone",
		Value: `"UTC"`,
	}); err != nil {
		t.Fatalf("SetFact failed: %v", err)
	}

	// Seed a note that keyword matches "user" and has tags "auth", "session"
	seed(t, s, "global", "note", "user authentication guidelines", []string{"auth", "session"})

	// Recall with query "user". EnrichQuery will expand Keyword and Semantic queries to "user auth session".
	// But Facts must receive unexpanded "user", allowing key LIKE "user%" to match "user.timezone".
	res, err := se.Recall(ctx, search.Query{Text: "user", Scope: "global", Inherit: true, Top: 5})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	var foundFact bool
	for _, r := range res {
		if r.Type == "fact" && strings.Contains(r.Content, "user.timezone") {
			foundFact = true
			break
		}
	}
	if !foundFact {
		t.Fatalf("Recall with expanded query missed prefix fact match; results: %+v", res)
	}
}
