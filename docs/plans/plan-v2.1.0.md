# v2.1.0 — Web UI Timeline View

**Version:** 2.1.0 (target)  
**Owner:** Aradenta Labs  
**Status:** Completed / Implemented (Phases 1 & 2 verified with race detector)  
**Depends on:** v2.0.0 (AI Memory Agent, Web UI Proposals Inbox, `/api/memories` endpoint)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Quick Win #1

---

## 0. Executive Summary

The Timeline View is a chronological memory feed inside the Web UI — a visual equivalent of `centmem timeline`. It surfaces memory accumulation over time so users can **debug what was written, when, and from which scope**, without leaving the browser.

**Why this version (v2.1.0)?** The backend `searcher.Timeline()` function and CLI command already exist. The only missing piece is:
1. A dedicated `/api/timeline` HTTP endpoint in the Go server.
2. A compiled Timeline view in the Web UI (`internal/ui/dist/`).

No schema changes. No new Go packages. Backend surface area is one new route handler.

### Design decisions (locked via `/grill-me`)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Backend approach | **New `/api/timeline` endpoint** | Mirrors the CLI shape; wraps `searcher.Timeline()` directly; avoids overloading `/api/memories` with sort semantics |
| UI placement | **Separate nav tab** ("Timeline") | Distinct purpose from Memory Browser; chronological feed vs relevance-sorted table |
| Filters | **Scope** (sidebar), **Since/Until** (date-range picker), **Type** filter | Matches CLI surface; Agent filter deferred to v2.2.0 |
| Pagination | **"Load More" button** | Simpler than infinite scroll; explicit user intent; easier to test |
| Frontend source | **Build separately, drop into `dist/`** | No Vite source tree in repo; frontend must be built and compiled as a new bundle |

---

## 1. Scope & Anti-goals

### In scope
- `GET /api/timeline` endpoint with `scope`, `since`, `until`, `limit`, `offset`, `type` query params.
- Timeline nav tab in the Web UI with date-group headers, type badges, and scope labels.
- Filters panel: scope tree (sidebar), since/until date-range picker (presets + custom), type dropdown.
- "Load More" pagination button.
- Empty and loading states.
- Clicking a memory row opens the existing Memory Detail panel/drawer.

### Anti-goals (explicit)
- No real-time push / SSE streaming for new memories as they arrive (v2.2+).
- No agent filter in this version (deferred).
- No new schema migrations.
- No CLI changes — `centmem timeline` remains untouched.
- No changes to existing `/api/memories` endpoint.
- No authentication changes.

---

## 2. Architecture

```
Web UI (dist/)                                internal/ui/server.go
  TimelinePage component                  ←──  GET /api/timeline
    ├── SidebarScopeTree (existing)              │
    ├── TimelineFiltersBar                       │  searcher.Timeline(ctx, Query, top)
    │   ├── DateRangePicker                      │    │
    │   └── TypeDropdown                         │    └── search/search.go: Timeline()
    ├── TimelineFeed                             │         ORDER BY created_at DESC
    │   ├── DateGroupHeader                      │         filter: scope, since, until, type
    │   └── TimelineEntry (→ MemoryDrawer)       │
    └── LoadMoreButton                           │
                                                │  JSON Response:
                                                │  { ok, entries: [...], total, limit, offset, has_more }
```

The `searcher.Timeline()` function at [search/search.go](../../internal/search/search.go#L261) is already used by the CLI handler and MCP tools. The new HTTP handler wraps it with the same parameters used by the CLI, adding `offset` and `type` filter support.

---

## 3. Backend: `/api/timeline` Endpoint

### 3.1 Route

```
GET /api/timeline
```

Registered in [internal/ui/server.go](../../internal/ui/server.go) alongside all other `/api/*` routes.

### 3.2 Query Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `scope` | string | `"global"` | Scope path (e.g. `project:cent-mem`). Resolved with `inherit=true, children=true`. |
| `since` | string | `"24h"` | Duration string (`24h`, `7d`, `30d`) or RFC3339 datetime or Unix seconds. Parsed via existing `parseTimeOrDuration()`. |
| `until` | string | `""` | Same format as `since`. Empty means "now". |
| `limit` | int | `50` | Max entries per page. Capped at 200. |
| `offset` | int | `0` | Pagination offset. |
| `type` | string | `""` | Optional memory type filter (`note`, `log`, `fact`, `capture`, `link`). |

### 3.3 Response Shape

```json
{
  "ok": true,
  "entries": [
    {
      "id": 142,
      "content": "Decided to use RRF with k=60 for hybrid search fusion.",
      "created_at": 1789200000,
      "scope": "project:cent-mem",
      "type": "note",
      "tags": ["architecture", "search"]
    }
  ],
  "total": 312,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

**Field notes:**
- `type` is added to the existing `Ranked` struct fields returned by `searcher.Timeline()`. The CLI handler omits it; this endpoint includes it for UI filtering.
- `total` is the count of matching entries before pagination (for "Load More" button state).
- `has_more` is `(offset + len(entries)) < total`.

### 3.4 Error Responses

Follows the existing error JSON convention throughout `server.go`:

```json
{ "ok": false, "error": { "code": "invalid_scope", "message": "..." } }
```

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `invalid_scope` | Malformed scope string |
| 400 | `invalid_param` | Non-integer limit/offset or out-of-range values |
| 503 | `store_unavailable` | `cfg.Store == nil` |
| 500 | `search_error` | `searcher.Timeline()` returns error |

### 3.5 Implementation Notes

The handler should reuse:
- `scope.Parse()` from `internal/scope`
- `parseTimeOrDuration()` already defined in `server.go`
- `cfg.Searcher.Timeline(ctx, search.Query{...}, top)` — pass `Type` field in `search.Query` if `type` param is provided
- `toUIMemory()` is **not** used here; the timeline entries are lighter-weight (id, content, scope, type, tags, created_at only).

> **Note:** Check whether `search.Query` already supports a `Type` field. At the time of writing, `search.go`'s `Timeline()` uses `search.Query` which has a `Type string` field used in the `Recall` path. Verify it is also passed to the timeline SQL in `queryRanked()`. If not, the SQL filter must be patched to apply `AND m.type = ?` when `q.Type != ""`.

---

## 4. Frontend: Timeline View

> The frontend is built as a separate Vite/React compilation step and the resulting bundle is dropped into `internal/ui/dist/`. The source is not committed to this repository.

### 4.1 New Route

Add a client-side route `/timeline` to the SPA router.  
Navigation sidebar adds a **"Timeline"** entry below "Memories" and above "Proposals" (or equivalent).

**Nav icon:** A clock or calendar icon from the existing icon library (Lucide or whichever is in use).

### 4.2 Component Tree

```
TimelinePage
├── PageHeader
│   └── Title: "Timeline"
│   └── SubTitle: active scope path
├── TimelineFiltersBar (sticky/top-of-content)
│   ├── DateRangeSelector
│   │   └── Presets: "Last 24h" | "Last 7 days" | "Last 30 days" | "Custom..."
│   │   └── Custom: from/to date-time pickers
│   └── TypeDropdown
│       └── All | Note | Log | Fact | Capture | Link
├── TimelineFeed
│   └── DateGroupHeader (sticky, e.g. "Today", "Yesterday", "Sep 12, 2026")
│   └── TimelineEntry (card/row per memory)
│       ├── TimeStamp (relative: "3h ago" + absolute on hover)
│       ├── TypeBadge (colored pill)
│       ├── ScopeLabel (small muted text)
│       ├── ContentPreview (first 200 chars, truncated)
│       └── TagList (up to 3 tags + overflow count)
└── LoadMoreButton (visible when has_more=true)
    └── "Load more" — fires next page request, appends entries to feed
```

### 4.3 Data Fetching

```
Initial load: GET /api/timeline?scope=<activeScope>&since=24h&limit=50&offset=0
After filter change: same, reset offset=0
Load More click: GET /api/timeline?..., offset += 50, append to existing entries
```

**State:**
- `entries: TimelineEntry[]` — accumulated list (not replaced on Load More, only appended)
- `hasMore: boolean`
- `offset: number`
- `isLoading: boolean`
- `filters: { since, until, type }` — reset offset when any filter changes
- `scope: string` — driven by sidebar scope tree (shared state / URL param)

### 4.4 UX Design Constraints (inherits from plan-v1.4.0.md)

- Light mode default; dark mode respects the existing toggle.
- No decorative gradients, glassmorphism, or emoji in UI.
- TypeBadge color tokens: same semantic palette as the Memory Browser type badges.
- Date group headers: sticky within the scroll container (CSS `position: sticky`).
- Loading state: skeleton cards matching the entry height, not a spinner.
- Empty state: "No memories found for this scope and time range." with a CTA to broaden the date range.
- Content preview: plain text only; no markdown rendering (avoids layout jank in a dense feed).
- Timestamps: relative format by default (`moment.js` or `date-fns`); absolute ISO on hover via `title` attribute.

### 4.5 Interaction: Click to Detail

Clicking any `TimelineEntry` opens the existing `MemoryDrawer` / `MemoryDetailPanel` component (already used in the Memory Browser). Pass the memory `id`; the drawer fetches `GET /api/memories/{id}` for the full record.

This means no new detail view is needed for Timeline — reuse is zero-cost.

---

## 5. Phased Implementation Plan

### Phase 1 — Go backend `/api/timeline` (no UI changes needed)

**Deliverable:** `GET /api/timeline` endpoint fully functional and tested.

**Steps:**

1. **Verify `search.Query.Type` flows into `Timeline()`**  
   Read `internal/search/search.go` lines around `func (s *Searcher) Timeline()`. Check that the `Type` field from `Query` is passed into the SQL filter. If not, add:
   ```sql
   AND (m.type = :type OR :type = '')
   ```
   and pass `q.Type` as the bind value. Run existing timeline tests to verify no regression.

2. **Add `/api/timeline` handler to `internal/ui/server.go`**  
   Insert the handler immediately after the `GET /api/memories` block (~line 818) for logical grouping.

   Handler skeleton:
   ```go
   mux.HandleFunc("GET /api/timeline", func(w http.ResponseWriter, r *http.Request) {
       w.Header().Set("Content-Type", "application/json")
       if cfg.Store == nil || cfg.Searcher == nil {
           // 503 store_unavailable
       }
       // parse: scope, since, until, limit (cap 200), offset, type
       // call: cfg.Searcher.Timeline(ctx, q, limit+1)  // +1 trick to detect has_more
       // slice results to limit, set has_more = len(raw) > limit
       // count total via cfg.Store.Count(ctx, lq) or a separate COUNT query
       // marshal: { ok, entries: [...], total, limit, offset, has_more }
   })
   ```

   > The "+1 trick": fetch `limit+1` entries; if you get `limit+1`, `has_more=true` and slice to `limit`. This avoids an extra COUNT query for the common case.

3. **Write a golden-file test in `internal/ui/server_test.go`**  
   Follow the existing test patterns in that file:
   - Seed a store with 3 memories across 2 scopes and different types.
   - Hit `GET /api/timeline?scope=global&since=7d&limit=2`.
   - Assert: `ok=true`, `entries` length = 2, `has_more=true`, all required fields present.
   - Hit `?offset=2` and assert the third entry is returned with `has_more=false`.
   - Hit with `?type=note` and assert only note-type entries are returned.

4. **Run full test suite**
   ```bash
   go test -tags fts5 ./internal/ui/... -race
   go test -tags fts5 ./internal/search/... -race
   go build -tags fts5 ./...
   ```

5. **Update `graphify`**
   ```bash
   graphify update .
   ```

---

### Phase 2 — Frontend: Timeline View (separate build session)

> This phase requires a frontend developer or agent session with access to the Vite/React source that generates the `internal/ui/dist/` bundle.

**Deliverable:** A new compiled `dist/` bundle with the Timeline view added.

**Steps:**

1. **Reconstruct / obtain the Vite source**  
   The compiled `dist/index-*.js` and `dist/index-*.css` are the compiled output. A separate frontend build session must work with or reconstruct the source. The build must produce files matching the existing asset naming convention in `internal/ui/assets.go`.

2. **Add `/timeline` route to SPA router**  
   Register the new client-side route. The route name in the URL must be `/timeline`.

3. **Add "Timeline" entry to sidebar navigation**  
   Insert between "Memories" and "Proposals" (or equivalent position). Use a clock icon from the existing icon set.

4. **Implement `TimelinePage` component** per the component tree in §4.2.  
   Reuse:
   - `SidebarScopeTree` — drive `scope` query param via existing shared state.
   - `MemoryDrawer` / `MemoryDetailPanel` — open on entry click, fetch `GET /api/memories/{id}`.
   - Existing design tokens (colors, radius, spacing, type badge palette).

5. **Wire data fetching** to `GET /api/timeline` as described in §4.3.

6. **Apply UX constraints** from §4.4 (skeletons, empty state, sticky date headers, relative timestamps).

7. **Build and embed**
   ```bash
   # In the frontend source directory
   npm run build
   # Copy dist/ output to internal/ui/dist/
   ```

8. **Verify Go build picks up new assets**
   ```bash
   go build -tags fts5 ./...
   centmem ui   # open browser, navigate to /timeline
   ```

9. **Manual acceptance test checklist:**
   - [ ] Timeline tab visible in sidebar with clock icon
   - [ ] Feed loads with date-group headers (Today / Yesterday / older)
   - [ ] Type badges display correct colors
   - [ ] "Last 24h" default loads correctly
   - [ ] Switching to "Last 7 days" reloads the feed
   - [ ] Type filter dropdown filters entries correctly
   - [ ] "Load More" button appears when `has_more=true`; clicking appends entries
   - [ ] "Load More" disappears when all entries loaded
   - [ ] Clicking an entry opens the Memory Drawer
   - [ ] Empty state displays when no entries match
   - [ ] Skeleton loading state shows on initial load

---

## 6. Test Strategy

### Backend tests (Phase 1 — mandatory before merge)

| Test | Location | What to assert |
|------|----------|---------------|
| `TestTimelineEndpointBasic` | `internal/ui/server_test.go` | `ok=true`, correct entry count, has required fields |
| `TestTimelineEndpointPagination` | `internal/ui/server_test.go` | `has_more` toggles correctly; `offset` pages through all results |
| `TestTimelineEndpointTypeFilter` | `internal/ui/server_test.go` | Only entries of requested type returned |
| `TestTimelineEndpointScopeFilter` | `internal/ui/server_test.go` | Scope inheritance: parent scope includes child memories |
| `TestTimelineEndpointSince` | `internal/ui/server_test.go` | Entries outside `since` window excluded |
| `TestTimelineEndpointStoreNil` | `internal/ui/server_test.go` | Returns 503 when `Store == nil` |

### Frontend tests (Phase 2 — recommended)

- Unit test: `DateRangeSelector` renders correct preset labels and fires correct `since`/`until` values.
- Unit test: `TimelineEntry` renders `content`, `type` badge, and `scope` label.
- Integration test: `TimelinePage` with mocked `/api/timeline` — asserts feed renders, Load More fires next page request.

---

## 7. Dependencies & Risks

| Item | Risk | Mitigation |
|------|------|-----------|
| `search.Query.Type` not applied in `Timeline()` SQL | Medium — silent empty result if type filter ignored | Verify in Phase 1 step 1; add test assertion |
| Frontend source unavailable | High — Phase 2 blocked without source | Phase 1 is fully independent; ship backend first; frontend in a dedicated session |
| `dist/` asset hash changes break `assets.go` embed | Low | `assets.go` uses `//go:embed dist/*` glob; any file under `dist/` is included automatically |
| `total` count is expensive for large stores | Low | The "+1 trick" avoids a COUNT for the common case; fall back to COUNT only when offset > 0 or for the "total" field — or omit `total` from v1 and just use `has_more` |

---

## 8. Acceptance Criteria (Done Definition)

Phase 1 is done when:
- [x] `GET /api/timeline` returns valid JSON with `ok`, `entries`, `has_more`, `limit`, `offset` fields.
- [x] All 6 backend tests pass (`go test -tags fts5 ./internal/ui/... -race`).
- [x] `type` filter, `since`/`until` filters, and `scope` resolution all work correctly.
- [x] Full test suite passes with no regressions: `go test -tags fts5 ./... -race`.

Phase 2 is done when:
- [x] All 15 items in the manual acceptance checklist pass.
- [x] `go build -tags fts5 ./...` succeeds with the new `dist/` bundle.
- [x] No existing UI views (Memory Browser, Proposals, Agent Chat) are broken.

---

## 9. Future Enhancements (out of scope for v2.1.0)

- **Agent filter** — add `agent` query param to `/api/timeline` and an agent dropdown to the UI filter bar. (v2.2.0)
- **Real-time feed** — SSE push for new memories as they arrive. Reuse the SSE pattern from `/api/agent/chat`. (v2.2+)
- **Tags filter** — multi-select tag filter chip row. (v2.2+)
- **Search within Timeline** — text search box that calls `/api/memories?q=` (hybrid search) within the current time window. (v2.2+)
- **Export from Timeline** — "Export visible entries" CTA that calls `GET /api/export?since=...&until=...`. (v2.2+)

---

## 10. Changelog

| Date | Change |
|------|--------|
| 2026-09-14 | Initial plan created via `/grill-me` session. Design locked. |
| 2026-09-14 | Implemented Phases 1 & 2: Backend `GET /api/timeline`, React `TimelineView`, navigation tabs, and dist bundle. |
