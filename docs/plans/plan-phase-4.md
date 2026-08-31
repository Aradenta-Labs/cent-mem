# Phase 4 (M4) — Retention, Compaction & Polish

**Goal:** Long-term growth is managed (auto-summarize old memories), ops tooling exists (`doctor`, `backup`), docs are complete, and the project hits v1.0 quality bars.

**Exit criteria:**
- `compact` summarizes eligible memories (past `summarize_at`) into a consolidated `note` and archives originals.
- A 30-day simulation shows the active-store row count drops ≥ 60% after compaction of logs/notes.
- `doctor` passes a healthy store and reports actionable failures on a broken one.
- `backup` produces a restorable snapshot; `restore` verified.
- README + install guide let a stranger go from zero → working memory in < 10 min.
- Coverage ≥ 70% on `internal/{store,scope,search,embed,compact}`.

---

## 4.1 Prerequisites

- Phase 3 complete (released binary + working skill).

## 4.2 Compaction — `internal/compact/compact.go`

```go
type Summarizer interface {
    Summarize(ctx context.Context, memories []store.Memory) (summary string, err error)
}

// HeuristicSummarizer is the default: deterministic, no network.
type HeuristicSummarizer struct{ MaxSentences int }

// LLMSummarizer is a hook (optional): POST to a local/remote endpoint.
type LLMSummarizer struct{ Endpoint string; APIKey string }
```

**Heuristic algorithm (default):**
1. Sort by `created_at`.
2. Extract key sentences (frequency-ranked unique tokens, top-N).
3. Emit: `Summary of N memories (scope=X, YYYY-MM-DD..YYYY-MM-DD): <top sentences>`.

**Compaction job** (`compact.Compact`):
1. Select `active` memories where `summarize_at <= now`.
2. Group by `scope_id` + `type`.
3. For each group: `Summarize` → insert a new `note` with `status='active'`, tags = union of originals, `summarize_at=NULL`.
4. Mark originals `status='archived'` (keep them for audit; `events` already recorded `insert`).
5. Append `summarize` events for each.
6. Drop their `embeddings`/`memories_vec` rows (archived rows aren't searchable).
7. Return counts: `summarized`, `archived`, `new_memory_ids`.

**CLI `compact` command:**
- `centmem compact [--scope X] [--dry-run]`.
- `--dry-run` prints what would be summarized without writing.

**CLI `restore` subcommand** (paired with backup): `centmem restore --from file.db`.

## 4.3 Retention defaults (config)

`~/.centmem/config.toml`:

```toml
[retention]
fact_keep_days = 0        # 0 = forever
note_summarize_after_days = 30
log_summarize_after_days  = 14
log_drop_after_days       = 30
archive_keep_days         = 365
```

Loaded by `internal/config`. `compact` respects these per scope/type.

## 4.4 `doctor` — `cmd/centmem/commands/doctor.go`

Checks:
1. DB opens; `PRAGMA integrity_check` → "ok".
2. `meta.schema_version` matches the binary's expectation.
3. `sqlite-vec` and `fts5` extensions present.
4. Model file exists + sha256 matches catalog.
5. Embed queue backlog (`pending_embeddings`) reported; warn if > 1000.
6. Permissions: `~/.centmem` is `0700`, DB is `0600`.

Output JSON:
```json
{"ok": true, "checks": [{"name":"integrity","status":"ok"}, ...], "warnings": []}
```
Exit 0 if all pass; exit 1 with failing checks listed.

## 4.5 `backup` / `restore`

- `backup --to path.db`: `VACUUM INTO` a timestamped file; print `{"ok":true,"backup":"...","size_mb":X}`.
- `restore --from path.db`: verify sha + integrity, then replace current DB (with a `.pre-restore.bak` safety copy).

## 4.6 Documentation polish

- **README.md**: real quickstart (install binary → `init` → first recall), GIF/asciinema demo optional.
- **docs/guides/getting-started.md**: step-by-step install for a first-time user.
- **docs/guides/troubleshooting.md**: common issues (model download blocked, vec0 missing, DB locked) + fixes.
- Keep `docs/cli-contract.md` and `skill/SKILL.md` in sync (contract test from M3 enforces).

## 4.7 Polish & robustness

- Consistent error hints in stderr JSON (`hint` field populated).
- `--quiet` honored across commands.
- Graceful signal handling (SIGINT mid-compact doesn't corrupt; tx rollback).
- Config validation on load (`invalid retention days` → clear error).
- Log all errors with `log/slog` at `--verbose`.

## 4.8 Implementation order

1. `Summarizer` interface + `HeuristicSummarizer`.
2. `Compact` job + grouping + archive + event appends.
3. `compact` command + `--dry-run`.
4. `doctor` command + checks.
5. `backup`/`restore`.
6. Config retention fields + validation.
7. Docs polish.
8. Final coverage + lint + release prep.

## 4.9 Checklist

- [ ] `HeuristicSummarizer` deterministic + tested
- [ ] `Compact` archives originals + writes consolidated note
- [ ] `compact --dry-run` safe (no writes)
- [ ] `events` records `summarize` ops
- [ ] `doctor` checks integrity/extensions/model/perms/queue
- [ ] `backup` produces restorable file; `restore` verified round-trip
- [ ] Config retention fields honored
- [ ] README + getting-started + troubleshooting written
- [ ] Signal-safe compaction (interrupt → clean state)
- [ ] Coverage ≥ 70% on core packages
- [ ] Tag `v1.0.0` release

## 4.10 Phase 4 test plan (validation)

### Compaction

| Test | Package | Validates |
|------|---------|-----------|
| `TestHeuristicSummarizer_Deterministic` | compact | same input → same summary |
| `TestHeuristicSummarizer_Shorter` | compact | summary shorter than concatenated originals |
| `TestCompact_SelectsEligible` | compact | only rows past `summarize_at` are touched |
| `TestCompact_GroupedByScopeAndType` | compact | separate groups → separate summaries |
| `TestCompact_ArchivesOriginals` | compact | originals `status='archived'`, excluded from recall |
| `TestCompact_WritesConsolidatedNote` | compact | new note has union tags, `status='active'` |
| `TestCompact_AppendsEvents` | compact | `summarize` events recorded |
| `TestCompact_DropsVectors` | compact | archived rows removed from `memories_vec` |
| `TestCompact_DryRun` | compact | no DB writes in dry-run |
| `TestCompact_RespectsRetentionConfig` | compact | facts never compacted (keep forever) |
| `TestCompact_ReductionTarget` | compact (integration) | 30-day sim: ≥60% active-row reduction |
| `TestCompact_SignalSafe` | compact | SIGINT mid-run → no partial archive |

### doctor

| Test | Validates |
|------|-----------|
| `TestDoctor_Healthy` | all checks pass; exit 0 |
| `TestDoctor_CorruptDB` | integrity_check fails → exit 1 with actionable message |
| `TestDoctor_ModelMissing` | reports missing model + how to `init` |
| `TestDoctor_BadPerms` | reports permissions issue |
| `TestDoctor_QueueBacklog` | warns when pending > 1000 |

### backup / restore

| Test | Validates |
|------|-----------|
| `TestBackup_RoundTrip` | backup → modify → restore → original data present |
| `TestRestore_SafetyCopy` | `.pre-restore.bak` created |
| `TestRestore_RejectsCorrupt` | corrupt file rejected with exit 1 |

### CLI golden

| Test | Validates |
|------|-----------|
| `TestCLI_Compact_DryRun` | JSON shape `{"ok":true,"summarized":N,...}` |
| `TestCLI_Doctor` | JSON checks array |
| `TestCLI_Backup` | JSON `backup` path + size |

### Docs

| Test | Validates |
|------|-----------|
| `TestDocs_ContractConsistency` (from M3) | still green |
| `TestGettingStarted_SmokeTest` (manual script) | a fresh-user runbook executes cleanly |

**Definition of done:** all tests green; `v1.0.0` tagged; release artifacts published; README quickstart verified on a clean machine.
