package embed_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/farras/cent-mem/internal/embed"
)

// countingEmbedder wraps a stub and counts Embed calls.
type countingEmbedder struct {
	inner embed.Embedder
	calls int64
}

func (c *countingEmbedder) Dims() int    { return c.inner.Dims() }
func (c *countingEmbedder) Close() error { return c.inner.Close() }
func (c *countingEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	atomic.AddInt64(&c.calls, 1)
	return c.inner.Embed(ctx, texts)
}

// I.15a — a repeated text is a cache hit (model not called again for it).
func TestCache_Hit(t *testing.T) {
	inner := &countingEmbedder{inner: embed.NewStub(16)}
	c := embed.NewCachingEmbedder(inner, 256)
	ctx := context.Background()

	texts := []string{"repeat me", "unique one", "repeat me"}
	if _, err := c.Embed(ctx, texts); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	// Both unique texts are embedded in a single batch → 1 model call.
	if got := atomic.LoadInt64(&inner.calls); got != 1 {
		t.Fatalf("expected 1 batch model call, got %d", got)
	}

	// A cache hit on the repeated text must NOT trigger another model call.
	if _, err := c.Embed(ctx, []string{"repeat me"}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if got := atomic.LoadInt64(&inner.calls); got != 1 {
		t.Fatalf("expected still 1 model call after cache hit, got %d", got)
	}
}

// I.15b — LRU evicts entries past capacity.
func TestCache_Eviction(t *testing.T) {
	inner := &countingEmbedder{inner: embed.NewStub(8)}
	c := embed.NewCachingEmbedder(inner, 2) // tiny capacity
	ctx := context.Background()

	// Insert three distinct texts; capacity 2 means the first is evicted.
	for i := 0; i < 3; i++ {
		if _, err := c.Embed(ctx, []string{"text" + string(rune('a'+i))}); err != nil {
			t.Fatalf("Embed: %v", err)
		}
	}
	// "texta" was evicted → re-embedding it calls the model again.
	if _, err := c.Embed(ctx, []string{"texta"}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	// 3 inserts + 1 re-embed of evicted = 4 calls.
	if got := atomic.LoadInt64(&inner.calls); got != 4 {
		t.Fatalf("expected 4 model calls (eviction), got %d", got)
	}
}
