package search

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func testSearcher(t *testing.T) (*Searcher, *store.Store) {
	t.Helper()
	cfg := config.Config{DBPath: t.TempDir() + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s), s
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

func seedDupe(t *testing.T, s *store.Store, scopePath, typ, content string, tags []string) int64 {
	t.Helper()
	// Backdate existing memories so the 60-second store-level dedup does not merge
	_, _ = s.DB().Exec("UPDATE memories SET updated_at = updated_at - 1000000000")
	id, _, err := s.PutMemory(context.Background(), store.MemoryInput{
		Scope:   scopePath,
		Type:    typ,
		Content: content,
		Tags:    tags,
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	_, _ = s.DB().Exec("UPDATE memories SET updated_at = updated_at - 1000000000 WHERE id = ?", id)
	return id
}

// ============================================================================
// Group 1: collapseNearDupes Edge Cases
// ============================================================================

func TestAdversarial_CollapseNearDupes_EmptyHash(t *testing.T) {
	// Edge Case 1: Entries with empty ContentHash must NEVER be collapsed
	// against each other or against non-empty hashes.

	now := time.Now()
	inputs := []*Ranked{
		{ID: 1, Content: "Empty hash 1", ContentHash: "", Score: 0.90, CreatedAt: now},
		{ID: 2, Content: "Non-empty hash A", ContentHash: "hash_a", Score: 0.85, CreatedAt: now},
		{ID: 3, Content: "Empty hash 2", ContentHash: "", Score: 0.80, CreatedAt: now},
		{ID: 4, Content: "Duplicate hash A", ContentHash: "hash_a", Score: 0.75, CreatedAt: now},
		{ID: 5, Content: "Empty hash 3", ContentHash: "", Score: 0.70, CreatedAt: now},
		{ID: 6, Content: "Empty hash 4", ContentHash: "", Score: 0.60, CreatedAt: now},
	}

	collapsed := collapseNearDupes(inputs)

	// Expected: ID 1, ID 2, ID 3, ID 5, ID 6 (ID 4 collapsed because hash_a was seen in ID 2)
	expectedIDs := []int64{1, 2, 3, 5, 6}
	if len(collapsed) != len(expectedIDs) {
		t.Fatalf("expected %d results, got %d", len(expectedIDs), len(collapsed))
	}
	for i, exp := range expectedIDs {
		if collapsed[i].ID != exp {
			t.Errorf("at index %d: expected ID %d, got ID %d", i, exp, collapsed[i].ID)
		}
	}
}

func TestAdversarial_CollapseNearDupes_MultiClusters(t *testing.T) {
	// Edge Case 2: Multi-duplicate clusters interleaved across scores
	// Cluster alpha: 4 items (hash_alpha)
	// Cluster beta: 3 items (hash_beta)
	// Cluster gamma: 2 items (hash_gamma)
	// Uniques: hash_delta, hash_epsilon
	// Empty hashes: 2 items

	now := time.Now()
	inputs := []*Ranked{
		{ID: 101, ContentHash: "hash_alpha", Score: 0.95, CreatedAt: now},
		{ID: 102, ContentHash: "", Score: 0.92, CreatedAt: now},
		{ID: 103, ContentHash: "hash_beta", Score: 0.90, CreatedAt: now},
		{ID: 104, ContentHash: "hash_gamma", Score: 0.85, CreatedAt: now},
		{ID: 105, ContentHash: "hash_alpha", Score: 0.80, CreatedAt: now}, // duplicate alpha -> drop
		{ID: 106, ContentHash: "hash_delta", Score: 0.75, CreatedAt: now},
		{ID: 107, ContentHash: "hash_beta", Score: 0.70, CreatedAt: now},  // duplicate beta -> drop
		{ID: 108, ContentHash: "", Score: 0.68, CreatedAt: now},
		{ID: 109, ContentHash: "hash_gamma", Score: 0.65, CreatedAt: now}, // duplicate gamma -> drop
		{ID: 110, ContentHash: "hash_alpha", Score: 0.60, CreatedAt: now}, // duplicate alpha -> drop
		{ID: 111, ContentHash: "hash_epsilon", Score: 0.55, CreatedAt: now},
		{ID: 112, ContentHash: "hash_beta", Score: 0.50, CreatedAt: now},  // duplicate beta -> drop
		{ID: 113, ContentHash: "hash_alpha", Score: 0.40, CreatedAt: now}, // duplicate alpha -> drop
	}

	collapsed := collapseNearDupes(inputs)

	expectedIDs := []int64{101, 102, 103, 104, 106, 108, 111}
	if len(collapsed) != len(expectedIDs) {
		t.Fatalf("expected %d survivors, got %d", len(expectedIDs), len(collapsed))
	}
	for i, exp := range expectedIDs {
		if collapsed[i].ID != exp {
			t.Errorf("at index %d: expected ID %d, got ID %d", i, exp, collapsed[i].ID)
		}
	}
}

func TestAdversarial_CollapseNearDupes_TieBreaking(t *testing.T) {
	// Edge Case 3: Score tie-breaking
	t.Run("same score different CreatedAt: newer wins", func(t *testing.T) {
		tOld := time.Now().Add(-2 * time.Hour)
		tNew := time.Now()

		items := []*Ranked{
			{ID: 1, ContentHash: "h1", Score: 0.8, CreatedAt: tNew},
			{ID: 2, ContentHash: "h1", Score: 0.8, CreatedAt: tOld},
		}
		slices.SortFunc(items, func(a, b *Ranked) int {
			if b.Score > a.Score {
				return 1
			}
			if b.Score < a.Score {
				return -1
			}
			if b.CreatedAt.After(a.CreatedAt) {
				return 1
			}
			if a.CreatedAt.After(b.CreatedAt) {
				return -1
			}
			if b.ID > a.ID {
				return 1
			}
			if a.ID > b.ID {
				return -1
			}
			return 0
		})

		collapsed := collapseNearDupes(items)
		if len(collapsed) != 1 {
			t.Fatalf("expected 1 result, got %d", len(collapsed))
		}
		if collapsed[0].ID != 1 {
			t.Errorf("expected newer ID 1 to win, got ID %d", collapsed[0].ID)
		}
	})

	t.Run("same score same CreatedAt: higher ID wins", func(t *testing.T) {
		tSame := time.Now()
		items := []*Ranked{
			{ID: 10, ContentHash: "h1", Score: 0.8, CreatedAt: tSame},
			{ID: 20, ContentHash: "h1", Score: 0.8, CreatedAt: tSame},
		}
		slices.SortFunc(items, func(a, b *Ranked) int {
			if b.Score > a.Score {
				return 1
			}
			if b.Score < a.Score {
				return -1
			}
			if b.CreatedAt.After(a.CreatedAt) {
				return 1
			}
			if a.CreatedAt.After(b.CreatedAt) {
				return -1
			}
			if b.ID > a.ID {
				return 1
			}
			if a.ID > b.ID {
				return -1
			}
			return 0
		})

		collapsed := collapseNearDupes(items)
		if len(collapsed) != 1 {
			t.Fatalf("expected 1 result, got %d", len(collapsed))
		}
		if collapsed[0].ID != 20 {
			t.Errorf("expected higher ID 20 to win, got ID %d", collapsed[0].ID)
		}
	})
}

func TestAdversarial_CollapseNearDupes_TopNPromotion_EndToEnd(t *testing.T) {
	// Edge Case 4: Top-N Promotion in Recall
	se, s := testSearcher(t)

	// Seed 6 duplicate memories with exact same content (high match score)
	dupeContent := "identical cluster deployment manifest for ingress controller"
	var dupeIDs []int64
	for i := 0; i < 6; i++ {
		id := seedDupe(t, s, "global", "note", dupeContent, []string{"k8s", "ingress"})
		dupeIDs = append(dupeIDs, id)
	}

	// Verify that 6 distinct memory rows were created
	uniqueCheck := make(map[int64]bool)
	for _, id := range dupeIDs {
		uniqueCheck[id] = true
	}
	if len(uniqueCheck) != 6 {
		t.Fatalf("expected 6 distinct duplicate IDs seeded, got %d", len(uniqueCheck))
	}

	// Seed 4 unique memories with distinct content (slightly lower match score)
	uniqueContents := []string{
		"cluster deployment ingress service mesh",
		"cluster deployment ingress canary release",
		"cluster deployment ingress tls cert manager",
		"cluster deployment ingress rate limiter envoy",
	}
	for _, uc := range uniqueContents {
		seed(t, s, "global", "note", uc, []string{"k8s"})
	}

	ctx := context.Background()
	// Ask for Top = 4.
	// Without near-dupe collapse, all 4 slots would be consumed by dupeIDs[0..3].
	// With collapseNearDupes, only 1 dupe is retained, and 3 unique memories must be promoted!
	res, err := se.Recall(ctx, Query{
		Text: "cluster deployment ingress controller", Top: 4, Scope: "global", Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	if len(res) != 4 {
		t.Fatalf("expected 4 results, got %d", len(res))
	}

	// First result must be one of the dupeIDs
	var foundDupe bool
	for _, dID := range dupeIDs {
		if res[0].ID == dID {
			foundDupe = true
			break
		}
	}
	if !foundDupe {
		t.Errorf("expected top result to be one of the dupes, got ID %d", res[0].ID)
	}

	// Remaining 3 results must be unique memories, NONE of them can be another dupe
	for i := 1; i < 4; i++ {
		rID := res[i].ID
		for _, dID := range dupeIDs {
			if rID == dID {
				t.Errorf("result slot %d contains duplicate ID %d which should have been collapsed", i, rID)
			}
		}
	}
}

func TestAdversarial_CollapseNearDupes_AllDuplicates(t *testing.T) {
	// Edge Case 5: When ALL candidate results are duplicates of each other
	se, s := testSearcher(t)

	dupeContent := "identical solitary configuration content"
	for i := 0; i < 10; i++ {
		seedDupe(t, s, "global", "note", dupeContent, []string{"config"})
	}

	ctx := context.Background()
	res, err := se.Recall(ctx, Query{
		Text: "solitary configuration content", Top: 5, Scope: "global", Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	if len(res) != 1 {
		t.Errorf("expected exactly 1 result when all candidates are duplicates, got %d", len(res))
	}
}

// ============================================================================
// Group 2: EnrichQuery Edge Cases
// ============================================================================

func TestAdversarial_EnrichQuery_SpecialCharacters(t *testing.T) {
	se, s := testSearcher(t)

	// Seed memories with various tags
	seed(t, s, "global", "note", "security vulnerability scanning and remediation", []string{"sec", "cve", "vuln"})
	seed(t, s, "global", "note", "database connection pool tuning parameters", []string{"db", "postgres", "pool"})

	ctx := context.Background()

	// List of challenging queries with special FTS syntax, punctuation, quotes, injections
	adversarialInputs := []string{
		`security "scanning"`,
		`security "unbalanced quote`,
		`""`,
		`""""`,
		`"`,
		`security AND vulnerability`,
		`security OR database`,
		`security NOT database`,
		`NEAR(security scanning, 5)`,
		`sec*`,
		`security^2`,
		`db:postgres`,
		`hello/world`,
		`foo$bar%baz#qux@test&co!`,
		`[brackets] {braces} (parens)`,
		`math > 5 < 10 = 7`,
		`🚀 rocket cluster`,
		`привет мир`,
		`日本語 テスト`,
		`café résumé`,
		`'; DROP TABLE memories; --`,
		`" OR 1=1 --`,
		`17" display`,
		`foo""bar`,
		`"""hello"""`,
		`"open quote only`,
		`close quote only"`,
		`!@#$%^&*()`,
		`{test}`,
		`[test]`,
		`🚀🔥`,
		"security\n\tscanning",
		"   ",
		"...",
		"***",
	}

	for _, input := range adversarialInputs {
		t.Run(fmt.Sprintf("input_%q", input), func(t *testing.T) {
			q := Query{Text: input, Scope: "global", Inherit: true}
			enriched, err := se.EnrichQuery(ctx, q)
			if err != nil {
				t.Fatalf("EnrichQuery failed unexpectedly on %q: %v", input, err)
			}
			// Must never panic and must preserve or augment text
			if strings.TrimSpace(input) == "" {
				if enriched.Text != input {
					t.Errorf("expected whitespace query to return unchanged, got %q", enriched.Text)
				}
			}

			// End-to-end Recall must also not crash
			_, err = se.Recall(ctx, q)
			if err != nil {
				t.Fatalf("Recall failed unexpectedly on adversarial input %q: %v", input, err)
			}
		})
	}
}

func TestAdversarial_Vulnerability_FTS5InteriorQuotesAndSyntax(t *testing.T) {
	// Demonstrates Bug 1: Queries containing interior double quotes, measurements (2"x4"),
	// key="value", or JSON syntax fail with SQLite FTS5 syntax errors / unterminated string
	// instead of being sanitized or handled gracefully.
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "security vulnerability scanning and remediation", []string{"sec", "vuln"})

	ctx := context.Background()
	vulnerableInputs := []string{
		`foo"bar`,
		`key="value"`,
		`dimensions 2"x4"`,
		`{"key": "val"}`,
		"security\x00scanning",
	}

	for _, input := range vulnerableInputs {
		t.Run(fmt.Sprintf("vulnerable_%q", input), func(t *testing.T) {
			q := Query{Text: input, Scope: "global", Inherit: true}
			_, enrichErr := se.EnrichQuery(ctx, q)
			_, recallErr := se.Recall(ctx, q)

			if enrichErr != nil || recallErr != nil {
				t.Errorf("CONFIRMED VULNERABILITY: query %q causes unhandled crash: EnrichQuery=%v, Recall=%v",
					input, enrichErr, recallErr)
			}
		})
	}
}

func TestAdversarial_EnrichQuery_AlreadyPresentWords(t *testing.T) {
	se, s := testSearcher(t)

	// Seed memories
	seed(t, s, "global", "note", "redis caching cluster redis distributed cache", []string{"redis", "cache", "distributed", "nosql"})
	seed(t, s, "global", "note", "rate limiting middleware http proxy", []string{"rate-limiting", "middleware", "proxy"})

	ctx := context.Background()

	t.Run("case insensitive word exclusion", func(t *testing.T) {
		// "REDIS" and "CACHE" in query; "redis" and "cache" tags must NOT be appended
		q := Query{Text: "REDIS CACHE", Scope: "global", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		tokens := strings.Fields(enriched.Text)
		for _, tok := range tokens[2:] { // check appended tokens
			if strings.EqualFold(tok, "redis") || strings.EqualFold(tok, "cache") {
				t.Errorf("tag %q should have been excluded since query contains it (case-insensitive)", tok)
			}
		}
	})

	t.Run("punctuation attached to query word", func(t *testing.T) {
		// "redis," should still recognize "redis"
		q := Query{Text: "redis, cache.", Scope: "global", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		tokens := strings.Fields(enriched.Text)
		for _, tok := range tokens[2:] {
			if strings.EqualFold(tok, "redis") || strings.EqualFold(tok, "cache") {
				t.Errorf("tag %q should have been excluded even with punctuation in query", tok)
			}
		}
	})

	t.Run("compound tag exclusion when all subparts present", func(t *testing.T) {
		// Tag is "rate-limiting". Query has "rate limiting". Tag must NOT be added.
		q := Query{Text: "rate limiting middleware", Scope: "global", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if strings.Contains(enriched.Text, "rate-limiting") {
			t.Errorf("compound tag 'rate-limiting' should be excluded when query contains 'rate limiting', got %q", enriched.Text)
		}
	})

	t.Run("compound tag allowed when only one subpart present", func(t *testing.T) {
		// Tag is "rate-limiting". Query only has "middleware". "rate-limiting" is eligible!
		q := Query{Text: "middleware", Scope: "global", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if !strings.Contains(enriched.Text, "rate-limiting") {
			t.Errorf("compound tag 'rate-limiting' should be included when query doesn't have it, got %q", enriched.Text)
		}
	})

	t.Run("duplicate tags within same memory row counted only once", func(t *testing.T) {
		// Memory with duplicate tags in string
		seed(t, s, "global", "note", "duplicate tag test memory content", []string{"dup", "dup", "dup"})
		seed(t, s, "global", "note", "another test memory content", []string{"other", "other"})

		q := Query{Text: "duplicate test", Scope: "global", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		// Count occurrences of "dup" in enriched.Text
		countDup := 0
		for _, tok := range strings.Fields(enriched.Text) {
			if tok == "dup" {
				countDup++
			}
		}
		if countDup > 1 {
			t.Errorf("tag 'dup' appended multiple times: %d in %q", countDup, enriched.Text)
		}
	})
}

func TestAdversarial_EnrichQuery_ScopeRestrictions(t *testing.T) {
	se, s := testSearcher(t)

	// Create scopes:
	// global
	// project:p_alpha
	// project:p_beta
	seed(t, s, "global", "note", "infrastructure observability monitoring", []string{"tag-global"})
	seed(t, s, "project:p_alpha", "note", "infrastructure observability metrics", []string{"tag-proj-alpha"})
	seed(t, s, "project:p_beta", "note", "infrastructure observability telemetry", []string{"tag-proj-beta"})

	ctx := context.Background()

	t.Run("Inherit=false in project:p_alpha restricts strictly", func(t *testing.T) {
		q := Query{Text: "infrastructure observability", Scope: "project:p_alpha", Inherit: false}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if !strings.Contains(enriched.Text, "tag-proj-alpha") {
			t.Errorf("expected tag-proj-alpha in enriched text, got %q", enriched.Text)
		}
		if strings.Contains(enriched.Text, "tag-proj-beta") ||
			strings.Contains(enriched.Text, "tag-global") {
			t.Errorf("leaked tags across scope boundaries (Inherit=false): %q", enriched.Text)
		}
	})

	t.Run("Inherit=true in project:p_alpha includes global but NOT project:p_beta", func(t *testing.T) {
		q := Query{Text: "infrastructure observability", Scope: "project:p_alpha", Inherit: true}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery: %v", err)
		}
		if !strings.Contains(enriched.Text, "tag-proj-alpha") {
			t.Errorf("expected tag-proj-alpha in enriched text, got %q", enriched.Text)
		}
		if strings.Contains(enriched.Text, "tag-proj-beta") {
			t.Errorf("leaked tags from sibling project:p_beta with Inherit=true: %q", enriched.Text)
		}
	})

	t.Run("Non-existent scope returns unchanged without error", func(t *testing.T) {
		q := Query{Text: "infrastructure observability", Scope: "project:nonexistent_xyz", Inherit: false}
		enriched, err := se.EnrichQuery(ctx, q)
		if err != nil {
			t.Fatalf("EnrichQuery returned error for non-existent scope: %v", err)
		}
		if enriched.Text != "infrastructure observability" {
			t.Errorf("expected unchanged text for non-existent scope, got %q", enriched.Text)
		}
	})
}

// ============================================================================
// Group 3: Facts Prefix Search Verification
// ============================================================================

func TestAdversarial_Facts_PrefixPreservation(t *testing.T) {
	ctx := context.Background()
	se, s := testSearcher(t)

	// Seed multiple facts under a hierarchical namespace
	factsToSeed := []struct {
		key   string
		value string
		tags  []string
	}{
		{"system.network.dns.primary", `"1.1.1.1"`, []string{"dns", "cloudflare"}},
		{"system.network.dns.secondary", `"8.8.8.8"`, []string{"dns", "google"}},
		{"system.network.mtu", `1500`, []string{"mtu", "network"}},
		{"system.storage.mount", `"/data"`, []string{"storage"}},
	}

	for _, f := range factsToSeed {
		if _, _, err := s.SetFact(ctx, store.FactInput{
			Scope: "global",
			Key:   f.key,
			Value: f.value,
			Tags:  f.tags,
		}); err != nil {
			t.Fatalf("SetFact %s failed: %v", f.key, err)
		}
	}

	// Seed a note that will trigger query expansion for "system.network"
	seed(t, s, "global", "note", "system network configuration guidelines and protocols", []string{"protocols", "routing", "firewall"})

	t.Run("direct Facts call with prefix matches prefix keys only", func(t *testing.T) {
		factsRes, err := se.Facts(ctx, Query{
			Text: "system.network", Scope: "global", Inherit: true,
		}, 10)
		if err != nil {
			t.Fatalf("Facts call failed: %v", err)
		}

		matchedKeys := make(map[string]bool)
		for _, r := range factsRes {
			for _, f := range factsToSeed {
				if strings.Contains(r.Content, f.key) {
					matchedKeys[f.key] = true
				}
			}
		}

		if !matchedKeys["system.network.dns.primary"] ||
			!matchedKeys["system.network.dns.secondary"] ||
			!matchedKeys["system.network.mtu"] {
			t.Errorf("Facts prefix search missed expected keys: %+v", matchedKeys)
		}
		if matchedKeys["system.storage.mount"] {
			t.Errorf("Facts prefix search incorrectly matched system.storage.mount")
		}
	})

	t.Run("Recall with expanded query preserves prefix fact matches via facts ranker", func(t *testing.T) {
		// When query is "system.network", EnrichQuery expands Keyword and Semantic to "system.network protocols routing firewall".
		// But Facts must receive unexpanded "system.network", matching prefix keys.
		res, err := se.Recall(ctx, Query{
			Text: "system.network", Scope: "global", Inherit: true, Top: 10,
		})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}

		factsRankerMatched := make(map[string]bool)
		for _, r := range res {
			if slices.Contains(r.MatchedBy, "facts") {
				for _, f := range factsToSeed {
					if strings.Contains(r.Content, f.key) {
						factsRankerMatched[f.key] = true
					}
				}
			}
		}

		if !factsRankerMatched["system.network.dns.primary"] {
			t.Errorf("facts ranker in Recall missed system.network.dns.primary")
		}
		if !factsRankerMatched["system.network.dns.secondary"] {
			t.Errorf("facts ranker in Recall missed system.network.dns.secondary")
		}
		if !factsRankerMatched["system.network.mtu"] {
			t.Errorf("facts ranker in Recall missed system.network.mtu")
		}
		if factsRankerMatched["system.storage.mount"] {
			t.Errorf("facts ranker in Recall incorrectly matched system.storage.mount")
		}
	})

	t.Run("exact key search returns specific fact", func(t *testing.T) {
		res, err := se.Recall(ctx, Query{
			Text: "system.network.dns.primary", Scope: "global", Inherit: true, Top: 5,
		})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}
		if len(res) == 0 {
			t.Fatalf("expected at least 1 result for exact key search")
		}
		var foundExact bool
		for _, r := range res {
			if r.Type == "fact" && strings.Contains(r.Content, "system.network.dns.primary") && slices.Contains(r.MatchedBy, "facts") {
				foundExact = true
				break
			}
		}
		if !foundExact {
			t.Errorf("exact fact key search failed to return target fact in results: %+v", res)
		}
	})
}

// ============================================================================
// Group 4: Concurrency, Scale & Robustness
// ============================================================================

func TestAdversarial_Recall_ConcurrentSafe(t *testing.T) {
	se, s := testSearcher(t)

	// Seed data
	for i := 0; i < 20; i++ {
		seed(t, s, "global", "note", fmt.Sprintf("concurrent test memory content payload %d", i), []string{"concurrency", "test"})
	}

	ctx := context.Background()
	concurrency := 16
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			q := Query{
				Text:    "concurrent test memory",
				Scope:   "global",
				Inherit: true,
				Top:     5,
			}
			_, err := se.Recall(ctx, q)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d Recall: %w", idx, err)
				return
			}
			_, err = se.EnrichQuery(ctx, q)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d EnrichQuery: %w", idx, err)
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent execution failure: %v", err)
		}
	}
}

func TestAdversarial_Vulnerability_WithDecayDays_DataRace(t *testing.T) {
	// Demonstrates Bug 2: WithDecayDays mutates Searcher in place rather than copying
	// by value (dup := *s), triggering a data race when called concurrently.
	se, s := testSearcher(t)
	seed(t, s, "global", "note", "concurrent test memory", []string{"test"})

	ctx := context.Background()
	concurrency := 8
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			// WithDecayDays mutates s.decayHalfLifeDays without mutex/value-copy
			forked := se.WithDecayDays(idx % 10)
			q := Query{Text: "concurrent test memory", Scope: "global", Inherit: true, Top: 2}
			_, err := forked.Recall(ctx, q)
			errCh <- err
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent error: %v", err)
		}
	}
}

func TestAdversarial_CollapseNearDupes_LargeScaleStress(t *testing.T) {
	const n = 10000
	now := time.Now()
	items := make([]*Ranked, n)

	// Interleave 10 distinct hashes, each appearing 500 times, plus 5000 empty-hash items
	for i := 0; i < n; i++ {
		var hash string
		if i%2 == 0 {
			hash = fmt.Sprintf("hash_%d", (i/2)%10)
		}
		items[i] = &Ranked{
			ID:          int64(i + 1),
			ContentHash: hash,
			Score:       float64(n - i),
			CreatedAt:   now.Add(time.Duration(-i) * time.Minute),
		}
	}

	start := time.Now()
	collapsed := collapseNearDupes(items)
	duration := time.Since(start)

	// Expected survivors: 10 non-empty hashes + 5000 empty hashes = 5010
	expectedCount := 10 + 5000
	if len(collapsed) != expectedCount {
		t.Fatalf("expected %d survivors from %d items, got %d", expectedCount, n, len(collapsed))
	}

	// Performance check: 10,000 items should take under 50ms
	if duration > 50*time.Millisecond {
		t.Errorf("collapseNearDupes took too long on 10k items: %v", duration)
	}
}

func TestAdversarial_EnrichQuery_WeirdTags(t *testing.T) {
	se, s := testSearcher(t)

	// Seed memories with weird tag formats: emojis, empty, spaces, punctuation
	seed(t, s, "global", "note", "unusual tags memory content alpha", []string{"🏷️-emoji", "  padded  ", "", "tag,with,commas", "UPPERCASE-TAG"})

	ctx := context.Background()
	q := Query{Text: "unusual tags memory alpha", Scope: "global", Inherit: true}
	enriched, err := se.EnrichQuery(ctx, q)
	if err != nil {
		t.Fatalf("EnrichQuery failed on weird tags: %v", err)
	}

	if strings.TrimSpace(enriched.Text) == "" {
		t.Fatalf("enriched text became empty")
	}
}

