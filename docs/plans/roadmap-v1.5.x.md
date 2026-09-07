# v1.5.x — Feature Roadmap

**Owner:** Aradenta Labs  
**Status:** In Execution — Individual milestone implementation plans defined  
**Depends on:** v1.4.x series shipped and stable  
**Session:** Derived from `/grill-me` brainstorm on 2026-09-07

---

## 0. Strategic Overview

v1.5.x transforms centmem from a manual agent memory store into a self-populating, widely-integrated, relationship-aware shared brain. Five themes, five milestones — each independently shippable with dedicated implementation plans.

| Milestone | Theme | Key Deliverable | Detailed Plan |
|---|---|---|---|
| **v1.5.0** | Search & Recall | Importance scoring (access-frequency boost) | [plan-v1.5.0.md](plan-v1.5.0.md) |
| **v1.5.1** | Richer Capture | Git, docs, shell, code comment capture | [plan-v1.5.1.md](plan-v1.5.1.md) |
| **v1.5.2** | Memory Relationships | Link graph + auto-suggest | [plan-v1.5.2.md](plan-v1.5.2.md) |
| **v1.5.3** | Integrations | MCP server + client, remote REST, VS Code | [plan-v1.5.3.md](plan-v1.5.3.md) |
| **v1.5.4** | Sync | `centmemd` gRPC daemon | [plan-v1.5.4.md](plan-v1.5.4.md) |

---

## 1. Feature Priority & Sequencing Matrix

| Feature | Impact | Effort | Ship Order | Milestone |
|---|---|---|---|---|
| Importance scoring | High | Low | 1st | v1.5.0 |
| MCP server (stdio transport) | Very High | Medium | 2nd | v1.5.3 |
| `capture git` | High | Medium | 3rd | v1.5.1 |
| Memory relationships & links | Medium | Medium | 4th | v1.5.2 |
| Remote REST + token auth | Medium | Low | 5th | v1.5.3 |
| `capture docs` | Medium | Low | 6th | v1.5.1 |
| VS Code extension | Medium | High | 7th | v1.5.3 |
| MCP client (capture enrichment) | Low | Medium | 8th | v1.5.3 |
| `capture shell` | Low | Low | 9th | v1.5.1 |
| `capture comments` | Low | Low | 10th | v1.5.1 |
| `centmemd` daemon | Very High | Very High | Last | v1.5.4 |

**Sequencing Rationale:**
1. **v1.5.0 (Importance Scoring)** ships first: Zero new CLI commands or UX learning curves, improves search quality immediately, low blast radius.
2. **v1.5.3 (Integrations - MCP)** is prioritized next: Unlocks all MCP-compatible AI agent harnesses (Claude Code, Cursor, Windsurf) without requiring custom CLI adapter installs.
3. **v1.5.1 (Richer Capture)** populates the store automatically from Git commits, Markdown docs, shell history, and source code annotations.
4. **v1.5.2 (Memory Relationships)** introduces structural edges (`supports`, `refines`, `contradicts`, `depends-on`, `supersedes`) between memories.
5. **v1.5.4 (Sync / Daemon)** delivers process decoupling, zero-lock SQLite concurrency, and foundations for multi-machine synchronization.

---

## 2. Milestone Summaries

### v1.5.0 — Search & Recall: Importance Scoring
*Plan:* [docs/plans/plan-v1.5.0.md](plan-v1.5.0.md)
- Adds `access_count` and `last_accessed_at` tracking to `memories` table.
- Non-blocking asynchronous access recording during `centmem recall`.
- Logarithmic score multiplier applied during retrieval:
  $$\text{importance} = \min\left(2.0,\, 1.0 + \ln(1 + \text{access\_count}) \times 0.1\right)$$
- Exposes access metrics in `recall` JSON, `stats`, and Web UI.

### v1.5.1 — Richer Capture: Git, Docs, Shell, Comments
*Plan:* [docs/plans/plan-v1.5.1.md](plan-v1.5.1.md)
- Four specialized capture subcommands:
  - `centmem capture git`: Incremental commit message & PR parsing via `.centmem/git-cursor`.
  - `centmem capture docs`: Heading-delimited Markdown/RST chunking via `.centmem/docs-cursor`.
  - `centmem capture shell`: Command frequency & pattern extraction from zsh, bash, and fish histories.
  - `centmem capture comments`: Inline code annotation extraction (`TODO`, `FIXME`, `HACK`, `SECURITY`).
- Shared cursor infrastructure, dry-run simulation, and secret scrubbing.

### v1.5.2 — Memory Relationships: Link Graph
*Plan:* [docs/plans/plan-v1.5.2.md](plan-v1.5.2.md)
- New relational table `memory_links` with cascading deletes.
- CLI commands: `centmem link`, `unlink`, `links`, `link confirm`, `link dismiss`.
- Auto-suggestion on `centmem put` using similarity heuristics and linguistic transition cues.
- Relationship context expansion in `centmem recall --include-links`.
- Visual relationship browser and confirmation badges in Web UI.

### v1.5.3 — Integrations: MCP Server & Client, Remote REST, VS Code
*Plan:* [docs/plans/plan-v1.5.3.md](plan-v1.5.3.md)
- Model Context Protocol (MCP) server over `stdio` implementing `centmem_recall`, `centmem_put`, `centmem_set`, `centmem_get`, `centmem_timeline`, `centmem_stats`, `centmem_forget`.
- MCP client support in classifier pipeline to enrich capture context via external tools.
- Remote REST API binding (`0.0.0.0`) guarded by mandatory bearer token auth (`--token` / `CENTMEM_UI_TOKEN`).
- Lightweight VS Code sidebar extension with real-time selection context recall and one-click memory capture.

### v1.5.4 — Sync: `centmemd` Daemon
*Plan:* [docs/plans/plan-v1.5.4.md](plan-v1.5.4.md)
- Long-running background daemon owning SQLite handle and embedder queue exclusively.
- Unix domain socket IPC (`~/.centmem/centmemd.sock`) and optional gRPC TCP (`:50051`).
- Transparent client delegation in CLI: auto-detects running daemon with graceful fallback to direct SQLite access.
- Replication primitives streaming from `events` table with deterministic conflict resolution (LWW for facts, append-only for notes/logs).

---

## 3. Constraints & Non-Goals

1. **CLI Contract Stability:** All new commands follow existing JSON and exit-code conventions. No existing command schema changes shape.
2. **Offline-First Non-Negotiable:** Zero runtime network calls except for explicit cloud classifier fallbacks or remote REST endpoints.
3. **Performance SLA:** p95 read < 300 ms @ 100k memories; p95 write overhead < 50 ms.
4. **Process Isolation:** MCP server runs as a standalone process and does not require `centmemd` daemon.
5. **No Full-Codebase File Indexing:** centmem stores *agent working memory*, not an AST codebase index (that role belongs to tools like graphify).
