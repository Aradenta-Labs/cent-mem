# PRD: cent-mem — Shared Memory for AI Agents

**Version:** 1.0 (v1 scope)
**Owner:** Aradenta Labs
**Date:** 2026-08-31
**Status:** Approved — ready for implementation

---

## 1. Problem Statement

Today each AI agent, harness, and coding tool (Claude Code, Cursor, Codex, custom harnesses, etc.) maintains its own isolated context/memory. This causes:

1. **Context fragmentation** — decisions, preferences, and facts learned by one agent are invisible to every other agent.
2. **Redundant onboarding** — every new session/agent re-discovers the same project conventions, user preferences, and past decisions.
3. **Inconsistent behavior** — agents contradict each other because they don't share a single source of truth.
4. **Token waste** — agents re-read files and re-derive context they could instead retrieve cheaply from a shared store.

## 2. Solution

**cent-mem** is a fast, local-first, hierarchical shared memory store for AI agents. Every agent integrates via a **skill** that wraps a single **Go CLI** (`centmem`). All agents read and write the same memory, scoped by project > agent > session, and retrieve "just the memory that's required" via hybrid search (semantic + keyword + facts + timeline).

### One-sentence vision

> One shared brain for all your AI agents, one `centmem` command away.

## 3. Goals (v1)

| # | Goal | Success metric |
|---|------|----------------|
| G1 | Any agent can write a memory in < 50 ms CLI overhead | p95 write latency < 50 ms |
| G2 | Any agent can retrieve relevant memory in < 300 ms | p95 hybrid query < 300 ms on 100k memories |
| G3 | One skill install works across Claude Code, Cursor, Codex, custom harnesses | skill installs + works in ≥ 3 harnesses |
| G4 | Memory is scoped: global → project → agent → session | scoping tests pass |
| G5 | Old memories auto-summarize to control growth | compaction job reduces raw rows ≥ 60% after 30 days |
| G6 | Fully offline; no API keys required for core operation | works with network disabled |

## 4. Non-Goals (v1)

- Real-time multi-machine sync (v2 — server sync is planned, schema is designed sync-ready).
- Multi-user / team RBAC (v2).
- GUI / dashboard (CLI + TUI maybe later).
- Automatic capture from transcripts (v1 is explicit write; auto-capture is v1.1).
- Memory quality scoring / reinforcement (v2).

## 5. Users & Personas

| Persona | Description | Primary need |
|---------|-------------|--------------|
| **Solo power user (you)** | Runs multiple AI agents/harnesses daily on one machine | Shared context across all agents, zero friction |
| **AI Agent** | Claude Code, Codex, Cursor, custom harness | Cheap, deterministic retrieve + store via skill |
| **Custom harness developer** | Builds their own agent loops | Embed/link a skill or call CLI programmatically |

## 6. Functional Requirements

### 6.1 Memory model (hierarchical scoping)

- **FR-1** Scopes are hierarchical: `global` → `project` → `agent` → `session`.
- **FR-2** Every write lands in a scope; every read can target a scope **with inheritance** (a project-scope read also sees matching global memories).
- **FR-3** Memory types:
  - `fact` — structured key/value (JSON value), e.g. `user.timezone = Asia/Jakarta`
  - `note` — free-text memory (decisions, preferences, insights)
  - `log` — append-only chronological event/context entry
- **FR-4** Every memory carries: `id`, `scope`, `type`, `content/value`, `tags[]`, `source_agent`, `source_session`, `created_at`, `updated_at`, `ttl/summarize_at`, `embedding`.

### 6.2 Write path

- **FR-5** `put` writes a memory to a scope with optional tags.
- **FR-6** `set` upserts a key-value fact.
- **FR-7** `log` appends a chronological entry.
- **FR-8** Writes are deduplicated/merged on identical content within a short window (same scope+tags+content hash → update not insert).
- **FR-9** Writes are batched and embedded asynchronously (embed queue) so CLI returns immediately.

### 6.3 Read path

- **FR-10** `recall` performs **hybrid search**: semantic (vector) + keyword (FTS5) + fact lookup + timeline filter.
- **FR-11** `recall` supports filters: scope, tags, type, time range, source_agent.
- **FR-12** `recall` returns a compact, token-efficient result set (ranked, deduped, truncated to N) suitable for direct injection into an agent's context.
- **FR-13** `get` fetches a fact by key.
- **FR-14** `timeline` returns chronological logs for a scope + time range.

### 6.4 Retention & compaction

- **FR-15** Each scope/type has a default retention policy: facts = forever, notes = summarize after N days, logs = summarize after M days then drop raw.
- **FR-16** A `compact` job summarizes old memories (local LLM or heuristic extract) into consolidated memories and archives the originals.
- **FR-17** `stats` reports store size, counts by scope/type, and last compaction.

### 6.5 Skill & integration

- **FR-18** A single installable **skill** (`centmem`) documents the CLI contract for agents.
- **FR-19** All CLI output is machine-readable JSON (`--json` default, human `--pretty` optional).
- **FR-20** CLI exit codes are deterministic: 0 success, 1 error, 2 not-found, 3 conflict.

### 6.6 Security (v1)

- **FR-21** API key auth for any server/daemon mode (v2-ready); v1 local CLI requires no auth but respects file permissions (0600 DB).
- **FR-22** No data leaves the machine; embeddings run locally.

## 7. Non-Functional Requirements

| Area | Requirement |
|------|-------------|
| **Performance** | p95 read < 300 ms @ 100k memories; p95 write overhead < 50 ms; cold start < 100 ms |
| **Language** | Go ≥ 1.22; single static binary; no runtime deps beyond the binary |
| **Storage** | SQLite (WAL) + sqlite-vec; single file, portable, sync-ready |
| **Embeddings** | Local ONNX model (e.g. BGE-small / nomic-embed), 384–768 dims, cached in-process |
| **Offline** | 100% functional with network disabled |
| **Portability** | darwin/arm64 primary; linux/amd64+arm64, darwin/amd64 secondary |
| **Reliability** | WAL + checkpoints; crash-safe; DB integrity self-check command |
| **Observability** | Structured JSON logs to stderr; `--verbose` for tracing |
| **Testing** | ≥ 70% line coverage on core; golden-file tests for CLI output |

## 8. Constraints & Assumptions

- Single-user, single-machine in v1.
- User controls all agents via CLI/skill; no untrusted third-party writes.
- Embedding model files (~100 MB) are downloaded once on first `init` (network needed once), then cached locally.
- Auto-summarization in v1 uses a pluggable "summarizer" (heuristic first; optional local/remote LLM hook).

## 9. Release Plan (summary)

See [implementation-plan.md](./implementation-plan.md) for full detail.

| Milestone | Scope |
|-----------|-------|
| **M0** | Repo scaffold, Go toolchain, CI, SQLite+vec proven |
| **M1** | Core store: put/set/log/get/recall/timeline + scoping + FTS5 |
| **M2** | Local embeddings + semantic search + hybrid ranking |
| **M3** | Skill packaging + harness adapters (Claude Code, Codex, Cursor) |
| **M4** | Retention/compaction + stats + polish |
| **v1.0** | Release: single binary + skill |
| **v1.1+** | Auto-capture, TUI, sync server design, multi-machine |

## 10. Open Questions

| # | Question | Default |
|---|----------|---------|
| Q1 | Which embedding model default — BGE-small (fast, 384d) or nomic-embed (better, 768d)? | BGE-small; make model swappable via config |
| Q2 | Summarizer for compaction: heuristic extract vs local LLM vs hook to existing agent? | Heuristic first, pluggable |
| Q3 | Where to store DB by default? | `~/.centmem/centmem.db` (overridable via env `CENTMEM_HOME`) |
