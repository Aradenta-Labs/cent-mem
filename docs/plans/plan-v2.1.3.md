# v2.1.3 — Keyboard Shortcuts in Web UI

**Version:** 2.1.3 (target)  
**Owner:** Aradenta Labs  
**Status:** Grilling complete — Design locked, ready for phased implementation  
**Depends on:** v2.0.0 (Web UI Memory Browser, `store.UpdateMemoryContent`)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Quick Win #4

---

## 0. Executive Summary

Keyboard shortcuts make the Web UI's Memory Browser navigable without a mouse: `j`/`k` to move through rows, `/` to search, `e` to edit, `d` to delete, `?` for help.

**Why this version (v2.1.3)?** The nice-to-have claims "the Web UI is mouse-only today," but [docs/ui.md](../ui.md) §5 documents `j`/`k`, `/`, `Enter`, `Del`/`Backspace`, `?`, `Cmd+,` as **already implemented** — and the UI anatomy diagram shows keyboard hints. The honest plan is therefore **audit + gap-fill**, not greenfield:

1. **Audit** the actual compiled `dist/` bundle (static grep + manual walkthrough) to find which shortcuts really work.
2. **Gap-fill** the two keys the nice-to-have adds and docs don't cover: `d` (delete alias) and `e` (edit).
3. **`e` requires backend work** — `store.UpdateMemoryContent()` exists but has no HTTP route. We add `PATCH /api/memories/{id}`. This contradicts the nice-to-have's "pure frontend" signal; the plan is honest about it.

### Design decisions (locked via `/grill-me`)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Ground truth | **Audit + gap-fill** | docs/ui.md documents most shortcuts already; only `d` and `e` are new |
| `e` (edit) | **Add `PATCH /api/memories/{id}`** + drawer edit mode | `store.UpdateMemoryContent()` already exists; only the HTTP route is missing |
| Edit UX | **Drawer edit mode** | `e` opens the existing detail drawer with editable content + tags; Save calls PATCH |
| `d` (delete) | **Alias alongside Del/Backspace** | No regressions; `d` triggers the same confirm dialog |
| Delivery | **Split: Go in-repo + frontend build session** | Same dist/-only constraint as v2.1.0/v2.1.1 |
| Audit method | **Static grep of bundle + manual walkthrough** | Produces an evidence-based gap table before implementation |
| `?` overlay | **Update static overlay** | Same non-interactive reference as today; grouped by category |

---

## 1. Scope & Anti-goals

### In scope
- Audit of the current shortcut behavior in the compiled bundle.
- `PATCH /api/memories/{id}` endpoint wrapping `store.UpdateMemoryContent()`.
- Frontend: `d` delete alias, `e` → drawer edit mode (textarea + tags + Save/Cancel), `?` overlay updated with the full shortcut set.
- Shortcut suppression rules: no key handling while typing in inputs/textareas/dialogs.
- Docs updates: `docs/ui.md` §5 and the API reference table.

### Anti-goals (explicit)
- No new CLI commands or CLI contract changes.
- No inline table editing (drawer edit mode only).
- No interactive `?` overlay (static reference).
- No shortcut remapping/configuration UI (hardcoded key map).
- No changes to `backup`, `restore`, export, or import (v2.1.2 territory).
- No schema migrations.

---

## 2. Current State & Gap Analysis

### 2.1 Documented shortcuts (docs/ui.md §5)

| Shortcut | Action | Nice-to-have overlap |
|---|---|---|
| `/` or `Cmd/Ctrl+K` | Focus memory search input | ✓ (nice-to-have: `/`) |
| `J` / `↓` | Select next memory row | ✓ |
| `K` / `↑` | Select previous memory row | ✓ |
| `Enter` | Open selected memory detail drawer | — |
| `Backspace` / `Del` | Forget selected memory (confirmation) | partial (nice-to-have: `d`) |
| `Cmd/Ctrl+,` | Open Settings dialog | — |
| `Esc` | Close drawer/modal/popover | — |
| `?` | Toggle keyboard shortcuts overlay | ✓ |

### 2.2 Gaps introduced by the nice-to-have

| Key | Gap | Resolution in this plan |
|---|---|---|
| `d` | Not documented; Del/Backspace exist | Add `d` as alias |
| `e` | No edit capability exists in UI or API | Add `PATCH /api/memories/{id}` + drawer edit mode |

### 2.3 Audit output template (Phase 0 artifact)

| Shortcut | Static evidence (bundle grep) | Manual result | Action |
|---|---|---|---|
| `/` focus search | … | pass/fail | … |
| `j`/`k` navigate | … | pass/fail | … |
| `Enter` open drawer | … | pass/fail | … |
| `Del`/`Backspace` delete | … | pass/fail | … |
| `?` overlay | … | pass/fail | … |
| `Cmd+,` settings | … | pass/fail | … |
| `Esc` close | … | pass/fail | … |
| `d` delete alias | absent | n/a | **add** |
| `e` edit | absent | n/a | **add (backend + frontend)** |

---

## 3. Backend: `PATCH /api/memories/{id}`

### 3.1 Route

```
PATCH /api/memories/{id}
```

Registered in [internal/ui/server.go](../../internal/ui/server.go) next to `POST /api/memories/{id}/forget`.

### 3.2 Request body

```json
{ "content": "Updated content", "tags": ["architecture", "search"] }
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `content` | string | yes | Non-empty after trim |
| `tags` | []string | no | Empty/absent clears tags; present replaces |

### 3.3 Response

**200:**
```json
{ "ok": true, "id": 142, "updated_at": 1789300000, "reindexed": true }
```

- `reindexed` is `true` when the content changed (store re-queues embedding), `false` when only tags changed.

**Errors:**

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `invalid_id` | Non-integer or ≤ 0 id |
| 400 | `invalid_payload` | Malformed JSON or empty content |
| 404 | `not_found` | Memory doesn't exist |
| 503 | `store_unavailable` | `cfg.Store == nil` |
| 500 | `store_error` | `UpdateMemoryContent` failure |

### 3.4 Handler sketch

```go
mux.HandleFunc("PATCH /api/memories/{id}", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if cfg.Store == nil { /* 503 store_unavailable */ }

    id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil || id <= 0 { /* 400 invalid_id */ }

    var in struct {
        Content string   `json:"content"`
        Tags    []string `json:"tags"`
    }
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil { /* 400 invalid_payload */ }
    if strings.TrimSpace(in.Content) == "" { /* 400 invalid_payload */ }

    if err := cfg.Store.UpdateMemoryContent(r.Context(), id, in.Content, in.Tags); err != nil {
        if errors.Is(err, store.ErrNotFound) { /* 404 not_found */ }
        /* 500 store_error */
    }
    m, _ := cfg.Store.GetMemory(r.Context(), id)
    _ = json.NewEncoder(w).Encode(map[string]any{
        "ok":         true,
        "id":         id,
        "updated_at": m.UpdatedAt.Unix(),
    })
})
```

> **Store semantics (already implemented):** `UpdateMemoryContent` recomputes `content_hash`, updates `updated_at`, and — when content changed — deletes stale embeddings (`embeddings`, `memories_vec`, `embed_queue`) and re-enqueues embedding with priority 0. No store changes needed; the endpoint is a thin wrapper.

---

## 4. Frontend: Shortcut Gap-Fill

> Frontend is a separate build session (same dist/-only constraint as v2.1.0/v2.1.1).

### 4.1 Key map (final target)

| Shortcut | Action | Scope guard |
|---|---|---|
| `/` or `Cmd/Ctrl+K` | Focus search input | global, except when already in an input |
| `J` / `↓` | Next row | memory list focused |
| `K` / `↑` | Previous row | memory list focused |
| `Enter` | Open drawer for selected row | memory list focused |
| `E` | **Open drawer in edit mode** | memory list focused |
| `D` | Delete selected (confirm dialog) | memory list focused |
| `Del` / `Backspace` | Delete selected (confirm dialog) | memory list focused |
| `Esc` | Close drawer/modal; exit edit mode (discard) | global |
| `?` | Toggle shortcuts overlay | global, except while typing |
| `Cmd/Ctrl+,` | Open Settings | global |

### 4.2 Guard rules (critical for correctness)

1. **Never intercept keys while typing**: if `document.activeElement` is an `input`, `textarea`, or `[contenteditable]`, all shortcuts except `Esc` are suppressed.
2. **Edit-mode guard**: while the drawer is in edit mode, `j`/`k`/`d`/`e` are inert; `Esc` discards, `Cmd/Ctrl+Enter` saves (in addition to the Save button).
3. **Focus ring**: after `j`/`k` changes the selected row, the row receives `aria-activedescendant` / visible focus ring (per impeccable Operate-mode keyboard accessibility).
4. **Shortcuts only active on the Memory Browser route** — not on Assistant Chat or Proposals (future milestones may opt in).

### 4.3 Drawer edit mode (`e`)

```
MemoryDrawer (existing)
├── View mode (existing): content rendered read-only, actions row
└── Edit mode (new):
    ├── Textarea prefilled with content
    ├── Tags editor (chip add/remove)
    ├── Footer: [Cancel] [Save]  (Esc = Cancel, Cmd+Enter = Save)
    └── On Save: PATCH /api/memories/{id} → toast → back to View mode with fresh data
```

**States:**
- **Dirty indicator** — footer shows a dot when content/tags differ from loaded values (same pattern as Settings dialog in plan-v1.4.0).
- **Saving** — Save button disabled + spinner while PATCH in flight.
- **Error** — inline error banner on 404/500; edit mode stays open so the user doesn't lose work.
- **Empty content** — Save disabled when content is whitespace-only.

### 4.4 `?` overlay update

Static overlay, grouped:

```
Navigation                Actions                  System
──────────                ───────                  ──────
j / ↓    next row         Enter  open drawer       ?       this overlay
k / ↑    prev row         e      edit memory       / ⌘K    focus search
Esc      close/back       d, ⌫, Del  delete        ⌘,      settings
```

Rendered with the existing modal component, dismiss via `?`/`Esc`/click-outside.

---

## 5. Phased Implementation Plan

### Phase 0 — Audit (in-repo, no code changes)

1. **Static grep** of `internal/ui/dist/assets/*.js`:
   ```bash
   grep -o 'keydown\|keyup\|KeyJ\|KeyK\|Escape\|Backspace\|Delete' internal/ui/dist/assets/*.js | sort | uniq -c
   ```
   Record per-shortcut evidence in the gap table (§2.3).
2. **Manual walkthrough**: `centmem ui` → open Memory Browser → execute each documented shortcut → record pass/fail.
3. **Write the audit result** into this plan file (replace the template table with findings) or a `docs/plans/plan-v2.1.3-audit.md` appendix. The gap list drives Phase 2.

### Phase 1 — Go backend (`PATCH /api/memories/{id}`)

1. Add the handler to `internal/ui/server.go` (after `POST /api/memories/{id}/forget`, ~line 1395).
2. Tests in `internal/ui/server_test.go`:
   - `TestPatchMemoryOK` — PATCH updates content + tags; response `ok: true`.
   - `TestPatchMemoryReindexed` — content change → new embedding queued (verify `embed_queue` row for the id).
   - `TestPatchMemoryTagsOnly` — tags-only change → no new embed queue row.
   - `TestPatchMemoryNotFound` — 404.
   - `TestPatchMemoryEmptyContent` — 400.
   - `TestPatchMemoryStoreNil` — 503.
3. `go build -tags fts5 ./...` + `go test -tags fts5 ./internal/ui/... -race`.

### Phase 2 — Frontend build session (separate)

1. Reconstruct/obtain Vite source (same process as v2.1.0 Phase 2).
2. Add `d` delete alias (same handler as Del/Backspace).
3. Add `e` → drawer edit mode with the states in §4.3.
4. Implement guard rules (§4.2) — focus-element suppression, edit-mode inertness, route scoping.
5. Update `?` overlay with the grouped layout (§4.4).
6. Fix any broken shortcuts found in Phase 0.
7. Rebuild and drop into `internal/ui/dist/`.
8. Manual acceptance checklist (below).

### Phase 3 — Docs & verification

1. Update `docs/ui.md` §5 (add `d`, `e`; note `e` opens drawer edit mode) and the API table (add `PATCH /api/memories/:id`).
2. Update `docs/v2-nice-to-have.md` item 4 status: mark as implemented / note the "pure frontend" claim was superseded by the PATCH endpoint.
3. `graphify update .`.
4. Full suite: `go test -tags fts5 ./... -race`.

---

## 6. Manual Acceptance Checklist

- [ ] `/` focuses search from anywhere on the Memory Browser page.
- [ ] `j`/`k` (and arrows) move selection; visible focus ring follows.
- [ ] `Enter` opens the drawer for the selected row.
- [ ] `e` opens the drawer in edit mode with prefilled content.
- [ ] Editing content + Save → PATCH succeeds → toast → view mode shows new content.
- [ ] Tags can be added/removed in edit mode; Save persists.
- [ ] `Esc` in edit mode discards changes and returns to view mode.
- [ ] `d` shows the same confirm dialog as Del/Backspace; confirm deletes; Undo toast appears.
- [ ] `?` shows the updated grouped overlay; `?`/`Esc`/click-outside dismisses it.
- [ ] Typing in the search box does NOT trigger j/k/d/e.
- [ ] `Cmd+,` opens Settings.
- [ ] Shortcuts do not fire on Assistant Chat / Proposals tabs.
- [ ] Dark mode renders the new edit-mode controls correctly.

---

## 7. Test Strategy

| Layer | Tests |
|-------|-------|
| `internal/ui` (Go) | 6 endpoint tests (§5 Phase 1) |
| Frontend build session | Component tests for drawer edit mode (save/cancel/dirty/error states) + keydown handler unit tests with a mocked DOM for guard rules |

---

## 8. Dependencies & Risks

| Item | Risk | Mitigation |
|------|------|-----------|
| Compiled bundle is opaque | Medium — static grep may be inconclusive | Manual walkthrough is authoritative; grep is a hint |
| docs/ui.md stale (shortcuts exist on paper only) | Medium | Phase 0 audit catches it; fix list feeds Phase 2 |
| `e` contradicts "pure frontend" signal | Low — plan is explicit | Backend endpoint is a thin wrapper over existing store function |
| Edit of `fact` memories (key/value pairs) | Medium — editing `content` of a fact may desync `key`/`value_json` | V1: allow editing any type but show a warning banner for `fact` type; dedicated fact-key editing is out of scope |
| Drawer edit-mode state leaks after close | Low | State reset on drawer unmount (explicit in component spec) |
| Key interception breaks existing input UX | High if guards missing | Guard rules in §4.2 are acceptance-gated (checklist items) |

---

## 9. Acceptance Criteria (Done Definition)

- [ ] Phase 0 audit table is filled with evidence for all 9 documented + 2 new shortcuts.
- [ ] `PATCH /api/memories/{id}` passes all 6 Go tests.
- [ ] `d` and `e` work per the checklist; `?` overlay is updated.
- [ ] Guard rules hold (no shortcut firing while typing).
- [ ] `docs/ui.md` §5 + API table updated.
- [ ] `docs/v2-nice-to-have.md` item 4 marked with implementation status.
- [ ] `go test -tags fts5 ./... -race` passes; `graphify update .` run.

---

## 10. Future Enhancements (out of scope for v2.1.3)

- **Shortcut customization** — user-configurable key map in Settings.
- **Shortcuts on other tabs** — Assistant Chat (`Cmd+Enter` to send), Proposals (`a` approve / `r` reject).
- **Inline table editing** — edit content directly in the row.
- **Fact key editing** — dedicated UI for editing `key`/`value_json` pairs.
- **Global command palette** — `Cmd+K` palette (search, jump to scope, run actions).

---

## 11. Changelog

| Date | Change |
|------|--------|
| 2026-09-14 | Initial plan created via `/grill-me` session. Design locked. |
