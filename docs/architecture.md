# Architecture: cent-mem

**Version:** 1.0
**Date:** 2026-08-31

---

## 1. Design Principles

1. **Local-first, sync-ready.** One process owns the DB file. Schema and write path are designed so a sync layer (v2) can replicate via logical replication / event log without breaking v1.
2. **Skill = thin wrapper over a CLI.** The skill contains no logic — it documents the CLI contract and how to call it. This keeps one source of truth and lets any agent/harness integrate trivially.
3. **Fast writes, async embeddings.** Writes return immediately after persisting the row; embeddings are computed by a background embedder reading from a queue table. Reads against un-embedded memories gracefully fall back to keyword search.
4. **Deterministic machine output.** Every command emits stable JSON by default. This is what makes the skill usable across diverse agents.
5. **Single static binary.** No runtime dependencies; the user installs one binary and one skill.

## 2. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                       AI Agents / Harnesses                       │
│   Claude Code  │  Codex  │  Cursor  │  Custom harness  │  ...     │
└──────────┬───────────────┬───────────────┬───────────────┬───────┘
           │               │               │               │
           ▼               ▼               ▼               ▼
   ┌──────────────────────────────────────────────────────────┐
   │                   centmem SKILL (docs/contract)           │
   │   describes: commands, args, JSON shapes, exit codes      │
   └──────────────────────┬───────────────────────────────────┘
                          │ exec()
                          ▼
   ┌──────────────────────────────────────────────────────────┐
   │                    centmem CLI (Go)                        │
   │  ┌─────────────┐ ┌──────────────┐ ┌───────────────────┐  │
   │  │   Commands   │ │   JSON I/O   │ │  Exit-code map    │  │
   │  └─────────────┘ └──────────────┘ └───────────────────┘  │
   │  ┌──────────────────────────────────────────────────────┐ │
   │  │              Core (Go packages)                       │ │
   │  │  store  │  search  │  embed  │  compact  │  config    │ │
   │  └──────────────────────────────────────────────────────┘ │
   └────────────┬────────────────────────────────────┬──────────┘
                │                                     │
        ┌───────▼────────┐                  ┌─────────▼─────────┐
        │  SQLite (WAL)   │                  │  Embed queue (DB) │
        │  + FTS5 + vec   │                  │  + ONNX runtime   │
        └─────────────────┘                  └───────────────────┘
                          │
                  ┌───────▼────────┐
                  │ ~/.centmem/    │
                  │  centmem.db    │
                  │  models/       │
                  │  config.toml   │
                  └────────────────┘
```

## 3. Component Breakdown

### 3.1 `cmd/centmem` — CLI entrypoint
- Uses a minimal flag library (stdlib `flag` or `pflag`).
- Routes subcommands to handlers; one handler per command.
- Default `--json`; `--pretty` for humans.

### 3.2 `internal/store` — persistence layer
- Owns the SQLite connection (WAL, busy_timeout, foreign_keys).
- SQL is centralized in `internal/store/sql.go` for review/grep.
- Exposes typed methods; no raw SQL leaks outside this package.
- **Migration system**: versioned migrations in `internal/store/migrations/` applied on open; `init` creates schema.

### 3.3 `internal/search` — query engine
- `Hybrid(query, filters)` orchestrates:
  1. semantic: `vec_distance_cosine` over filtered rows
  2. keyword: FTS5 `bm25` ranking
  3. fact: direct key prefix lookup
  4. timeline: `created_at` range
- Fuses rankings via Reciprocal Rank Fusion (RRF) — fast, parameter-light.
- Applies final filters, dedup, truncate-to-N.

### 3.4 `internal/embed` — embedding service
- Loads ONNX model once (lazy, cached on `Embedder` struct).
- Reads `embed_queue` rows, embeds, writes `embeddings` rows, marks queue done.
- Runs **inline** in the CLI process as a short-lived worker: on each CLI invocation, drain up to K queued items before exiting. This avoids a daemon in v1 while keeping embeddings cheap.
- v1.1+: optional `centmem embedd` long-running daemon for higher throughput.

### 3.5 `internal/compact` — retention
- `Compact(scope, policy)` selects memories past threshold, invokes a `Summarizer`, writes a consolidated `note`, archives originals (soft-delete to `archived` table).
- `Summarizer` interface: `HeuristicSummarizer` (default, deterministic) and `LLMSummarizer` (pluggable hook).

### 3.6 `internal/config`
- Loads `~/.centmem/config.toml` (env overrides via `CENTMEM_*`).
- Keys: `home`, `db_path`, `model.name`, `model.path`, `model.dims`, `retention.*`, `api_key` (v2).

### 3.7 Skill package `skill/`
- `SKILL.md` — the contract agents read (commands, JSON, exit codes, examples).
- `install.sh` — installs the skill into Claude Code, Cursor, Codex, and a generic location.
- Per-harness adapter notes in `skill/adapters/`.

## 4. Data Flow

### 4.1 Write (e.g. `put`)
```
agent → skill → exec `centmem put --scope project:X --type note --content "..." --tags a,b`
  → store.Put → BEGIN tx → insert memory row → insert embed_queue row → COMMIT
  → embedder drains queue (inline, ≤K) → upsert embeddings row
  → stdout JSON {id, status:"embedded"|"queued"}
```

### 4.2 Read (e.g. `recall`)
```
agent → skill → exec `centmem recall "how do we deploy" --scope project:X --top 5`
  → search.Hybrid:
       semantic: SELECT ... ORDER BY vec_distance_cosine(embedding, ?)
       keyword:  SELECT ... FROM memories_fts WHERE memories_fts MATCH ? ORDER BY bm25
       facts:    SELECT ... FROM facts WHERE key LIKE ?||'%'
       timeline: optional ORDER BY created_at filter
  → RRF fuse → dedup → slice top N
  → stdout JSON [{id,type,scope,content,tags,score,...}]
```

## 5. Concurrency Model

- **Single writer, many readers** via SQLite WAL. Multiple agents may run `centmem` concurrently; SQLite serializes writes with `busy_timeout`.
- **No long-lived process in v1.** Each CLI invocation is short-lived. The inline embedder uses `SELECT ... LIMIT K FOR UPDATE`-style claiming (via `UPDATE embed_queue SET claimed_at=now WHERE id IN (...)`) to avoid duplicate work across concurrent invocations.
- v2 daemon will own the embedder loop and accept gRPC.

## 6. Performance Strategy

| Concern | Approach |
|---------|----------|
| Embedding latency | Inline worker limited to K items per invocation; cost amortized across writes |
| Search latency | Covering indexes on (scope,type,created_at); vec index (HNSW via sqlite-vec) on embeddings |
| JSON parse cost (agents) | Output is compact, stable-shape JSON; agents parse once |
| Process startup | Go binary cold start ~10–30 ms; SQLite open ~5 ms |
| DB growth | Compaction + archive; periodic `VACUUM` after compaction |

## 7. Reliability

- WAL mode + `synchronous=NORMAL` for durability/speed balance.
- `PRAGMA integrity_check` exposed via `centmem doctor`.
- Migrations are idempotent and forward-only; `migrations` table tracks applied.
- Backups: `centmem backup --to file` does `VACUUM INTO`.

## 8. Security

- DB file `0600`, dir `0700`.
- No telemetry, no network calls except one-time model download (over HTTPS, pinned checksum).
- v2 API key stored in config (also `0600`), never logged.

## 9. Extensibility Points

| Point | How |
|-------|-----|
| New memory type | Add enum value + handler in store/search |
| Different embedder | Implement `Embedder` interface |
| Different summarizer | Implement `Summarizer` interface |
| New harness adapter | Add `skill/adapters/<harness>.md` |
| Sync (v2) | Add `internal/sync` reading the append-only event log already recorded per write |

## 10. Technology Choices (locked)

| Choice | Selected | Why |
|--------|----------|-----|
| Language | Go 1.22+ | Single static binary, fast startup, simple concurrency |
| DB | SQLite (modernc.org/sqlite pure-Go or mattn/go-sqlite3) | Zero-ops, single-file, sync-ready. Prefer pure-Go build for ease; cgo variant only if vec perf needs it |
| Vector | sqlite-vec | Same-file vector search, no extra service |
| FTS | SQLite FTS5 | Built-in, bm25 ranking |
| Embeddings | ONNX Runtime (gonnx or k2onnx) + BGE-small | Local, deterministic, ~384d |
| Config | TOML (pelletier/go-toml) | Human-editable, comments |
| CLI flags | spf13/pflag | POSIX-style flags, subcommands |
| Logging | log/slog (stdlib) | Structured, no dep |
| Testing | testing + testify/require + golden files | Stdlib-first |

## 11. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| sqlite-vec maturity | search correctness | Pin version; add property tests; fallback to brute-force cosine if needed |
| Pure-Go SQLite slower than cgo | perf | Benchmark early in M0; switch to cgo build tag if needed |
| ONNX Go bindings uneven | embed blocker | Pre-encode using a tiny bundled model as fallback; allow external embedder via subprocess |
| Concurrent writers contention | latency | busy_timeout + retries; WAL handles it |
| Skill differences across harnesses | integration gaps | One canonical SKILL.md + per-harness adapter notes |
