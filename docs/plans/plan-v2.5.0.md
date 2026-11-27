# v2.5.0 — Distributed Multi-Machine Sync & Per-Agent RBAC

**Version:** 2.5.0 (target) *(candidate for `v3.0.0` if paired with any breaking contract cleanup)*  
**Owner:** Aradenta Labs  
**Status:** Planned *(official roadmap item)*  
**Schema Migration:** `m0010_sync_rbac.sql` (Schema v10)  
**Depends on:** `centmemd` gRPC daemon (`v1.5.4`, `internal/daemon/service_sync.go`)  
**Source:** [docs/implementation-plan.md](../implementation-plan.md) · [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — High Effort #11

---

## 0. Goal & Context

`v1.5.4` shipped `centmemd` with a local `SyncService` (`StreamEvents` and `PushEvents` in `internal/daemon/service_sync.go`) and an `events` CDC log, but replication is still single-machine: there is no TLS/authentication on gRPC, no peer registry or cursor tracking, no background sync worker, and no per-agent access control.

This release turns `centmemd` into a **peer-to-peer / hub-and-spoke multi-machine sync engine** while preserving cent-mem's **local-first guarantee**: sync is strictly opt-in, every machine keeps a full local SQLite store, and offline operation never degrades.

---

## 1. Schema Migration (`m0010_sync_rbac.sql`)

```sql
-- 1. Registered remote sync peers
CREATE TABLE IF NOT EXISTS sync_peers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    address         TEXT NOT NULL,          -- host:port
    direction       TEXT NOT NULL CHECK(direction IN ('pull', 'push', 'bidirectional')),
    scope_filter    TEXT NOT NULL DEFAULT 'global',
    enabled         INTEGER NOT NULL DEFAULT 1,
    last_pull_id    INTEGER NOT NULL DEFAULT 0,
    last_push_id    INTEGER NOT NULL DEFAULT 0,
    last_synced_at  INTEGER,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL
);

-- 2. Idempotent remote event deduplication ledger
CREATE TABLE IF NOT EXISTS sync_applied_events (
    peer_id         TEXT NOT NULL,
    remote_event_id INTEGER NOT NULL,
    applied_at      INTEGER NOT NULL,
    PRIMARY KEY (peer_id, remote_event_id)
);

-- 3. Per-agent RBAC tokens & scope policies
CREATE TABLE IF NOT EXISTS rbac_tokens (
    id              TEXT PRIMARY KEY,
    agent_name      TEXT NOT NULL,
    token_hash      TEXT NOT NULL UNIQUE,   -- SHA-256 hex of bearer token
    role            TEXT NOT NULL CHECK(role IN ('reader', 'writer', 'admin')),
    scope_prefix    TEXT NOT NULL DEFAULT 'global',
    revoked_at      INTEGER,
    created_at      INTEGER NOT NULL
);

INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '10');
```

---

## 2. Phased Implementation Plan

### Phase 1 — Transport Security (TLS / mTLS) & Auth Interceptors (`internal/daemon`)

**Scope:**
1. Extend `[daemon]` config in `internal/config/config.go` with TLS settings (`tls_cert`, `tls_key`, `tls_ca`, `require_auth` when binding non-loopback TCP).
2. Fail closed: refuse to bind `centmemd` on non-localhost TCP (`0.0.0.0`) unless TLS + auth are configured (or explicit `--insecure-local-dev` flag is passed).
3. Add unary and streaming gRPC auth interceptors in `internal/daemon/auth_interceptor.go` validating `authorization: Bearer <token>` against `rbac_tokens` (bypassed only on local Unix domain socket IPC).
4. Unit tests in `internal/daemon/auth_test.go` covering TLS handshake, valid/revoked/missing tokens, and loopback vs non-loopback enforcement.

**Exit criteria:** Unauthenticated remote gRPC calls are rejected with `codes.Unauthenticated`; local Unix socket CLI workflows remain zero-config.

---

### Phase 2 — Peer Replication Worker & Deterministic Conflict Resolution

**Scope:**
1. Add `internal/store/migrations/m0010_sync_rbac.sql` and update `docs/data-model.md`.
2. Build `internal/daemon/sync_worker.go`:
   - Manages one resilient goroutine per enabled `sync_peers` entry with exponential backoff + jitter on disconnect.
   - **Pull loop:** calls remote `SyncService.StreamEvents(since_id = last_pull_id, scope = scope_filter)`, skips already-applied `(peer_id, remote_event_id)` pairs, applies mutations locally, and advances `last_pull_id`.
   - **Push loop:** tails local `events` since `last_push_id`, filters out events originating from that peer (loop prevention via `origin_node_id`), and streams to remote `SyncService.PushEvents`.
3. **Conflict resolution rules (deterministic, CRDT-friendly):**
   - **Facts (`set`):** Last-Write-Wins (LWW) by `(updated_at, origin_node_id)` tie-breaker; older conflicting values are preserved as a `memory_revisions` entry (leveraging v2.3.0).
   - **Notes / Logs (`put`):** Append-only deduplicated by `(scope_id, content_hash)`.
   - **Forgets (`forget`):** Tombstone wins (`archived_at` propagates monotonically).
   - **Links (`link`/`unlink`):** Set-union with tombstone precedence.
4. Multi-daemon integration tests in `internal/daemon/sync_e2e_test.go` spinning up 2–3 in-process `centmemd` instances with partition/reconnect simulation.

**Exit criteria:** Two daemons converge to identical state after concurrent offline edits, restarts, and network partitions with zero infinite echo loops.

---

### Phase 3 — Per-Agent RBAC Enforcement & CLI (`centmem auth`, `centmem sync`)

**Scope:**
1. Enforce `rbac_tokens` policy (`role` + `scope_prefix`) across all `MemoryService` and `SyncService` RPCs and remote REST endpoints:
   - `reader`: `Recall`, `Get`, `List`, `Timeline`, `Stats`, `Links`, `StreamEvents` within `scope_prefix`.
   - `writer`: `reader` + `Put`, `Set`, `Forget`, `Link`, `Unlink`, `PushEvents` within `scope_prefix`.
   - `admin`: full access including `Compact`, `DeleteScopeTree`, peer management, and token management.
2. Add CLI commands in `cmd/centmem/`:
   - `centmem sync peer add <name> <addr> [--direction bidirectional] [--scope project:X]`
   - `centmem sync peer list` / `remove <name>` / `status`
   - `centmem sync now [--peer <name>]` (trigger immediate one-shot sync)
   - `centmem auth token create --agent <name> --role <reader|writer|admin> --scope <scope>`
   - `centmem auth token list` / `revoke <id>`
3. Golden-file CLI contract tests in `cmd/centmem/`.

**Exit criteria:** A `reader` token scoped to `project:alpha` can recall `project:alpha` but receives `codes.PermissionDenied` on `put` or on `project:beta`.

---

### Phase 4 — Web UI Sync Dashboard, Hardening & Release (`v2.5.0`)

**Scope:**
1. Add `GET /api/sync/status` and `POST /api/sync/peers` endpoints in `internal/ui/server.go`.
2. Add a **Sync & Access** tab in Web UI Settings showing connected peers, replication lag (`last_synced_at`, pending events, last error), and token management.
3. Add `"sync_peers"` health check to `centmem doctor`.
4. Stress-test 10,000-event initial sync and verify p95 local read/write SLAs remain unaffected while background sync runs.
5. Update `docs/guides/daemon.md`, `docs/cli-contract.md`, `docs/data-model.md`, `skill/SKILL.md` + mirrors, and mark #11 complete in `docs/v2-nice-to-have.md`.
6. Bump version to `2.5.0` (or `3.0.0`), update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
