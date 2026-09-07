# Architecture & Scoping — centmem

This reference explains the internal architecture, scope resolution rules, and search fusion engine of `centmem`.

## System Overview

```
AI Agent (any harness) ──► centmem CLI (Go) ──► SQLite (WAL) + sqlite-vec + FTS5
                                                   └── local ONNX model (BGE-small 384d)
```

- **Local-First & Offline**: 100% on-device. No telemetry, no external API calls at runtime.
- **Embedded Database**: Single SQLite database file at `~/.centmem/centmem.db` with permissions `0600` in directory `0700`.
- **Fast Search**: p95 read < 300 ms across 100k memories; p95 write overhead < 50 ms.

---

## Scoping Grammar & Inheritance

cent-mem uses a 4-tier hierarchical scope model:

```
global
  └── project:<project-name>
        └── project:<project-name>/agent:<agent-name>
              └── project:<project-name>/agent:<agent-name>/session:<session-id>
```

### Path Syntax
- Identifiers must match `[a-z0-9-_.]+` (case-insensitive, normalized to lowercase).
- Examples:
  - `global`
  - `project:cent-mem`
  - `project:cent-mem/agent:claude`
  - `project:cent-mem/agent:antigravity/session:sess-9481`

### Inheritance Rules
1. **Ancestry Walk (Reads)**:
   - When querying `project:cent-mem/agent:claude`, the default `--inherit` flag walks up the tree to include memories in `project:cent-mem` and `global`.
   - To disable ancestor inheritance, pass `--no-inherit`.
2. **Descendant Search (Recall)**:
   - When searching at `project:cent-mem`, pass `--children` to also include all sub-agents and session memories beneath that project.
3. **Write Scoping**:
   - Writes always require an explicit `--scope`.
   - A write creates any non-existent ancestor scope records automatically.

---

## Hybrid Search Engine (Two-Stage Retrieval & Importance Scoring)

`centmem recall` implements a high-precision, two-stage retrieval pipeline combining multi-signal candidate generation, calibrated composite re-ranking, contextual affinity boosts, and dynamic importance scoring.

```
                    ┌─────────────────────────────────┐
                    │  Query Expansion (Tags/Context) │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │   Stage 1: Multi-Signal Search   │
                    │   (FTS5 + Vec Cosine + Timeline)│
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │  Stage 1 RRF Fusion & Filtering │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │  Stage 2: Composite Re-Ranking  │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │ Session & Agent Proximity Boost │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
            ┌─────────────────────────────────────────────────┐
            │        Importance Boost Multiplier (v1.5.0)     │
            │  importance = min(2.0, 1.0 + ln(1 + count)*0.1) │
            │                score *= importance              │
            └────────────────────────┬────────────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │    Recency Decay (if enabled)   │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │ Near-Dup Collapse & Top Slice   │
                    └────────────────┬────────────────┘
                                     │
                  ┌──────────────────┴──────────────────┐
                  │                                     │
                  ▼ (synchronous return)                ▼ (asynchronous background)
      Return Top-N JSON to caller             RecordAccessAsync Channel
      {"id": 42, "access_count": 14, ...}                │
                                                         ▼
                                              Batch Update SQLite:
                                              UPDATE memories
                                              SET access_count = access_count + 1,
                                                  last_accessed_at = ?
                                              WHERE id IN (...)
```

### Stage 1: Multi-Signal Candidate Generation & RRF Fusion
Four parallel search pipelines retrieve candidate memories:
1. **Semantic Vector Search**: Generates 384-dimensional dense vectors using local BGE-small-en-v1.5 via ONNX Runtime, computing cosine distance via `sqlite-vec`.
2. **Keyword Full-Text Search (FTS5)**: Tokenizes queries with SQLite FTS5 using the Porter stemmer and BM25 ranking.
3. **Exact Fact Matcher**: Evaluates exact key/value fact names, string values, and attached tags.
4. **Timeline Recency Scorer**: Generates candidate matches from recent chronological activity.

Candidates are fused using **Reciprocal Rank Fusion (RRF, $k=60$)**:
$$RRF(m) = \sum_{r \in Rankers} \frac{1}{60 + rank_r(m)}$$

This ensures balanced retrieval: memories that match either semantic intent or exact technical tokens are promoted without score skew.

### Stage 2: Composite Re-Ranking Pipeline
Stage 1 RRF produces a broad candidate pool. The Stage 2 re-ranker evaluates the top candidate window (configurable via `search.rerank_window`, default `30`) using in-process lexical and semantic feature scoring:
- **Dense Semantic Similarity ($S_{\text{sem}}$)**: Exact cosine similarity of query and content embeddings.
- **Lexical Token Coverage ($S_{\text{lex}}$)**: Fraction of query tokens present in the target content and tags.
- **Exact Phrase Bonus ($S_{\text{phrase}}$)**: Multiplier boost for contiguous substring matches.
- **Stage 1 Rank Signal ($S_{\text{rrf\_norm}}$)**: Preserves initial multi-signal fusion consensus.

#### Contextual Proximity & Affinity Boosts
Immediately following composite scoring:
- **Session Proximity Boost**: 1.25x (+25%) multiplier for memories created within the current active session scope.
- **Agent Affinity Boost**: 1.15x (+15%) multiplier for memories authored by the caller agent (matched via `--caller-agent` or `$CENTMEM_AGENT`).

### Access Frequency Importance Scoring
To reinforce memories that prove practically valuable to AI agents over time, centmem applies a sub-linear importance multiplier based on historical access count:

$$\text{importance}(c) = \min\left(2.0,\, 1.0 + \ln(1 + c) \times 0.1\right)$$

where $c = \text{access\_count} \ge 0$.

The candidate score is scaled directly:
$$\text{score} \leftarrow \text{score} \times \text{importance}(c)$$

#### Progression Curve
| Access Count ($c$) | $\ln(1 + c)$ | Multiplier | Boost (%) | Semantic Meaning |
|---|---|---|---|---|
| **0** | 0.000 | **1.000×** | +0.0% | Fresh / unaccessed memory (neutral baseline) |
| **1** | 0.693 | **1.069×** | +6.9% | Recalled once |
| **3** | 1.386 | **1.139×** | +13.9% | Moderately reinforced |
| **7** | 2.079 | **1.208×** | +20.8% | Established team pattern |
| **15** | 2.773 | **1.277×** | +27.7% | Frequently consulted decision |
| **35** | 3.584 | **1.358×** | +35.8% | Core architectural pillar |
| **100** | 4.615 | **1.462×** | +46.2% | Heavily referenced standard |
| **1,000** | 6.909 | **1.691×** | +69.1% | Universal convention |
| **$\ge 22,025$** | $\ge 10.0$ | **2.000×** | +100.0% | Hard ceiling cap ($2.0\times$) |

#### Core Properties & Guarantees
1. **Sub-linear Diminishing Returns**: Early recalls provide immediate reinforcement (+6.9% on first access), but the natural logarithmic curve mathematically bounds runaway score inflation.
2. **Bounded Ceiling ($2.0\times$)**: A hard cap prevents high-frequency memories from overpowering direct lexical or semantic query relevance on mismatched searches.
3. **Zero-Starvation Guarantee**: Unaccessed memories start at $1.0\times$ baseline. A fresh, highly relevant memory ($> 0.85$ semantic similarity) will readily outrank an irrelevant older memory despite high historical access counts.
4. **Recency Decay**: When configured (`search.decay_half_life_days > 0`), exponential decay is applied after importance boosting.

### Asynchronous Feedback Loop (Agent Recall Reinforcement)
When `centmem recall` finishes ranking and slices the top-$N$ winning results:
1. The JSON response containing `access_count` and `last_accessed_at` is returned synchronously to the caller with zero latency overhead.
2. A non-blocking background goroutine (`RecordAccessAsync`) queues the returned memory IDs over an asynchronous channel.
3. The store batches an atomic SQLite update:
   ```sql
   UPDATE memories
   SET access_count = access_count + 1,
       last_accessed_at = ?
   WHERE id IN (...)
   ```
4. Over time, memories actively recalled across agent workflows naturally float higher in future queries, while unused memories remain at baseline until pruned or summarized by compaction.
