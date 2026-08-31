# Phase 2 (M2) — Embeddings & Hybrid Search

**Goal:** `recall` returns semantically relevant results even when there is no keyword overlap. Hybrid ranking (semantic + keyword + facts + timeline via RRF) is fully wired. Writes trigger background embedding with no user-visible latency.

**Exit criteria:**
- An embedded memory is retrievable by paraphrase (no shared keywords).
- p95 `recall` < 300 ms on a 10k-memory benchmark; < 800 ms on 100k.
- Write CLI returns in < 50 ms p95 (embed queue drained in background).
- `memories_vec` is populated; matches in the index are correct.

---

## 2.1 Prerequisites

- Phase 1 complete: keyword search, facts, timeline, JSON contract all locked.
- Embedder spike from Phase 0 confirmed the binding loads and embeds correctly.

## 2.2 Embedder service — `internal/embed/embed.go`

Build the production embedder around the binding chosen in M0.

```go
type Embedder interface {
    Dims() int
    Embed(ctx, texts []string) ([][]float32, error)
    Close() error
}

func NewONNX(modelPath string, dims int) (Embedder, error)
```

- Lazy model load on first call; cache the session.
- Censor / sanitize: strip NUL bytes, enforce a max text length (config, default 8 KB).
- `Close()` releases the ONNX session.
- Determinism: test asserts same input → same vector (within float epsilon).

**Thread safety:** Embedder is safe for concurrent calls (most bindings are; verify with a stress test).

## 2.3 Vec index — `internal/store/migrations/m0002_vec.sql`

`m0001_init.sql` already created `embeddings`; add `m0002_vec.sql` to create the vec0 table and a migration that runs *only* if the vec0 extension loaded successfully (otherwise M0 spike would have failed):

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS memories_vec USING vec0(
  memory_id INTEGER PRIMARY KEY,
  embedding float[384]
);
CREATE INDEX IF NOT EXISTS idx_memories_status_scope ON memories(status, scope_id);
```

**Apply the same vec existence check from M0 spike**: if `CREATE VIRTUAL TABLE` fails, abort the migration with a clear error instructing the user to switch to a cgo SQLite driver (catches the rare pure-Go + vec0 incompatibility).

## 2.4 Queue drain — `internal/embed/queue.go`

```go
type Queue struct {
    store *store.Store
    emb   Embedder
    BatchSize int  // default 16
    MaxTime   time.Duration // default 250ms
}

func (q *Queue) Drain(ctx) (processed int, err error)
```

**Claim algorithm** (concurrent-safe):
```sql
UPDATE embed_queue
   SET claimed_at = ?1
 WHERE memory_id IN (
   SELECT memory_id FROM embed_queue
    WHERE claimed_at IS NULL OR claimed_at < ?2   -- stale claims
    ORDER BY priority DESC, created_at ASC
    LIMIT ?3
   )
 RETURNING memory_id;
```

**Drain loop (used both in inline mode and any future daemon):**
1. Claim up to `BatchSize` rows whose claim is stale.
2. Load the corresponding memories (id, content, key).
3. Embed texts.
4. For each `(memory_id, vec)`:
   - `INSERT OR REPLACE INTO embeddings(memory_id, embedding, model, embedded_at) VALUES (?,?,?,?)`.
   - `INSERT OR REPLACE INTO memories_vec(memory_id, embedding) VALUES (?, ?)`.
5. Set the memory's `status` if it was `queued` → `embedded` (so the CLI's `status` field stays accurate).
6. Repeat until no rows or `MaxTime` elapsed.

## 2.5 Inline drain in CLI writes

In `cmd/centmem/commands/put.go` and `set.go`:
- After successful write, spawn a goroutine (or call Drain with a context deadline) that drains the queue with `MaxTime=200ms`.
- The CLI returns immediately with `status:"queued"`; the next CLI invocation continues draining.
- Tradeoff: a process exit stops draining. Acceptable for v1 because writes are rare and the next read/write will pick up the slack. A v1.1 daemon will replace this.

**Crash safety:** Queue rows are claimed then processed; if the process dies mid-batch, the next run's stale-claim logic (`claimed_at < now - 60s`) recovers.

## 2.6 Vector search — `internal/search/semantic.go`

```go
func (s *Searcher) Semantic(ctx, q Query) ([]Ranked, error) {
    // 1. Embed q.Text via Embedder (cache embeddings by query hash if perf demands)
    // 2. Run vec0 nearest-neighbor query constrained by scope/filter:
    //      SELECT memory_id, distance
    //        FROM memories_vec
    //       WHERE embedding MATCH ?1
    //       ORDER BY distance ASC
    //       LIMIT ?2
    // 3. Join back to memories to apply type/tags/since/until/agent filters
    // 4. Return Ranked slice with score = 1 - (distance / maxDistance)
}
```

**`Recall` full hybrid path** (`internal/search/search.go`):
1. Run `Keyword`, `Facts`, `Timeline`, `Semantic` (each in parallel where possible via `errgroup`).
2. Fuse with RRF (k=60).
3. Apply post-filters.
4. Dedupe by `id`.
5. Slice `top`.

**Score normalization:** RRF produces values in [0, 1). Per-rank scores are not exposed; only the fused score.

## 2.7 Embedding cache

`internal/embed/cache.go` — LRU keyed by `sha256(text)`, value = vector. Avoid re-embedding the same query text (common: the same recall query repeated).

- LRU size: 256 entries (config).
- Eviction by `container/list`.

## 2.8 Benchmarks

`internal/search/search_bench_test.go`:

| Benchmark | Setup | Asserts |
|-----------|-------|---------|
| `BenchmarkRecall_Hybrid_10k` | 10k memories with embeddings | p50, p95, p99, allocations |
| `BenchmarkRecall_Keyword_Only_10k` | same | compare to hybrid |
| `BenchmarkEmbedQueue_Drain_64` | 64 queued rows | throughput (vec/s) |
| `BenchmarkEmbedQuery` | cold + warm cache | cold vs warm latency |

Record results in a `testdata/bench-results.txt` after each run. Targets:

- `Recall` p95 < 300 ms @ 10k, < 800 ms @ 100k.
- `Embed` warm > 200 sentences/sec.
- `Queue.Drain` > 100 mems/sec.

## 2.9 Implementation order

1. `internal/embed/embed.go` (real model load + Embed).
2. `internal/embed/queue.go` (drain + claim).
3. `m0002_vec.sql` migration + apply on Open.
4. `internal/search/semantic.go` (vec0 query + join + filter).
5. `Recall` upgraded to hybrid.
6. `put`/`set` hook to drain queue.
7. `internal/embed/cache.go`.
8. Benchmarks + tuning.

## 2.10 Checklist

- [ ] Embedder loads `bge-small-en-v1.5.onnx` from `~/.centmem/models/`
- [ ] `Embed` deterministic, thread-safe
- [ ] `m0002_vec.sql` applies on Open with vec0 verified
- [ ] `memories_vec` populated for every embedded memory
- [ ] Queue drain is concurrent-safe (stale-claim recovery tested)
- [ ] Inline drain runs in put/set without blocking CLI > 50 ms
- [ ] `Recall` returns semantically relevant results
- [ ] Cache reduces repeated query latency
- [ ] All Phase 1 tests still pass (no contract regression)
- [ ] Benchmarks recorded; targets met

## 2.11 Phase 2 test plan (validation)

### Unit tests

| Test | Package | Validates |
|------|---------|-----------|
| `TestEmbedder_Deterministic` | embed | same text → same vector |
| `TestEmbedder_Dims` | embed | output len == Dims() |
| `TestEmbedder_Finite` | embed | no NaN/Inf in vector |
| `TestEmbedder_Empty` | embed | empty string handled (return zero vec or skip) |
| `TestEmbedder_Truncate` | embed | overlong text is truncated, not crashed |
| `TestQueue_Claim_Stale` | embed | stale `claimed_at` is reclaimed |
| `TestQueue_Drain_NoRows` | embed | returns 0 with no work |
| `TestQueue_Drain_BatchLimit` | embed | respects BatchSize |
| `TestQueue_Drain_WritesVec` | embed | memories_vec has rows after drain |
| `TestQueue_Drain_PartialFailure` | embed | one bad row doesn't block others |
| `TestSemantic_NearestNeighbor` | search | embeds query, returns expected top match |
| `TestSemantic_FilterByScope` | search | scope filter applied after vec match |
| `TestSemantic_FilterByType` | search | type filter applied |
| `TestRecall_HybridFuse` | search | items matched only by semantic appear |
| `TestRecall_ParaphraseQuery` | search | "how do we ship" finds "deploy via GitHub Actions" |
| `TestCache_Hit` | embed | second embed of same text is a cache hit (count model calls) |
| `TestCache_Eviction` | embed | LRU evicts past 256 |

### Integration / end-to-end

| Test | Validates |
|------|-----------|
| `TestE2E_ParaphraseRetrieval` | write "we deploy to fly.io"; recall "how do we ship?" → finds it |
| `TestE2E_InlineDrain` | write 50 memories; `stats.pending_embeddings` returns to 0 within 5s |
| `TestE2E_RecallIsFast` | seed 10k; measure 100 `recall` calls; p95 < 300 ms |
| `TestE2E_RecollisionAfterRestart` | queue has 5 rows; kill CLI mid-drain; rerun; queue drains |
| `TestE2E_NoNetwork` | disable network; embedder + recall still work |

### Golden / CLI

- `TestCLI_Recall_SemanticMatched` — update the M1 golden file to allow `matched_by` including `"semantic"`. Add new golden for paraphrase scenario.

### Performance tests (`_bench_test.go`)

| Bench | Threshold |
|-------|-----------|
| `BenchmarkRecall_Hybrid_10k` | < 300 ms p95 |
| `BenchmarkRecall_Hybrid_100k` | < 800 ms p95 |
| `BenchmarkEmbedQueue_Drain_64` | > 100 mems/sec |
| `BenchmarkEmbedQuery_WarmCache` | < 5 ms |

**Definition of done:** all unit + integration + bench thresholds met; `go test ./... -race` green; then proceed to [plan-phase-3.md](plan-phase-3.md).
