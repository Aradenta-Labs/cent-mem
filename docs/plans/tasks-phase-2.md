# cent-mem — Phase 2 (M2) Task List
# Embeddings & Hybrid Search
# Source: plan-phase-2.md, ../implementation-plan.md, ../PRD.md,
#         ../architecture.md, ../data-model.md, ../cli-contract.md
# Goal: `recall` returns semantically relevant results even with zero keyword
#       overlap. Hybrid ranking (semantic + keyword + facts + timeline via RRF)
#       fully wired. Writes trigger background embedding with no visible latency.

> **STATUS: COMPLETE** — Phase 2 (M2) implemented and verified. Real ONNX
> inference is wired with an offline `StubEmbedder` fallback, the vec0 index
> migration applies on Open, queue drain runs inline on writes with stale-claim
> recovery, and hybrid recall fuses semantic + keyword + facts + timeline via RRF.
> All unit, e2e, golden, race, coverage, and benchmark gates pass.

## Prerequisites (blockers)
- [x] 0.1 Verify Phase 1 complete: keyword search, facts, timeline, JSON contract locked
- [x] 0.2 Verify embedder spike from Phase 0 confirmed the ONNX binding loads + embeds correctly
      (note: current `internal/embed/embed.go` `Embed` is a placeholder returning a
      deterministic pseudo-vector; real tokenization + tensor I/O + pooling is Phase 2 work)

## Track A — Embedder Service (internal/embed/embed.go)
- [x] A.1 Resolve the `Embedder` interface to the Phase 2 shape:
        `Dims() int`, `Embed(ctx, texts []string) ([][]float32, error)`, `Close() error`
        (current interface is single-text `Embed(ctx, text)`; decide batch signature)
- [x] A.2 Real tokenizer: convert text → `input_ids` + `attention_mask` tensors
        (tokenizer compatible with bge-small-en-v1.5; fixed-length, right-padded/truncated)
- [x] A.3 Real session execution via ONNX `AdvancedSession`: run inputs, read
        `last_hidden_state`, mean-pool (excluding padding) + L2-normalize
- [x] A.4 Lazy model load on first call; cache the session (already stubbed)
- [x] A.5 Sanitize input: strip NUL bytes; enforce max text length (config, default 8 KB)
- [x] A.6 `Close()` releases the ONNX session (already stubbed)
- [x] A.7 Thread safety: verify concurrent `Embed` is safe; add stress test
- [x] A.8 Keep `StubEmbedder` for offline/CI tests (no shared lib / no model)

## Track B — Vec Index Migration (internal/store/migrations/m0002_vec.sql)
- [x] B.1 Create `m0002_vec.sql`: `CREATE VIRTUAL TABLE IF NOT EXISTS memories_vec
        USING vec0(memory_id INTEGER PRIMARY KEY, embedding float[384]);`
        (note: m0001 already created `memories_vec` guarded; reconcile — m0002 should
        ensure it exists + add `idx_memories_status_scope ON memories(status, scope_id)`)
- [x] B.2 Apply the same vec0 existence check from the M0 spike: if `CREATE VIRTUAL TABLE`
        fails, abort migration with a clear error telling the user to use the cgo driver
- [x] B.3 Ensure migration runner picks up m0002 (idempotent; records `applied_migration`)

## Track C — Queue Drain (internal/embed/queue.go)
- [x] C.1 Define `Queue{store, emb, BatchSize (16), MaxTime (250ms)}` and
        `Drain(ctx) (processed int, err error)`
- [x] C.2 Claim algorithm (concurrent-safe) via:
        `UPDATE embed_queue SET claimed_at = ? WHERE memory_id IN (
          SELECT memory_id FROM embed_queue WHERE claimed_at IS NULL OR claimed_at < ?
          ORDER BY priority DESC, created_at ASC LIMIT ?) RETURNING memory_id`
- [x] C.3 Drain loop: claim ≤ BatchSize → load memories (id, content, key) → embed →
        for each (memory_id, vec): `INSERT OR REPLACE INTO embeddings(...)` +
        `INSERT OR REPLACE INTO memories_vec(...)`
- [x] C.4 Flip memory `status` queued → embedded after drain (so CLI `status` stays accurate)
- [x] C.5 Repeat until no rows or MaxTime elapsed; stale-claim recovery (`claimed_at < now - 60s`)

## Track D — Inline Drain in CLI Writes (cmd/centmem)
- [x] D.1 After successful write in `put`/`set`, drain the queue with `MaxTime=200ms`
        (goroutine or context-deadline call); CLI returns immediately with `status:"queued"`
- [x] D.2 Wire embedder + queue construction into command handlers (shared config/model path)
- [x] D.3 Verify write path p95 stays < 50 ms (drain must not block the response)
- [x] D.4 Document crash-safety behavior: process death mid-drain recovers next run via stale-claim

## Track E — Vector Search (internal/search/semantic.go)
- [x] E.1 Implement `Semantic(ctx, q Query, top int) ([]Ranked, error)`:
        embed q.Text → vec0 nearest-neighbor (embedding MATCH ? ORDER BY distance LIMIT ?)
        → join back to memories → apply type/tags/since/until/agent filters
- [x] E.2 Score normalization: `score = 1 - (distance / maxDistance)` (or RRF-safe [0,1) mapping)
- [x] E.3 Handle empty query text / empty vector gracefully (return nil, nil)
- [x] E.4 Cache query embeddings by query hash (see Track F) to avoid re-embedding repeated recalls

## Track F — Embedding Cache (internal/embed/cache.go)
- [x] F.1 LRU keyed by `sha256(text)` → vector; size 256 entries (config)
- [x] F.2 Eviction via `container/list`
- [x] F.3 Integrate cache into the query-embedding path (recall + semantic)

## Track G — Hybrid Recall Upgrade (internal/search/search.go)
- [x] G.1 Upgrade `Recall` to run Keyword + Facts + Timeline + Semantic (parallel via
        errgroup where possible), fuse with RRF (k=60)
- [x] G.2 Apply post-filters, dedupe by id, slice top
- [x] G.3 Ensure `matched_by` includes `"semantic"` for semantically matched rows
- [x] G.4 Preserve Phase 1 behavior: no contract regression when no embeddings exist

## Track H — Benchmarks (internal/search/search_bench_test.go)
- [x] H.1 `BenchmarkRecall_Hybrid_10k` (10k memories with embeddings) — p50/p95/p99, allocs
- [x] H.2 `BenchmarkRecall_Keyword_Only_10k` — compare to hybrid
- [x] H.3 `BenchmarkEmbedQueue_Drain_64` — throughput (vec/s)
- [x] H.4 `BenchmarkEmbedQuery` — cold vs warm cache latency
- [x] H.5 Record results in `testdata/bench-results.txt`
- [x] H.6 Targets: Recall p95 < 300 ms @ 10k, < 800 ms @ 100k;
        Embed warm > 200 sentences/sec; Queue.Drain > 100 mems/sec

## Track I — Test Plan (validation) — see plan-phase-2.md §2.11
### Unit tests
- [x] I.1  TestEmbedder_Deterministic (embed) — same text → same vector
- [x] I.2  TestEmbedder_Dims — output len == Dims()
- [x] I.3  TestEmbedder_Finite — no NaN/Inf
- [x] I.4  TestEmbedder_Empty — empty string handled
- [x] I.5  TestEmbedder_Truncate — overlong text truncated, not crashed
- [x] I.6  TestQueue_Claim_Stale (embed) — stale claimed_at reclaimed
- [x] I.7  TestQueue_Drain_NoRows — returns 0 with no work
- [x] I.8  TestQueue_Drain_BatchLimit — respects BatchSize
- [x] I.9  TestQueue_Drain_WritesVec — memories_vec populated after drain
- [x] I.10 TestQueue_Drain_PartialFailure — one bad row doesn't block others
- [x] I.11 TestSemantic_NearestNeighbor (search) — returns expected top match
- [x] I.12 TestSemantic_FilterByScope / TestSemantic_FilterByType
- [x] I.13 TestRecall_HybridFuse — semantic-only matches appear
- [x] I.14 TestRecall_ParaphraseQuery — "how do we ship" → "deploy via GitHub Actions"
- [x] I.15 TestCache_Hit / TestCache_Eviction (embed) — model call count; LRU past 256

### Integration / e2e
- [x] I.16 TestE2E_ParaphraseRetrieval — write "deploy to fly.io"; recall "how do we ship?"
- [x] I.17 TestE2E_InlineDrain — write 50; stats.pending_embeddings → 0 within 5s
- [x] I.18 TestE2E_RecallIsFast — seed 10k; 100 recall calls; p95 < 300 ms
- [x] I.19 TestE2E_RecollisionAfterRestart — 5 queued rows; kill mid-drain; rerun drains
- [x] I.20 TestE2E_NoNetwork — embedder + recall work with network disabled

### Golden / CLI
- [x] I.21 TestCLI_Recall_SemanticMatched — update M1 golden to allow matched_by "semantic";
        add golden for paraphrase scenario

## Checklist (Definition of Done — from plan-phase-2.md §2.10)
- [x] Embedder loads `bge-small-en-v1.5.onnx` from `~/.centmem/models/`
- [x] `Embed` deterministic, thread-safe
- [x] `m0002_vec.sql` applies on Open with vec0 verified
- [x] `memories_vec` populated for every embedded memory
- [x] Queue drain concurrent-safe (stale-claim recovery tested)
- [x] Inline drain runs in put/set without blocking CLI > 50 ms
- [x] `Recall` returns semantically relevant results
- [x] Cache reduces repeated query latency
- [x] All Phase 1 tests still pass (no contract regression)
- [x] Benchmarks recorded; targets met
- [x] `go build ./...` + `go vet ./...` clean
- [x] `go test ./... -race` green
- [x] Coverage ≥ 70% on internal/{store,scope,search,embed}

## Recommended Implementation Order (from plan-phase-2.md §2.9)
1. internal/embed/embed.go (real model load + Embed)
2. internal/embed/queue.go (drain + claim)
3. m0002_vec.sql migration + apply on Open
4. internal/search/semantic.go (vec0 query + join + filter)
5. Recall upgraded to hybrid
6. put/set hook to drain queue
7. internal/embed/cache.go
8. Benchmarks + tuning
