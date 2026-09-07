# v1.5.0 — Search & Recall: Importance Scoring

**Version:** 1.5.0 (target)  
**Owner:** Aradenta Labs  
**Status:** Approved — Ready for implementation  
**Depends on:** v1.4.4 (Phase C Two-Stage Re-Ranking shipped); backwards-compatible CLI and schema extension  
**Roadmap Reference:** [docs/plans/roadmap-v1.5.x.md](roadmap-v1.5.x.md)

---

## 0. Executive Summary & Objective

In **v1.4.4**, `centmem recall` introduced the Phase C retrieval pipeline: two-stage re-ranking (Stage 1 candidate generation fused via weighted RRF, followed by Stage 2 composite re-ranking), session and agent proximity boosting, tag-weighted embeddings (`m0003_embedding_v2.sql`), and recency decay.

However, all memories remain equal in baseline weight regardless of how frequently or recently they have proven useful to agents. A critical decision or convention recalled 60 times across multiple agent sessions carries the exact same foundational weight as a one-off note written six months ago that has never been retrieved since.

**v1.5.0 (Search & Recall: Importance Scoring)** introduces an access-frequency importance feedback loop into the retrieval pipeline:
1. **Access Tracking:** Schema migration `m0004_importance.sql` adds `access_count` and `last_accessed_at` columns to `memories`.
2. **Asynchronous Non-Blocking Recording:** Every `recall` execution records access for its returned memories asynchronously in a background write queue, imposing 0 ms read latency penalty.
3. **Logarithmic Score Calibration:** Stage 2 retrieval applies an importance multiplier:
   $$\text{importance} = \min\left(2.0,\, 1.0 + \ln(1 + \text{access\_count}) \times 0.1\right)$$
   Memories recalled repeatedly receive a compounding, bounded rank boost without starving newly introduced memories.
4. **Visibility & Diagnostics:** `centmem recall` surfaces `access_count` per item; `centmem stats` exposes an `importance_distribution` breakdown; the Web UI displays access frequency badges in the memory table and detail drawer.

**Performance SLA (non-negotiable):**
- p95 read latency < 300 ms @ 100k memories
- p95 write overhead < 50 ms
- Non-blocking access writes: 0 ms added to recall response time
- 100% offline-first; zero network calls

---

## 1. Problem Statement & Root Causes

| # | Current Limitation | Root Cause in Code | v1.5.0 Solution |
|---|---|---|---|
| 1 | Useful memories decay uniformly with irrelevant ones | `search.go:395-402`: Recency decay calculates age solely from `created_at`. A 90-day-old decision that was recalled yesterday decays at the same rate as an abandoned 90-day-old note. | Track `last_accessed_at` and `access_count`; apply importance boost that counteracts stale decay for active memories. |
| 2 | Purely static relevance scoring | `search.go:334-360`: Retrieval relies solely on lexical match, semantic cosine distance, and proximity. No behavioral feedback loop informs ranking. | Dynamic importance multiplier based on sub-linear access count: $\min(2.0, 1.0 + \ln(1 + N) \times 0.1)$. |
| 3 | Lack of access telemetry | `internal/store/types.go:7-23`: `Memory` struct lacks access metadata. Agents and developers cannot discern which memories are actively referenced. | Schema migration `m0004_importance.sql` adds `access_count` and `last_accessed_at`. Surfaced in CLI and Web UI. |
| 4 | Risk of write lock contention during recall | `cmd/centmem/handlers.go:337`: If `recall` executed synchronous SQLite updates to record access counts, concurrent recall queries would suffer WAL write-lock contention. | Detached, buffered channel/goroutine with batched atomic updates (`RecordAccess`). |

---

## 2. Technical Architecture & Data Flow

```
                      centmem recall "sqlite wal mode"
                                     │
                                     ▼
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
            │       IMPORTANCE BOOST (New in v1.5.0)          │
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
                  ▼ (synchronous)                       ▼ (async background)
      Return Top-N JSON to caller             Write to RecordAccess Channel
      {"id": 42, "access_count": 14, ...}                │
                                                         ▼
                                              Batch Update SQLite:
                                              UPDATE memories
                                              SET access_count = access_count + 1,
                                                  last_accessed_at = ?
                                              WHERE id IN (...)
```

---

## 3. Data Model & Schema Migrations

### Migration `internal/store/migrations/m0004_importance.sql`

```sql
-- m0004_importance.sql: Access frequency tracking and importance scoring.
-- Adds access_count and last_accessed_at to memories table.

ALTER TABLE memories ADD COLUMN access_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memories ADD COLUMN last_accessed_at INTEGER; -- unix microseconds

-- Create index on access_count for fast distribution statistics and sorting
CREATE INDEX IF NOT EXISTS idx_memories_access_count ON memories(access_count);
CREATE INDEX IF NOT EXISTS idx_memories_last_accessed_at ON memories(last_accessed_at);

-- Record schema version
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '4');
```

### Idempotency & Downgrade Safety
- `ALTER TABLE ... ADD COLUMN` in SQLite defaults to `0` and `NULL` without table rewrite.
- Read queries check column presence or fall back safely to `0` if opening an older database file.

---

## 4. Mathematical Specifications & Score Calibration

### Importance Multiplier Formula

$$\text{importance}(c) = \min\left(2.0,\, 1.0 + \ln(1 + c) \times 0.1\right)$$

where $c = \text{access\_count} \ge 0$.

### Progression Curve

| Access Count ($c$) | $\ln(1 + c)$ | Multiplier | Boost (%) | Description |
|---|---|---|---|---|
| **0** | 0.000 | **1.000×** | +0.0% | Fresh / unaccessed memory (neutral baseline) |
| **1** | 0.693 | **1.069×** | +6.9% | Recalled once |
| **3** | 1.386 | **1.139×** | +13.9% | Moderately reinforced |
| **7** | 2.079 | **1.208×** | +20.8% | Established team pattern |
| **15** | 2.773 | **1.277×** | +27.7% | Frequently consulted decision |
| **35** | 3.584 | **1.358×** | +35.8% | Core architectural pillar |
| **100** | 4.615 | **1.462×** | +46.2% | Heavily referenced standard |
| **1,000** | 6.909 | **1.691×** | +69.1% | Universal convention |
| **$\ge 22,025$** | $\ge 10.0$ | **2.000×** | +100.0% | Hard cap ceiling (strictly $2.0\times$) |

### Design Properties
1. **Sub-linear Diminishing Returns:** Early accesses provide immediate reinforcement (+6.9% on first recall), but runaway score inflation is mathematically bounded by the natural logarithm.
2. **Bounded Ceiling ($2.0\times$):** Even after tens of thousands of accesses, a memory can never overpower direct lexical or semantic query mismatches.
3. **Zero-Starvation Guarantee:** New memories start at $1.0\times$ multiplier. A highly relevant new memory with strong cosine similarity ($> 0.85$) will outrank an irrelevant older memory with high access count.

---

## 5. Configuration & CLI Surface Updates

### 5.1 Configuration (`config.toml`)

```toml
[search]
# Existing fields
decay_half_life_days = 0
reranker = "composite"
rerank_window = 30
session_boost = 1.25
agent_boost = 1.15

# New in v1.5.0
importance_boost_enabled = true   # enable/disable access-frequency boost
importance_weight = 0.1           # logarithmic multiplier scale factor (default 0.1)
importance_cap = 2.0              # maximum allowable boost factor (default 2.0)
```

### 5.2 CLI Output Changes

#### `centmem recall`
Each result object in `"results"` array gains `"access_count"` and `"last_accessed_at"`:

```json
{
  "ok": true,
  "query": "sqlite wal mode",
  "results": [
    {
      "id": 42,
      "type": "note",
      "scope": "project:cent-mem",
      "content": "All SQLite connections must enable WAL journal mode and busy_timeout=5000",
      "tags": ["sqlite", "architecture"],
      "created_at": 1788400000,
      "score": 0.3842,
      "matched_by": ["semantic", "keyword"],
      "access_count": 18,
      "last_accessed_at": 1788748000
    }
  ]
}
```

#### `centmem stats`
Gains `"importance_distribution"` reporting:

```json
{
  "ok": true,
  "scope": "project:cent-mem",
  "total_memories": 1420,
  "importance_distribution": {
    "zero_access": 1105,
    "low_access_1_5": 215,
    "medium_access_6_20": 78,
    "high_access_21_plus": 22,
    "max_access_count": 84,
    "avg_access_count": 1.42
  }
}
```

---

## 6. Internal Package Changes

### 6.1 `internal/store`

#### Updates to `types.go`:
```go
type Memory struct {
    ID             int64
    ScopeID        int64
    ScopePath      string
    Type           string
    Content        string
    Key            string
    ValueJSON      string
    Tags           []string
    SourceAgent    string
    SourceSession  string
    ContentHash    string
    Status         string
    SummarizeAt    *int64
    AccessCount    int        // NEW
    LastAccessedAt *time.Time // NEW
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

#### New Store Method (`store.go` / `access.go`):
```go
// RecordAccess updates access_count and last_accessed_at for the given memory IDs.
// It executes in a single transaction with immediate return.
func (s *Store) RecordAccess(ctx context.Context, ids []int64) error {
    if len(ids) == 0 {
        return nil
    }
    nowMicros := time.Now().UnixNano() / 1000
    // Generate parameterized query for batch update
    query := `UPDATE memories 
              SET access_count = access_count + 1, 
                  last_accessed_at = ? 
              WHERE id IN (` + makePlaceholders(len(ids)) + `)`
    args := make([]any, 0, len(ids)+1)
    args = append(args, nowMicros)
    for _, id := range ids {
        args = append(args, id)
    }
    _, err := s.db.ExecContext(ctx, query, args...)
    return err
}
```

### 6.2 `internal/search`

#### Updates to `search.go`:
```go
type Ranked struct {
    // Existing fields...
    AccessCount    int        // NEW: propagated from memories.access_count
    LastAccessedAt *time.Time // NEW: propagated from memories.last_accessed_at
    // ...
}

func applyImportanceBoost(r *Ranked, enabled bool, weight float64, cap float64) {
    if !enabled || r.AccessCount <= 0 {
        return
    }
    mult := 1.0 + math.Log1p(float64(r.AccessCount)) * weight
    if mult > cap {
        mult = cap
    }
    r.Score *= mult
}
```

#### Pipeline Integration in `Searcher.Recall`:
```go
// Step 5: Session & Agent Boosting
for _, r := range results {
    applyScopeProximityBoost(r, q, s.sessionBoost)
    applyAgentAffinityBoost(r, q, s.agentBoost)
    // Step 5b: Importance Boosting
    applyImportanceBoost(r, s.importanceEnabled, s.importanceWeight, s.importanceCap)
}

// Step 6: Recency Decay...
```

#### Asynchronous Write Trigger in `cmd/centmem/handlers.go`:
```go
// After successful Recall:
returnedIDs := make([]int64, len(results))
for i, r := range results {
    returnedIDs[i] = r.ID
}
// Fire and forget via detached context to guarantee non-blocking response
go func(ids []int64) {
    bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    _ = s.RecordAccess(bgCtx, ids)
}(returnedIDs)
```

---

## 7. Web UI Updates

1. **MemoryRow Badge:** Displays an access counter badge next to tags when `access_count > 0` (e.g., `⚡ 14` with hover tooltip: `"Recalled 14 times"`).
2. **Detail Drawer:** Shows `"Access History"` section with exact `access_count` and relative timestamp of `last_accessed_at` (`"Last accessed 2 hours ago"`).
3. **Table Sorting:** Adds `"Access Count"` as a selectable sort column in the Memory Browser.
4. **Dashboard Stats Card:** Adds an "Active Brain Index" widget showing the percentage of memories with $\ge 1$ access.

---

## 8. Testing & Verification Plan

### Automated Tests
1. **Migration Test (`internal/store/migration_test.go`):**
   - Verify `m0004_importance.sql` cleanly upgrades an existing schema version 3 database.
   - Verify default values (`access_count = 0`, `last_accessed_at = NULL`).
2. **Unit Tests (`internal/search/importance_test.go`):**
   - Test logarithmic formula against table values ($c=0 \to 1.0$, $c=1 \to 1.069$, $c=100 \to 1.462$).
   - Test hard cap boundary ($c = 100,000 \to 2.0$).
   - Test toggle disabling (`importance_boost_enabled = false`).
3. **Store Concurrency Test (`internal/store/access_test.go`):**
   - Run 50 concurrent goroutines calling `RecordAccess` on overlapping ID sets; verify exact cumulative count matching.
4. **Integration Golden Test (`cmd/centmem/handlers_test.go`):**
   - Put two identical memories. Recall one 10 times. Recall query with both candidates; verify the frequently recalled memory ranks #1.
   - Verify JSON output contract includes `access_count` field.
5. **Benchmarks:**
   - `BenchmarkSearchWithImportance`: ensure $< 1\%$ difference in recall computation time.
   - `BenchmarkRecordAccessBatch`: verify batched SQLite update takes $< 5$ ms for 20 IDs.

---

## 9. Files Affected & Implementation Checklist

- [ ] **Migration:** `internal/store/migrations/m0004_importance.sql` [NEW]
- [ ] **Data Model:** `internal/store/types.go` (add fields to `Memory`) [MODIFY]
- [ ] **Store Logic:** `internal/store/store.go` & `internal/store/access.go` (`RecordAccess`) [NEW/MODIFY]
- [ ] **Search Engine:** `internal/search/search.go` (`applyImportanceBoost`, query loading) [MODIFY]
- [ ] **Config:** `internal/config/config.go` (`SearchConfig` fields & validation) [MODIFY]
- [ ] **CLI Handler:** `cmd/centmem/handlers.go` (`cmdRecall` async trigger, `cmdStats` aggregation) [MODIFY]
- [ ] **CLI Contract:** `docs/cli-contract.md` (document `access_count` in recall response) [MODIFY]
- [ ] **Web UI:** `ui/src/components/MemoryBrowser.tsx` & `ui/src/components/MemoryDrawer.tsx` [MODIFY]
- [ ] **Tests:** Unit, integration, and benchmark tests [NEW]
- [ ] **Knowledge Graph:** Run `graphify update .` after completion
