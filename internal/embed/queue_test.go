package embed_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/farras/cent-mem/internal/config"
	"github.com/farras/cent-mem/internal/embed"
	"github.com/farras/cent-mem/internal/store"
)

func testStoreAndQueue(t *testing.T) (*store.Store, *embed.Queue) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{DBPath: dir + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	q := embed.NewQueue(s, embed.NewStub(384))
	q.BatchSize = 16
	q.MaxTime = 500 * time.Millisecond
	return s, q
}

func pendingCount(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var n int64
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM embed_queue`).Scan(&n); err != nil {
		t.Fatalf("count embed_queue: %v", err)
	}
	return n
}

func vecCount(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var n int64
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM memories_vec`).Scan(&n); err != nil {
		t.Fatalf("count memories_vec: %v", err)
	}
	return n
}

// I.7 — Drain with no work returns 0.
func TestQueue_Drain_NoRows(t *testing.T) {
	_, q := testStoreAndQueue(t)
	n, err := q.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 processed, got %d", n)
	}
}

// I.8 — Drain respects BatchSize (only claims up to the limit per pass).
func TestQueue_Drain_BatchLimit(t *testing.T) {
	s, q := testStoreAndQueue(t)
	// 5 memories with distinct content (PutMemory dedups identical content).
	q.BatchSize = 3
	for i := 0; i < 5; i++ {
		if _, _, err := s.PutMemory(context.Background(), store.MemoryInput{
			Scope: "global", Type: "note", Content: fmt.Sprintf("batch memory %d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	n, err := q.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	// With 3-claim and repeat-until-empty, all are processed.
	if n != 5 {
		t.Fatalf("expected 5 processed, got %d", n)
	}
	if pendingCount(t, s) != 0 {
		t.Fatalf("expected queue empty after drain")
	}
}

// I.9 — Drain writes rows to memories_vec and clears the queue.
func TestQueue_Drain_WritesVec(t *testing.T) {
	s, q := testStoreAndQueue(t)
	for i := 0; i < 4; i++ {
		if _, _, err := s.PutMemory(context.Background(), store.MemoryInput{
			Scope: "global", Type: "note", Content: fmt.Sprintf("vec memory %d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if vecCount(t, s) != 0 {
		t.Fatalf("expected no vectors before drain")
	}
	if _, err := q.Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if vecCount(t, s) != 4 {
		t.Fatalf("expected 4 vectors after drain, got %d", vecCount(t, s))
	}
	if pendingCount(t, s) != 0 {
		t.Fatalf("expected queue empty, got %d", pendingCount(t, s))
	}
}

// I.6 — Stale claims are reclaimed.
func TestQueue_Claim_Stale(t *testing.T) {
	s, q := testStoreAndQueue(t)
	if _, _, err := s.PutMemory(context.Background(), store.MemoryInput{
		Scope: "global", Type: "note", Content: "stale claim",
	}); err != nil {
		t.Fatal(err)
	}

	// Simulate a stale claim: set claimed_at to 10 minutes ago.
	if _, err := s.DB().Exec(`UPDATE embed_queue SET claimed_at = ?`,
		time.Now().Add(-10*time.Minute).UnixMicro()); err != nil {
		t.Fatal(err)
	}

	q.StaleAfter = 60 * time.Second
	n, err := q.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected stale claim reclaimed and processed, got %d", n)
	}
	if pendingCount(t, s) != 0 {
		t.Fatalf("expected queue empty after reclaim")
	}
}

// flakyEmbedder wraps a StubEmbedder and fails when it sees the sentinel text,
// simulating a per-batch embedding failure.
type flakyEmbedder struct {
	inner   embed.Embedder
	failOn  string
	failErr error
}

func (f *flakyEmbedder) Dims() int    { return f.inner.Dims() }
func (f *flakyEmbedder) Close() error { return f.inner.Close() }
func (f *flakyEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	for _, t := range texts {
		if t == f.failOn {
			return nil, f.failErr
		}
	}
	return f.inner.Embed(ctx, texts)
}

var errEmbedFail = errors.New("simulated embed failure")

// I.10 — A partial failure does not block earlier processed rows. We use an
// embedder that fails on a later batch; rows already embedded are counted.
func TestQueue_Drain_PartialFailure(t *testing.T) {
	s, _ := testStoreAndQueue(t)

	put := func(content string) {
		t.Helper()
		if _, _, err := s.PutMemory(context.Background(), store.MemoryInput{
			Scope: "global", Type: "note", Content: content,
		}); err != nil {
			t.Fatal(err)
		}
	}
	put("valid one")
	put("valid two")
	put("BROKEN")

	q := embed.NewQueue(s, &flakyEmbedder{
		inner:   embed.NewStub(384),
		failOn:  "BROKEN",
		failErr: errEmbedFail,
	})
	q.BatchSize = 2
	q.MaxTime = 500 * time.Millisecond

	n, err := q.Drain(context.Background())
	// Batch 1 (valid one + valid two) succeeds; batch 2 (BROKEN) fails.
	if err == nil {
		t.Fatalf("expected Drain to surface the batch embed failure")
	}
	if n != 2 {
		t.Fatalf("expected 2 rows processed before failure, got %d", n)
	}
	if vecCount(t, s) != 2 {
		t.Fatalf("expected 2 vectors written before failure, got %d", vecCount(t, s))
	}
}
