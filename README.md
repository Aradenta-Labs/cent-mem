# cent-mem

Fast, local-first **shared memory for AI agents**. One `centmem` command, one shared brain — across Claude Code, Codex, Cursor, and any custom harness.

> **Status:** v1 in design/planning. This repo currently holds the full specification and implementation plan.

## What it does

- **Recall** relevant context (semantic + keyword + facts + timeline) in < 300 ms.
- **Store** decisions, preferences, facts, and session logs from any agent.
- **Scope** memory hierarchically: `global → project → agent → session`.
- **Auto-summarize** old memories to keep the store lean.
- **Offline** — local embeddings, no API keys required for core use.

## How it works

Every agent integrates via a **skill** that wraps a single **Go CLI**:

```
AI Agent ──► skill ──► centmem CLI ──► SQLite + sqlite-vec + FTS5
```

## Documentation

| Doc | Purpose |
|-----|---------|
| [docs/PRD.md](docs/PRD.md) | Requirements, goals, non-goals |
| [docs/architecture.md](docs/architecture.md) | Components & data flow |
| [docs/data-model.md](docs/data-model.md) | Schema & tables |
| [docs/cli-contract.md](docs/cli-contract.md) | CLI surface & JSON contract |
| [skill/SKILL.md](skill/SKILL.md) | The runtime contract agents read |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Milestones & ordering |
| [docs/plan-phase-0.md](docs/plan-phase-0.md) | M0 detailed plan + tests |
| [docs/plan-phase-1.md](docs/plan-phase-1.md) | M1 detailed plan + tests |
| [docs/plan-phase-2.md](docs/plan-phase-2.md) | M2 detailed plan + tests |
| [docs/plan-phase-3.md](docs/plan-phase-3.md) | M3 detailed plan + tests |
| [docs/plan-phase-4.md](docs/plan-phase-4.md) | M4 detailed plan + tests |
| [docs/release-plan.md](docs/release-plan.md) | Release & QA plan |
| [AGENTS.md](AGENTS.md) | Instructions for agents working on this repo |

## Quick reference (target CLI)

```bash
centmem init
centmem recall "how do we deploy" --scope project:myapp --top 5
centmem set --scope project:myapp --key user.timezone --value '"Asia/Jakarta"'
centmem put --scope project:myapp --type note --content "..." --tags decision
centmem timeline --scope project:myapp --since 24h
```

## Status / roadmap

- [ ] M0 — Foundation & toolchain
- [ ] M1 — Core store & CRUD (keyword + facts + timeline)
- [ ] M2 — Embeddings & hybrid search
- [ ] M3 — Skill packaging & harness adapters
- [ ] M4 — Retention, compaction & polish
- [ ] v1.0 release
- [ ] v1.1+ — auto-capture, TUI, sync server (v2)

See [docs/implementation-plan.md](docs/implementation-plan.md).
