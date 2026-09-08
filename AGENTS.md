# AGENTS.md — Working on cent-mem

This file tells any AI agent working **on this repo** what to do and how.

## What this project is

cent-mem is a fast, local-first shared memory store for AI agents. Agents integrate via a **skill** that wraps the `centmem` CLI (Go). All agents read/write the same memory, scoped by project > agent > session, retrieved via hybrid search (semantic + keyword + facts + timeline).

## ⚡ Start here — graphify knowledge graph (mandatory)

**Before reading any source file or doc, query the knowledge graph first.**

This repo has a pre-built graphify knowledge graph at [`graphify-out/`](graphify-out/). It indexes every Go package, doc, migration, test, and skill file as a navigable graph. Use it to orient yourself in seconds instead of grepping the whole repo.

```bash
# Answer any architecture or codebase question
graphify query "<your question>"

# Find how two components are connected
graphify path "Store" "Hybrid Search Engine"

# Get a plain-language explanation of any concept
graphify explain "embed_queue"

# For a broad overview, read the report
cat graphify-out/GRAPH_REPORT.md
```

**Key facts from the graph (596 nodes, 1333 edges, 49 communities):**
- **God nodes** — the most connected abstractions you must understand first:
  `Store` (42 edges) · `Open()` (34) · `runCLI()` (26) · `newHome()` (25) · `Compact()` (19)
- **Major communities:** CLI Command Dispatch · Scope Resolution · Embedder Cache & Queue · Hybrid Search Engine · Compaction & Summarization · Config & Retention · Model Download & Init
- **Cross-cutting bridge:** `Open()` (store initializer) connects all 8 subsystems — any feature touching persistence goes through it.

**After modifying any code file in this session, run:**
```bash
graphify update .
```
This re-extracts only changed files (AST-only, no API cost, ~seconds) to keep the graph current for the next agent.

---

**Then read these docs, in order:**
1. [docs/PRD.md](docs/PRD.md) — requirements & goals
2. [docs/architecture.md](docs/architecture.md) — components & data flow
3. [docs/data-model.md](docs/data-model.md) — schema & tables
4. [docs/cli-contract.md](docs/cli-contract.md) — CLI surface & JSON shapes
5. [skill/SKILL.md](skill/SKILL.md) — the contract agents use at runtime
6. [docs/implementation-plan.md](docs/implementation-plan.md) — milestones & ordering
7. The **active phase plan** in [docs/plans/plan-phase-0.md](docs/plans/plan-phase-0.md) … [docs/plans/plan-phase-4.md](docs/plans/plan-phase-4.md) — detailed steps + tests for the current milestone
8. **[docs/plans/plan-v1.4.0.md](docs/plans/plan-v1.4.0.md)** — Web UI Memory Browser (design direction + phased UI plan, when working on the UI)

## Ground rules

1. **The CLI contract is stable.** Do not change JSON field names, command names, or exit codes without bumping the version in [docs/cli-contract.md](docs/cli-contract.md) and [skill/SKILL.md](skill/SKILL.md).
2. **Default output is JSON.** Never emit human-only text on stdout. Errors go to stderr as JSON `{"error":{...}}`.
3. **Keep the schema single-source-of-truth** in `internal/store/migrations/`. The [docs/data-model.md](docs/data-model.md) doc must be updated to match.
4. **No network calls at runtime** except the one-time model download in `init`. Never add telemetry.
5. **Fast matters.** Benchmark before and after changes that touch search or the embedder. Targets: p95 read < 300 ms @ 100k memories; p95 write overhead < 50 ms.
6. **Tests gate merges.** Every command gets a golden-file JSON test. Core packages target ≥ 70% coverage.
7. **Keep the graph current.** Run `graphify update .` after any code change so the next agent inherits an accurate graph.

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
├── graphify-out/         # knowledge graph (query before reading raw files)
├── testdata/             # golden files, fixtures
├── scripts/              # build/release helpers
├── .github/workflows/    # CI
├── go.mod
└── README.md
```

## Development commands

```bash
go build -tags fts5 ./...
go vet -tags fts5 ./...
go test -tags fts5 ./... -race
go test -tags fts5 ./... -bench=.   # perf-sensitive changes

# Knowledge graph
graphify query "<question>"   # ask the graph
graphify update .              # update after code changes
```

## Milestone ordering — do not skip ahead

Follow [docs/implementation-plan.md](docs/implementation-plan.md). Current active milestone is tracked by the maintainer; check before starting a new milestone.

## When you are unsure

- **Query the graph first:** `graphify query "<your question>"` — it usually answers faster than reading files.
- Prefer extending the existing contract over inventing new commands.
- Ask before changing: scope grammar, exit codes, JSON field names, or the embedding model default.
- Record significant decisions as a note memory (`centmem put --type note --tags decision`) so future sessions can recall them.

## Bootstrapping (using centmem itself)

Once M1 is complete, this repo should use centmem to store its own development decisions:

```bash
export CENTMEM_PROJ=cent-mem CENTMEM_AGENT=<your-agent>
centmem recall "architecture decisions" --scope project:cent-mem --top 5
```

<!-- centmem:start -->
## cent-mem Workflow
All agents working on this project share persistent memory via `centmem`. Detailed skill artifacts and reference guides are located in `.agents/skills/centmem/`.

Whenever you start a task, follow this 3-step loop:
1. **READ**: Run `centmem recall "<task context>"` to load prior decisions, conventions, and learnings.
2. **DO**: Execute the user's prompt.
3. **UPDATE**: Run `centmem put` or `centmem set` to store new learnings, decisions, conventions, or checkpoints before finishing.

### Essential Commands
- **Recall Context**: `centmem recall "<query>" --scope "project:$CENTMEM_PROJ" --top 5`
- **Save Decision / Note**: `centmem put --scope "project:$CENTMEM_PROJ" --type note --content "<decision>" --tags decision`
- **Save Key/Value Fact**: `centmem set --scope "project:$CENTMEM_PROJ" --key "<key>" --value '<json>'`
- **Recent Activity Timeline**: `centmem timeline --scope "project:$CENTMEM_PROJ" --since 24h`
- **Check Health**: `centmem doctor`
<!-- centmem:end -->

<!-- centmem-command:start -->
## /centmem Command
When the user types `/centmem <input>`, act as the Memory Manager. Analyze the intent:
- **Recall / Context**: If asking a question or looking for context, run `centmem recall "<input>" --scope "project:$CENTMEM_PROJ" --top 5` or `centmem timeline`.
- **Save Decisions / Facts**: If stating a decision, convention, preference, or learning to save, run `centmem put --scope "project:$CENTMEM_PROJ" --type note --content "<input>"` or `centmem set --scope "project:$CENTMEM_PROJ" --key "<key>" --value '<json>'`.
- **Ingest / Capture**: If asking to ingest repository knowledge, commits, docs, shell history, or code annotations, run `centmem capture git`, `centmem capture docs`, `centmem capture shell`, or `centmem capture comments`.
- **Maintenance / Health**: If requesting maintenance, health checks, or statistics, run `centmem doctor`, `centmem stats`, or `centmem compact`.
- **Visual Dashboard / UI**: If asking to view, browse, inspect, or manage memories in a browser interface, run `centmem ui`.

Always verify execution results from stdout JSON and report them clearly to the user.
<!-- centmem-command:end -->
