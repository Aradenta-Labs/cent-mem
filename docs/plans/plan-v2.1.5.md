# v2.1.5 — Streaming Proposal Application (SSE Batch Progress)

**Version:** 2.1.5 (target)  
**Owner:** Aradenta Labs  
**Status:** Planned  
**Schema Migration:** None  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Medium Effort #7

---

## 0. Goal & Context

`POST /api/proposals/batch` and `POST /api/links/suggested/batch` currently process all items synchronously before returning a single JSON response. On large scopes (hundreds of proposals or auto-suggested links), the UI appears frozen until the request finishes and cannot be cancelled mid-flight.

This release adds **SSE streaming progress** (`POST /api/proposals/batch/stream` and `POST /api/links/suggested/batch/stream`) with real-time progress bars, per-item failure reporting, and client-side cancellation (`AbortController`).

---

## 1. Phased Implementation Plan

### Phase 1 — Backend SSE Batch Streaming Endpoints

**Scope:**
1. Add `POST /api/proposals/batch/stream` and `POST /api/links/suggested/batch/stream` in `internal/ui/server.go` (keeping existing non-streaming `/batch` endpoints intact for backward compatibility).
2. Request body matches existing batch payload: `{"action": "apply"|"dismiss", "ids": [1, 2, ...]}`.
3. Stream `text/event-stream` events using `http.Flusher` (reusing the SSE framing pattern from `POST /api/agent/chat`):
   - `event: start` → `{"total": 120, "action": "apply"}`
   - `event: progress` → `{"id": 42, "processed": 15, "succeeded": 14, "failed": 1, "total": 120, "error": "..."}`
   - `event: done` → `{"processed": 120, "succeeded": 119, "failed": 1, "cancelled": false}`
4. Honor `r.Context().Done()` between items: if the client cancels mid-batch, stop processing remaining IDs cleanly (already-applied items remain committed) and emit a final `done` event with `"cancelled": true` if writable.
5. Unit tests in `internal/ui/server_agent_test.go`:
   - Full stream completion with mixed success/failure IDs.
   - Mid-stream context cancellation stops remaining mutations.
   - Invalid action / empty IDs return `400` before switching to SSE.

**Exit criteria:** `go test -tags fts5 ./internal/ui/... -race` passes.

---

### Phase 2 — Web UI Progress Bar, Cancellation & Release (`v2.1.5`)

**Scope:**
1. Update `ProposalsView.tsx` and `BatchProposalConfirmDialog.tsx` to consume the SSE stream via `fetch()` + `ReadableStream` with an `AbortController`.
2. Render an inline progress banner/modal showing:
   - Progress bar (`processed / total`) and percentage.
   - Live counters (`succeeded`, `failed`) and expandable error list if any item fails.
   - **Cancel** button that aborts remaining items and refreshes the proposals list + Sidebar badge count.
3. Fall back gracefully to non-streaming `POST /api/proposals/batch` if SSE is unsupported.
4. Update `docs/ui.md`, mark #7 complete in `docs/v2-nice-to-have.md`, bump version to `2.1.5`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
