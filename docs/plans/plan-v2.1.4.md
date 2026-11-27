# v2.1.4 — Web UI Keyboard Shortcuts & Drawer Edit

**Version:** 2.1.4 (target)  
**Owner:** Aradenta Labs  
**Status:** Ready for implementation *(re-versioned from `plan-v2.1.3.md` after v2.1.3 shipped Knowledge Graph View)*  
**Schema Migration:** None  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Quick Win #4

---

## 0. Goal & Context

Make the Web UI Memory Browser fully keyboard-navigable (`j`/`k`, `/`, `Enter`, `e`, `d`, `Esc`, `?`) and add in-drawer memory editing via a new `PATCH /api/memories/{id}` route backed by the existing `store.UpdateMemoryContent()`.

Most navigation shortcuts (`j`/`k`, `/`, `Enter`, `Del`/`Backspace`, `?`, `Cmd+,`, `Esc`) are already documented in [docs/ui.md](../ui.md) §5; this release audits them, adds the missing `d` (delete alias) and `e` (edit) shortcuts, and exposes the backend edit route.

---

## 1. Phased Implementation Plan

### Phase 1 — Shortcut Audit & Backend Edit Endpoint (`PATCH /api/memories/{id}`)

**Scope:**
1. Audit existing keyboard handlers in `ui/src/` (`MemoryBrowser.tsx`, `MemoryDetailDrawer.tsx`, `TopBar.tsx`, shortcuts overlay) to confirm every documented shortcut works and is suppressed inside `<input>`, `<textarea>`, `[contenteditable]`, and open modals.
2. Add `PATCH /api/memories/{id}` in `internal/ui/server.go` next to `POST /api/memories/{id}/forget`:
   - Request: `{"content": "string", "tags": ["a", "b"]}` (`content` required, non-empty after trim).
   - Calls `cfg.Store.UpdateMemoryContent(ctx, id, in.Content, in.Tags)`.
   - Response `200`: `{"ok": true, "id": 142, "updated_at": 1789300000, "reindexed": true}`.
   - Errors: `400` (`invalid_id`, `invalid_payload`), `404` (`not_found`), `503` (`store_unavailable`), `500` (`store_error`).
3. Unit tests in `internal/ui/server_test.go`: valid update, tag-only update, empty content `400`, missing ID `404`, and FTS/embed re-queue verification.

**Exit criteria:** `go test -tags fts5 ./internal/ui/...` passes with full coverage of `PATCH /api/memories/{id}`.

---

### Phase 2 — Frontend Shortcuts (`d`, `e`) & Drawer Edit Mode

**Scope:**
1. Add `d` as an alias for `Backspace`/`Delete` in `MemoryBrowser.tsx` (opens the existing forget confirmation dialog for the focused row).
2. Add `e` shortcut in `MemoryBrowser.tsx` and an **Edit** button in `MemoryDetailDrawer.tsx`:
   - Opens the detail drawer in edit mode with content `<textarea>`, editable tag chips, **Save** (`Cmd/Ctrl+Enter`), and **Cancel** (`Esc`).
   - Calls `PATCH /api/memories/{id}`, updates row state optimistically/via refresh, and displays a toast notification.
3. Centralize input-focus guard (`isEditableTarget(e.target)`) so single-key shortcuts never hijack typing in Assistant chat, Settings, Timeline date pickers, or Graph search.
4. Update the `?` Keyboard Shortcuts overlay to group all shortcuts cleanly (Navigation, Actions, Global).

**Exit criteria:** `npm run build` in `ui/` succeeds cleanly and manual walkthrough confirms all shortcuts work across Browser, Timeline, Graph, and Proposals tabs without input collisions.

---

### Phase 3 — Docs & Release (`v2.1.4`)

**Scope:**
1. Update `docs/ui.md` §5 shortcut table and REST API table (`PATCH /api/memories/{id}`).
2. Mark Quick Win #4 complete in `docs/v2-nice-to-have.md`.
3. Bump version to `2.1.4`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
