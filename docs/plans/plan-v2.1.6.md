# v2.1.6 — Webhook / Notification Hooks

**Version:** 2.1.6 (target)  
**Owner:** Aradenta Labs  
**Status:** Planned  
**Schema Migration:** None  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Medium Effort #8

---

## 0. Goal & Context

Teams running `centmem curate` on a schedule or across multi-agent workflows need a lightweight way to get notified when agent proposals are created or applied (Slack, Discord, n8n, custom HTTP endpoints) without adding runtime dependencies or violating the offline-first principle (hooks are strictly opt-in and disabled by default).

---

## 1. Configuration Design

Add an optional `[hooks]` section in `~/.centmem/config.toml` (`internal/config/config.go`):

```toml
[hooks]
enabled = false
proposals_created = ""     # HTTP(S) URL
proposals_applied = ""     # HTTP(S) URL
proposals_dismissed = ""   # Optional HTTP(S) URL
secret = ""                # Optional HMAC-SHA256 signing secret (env var name or direct)
timeout_seconds = 5        # Default 5s
```

Payload envelope (JSON POST with `Content-Type: application/json`, `X-Centmem-Event`, and optional `X-Centmem-Signature-256`):
```json
{
  "event": "proposals.created",
  "timestamp": "2026-11-27T10:35:00Z",
  "scope": "project:cent-mem",
  "count": 3,
  "proposals": [
    {"id": 12, "type": "merge", "title": "Merge duplicate auth notes"}
  ]
}
```

---

## 2. Phased Implementation Plan

### Phase 1 — Config Schema & Async Webhook Dispatcher (`internal/hooks`)

**Scope:**
1. Add `HooksConfig` struct to `internal/config/config.go` with defaults (`Enabled: false`, `TimeoutSeconds: 5`) and validation (reject non-`http`/`https` schemes).
2. Create `internal/hooks/dispatcher.go`:
   - Non-blocking `Emit(ctx, event, payload)` using a bounded goroutine/worker queue (never blocks CLI writes or HTTP handlers; respects p95 write SLA).
   - Optional HMAC-SHA256 signature header (`X-Centmem-Signature-256: sha256=<hex>`).
   - Single retry with short backoff (500ms) on `5xx`/network timeout; logs failures to stderr without failing the primary operation.
   - `Test(ctx, url)` helper for synchronous dry-run connectivity testing.
3. Unit tests in `internal/hooks/dispatcher_test.go` using `httptest.Server` covering delivery, HMAC verification, timeout handling, and disabled no-op behavior.

**Exit criteria:** `go test -tags fts5 ./internal/hooks/... ./internal/config/... -race` passes.

---

### Phase 2 — Lifecycle Wiring (CLI + Web UI + `centmem doctor`)

**Scope:**
1. Wire `hooks.Dispatcher` into:
   - `internal/agent/engine.go` (`Curate()` when new proposals are persisted → `proposals.created`).
   - `cmd/centmem/` (`proposals apply` / `dismiss` → `proposals.applied` / `proposals.dismissed`).
   - `internal/ui/server.go` (single + batch proposal apply/dismiss handlers, and hot-reload dispatcher on `PATCH /api/config`).
2. Add `POST /api/config/test-hook` endpoint in `internal/ui/server.go` to send a sample `hooks.test` payload and return HTTP status + latency ms.
3. Ensure CLI flushes pending hook deliveries on exit with a bounded grace timeout (e.g., 2s) so short-lived CLI commands (`centmem curate`) don't drop webhooks on process exit.

**Exit criteria:** Integration tests confirm `centmem curate` and UI proposal approvals emit expected webhook payloads when configured and make zero network calls when disabled.

---

### Phase 3 — Web UI Settings, Docs & Release (`v2.1.6`)

**Scope:**
1. Add a **Webhooks** section in Web UI Settings (`SettingsModal.tsx` / `HooksTab.tsx` or inside Agent tab) with URL fields, secret input, enable toggle, and a **Test Webhook** button.
2. Document `[hooks]` in `docs/cli-contract.md`, `docs/ui.md`, and `skill/references/`.
3. Mark #8 complete in `docs/v2-nice-to-have.md`, bump version to `2.1.6`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
