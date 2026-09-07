# Data Model: cent-mem

**Version:** 1.0
**DB:** SQLite (WAL) + FTS5 + sqlite-vec

---

## 1. Entity Overview

```
scope      ──┐
             │
memory   ────┼── belongs to a scope; has type, content, tags, embedding, timestamps
             │
fact     ────┤── special memory type with key + JSON value
             │
log      ────┤── special memory type, append-only chronological
             │
embedding ───┘── vector row per memory (1:1, lazy)
```

**Canonical schema** (single source of truth lives in `internal/store/migrations/`).

## 2. Tables

### 2.1 `scopes`

Hierarchical scope registry. Row per scope. `path` is a materialized path used for inheritance queries.

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| path | TEXT UNIQUE NOT NULL | e.g. `global`, `project:cent-mem`, `project:cent-mem/agent:claude` |
| parent_path | TEXT | nullable; used for inheritance |
| kind | TEXT CHECK(kind IN ('global','project','agent','session')) | |
| name | TEXT NOT NULL | display name |
| created_at | INTEGER NOT NULL | unix seconds |

**Indexes:** `UNIQUE(path)`, `INDEX(parent_path)`, `INDEX(kind)`

### 2.2 `memories`

All memory types live in one table for unified search.

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| scope_id | INTEGER NOT NULL REFERENCES scopes(id) | |
| type | TEXT NOT NULL CHECK(type IN ('fact','note','log')) | |
| content | TEXT NOT NULL | main text; for facts this is derived (`key: value_summary`) |
| key | TEXT | for `type='fact'`; unique within scope |
| value_json | TEXT | for `type='fact'`; JSON blob |
| tags | TEXT | comma-separated, normalized lowercase |
| source_agent | TEXT | e.g. `claude`, `codex`, `cursor`, `custom` |
| source_session | TEXT | agent's session/conversation id |
| content_hash | TEXT NOT NULL | sha256(scope_path||type||key||content) for dedup |
| status | TEXT NOT NULL DEFAULT 'active' | 'active' \| 'archived' \| 'summarized' |
| access_count | INTEGER NOT NULL DEFAULT 0 | total recall/read count (v1.5.0) |
| last_accessed_at | INTEGER | unix microseconds of last recall access (nullable, v1.5.0) |
| summarize_at | INTEGER | unix seconds when eligible for compaction |
| created_at | INTEGER NOT NULL | |
| updated_at | INTEGER NOT NULL | |

**Indexes:**
- `INDEX(scope_id, type, status, created_at)`
- `UNIQUE(scope_id, key) WHERE type='fact'` (partial unique for facts)
- `INDEX(content_hash)`
- `INDEX(summarize_at) WHERE status='active' AND summarize_at IS NOT NULL`
- `INDEX(created_at)`
- `INDEX(access_count)` (`idx_memories_access_count`)
- `INDEX(last_accessed_at)` (`idx_memories_last_accessed_at`)

### 2.3 `memories_fts` (FTS5 contentless)

External-content FTS5 index over `content` + `tags`.

```sql
CREATE VIRTUAL TABLE memories_fts USING fts5(
  content,
  tags,
  content='memories',
  content_rowid='id',
  tokenize='porter unicode61'
);
```

Maintained via triggers on `memories` (AFTER INSERT/UPDATE/DELETE).

### 2.4 `embeddings`

| Column | Type | Notes |
|--------|------|-------|
| memory_id | INTEGER PK REFERENCES memories(id) ON DELETE CASCADE | |
| embedding | BLOB NOT NULL | float32 vector (dims from config) |
| model | TEXT NOT NULL | e.g. `bge-small-en-v1.5` |
| embedded_at | INTEGER NOT NULL | |

**Vector index** (sqlite-vec):
```sql
CREATE VIRTUAL TABLE memories_vec USING vec0(
  memory_id INTEGER PRIMARY KEY,
  embedding float[384]
);
```

### 2.5 `embed_queue`

Write-time queue for async embedding.

| Column | Type | Notes |
|--------|------|-------|
| memory_id | INTEGER PK REFERENCES memories(id) ON DELETE CASCADE | |
| priority | INTEGER NOT NULL DEFAULT 0 | |
| claimed_at | INTEGER | worker claim timestamp |
| created_at | INTEGER NOT NULL | |

**Index:** `INDEX(claimed_at, priority, created_at)`

### 2.6 `events` (append-only audit/sync log)

Powers v2 sync and debugging.

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK AUTOINCREMENT | monotonic |
| memory_id | INTEGER NOT NULL | |
| op | TEXT NOT NULL CHECK(op IN ('insert','update','archive','summarize','delete')) | |
| scope_path | TEXT NOT NULL | snapshot for replication |
| payload_json | TEXT NOT NULL | full row snapshot |
| created_at | INTEGER NOT NULL | |

**Index:** `INDEX(id)` (already PK), `INDEX(memory_id)`

### 2.7 `meta`

| key TEXT PK | value TEXT |
|---|---|
| schema_version | current migration version (`4`) |
| embedding_version | embedding format version (`2`) |
| embedding_model | active model name (`bge-small-en-v1.5`) |
| embedding_dims | vector dims (`384`) |

### 2.8 Embedding Format v2

As of migration `m0003_embedding_v2.sql`, embeddings use a structured text template format:
```
Key: <key>
Tags: <comma-separated tags>

Content: <content>
```
Omitting `Key:` and `Tags:` sections when empty. This boosts semantic weighting of keys and tags during embedding inference.

### 2.9 Importance & Access Telemetry

As of migration `m0004_importance.sql`, memories record access count and timestamps upon recall retrieval.

## 3. Scope Inheritance

A read targeting `project:P` matches rows where:
- `scope.path = 'project:P'`, OR
- `scope.path` is an ancestor (`global`), OR
- `scope.path` is `project:P` and `agent`/`session` descendants when `include_children=1`.

Implemented as `scope.path LIKE 'project:P%'` for descendants plus `IN (ancestors)` for the upward chain. The upward chain is computed by walking `parent_path`.

## 4. Retention & Search Defaults (configurable)

### 4.1 Retention Defaults

| Type | Default policy |
|------|----------------|
| `fact` | keep forever; never summarize |
| `note` | `summarize_at = created_at + 30d` |
| `log` | `summarize_at = created_at + 14d`; raw dropped after summarize |

Compaction moves originals to `status='archived'` (kept in `events`) and inserts a new consolidated `note` pointing to the archive.

### 4.2 Search Defaults (configurable)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.decay_half_life_days` | integer | `0` (off) | Half-life in days for exponential recency decay. |
| `search.reranker` | string | `"composite"` | Re-ranking strategy (`none`, `composite`, `cross-encoder`, `llm`). |
| `search.rerank_window` | integer | `30` | Number of candidate results to pass to the Stage-2 re-ranker. |
| `search.session_boost` | float | `1.25` | Score multiplier for memories in exact matching session scope (+25%). |
| `search.agent_boost` | float | `1.15` | Score multiplier for memories authored by caller agent (+15%). |
| `search.importance_boost_enabled` | boolean | `true` | Enable frequency-based logarithmic importance score boosting. |
| `search.importance_weight` | float | `0.1` | Weight parameter in importance multiplier curve. |
| `search.importance_cap` | float | `2.0` | Maximum cap on importance score multiplier. |

When `search.importance_boost_enabled` is true, memories with prior recalls receive a logarithmic score multiplier applied during Stage 2 re-ranking before recency decay and candidate sorting:

$$\text{importance}(c) = \min\left(\text{cap},\, 1.0 + \ln(1 + c) \times \text{weight}\right)$$

Where $c = \text{access\_count} \ge 0$.

When `search.decay_half_life_days` > 0, memories older than the half-life receive an exponential score penalty applied after RRF fusion / re-ranking and before final rank sorting:

$$\text{score} \leftarrow \text{score} \times 0.5^{\frac{\text{age}}{\text{half\_life}}}$$

Where:
- $\text{age} = \text{now} - \text{memory.created\_at}$
- $\text{half\_life} = \text{decay\_half\_life\_days} \times 24\text{ hours}$
- If $\text{age} \le 0$ or $\text{decay\_half\_life\_days} \le 0$, no decay penalty is applied ($\text{factor} = 1.0$).

**Configuration in `config.toml`:**
```toml
[search]
decay_half_life_days = 0   # Default 0 (disabled). Set e.g. 30 to halve memory score every 30 days.
reranker = "composite"     # none | composite | cross-encoder | llm
rerank_window = 30         # candidate window for Stage 2 re-ranking
session_boost = 1.25       # 1.25x (+25%) score boost for current session memories
agent_boost = 1.15         # 1.15x (+15%) score boost for caller agent memories
importance_boost_enabled = true # Enable logarithmic importance boosting
importance_weight = 0.1    # Multiplier scaling factor
importance_cap = 2.0       # Maximum score ceiling multiplier
```

**Environment Variable Overrides:**
- `CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS`: Overrides `search.decay_half_life_days` (e.g. `export CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS=30`).
- `CENTMEM_SEARCH_RERANKER`: Overrides `search.reranker` (e.g. `export CENTMEM_SEARCH_RERANKER=composite`).
- `CENTMEM_SEARCH_RERANK_WINDOW`: Overrides `search.rerank_window` (e.g. `export CENTMEM_SEARCH_RERANK_WINDOW=30`).
- `CENTMEM_SEARCH_SESSION_BOOST`: Overrides `search.session_boost` (e.g. `export CENTMEM_SEARCH_SESSION_BOOST=1.25`).
- `CENTMEM_SEARCH_AGENT_BOOST`: Overrides `search.agent_boost` (e.g. `export CENTMEM_SEARCH_AGENT_BOOST=1.15`).
- `CENTMEM_SEARCH_IMPORTANCE_BOOST_ENABLED`: Overrides `search.importance_boost_enabled` (e.g. `export CENTMEM_SEARCH_IMPORTANCE_BOOST_ENABLED=true`).
- `CENTMEM_SEARCH_IMPORTANCE_WEIGHT`: Overrides `search.importance_weight` (e.g. `export CENTMEM_SEARCH_IMPORTANCE_WEIGHT=0.1`).
- `CENTMEM_SEARCH_IMPORTANCE_CAP`: Overrides `search.importance_cap` (e.g. `export CENTMEM_SEARCH_IMPORTANCE_CAP=2.0`).

## 5. Example Rows

**Fact**
```
scope: project:cent-mem, type: fact, key: user.timezone,
value_json: "Asia/Jakarta", content: "user.timezone = Asia/Jakarta",
tags: user,preference
```

**Note**
```
scope: project:cent-mem, type: note,
content: "We chose sqlite-vec over pgvector for local-first speed",
tags: decision,db
```

**Log**
```
scope: project:cent-mem/agent:claude/session:sess_123, type: log,
content: "Asked to create PRD; chose hierarchical scoping",
tags: decision
```

## 6. Sync-Readiness (v2)

- Every write appends to `events` (monotonic id).
- A sync server can tail `events` since `last_seen_id` per scope and upsert remotely.
- `content_hash` enables idempotent application on the receiver.
