# Documentation Index

Welcome to the **cent-mem** documentation. This directory houses architectural specifications, operational guides, and milestone implementation plans.

---

## 🗺️ Documentation Map

### 1. Core Specifications & Contracts

These documents define the architectural invariants, schemas, and public API contracts. They serve as the single source of truth for implementation.

| Document | Description | Key Audience |
|---|---|---|
| [PRD.md](PRD.md) | Product requirements, goals, non-goals, and design constraints | All contributors & agents |
| [architecture.md](architecture.md) | System components, data flow, embedding pipeline, and hybrid search engine | Developers & architects |
| [data-model.md](data-model.md) | SQLite schema, tables, triggers, indexes, and retention rules | Database & store engineers |
| [cli-contract.md](cli-contract.md) | Public CLI commands, arguments, exit codes, and JSON schemas (v1.x contract) | CLI & skill integrators |
| [release-plan.md](release-plan.md) | Release process, cross-platform build artifacts, and packaging QA | Maintainers & CI/CD |

### 2. Guides

Practical walkthroughs and diagnostic runbooks for developers and operators.

| Document | Description |
|---|---|
| [guides/getting-started.md](guides/getting-started.md) | Step-by-step setup, configuration, and basic CLI usage walkthrough |
| [ui.md](ui.md) | Web UI Memory Browser dashboard guide (`centmem ui`) |
| [guides/mcp-setup.md](guides/mcp-setup.md) | Model Context Protocol (MCP) server setup for agent harnesses |
| [guides/daemon.md](guides/daemon.md) | Background `centmemd` daemon architecture, IPC sockets, and gRPC sync |
| [guides/capture-hooks.md](guides/capture-hooks.md) | Comprehensive guide for auto-capture from agent transcripts & hook adapters |
| [guides/troubleshooting.md](guides/troubleshooting.md) | Diagnostic workflows, common error recovery (model download, DB locks, vec0) |

### 3. Implementation Plans & Milestones

Roadmaps and per-phase specifications tracking project milestones from inception to release.

| Document | Milestone | Status | Description |
|---|---|---|---|
| [implementation-plan.md](implementation-plan.md) | Overview | Canonical | Milestone ordering, deliverables, and dependency graph |
| [plans/plan-v1.2.0.md](plans/plan-v1.2.0.md) | v1.2.0 | Completed | Frictionless UX: npx installer, slash commands, instruction injection |
| [plans/plan-v1.4.0.md](plans/plan-v1.4.0.md) | v1.4.0 | Completed | **Web UI Memory Browser Dashboard** (design + phased plan) |
| [plans/plan-v1.4.1.md](plans/plan-v1.4.1.md) | v1.4.1 | Completed | **Web UI Configuration & Settings** (design + phased plan) |
| [plans/plan-v1.4.4.md](plans/plan-v1.4.4.md) | v1.4.4 | Completed | Recall Accuracy Enhancements (Phase C Re-Ranking) |
| [plans/plan-v1.4.5.md](plans/plan-v1.4.5.md) | v1.4.5 | Completed | Smart Skills Installer |
| [plans/roadmap-v1.5.x.md](plans/roadmap-v1.5.x.md) | v1.5.x | Completed | **v1.5.x Feature Roadmap** (5 themes, priority matrix) |
| [plans/plan-v1.5.0.md](plans/plan-v1.5.0.md) | v1.5.0 | Completed | Search & Recall: Importance Scoring |
| [plans/plan-v1.5.1.md](plans/plan-v1.5.1.md) | v1.5.1 | Completed | Richer Capture (Git, Docs, Shell, Comments) |
| [plans/plan-v1.5.2.md](plans/plan-v1.5.2.md) | v1.5.2 | Completed | Memory Relationships: Link Graph & Auto-Suggest |
| [plans/plan-v1.5.3.md](plans/plan-v1.5.3.md) | v1.5.3 | Completed | Integrations (MCP Server/Client, Remote REST, VS Code) |
| [plans/plan-v1.5.4.md](plans/plan-v1.5.4.md) | v1.5.4 | Completed | Sync: `centmemd` gRPC Daemon |

---

## 📖 Recommended Reading Order

### For Developers & AI Agents Working on the Codebase

1. **[PRD.md](PRD.md)** — Understand what cent-mem is and why it exists.
2. **[architecture.md](architecture.md)** — Understand the 8 primary subsystems and data flow.
3. **[data-model.md](data-model.md)** — Review the SQLite schema and indexing strategies.
4. **[cli-contract.md](cli-contract.md)** — Understand the stable CLI JSON interface and exit codes.
5. **[skill/SKILL.md](../skill/SKILL.md)** — Understand how external AI agents interact with the CLI.
6. **[guides/capture-hooks.md](guides/capture-hooks.md)** — Deep dive on auto-capture and transcript hooks.
7. **[implementation-plan.md](implementation-plan.md)** — Review milestone history and roadmap.
8. **Active Phase Plan** (e.g. [plans/plan-v1.4.0.md](plans/plan-v1.4.0.md)) — Detailed tasks and test acceptance criteria for the current milestone.

---

## ⚖️ Governance & Invariants

1. **Stable CLI Contract:** Do not change JSON keys, command names, or exit codes without a version bump in [cli-contract.md](cli-contract.md) and [../skill/SKILL.md](../skill/SKILL.md). Verified by `scripts/check_contract.sh`.
2. **Schema Invariant:** SQLite migrations in `internal/store/migrations/` are the single source of truth. Keep [data-model.md](data-model.md) synchronized with schema updates.
3. **Strict JSON:** Commands emit JSON on `stdout`. Errors emit JSON on `stderr`.
