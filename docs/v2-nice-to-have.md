# v2 Nice-to-Have Features

> Generated: 2026-09-14  
> Status: Planned — Phased implementation plans and versioning roadmap defined in [docs/plans/roadmap-v2.x.md](plans/roadmap-v2.x.md).  
> For the official roadmap, see [docs/implementation-plan.md](implementation-plan.md).

This document captures feature ideas that are not blockers for the current milestone but would meaningfully improve the product. Items are grouped by implementation effort and each includes a rationale and priority signal.

---

## 🟢 Low Effort — Quick Wins

### 1. Web UI Timeline View ✅ *(Implemented in v2.1.0)*

**Plan & Implementation:** [docs/plans/plan-v2.1.0.md](plans/plan-v2.1.0.md)

**What:** A chronological memory feed inside the Web UI, filterable by scope, type, and date range (`24h`, `7d`, `30d`, `custom`) — the visual equivalent of `centmem timeline`.

**Why:** Users have no way to visually audit memory accumulation. The CLI command already exists; the Web UI just needs a view wired to `/api/timeline`.

**Signals:** Immediately useful for debugging; low code surface; shipped in v2.1.0.

---

### 2. `centmem recall --interactive` Interactive TUI ✅ *(Implemented in v2.1.1)*

**Plan & Implementation:** [docs/plans/plan-v2.1.1.md](plans/plan-v2.1.1.md)

**What:** A `fzf`-style fuzzy browser for the terminal — `centmem recall --interactive` — that lets developers navigate, preview, and open memories without leaving the shell.

**Why:** Power users spend most of their time in the terminal. An interactive recall browser lowers the friction of the read loop significantly during local development.

**Signals:** Shipped in v2.1.1; built on Charmbracelet `bubbletea`/`lipgloss` with debounced hybrid search and viewport scrolling.

---

### 3. Memory Export / Import ✅ *(Implemented in v2.1.2)*

**Plan & Implementation:** [docs/plans/plan-v2.1.2.md](plans/plan-v2.1.2.md)

**What:** `centmem export --scope project:X --format json` dumps memories to a portable JSON file. `centmem import <file>` ingests them into any centmem instance.

**Why:** Users have no backup mechanism today. Also unlocks team onboarding — share a memory snapshot with a new contributor.

**Signals:** Shipped in v2.1.2; complete with `internal/export` streaming codec, CLI `export` and `import` commands, `POST /api/import` Web UI upload, and browser Import toolbar button.

---

### 4. Keyboard Shortcuts in Web UI ✅ *(Implemented in v2.1.4)*

**Plan & Implementation:** [docs/plans/plan-v2.1.4.md](plans/plan-v2.1.4.md)

**What:** Standard power-user hotkeys in the memory browser: `j`/`k` to navigate, `/` to search, `e` to edit, `d` to delete, `?` to show help.

**Why:** Power-user keyboard navigation and fast in-drawer editing remove mouse friction across the Memory Browser.

**Signals:** Shipped in v2.1.4 with `PATCH /api/memories/{id}` backend route wrapping `store.UpdateMemoryContent`, in-drawer edit mode (content textarea, tag chips, `Cmd/Ctrl+Enter` save, `Esc` cancel), `d` delete alias, centralized input/modal focus guards, and a categorized `?` overlay. *(Note: the original "pure frontend" signal was superseded by adding `PATCH /api/memories/{id}` to support `e` editing.)*

---

## 🟡 Medium Effort — High Value

### 5. Taxonomy & Tag Clustering *(on roadmap — Planned for v2.2.0)*

**Plan:** [docs/plans/plan-v2.2.0.md](plans/plan-v2.2.0.md)

**What:** Detect and normalize fragmented tags across a scope — e.g. `db`, `database`, and `sqlite` clustered under a canonical tag. Surface conflicts via `centmem curate`.

**Why:** Tag drift silently degrades recall precision. Fixing it at the data layer improves every search query without any user action.

**Signals:** Already listed in [implementation-plan.md](implementation-plan.md). Feeds directly into the curate pipeline.

---

### 6. Memory Health & Quality Scoring *(on roadmap — Planned for v2.2.1)*

**Plan:** [docs/plans/plan-v2.2.1.md](plans/plan-v2.2.1.md)

**What:** Assign each memory a staleness/vagueness/redundancy score. Surface low-quality memories in the Web UI with "needs review" badges and in `centmem curate` output.

**Why:** Not all memories age equally. Scoring lets the system guide users toward maintenance rather than requiring manual audit.

**Signals:** Already listed in [implementation-plan.md](implementation-plan.md). Score could become an input to compaction priority.

---

### 7. Streaming Proposal Application ✅ *(Implemented in v2.1.5)*

**Plan & Implementation:** [docs/plans/plan-v2.1.5.md](plans/plan-v2.1.5.md)

**What:** Stream progress over SSE when applying large batches of proposals and auto-suggested links — `POST /api/proposals/batch/stream` and `POST /api/links/suggested/batch/stream` emit `start`, per-item `progress`, and `done` events; the Web UI renders a live progress bar, per-item error list, and mid-flight cancellation.

**Why:** Applying proposals on a 500-memory scope previously blocked the UI until complete. Real-time progress feedback and cancellation make large batch operations transparent and safe.

**Signals:** Shipped in v2.1.5 with `POST /api/proposals/batch/stream` and `POST /api/links/suggested/batch/stream` (keeping existing non-streaming `/batch` endpoints intact), pre-stream `400`/`503` validation, `r.Context().Done()` cancellation between items, and `BatchProposalConfirmDialog` / `ProposalsView` live SSE progress UI with `AbortController` and non-streaming fallback.

---

### 8. Webhook / Notification Hooks ✅ *(Implemented in v2.1.6)*

**Plan & Implementation:** [docs/plans/plan-v2.1.6.md](plans/plan-v2.1.6.md)

**What:** Opt-in, disabled-by-default HTTP webhook notifications for proposal lifecycle events (`proposals.created`, `proposals.applied`, `proposals.dismissed`) across CLI and Web UI, with optional HMAC-SHA256 signing (`X-Centmem-Signature-256`), bounded async non-blocking delivery with single 500ms retry on network error/`5xx`, `POST /api/config/test-hook`, and a Web UI Webhooks Settings tab. Config:

```toml
[hooks]
enabled = false
proposals_created = "https://hooks.slack.com/..."
proposals_applied = "https://n8n.example.com/webhook/..."
proposals_dismissed = ""
secret = ""
timeout_seconds = 5
```

**Why:** Teams using centmem in multi-agent workflows have no way to receive external notifications when the agent proposes changes. Webhooks unlock Slack alerts, n8n flows, and custom dashboards at zero runtime cost.

**Signals:** Shipped in v2.1.6 with zero external dependencies, strict no-op when disabled, bounded CLI shutdown flush (`2s`), aggregated batch event emission, and hot-reload on `PATCH /api/config`.

---

## 🔴 High Effort — Strategic

### 9. Cross-Session Working Memory *(Planned for v2.4.0)*

**Plan:** [docs/plans/plan-v2.4.0.md](plans/plan-v2.4.0.md)

**What:** Persist conversation history for `centmem ask` sessions across restarts. A `--session <id>` flag resumes a named thread; the session is stored in SQLite and recalled on next invocation.

**Why:** Currently every `centmem ask` starts cold. Agents that work across multiple invocations (e.g. overnight runs) lose all context on restart. A session layer would let long-running agent tasks accumulate context incrementally.

**Signals:** Requires a new `sessions` table and a thread-replay mechanism in `engine.go`. Medium schema impact, high UX impact.

---

### 10. Memory Versioning / Edit History *(Planned for v2.3.0)*

**Plan:** [docs/plans/plan-v2.3.0.md](plans/plan-v2.3.0.md)

**What:** A lightweight `revisions` table stores previous `content` snapshots with timestamps whenever a memory is updated or compacted. A `centmem history <id>` command and Web UI diff view expose the audit trail.

**Why:** Once a memory is updated or merged by compaction, the prior version is irrecoverable. This is a correctness gap for high-stakes knowledge (architectural decisions, security notes).

**Signals:** Schema additive; compaction and `put` paths need a `saveRevision()` hook. Storage cost is manageable with a configurable max-revisions-per-memory cap.

---

### 11. Distributed Multi-Machine Sync *(on roadmap — Planned for v2.5.0)*

**Plan:** [docs/plans/plan-v2.5.0.md](plans/plan-v2.5.0.md)

**What:** Remote gRPC event replication across centmem daemons + per-agent RBAC so teams can share a memory store across machines.

**Why:** The largest architectural gap between local-first and team-scale usage. Enables shared project memories without manual export/import.

**Signals:** Already listed in [implementation-plan.md](implementation-plan.md) as a v2.1 milestone. Requires careful CRDTs or conflict-resolution strategy.

---

## Priority Recommendation

If scoping the next sprint, these three deliver the highest value for the least effort:

| Rank | Feature | Why Now |
|------|---------|---------|
| 1 | **Web UI Timeline View** | Minimal code, immediately useful, zero backend work |
| 2 | **Memory Export / Import** | Unlocks backups and team onboarding — frequently requested use case |
| 3 | **Taxonomy & Tag Clustering** | Directly improves retrieval quality, which is the core value proposition |
