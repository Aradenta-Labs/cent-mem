# v2.2.1 — Memory Health & Quality Scoring

**Version:** 2.2.1 (target)  
**Owner:** Aradenta Labs  
**Status:** Planned *(official roadmap item)*  
**Schema Migration:** `m0007_memory_health.sql` (Schema v7)  
**Source:** [docs/implementation-plan.md](../implementation-plan.md) · [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Medium Effort #6

---

## 0. Goal & Context

Not all memories age equally. Over months of use, stores accumulate stale decisions never accessed again, ultra-short or vague captures ("fixed it", "todo check later"), orphaned notes with no tags/links, and unverified contradictions.

This release introduces a deterministic **0–100 Memory Health Score** and structured quality flags (`stale`, `vague`, `untagged`, `orphan`, `superseded`, `duplicate_risk`) computed offline with zero LLM cost, exposed via CLI (`centmem health` / `centmem curate --type health`) and Web UI badges.

---

## 1. Schema Migration (`m0007_memory_health.sql`)

```sql
ALTER TABLE memories ADD COLUMN health_score INTEGER;          -- 0..100 (NULL = unscored)
ALTER TABLE memories ADD COLUMN health_flags_json TEXT;        -- JSON array e.g. ["stale","vague"]
ALTER TABLE memories ADD COLUMN health_checked_at INTEGER;     -- unix microseconds

CREATE INDEX IF NOT EXISTS idx_memories_health
    ON memories(scope_id, health_score) WHERE archived_at IS NULL;

INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '7');
```

---

## 2. Phased Implementation Plan

### Phase 1 — Migration & Heuristic Scoring Engine (`internal/health`)

**Scope:**
1. Add `internal/store/migrations/m0007_memory_health.sql` and update `docs/data-model.md`.
2. Create `internal/health/scorer.go` computing a 0–100 score from fast local signals:
   - **Freshness / Staleness (0–30 pts):** Decay curve based on `max(updated_at, last_accessed_at)` and `access_count` (never-accessed memories older than 90d get flagged `stale`).
   - **Information Density (0–30 pts):** Token length, lexical diversity, presence of concrete entities/paths/code vs vague filler (< 25 chars or low-entropy phrases flagged `vague`).
   - **Metadata & Graph Hygiene (0–20 pts):** Has tags, has confirmed links (`memory_links`), not superseded/contradicted (`untagged`, `superseded`).
   - **Uniqueness (0–20 pts):** Optional vector nearest-neighbor check against top-1 sibling in the same scope (cosine ≥ 0.94 flagged `duplicate_risk`).
3. Add `Store.UpdateMemoryHealth(ctx, id, score, flags)` and batch updater `Store.BatchUpdateHealth(ctx, updates)`.
4. Unit tests in `internal/health/scorer_test.go` + migration test in `internal/store/`.

**Exit criteria:** Scoring 10,000 memories completes in < 2 seconds offline; `go test -tags fts5 ./internal/health/... ./internal/store/... -race` passes.

---

### Phase 2 — CLI (`centmem health`), Curate & Compaction Integration

**Scope:**
1. Add `centmem health --scope <scope> [--recompute] [--below 50] [--flag stale] [--limit 50]` outputting JSON summary (`avg_score`, histogram buckets, flag counts, lowest-scoring memories).
2. Extend `centmem curate --type health` (and `--type all`) to score memories and optionally stage `archive` or `update` proposals for chronically low-health items (< threshold).
3. Optional opt-in tie-breaker in `internal/compact/` so low-health memories are prioritized first when compaction runs under retention pressure.
4. Golden-file CLI tests in `cmd/centmem/`.

**Exit criteria:** `centmem health` returns accurate JSON metrics; `curate --type health` stages reviewable proposals without auto-deleting data.

---

### Phase 3 — Web UI Health Badges, Filters & Release (`v2.2.1`)

**Scope:**
1. Expose `health_score` and `health_flags` in `GET /api/memories`, `GET /api/memories/{id}`, and add `GET /api/health?scope=<scope>` + `POST /api/health/recompute`.
2. In `MemoryBrowser.tsx` and `MemoryDetailDrawer.tsx`:
   - Render subtle **"Needs review"** / score badges (green ≥ 75, amber 45–74, red < 45) with flag tooltips.
   - Add a **Health** filter pill (`All`, `Needs Review (<50)`, `Stale`, `Vague`) in the browser filter bar.
3. Update `docs/cli-contract.md`, `docs/ui.md`, `docs/data-model.md`, `skill/SKILL.md` + mirrors, and mark #6 complete in `docs/v2-nice-to-have.md`.
4. Bump version to `2.2.1`, update `CHANGELOG.md`, run full test suite + `scripts/check_contract.sh` + `graphify update .`.
