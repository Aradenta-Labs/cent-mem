# Phase 7 — Documentation, Release & Governance Task List

> **Source:** `docs/plan-v1.3.0.md` (§ Phase 7 — Documentation & Release), `docs/architecture.md`, `docs/cli-contract.md`, `docs/data-model.md`  
> **Target Version:** `v1.3.0`  
> **Status:** COMPLETE / VALIDATED  
> **Goal:** Complete all user-facing and developer documentation, author the comprehensive Capture Hooks Guide (`docs/guides/capture-hooks.md`), update getting started guides, refresh `skill/SKILL.md` recipes, draft `CHANGELOG.md` for `v1.3.0`, update `README.md` roadmap and badges, sync `docs/README.md`, enforce doc-contract consistency and markdown link integrity with automated tests, and synchronize the AST knowledge graph.

---

## 1. Prerequisites & Dependencies

- [x] **1.1** Verify Phase 1 (`internal/capture` core engine: reader, classifier, dedup, writer, summary, session) is fully functional and tested.
- [x] **1.2** Verify Phase 2 (`cmd/centmem/handlers_capture.go`: `run`, `summary`, `config`, `categories`, `convert`) CLI commands operate reliably.
- [x] **1.3** Verify Phase 3 (`cmd/centmem/handlers_config.go`, init wizard, and `internal/config` TOML persistence) provides complete `CaptureConfig` access.
- [x] **1.4** Verify Phase 4 (Per-harness hook adapters in `skill/adapters/hooks/` and incremental watcher) is complete and operational.
- [x] **1.5** Verify Phase 5 (Multi-backend classification priority chain, prompt templating, recall dedup, and confidence filtering) is complete.
- [x] **1.6** Verify Phase 6 (Unit, golden, E2E integration test suites, and performance benchmarks) pass cleanly with `go test -tags fts5 -race ./...`.

---

## 2. Track A — Getting Started Guide Updates (`docs/guides/getting-started.md`)

*Update the primary onboarding guide to introduce transcript auto-capture alongside existing manual memory operations.*

- [x] **A.1 Add Section: "9. Auto-capture from agent transcripts"**
  - Added Section 9 following the existing "8. Connect your AI agents" section.
  - Explains the **zero-touch memory loop**: transcripts are ingested automatically in the background without requiring agents to manually format `put` / `set` commands.
- [x] **A.2 Document Capture Trigger Modes**
  - **Background / per-message:** Real-time incremental file watcher (`centmem capture run --watch`).
  - **Session-end:** Shell exit traps / harness lifecycle hooks running `centmem capture run` and printing summaries.
  - **On-demand:** Running `centmem capture run --transcript <path> --scope project:myapp`.
- [x] **A.3 Document Verification & Inspection Commands**
  - Shows how to inspect captured memories using `centmem capture summary`.
  - Shows how to verify stored memories using `centmem recall` and `centmem get`.
- [x] **A.4 Document Category Customization**
  - Shows `centmem capture categories --list`, `--add <category>`, and `--remove <category>`.
- [x] **A.5 Cross-Link to Deep-Dive Guide**
  - Explicit markdown link to `docs/guides/capture-hooks.md` for full harness configurations and custom backend setup.

---

## 3. Track B — Comprehensive Capture Hooks Guide (`docs/guides/capture-hooks.md`) [NEW]

*Author an exhaustive, definitive user and operator guide dedicated to the auto-capture subsystem.*

- [x] **B.1 Guide Architecture & Overview**
  - High-level data flow diagram showing: `Agent Transcript` → `Harness Hook Adapter` → `Transcript Reader / Converter` → `3-Tier Classifier` → `Recall-Before-Write Dedup` → `SQLite Store` (`memories` / `facts`).
  - Explains design invariants: local-first privacy, zero network egress default, non-blocking asynchronous execution, and idempotent session logging.
- [x] **B.2 Supported Harnesses & Access Matrix**
  - Includes detailed comparison matrix across all 7 supported harnesses:
    | Harness | Transcript Mechanism | Default Path / Method | Recommended Trigger |
    |---|---|---|---|
    | **Antigravity** | JSONL file watcher | `~/.gemini/antigravity/brain/<id>/.system_generated/logs/transcript.jsonl` | Per-message (`--watch`) |
    | **Trae** | Session log monitor | `~/.trae/logs/sessions/<id>/session.log` | Session-end |
    | **Claude Code** | Shell `EXIT` trap | `.claude/` session directory & subshell trap | Session-end |
    | **Cursor** | Composer rule / file export | `.cursorrules` / `.cursor/logs/` | Post-prompt / On-demand |
    | **Codex** | Stdin/stdout pipe shim | `codex "$@" \| tee >(centmem capture run --harness codex)` | Streaming pipe |
    | **DeepSeek** | Session export watcher | `~/.deepseek/sessions/` | Session-end / On-demand |
    | **Hermes** | Event bus plugin | Hermes lifecycle hook API | Per-message / Session-end |
- [x] **B.3 Unified Hook Installer (`install.sh`)**
  - Documents `skill/adapters/hooks/install.sh` CLI options:
    - Auto-detection: `bash skill/adapters/hooks/install.sh --all`
    - Target specific harness: `bash skill/adapters/hooks/install.sh --harness claude-code`
    - Scope override: `bash skill/adapters/hooks/install.sh --scope "project:myapp"`
    - Trigger configuration: `bash skill/adapters/hooks/install.sh --trigger "message,session-end"`
    - Inspection & safety: `--list`, `--dry-run`, `--uninstall`
  - Documents background daemon setup: macOS `launchd` plist and Linux `systemd` user units.
- [x] **B.4 Configuration Reference (Three Interfaces)**
  - **Interface 1 (Interactive Wizard):** `centmem init` capture setup prompts.
  - **Interface 2 (CLI Setters/Getters):** `centmem config set capture.<key> <value>` and `centmem config get capture.<key>`.
  - **Interface 3 (TOML File & Environment Variables):** Comprehensive table of all `[capture]` TOML fields, types, defaults, and corresponding `CENTMEM_CAPTURE_*` env overrides:
    - `enabled`, `harness`, `triggers`, `scope`, `categories`, `transcript_path`, `backend`, `local_llm_endpoint`, `local_llm_model`, `api_base_url`, `api_key_env`, `api_model`, `confidence_threshold`.
- [x] **B.5 Multi-Backend Classifier & Fallback Chain**
  - Deep dive on 3 classifier tiers:
    1. **Local LLM (`local-llm`):** Zero-egress local OpenAI-compatible inference (Ollama `llama3.2`, LM Studio, LocalAI).
    2. **Heuristic (`heuristic`):** Instant, deterministic regex pattern matching across all 7 default categories with zero external dependencies.
    3. **BYOK OpenAI-Compatible (`openai-compatible`):** Cloud LLMs (OpenAI, Groq, Together, DeepSeek) using environment variable key resolution.
  - Explains priority fallback order and one-time retry on malformed JSON responses.
  - Documents privacy guarantee: API keys are never written to `config.toml` (only env var names).
- [x] **B.6 Default Categories & Schema Mapping Rules**
  - Documents all 7 default categories, trigger patterns, and store write mappings:
    - `decision` → `put --type note --tags decision`
    - `fact` → `set --key <sanitized_key> --value <content>`
    - `preference` → `put --type note --tags preference`
    - `code` → `put --type note --tags code,snippet`
    - `log` → `put --type log`
    - `error` → `put --type note --tags error,resolution`
    - `dependency` → `set --key dep.<pkg> --value <version/content>`
  - Explains custom category management: `centmem capture categories --add/remove/list`.
- [x] **B.7 Transcript Normalization (`centmem capture convert`)**
  - Documents conversion usage: `centmem capture convert --harness <name> --input <in> [--output <out>]`.
  - Shows standard normalized JSONL structure (`role`, `content`, `timestamp`).
- [x] **B.8 Session Telemetry & Summary Inspection**
  - Explains `centmem capture summary [--session <id>]` output fields.
  - Documents disk location `~/.centmem/capture-summary-<session-id>.json`.
- [x] **B.9 Customizing Prompt Templates**
  - Explains `~/.centmem/capture-prompt.md` user customization.
  - Documents dynamic interpolation variables: `{{CATEGORIES}}`, `{{CONFIDENCE_THRESHOLD}}`, `{{RECALL_CONTEXT}}`.
- [x] **B.10 Troubleshooting & Common Operational Pitfalls**
  - Recovering from offline local LLM endpoints.
  - Resolving unset API key environment variables.
  - Handling transcript file permission errors.
  - Resolving stale `.centmem/capture-session.lock` files.

---

## 4. Track C — Skill Contract & Runtime Reference (`skill/SKILL.md`)

*Ensure `skill/SKILL.md` provides complete, accurate recipes for AI agents interacting with the capture subsystem.*

- [x] **C.1 Synchronize Command Reference Table**
  - Verifies all 15 CLI commands are listed in `skill/SKILL.md`:
    `init`, `put`, `set`, `get`, `recall`, `timeline`, `list`, `forget`, `stats`, `compact`, `doctor`, `backup`, `restore`, `capture`, `config`.
  - Ensures command usages and descriptions match `docs/cli-contract.md`.
- [x] **C.2 Add Dedicated Auto-Capture Recipes**
  - **Recipe: Trigger On-Demand Capture Pass**
    ```bash
    centmem capture run --transcript "$TRANSCRIPT_PATH" --scope "project:$PROJ"
    ```
  - **Recipe: Inspect Last Session's Captured Memories**
    ```bash
    centmem capture summary
    ```
  - **Recipe: List and Manage Active Categories**
    ```bash
    centmem capture categories --list
    centmem capture categories --add "security"
    ```
  - **Recipe: Normalize External Transcript Files**
    ```bash
    centmem capture convert --harness cursor --input .cursor/logs/session.json
    ```
- [x] **C.3 Document Provenance & Memory Integrity**
  - Clarifies that auto-captured memories carry `source-agent: capture-hook` and `source-session: <id>` metadata.
  - Advises agents on how to leverage auto-captured memories during context initialization.
- [x] **C.4 Cross-Reference Adapter Documentation**
  - Links to `skill/adapters/hooks/` and `docs/guides/capture-hooks.md`.

---

## 5. Track D — Top-Level Documentation & Roadmap Updates (`README.md`, `docs/README.md`)

*Refresh public README, feature matrices, documentation sitemaps, and roadmap milestones.*

- [x] **D.1 Update `README.md`**
  - Update version badge: `v1.3.0`.
  - Add Auto-Capture feature row in the features table:
    `🎣 | **Auto-capture** — extracts durable decisions, facts, and code from agent transcripts automatically`
  - Update CLI command reference and cheatsheet to include `centmem capture` and `centmem config`.
  - Add `docs/guides/capture-hooks.md` to the user documentation table.
  - Update Roadmap: Mark `v1.3` (Auto-capture from agent transcripts / hooks) as completed:
    ```markdown
    - [x] **v1.0** — Core store, hybrid search, skill, compaction, polish
    - [x] **v1.2** — Frictionless UX (npx installer, workflow loop injection, /centmem slash command)
    - [x] **v1.3** — Auto-capture from agent transcripts (hooks & multi-backend classification)
    - [ ] **v1.4** — TUI browser (`centmem ui`)
    - [ ] **v2.0** — Sync server (`centmemd`), multi-machine replication via events log, remote embedding providers
    ```
- [x] **D.2 Update `docs/README.md` (Documentation Sitemap)**
  - Add `[guides/capture-hooks.md](guides/capture-hooks.md)` to Guides section table.
  - Update Implementation Plans & Milestones table:
    - Mark Phase 5 (`plans/tasks-phase-5.md`), Phase 6 (`plans/tasks-phase-6.md`), and Phase 7 (`plans/tasks-phase-7.md`) as Completed.
  - Update reading order recommendation for developers working on the capture engine.

---

## 6. Track E — Release Notes & Version Governance (`CHANGELOG.md`, `npm/package.json`)

*Draft comprehensive, professional release notes adhering to Keep a Changelog standards.*

- [x] **E.1 Draft `[1.3.0] - 2026-09-02` Section in `CHANGELOG.md`**
  - **Summary:** Major feature release adding automatic transcript capture, multi-backend classification, hook adapters for 7 agent harnesses, real-time incremental watching, and unified configuration management.
  - **Added:**
    - **Capture Engine (`internal/capture/`):** Full auto-capture pipeline with `reader.go`, `classifier.go`, `dedup.go`, `writer.go`, `summary.go`, `session.go`, `watcher.go`, and `convert.go`.
    - **Three-Tier Classification Engine:** Dynamic fallback chain supporting Local LLM (Ollama/vLLM), deterministic Heuristic pattern matching, and BYOK OpenAI-compatible endpoints with one-time retry and confidence threshold gating.
    - **Harness Hook Adapters (`skill/adapters/hooks/`):** Adapter specs and recipes for Antigravity, Trae, Claude Code, Cursor, Codex, DeepSeek, and Hermes.
    - **Hook Installer (`skill/adapters/hooks/install.sh`):** Automated harness detection, idempotent hook registration, shell trap injection, and `launchd`/`systemd` service setup.
    - **Real-Time Incremental Watcher (`--watch`):** Low-overhead file watcher with state persistence across restarts, debouncing, and graceful interrupt handling.
    - **CLI Commands (`cmd/centmem/`):** `centmem capture run`, `centmem capture summary`, `centmem capture categories`, `centmem capture convert`, `centmem config set`, `centmem config get`.
    - **Init Wizard Extension:** Interactive setup prompts for capture backend, endpoints, triggers, and categories during `centmem init`.
    - **Prompt Templating:** User-editable `~/.centmem/capture-prompt.md` with dynamic variable interpolation (`{{CATEGORIES}}`, `{{CONFIDENCE_THRESHOLD}}`, `{{RECALL_CONTEXT}}`).
    - **Comprehensive Documentation:** Added `docs/guides/capture-hooks.md` and updated `docs/guides/getting-started.md`, `skill/SKILL.md`, and `README.md`.
  - **Changed:**
    - `internal/config`: Expanded `Config` schema with `[capture]` block and `CENTMEM_CAPTURE_*` environment variable overrides.
    - Contract test suite extended to enforce CLI and documentation synchronization for `capture` and `config`.
  - **Security:**
    - 100% local-first classification default ensuring zero data egress.
    - API keys for cloud backends are never written to `config.toml` (only environment variable names are referenced).
    - Session lockfiles (`.centmem/capture-session.lock`) prevent concurrent write races.
- [x] **E.2 Verify Release Links & Tags**
  - Ensure release comparison link `[1.3.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.3.0` is added at the bottom of `CHANGELOG.md`.

---

## 7. Track F — Documentation Consistency & Link Integrity Test Suites

*Create automated tests to prevent documentation drift, broken internal links, and invalid CLI examples.*

- [x] **F.1 Verify CLI & Doc Contract Alignment (`cmd/centmem/contract_test.go`)**
  - Run existing `TestContract_CommandsMatchDocs` to verify that all 15 commands in `Names()` match `docs/cli-contract.md` AND `skill/SKILL.md`.
  - Ensure zero discrepancies between registered commands and documented commands.
- [x] **F.2 Implement Documentation Link Integrity Test (`cmd/centmem/doc_links_test.go`) [NEW]**
  - Implement `TestDocs_InternalMarkdownLinks`:
    - Parse all markdown links (`[text](path)`) in `README.md`, `docs/README.md`, `docs/guides/getting-started.md`, `docs/guides/capture-hooks.md`, and `skill/SKILL.md`.
    - Resolve relative paths and verify target files exist on the filesystem.
    - Fails if any relative link is broken or points to a non-existent file.
- [x] **F.3 Implement Doc Code Example Smoke Test (`cmd/centmem/doc_examples_test.go`) [NEW]**
  - Implement `TestDocs_CLIExampleFlags`:
    - Extract all `centmem <subcommand> [flags]` code snippet invocations from `docs/guides/capture-hooks.md`, `docs/guides/getting-started.md`, and `skill/SKILL.md`.
    - Verify that all referenced subcommands and flags are registered in the CLI command registry.
    - Guards against documented flags drifting from actual CLI implementation.

---

## 8. Track G — Knowledge Graph & Quality Verification

*Synchronize the graphify knowledge graph and verify full repository test suite integrity.*

- [x] **G.1 AST Knowledge Graph Synchronization**
  - Execute `graphify update .` to index all new documentation, guides, and test suites.
  - Verify `graphify query "capture-hooks"` returns accurate references to the new guide and adapters.
- [x] **G.2 Full Regression & Race Test Gate**
  - Run the entire test suite with race detection:
    ```bash
    go test -tags fts5 -race ./...
    ```
  - Assert 100% tests passing across all packages.
- [x] **G.3 Linter & Code Hygiene**
  - Run `go vet -tags fts5 ./...` to ensure zero linter or compiler warnings.
  - Run `gofmt -s -w .` to verify formatting across all Go files.
- [x] **G.4 Node.js Installer Test Suite Gate**
  - Run `npm test` in the `npm/` directory to ensure npm installer tests continue to pass.

---

## 9. Implementation Order

1. **Step 1:** Author comprehensive `docs/guides/capture-hooks.md` covering architecture, installation, configuration, backends, categories, normalization, telemetry, prompt customization, and troubleshooting.
2. **Step 2:** Update `docs/guides/getting-started.md` with Section 9 ("Auto-capture from agent transcripts") and cross-links.
3. **Step 3:** Update `skill/SKILL.md` with `capture` and `config` command entries, recipe 6 (Auto-capture), and provenance guidance.
4. **Step 4:** Update `README.md` (version badge, feature matrix, usage cheatsheet, roadmap) and `docs/README.md` (guide tables and plan statuses).
5. **Step 5:** Draft release notes in `CHANGELOG.md` for `v1.3.0`.
6. **Step 6:** Implement doc link integrity test in `cmd/centmem/doc_links_test.go` and doc example flag verification in `cmd/centmem/doc_examples_test.go`.
7. **Step 7:** Run contract tests (`go test -tags fts5 -run TestContract ./cmd/centmem/...`) and doc tests (`go test -tags fts5 -run TestDocs ./cmd/centmem/...`).
8. **Step 8:** Run full repository race test suite (`go test -tags fts5 -race ./...`) and linter (`go vet -tags fts5 ./...`).
9. **Step 9:** Execute `graphify update .` to synchronize knowledge graph with all Phase 7 documentation and tests.

---

## 10. Test Plan (Validation)

*The following exact automated test commands and test cases validate this phase:*

### 10.1 Contract & Doc Consistency Tests
```bash
go test -tags fts5 -v -run "TestContract" ./cmd/centmem/...
```
- [x] `TestContract_CommandsMatchDocs`: Verifies all registered commands match `docs/cli-contract.md` and `skill/SKILL.md`.
- [x] `TestContract_ExitCodesMatchDocs`: Verifies exit code constants match specification (0, 1, 2, 3).
- [x] `TestContract_JSONFieldsStable`: Verifies stable JSON output fields across golden fixtures.

### 10.2 Documentation Link & Example Integrity Tests
```bash
go test -tags fts5 -v -run "TestDocs" ./cmd/centmem/...
```
- [x] `TestDocs_InternalMarkdownLinks`: Verifies all relative links across `README.md`, `docs/README.md`, `getting-started.md`, `capture-hooks.md`, and `SKILL.md` resolve to valid files.
- [x] `TestDocs_CLIExampleFlags`: Verifies that CLI commands and flags referenced in documentation examples exist in the binary registry.

### 10.3 Adapter Doc Contract Tests
```bash
go test -tags fts5 -v -run "TestHookAdapters_FlagsMatchCLI" ./cmd/centmem/...
```
- [x] `TestHookAdapters_FlagsMatchCLI`: Verifies all CLI flag invocations across all 7 hook adapter files (`skill/adapters/hooks/*.md`) are valid.

### 10.4 Full Repository Race & Regression Suite
```bash
go test -tags fts5 -race ./...
```
- [x] 100% of unit, integration, golden, and E2E tests pass across all packages (`internal/capture`, `internal/config`, `internal/store`, `internal/search`, `internal/embed`, `internal/compact`, `cmd/centmem`).
- [x] Zero race conditions detected.

### 10.5 Shell Hook Installer Test Suite
```bash
bash skill/adapters/hooks/install_test.sh
```
- [x] Auto-detection, specific harness target, idempotency, dry-run, uninstall, and daemon generation test cases pass.

### 10.6 NPM Package Test Suite
```bash
cd npm && npm test && cd ..
```
- [x] Installer argument parsing, idempotent workflow injection, and multi-target installations pass.

---

## 11. Checklist (Definition of Done)

- [x] `docs/guides/getting-started.md` updated with Auto-capture section and verification commands.
- [x] `docs/guides/capture-hooks.md` created with complete end-to-end guidance across all 7 harnesses, 3 interfaces, 3 backends, and prompt customization.
- [x] `skill/SKILL.md` updated with full command table parity, auto-capture recipes, and provenance details.
- [x] `README.md` updated with `v1.3.0` badge, feature row, CLI cheatsheet, and roadmap completion check.
- [x] `docs/README.md` updated with new guides, milestone statuses, and recommended reading order.
- [x] `CHANGELOG.md` updated with complete `v1.3.0` release notes adhering to Keep a Changelog.
- [x] Automated doc link integrity test (`doc_links_test.go`) and CLI example flag test (`doc_examples_test.go`) implemented and passing.
- [x] `TestContract_CommandsMatchDocs` passes with 0 errors.
- [x] `go test -tags fts5 -race ./...` passes 100% across the entire repository.
- [x] `go vet -tags fts5 ./...` reports 0 issues.
- [x] `graphify update .` executed and knowledge graph synchronized.
