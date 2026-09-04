# v1.4.3 — Recall Accuracy Enhancements

**Version:** 1.4.3 (target)  
**Owner:** Aradenta Labs  
**Status:** Decisions resolved — implementation ready (Phase A + B in scope; Phase C → v1.5.x)  
**Depends on:** v1.4.2 (if any); no breaking changes to CLI contract

---

## 0. Executive Summary & Objective

`centmem recall` uses a four-ranker hybrid search pipeline (FTS5 keyword, fact
key prefix, timeline, and semantic vector KNN) fused via Reciprocal Rank Fusion
(RRF, k=60). The mechanism is correct but has several precision gaps that cause
less-relevant memories to surface above highly-relevant ones.

**v1.4.3** tightens the recall pipeline with a series of targeted, non-breaking
improvements across three phases. Every change is internal to `internal/search/`
and `internal/embed/`; the CLI contract, JSON shapes, exit codes, and DB schema
are untouched.

**Performance target (existing, must not regress):**
- p95 read < 300 ms @ 100k memories
- p95 write overhead < 50 ms

---

## 1. Root Causes

The following weaknesses were identified via code inspection of
[`internal/search/search.go`](../internal/search/search.go) and
[`internal/search/semantic.go`](../internal/search/semantic.go):

| # | Location | Problem | Severity |
|---|---|---|---|
| 1 | `search.go:190` | `cand = top * 3` — candidate pool is tiny; at the default `top=5`, only 15 candidates flow into RRF, so many relevant memories are never considered. | High |
| 2 | `search.go:209–232` | All 4 rankers carry equal RRF weight. The timeline ranker (pure recency) floods results for text queries that have nothing to do with recency. | High |
| 3 | `semantic.go:99–103` | `score = 1 – distance` with no cutoff — distant vectors (distance > 0.5) still receive a non-zero RRF contribution and can appear in results. | Medium |
| 4 | `search.go:338–350` | `ftsQuery` wraps each token in `"..."` (exact phrase only) — a search for `"arch"` does not match `"architecture"`. | Medium |
| 5 | `search.go:136–159` | `Timeline` ignores query text entirely — it always returns the N most recent memories, regardless of their relevance. | Medium |
| 6 | `Recall()` | No deduplication of semantically near-identical content — the same fact stored twice (from two ingestion passes) consumes two result slots. | Low |
| 7 | `search.go:182` | No query expansion — a single phrasing is the only signal. Synonymous tags that appear in matching memories are never appended to the query. | Low |

---

## 2. Proposed Changes

Changes are organized into three phases by complexity and risk. Phase A should
ship first; Phases B and C are additive and non-breaking.

---

### Phase A — Quick Wins (no new deps, ~1–2 days)

These are pure logic changes inside `internal/search/`. No new config keys,
no schema changes, no CLI contract changes.

#### A1. Larger Candidate Pool

**File:** [`internal/search/search.go`](../../internal/search/search.go) — `Recall()`, line 190

**Before:**
```go
cand := top * 3
```

**After:**
```go
const candidateMultiplier = 5
const candidateFloor     = 50

cand := max(top*candidateMultiplier, candidateFloor)
```

At the default `top=5`, the candidate window grows from 15 → 50, giving
all four rankers a much larger view of the corpus before RRF fusion.

**Risk:** Slightly more SQL row scanning. Acceptable: the queries are indexed
(FTS5, vec0 KNN, scope_id index). Benchmark before/after.

---

#### A2. Semantic Distance Threshold

**File:** [`internal/search/semantic.go`](../../internal/search/semantic.go) — `Semantic()`, after line 67

**New constant:**
```go
// semanticDistThreshold is the maximum cosine distance a memory may have
// from the query vector to be included in the semantic candidate set.
// Distance 0 = identical; 1 = orthogonal; 2 = opposite.
// Tune this value empirically; 0.45 filters out clearly unrelated content.
const semanticDistThreshold = 0.45
```

**Change in the loop over `cands`:**
```go
for _, c := range cands {
    if c.distance > semanticDistThreshold {
        continue // skip: too semantically distant
    }
    ids[i] = c.id
    dist[c.id] = c.distance
}
```

**Rationale:** `bge-small-en-v1.5` produces cosine distances in [0, 2].
Memories with distance > 0.45 are typically unrelated to the query.
Dropping them before RRF prevents irrelevant items from entering the fusion
window just because they were in the KNN result set.

**Risk:** May reduce recall for very short or ambiguous queries. Threshold
is a named constant and trivially tunable.

---

#### A3. FTS5 Prefix Fallback

**File:** [`internal/search/search.go`](../../internal/search/search.go) — `ftsQuery()` and `Keyword()`

Replace the single `ftsQuery()` helper with a two-variant version:

```go
// ftsQueries returns an exact-phrase query and a prefix fallback query.
// The caller tries the exact query first; if it returns 0 rows, retries
// with the prefix query.
func ftsQueries(text string) (exact, prefix string) {
    fields := strings.Fields(text)
    var eq, pf []string
    for _, f := range fields {
        f = strings.Trim(f, `"'()*`)
        if f == "" {
            continue
        }
        eq = append(eq, `"`+f+`"`)
        pf = append(pf, f+`*`)
    }
    return strings.Join(eq, " "), strings.Join(pf, " ")
}
```

In `Keyword()`: try `exact` query first; if `len(rows) == 0`, retry with `prefix`.
Mark results from prefix retry with `matchedBy = "keyword_prefix"` for
observability.

**Risk:** Two SQL round-trips on zero-result exact queries. Negligible —
empty FTS5 MATCH returns instantly.

---

#### A4. Weighted RRF

**File:** [`internal/search/search.go`](../../internal/search/search.go) — `Recall()`, lines 209–232

Replace the uniform ranker loop with a weighted variant. Weights are applied
only when `q.Text != ""` (pure browse/filter calls keep all weights at 1.0):

```go
type rankerDef struct {
    name   string
    items  []Ranked
    weight float64
}

var timelineWeight float64 = 1.0
if q.Text != "" {
    timelineWeight = 0.3  // recency ≠ relevance for text queries
}

lists := []rankerDef{
    {"keyword",  kw,    1.0},
    {"facts",    facts, 1.0},
    {"timeline", tl,    timelineWeight},
    {"semantic", sem,   1.2},  // semantic similarity is highest-confidence signal
}

scores := map[int64]*Ranked{}
for _, l := range lists {
    for rank, item := range l.items {
        contrib := l.weight / (float64(rrfK) + float64(rank+1))
        if existing, ok := scores[item.ID]; ok {
            existing.Score += contrib
            if !contains(existing.MatchedBy, l.name) {
                existing.MatchedBy = append(existing.MatchedBy, l.name)
            }
            continue
        }
        item.Score = contrib
        item.MatchedBy = []string{l.name}
        scores[item.ID] = &item
    }
}
```

**Rationale:** Timeline is a catch-all that always fires, meaning the most
recently written memories have a structural advantage over semantically
superior but older ones. A 0.3× multiplier reduces this bias without removing
timeline from the fusion entirely (it still surfaces recent memories when
other rankers agree).

**Risk:** Changes result ordering for all recall calls. This is intentional
and desirable — covered by the existing benchmark + golden tests.

---

### Phase B — Medium Effort (~3–4 days)

These changes add new functionality but remain contained to `internal/search/`.
Phase B introduces one new config key (`search.decay_half_life_days`).

#### B1. Query Expansion via Tag Enrichment

**File:** new `internal/search/expand.go`

Before hitting the four rankers, expand the query text with tags from the
top keyword matches:

```go
// EnrichQuery returns a copy of q with q.Text augmented by the most frequent
// tags appearing in memories that keyword-match q.Text. At most 3 tags are
// appended. If expansion produces no new tokens, q is returned unchanged.
func (s *Searcher) EnrichQuery(ctx context.Context, q Query) (Query, error)
```

Implementation: lightweight `FTS5 MATCH q.Text LIMIT 20`, collect tags,
count frequency, pick top-3 not already in `q.Text`, join with spaces.

Called at the top of `Recall()` before the four ranker calls.

---

#### B2. Recency Decay

**File:** [`internal/search/search.go`](../../internal/search/search.go) — new `applyDecay()` helper + one config read

Memories older than a half-life get an exponential score penalty, applied
**after** RRF fusion and **before** final sort:

```go
// applyDecay multiplies r.Score by an exponential decay factor based on age.
// halfLife = 0 disables decay.
func applyDecay(r *Ranked, now time.Time, halfLife time.Duration) {
    if halfLife <= 0 {
        return
    }
    age := now.Sub(r.CreatedAt)
    if age <= 0 {
        return
    }
    // score *= 0.5 ^ (age / halfLife)
    r.Score *= math.Pow(0.5, float64(age)/float64(halfLife))
}
```

**New config key:** `search.decay_half_life_days` (int, default `0` = off)  
Added to `internal/config/keys.go` and `docs/data-model.md`.

**Default off** until users can validate the behaviour. Opt-in via:
```toml
[search]
decay_half_life_days = 30
```

---

#### B3. Near-Duplicate Collapse

**File:** [`internal/search/search.go`](../../internal/search/search.go) — new `collapseNearDupes()` helper

After RRF fusion, compare content hashes already available on the `Ranked`
struct (loaded from the `memories.content_hash` column):

```go
// collapseNearDupes removes entries whose content_hash has already been seen,
// keeping only the highest-scored copy. Prevents two ingestion passes of the
// same content from occupying two result slots.
func collapseNearDupes(results []*Ranked) []*Ranked
```

`content_hash` is already stored per-memory (SHA-256 of content). No new
columns needed. This is a Go-layer dedup after fetch, not a SQL DISTINCT.

---

### Phase C — Deferred to v1.5.x

Phase C (two-stage re-rank, session/agent boosting, tag-weighted embedding)
is algorithmic in nature and warrants its own release with a proper offline
evaluation and benchmark baseline. It is **out of scope for v1.4.3**.

For reference, the Phase C designs are preserved in the original proposal at
`docs/plans/plan-v1.4.3.md` version history and can be picked up in the v1.5.x
planning cycle.

---

## 3. Files Affected

### Phase A

| File | Change |
|---|---|
| [`internal/search/search.go`](../../internal/search/search.go) | A1: `cand` floor; A3: `ftsQueries()`; A4: weighted RRF loop |
| [`internal/search/semantic.go`](../../internal/search/semantic.go) | A2: distance threshold constant + filter in candidate loop |

### Phase B

| File | Change |
|---|---|
| [`internal/search/expand.go`](../../internal/search/expand.go) | **[NEW]** `EnrichQuery()` |
| [`internal/search/search.go`](../../internal/search/search.go) | B2: `applyDecay()`; B3: `collapseNearDupes()` |
| [`internal/config/keys.go`](../../internal/config/keys.go) | Add `search.decay_half_life_days` to `KnownConfigKeys` |
| [`docs/data-model.md`](../data-model.md) | Document new config key |

---

## 4. Testing & Verification Plan

### Automated

```bash
# All existing tests must still pass
go test -tags fts5 ./... -race

# Run benchmarks before and after Phase A to guard performance regression
go test -tags fts5 ./internal/search/... -bench=. -benchmem -count=3 \
  | tee bench_before.txt   # before

# (after changes)
go test -tags fts5 ./internal/search/... -bench=. -benchmem -count=3 \
  | tee bench_after.txt

benchstat bench_before.txt bench_after.txt
```

### New tests required

| Test | What it proves |
|---|---|
| `TestRecall_CandidatePool` | At `top=5`, verify `cand ≥ 50` semantically searched |
| `TestSemantic_DistanceThreshold` | Items with distance > 0.45 are excluded |
| `TestKeyword_PrefixFallback` | `"arch"` matches `"architecture"` via prefix retry |
| `TestRecall_WeightedRRF` | Timeline-only result ranks below keyword+semantic result |
| `TestRecall_DecayOrdering` | With decay enabled, recent memory ranks above older equal-score memory |
| `TestRecall_NearDupe` | Two entries with identical `content_hash` produce one result |

### Manual spot-check

```bash
# Baseline before any changes — save this output
centmem recall "architecture decisions" --scope "project:cent-mem" --top 5 --pretty \
  > /tmp/recall_baseline.json

# After Phase A — compare
centmem recall "architecture decisions" --scope "project:cent-mem" --top 5 --pretty \
  > /tmp/recall_after_a.json

diff /tmp/recall_baseline.json /tmp/recall_after_a.json
```

---

## 5. Resolved Decisions

All design questions have been resolved. No approval needed before implementation.

| # | Decision | Resolution | Rationale |
|---|---|---|---|
| 1 | **Phase scope** | Ship Phase A + Phase B in v1.4.3. Phase C deferred to v1.5.x. | A+B together deliver measurable, reviewable improvements. Phase C is algorithmic enough to warrant its own evaluation cycle. Patch releases shouldn't ship speculative algorithms. |
| 2 | **Threshold tuning** | Ship with analytical values (`distThreshold = 0.45`, timeline `weight = 0.3`) as named constants. No offline eval gate. | `bge-small-en-v1.5` is well-characterized — 0.45 cosine distance is a standard cutoff for this model family. Both values are named constants and trivially adjustable in a follow-up patch if empirical data suggests different values. |
| 3 | **Decay default** | **Off by default** (`decay_half_life_days = 0`). Opt-in via config. | Decay changes result ordering for every existing user. Important older decisions (architecture choices, conventions) must still surface reliably. Changing retrieval semantics by default is a breaking behaviour change, even if the schema and contract are untouched. |
| 4 | **Top cap location** | Move the cap from `Recall()` core to the CLI handler. Add a high safety limit (`maxTop = 200`) in `Recall()` only to prevent pathological callers. | The core library has no business imposing a UI-level constraint. The CLI defaults to 20 (unchanged); the Web UI can now request up to 100 results per page without arbitrary truncation. |

---

## 6. Milestone Checklist

**Phase A**
- [ ] Benchmarks baseline captured (`bench_before.txt`)
- [ ] A1: Candidate pool floor raised to `max(top*5, 50)`
- [ ] A2: Semantic distance threshold `0.45` applied in `semantic.go`
- [ ] A3: FTS prefix fallback added to `Keyword()` (`keyword_prefix` matchedBy label)
- [ ] A4: Weighted RRF loop implemented (`timeline 0.3×`, `semantic 1.2×` for text queries)
- [ ] A4: Top cap (`≤ 20`) moved from `Recall()` core to CLI handler; `maxTop = 200` safety limit in core
- [ ] All existing tests pass (`go test -tags fts5 ./... -race`)
- [ ] New tests pass: `TestRecall_CandidatePool`, `TestSemantic_DistanceThreshold`, `TestKeyword_PrefixFallback`, `TestRecall_WeightedRRF`
- [ ] Benchmarks after Phase A: p95 read < 300 ms @ 100k memories confirmed

**Phase B**
- [ ] B1: `internal/search/expand.go` — `EnrichQuery()` implemented and wired into `Recall()`
- [ ] B2: `applyDecay()` implemented; `search.decay_half_life_days = 0` (off by default)
- [ ] B2: Config key added to `internal/config/keys.go` and documented in `docs/data-model.md`
- [ ] B3: `collapseNearDupes()` by `content_hash` implemented
- [ ] New tests pass: `TestRecall_DecayOrdering`, `TestRecall_NearDupe`

**Release**
- [ ] `CHANGELOG.md` updated with `[1.4.3]` section
- [ ] `cmd/centmem/main.go` version bumped to `1.4.3`
- [ ] Tagged `v1.4.3` and pushed; GitHub Actions release runs
- [ ] `graphify update .` run after all code changes
