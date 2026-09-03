# Implementation Plan: cent-mem

**Version:** 1.0
**Target release:** v1.0 (single binary + skill)

---

## Overview

Phased plan from zero to a working v1. Each milestone is independently shippable and leaves the repo in a green state. Work is sized for a solo builder with AI assistance.

Legend: `[S]` small (~few hours), `[M]` medium (~a day), `[L]` large (multi-day).

> **Detailed per-phase plans** (with implementation steps + test plans): [plans/plan-phase-0.md](plans/plan-phase-0.md) · [plans/plan-phase-1.md](plans/plan-phase-1.md) · [plans/plan-phase-2.md](plans/plan-phase-2.md) · [plans/plan-phase-3.md](plans/plan-phase-3.md) · [plans/plan-phase-4.md](plans/plan-phase-4.md) · [release-plan.md](release-plan.md)

---

## Milestone M0 — Foundation & Toolchain

**Goal:** Compilable Go repo, CI green, SQLite + sqlite-vec proven, embedding model strategy validated.

| Task | Size | Deliverable |
|------|------|-------------|
| Init Go module, go.mod, .gitignore | S | repo scaffold |
| Choose SQLite driver (pure-Go modernc vs cgo mattn) | S | decision recorded |
| Vendor/build sqlite-vec for target platforms | M | `vec0` works in a test |
| Embedding strategy spike: ONNX Go binding + BGE-small | M | embed one sentence, assert dims |
| CI (GitHub Actions): lint, vet, test, build matrix | M | green CI |
| `internal/config` TOML loader | S | config reads `~/.centmem/config.toml` |

**Exit criteria:** `go build ./...` succeeds; `TestVec0Roundtrip` passes; embedding spike returns 384-float vector.

**Risks addressed early:** sqlite-vec maturity, ONNX binding viability, pure-Go vs cgo perf.

---

## Milestone M1 — Core Store & CRUD (no embeddings)

**Goal:** All writes/reads work with keyword + facts + timeline only. Semantic comes in M2.

| Task | Size | Deliverable |
|------|------|-------------|
| `internal/store` schema + migrations (m0 initial) | M | tables from data-model.md |
| Scope CRUD + inheritance resolution | M | `ResolveScope(path, inherit)` |
| `put` / `set` / `get` commands | M | facts + notes + logs working |
| FTS5 triggers + `recall` keyword-only mode | M | bm25 ranking works |
| `timeline` command | S | chronological read |
| `list` / `forget` / `stats` commands | M | full CRUD surface |
| Exit-code + JSON contract tests (golden files) | M | contract locked |

**Exit criteria:** All commands except semantic ranking work end-to-end; JSON contract tests pass.

---

## Milestone M2 — Embeddings & Hybrid Search

**Goal:** Semantic search integrated; hybrid `recall` is the flagship command.

| Task | Size | Deliverable |
|------|------|-------------|
| `internal/embed` ONNX embedder | M | `Embed(texts) ([][]float32, error)` |
| `embed_queue` + inline drain worker | M | writes queue; CLI drains ≤K per call |
| `memories_vec` vec0 index | S | vector insert/query works |
| `search.Hybrid` + RRF fusion | M | semantic + keyword fused |
| `--top`, `--type`, `--tags`, `--since/--until`, `--agent` filters | M | full recall surface |
| Benchmarks: 10k/100k memories | M | p95 recall < 300 ms |

**Exit criteria:** `recall` returns semantically relevant results even with no keyword overlap; perf targets met at 10k.

---

## Milestone M3 — Skill Packaging & Harness Adapters

**Goal:** Any supported agent can install and use the skill with zero extra work.

| Task | Size | Deliverable |
|------|------|-------------|
| `skill/SKILL.md` (done — keep in sync with CLI) | S | canonical contract |
| `skill/install.sh` (done — test across paths) | S | installs to all targets |
| Adapter docs (done — add real screenshots/examples) | S | claude-code, codex, cursor |
| End-to-end test: a scripted agent recalls + writes via skill | M | demo works |
| Binary distribution (GitHub Releases, checksums) | M | installable artifact |

**Exit criteria:** Fresh machine: install binary + skill, run two agents, both see shared memory.

---

## Milestone M4 — Retention, Compaction & Polish

**Goal:** Long-term growth is managed; UX is polished; docs complete.

| Task | Size | Deliverable |
|------|------|-------------|
| `internal/compact` + `Summarizer` interface | M | heuristic summarizer works |
| `compact` command + `--dry-run` | S | retention active |
| `doctor` + `backup` commands | S | ops tooling |
| README + install guide | S | onboarding smooth |
| Coverage ≥ 70% on core packages | M | `go test ./...` |
| v1.0 release notes + tag | S | shipped |

**Exit criteria:** Compaction reduces raw rows; docs let a stranger install and use it.

---

## Post-v1.0 Milestones

- [x] **v1.2 (Frictionless UX)**: Standalone `@aradenta.labs/centmem-skills` installer, auto-instruction injection (`AGENTS.md`), and smart-routing `/centmem` slash command.
- [x] **v1.3 (Auto-capture)**: File watchers, exit traps, and 3-tier classification backends extracting durable knowledge from agent transcripts.
- [x] **v1.4 (Web UI Dashboard)**: Browser-based memory browser and management UI (`centmem ui`) served by single binary via `go:embed`.

## Backlog / v2 (server sync)

- `centmemd` daemon: gRPC, API-key auth, owns embedder loop.
- Multi-machine replication via logical replication of `events`.
- Per-agent identities + scoped read/write permissions (RBAC).

---

## Task Sequencing & Dependencies

```
M0 ──► M1 ──► M2 ──► M3 ──► M4 ──► v1.0
 │      │      │      │      │
 ▼      ▼      ▼      ▼      ▼
CI    CRUD   Hybrid  Skill  Retention
setup        search  +dist  +docs
```

Parallelizable:
- M1 schema ↔ M0 embedding spike (independent).
- M3 skill packaging ↔ M2 (independent of embeddings).
- M4 docs polish ↔ M4 code.

## Definition of Done (every milestone)

- [ ] Code compiles; `go vet ./...` clean
- [ ] New/changed behavior covered by tests
- [ ] CLI JSON contract unchanged or versioned
- [ ] Docs updated (PRD/arch/data-model/cli/skill)
- [ ] CI green

## Key Technical Decisions (locked)

| Decision | Choice | Revisit if |
|----------|--------|-----------|
| SQLite driver | modernc.org/sqlite (pure-Go) | perf benchmarks fail |
| Vector ext | sqlite-vec | correctness issues |
| Embedder | ONNX + BGE-small (384d) | quality insufficient |
| Fusion | Reciprocal Rank Fusion | ranking quality poor |
| CLI output | JSON default | — |
| Config format | TOML | — |
