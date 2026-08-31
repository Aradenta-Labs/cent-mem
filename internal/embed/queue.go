package embed

import (
	"context"
	"time"

	"github.com/farras/cent-mem/internal/store"
)

// Queue drains the embed_queue asynchronously (inline in the CLI, or a future
// daemon). It is concurrent-safe via the store's atomic claim.
type Queue struct {
	store     *store.Store
	emb       Embedder
	BatchSize int           // default 16
	MaxTime   time.Duration // default 250ms
	// StaleAfter is how old a claim must be before it is reclaimed. Default 60s.
	StaleAfter time.Duration
	// Model is the embedding model name recorded in the embeddings row.
	Model string
}

// NewQueue builds a Queue with sane defaults.
func NewQueue(s *store.Store, emb Embedder) *Queue {
	return &Queue{
		store:      s,
		emb:        emb,
		BatchSize:  16,
		MaxTime:    250 * time.Millisecond,
		StaleAfter: 60 * time.Second,
		Model:      "bge-small-en-v1.5",
	}
}

// Drain processes up to BatchSize queued memories per iteration, embedding them
// and writing their vectors, until the queue is empty or MaxTime has elapsed.
// It returns the total number of memories processed.
func (q *Queue) Drain(ctx context.Context) (int, error) {
	batch := q.BatchSize
	if batch <= 0 {
		batch = 16
	}
	maxTime := q.MaxTime
	if maxTime <= 0 {
		maxTime = 250 * time.Millisecond
	}
	stale := q.StaleAfter
	if stale <= 0 {
		stale = 60 * time.Second
	}

	deadline := time.Now().Add(maxTime)
	processed := 0

	for {
		if time.Now().After(deadline) {
			break
		}
		staleBefore := time.Now().Add(-stale).UnixMicro()

		ids, err := q.store.ClaimEmbedJobs(ctx, batch, staleBefore)
		if err != nil {
			return processed, err
		}
		if len(ids) == 0 {
			break
		}

		texts, err := q.store.LoadMemoryEmbedTexts(ctx, ids)
		if err != nil {
			return processed, err
		}
		if len(texts) == 0 {
			break
		}

		ordered := make([]string, len(ids))
		for i, id := range ids {
			ordered[i] = texts[id]
		}

		vecs, err := q.emb.Embed(ctx, ordered)
		if err != nil {
			return processed, err
		}

		for i, id := range ids {
			// A single bad row must not block the others.
			if i >= len(vecs) {
				continue
			}
			if err := q.store.WriteEmbedding(ctx, id, vecs[i], q.Model); err != nil {
				// Leave the row claimed; the next stale pass will retry it.
				continue
			}
			processed++
		}
	}
	return processed, nil
}
