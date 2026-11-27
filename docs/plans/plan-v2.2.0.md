# v2.2.0 — Taxonomy & Tag Clustering

**Version:** 2.2.0  
**Owner:** Aradenta Labs  
**Status:** Implemented *(shipped in v2.2.0)*  
**Schema Migration:** None *(reuses `agent_proposals` with `proposal_type = 'update'` and a `tag_merge` metadata action)*  
**Source:** [docs/implementation-plan.md](../implementation-plan.md) · [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Medium Effort #5

---

## 0. Goal & Context

Over time, multiple agents and sessions introduce fragmented tags for the same concept (e.g. `db`, `database`, `sqlite`, `sql-db` or `auth`, `authentication`, `authn`). Because `--tags` filters match exact tag strings, tag drift silently degrades recall precision.

This release adds **deterministic + semantic tag clustering** that detects synonymous or near-duplicate tags within a scope, stages human-reviewable canonicalization proposals through the existing Proposals Review Center, and optionally exposes a `centmem tags` CLI inspection surface.

---

## 1. Phased Implementation Plan

### Phase 1 — Tag Inventory & Clustering Engine (`internal/taxonomy`)

**Scope:**
1. Add `Store.ListTagStats(ctx, scopeIDs)` in `internal/store/` returning each distinct tag, memory count, and last-used timestamp within a scope subtree.
2. Create `internal/taxonomy/cluster.go`:
   - **Pass 1 (Lexical normalization — offline, zero LLM):** Case folding, plural/singular stemming (`decisions` → `decision`), separator normalization (`auth_flow` vs `auth-flow`), and Levenshtein distance ≤ 1 for tags ≥ 5 chars.
   - **Pass 2 (Semantic similarity — local ONNX embedder):** Embed distinct tags using the existing `internal/embed` BGE-small model and group pairs with cosine similarity ≥ configurable threshold (default `0.86`).
   - **Canonical tag selection:** Pick the highest-frequency tag in each cluster (tie-break: shortest lowercase kebab-case form).
3. Add `Store.BulkRenameTag(ctx, scopeIDs, fromTags []string, toTag string)` in `internal/store/` running inside a single transaction, deduplicating resulting tag arrays and preserving `updated_at`.
4. Unit tests in `internal/taxonomy/cluster_test.go` covering lexical clusters, semantic clusters (mock embedder), canonical selection, and idempotency.

**Exit criteria:** `go test -tags fts5 ./internal/taxonomy/... ./internal/store/... -race` passes.

---

### Phase 2 — Curate Integration, Proposals & CLI Surface

**Scope:**
1. Extend `centmem curate --type` to accept `tags` (and include tag clustering in `--type all`).
2. In `internal/agent/engine.go`, stage each detected tag cluster as a proposal in `agent_proposals` (`proposal_type = 'update'`, payload `{"kind": "tag_merge", "from_tags": ["db", "sqlite"], "to_tag": "database", "affected_count": 42}`).
   - Works in **both LLM and offline modes** (LLM optionally refines cluster names/reasoning; offline mode runs lexical + embedding clustering directly).
3. Teach `Store.ApplyProposal()` to execute `tag_merge` payloads atomically via `BulkRenameTag`.
4. Add `centmem tags list --scope <scope>` (JSON list of `{tag, count, last_used}`) and `centmem tags merge --scope <scope> --from a,b --to canonical [--dry-run]` CLI commands in `cmd/centmem/`.
5. Golden-file JSON contract tests in `cmd/centmem/`.

**Exit criteria:** `centmem curate --scope project:X --type tags` generates proposals, `centmem proposals apply <id>` merges tags cleanly, and `centmem recall --tags canonical` finds all previously fragmented memories.

---

### Phase 3 — Web UI Tag Hygiene, Docs & Release (`v2.2.0`)

**Scope:**
1. Render `tag_merge` proposals cleanly in `ProposalsView.tsx` with visual tag pills (`db`, `sqlite` → `database`) and affected memory count.
2. Add `GET /api/tags?scope=<scope>` endpoint and a compact Tag Hygiene / Tag Filter summary in Web UI.
3. Update `docs/cli-contract.md`, `skill/SKILL.md` + all skill mirrors, `docs/implementation-plan.md`, and mark #5 complete in `docs/v2-nice-to-have.md`.
4. Bump version to `2.2.0`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
