# Phase 5 (v1.4.0) — centmem Web UI (Memory Browser Dashboard)

**Version:** 1.4.0 (target)
**Owner:** Aradenta Labs
**Mode:** Operate (dashboard) — see [impeccable operate reference]
**Status:** Design direction locked; ready for phased implementation

---

## 0. Summary

A **browser-based dashboard** for centmem that lets end users browse, search, and understand all memories their AI agents have written. Single-page web app served by the `centmem` binary itself (`centmem ui` launches an embedded HTTP server). Light + minimal visual style; sidebar navigation with a scope tree; advanced filtering; real-time data from the existing Go backend.

**Design constraints applied:**
- **impeccable (Operate mode):** earned familiarity, consistent component vocabulary, restrained color, motion only for state change, skeleton loading, honest empty states.
- **antislop-ui:** no default gradients/glass/glow, restrained radius + shadow, icons from one real library (relevance-first), no emoji in UI, real data only (no filler), every control functional, responsive + keyboard-accessible.

---

## 1. Confirmed requirements (from discovery)

| Decision | Answer | Rationale |
|----------|--------|-----------|
| **Primary purpose** | Dashboard (Operate mode) | Users are *in a task* (managing/understanding memory), not being persuaded. |
| **Platform** | Web app only (browser-based) | No install; runs wherever centmem runs. |
| **Primary audience** | End users (individuals using their own AI agents) | Must be simple, friendly, and not require deep technical knowledge. |
| **Must-have feature** | Memory Browser (view/search/filter across scopes) | The core value proposition. |
| **Visual style** | Light + minimal (freshness, clarity) | Clean whites, subtle shadows, generous whitespace, soft accent. |
| **Layout** | Sidebar + Main Content | Sidebar for scope navigation, main area for memory list. |
| **Search/Filter** | Advanced: scope tree + filters panel | Full navigation tree + dedicated filter panel for date/type/tags/agent. |

### Anti-goals (explicit)
- Not an admin panel for multi-user/team settings (that's v2 sync-server territory).
- Not a marketing/landing page (that would be a separate Persuade-mode surface).
- Not a mobile app (responsive web only; desktop-first).
- No user login/auth in v1.4.0 (single-user local tool; API key auth is v2).

---

## 2. Design direction (impeccable "new-work" applied)

Since there is **no existing UI**, this is a new visual world. However, per impeccable's guidance for **Operate** mode, the direction must be *restrained and task-serving*, not expressive. The "world" is defined by clarity, density, and calm.

### 2.1 The one-sentence mechanism
> centmem is the shared brain of your AI agents; the UI is the quiet, trustworthy window into that brain — every memory visible, searchable, and explainable, with nothing decorative between the user and their data.

### 2.2 Visual authority — Light + Minimal (Restrained color strategy)
Per antislop-ui and impeccable's Operate-mode guidance:

- **Theme:** Light mode as the default (clarity for a data-heavy dashboard). **Working dark mode toggle** (since developer tools legitimately need it), not dark-by-default.
- **Color strategy:** **Restrained** — neutral base (cool grays/off-whites) + **one accent color** used only for primary actions, current selection, and active states. No accent color scattered across the page.
  - Suggested accent: a calm, fresh **teal/sky blue** (e.g. `#0ea5a4`-family or similar) — communicates "calm, trustworthy, memory/clarity" without the generic blue-purple gradient cliché. **Reason recorded:** teal conveys clarity/focus, and is not the over-used AI blue-purple default (antislop R-01).
  - Semantic palette: success, warning, error, info, each with a muted-but-legible token.
- **Surfaces:** Second neutral layer for the sidebar/toolbar (slightly cooler/warmer than content). No glassmorphism, no heavy blur, no decorative gradients.
- **Depth:** Shadows only as elevation markers (with offset + soft blur). Most elements sit flat (antislop R-12). A colored halo (zero-offset glow) is forbidden (craft-floor).
- **Radius:** Small, deliberate set (e.g. 4px for inputs, 6px for cards, 8px for modals). Not everything pill-shaped (antislop R-11). One generous radius may be used on the primary CTA only.
- **Glow:** Max 1 element, only as a focus accent (antislop R-13).

### 2.3 Typography (Operate mode)
- **One well-tuned sans family** for everything (headings, buttons, labels, body, data). No display/body pairing needed.
  - Candidate faces (not the clichéd Inter/Geist defaults without reason): **Geist Sans**, **IBM Plex Sans**, or **system-ui stack** for zero-dependency. Record reason: "a neutral, highly-legible UI face that disappears into the task."
  - **Monospace** used only for *code/data/IDs/paths* (not as an aesthetic costume).
- **Fixed rem scale** (not fluid clamp), ratio ~1.125–1.2 between steps.
- **Measure:** prose 65–75ch; tables/data can run denser (up to ~120ch).

### 2.4 Components — state-complete vocabulary (impeccable)
Every interactive component ships with: **default, hover, focus, active, disabled, loading, error**.
- **Skeleton loaders** for loading states (not bare spinners).
- **Empty states that teach** ("No memories yet — your agents haven't written anything. Run `centmem put …` or let an agent record a checkpoint."), never "No data available."
- **Consistent affordances** across the whole surface: same button shapes, same form-control vocabulary, same icon style.
- **Overlays escape containers** — use `<dialog>` / popover API / portal for dropdowns & modals (never clipped by `overflow: hidden`).

### 2.5 Icons (antislop R-04)
- **One real icon library** chosen for relevance, not default import. Recommended: **Lucide** is fine *if* we use it as a deliberate choice and only for icons that genuinely fit; otherwise consider **Phosphor** or a small authored SVG set.
- **No generic "AI" glyphs** (sparkle/star/magic/robot) as decoration.
- **No emoji in UI text.** Any mark is a real, relevant icon or none.
- Consistent stroke weight and size across the app.

### 2.6 Motion (impeccable Operate + antislop R-19)
- **150–250 ms** transitions; motion conveys state, not decoration.
- **No orchestrated page-load sequences** (users load into a task, not a show).
- **One authored moment** if any (e.g. a subtle reveal on first load), not scattered effects. Exponential ease-out from an already-visible default.
- No endless pulses/loops. No floating elements.

### 2.7 Antislop-ui compliance checklist (must all pass)
- Palette derived from a written brand identity / DESIGN.md, not default gradients. (R-01, R-29)
- Accent used at key moments only, not everywhere. (core Part 3)
- No decorative emoji in headings/bullets/buttons. (R-04)
- Section compositions vary with a declared RHYTHM dial (not one repeated template). (R-05)
- No bento-grid mosaic, fake terminal, three-pricing-columns, or meaningless colored left stripes. (R-05, R-01)
- Every nav item/interactive element has a real destination or visible "Coming soon." (R-24, R-26)
- Motion follows MOTION dial with written purpose; no loops. (R-19)
- Glass/glow/shadow/radius at dose caps, not page-wide defaults. (R-10–R-13)
- Layout built around the user's decision/task, not sidebar+stat-row+chart+table default. (C-3, R-20)
- Every number/feed/table row is real or a labeled placeholder; no invented metrics. (R-17, R-18, R-38)
- Empty form fields/table cells stay empty or use honest placeholders (`Your Name`, `email@example.com`), not fake-looking data (`John Doe`). (R-23, R-38)
- Empty/loading/error states name the cause and next action. (R-27)
- Holds up at every breakpoint, theme, and state; keyboard-only usable. (R-03, R-34, C-4)

---

## 3. Information architecture & UX flow

### 3.1 Primary screen anatomy (Sidebar + Main Content)

```
┌───────────────────────────────────────────────────────────────────────────┐
│ Top bar: centmem logo  |  global search input  |  theme toggle  |  health  │
├──────────────┬────────────────────────────────────────────────────────────┤
│              │  Breadcrumb: global > project:myapp > agent:claude          │
│  SIDEBAR     │  ┌──────────────────────────────────────────────────────┐  │
│              │  │ Filters panel (collapsible)                          │  │
│  Scope tree  │  │  Type [fact|note|log]  Tags [+]  Date range          │  │
│  ──────────  │  │  Agent [▾]           Session [▾]   [Clear]           │  │
│  ▸ global    │  └──────────────────────────────────────────────────────┘  │
│  ▸ project:X │                                                            │
│    ▸ agent:A │  Memory list (table or cards)                              │
│      ▸ sess1 │  ┌──────────────────────────────────────────────────────┐  │
│    ▸ agent:B │  │ type | content preview | tags | scope | updated | …   │  │
│  ▸ project:Y │  │ ...                                                    │  │
│              │  └──────────────────────────────────────────────────────┘  │
│  ──────────  │  Pagination / infinite scroll                               │
│  + New scope │                                                            │
└──────────────┴────────────────────────────────────────────────────────────┘
```

### 3.2 Key interactions
- **Scope tree** in sidebar: expandable, shows memory counts per scope. Clicking a scope filters the main view.
- **Global search** in top bar: full hybrid `recall` (semantic + keyword), respects current scope filter.
- **Filters panel**: collapsible; type/tags/date-range/agent/session filters. "Clear" resets.
- **Memory row/card**: click to open detail (side panel or modal), showing full content, metadata, related memories.
- **Breadcrumb** shows current scope path.
- **Health indicator** in top bar: `centmem doctor` status (green/yellow/red dot + tooltip).

### 3.3 States to design (must-have)
| State | Requirement |
|-------|-------------|
| **Loading** | Skeleton rows for memory list; skeleton tree for sidebar. |
| **Empty (first run)** | "No memories yet" with clear next action (link to docs / copy-paste command). |
| **Empty (filtered)** | "No memories match these filters" + Clear-filters button. |
| **Error (backend down)** | Banner: "centmem backend unreachable. Is `centmem ui` running?" + retry. |
| **Error (query failed)** | Inline error with cause + recovery. |
| **Permission/readonly** | (Future) disabled actions with tooltip. |
| **Keyboard** | Full navigation, focus rings themed from palette, `Esc` closes overlays. |

---

## 4. Technical architecture

### 4.1 Stack (chosen for speed + single-binary + offline)
| Layer | Choice | Why |
|-------|--------|-----|
| **Backend** | Go (existing `centmem`) | Add `centmem ui` subcommand that embeds + serves the frontend and exposes a small JSON HTTP API on top of the existing store/search. |
| **Frontend** | **React + Vite + TypeScript** (or **Svelte/SvelteKit**) | Fast build, small bundle, great DX. (Decision recorded below.) |
| **Styling** | **Tailwind CSS v4** + design tokens (CSS custom properties) | Utility-first, design-token friendly, avoids CSS bloat. |
| **State** | React Query (TanStack) or Svelte stores | Server-state caching, optimistic updates. |
| **Icons** | Lucide (chosen deliberately for relevance) or Phosphor | One consistent set. |
| **Build output** | Embedded into the Go binary via `go:embed` | Single static binary; no separate asset server. |

**Frontend framework decision:** Recommend **React + Vite + TypeScript** (ecosystem, hiring pool, mature libraries) **OR** **Svelte/SvelteKit** (smaller bundle, less JS shipped). Since the audience is end-users and bundle size + speed matter, and the team is small, **SvelteKit is a strong candidate** — but if the maintainers are more comfortable with React, React is acceptable. **Record the decision in this file before implementation.**

### 4.2 API surface (new, minimal)
Expose a small JSON HTTP API from `centmem ui` (local only, binds `127.0.0.1:PORT`):

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /api/scopes` | GET | List scope tree with counts. |
| `GET /api/memories` | GET | List/filter memories (query params: scope, type, tags, since, until, agent, q). |
| `GET /api/memories/:id` | GET | Full memory detail + related. |
| `GET /api/stats` | GET | Store stats (for health/overview). |
| `GET /api/health` | GET | Doctor status. |
| `POST /api/memories/:id/forget` | POST | Delete a memory (with confirm). |

**Security:** binds localhost only; no auth in v1.4.0 (single-user local tool). Add a warning in docs that it must not be exposed to a network.

### 4.3 Embedding the frontend
- Frontend built to static assets → `internal/ui/dist/`.
- Go serves via `go:embed` + `http.FileServer`.
- `centmem ui` starts the server and opens the default browser (`browser.OpenURL`).

---

## 5. Phased implementation plan

> Each phase is independently shippable and leaves the repo green. Follow the same discipline as Phases 0–4.

### Phase 5.0 — Foundation & Design System (design tokens + scaffold)

**Goal:** A running web scaffold with the design system (tokens, theme, base components) and the embedded-server plumbing, before any real features.

| Task | Deliverable |
|------|-------------|
| Scaffold `ui/` (Vite + React/TS or Svelte) | `ui/` folder builds to `internal/ui/dist/` |
| Design tokens (`tokens.css`) | CSS vars for palette, spacing, radius, shadows, typography, dark mode |
| Base components | Button, Input, Select, Badge, Card, Skeleton, Modal/Dialog, Toast |
| Theme toggle | Light/dark with persistence (localStorage) + `prefers-color-scheme` default |
| `centmem ui` command | Embedded server + browser open |
| Design-system page (storybook-lite) | `/ui/design-system` route showing all components + states |

**Tests:** token presence, theme toggle persists, `centmem ui` serves index, component render tests.

---

### Phase 5.1 — Layout + Scope Tree + Sidebar

**Goal:** The dashboard shell with working scope-tree navigation.

| Task | Deliverable |
|------|-------------|
| Top bar | Logo, global search input, theme toggle, health indicator |
| Sidebar | Scope tree (expandable, memory counts), "New scope" action |
| Main layout | Responsive sidebar (collapsible on narrow), breadcrumb |
| `GET /api/scopes` | Backend endpoint returning tree + counts |

**Tests:** scope tree renders from API, expand/collapse works, selecting scope updates breadcrumb + URL, sidebar collapses on mobile.

---

### Phase 5.2 — Memory Browser (list + filters + detail)

**Goal:** The core feature — browse, search, filter, and inspect memories.

| Task | Deliverable |
|------|-------------|
| `GET /api/memories` | Backend endpoint with filters |
| Memory list | Table (dense, scannable) with columns: type, content preview, tags, scope, updated |
| Filters panel | Type, tags, date range, agent, session; Clear; collapsible |
| Global search | Hybrid `recall` via `?q=` |
| Memory detail | Side panel (or modal) with full content + metadata |
| Loading/empty/error states | Skeleton, teaching empty state, error banner |

**Tests:** list renders; each filter works alone + combined; search returns semantic matches; detail opens; all three states render correctly.

---

### Phase 5.3 — Stats, Health & Polish

**Goal:** Overview stats, health check, and the quality/craft-floor polish pass.

| Task | Deliverable |
|------|-------------|
| `GET /api/stats` | Counts by type/scope, size, pending embeddings |
| Overview panel | On main view when no filters active: total memories, by type, recent activity |
| `GET /api/health` + indicator | Doctor status in top bar |
| Accessibility pass | Keyboard nav, focus rings, contrast (≥4.5:1 body / ≥3:1 large text) |
| Craft-floor pass | Spacing, typography, states, browser-surface theming (selection, caret, scrollbars, focus rings, underline offset, tabular numerals) |
| Antislop-ui final audit | Run the full checklist (section 2.7) |

**Tests:** stats accurate; health reflects backend; axe/a11y clean; contrast verified; keyboard-only walkthrough passes.

---

### Phase 5.4 — Actions & Safety (forget, maybe export)

**Goal:** Safe destructive/action flows.

| Task | Deliverable |
|------|-------------|
| `POST /api/memories/:id/forget` | Delete with confirmation dialog |
| Confirmation dialog | Names the consequence; undo if feasible |
| (Optional) Export | `GET /api/export?scope=...` → JSON/CSV download |

**Tests:** delete confirms before acting; list updates after delete; export downloads correct data.

---

### Phase 5.5 — Build, Release, Docs

**Goal:** Ship v1.4.0.

| Task | Deliverable |
|------|-------------|
| Production build → `go:embed` | Single binary with UI embedded |
| Update release workflow | Build frontend in CI before Go build |
| Docs | `docs/ui.md` (how to use, screenshots), update README + cli-contract + CHANGELOG |
| `v1.4.0` tag + release | Binaries with embedded UI |

**Tests:** release binary serves UI; `centmem ui` works on all 4 platforms; docs screenshots match.

---

## 6. Testing plan (per phase, mapped to quality gates)

### 6.1 Functional tests
| Test | Validates |
|------|-----------|
| Scope tree loads from `/api/scopes` | Backend + frontend integration |
| Memory list renders with real data | No filler/mock data (antislop R-38) |
| Each filter + combinations | Filter logic correct |
| Hybrid search returns semantic match | `recall` wired to UI |
| Memory detail shows full content | Detail view complete |
| Forget requires confirmation + works | Safety |
| All states (loading/empty/error) render | State coverage |

### 6.2 Design/antislop-ui tests
| Test | Validates |
|------|-----------|
| Palette tokens match DESIGN.md | No default gradients (R-01) |
| Accent color count ≤ key moments | Accent restraint |
| No emoji in UI text | (R-04) |
| All nav items have real destinations | (R-24) |
| All controls functional | (R-26) |
| No endless motion loops | (R-19) |
| Shadow/radius/glass within dose caps | (R-10–R-13) |
| Empty/loading/error states informative | (R-27) |
| Responsive at all breakpoints + keyboard-only | (R-03, R-34, C-4) |
| No invented metrics/filler data | (R-17, R-18, R-38) |

### 6.3 Craft-floor checks (impeccable)
| Check | Threshold |
|-------|-----------|
| Contrast | Body ≥4.5:1, large text ≥3:1 |
| Shadows | Offset + soft blur; no zero-offset halo |
| Spacing | More space above heading than below |
| Type | Measure 65–75ch; display ≤6rem; tracking ≥-0.04em |
| Motion | 150–250ms, exponential ease-out, one authored moment |
| States | hover/focus/active/disabled/loading/error/empty all present |
| Browser surfaces | Themed selection, caret, scrollbar, focus ring, tabular numerals |
| Copy | Controls name actions; errors name problem + recovery |
| Coverage | Every requirement findable in seconds |

### 6.4 Visual regression / finish review (impeccable)
- Batched screenshot round (desktop + one narrow breakpoint) after each phase's "Inspect & finish."
- One batched defect fix pass, then confirm; max two rounds (impeccable discipline).
- On completion, run a **finish reviewer** pass against the direction contract.

---

## 7. Definition of Done (v1.4.0)

- [ ] `centmem ui` serves a working, responsive dashboard from a single binary on all 4 platforms.
- [ ] Memory Browser (list + search + filters + detail) fully functional with real data only.
- [ ] Scope tree navigation works with counts.
- [ ] All states (loading/empty/error) implemented and informative.
- [ ] Light + dark theme toggle works and persists.
- [ ] Antislop-ui checklist (section 2.7) 100% pass.
- [ ] Craft-floor checks (section 6.3) all green.
- [ ] Accessibility: contrast + keyboard nav + focus states verified.
- [ ] `docs/ui.md` written with screenshots; README/CHANGELOG/cli-contract updated.
- [ ] `v1.4.0` tagged; release binaries embed the UI.

---

## 8. Open decisions to lock before Phase 5.0

| # | Decision | Recommendation |
|---|----------|----------------|
| D1 | **Frontend framework** | SvelteKit (small bundle, fast) or React+Vite (ecosystem). **Pick one and record it here.** |
| D2 | **Accent color** | Teal (`#0ea5a4`-family). Confirm or choose from a small swatch set. |
| D3 | **Primary sans font** | System-ui (zero-dep) vs Geist Sans vs IBM Plex Sans. Record reason. |
| D4 | **Icon library** | Lucide (deliberate choice) vs Phosphor. Record reason. |
| D5 | **Default port** | e.g. `127.0.0.1:4231` (configurable via flag/env). |
| D6 | **Table vs cards** for memory list | Table (dense, scannable) recommended for Operate mode. Confirm. |

---

## 9. References

- [docs/PRD.md](PRD.md) — v1 product context
- [docs/cli-contract.md](cli-contract.md) — CLI contract the UI consumes via `centmem ui`
- [docs/architecture.md](architecture.md) — backend components the UI wraps
- impeccable references: operate.md, new-work.md, shape.md, craft-floor.md (`.agents/skills/impeccable/reference/`)
- antislop-ui rules (`.agents/skills/antislop-ui/`)
