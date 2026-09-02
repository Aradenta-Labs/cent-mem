<div align="center">

# cent-mem

**Shared memory for AI agents — one `centmem` command, one shared brain.**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](#)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](#)
[![Version](https://img.shields.io/badge/version-v1.3.0-success.svg)](CHANGELOG.md)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux-lightgrey)](#installation)
[![Offline](https://img.shields.io/badge/offline-100%25-brightgreen)](#privacy)
[![No API key](https://img.shields.io/badge/api%20key-not%20required-brightgreen)](#privacy)

</div>

---

> *Today every AI agent lives on its own island. **cent-mem** puts them all on the same continent.*

cent-mem is a fast, local-first shared memory store. Every agent on your machine integrates via a tiny **skill** that wraps a single **Go CLI**. They all read and write the same memory — scoped by project → agent → session — and retrieve "just the memory that's required" via hybrid search (semantic + keyword + facts + timeline).

Works with **Claude Code**, **Codex**, **Cursor**, **Continue**, **Amazon Q Dev**, and any custom harness that can exec a binary.

---

## Why cent-mem?

- **Stop re-deriving.** Decisions, conventions, and preferences learned by one agent are instantly visible to every other.
- **Stop contradicting.** All agents share one source of truth.
- **Save tokens.** A 50 ms `recall` replaces re-reading files and re-summarizing the project.
- **Offline, by default.** Local embeddings (BGE-small, ONNX). No API keys, no telemetry, no cloud.
- **One binary.** Single static Go executable. No runtime deps, no daemon in v1.

## Features

| | |
|---|---|
| 🚀 | **Fast** — p95 read < 300 ms on 100k memories; p95 write overhead < 50 ms |
| 🧠 | **Hybrid search** — semantic (vec) + keyword (FTS5 bm25) + facts + timeline, fused via RRF |
| 🗂️ | **Hierarchical scoping** — `global → project → agent → session`, with inheritance |
| 🎣 | **Auto-capture** — extracts durable decisions, facts, and code from agent transcripts automatically |
| 🧹 | **Auto-summarize** — old memories consolidate; the store stays lean forever |
| 🩺 | **Self-checks** — `doctor`, `backup`, `restore` for ops |
| 🔌 | **Pluggable** — swap embedders or summarizers via interfaces |
| 🔒 | **Privacy** — data never leaves your machine |

---

## Quickstart

```bash
# 1. Install the binary
go install -tags fts5 github.com/aradenta-labs/cent-mem/cmd/centmem@latest
# or download a release binary: https://github.com/aradenta-labs/cent-mem/releases

# 2. Initialize: creates ~/.centmem and downloads the default model (one-time, ~100 MB)
centmem init

# 3. Store a fact and a note from any agent
centmem set --scope project:myapp --key user.timezone --value '"Asia/Jakarta"'
centmem put --scope project:myapp --type note --content "we deploy via github actions" --tags decision

# 4. Recall — even with no keyword overlap
centmem recall "how do we ship?" --scope project:myapp --top 5
```

Output is JSON on stdout (errors are JSON on stderr). See [docs/guides/getting-started.md](docs/guides/getting-started.md) for the full walkthrough.

### Use from an AI agent

```bash
# Install the skill & slash command across all agent harnesses in one step
npx @aradenta.labs/centmem-skills
```

Or via bash:
```bash
bash <(curl -fsSL https://raw.githubusercontent.com/aradenta-labs/cent-mem/main/skill/install.sh)
```

Once installed, the installer automatically:
1. Installs `SKILL.md` to your agents' skill directories (Antigravity, Claude Code, Cursor, Codex, Trae, Hermes, DeepSeek, Continue, etc.).
2. Injects the **Read → Do → Update** memory loop into your project's `AGENTS.md` (or `.cursorrules` / `CLAUDE.md`).
3. Sets up the smart **`/centmem`** slash command so you can recall, save, and maintain memories directly from your chat prompt.


---

## Installation

### From source

```bash
go install -tags fts5 github.com/aradenta-labs/cent-mem/cmd/centmem@latest
```

Requires Go ≥ 1.22.

> **Note:** If your terminal says `command not found: centmem` after running `go install`, your Go `bin` directory is not in your system's `PATH`. To fix this (on macOS/Linux), run:
> ```bash
> echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
> source ~/.zshrc
> ```

### Pre-built binaries

Download the binary for your platform from [Releases](https://github.com/aradenta-labs/cent-mem/releases/latest):

| OS | Arch |
|----|------|
| macOS | arm64, amd64 |
| Linux | amd64, arm64 |

Verify the SHA256 checksum from the `SHA256SUMS` file attached to each release.

### From a package manager

_(Homebrew, apt, etc. — coming soon — see [docs/guides/troubleshooting.md](docs/guides/troubleshooting.md).)_

---

## Usage

```bash
centmem init                           # one-time setup + model download
centmem put   --scope <s> --type note  # write a note/log memory
centmem set   --scope <s> --key <k> --value '<json>'   # upsert a fact
centmem get   --scope <s> --key <k>    # fetch a fact (with inheritance)
centmem recall <q> --scope <s>         # hybrid search
centmem timeline --scope <s>           # chronological logs
centmem list   --scope <s>             # browse memories
centmem forget --id N                  # delete a memory
centmem compact [--dry-run]            # summarize + archive old memories
centmem doctor                         # health checks
centmem backup   --to <file>           # snapshot the DB
centmem restore  --from <file>         # restore from snapshot
centmem capture run [--watch]          # auto-capture from transcript
centmem capture summary                # view latest capture report
centmem capture categories --list      # list/manage capture categories
centmem capture convert --harness <h>  # normalize transcript to JSONL
centmem config set <k> <v>             # configure settings in config.toml
centmem stats                          # store summary
```

### Scope grammar

```
global
project:<name>
project:<name>/agent:<agent>
project:<name>/agent:<agent>/session:<id>
```

Reads default to inheriting ancestor scopes. Pass `--children` to include descendants. Pass `--no-inherit` to read strictly the given scope.

---

## How it works

```
   AI Agent  ──►  skill  ──►  centmem CLI  ──►  SQLite + sqlite-vec + FTS5
   (any)         (markdown)   (Go binary)        (one file in ~/.centmem)
```

- **Write path:** `put`/`set` persist to `memories`, queue an embedding. An inline worker drains the queue with stale-claim recovery so embeddings never block the CLI.
- **Read path:** `recall` runs four rankers in parallel (semantic, keyword, facts, timeline), fuses via **Reciprocal Rank Fusion (k=60)**, then applies filters + dedup + slice-to-top.
- **Auto-capture path:** Hooks intercept or watch transcripts, classify via local/cloud LLMs or heuristics, deduplicate via recall-before-write, and persist novel memories.
- **Compaction:** notes/logs past `summarize_at` are summarized into a consolidated note; originals are archived and dropped from the vector index.

See [docs/architecture.md](docs/architecture.md) for the full design.

---

## Documentation

Full documentation map: [docs/README.md](docs/README.md).

### For users

| Doc | Purpose |
|-----|---------|
| [docs/guides/getting-started.md](docs/guides/getting-started.md) | Step-by-step install for first-time users |
| [docs/guides/capture-hooks.md](docs/guides/capture-hooks.md) | Comprehensive guide for auto-capture & transcript hooks |
| [docs/guides/troubleshooting.md](docs/guides/troubleshooting.md) | Common issues & fixes |
| [skill/SKILL.md](skill/SKILL.md) | The contract your agent reads at runtime |
| [skill/adapters/](skill/adapters/) | Per-harness setup (Claude Code, Codex, Cursor, Continue, Amazon Q, custom) |
| [skill/adapters/hooks/](skill/adapters/hooks/) | Per-harness hook scripts & watcher setup |

### For contributors / agents working on this repo

| Doc | Purpose |
|-----|---------|
| [docs/README.md](docs/README.md) | Documentation index & reading order |
| [docs/PRD.md](docs/PRD.md) | Requirements, goals, non-goals |
| [docs/architecture.md](docs/architecture.md) | Components & data flow |
| [docs/data-model.md](docs/data-model.md) | Schema & tables |
| [docs/cli-contract.md](docs/cli-contract.md) | CLI surface & JSON contract (stable within v1.x) |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Milestones & ordering |
| [docs/plans/plan-phase-0.md](docs/plans/plan-phase-0.md) … [plan-phase-4.md](docs/plans/plan-phase-4.md) | Detailed per-phase plans + tests |
| [docs/plans/plan-v1.4.0.md](docs/plans/plan-v1.4.0.md) | v1.4.0 Web UI Memory Browser (design + plan) |
| [docs/release-plan.md](docs/release-plan.md) | Release & QA process |
| [AGENTS.md](AGENTS.md) | Rules for any AI agent editing this repo |
| [CHANGELOG.md](CHANGELOG.md) | Version history |


---

## Performance

Measured on darwin/arm64, default config (BGE-small, 384-dim):

| Operation | Target | Notes |
|-----------|-------:|-------|
| `put` / `set` (write overhead) | < 50 ms p95 | Embedding is async/queued |
| `recall` (hybrid, 10k memories) | < 300 ms p95 | Vec index + FTS5 + RRF |
| `recall` (hybrid, 100k memories) | < 800 ms p95 | Same path; mostly vec search |
| Cold start | < 100 ms | Go binary + SQLite open |

Benchmarks live in `internal/search/search_bench_test.go` and are recorded in [`testdata/bench-results.txt`](testdata/bench-results.txt).

---

## Privacy

- No telemetry, no network calls at runtime (except the one-time model download in `init`, with sha256 verification).
- All data lives in `~/.centmem/centmem.db` with `0600` permissions; the directory is `0700`.
- Embeddings run **locally** via ONNX — your text never leaves the machine.
- The model is downloaded once from a pinned URL with a pinned checksum (see `internal/embed/models.go`).

---

## Roadmap

- [x] **v1.0** — Core store, hybrid search, skill, compaction, polish
- [x] **v1.2** — Frictionless UX (npx installer, workflow loop injection, /centmem slash command)
- [x] **v1.3** — Auto-capture from agent transcripts (hooks & multi-backend classification)
- [ ] **v1.4** — Web UI Memory Browser dashboard (`centmem ui`, embedded server)
- [ ] **v2.0** — Sync server (`centmemd`), multi-machine replication via `events` log, per-agent RBAC, remote embedding providers

See [docs/implementation-plan.md](docs/implementation-plan.md) and [docs/PRD.md § Open Questions](docs/PRD.md#10-open-questions).

---

## Contributing

PRs welcome. Two things to know:

1. **The CLI contract is stable.** Do not change command names, JSON field names, or exit codes without bumping the major version.
2. **Use the active phase plan.** See [AGENTS.md](AGENTS.md) for the rules and [docs/plans/plan-phase-*.md](docs/plans/) for the detailed plan + test list for the current milestone.

```bash
go build -tags fts5 ./...
go vet -tags fts5 ./...
go test -tags fts5 ./... -race
go test -tags fts5 ./... -bench=.    # before/after perf-sensitive changes
```

---

## License

[MIT](LICENSE)

---

## Acknowledgments

Built with [SQLite](https://sqlite.org/), [sqlite-vec](https://github.com/asg017/sqlite-vec), [BGE-small](https://huggingface.co/BAAI/bge-small-en-v1.5), and the excellent [ONNX Runtime](https://onnxruntime.ai/) — all running locally on your machine.

---

<div align="center">

**If cent-mem saves your agents from re-deriving the same context for the hundredth time, a star helps others find it.** ⭐

</div>