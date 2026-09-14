# v2 Nice-to-Have Features

> Generated: 2026-09-14  
> Status: Ideas — not yet scheduled or scoped.  
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

### 4. Keyboard Shortcuts in Web UI

**What:** Standard power-user hotkeys in the memory browser: `j`/`k` to navigate, `/` to search, `e` to edit, `d` to delete, `?` to show help.

**Why:** The Web UI is mouse-only today. Keyboard navigation is table stakes for a developer tool.

**Signals:** Pure frontend; no backend or API changes.

---

## 🟡 Medium Effort — High Value

### 5. Taxonomy & Tag Clustering *(on roadmap)*

**What:** Detect and normalize fragmented tags across a scope — e.g. `db`, `database`, and `sqlite` clustered under a canonical tag. Surface conflicts via `centmem curate`.

**Why:** Tag drift silently degrades recall precision. Fixing it at the data layer improves every search query without any user action.

**Signals:** Already listed in [implementation-plan.md](implementation-plan.md). Feeds directly into the curate pipeline.

---

### 6. Memory Health & Quality Scoring *(on roadmap)*

**What:** Assign each memory a staleness/vagueness/redundancy score. Surface low-quality memories in the Web UI with "needs review" badges and in `centmem curate` output.

**Why:** Not all memories age equally. Scoring lets the system guide users toward maintenance rather than requiring manual audit.

**Signals:** Already listed in [implementation-plan.md](implementation-plan.md). Score could become an input to compaction priority.

---

### 7. Streaming Proposal Application

**What:** Stream progress over SSE when applying large batches of proposals — `POST /api/proposals/batch` emits incremental `{applied: N, total: M}` events; Web UI shows a progress bar.

**Why:** Applying proposals on a 500-memory scope currently blocks the UI until complete. Progress feedback makes the operation feel safe and cancellable.

**Signals:** Backend SSE pattern already exists in `/api/agent/chat`; reuse is straightforward.

---

### 8. Webhook / Notification Hooks

**What:** Fire a configurable webhook when agent proposals are created or applied. Config:

```toml
[hooks]
proposals_created = "https://hooks.slack.com/..."
proposals_applied = "https://n8n.example.com/webhook/..."
```

**Why:** Teams using centmem in multi-agent workflows have no way to receive external notifications when the agent proposes changes. Webhooks unlock Slack alerts, n8n flows, and custom dashboards at zero runtime cost.

**Signals:** No new dependencies; fire-and-forget HTTP POST; very composable.

---

## 🔴 High Effort — Strategic

### 9. Cross-Session Working Memory

**What:** Persist conversation history for `centmem ask` sessions across restarts. A `--session <id>` flag resumes a named thread; the session is stored in SQLite and recalled on next invocation.

**Why:** Currently every `centmem ask` starts cold. Agents that work across multiple invocations (e.g. overnight runs) lose all context on restart. A session layer would let long-running agent tasks accumulate context incrementally.

**Signals:** Requires a new `sessions` table and a thread-replay mechanism in `engine.go`. Medium schema impact, high UX impact.

---

### 10. Memory Versioning / Edit History

**What:** A lightweight `revisions` table stores previous `content` snapshots with timestamps whenever a memory is updated or compacted. A `centmem history <id>` command and Web UI diff view expose the audit trail.

**Why:** Once a memory is updated or merged by compaction, the prior version is irrecoverable. This is a correctness gap for high-stakes knowledge (architectural decisions, security notes).

**Signals:** Schema additive; compaction and `put` paths need a `saveRevision()` hook. Storage cost is manageable with a configurable max-revisions-per-memory cap.

---

### 11. Distributed Multi-Machine Sync *(on roadmap for v2.1)*

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
