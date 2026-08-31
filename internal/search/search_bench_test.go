package search_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/farras/cent-mem/internal/config"
	"github.com/farras/cent-mem/internal/embed"
	"github.com/farras/cent-mem/internal/search"
	"github.com/farras/cent-mem/internal/store"
)

// benchStore seeds n memories with distinct content and drains the embed queue
// so every memory has a vector. Returns a searcher with the matching embedder.
func benchStore(b *testing.B, n int) (*search.Searcher, *store.Store) {
	b.Helper()
	dir := b.TempDir()
	s, err := store.Open(config.Config{DBPath: dir + "/centmem.db"})
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	b.Cleanup(func() { s.Close() })

	for i := 0; i < n; i++ {
		if _, _, err := s.PutMemory(context.Background(), store.MemoryInput{
			Scope: "global", Type: "note",
			Content: fmt.Sprintf("memory about domain topic number %d for benchmark", i),
		}); err != nil {
			b.Fatalf("PutMemory: %v", err)
		}
	}
	q := embed.NewQueue(s, embed.NewStub(384))
	if _, err := q.Drain(context.Background()); err != nil {
		b.Fatalf("Drain: %v", err)
	}
	return search.New(s).WithEmbedder(embed.NewStub(384)), s
}

// BenchmarkRecall_Hybrid_10k measures hybrid (semantic+keyword) recall at 10k.
func BenchmarkRecall_Hybrid_10k(b *testing.B) {
	se, _ := benchStore(b, 10000)
	ctx := context.Background()
	q := search.Query{Text: "domain topic 1234", Top: 5, Scope: "global", Inherit: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := se.Recall(ctx, q); err != nil {
			b.Fatalf("Recall: %v", err)
		}
	}
}

// BenchmarkRecall_Keyword_Only_10k measures keyword-only recall at 10k.
func BenchmarkRecall_Keyword_Only_10k(b *testing.B) {
	se, _ := benchStore(b, 10000)
	ctx := context.Background()
	q := search.Query{Text: "domain topic 1234", Top: 5, Scope: "global", Inherit: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := se.Keyword(ctx, q, 15); err != nil {
			b.Fatalf("Keyword: %v", err)
		}
	}
}

// BenchmarkEmbedQueue_Drain_64 measures queue drain throughput with 64 rows.
func BenchmarkEmbedQueue_Drain_64(b *testing.B) {
	s, err := store.Open(config.Config{DBPath: b.TempDir() + "/centmem.db"})
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// Seed 64 distinct memories once.
	ids := make([]int64, 64)
	for i := range ids {
		id, _, err := s.PutMemory(ctx, store.MemoryInput{
			Scope: "global", Type: "note", Content: fmt.Sprintf("benchmark drain row %d", i),
		})
		if err != nil {
			b.Fatalf("PutMemory: %v", err)
		}
		ids[i] = id
	}
	// Clear the initial queue so we control re-enqueueing below.
	if _, err := s.DB().Exec(`DELETE FROM embed_queue`); err != nil {
		b.Fatalf("clear queue: %v", err)
	}

	reEnqueue := func() {
		// Clear any claimed-but-not-yet-written rows from the previous iteration
		// so re-inserting the full set is always clean.
		if _, err := s.DB().Exec(`DELETE FROM embed_queue`); err != nil {
			b.Fatalf("clear queue: %v", err)
		}
		for _, id := range ids {
			if _, err := s.DB().Exec(`INSERT INTO embed_queue(memory_id, created_at) VALUES (?, 0)`, id); err != nil {
				b.Fatalf("enqueue: %v", err)
			}
		}
	}

	q := embed.NewQueue(s, embed.NewStub(384))
	q.MaxTime = time.Hour // ensure every iteration drains all 64 rows
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reEnqueue()
		if _, err := q.Drain(ctx); err != nil {
			b.Fatalf("Drain: %v", err)
		}
	}
}

// BenchmarkEmbedQuery measures a single-text embed (cold-ish, but stub-backed).
func BenchmarkEmbedQuery(b *testing.B) {
	emb := embed.NewStub(384)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := emb.Embed(ctx, []string{"how do we deploy the service"}); err != nil {
			b.Fatalf("Embed: %v", err)
		}
	}
}

// BenchmarkEmbedQuery_WarmCache measures embedding through the LRU cache on a hit.
func BenchmarkEmbedQuery_WarmCache(b *testing.B) {
	c := embed.NewCachingEmbedder(embed.NewStub(384), 256)
	ctx := context.Background()
	text := "how do we deploy the service"
	if _, err := c.Embed(ctx, []string{text}); err != nil {
		b.Fatalf("Embed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Embed(ctx, []string{text}); err != nil {
			b.Fatalf("Embed: %v", err)
		}
	}
}
