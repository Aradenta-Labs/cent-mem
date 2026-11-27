# v2.3.0 — Memory Versioning / Edit History

**Version:** 2.3.0 (target)  
**Owner:** Aradenta Labs  
**Status:** Planned  
**Schema Migration:** `m0008_memory_revisions.sql` (Schema v8)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — High Effort #10

---

## 0. Goal & Context

Whenever a memory is edited (`PATCH /api/memories/{id}`), updated by `centmem set` (fact overwrite), modified by an agent proposal (`update` / `merge`), or summarized by `centmem compact`, the previous content is currently overwritten and lost.

This release adds a lightweight, capped **`memory_revisions`** table that automatically snapshots prior versions before any destructive mutation, plus CLI (`centmem history`, `centmem rollback`) and Web UI diff/restore views.

---

## 1. Schema Migration (`m0008_memory_revisions.sql`)

```sql
CREATE TABLE IF NOT EXISTS memory_revisions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id     INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    revision_num  INTEGER NOT NULL,
    content       TEXT NOT NULL,
    tags_json     TEXT NOT NULL DEFAULT '[]',
    value_json    TEXT,                 -- populated for fact memories
    change_source TEXT NOT NULL,        -- 'ui_edit' | 'cli_set' | 'proposal_apply' | 'compact' | 'rollback'
    actor_agent   TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL      -- unix microseconds
);

CREATE INDEX IF NOT EXISTS idx_revisions_memory
    ON memory_revisions(memory_id, revision_num DESC);

INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '8');
```

Config knob in `[retention]` (`internal/config/config.go`):
```toml
[retention]
max_revisions_per_memory = 20   # 0 = unlimited; default 20
```

---

## 2. Phased Implementation Plan

### Phase 1 — Schema, Revision Snapshot Hook & Pruning (`internal/store`)

**Scope:**
1. Add `internal/store/migrations/m0008_memory_revisions.sql` and update `docs/data-model.md`.
2. Add `saveRevisionTx(ctx, tx, mem, changeSource, actorAgent)` helper in `internal/store/revisions.go`:
   - Captures the **pre-mutation** state (`content`, `tags_json`, `value_json`) with monotonic `revision_num`.
   - Enforces `max_revisions_per_memory` by pruning oldest revisions beyond the cap in the same transaction.
3. Wire `saveRevisionTx` atomically into all mutation paths:
   - `Store.UpdateMemoryContent` (`ui_edit` / `proposal_apply`)
   - `Store.Set` when overwriting an existing fact key (`cli_set`)
   - `Store.ApplyProposal` for `update` and `merge` proposals (`proposal_apply`)
   - `compact.Compact` before archiving/replacing summarized memories (`compact`)
4. Add `Store.ListRevisions(ctx, memoryID, limit)` and `Store.RollbackMemory(ctx, memoryID, revisionNum)` (which snapshots current state as a new revision with source `rollback`, restores the target revision, updates FTS, and re-queues embedding).
5. Unit tests in `internal/store/revisions_test.go` covering every mutation hook, cap pruning, and round-trip rollback.

**Exit criteria:** `go test -tags fts5 ./internal/store/... ./internal/compact/... -race` passes with zero regression on write latency (< 50 ms p95).

---

### Phase 2 — CLI Commands (`history`, `rollback`) & REST API

**Scope:**
1. Add CLI commands in `cmd/centmem/`:
   - `centmem history <memory_id> [--limit 20]` → JSON list of revisions (newest first) with metadata and content snapshots.
   - `centmem rollback <memory_id> --revision <num> [--dry-run]` → restores target revision and returns JSON diff summary.
2. Add REST endpoints in `internal/ui/server.go`:
   - `GET /api/memories/{id}/revisions`
   - `POST /api/memories/{id}/rollback` (`{"revision_num": N}`)
3. Golden-file contract tests in `cmd/centmem/` and HTTP handler tests in `internal/ui/server_test.go`.

**Exit criteria:** End-to-end CLI and API tests prove edit → history → rollback restores exact content, tags, and searchability.

---

### Phase 3 — Web UI History Tab, Side-by-Side Diff & Release (`v2.3.0`)

**Scope:**
1. Add a **History** tab inside `MemoryDetailDrawer.tsx` showing revision timeline (`revision_num`, timestamp, `change_source`, actor).
2. Render a lightweight inline/side-by-side text + tag diff between any selected revision and current content, with a **Restore this version** button + confirmation dialog.
3. Update `docs/cli-contract.md`, `docs/ui.md`, `docs/data-model.md`, `skill/SKILL.md` + mirrors, and mark #10 complete in `docs/v2-nice-to-have.md`.
4. Bump version to `2.3.0`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
