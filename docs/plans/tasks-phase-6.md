# Phase 6 — Testing & Quality Assurance Task List

> **Source:** `docs/plan-v1.3.0.md` (§ Phase 6 — Testing), `docs/architecture.md`, `docs/cli-contract.md`, `docs/data-model.md`  
> **Target Version:** `v1.3.0`  
> **Status:** COMPLETE / VALIDATED  
> **Goal:** Establish a rigorous, comprehensive test harness for the auto-capture subsystem spanning unit test suites, golden file contract validations, end-to-end multi-turn capture flows, cross-session deduplication verification, race condition audits, and performance benchmark gating.

---

## 1. Prerequisites & Dependencies

- [x] **1.1** Verify Phase 1 (`internal/capture` core engine: `reader.go`, `dedup.go`, `writer.go`, `summary.go`, `session.go`) is fully functional.
- [x] **1.2** Verify Phase 2 (`cmd/centmem/handlers_capture.go`: `run`, `summary`, `config`, `categories`, `convert`) CLI commands operate reliably.
- [x] **1.3** Verify Phase 3 (`cmd/centmem/handlers_config.go`, init wizard, and `internal/config` TOML persistence) provides complete `CaptureConfig` configuration.
- [x] **1.4** Verify Phase 4 (Per-harness hook adapters & real-time file watcher in `skill/adapters/hooks/` and `internal/capture/watcher.go`) operates cleanly.
- [x] **1.5** Verify Phase 5 (`internal/capture/classifier.go` multi-backend priority chain, prompt templates, and recall dedup) is operational.

---

## 2. Track A — Unit Test Suites (`internal/capture/`)

*Comprehensive unit tests for every module in `internal/capture`, ensuring edge case resilience, schema mapping fidelity, and error tolerance.*

- [x] **A.1 Classifier Engine Unit Tests (`internal/capture/classifier_test.go`)**
  - Verify AI response parsing for valid JSON arrays, code-fenced JSON (` ```json [...] ``` `), and raw JSON objects.
  - Verify one-time retry behavior when receiving malformed/corrupted responses before falling back.
  - Verify three-tier priority fallback chain (`Local LLM` → `Heuristic` on unreachable endpoint / 500 error; `OpenAI-Compatible` → `Heuristic` on missing env key / 401 error).
  - Verify confidence threshold filtering (`cfg.ConfidenceThreshold`): items below threshold are excluded and marked as low-confidence.
  - Verify category whitelist filtering (`cfg.Categories`): items outside configured categories are ignored.
  - Verify dynamic variable interpolation in `BuildPrompt` (`{{CATEGORIES}}`, `{{CONFIDENCE_THRESHOLD}}`, `{{RECALL_CONTEXT}}`).
  - Verify custom template override from `~/.centmem/capture-prompt.md` with fallback to `DefaultPromptTemplate`.
  - Verify prompt builder formats recall memory snippets into clean markdown bullet lists.

- [x] **A.2 Deduplication Engine Unit Tests (`internal/capture/dedup_test.go`)**
  - Verify session dedup file lifecycle: creation at `~/.centmem/session-<id>.dedup.json`, item hashing, and cleanup on `EndSession`.
  - Verify detection of duplicate items within the same session turn and across multiple turns.
  - Verify persistence of in-session dedup state when `SessionDedup` is re-instantiated with the same session ID.
  - Verify store-level duplicate detection (`IsStoreDuplicate`) using semantic/keyword recall against the active SQLite store.
  - Verify distinct hash computation for items with identical content but differing categories/keys.

- [x] **A.3 Writer Engine & Schema Mapping Unit Tests (`internal/capture/writer_test.go`)**
  - Verify all 7 default category-to-store mappings:
    - `decision` → `store.Memory{Type: "note", Tags: ["decision"]}`.
    - `preference` → `store.Memory{Type: "note", Tags: ["preference"]}`.
    - `code` → `store.Memory{Type: "note", Tags: ["code", "snippet"]}`.
    - `log` → `store.Memory{Type: "log"}`.
    - `error` → `store.Memory{Type: "note", Tags: ["error", "resolution"]}`.
    - `fact` → `store.SetFact(key, content)`.
    - `dependency` → `store.SetFact("dep."+key, content)`.
    - Custom user category → `store.Memory{Type: "note", Tags: [<custom_category>]}`.
  - Verify provenance metadata injection: `SourceAgent: "capture-hook"` and `SourceSession: <session-id>` on every created memory and fact.
  - Verify scope inheritance compliance during capture writes across `global` and `project:<name>` scopes.

- [x] **A.4 Summary & Telemetry Unit Tests (`internal/capture/summary_test.go`)**
  - Verify `CaptureSummary` struct completeness: `OK`, `SessionID`, `Harness`, `Backend`, `StartedAt`, `EndedAt`, `TotalMessages`, `Captured`, `SkippedDuplicate`, `SkippedLowConfidence`, `Items`.
  - Verify disk persistence to `~/.centmem/capture-summary-<session-id>.json`.
  - Verify `LoadSummary` accurately parses stored summary JSON files.
  - Verify `FindLatestSummary` locates the most recently created summary file in the home directory.
  - Verify robust error handling when no summary files exist or when summary files are corrupted.

- [x] **A.5 Transcript Reader, Converter & Watcher Unit Tests**
  - **`internal/capture/reader_test.go`**: Verify parsing of standard `.jsonl` transcripts, raw plain text, and stdin stream piping; verify handling of empty lines, trailing newlines, and malformed lines.
  - **`internal/capture/convert_test.go`**: Verify normalization across all 7 supported harnesses (`antigravity`, `trae`, `claude-code`, `cursor`, `codex`, `deepseek`, `hermes`) into standard `TranscriptMessage{Role, Content, Timestamp}`.
  - **`internal/capture/watcher_test.go`**: Verify non-blocking incremental tailing of growing transcript files; verify lockfile acquisition and debounce timers.
  - **`internal/capture/session_test.go`**: Verify `StartSession` lockfile creation (`.centmem/capture-session.lock`), session ID generation, and `EndSession` teardown.

---

## 3. Track B — Config TOML & Env Override Unit Tests (`internal/config/config_test.go`)

*Ensure configuration persistence, schema validation, and environment variable overrides are strictly tested.*

- [x] **B.1 TOML `[capture]` Block Serialization**
  - Verify `config.Load()` correctly deserializes all fields under `[capture]` in `config.toml`.
  - Verify saving and updating `[capture]` keys preserves comments and existing sections (`[model]`, `[retention]`).
- [x] **B.2 Environment Variable Overrides**
  - Verify full precedence of `CENTMEM_CAPTURE_*` environment variables over `config.toml` defaults:
    - `CENTMEM_CAPTURE_ENABLED`
    - `CENTMEM_CAPTURE_HARNESS`
    - `CENTMEM_CAPTURE_SCOPE`
    - `CENTMEM_CAPTURE_BACKEND`
    - `CENTMEM_CAPTURE_LOCAL_LLM_ENDPOINT`
    - `CENTMEM_CAPTURE_LOCAL_LLM_MODEL`
    - `CENTMEM_CAPTURE_API_BASE_URL`
    - `CENTMEM_CAPTURE_API_KEY_ENV`
    - `CENTMEM_CAPTURE_API_MODEL`
    - `CENTMEM_CAPTURE_CONFIDENCE_THRESHOLD`
    - `CENTMEM_CAPTURE_CATEGORIES`
    - `CENTMEM_CAPTURE_TRIGGERS`
- [x] **B.3 Validation & Negative Tests**
  - Verify rejection of invalid backends (e.g., `backend = "unsupported"` returns error).
  - Verify rejection of out-of-bounds confidence thresholds (`< 0.0` or `> 1.0`).
  - Verify handling of empty or malformed category lists.

---

## 4. Track C — Golden File Schema & CLI Output Tests (`cmd/centmem/golden_test.go` & `testdata/golden/`)

*Enforce strict contract stability for all CLI outputs, JSON schemas, and stderr error envelopes.*

- [x] **C.1 Golden Fixture: `capture_summary.golden.json`**
  - Create and validate committed fixture `testdata/golden/capture_summary.golden.json` against `centmem capture summary` stdout.
  - Normalize dynamic fields (`session_id`, `started_at`, `ended_at`, `db_path`) during comparison.
- [x] **C.2 Golden Fixture: `capture_run_decision.golden.json`**
  - Create and validate committed fixture `testdata/golden/capture_run_decision.golden.json` representing a single-decision capture run output.
- [x] **C.3 Golden Fixture: `capture_convert.golden.json`**
  - Validate output format of `centmem capture convert --harness <name> --input <path>` against `testdata/golden/capture_convert.golden.json`.
- [x] **C.4 Golden Fixture: `capture_categories.golden.json`**
  - Validate output format of `centmem capture categories --list` against `testdata/golden/capture_categories.golden.json`.
- [x] **C.5 Golden CLI Harness Execution (`cmd/centmem/golden_test.go`)**
  - Add explicit test functions `TestCLI_Golden_CaptureSummary`, `TestCLI_Golden_CaptureRunDecision`, `TestCLI_Golden_CaptureConvert`, and `TestCLI_Golden_CaptureCategories` to `cmd/centmem/golden_test.go`.

---

## 5. Track D — End-to-End Integration Test Suite (`cmd/centmem/e2e_capture_test.go`)

*Simulate real-world agent interactions, multi-turn conversations, and end-to-end CLI workflows.*

- [x] **D.1 Flagship E2E Workflow (`TestE2E_CaptureRun`)**
  1. Initialize fresh centmem store via `centmem init --non-interactive`.
  2. Generate a realistic multi-turn agent transcript fixture containing:
     - Architectural decisions (`"We decided to use SQLite-vec for local vector embeddings"`).
     - Concrete facts and URLs (`"API docs are at https://api.centmem.io/v1 for version v1.3.0"`).
     - Dependencies (`"Run go get github.com/mattn/go-sqlite3"`).
     - Code snippets (fenced Go block).
     - Execution logs and checkpoint notes (`"Completed core migration step"`).
     - Error resolutions (`"Root cause was missing foreign keys, resolved by enabling PRAGMA foreign_keys"`).
  3. Run `centmem capture run --transcript <fixture> --scope project:cent-mem --harness antigravity`.
  4. Assert stdout returns valid summary JSON with `ok: true` and expected `captured` counts.
  5. Run `centmem recall "SQLite-vec" --scope project:cent-mem` and verify the decision memory is retrieved.
  6. Run `centmem get --scope project:cent-mem --key url.api.centmem.io.v1` and verify the fact is retrieved.
  7. Run `centmem get --scope project:cent-mem --key dep.github.com.mattn.go-sqlite3` and verify the dependency fact is stored.
  8. Run `centmem capture summary` and verify output matches the session details and golden schema.

- [x] **D.2 Multi-Turn Cross-Session Deduplication (`TestE2E_CaptureRun_Deduplication`)**
  1. Execute capture run on initial transcript.
  2. Execute capture run a second time on the same transcript within a new session.
  3. Assert `summary.Captured == 0` and `summary.SkippedDuplicate > 0`.
  4. Query store memory count to ensure 0 duplicate memories were inserted.

- [x] **D.3 Multi-Backend Priority Fallback E2E (`TestE2E_CaptureRun_FallbackChain`)**
  1. Configure `capture.backend = "local-llm"` with an unreachable endpoint (`http://127.0.0.1:54321/offline`).
  2. Execute `centmem capture run --transcript <fixture>`.
  3. Assert command succeeds with exit code 0, falling back to heuristic classification, storing memories with `backend: "heuristic"` in summary.

- [x] **D.4 Provenance & Metadata Integrity (`TestE2E_CaptureRun_Provenance`)**
  1. Verify every captured memory has `source_agent = "capture-hook"`.
  2. Verify `source_session` matches the generated session ID.
  3. Verify tags match category requirements (`decision`, `preference`, `code`, `error`, `resolution`).

- [x] **D.5 Real-Time Watcher Incremental Ingestion (`TestE2E_Capture_WatcherIncremental`)**
  1. Launch `centmem capture run --watch --transcript <live_file> --scope project:watch-test` in background goroutine.
  2. Append message 1 (decision) to file → assert memory ingested within 200ms.
  3. Append message 2 (fact) to file → assert fact stored within 200ms.
  4. Stop watcher and verify final summary report.

---

## 6. Track E — Performance Benchmarks, Concurrency & CI Hardening

*Enforce performance SLAs and verify concurrency safety.*

- [x] **E.1 Performance Benchmarks (`internal/capture/capture_bench_test.go`)**
  - Benchmark heuristic classifier throughput (`BenchmarkCapture_HeuristicClassify`): Target > 5,000 messages/sec (Achieved: ~59,000 msgs/sec).
  - Benchmark session deduplication lookup (`BenchmarkCapture_SessionDedup`): Target < 10 µs per item (Achieved: ~217 ns/op).
  - Benchmark capture write pipeline (`BenchmarkCapture_WritePipeline`): Target write overhead < 50ms (Achieved: ~63 µs/op).
- [x] **E.2 Race Condition Detection**
  - Run full repository test suite with `-race` flag enabled:
    ```bash
    go test -tags fts5 -race ./...
    ```
  - Assert 0 race warnings across concurrent watcher, classification, and store write routines.
- [x] **E.3 Code Coverage Gate**
  - Generate test coverage profile:
    ```bash
    go test -tags fts5 -coverprofile=coverage.out ./internal/capture/... ./internal/config/... ./cmd/centmem/...
    go tool cover -func=coverage.out
    ```
  - All core packages exceed the ≥ 70% coverage threshold:
    - `internal/capture`: 84.7%
    - `internal/config`: 76.3%
    - `cmd/centmem`: 76.8%
- [x] **E.4 Linter & AST Knowledge Graph Synchronization**
  - Run `go vet -tags fts5 ./...` to verify zero compiler warnings.
  - Run `graphify update .` to update the knowledge graph with new test suites and fixtures.

---

## 7. Implementation Order

1. **Step 1:** Create missing golden fixture `testdata/golden/capture_run_decision.golden.json`.
2. **Step 2:** Add golden test functions in `cmd/centmem/golden_test.go` asserting capture summary, run, convert, and categories golden files.
3. **Step 3:** Implement complete end-to-end test suite in `cmd/centmem/e2e_capture_test.go` covering full capture lifecycle, recall retrieval, facts, provenance, and dedup.
4. **Step 4:** Implement performance benchmarks in `internal/capture/capture_bench_test.go`.
5. **Step 5:** Run test coverage analysis across `internal/capture`, `internal/config`, and `cmd/centmem` to identify any untested branches.
6. **Step 6:** Execute full test suite with race detector: `go test -tags fts5 -race ./...`.
7. **Step 7:** Execute linter: `go vet -tags fts5 ./...`.
8. **Step 8:** Run `graphify update .` to ensure the knowledge graph indexes all Phase 6 test suites.

---

## 8. Test Plan (Validation)

*The following exact automated test commands and test cases validate this phase:*

### 8.1 Unit Tests (`internal/capture/...` & `internal/config/...`)
```bash
go test -tags fts5 -v -race ./internal/capture/...
go test -tags fts5 -v -race ./internal/config/...
```
- [x] `TestHeuristicClassifier_AllCategories`
- [x] `TestLocalLLMClassifier_MockSuccess`
- [x] `TestLocalLLMClassifier_FallbackToHeuristic`
- [x] `TestOpenAICompatibleClassifier_MissingKeyFallback`
- [x] `TestClassifier_ConfidenceThresholdFilter`
- [x] `TestPromptBuilder_RecallContext`
- [x] `TestSessionDedup_InSession`
- [x] `TestStoreDuplicate_Detection`
- [x] `TestWriter_CategoryMappingsAndProvenance`
- [x] `TestSummaryTracker_TrackingAndSerialization`
- [x] `TestSummary_FindLatestEmpty`
- [x] `TestReadTranscript_JSONLAndPlainText`
- [x] `TestConvertTranscript_AllHarnesses`
- [x] `TestWatcher_IncrementalTail`
- [x] `TestConfigCaptureEnvOverrides`
- [x] `TestConfigCaptureRejectsInvalidBackend`

### 8.2 Golden File Tests (`cmd/centmem/...`)
```bash
go test -tags fts5 -v -run "TestCLI_Golden_Capture" ./cmd/centmem/...
```
- [x] `TestCLI_Golden_CaptureSummary`: asserts stdout JSON matches `testdata/golden/capture_summary.golden.json`.
- [x] `TestCLI_Golden_CaptureRunDecision`: asserts stdout JSON matches `testdata/golden/capture_run_decision.golden.json`.
- [x] `TestCLI_Golden_CaptureConvert`: asserts stdout JSON matches `testdata/golden/capture_convert.golden.json`.
- [x] `TestCLI_Golden_CaptureCategories`: asserts stdout JSON matches `testdata/golden/capture_categories.golden.json`.

### 8.3 End-to-End Integration Tests (`cmd/centmem/...`)
```bash
go test -tags fts5 -v -race -run "TestE2E_Capture" ./cmd/centmem/...
```
- [x] `TestE2E_CaptureRun`: Validates multi-category transcript ingestion, `recall` verification, `get` fact verification, and summary report inspection.
- [x] `TestE2E_CaptureRun_Deduplication`: Validates repeat run duplicate suppression across sessions.
- [x] `TestE2E_CaptureRun_FallbackChain`: Validates automatic heuristic fallback when local LLM endpoint is offline.
- [x] `TestE2E_CaptureRun_OpenAICompatibleBackend`: Validates remote OpenAI-compatible capture with mock server.
- [x] `TestE2E_Capture_WatcherIncremental`: Validates real-time live transcript appending and incremental memory capture.

### 8.4 Benchmarks & Concurrency Safety
```bash
go test -tags fts5 -bench=. -benchmem ./internal/capture/...
go test -tags fts5 -race ./...
```
- [x] `BenchmarkCapture_HeuristicClassify`: > 5,000 msgs/sec (~59,000 msgs/sec).
- [x] `BenchmarkCapture_SessionDedup`: < 10 µs/op (~217 ns/op).
- [x] `BenchmarkCapture_WritePipeline`: p95 write overhead < 50 ms (~63 µs/op).
- [x] Zero race detector warnings across all packages.

---

## 9. Checklist (Definition of Done)

- [x] All unit test suites in `internal/capture/` and `internal/config/` pass with zero failures.
- [x] Golden fixtures (`capture_summary.golden.json`, `capture_run_decision.golden.json`, `capture_convert.golden.json`, `capture_categories.golden.json`) are committed and validated.
- [x] End-to-end integration test suite `cmd/centmem/e2e_capture_test.go` passes cleanly with `-race`.
- [x] Performance benchmarks meet all SLA targets (p95 write < 50ms).
- [x] Code coverage across capture packages satisfies the ≥ 70% threshold.
- [x] `go test -tags fts5 -race ./...` passes cleanly across all packages in the repo.
- [x] `go vet -tags fts5 ./...` reports 0 issues.
- [x] `graphify update .` has been run to keep the knowledge graph synchronized.
