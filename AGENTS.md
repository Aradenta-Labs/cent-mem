# AGENTS.md — Working on cent-mem

This file tells any AI agent working **on this repo** what to do and how.

## What this project is

cent-mem is a fast, local-first shared memory store for AI agents. Agents integrate via a **skill** that wraps the `centmem` CLI (Go). All agents read/write the same memory, scoped by project > agent > session, retrieved via hybrid search (semantic + keyword + facts + timeline).

**Read these first, in order:**
1. [docs/PRD.md](docs/PRD.md) — requirements & goals
2. [docs/architecture.md](docs/architecture.md) — components & data flow
3. [docs/data-model.md](docs/data-model.md) — schema & tables
4. [docs/cli-contract.md](docs/cli-contract.md) — CLI surface & JSON shapes
5. [skill/SKILL.md](skill/SKILL.md) — the contract agents use at runtime
6. [docs/implementation-plan.md](docs/implementation-plan.md) — milestones & ordering
7. The **active phase plan** in [docs/plan-phase-0.md](docs/plan-phase-0.md) … [docs/plan-phase-4.md](docs/plan-phase-4.md) — detailed steps + tests for the current milestone

## Ground rules

1. **The CLI contract is stable.** Do not change JSON field names, command names, or exit codes without bumping the version in [docs/cli-contract.md](docs/cli-contract.md) and [skill/SKILL.md](skill/SKILL.md).
2. **Default output is JSON.** Never emit human-only text on stdout. Errors go to stderr as JSON `{"error":{...}}`.
3. **Keep the schema single-source-of-truth** in `internal/store/migrations/`. The [docs/data-model.md](docs/data-model.md) doc must be updated to match.
4. **No network calls at runtime** except the one-time model download in `init`. Never add telemetry.
5. **Fast matters.** Benchmark before and after changes that touch search or the embedder. Targets: p95 read < 300 ms @ 100k memories; p95 write overhead < 50 ms.
6. **Tests gate merges.** Every command gets a golden-file JSON test. Core packages target ≥ 70% coverage.

## Repo layout (target)

```
cent-mem/
├── cmd/centmem/          # CLI entrypoint, command handlers
├── internal/
│   ├── config/           # TOML config + env overrides
│   ├── store/            # SQLite layer, migrations, SQL
│   │   └── migrations/   # versioned schema files
│   ├── search/           # hybrid search + RRF fusion
│   ├── embed/            # ONNX embedder + queue drain
│   ├── compact/          # retention + summarizer
│   └── scope/            # scope parsing + inheritance
├── skill/                # SKILL.md + adapters + install.sh
├── docs/                 # this documentation set
├── testdata/             # golden files, fixtures
├── scripts/              # build/release helpers
├── .github/workflows/    # CI
├── go.mod
└── README.md
```

## Development commands

```bash
go build ./...
go vet ./...
go test ./... -race
go test ./... -bench=.   # perf-sensitive changes
```

## Milestone ordering — do not skip ahead

Follow [docs/implementation-plan.md](docs/implementation-plan.md). Current active milestone is tracked by the maintainer; check before starting a new milestone.

## When you are unsure

- Prefer extending the existing contract over inventing new commands.
- Ask before changing: scope grammar, exit codes, JSON field names, or the embedding model default.
- Record significant decisions as a note memory (`centmem put --type note --tags decision`) so future sessions can recall them.

## Bootstrapping (using centmem itself)

Once M1 is complete, this repo should use centmem to store its own development decisions:

```bash
export CENTMEM_PROJ=cent-mem CENTMEM_AGENT=<your-agent>
centmem recall "architecture decisions" --scope project:cent-mem --top 5
```
