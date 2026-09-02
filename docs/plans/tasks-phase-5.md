# Phase 5 — Classification Backends & Prompt Engine Task List

> **Source:** `docs/plan-v1.3.0.md` (§ Phase 5 — Classification Backends & Prompt Engine), `docs/architecture.md`, `docs/cli-contract.md`, `docs/data-model.md`  
> **Target Version:** `v1.3.0`  
> **Status:** IN PROGRESS / IMPLEMENTATION & VALIDATION  
> **Goal:** Implement a resilient, multi-backend classification engine supporting a three-tier priority chain (`Local LLM` → `Heuristic` → `BYOK OpenAI-compatible`), customizable prompt templating (`capture-prompt.md`), recall-before-write context injection, one-time malformed response retry with graceful fallback, and strict confidence threshold filtering.

---

## 1. Prerequisites & Dependencies

- [x] **1.1** Verify Phase 1 (`internal/capture` core engine: reader, dedup, writer, summary, session) is fully functional.
- [x] **1.2** Verify Phase 2 (`cmd/centmem/handlers_capture.go`: `run`, `summary`, `categories`, `convert`) CLI commands operate reliably.
- [x] **1.3** Verify Phase 3 (`internal/config` TOML persistence & CLI config getters/setters) provides complete `CaptureConfig` fields.
- [x] **1.4** Verify Phase 4 (Per-harness hook adapters & real-time incremental watcher) operates cleanly across all 7 harnesses.

---

## 2. Track A — Backend Priority Chain & Selection Logic (`internal/capture/classifier.go`)

*Implement dynamic multi-backend dispatch and priority fallback chain ensuring classification never fails silently or blocks execution.*

- [x] **A.1 Priority Fallback Chain Architecture**
  - Chain order:
    1. If `backend == "local-llm"`: attempt local endpoint → on unreachable/error/malformed retry exhaust → fall back to `HeuristicClassifier`.
    2. If `backend == "openai-compatible"`: check API key env → attempt remote endpoint → on missing key/auth error/network failure/malformed retry exhaust → fall back to `HeuristicClassifier`.
    3. If `backend == "heuristic"` or unrecognized: execute `HeuristicClassifier` directly.
- [x] **A.2 Unified Interface & Dispatcher (`ClassifyWithConfig`)**
  - Function signature:
    ```go
    func ClassifyWithConfig(ctx context.Context, messages []TranscriptMessage, cfg CaptureConfig, recallFn RecallFunc) ([]CaptureItem, error)
    ```
  - Factory dispatches to the configured classifier implementation.
  - Automatically filters returned items by `cfg.Categories` (case-insensitive whitelist) and `cfg.ConfidenceThreshold`.
  - Filters out items marked `Skip == true` from returned store writes.

---

## 3. Track B — Local LLM Backend Engine (`internal/capture/classifier.go`)

*Integrate with local OpenAI-compatible inference servers (e.g., Ollama, vLLM, LM Studio, LocalAI) ensuring 100% data privacy with zero external egress.*

- [x] **B.1 Configuration & Endpoint Resolution**
  - Default endpoint: `http://localhost:11434/v1` (Ollama standard).
  - Default model: `llama3.2`.
  - Path normalization: append `/chat/completions` to `LocalLLMEndpoint`.
  - Client timeout: 30 seconds with context deadline propagation.
- [x] **B.2 Payload Construction & Zero-Temperature Request**
  - Format standard chat completions request:
    - `model`: `cfg.LocalLLMModel` (default: `llama3.2`).
    - `messages`: `system` message containing rendered prompt with recall context; `user` message containing JSON array of transcript messages.
    - `temperature`: `0.0` for deterministic, reproducible extraction.
- [x] **B.3 One-Time Retry on Malformed Response & Fallback**
  - Parse response content, stripping markdown code fences (```json ... ``` or ``` ... ```).
  - If JSON unmarshaling fails or HTTP status is 5xx/network error:
    - Retry once immediately.
    - If retry also fails: log warning and seamlessly fall back to `HeuristicClassifier`.

---

## 4. Track C — Lightweight Heuristic Pattern Engine (`internal/capture/classifier.go`)

*Zero-dependency, deterministic pattern-matching engine providing instant classification across all 7 default categories without requiring an active model.*

- [x] **C.1 Multi-Category Pattern Matchers**
  - **`decision`**:
    - Keywords: `"we decided"`, `"we chose"`, `"going with"`, `"agreed on"`, `"architecture decision"`, `"decision:"`, `"we selected"`.
    - Output: `CaptureItem{Category: "decision", Confidence: 0.75, Tags: ["decision"]}`.
  - **`fact`**:
    - URLs: `https?://...` → sanitized key `url.<hostname>.<path>`.
    - Key-Value pairs: `key = value` → key `key`, content `value`.
    - Versions: `vX.Y.Z` semantic version strings.
    - Output: `CaptureItem{Category: "fact", Confidence: 0.75, Tags: ["fact"]}`.
  - **`preference`**:
    - Keywords: `"I prefer"`, `"always use"`, `"don't use"`, `"my convention"`, `"please use"`, `"prefer to"`.
    - Output: `CaptureItem{Category: "preference", Confidence: 0.75, Tags: ["preference"]}`.
  - **`code`**:
    - Patterns: fenced code blocks (` ```<lang>\n<code>\n``` `).
    - Output: extracted code snippet with `Tags: ["code", "snippet"]`, `Confidence: 0.75`.
  - **`log`**:
    - Keywords: leading action verbs `"completed"`, `"finished"`, `"implemented"`, `"added"`, `"fixed"`, `"migrated"`, `"updated"`.
    - Output: `CaptureItem{Category: "log", Confidence: 0.75, Tags: ["log"]}`.
  - **`error`**:
    - Keywords: `"bug:"`, `"root cause"`, `"the fix was"`, `"resolved by"`, `"error:"`, `"failed because"`, `"solution was"`.
    - Output: `CaptureItem{Category: "error", Confidence: 0.75, Tags: ["error", "resolution"]}`.
  - **`dependency`**:
    - Patterns: `go get <pkg>`, `npm install <pkg>`, `pip install <pkg>`, `import "<pkg>"`, `require "<pkg>"`.
    - Output: `Key: "dep.<pkg>"`, `Category: "dependency"`, `Confidence: 0.75`.
- [x] **C.2 Sanitization & Key Normalization**
  - Implement `sanitizeKey()` to convert URLs, package paths, and identifiers into dot-notation strings (`github.com/org/repo` → `github.com.org.repo`).
  - Strip ANSI sequences and extraneous whitespace.

---

## 5. Track D — BYOK OpenAI-Compatible Backend Engine (`internal/capture/classifier.go`)

*Connect to cloud inference providers (OpenAI, Groq, Together, Deepseek, Anthropic shim) using Bring-Your-Own-Key authentication.*

- [x] **D.1 Secure Key Resolution (Zero Secrets in Config)**
  - Read key dynamically from environment variable specified by `cfg.APIKeyEnv` (default: `OPENAI_API_KEY`).
  - Never store plaintext API keys in `config.toml`.
  - If target environment variable is empty or unset:
    - Seamlessly trigger fallback to `HeuristicClassifier`.
- [x] **D.2 Endpoint & Model Configuration**
  - Default base URL: `https://api.openai.com/v1`.
  - Default model: `gpt-4o-mini`.
  - Pass `Authorization: Bearer <API_KEY>` header.
  - Timeout: 30 seconds with context deadline.
- [x] **D.3 Malformed Response Retry & Fallback**
  - If remote API returns 401/403 (invalid key) or 429/500, or if response JSON is corrupted:
    - Retry once on malformed response.
    - Fall back to `HeuristicClassifier` on continued failure.

---

## 6. Track E — Prompt Template Engine & Customization (`internal/capture/classifier.go`)

*Provide customizable classification prompt formatting with runtime variable interpolation and user-override support.*

- [x] **E.1 Default Prompt Template (`DefaultPromptTemplate`)**
  - Encodes the strict JSON contract:
    - `category`: one of `[{{CATEGORIES}}]`.
    - `content`: concise, self-contained summary.
    - `key`: dot-notation key for facts and dependencies.
    - `tags`: array of relevant string tags.
    - `confidence`: float between 0.0 and 1.0.
    - Deduplication rule: `{"skip": true, "skip_reason": "already exists", "content": "..."}` if info exists in `{{RECALL_CONTEXT}}`.
- [x] **E.2 User-Editable Prompt File (`~/.centmem/capture-prompt.md`)**
  - `BuildPrompt` checks for custom template file in home directory or `cfg.PromptTemplatePath`.
  - Falls back to `DefaultPromptTemplate` if file is absent.
- [x] **E.3 Dynamic Variable Interpolation**
  - `{{CATEGORIES}}` → comma-separated list of active categories.
  - `{{CONFIDENCE_THRESHOLD}}` → formatted threshold string (`"0.70"`).
  - `{{RECALL_CONTEXT}}` → formatted bullet list of recalled memory snippets or `"(none)"`.

---

## 7. Track F — Recall-Before-Write & Deduplication Integration

*Prevent repetitive memory ingestion across sessions and conversation turns by fusing vector/keyword recall into classification.*

- [x] **F.1 Context Gathering via `RecallFunc`**
  - Before LLM prompt execution, sample conversation messages and invoke `recallFn(ctx, query)`.
  - Retrieve top-3 to top-5 most semantically relevant memories from active store.
  - Inject retrieved snippets into `{{RECALL_CONTEXT}}`.
- [x] **F.2 Skip Item Handling**
  - LLM outputs `{"skip": true, "skip_reason": "..."}` for redundant information.
  - Filter skip items from write queue while logging duplicate count to `SummaryTracker`.
- [x] **F.3 Confidence Threshold Enforcement**
  - Filter items where `item.Confidence < cfg.ConfidenceThreshold`.
  - Low-confidence candidates are recorded in summary tracker as `skipped_low_confidence` and excluded from store writes.

---

## 8. Track G — Summary & Provenance Telemetry (`internal/capture/summary.go`)

*Ensure transparent accounting of classification quality, backend usage, and skip reasons.*

- [x] **G.1 Backend Telemetry in `CaptureSummary`**
  - `Backend`: name of classifier backend used (`"local-llm"`, `"heuristic"`, `"openai-compatible"`).
  - `TotalMessages`: count of messages processed.
  - `Captured`: count of successfully stored memories.
  - `SkippedDuplicate`: count of memories skipped due to existing matches.
  - `SkippedLowConfidence`: count of items rejected due to confidence score below threshold.
- [x] **G.2 Persistence & Inspection**
  - Summary saved to `~/.centmem/capture-summary-<session-id>.json`.
  - Accessible via `centmem capture summary [--session <id>]`.

---

## 9. Implementation Order

1. **Step 1:** Review and extend `internal/capture/classifier.go` with one-time retry logic on malformed LLM responses.
2. **Step 2:** Ensure `BuildPrompt` supports custom template loading from `~/.centmem/capture-prompt.md` with fallback to `DefaultPromptTemplate`.
3. **Step 3:** Validate regex matchers and key sanitization in `HeuristicClassifier` for all 7 default categories.
4. **Step 4:** Verify BYOK OpenAI-compatible classifier behavior with missing env var and mock API endpoints.
5. **Step 5:** Verify Local LLM classifier fallback behavior when endpoints are offline or returning 500s.
6. **Step 6:** Implement comprehensive unit test suite `internal/capture/classifier_phase5_test.go` covering all priority chain paths, retries, prompt interpolation, and confidence thresholds.
7. **Step 7:** Run full race detector and integration tests across the entire repository (`go test -tags fts5 -race ./...`).
8. **Step 8:** Run `graphify update .` to keep knowledge graph up to date.

---

## 10. Test Plan (Validation)

*All tests below must pass cleanly with `go test -tags fts5 -race ./...`.*

### 10.1 Heuristic Classifier Unit Tests (`internal/capture/classifier_phase5_test.go`)
- [ ] `TestHeuristic_AllSevenCategories`: Verify extraction of `decision`, `preference`, `code`, `log`, `error`, `dependency`, and `fact` with confidence `0.75`.
- [ ] `TestHeuristic_KeySanitization`: Verify URL and package path sanitization into dot notation.
- [ ] `TestHeuristic_EmptyAndWhitespace`: Verify no crash or false items on empty/whitespace inputs.

### 10.2 Local LLM Classifier Tests
- [ ] `TestLocalLLM_Success`: Mock 200 OK OpenAI chat completion returning valid `CaptureItem` JSON.
- [ ] `TestLocalLLM_MalformedJSON_RetrySuccess`: Mock first response malformed, second response valid JSON -> verify successful extraction after retry.
- [ ] `TestLocalLLM_EndpointOffline_FallbackToHeuristic`: Mock unreachable endpoint -> verify seamless fallback to heuristic classifier without error.
- [ ] `TestLocalLLM_500InternalError_FallbackToHeuristic`: Mock 500 error from server -> verify fallback to heuristic classifier.

### 10.3 OpenAI-Compatible Classifier Tests
- [ ] `TestOpenAICompatible_Success`: Mock 200 OK with `Authorization: Bearer` header validation.
- [ ] `TestOpenAICompatible_MissingAPIKeyEnv_Fallback`: Set nonexistent env var -> verify instant fallback to heuristic without HTTP call.
- [ ] `TestOpenAICompatible_MalformedResponse_Fallback`: Mock persistent malformed response -> verify retry followed by fallback to heuristic.

### 10.4 Prompt Engine & Interpolation Tests
- [ ] `TestPrompt_DefaultInterpolation`: Verify `{{CATEGORIES}}`, `{{CONFIDENCE_THRESHOLD}}`, `{{RECALL_CONTEXT}}` correctly replaced.
- [ ] `TestPrompt_CustomTemplateFile`: Create temp custom `capture-prompt.md` -> verify `BuildPrompt` respects custom file content.
- [ ] `TestPrompt_RecallSnippetsFormatting`: Verify recalled memories format as markdown bullet list.

### 10.5 Recall-Before-Write & Confidence Filtering Tests
- [ ] `TestClassifyWithConfig_ConfidenceThresholdFilter`: Verify items below threshold are excluded.
- [ ] `TestClassifyWithConfig_CategoryWhitelist`: Verify items in non-configured categories are excluded.
- [ ] `TestClassifyWithConfig_SkipItemsFiltered`: Verify items with `Skip: true` are filtered out.

### 10.6 End-to-End Integration Tests (`cmd/centmem/e2e_capture_test.go`)
- [ ] `TestE2E_CaptureRun_LocalLLM_WithFallback`: Run `centmem capture run` with offline local LLM configured -> verify heuristic captures decisions and summary reports `backend: heuristic`.
- [ ] `TestE2E_CaptureRun_OpenAICompatible`: Run `centmem capture run` with mock OpenAI server -> verify memories stored with correct metadata and confidence.

---

## 11. Checklist (Definition of Done)

- [ ] `LocalLLMClassifier`, `OpenAICompatibleClassifier`, and `HeuristicClassifier` operate in three-tier priority fallback chain.
- [ ] One-time retry on malformed LLM responses is implemented before falling back.
- [ ] `capture-prompt.md` custom prompt loading and variable interpolation are fully supported.
- [ ] Recall context is injected into LLM classification prompts.
- [ ] Confidence threshold filtering cleanly excludes sub-threshold candidates.
- [ ] All 7 default categories are correctly recognized by heuristic pattern matching.
- [ ] Comprehensive Phase 5 test suite passes with `-race` enabled (`go test -tags fts5 -race ./...`).
- [ ] `go vet -tags fts5 ./...` reports 0 issues.
- [ ] `graphify update .` is run to keep the knowledge graph current.
