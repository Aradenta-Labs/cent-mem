# v2.x — Feature Roadmap & Versioning Plan

**Owner:** Aradenta Labs  
**Status:** Planned — Post-v2.1.3 release sequence  
**Baseline:** `v2.1.3` (Schema v6, CLI contract v2.1.3, Interactive Knowledge Graph + Unified Proposals Review Center)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) · [docs/implementation-plan.md](../implementation-plan.md)

---

## 0. Strategic Overview

After `v2.1.3`, eight features remain across quick UX polish, memory quality intelligence, operational safety, and multi-machine scale. This roadmap sequences them into **8 independently shippable releases** following Semantic Versioning:

- **`v2.1.4` – `v2.1.6` (Polish & Automation):** Zero schema migrations. Low blast radius. Closes UX, streaming, and external notification gaps.
- **`v2.2.0` – `v2.2.1` (Memory Quality & Hygiene):** Tag taxonomy normalization and automated memory health scoring — directly improving hybrid recall precision.
- **`v2.3.0` (Auditability & Safety):** Non-destructive memory versioning and rollback (`m0008`).
- **`v2.4.0` (Continuity):** Cross-session working memory built on `agent_conversations` (`m0009`).
- **`v2.5.0` (Team Scale):** Distributed multi-machine gRPC sync + per-agent RBAC (`m0010`). *(May alternatively be promoted to `v3.0.0` if bundled with any breaking CLI contract cleanup.)*

---

## 1. Versioning & Release Matrix

| Release | Feature | Effort | Schema Migration | Phases | Detailed Plan |
|---|---|---|---|---|---|
| **v2.1.4** | Web UI Keyboard Shortcuts & Drawer Edit *(re-versioned from `plan-v2.1.3.md`)* | S | None | 3 | [plan-v2.1.4.md](plan-v2.1.4.md) |
| **v2.1.5** | Streaming Proposal Application (SSE Batch Progress) | S–M | None | 2 | [plan-v2.1.5.md](plan-v2.1.5.md) |
| **v2.1.6** | Webhook / Notification Hooks | M | None | 3 | [plan-v2.1.6.md](plan-v2.1.6.md) |
| **v2.2.0** | Taxonomy & Tag Clustering | M | None *(reuses `agent_proposals`)* | 3 | [plan-v2.2.0.md](plan-v2.2.0.md) |
| **v2.2.1** | Memory Health & Quality Scoring | M | `m0007_memory_health.sql` (v7) | 3 | [plan-v2.2.1.md](plan-v2.2.1.md) |
| **v2.3.0** | Memory Versioning / Edit History | M–L | `m0008_memory_revisions.sql` (v8) | 3 | [plan-v2.3.0.md](plan-v2.3.0.md) |
| **v2.4.0** | Cross-Session Working Memory | L | `m0009_session_memory.sql` (v9) | 3 | [plan-v2.4.0.md](plan-v2.4.0.md) |
| **v2.5.0** | Distributed Multi-Machine Sync & Per-Agent RBAC | XL | `m0010_sync_rbac.sql` (v10) | 4 | [plan-v2.5.0.md](plan-v2.5.0.md) |

> **Note on `plan-v2.1.3.md`:** `v2.1.3` shipped with the Interactive Knowledge Graph View and Auto-Suggested Links in Proposals Review Center. The original Web UI Keyboard Shortcuts plan has been carried forward and re-versioned as **`v2.1.4`**.

---

## 2. Dependency Graph & Release Sequencing

```
v2.1.3 (current)
  ├──► v2.1.4 (Web UI Keyboard Shortcuts + PATCH /api/memories/{id})
  ├──► v2.1.5 (Streaming Proposal Application — SSE batch progress)
  └──► v2.1.6 (Webhook / Notification Hooks — proposal lifecycle events)
         │
         ▼
       v2.2.0 (Taxonomy & Tag Clustering — feeds curate + proposals)
         │
         ▼
       v2.2.1 (Memory Health & Quality Scoring — m0007, feeds curate + UI)
         │
         ▼
       v2.3.0 (Memory Versioning / Edit History — m0008, protects edits/merges/curate)
         │
         ▼
       v2.4.0 (Cross-Session Working Memory — m0009, extends agent_conversations)
         │
         ▼
       v2.5.0 (Distributed Multi-Machine Sync & RBAC — m0010, extends centmemd SyncService)
```

### Sequencing Rationale

1. **`v2.1.4`–`v2.1.6` first:** No migrations, no breaking changes, immediately polish the Web UI and Proposal Review Center before adding more proposal generators.
2. **`v2.2.0` before `v2.2.1`:** Tag clustering uses existing tables and immediately improves search recall; health scoring (`m0007`) can then incorporate tag hygiene and link density into its quality signals.
3. **`v2.3.0` before heavy autonomous curation:** Versioning (`m0008`) snapshots memory content before `PATCH /api/memories/{id}`, `put` updates, proposal merges, and compaction — giving users a safety net.
4. **`v2.4.0` builds on `agent_conversations`:** Schema v6 already persists threads; v2.4.0 exposes `--session` in CLI, adds rolling context compaction, and unifies CLI + Web UI sessions.
5. **`v2.5.0` ships last:** Multi-machine sync + RBAC touches transport security, `centmemd`, replication cursors, and authorization across every RPC — best tackled after the single-node schema stabilizes at v9.

---

## 3. Release Checklist (Applies to Every Version)

Before tagging `vX.Y.Z`:
1. `go build -tags fts5 ./...` and `go vet -tags fts5 ./...` pass cleanly.
2. `go test -tags fts5 ./... -race` passes across all packages.
3. `bash scripts/check_contract.sh` passes (skill mirrors, CLI JSON contract, exit codes).
4. `node --test npm/test/install.test.js` passes.
5. Version string bumped consistently across `cmd/centmem`, `cmd/centmemd`, `internal/mcp`, `internal/ui`, `internal/capture/mcp_client.go`, `ui/package.json`, `npm/package.json`, `editors/vscode/package.json`, `skill/SKILL.md` + mirrors, and `docs/cli-contract.md`.
6. `CHANGELOG.md` updated and `docs/v2-nice-to-have.md` item marked `✅`.
7. `graphify update .` run so the knowledge graph stays current.
