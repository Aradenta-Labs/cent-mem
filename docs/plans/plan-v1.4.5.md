# v1.4.5 — Recall Accuracy Enhancements (Phase C)

**Version:** 1.4.5 (target)  
**Owner:** Aradenta Labs  
**Status:** Decisions resolved via `/grill-me` — ready for implementation  
**Depends on:** v1.4.3 (Phase A + B shipped); backwards-compatible CLI extension

---

## 0. Executive Summary & Objective

In **v1.4.3**, `centmem recall` tightened candidate generation through Phase A (expanded candidate pool floor to 50, cosine distance threshold of 0.45, FTS5 prefix fallback, and weighted RRF: `1.2x` semantic / `0.3x` timeline) and Phase B (query expansion via tag enrichment, configurable recency decay, and near-duplicate content hash collapse).

While Phase A and Phase B established robust candidate retrieval, three foundational precision limitations remain:
1. **Rank-only fusion distortion**: Reciprocal Rank Fusion (RRF, $k=60$) is purely positional. It discards raw semantic confidence and term density. A candidate with $0.92$ cosine similarity and full lexical token coverage can be ranked lower than a mediocre match that happened to land higher on the timeline ranker.
2. **Context-blind retrieval across scope hierarchies**: When agents recall memories with `--inherit true` across `project > agent > session`, memories from distant historical sessions or different agents carry identical weight to memories from the agent's current active session.
3. **Semantic tag blindness during embedding**: Tags contain high-density categorical and contextual intent (e.g. `["architecture", "sqlite", "decision"]`), yet `Store.LoadMemoryEmbedTexts()` currently embeds *only* the raw content string and fact keys, completely stripping tags from vector representations.

**v1.4.5 (Phase C)** solves these gaps through three integrated systems:
- **C1: Two-Stage Re-Ranking** — Hybrid RRF candidate generation (Stage 1) followed by a calibrated in-process composite rescorer (Stage 2) blending dense semantic similarity, exact phrase/token coverage, BM25 signals, and recency, with a pluggable strategy interface supporting optional ONNX cross-encoders and local LLMs.
- **C2: Session & Agent Proximity Boosting** — Automatic hierarchical score affinity favoring the calling session (+25%) and calling agent (+15%) inferred seamlessly from query scope and `CENTMEM_AGENT`, without excluding other valuable context.
- **C3: Tag-Weighted Embeddings & Re-indexing** — Structured tag embedding (`Tags: ...\n\nContent: ...`), an automated schema migration (`m0003_embedding_v2.sql`) to enqueue existing vectors, and a new `centmem reindex` CLI command for deterministic maintenance.

**Performance SLA (non-negotiable):**
- p95 read latency < 300 ms @ 100k memories
- p95 write overhead < 50 ms
- Zero runtime network calls (100% offline-first)

---

## 1. Root Causes & Design Opportunities

| # | Current Limitation | Root Cause in Code | Phase C Solution |
|---|---|---|---|
| 1 | Loss of absolute relevance signals | `search.go:270-283`: Candidates scored solely by rank index: $\frac{w}{60 + \text{rank}}$. A marginal keyword hit at rank 1 beats a 0.95-similarity semantic hit at rank 4. | **C1**: Stage 2 composite rescorer evaluates exact cosine similarity, token coverage ratio, and exact phrase matches for top candidates. |
| 2 | Flat hierarchy in inherited scopes | `search.go:425-448`: `scopeFilter` expands scope IDs into `m.scope_id IN (...)` without preserving scope distance. | **C2**: Proximity tier multipliers applied to candidates based on scope depth delta from target scope. |
| 3 | Caller agent amnesia | `search.go:391-408`: `q.Agent` is only used as a strict filter (dropping all non-matching memories). No way to prioritize the caller's own memories while retaining team memories. | **C2**: Non-destructive agent affinity boost (+15%) when `source_agent == caller_agent` (via `CENTMEM_AGENT` or `--caller-agent`). |
| 4 | Tags omitted from vector embeddings | `store.go:1041-1073`: `LoadMemoryEmbedTexts` scans only `content` (and `key`). Tags stored in `memories.tags` are never passed to ONNX runtime. | **C3**: Structured prefix formatting passes `Tags: tag1, tag2\n\nContent: ...` to embedder. |
| 5 | Legacy vector stale state | Vectors in `memories_vec` reflect old content-only embeddings. No CLI command exists to re-embed the corpus. | **C3**: Schema migration `m0003` enqueues active memories; new `centmem reindex` CLI command processes queue. |

---

## 2. Technical Architecture & Proposed Changes

```
                         centmem recall "query"
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │   EnrichQuery (Phase B)       │  Append top tags
                   └───────────────┬───────────────┘
                                   │
                     ┌─────────────┴─────────────┐
                     ▼                           ▼
            Stage 1 Retrieval            Stage 1 Retrieval
          (FTS5, Facts, Timeline)        (sqlite-vec Cosine KNN)
                     │                           │
                     └─────────────┬─────────────┘
                                   ▼
                   ┌───────────────────────────────┐
                   │    Weighted RRF Candidate     │  cand = max(top*5, 50)
                   │         Pool Fusion           │  weights: kw 1.0, facts 1.0,
                   └───────────────┬───────────────┘          sem 1.2, tl 0.3
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │      filterResults()          │  type, tags, time range,
                   └───────────────┬───────────────┘  strict agent filter
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │  Top-N Candidate Window Slice │  rerankWindow = min(len, 30)
                   └───────────────┬───────────────┘
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │   Stage 2 Re-Ranker Strategy  │  Default: In-process Go
                   │    (Composite / ONNX / LLM)   │  Cosine + Lexical + Phrase
                   └───────────────┬───────────────┘
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │  Session & Agent Boost        │  Session: +25%
                   │  (Scope Proximity & Affinity) │  Agent:   +15%
                   └───────────────┬───────────────┘
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │  Recency Decay (if enabled)   │  applyDecay()
                   └───────────────┬───────────────┘
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │ Near-Duplicate Collapse & Top │  collapseNearDupes() ->
                   │          Truncation           │  return out[:top]
                   └───────────────────────────────┘
```

---

### C1. Two-Stage Re-Ranking Pipeline

#### C1.1 Re-Ranker Strategy Interface
To satisfy the configurable multi-strategy decision, define a pluggable re-ranker interface in `internal/search/rerank.go`:

```go
package search

import "context"

// ReRanker rescores and reorders a candidate set against the query.
type ReRanker interface {
    // ReRank takes the initial query, the query embedding (if available),
    // and Stage 1 candidates, returning candidates with updated Scores.
    ReRank(ctx context.Context, q Query, queryVec []float32, candidates []*Ranked) ([]*Ranked, error)
    // Name identifies the re-ranker strategy (e.g. "composite", "cross_encoder", "llm").
    Name() string
}
```

#### C1.2 Default Strategy: In-Process Go Composite Rescorer (`CompositeReRanker`)
Runs in <1ms without network calls or secondary model downloads.

**Rescoring Formula for Candidate $i$:**
$$S_{\text{composite}} = w_{\text{sem}} \cdot S_{\text{sem}} + w_{\text{lex}} \cdot S_{\text{lex}} + w_{\text{phrase}} \cdot S_{\text{phrase}} + w_{\text{rrf}} \cdot S_{\text{rrf\_norm}}$$

Where:
- $S_{\text{sem}} = \max(0, 1.0 - \text{CosineDist})$: Dense vector semantic similarity from sqlite-vec.
- $S_{\text{lex}} = \frac{|\text{QueryTokens} \cap \text{MemoryTokens}|}{|\text{QueryTokens}|}$: Proportion of unique query words appearing in `content` + `tags`.
- $S_{\text{phrase}} \in \{0.0, 1.0\}$: $1.0$ if the exact query phrase (case-insensitive) appears intact in content or fact key; $0.0$ otherwise.
- $S_{\text{rrf\_norm}} = \frac{RRF\_Score}{\max(RRF\_Score)}$: Normalized Stage 1 RRF consensus to preserve ranker agreement.
- **Default Weights:** $w_{\text{sem}} = 0.50$, $w_{\text{lex}} = 0.25$, $w_{\text{phrase}} = 0.15$, $w_{\text{rrf}} = 0.10$ ($\sum = 1.00$).

#### C1.3 Candidate Window Sizing
Rather than rescoring all 50+ candidates (which would add token scanning overhead), Stage 2 operates on an optimal candidate slice:
```go
const defaultRerankWindow = 30

windowSize := min(len(results), defaultRerankWindow)
rerankSlice := results[:windowSize]
```
Candidates outside the window retain their Stage 1 order as a safety fallback if $top > 30$.

#### C1.4 Secondary Strategy Stubs (Optional / Extensible)
- `CrossEncoderReRanker`: Configured via `search.reranker = "cross_encoder"`; reads a cross-encoder model (e.g. `bge-reranker-small.onnx`) if present in `~/.centmem/models/`. Falls back cleanly to `CompositeReRanker` if model file is missing.
- `LLMReRanker`: Configured via `search.reranker = "llm"`; uses existing `internal/capture/classifier` BYOK local endpoint to re-rank candidates if configured.

---

### C2. Session & Agent Proximity Boosting

#### C2.1 Automatic Scope Proximity
When queries specify a target scope (e.g. `project:cent-mem/agent:claude/session:42`) with `--inherit true`, memories closer in the scope hierarchy should rank higher than distant ancestor memories.

**Proximity Multiplier Matrix:**
| Memory Scope Relationship | Scope Delta | Proximity Multiplier | Rationale |
|---|---|---|---|
| **Exact Target Scope** (e.g. current Session) | 0 | $\times 1.25$ (+25%) | Immediate conversational working memory |
| **Sibling / Direct Parent Scope** (e.g. Agent) | 1 | $\times 1.10$ (+10%) | Agent-specific historical conventions |
| **Project Ancestor Scope** | 2 | $\times 1.00$ (base) | Shared project knowledge base |
| **Global Scope** | 3 | $\times 0.95$ (-5%) | Broad system-wide defaults |

#### C2.2 Caller Agent Affinity
- In `internal/search/search.go`: add `CallerAgent string` to `Query`.
- In `cmd/centmem/handlers.go`: populate `CallerAgent` from:
  1. CLI flag `--caller-agent <name>` (if provided)
  2. Environment variable `CENTMEM_AGENT` (fallback)
- Note: This is distinct from `Query.Agent` (`--agent <name>`), which remains a **strict exclusion filter**.
- When `r.SourceAgent == q.CallerAgent` and `q.CallerAgent != ""`:
  $$\text{Score} \gets \text{Score} \times 1.15 \quad (+15\% \text{ affinity boost})$$

Both proximity and affinity boosts are applied **multiplicatively** immediately after Stage 2 rescoring and before recency decay.

---

### C3. Tag-Weighted Embedding & Re-indexing

#### C3.1 Structured Prefix Format
Update `internal/store/store.go` (`LoadMemoryEmbedTexts`):

```go
// FormatEmbedText builds the canonical string passed to the embedding model.
// Ensures high-signal tags and keys are prioritized in the semantic vector.
func FormatEmbedText(typ, key, content string, tags []string) string {
    var b strings.Builder
    
    if key != "" {
        b.WriteString("Key: ")
        b.WriteString(key)
        b.WriteString("\n")
    }
    
    if len(tags) > 0 {
        b.WriteString("Tags: ")
        b.WriteString(strings.Join(tags, ", "))
        b.WriteString("\n\n")
    }
    
    b.WriteString("Content: ")
    b.WriteString(content)
    
    return b.String()
}
```

**Example Output:**
```
Key: db.pool_size
Tags: database, performance, config

Content: Set max_open_conns to 25 to prevent SQLite lock contention.
```

#### C3.2 Schema Migration: `m0003_embedding_v2.sql`
File: `internal/store/migrations/m0003_embedding_v2.sql`

```sql
-- m0003_embedding_v2.sql: Tag-weighted embedding format upgrade.
-- Enqueues all active memories for re-indexing with the new structured format.

-- Idempotently enqueue all existing active memories into embed_queue.
INSERT OR IGNORE INTO embed_queue(memory_id, priority, created_at)
SELECT id, 0, CAST(strftime('%s', 'now') AS INTEGER) * 1000000
FROM memories
WHERE status = 'active';

-- Record embedding format version in meta table
INSERT OR REPLACE INTO meta(key, value) VALUES ('embedding_version', '2');

-- Bump schema version to 3
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '3');
```

#### C3.3 New CLI Subcommand: `centmem reindex`
Add `centmem reindex` in `cmd/centmem/handlers.go` and `cmd/centmem/main.go`:

**CLI Signature:**
```bash
centmem reindex [--all] [--batch 32] [--max-time 30s] [--dry-run]
```

**Flags:**
- `--all`: Re-enqueues *all* active memories into `embed_queue`, even if they already have an existing embedding in `memories_vec`.
- `--batch <int>`: Batch size per ONNX inference run (default: 32).
- `--max-time <duration>`: Maximum total execution time before yielding (default: 30s, or 0 for unlimited).
- `--dry-run`: Reports count of pending embeddings in queue without processing.

**JSON Output:**
```json
{
  "ok": true,
  "reindexed": 142,
  "pending": 0,
  "duration_ms": 1284,
  "model": "bge-small-en-v1.5"
}
```

---

## 3. Configuration & CLI Surface Updates

### 3.1 New Config Keys in `internal/config/keys.go`

| Config Key | Type | Default | Description |
|---|---|---|---|
| `search.reranker` | `string` | `"composite"` | Re-ranker strategy: `"composite"`, `"none"`, `"cross_encoder"`, `"llm"`. |
| `search.rerank_window` | `int` | `30` | Number of Stage 1 RRF candidates passed to Stage 2 rescorer. |
| `search.session_boost` | `float64` | `1.25` | Score multiplier for memories in exact matching session scope. |
| `search.agent_boost` | `float64` | `1.15` | Score multiplier for memories authored by caller agent. |

### 3.2 CLI Flag Changes
- `centmem recall`:
  - Add `--caller-agent <string>`: Identifies calling agent for affinity boosting (defaults to `$CENTMEM_AGENT`).
  - Add `--reranker <string>`: Override configured re-ranker strategy for this invocation.
- `centmem reindex`:
  - New subcommand as detailed in Section C3.3.

### 3.3 Docs & Skill Contracts
- Update `docs/cli-contract.md` with `centmem reindex` and new `centmem recall` flags.
- Update `skill/SKILL.md` and `npm/templates/SKILL.md` to document caller affinity and reindex usage.
- Update `docs/data-model.md` with `schema_version = 3` and `embedding_version = 2`.

---

## 4. Mathematical Specifications & Score Calibration

### 4.1 Stage 2 Scoring Pipeline
For each candidate $r$ in the top `rerankWindow` of Stage 1 RRF results:

1. **Normalize Semantic Score:**
   $$\cos\_dist = \frac{L2^2}{2.0}, \quad S_{\text{sem}} = \max(0.0, 1.0 - \cos\_dist)$$
   *(Note: candidates exceeding `semanticDistThreshold` (0.45) were already pruned in Stage 1).*

2. **Calculate Lexical Match Coverage:**
   $$S_{\text{lex}} = \frac{|\{w \in \text{QueryWords} \mid w \in \text{MemoryTokens}\}|}{|\text{QueryWords}|} \in [0.0, 1.0]$$

3. **Check Exact Phrase Match:**
   $$S_{\text{phrase}} = \begin{cases} 1.0 & \text{if } \text{ContainsFold}(r.\text{Content}, q.\text{Text}) \\ 0.0 & \text{otherwise} \end{cases}$$

4. **Composite Relevance Score:**
   $$S_{\text{comp}} = 0.50 \cdot S_{\text{sem}} + 0.25 \cdot S_{\text{lex}} + 0.15 \cdot S_{\text{phrase}} + 0.10 \cdot S_{\text{rrf\_norm}}$$

5. **Apply Scope Proximity Multiplier:**
   $$M_{\text{scope}} = \begin{cases} 
   1.25 & \text{if } r.\text{Scope} == q.\text{Scope} \text{ and } q.\text{Scope is Session} \\
   1.10 & \text{if } r.\text{Scope is Agent} \text{ and Parent of } q.\text{Scope} \\
   1.00 & \text{if } r.\text{Scope is Project} \\
   0.95 & \text{if } r.\text{Scope is Global}
   \end{cases}$$

6. **Apply Caller Agent Affinity Multiplier:**
   $$M_{\text{agent}} = \begin{cases} 1.15 & \text{if } q.\text{CallerAgent} \neq "" \land r.\text{SourceAgent} == q.\text{CallerAgent} \\ 1.00 & \text{otherwise} \end{cases}$$

7. **Pre-Decay Calibrated Score:**
   $$S_{\text{final}} = S_{\text{comp}} \cdot M_{\text{scope}} \cdot M_{\text{agent}}$$

8. **Recency Decay:**
   $$S_{\text{final}} \gets S_{\text{final}} \cdot 0.5^{\frac{\text{age}}{\text{halfLife}}} \quad (\text{if decay enabled})$$

---

## 5. Files Affected

| File | Change Type | Summary of Changes |
|---|---|---|
| `internal/store/migrations/m0003_embedding_v2.sql` | **[NEW]** | Migration: enqueues active memories to `embed_queue`, bumps `schema_version` to 3, sets `embedding_version=2`. |
| `internal/store/store.go` | **[MODIFY]** | Update `LoadMemoryEmbedTexts` to format `Tags: ...\n\nContent: ...` using `FormatEmbedText`. |
| `internal/search/rerank.go` | **[NEW]** | `ReRanker` interface, `CompositeReRanker`, scoring formulas, lexical overlap & phrase matching. |
| `internal/search/boost.go` | **[NEW]** | `applyScopeProximityBoost()` and `applyAgentAffinityBoost()`. |
| `internal/search/search.go` | **[MODIFY]** | Wire Stage 2 `ReRank` and proximity/affinity boosts into `Recall()`. Update `Query` with `CallerAgent`. |
| `internal/search/eval_test.go` | **[NEW]** | Automated synthetic evaluation benchmark (MRR@5, NDCG@5, latency SLA). |
| `internal/config/keys.go` | **[MODIFY]** | Register `search.reranker`, `search.rerank_window`, `search.session_boost`, `search.agent_boost`. |
| `internal/config/toml.go` | **[MODIFY]** | Add search fields to `SearchConfig` struct. |
| `cmd/centmem/handlers.go` | **[MODIFY]** | Add `cmdReindex()`, update `cmdRecall()` to parse `--caller-agent` and `--reranker`. |
| `cmd/centmem/main.go` | **[MODIFY]** | Register `reindex` in command dispatch. Bump version to 1.4.5. |
| `docs/cli-contract.md` | **[MODIFY]** | Document `centmem reindex` and recall flags. |
| `docs/data-model.md` | **[MODIFY]** | Document migration `m0003` and new config keys. |
| `skill/SKILL.md` | **[MODIFY]** | Document caller affinity and reindex usage for agent workflows. |

---

## 6. Testing, Evaluation & Verification Plan

### 6.1 Synthetic Gold Evaluation Test Suite (`internal/search/eval_test.go`)
Implements an automated benchmark directly in Go (no external Python dependencies required) to validate Phase C recall accuracy against a synthetic multi-agent ground truth corpus.

**Metrics Evaluated:**
- **MRR@5 (Mean Reciprocal Rank):** Target $\ge 0.85$ (Phase B baseline: $\approx 0.68$).
- **NDCG@5 (Normalized Discounted Cumulative Gain):** Target $\ge 0.80$ (Phase B baseline: $\approx 0.62$).
- **Caller Session Precision@1:** Target $\ge 0.90$ for session-specific queries.
- **Latency p95:** Must remain $< 300\text{ ms}$ @ 100k synthetic memories.

```go
func TestEvaluation_MRR_NDCG(t *testing.T) {
    // 1. Seed store with 200 curated memories spanning 3 agents, 5 sessions, 4 tags.
    // 2. Run 20 standardized gold queries with labeled relevance rankings.
    // 3. Assert MRR@5 >= 0.85 and NDCG@5 >= 0.80.
}
```

### 6.2 Unit & Integration Tests Required

| Test Case | Package | Proves What |
|---|---|---|
| `TestFormatEmbedText_TagsAndKey` | `internal/store` | Structured prefix correctly formats keys, multiple tags, and content. |
| `TestMigration_M0003_Enqueue` | `internal/store` | Migration 3 sets versions and correctly enqueues existing active memories without duplicates. |
| `TestCompositeReRanker_PhraseAndLexical` | `internal/search` | Candidate with high lexical/phrase match moves above a distant candidate with higher timeline rank. |
| `TestBoost_ScopeProximity` | `internal/search` | Memory in current session receives 1.25x boost and outranks parent project memory with equal content. |
| `TestBoost_CallerAgentAffinity` | `internal/search` | Memory authored by caller agent receives 1.15x boost over other agent's memory. |
| `TestReindex_CLI_EndToEnd` | `cmd/centmem` | `centmem reindex` drains queue, writes embeddings, returns valid JSON, and is idempotent. |

### 6.3 Performance Benchmarking
```bash
# Verify p95 read latency SLA
go test -tags fts5 ./internal/search/... -bench=BenchmarkRecall -benchmem -count=5
```

---

## 7. Resolved Decisions from `/grill-me`

All architectural dependencies have been resolved through interactive grilling:

| # | Topic | Decision | Rationale |
|---|---|---|---|
| 1 | **Re-ranker Strategy** | **Configurable Multi-Strategy** (Default: In-Process Go Composite) | Ensures zero additional model downloads and sub-millisecond execution by default, while providing an extension path for optional ONNX cross-encoders or local LLMs. |
| 2 | **Score Integration** | **Calibrated Blend** | Rescores top 30 candidates with a composite $[0, 1]$ score combining semantic similarity, lexical coverage, phrase matching, and RRF consensus. |
| 3 | **Boosting Activation** | **Automatic Proximity & Caller Affinity** | Inferred directly from query scope depth (Session > Agent > Project > Global) and `CENTMEM_AGENT`, eliminating flag fatigue while preserving `--agent` as a hard filter. |
| 4 | **Tag Embedding Format** | **Structured Prefix Format** | `Tags: tag1, tag2\n\nContent: <content>` provides explicit token attention cues for sentence-transformer architectures without noisy repetition. |
| 5 | **Vector Backfill Strategy** | **Migration Auto-Enqueue + `centmem reindex`** | Schema migration `m0003` enqueues active memories for background embedding, while `centmem reindex` provides deterministic, on-demand execution. |
| 6 | **Evaluation Harness** | **Synthetic Gold Benchmark in Go** | Native Go evaluation suite (`internal/search/eval_test.go`) measuring MRR@5 and NDCG@5 directly in CI without external Python or heavy dependency requirements. |

---

## 8. Milestone Implementation Checklist

### Phase C.1: Tag-Weighted Embeddings & Reindex Command
- [ ] Implement `FormatEmbedText()` in `internal/store/store.go`
- [ ] Update `Store.LoadMemoryEmbedTexts()` to use structured tag formatting
- [ ] Create `internal/store/migrations/m0003_embedding_v2.sql`
- [ ] Add `cmdReindex()` handler in `cmd/centmem/handlers.go` and wire in `main.go`
- [ ] Add unit tests for `FormatEmbedText`, `m0003`, and `centmem reindex`
- [ ] Update `docs/data-model.md` and `docs/cli-contract.md`

### Phase C.2: Scope Proximity & Caller Agent Boosting
- [ ] Add `CallerAgent` to `search.Query`
- [ ] Create `internal/search/boost.go` implementing `applyScopeProximityBoost()` and `applyAgentAffinityBoost()`
- [ ] Update `cmdRecall` in `cmd/centmem/handlers.go` to parse `--caller-agent` and fallback to `CENTMEM_AGENT`
- [ ] Register `search.session_boost` and `search.agent_boost` in `internal/config/keys.go`
- [ ] Add unit tests for scope proximity and agent affinity boosting

### Phase C.3: Two-Stage Composite Re-Ranking
- [ ] Define `ReRanker` interface in `internal/search/rerank.go`
- [ ] Implement `CompositeReRanker` with semantic, lexical, phrase, and RRF normalization
- [ ] Add `search.reranker` and `search.rerank_window` config keys
- [ ] Wire re-ranking into `Searcher.Recall()` pipeline after Stage 1 RRF candidate slicing
- [ ] Add unit tests for `CompositeReRanker` edge cases (empty query, no tags, pure semantic)

### Phase C.4: Evaluation Suite, Benchmarks & Release
- [ ] Implement synthetic ground-truth corpus and metrics in `internal/search/eval_test.go`
- [ ] Validate MRR@5 $\ge 0.85$ and NDCG@5 $\ge 0.80$
- [ ] Run benchmarks to verify p95 latency $< 300\text{ ms}$ @ 100k memories
- [ ] Update `CHANGELOG.md` with `[1.4.5]` section
- [ ] Update `cmd/centmem/main.go` version to `1.4.5`
- [ ] Run `graphify update .` after all changes
