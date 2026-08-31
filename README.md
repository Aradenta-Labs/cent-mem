# cent-mem

Fast, local-first **shared memory for AI agents**. One `centmem` command, one shared brain — across Claude Code, Codex, Cursor, and any custom harness.

> **Status:** v1.0.0 — implemented and releasable.

## What it does

- **Recall** relevant context (semantic + keyword + facts + timeline) in < 300 ms.
- **Store** decisions, preferences, facts, and session logs from any agent.
- **Scope** memory hierarchically: `global → project → agent → session`.
- **Auto-summarize** old memories (`compact`) to keep the store lean.
- **Self-check** with `doctor`, and **snapshot/restore** with `backup` / `restore`.
- **Offline** — local embeddings, no API keys required for core use.

## Quickstart

From zero to your first recall in under a minute:

```bash
# 1. Build (or download a release binary for your OS)
go build -tags fts5 -o centmem ./cmd/centmem
sudo mv centmem /usr/local/bin/

# 2. Initialize: creates ~/.centmem and downloads the default model (one-time)
centmem init

# 3. Store something
centmem set --scope project:myapp --key user.timezone --value '"Asia/Jakarta"'
centmem put --scope project:myapp --type note --content "we deploy via github actions" --tags decision

# 4. Recall it later
centmem recall "how do we deploy" --scope project:myapp --top 5
```

Output is JSON on stdout (errors are JSON on stderr). See [docs/getting-started.md](docs/getting-started.md) for the full walkthrough and [docs/troubleshooting.md](docs/troubleshooting.md) for common issues.

## How it works

Every agent integrates via a **skill** that wraps a single **Go CLI**:

```
AI Agent ──► skill ──► centmem CLI ──► SQLite + sqlite-vec + FTS5
```

## Documentation

| Doc | Purpose |
|-----|---------|
| [docs/getting-started.md](docs/getting-started.md) | Step-by-step install for first-time users |
| [docs/troubleshooting.md](docs/troubleshooting.md) | Common issues & fixes |
| [docs/PRD.md](docs/PRD.md) | Requirements, goals, non-goals |
| [docs/architecture.md](docs/architecture.md) | Components & data flow |
| [docs/data-model.md](docs/data-model.md) | Schema & tables |
| [docs/cli-contract.md](docs/cli-contract.md) | CLI surface & JSON contract |
| [skill/SKILL.md](skill/SKILL.md) | The runtime contract agents read |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Milestones & ordering |
| [docs/release-plan.md](docs/release-plan.md) | Release & QA plan |
| [AGENTS.md](AGENTS.md) | Instructions for agents working on this repo |

## Commands

```bash
centmem init                 # setup + model download
centmem put --scope <s> ...  # write a note/log memory
centmem set --scope <s> ...  # upsert a fact
centmem get --scope <s> ...  # fetch a fact
centmem recall <q> --scope <s>  # hybrid search
centmem timeline --scope <s> # chronological logs
centmem list --scope <s>     # browse memories
centmem forget --id N        # delete a memory
centmem compact [--dry-run]  # summarize + archive old memories
centmem doctor               # health checks
centmem backup --to <path>   # snapshot DB
centmem restore --from <path># restore DB
centmem stats                # store summary
```

## Status / roadmap

- [x] M0 — Foundation & toolchain
- [x] M1 — Core store & CRUD (keyword + facts + timeline)
- [x] M2 — Embeddings & hybrid search
- [x] M3 — Skill packaging & harness adapters
- [x] M4 — Retention, compaction & polish
- [x] v1.0 release
- [ ] v1.1+ — auto-capture, TUI, sync server (v2)

See [docs/implementation-plan.md](docs/implementation-plan.md).
