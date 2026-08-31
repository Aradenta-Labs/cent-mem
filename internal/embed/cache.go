package embed

import (
	"container/list"
	"context"
	"crypto/sha256"
	"sync"
)

// lruCache is a simple thread-safe LRU keyed by string.
type lruCache struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List // front = most recent
	items map[string]*list.Element
}

type cacheEntry struct {
	key   string
	value []float32
}

func newLRU(capacity int) *lruCache {
	if capacity <= 0 {
		capacity = 1
	}
	return &lruCache{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

// Get returns the cached value and whether it was present.
func (c *lruCache) Get(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).value, true
}

// Put inserts or updates key, evicting the least-recently-used entry when over capacity.
func (c *lruCache) Put(key string, value []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*cacheEntry).value = value
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&cacheEntry{key: key, value: value})
	c.items[key] = el
	if c.ll.Len() > c.cap {
		last := c.ll.Back()
		if last != nil {
			c.ll.Remove(last)
			delete(c.items, last.Value.(*cacheEntry).key)
		}
	}
}

// CachingEmbedder wraps an Embedder and caches embeddings keyed by the text
// hash, avoiding re-embedding repeated query strings (e.g. the same recall).
type CachingEmbedder struct {
	inner Embedder
	cache *lruCache
}

// NewCachingEmbedder wraps e with an LRU of the given capacity (default 256).
func NewCachingEmbedder(e Embedder, capacity int) *CachingEmbedder {
	if capacity <= 0 {
		capacity = 256
	}
	return &CachingEmbedder{inner: e, cache: newLRU(capacity)}
}

// Dims returns the underlying embedder's dimension.
func (c *CachingEmbedder) Dims() int { return c.inner.Dims() }

// Close closes the underlying embedder.
func (c *CachingEmbedder) Close() error { return c.inner.Close() }

// Embed returns cached vectors for already-seen texts, embedding only the misses.
func (c *CachingEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	missIdx := make([]int, 0, len(texts))
	missTexts := make([]string, 0, len(texts))

	for i, t := range texts {
		key := cacheKey(t)
		if v, ok := c.cache.Get(key); ok {
			out[i] = v
			continue
		}
		missIdx = append(missIdx, i)
		missTexts = append(missTexts, t)
	}

	if len(missTexts) == 0 {
		return out, nil
	}

	vecs, err := c.inner.Embed(ctx, missTexts)
	if err != nil {
		return nil, err
	}
	for j, idx := range missIdx {
		if j >= len(vecs) {
			continue
		}
		out[idx] = vecs[j]
		c.cache.Put(cacheKey(missTexts[j]), vecs[j])
	}
	return out, nil
}

func cacheKey(text string) string {
	h := sha256.Sum256([]byte(text))
	return string(h[:])
}
