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

## Hybrid Search Engine (RRF k=60)

`centmem recall` executes four parallel search pipelines and fuses their results using **Reciprocal Rank Fusion (RRF)**:

1. **Semantic Vector Search**:
   - Generates 384-dimensional dense vectors using local BGE-small-en-v1.5 via ONNX Runtime.
   - Computes cosine distance using `sqlite-vec`.
2. **Keyword Full-Text Search (FTS5)**:
   - Tokenizes queries with SQLite FTS5 using the Porter stemmer.
   - Scores matches via BM25 ranker.
3. **Exact Fact Matcher**:
   - Searches key/value fact names, string values, and attached tags.
4. **Timeline Recency Scorer**:
   - Scores memories based on exponential decay from creation timestamp.

### Fusion Formula
For each candidate memory $m$, the combined score is:
$$RRF(m) = \sum_{r \in Rankers} \frac{1}{60 + rank_r(m)}$$

This ensures balanced retrieval: memories that match either semantic intent or exact technical tokens are promoted without score skew.
