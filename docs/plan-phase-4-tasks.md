# Phase 4 (M4) — Task List

> Generated from `docs/plan-phase-4.md` (source of truth for implementation order and tests).
> This file is a working checklist artifact, not a substitute for the plan.

**Goal:** Long-term growth is managed (auto-summarize old memories), ops tooling exists (`doctor`, `backup`), docs are complete, and the project hits v1.0 quality bars.

**Exit criteria:**
- `compact` summarizes eligible memories (past `summarize_at`) into a consolidated `note` and archives originals.
- A 30-day simulation shows the active-store row count drops ≥ 60% after compaction of logs/notes.
- `doctor` passes a healthy store and reports actionable failures on a broken one.
- `backup` produces a restorable snapshot; `restore` verified.
- README + install guide let a stranger go from zero → working memory in < 10 min.
- Coverage ≥ 70% on `internal/{store,scope,search,embed,compact}`.

---

## Task list (implementation order from §4.8)

### 1. Summarizer interface + HeuristicSummarizer
- [ ] Define `Summarizer` interface (`Summarize(ctx, []store.Memory) (string, error)`) in `internal/compact/compact.go`
- [ ] Implement `HeuristicSummarizer` (deterministic, no network; `MaxSentences int`)
- [ ] Heuristic algorithm: sort by `created_at` → frequency-ranked unique tokens (top-N) → emit `Summary of N memories (scope=X, YYYY-MM-DD..YYYY-MM-DD): <top sentences>`
- [ ] Add `LLMSummarizer` hook (`Endpoint string; APIKey string`) as optional

### 2. Compact job + grouping + archive + event appends
- [ ] Implement `compact.Compact`: select `active` memories where `summarize_at <= now`
- [ ] Group by `scope_id` + `type`
- [ ] Per group: `Summarize` → insert new `note` (`status='active'`, tags = union of originals, `summarize_at=NULL`)
- [ ] Mark originals `status='archived'`
- [ ] Append `summarize` events for each original
- [ ] Drop `embeddings`/`memories_vec` rows for archived rows
- [ ] Return counts: `summarized`, `archived`, `new_memory_ids`

### 3. `compact` command + `--dry-run`
- [ ] Implement `centmem compact [--scope X] [--dry-run]`
- [ ] `--dry-run` prints what would be summarized without writing
- [ ] Implement `centmem restore --from file.db` (paired with backup)

### 4. `doctor` command + checks
- [ ] `cmd/centmem/commands/doctor.go` with checks: DB opens + `PRAGMA integrity_check` → "ok"
- [ ] `meta.schema_version` matches binary expectation
- [ ] `sqlite-vec` and `fts5` extensions present
- [ ] Model file exists + sha256 matches catalog
- [ ] Embed queue backlog (`pending_embeddings`) reported; warn if > 1000
- [ ] Permissions: `~/.centmem` is `0700`, DB is `0600`
- [ ] Output JSON `{"ok": true, "checks": [...], "warnings": []}`; exit 0 healthy, exit 1 with failing checks

### 5. `backup` / `restore`
- [ ] `backup --to path.db`: `VACUUM INTO` timestamped file; print `{"ok":true,"backup":"...","size_mb":X}`
- [ ] `restore --from path.db`: verify sha + integrity → replace DB with `.pre-restore.bak` safety copy

### 6. Config retention fields + validation
- [ ] Add `[retention]` block to `internal/config`:
  - `fact_keep_days = 0` (forever)
  - `note_summarize_after_days = 30`
  - `log_summarize_after_days = 14`
  - `log_drop_after_days = 30`
  - `archive_keep_days = 365`
- [ ] `compact` respects retention per scope/type
- [ ] Config validation on load (invalid retention days → clear error)

### 7. Docs polish
- [ ] `README.md` real quickstart (install binary → `init` → first recall)
- [ ] `docs/getting-started.md` step-by-step install for first-time user
- [ ] `docs/troubleshooting.md` common issues (model download blocked, vec0 missing, DB locked) + fixes
- [ ] Keep `docs/cli-contract.md` and `skill/SKILL.md` in sync (contract test from M3 enforces)

### 8. Polish & robustness
- [ ] Consistent error hints in stderr JSON (`hint` field populated)
- [ ] `--quiet` honored across commands
- [ ] Graceful signal handling (SIGINT mid-compact doesn't corrupt; tx rollback)
- [ ] Log all errors with `log/slog` at `--verbose`

### 9. Final coverage + lint + release prep
- [ ] Coverage ≥ 70% on `internal/{store,scope,search,embed,compact}`
- [ ] `go vet ./...` clean; lint green
- [ ] Tag `v1.0.0` release

---

## Test plan (from §4.10)

### Compaction
- [ ] `TestHeuristicSummarizer_Deterministic` — same input → same summary
- [ ] `TestHeuristicSummarizer_Shorter` — summary shorter than concatenated originals
- [ ] `TestCompact_SelectsEligible` — only rows past `summarize_at` touched
- [ ] `TestCompact_GroupedByScopeAndType` — separate groups → separate summaries
- [ ] `TestCompact_ArchivesOriginals` — originals `status='archived'`, excluded from recall
- [ ] `TestCompact_WritesConsolidatedNote` — new note has union tags, `status='active'`
- [ ] `TestCompact_AppendsEvents` — `summarize` events recorded
- [ ] `TestCompact_DropsVectors` — archived rows removed from `memories_vec`
- [ ] `TestCompact_DryRun` — no DB writes in dry-run
- [ ] `TestCompact_RespectsRetentionConfig` — facts never compacted (keep forever)
- [ ] `TestCompact_ReductionTarget` (integration) — 30-day sim: ≥60% active-row reduction
- [ ] `TestCompact_SignalSafe` — SIGINT mid-run → no partial archive

### doctor
- [ ] `TestDoctor_Healthy` — all checks pass; exit 0
- [ ] `TestDoctor_CorruptDB` — integrity_check fails → exit 1 with actionable message
- [ ] `TestDoctor_ModelMissing` — reports missing model + how to `init`
- [ ] `TestDoctor_BadPerms` — reports permissions issue
- [ ] `TestDoctor_QueueBacklog` — warns when pending > 1000

### backup / restore
- [ ] `TestBackup_RoundTrip` — backup → modify → restore → original data present
- [ ] `TestRestore_SafetyCopy` — `.pre-restore.bak` created
- [ ] `TestRestore_RejectsCorrupt` — corrupt file rejected with exit 1

### CLI golden
- [ ] `TestCLI_Compact_DryRun` — JSON shape `{"ok":true,"summarized":N,...}`
- [ ] `TestCLI_Doctor` — JSON checks array
- [ ] `TestCLI_Backup` — JSON `backup` path + size

### Docs
- [ ] `TestDocs_ContractConsistency` (from M3) — still green
- [ ] `TestGettingStarted_SmokeTest` (manual script) — fresh-user runbook executes cleanly

---

## Checklist (§4.9)
- [x] `HeuristicSummarizer` deterministic + tested
- [x] `Compact` archives originals + writes consolidated note
- [x] `compact --dry-run` safe (no writes)
- [x] `events` records `summarize` ops
- [x] `doctor` checks integrity/extensions/model/perms/queue
- [x] `backup` produces restorable file; `restore` verified round-trip
- [x] Config retention fields honored
- [x] README + getting-started + troubleshooting written
- [x] Signal-safe compaction (interrupt → clean state)
- [x] Coverage ≥ 70% on core packages
- [ ] Tag `v1.0.0` release (needs user-authorized commit + tag push)

**Status (2026-08-31):** Implementation and QA gates complete. `go build`, `go vet`,
`gofmt`, and `go test -tags fts5 -race ./...` all green. Coverage: store 83.4%,
scope 91.2%, search 81.5%, embed 74.6%, compact 92.8% (all ≥ 70%). The remaining
`v1.0.0` tag requires committing the M4 changes and pushing a tag, which needs
explicit user authorization.

**Definition of done:** all tests green; `v1.0.0` tagged; release artifacts published; README quickstart verified on a clean machine.
