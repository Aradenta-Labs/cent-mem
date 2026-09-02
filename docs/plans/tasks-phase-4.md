# Phase 4 — Per-Harness Hook Adapters Task List

> **Source:** `docs/plan-v1.3.0.md` (§ Phase 4 — Per-Harness Hook Adapters), `docs/architecture.md`, `docs/cli-contract.md`, `docs/data-model.md`  
> **Target Version:** `v1.3.0`  
> **Status:** COMPLETE  
> **Goal:** Wire cent-mem's capture engine into the native execution lifecycle and transcript delivery mechanisms of all 7 supported AI agent harnesses (Antigravity, Trae, Claude Code, Cursor, Codex, Deepseek, Hermes), supporting real-time incremental per-message watching, clean session-end exit traps, and zero-touch auto-installation.

---

## 1. Prerequisites & Dependencies

- [x] **1.1** Verify Phase 1 (`internal/capture` engine: reader, classifier, dedup, writer, summary, session) is fully tested and functional.
- [x] **1.2** Verify Phase 2 (`cmd/centmem/handlers_capture.go`: `run`, `summary`, `categories`, `convert`) CLI commands operate reliably.
- [x] **1.3** Verify Phase 3 (`internal/config` TOML persistence & `centmem config set/get` / `init` wizard) provides complete `CaptureConfig` access.
- [x] **1.4** Confirm test fixture availability for transcripts across all 7 supported harnesses in `testdata/transcripts/`.

---

## 2. Track A — Per-Harness Hook Specifications (`skill/adapters/hooks/`)

*Author clear, complete, and reproducible adapter specifications and configuration guides for all 7 target agent harnesses in `skill/adapters/hooks/`.*

- [x] **A.1 Antigravity Adapter (`skill/adapters/hooks/antigravity.md`)**
  - **Harness Details:** Google Antigravity IDE / CLI runtime.
  - **Transcript Source:** `~/.gemini/antigravity/brain/<conversation-id>/.system_generated/logs/transcript.jsonl`.
  - **Capture Mechanism:**
    - Per-message trigger: Real-time file watcher on `transcript.jsonl` using `centmem capture run --watch --transcript <path> --harness antigravity`.
    - Session-end trigger: Hook into conversation completion or session checkpointing.
  - **Environment & Scope:** Defaults to active project directory name or `project:<repo-name>`; respects `CENTMEM_SCOPE` and `CENTMEM_AGENT`.
  - **Documentation:** Document path resolution, subagent transcript paths, and background watcher setup.

- [x] **A.2 Trae Adapter (`skill/adapters/hooks/trae.md`)**
  - **Harness Details:** Trae IDE agent environment.
  - **Transcript Source:** `~/.trae/logs/sessions/<session-id>/session.log` or project workspace `.trae/logs/`.
  - **Capture Mechanism:**
    - Session-end signal: File monitor polling or post-session execution trigger.
    - Transcript normalization: Built-in parser for Trae's JSON session format.
  - **Documentation:** Step-by-step setup in Trae settings, hook scripts, and automatic extraction workflow.

- [x] **A.3 Claude Code Adapter (`skill/adapters/hooks/claude-code.md`)**
  - **Harness Details:** Anthropic Claude Code CLI.
  - **Transcript Source:** `.claude/` session directories and shell exit lifecycle.
  - **Capture Mechanism:**
    - Session-end trap: Shell `EXIT` trap injected via `.claude/centmem.env` or shell profile:
      ```bash
      trap 'centmem capture run --transcript "$CLAUDE_TRANSCRIPT_PATH" --scope "project:${CENTMEM_PROJ:-global}" && centmem capture summary' EXIT
      ```
    - Hook config: `.claude/settings.json` lifecycle hook integration if supported.
  - **Documentation:** Configuration recipes, environment variable propagation, and interactive session-end summary reporting.

- [x] **A.4 Cursor Adapter (`skill/adapters/hooks/cursor.md`)**
  - **Harness Details:** Cursor IDE composer and chat sessions.
  - **Transcript Source:** `.cursor/rules/`, `.cursorrules`, and Cursor conversation export logs.
  - **Capture Mechanism:**
    - Post-command injection via `.cursorrules` / composer rules directing the agent to trigger `centmem capture run` on completion.
    - Direct export watcher: Ingest `.cursor/logs/` conversation files.
  - **Documentation:** `.cursorrules` snippet, setup commands, and manual on-demand workflow.

- [x] **A.5 Codex Adapter (`skill/adapters/hooks/codex.md`)**
  - **Harness Details:** OpenAI Codex CLI and custom script harnesses.
  - **Transcript Source:** Stdin/stdout interactive stream or execution logs.
  - **Capture Mechanism:**
    - Thin shell wrapper/shim (`codex-centmem-shim`) intercepting session I/O:
      ```bash
      codex "$@" | tee >(centmem capture run --harness codex)
      ```
    - Pipe-based ingestion: `centmem capture run --harness codex` directly from pipe.
  - **Documentation:** Shim installation instructions, wrapper aliases, and streaming capture rules.

- [x] **A.6 Deepseek Adapter (`skill/adapters/hooks/deepseek.md`)**
  - **Harness Details:** Deepseek CLI and local agent environments.
  - **Transcript Source:** Local session export directory (`~/.deepseek/sessions/`).
  - **Capture Mechanism:**
    - File watcher monitoring session export directory for newly closed session files.
    - On-demand `centmem capture run --transcript <file> --harness deepseek`.
  - **Documentation:** Directory structure mapping, export trigger setup, and automated summary verification.

- [x] **A.7 Hermes Adapter (`skill/adapters/hooks/hermes.md`)**
  - **Harness Details:** Hermes agent framework.
  - **Transcript Source:** Hermes event bus / plugin hook API (`on_message_end`, `on_session_end`).
  - **Capture Mechanism:**
    - Native Hermes hook plugin invoking `centmem capture run` or piping event payload.
    - Lifecycle callback handlers registered during Hermes startup.
  - **Documentation:** Plugin registration file, configuration block in Hermes config, and hook error tolerance.

---

## 3. Track B — Unified Hook Installer (`skill/adapters/hooks/install.sh`)

*Provide an automated, idempotent, multi-platform installer script for registering hooks across any or all detected agent harnesses.*

- [x] **B.1 Harness Auto-Detection Engine**
  - Detect installed agent environments by checking:
    - Binaries on `PATH` (`claude`, `cursor`, `trae`, `codex`, `hermes`, `antigravity`, `deepseek`).
    - Config directories (`~/.claude/`, `~/.cursor/`, `~/.trae/`, `~/.gemini/antigravity/`, `~/.codex/`, `~/.hermes/`, `~/.deepseek/`).
  - Display clear summary of detected harnesses vs. uninstalled targets.

- [x] **B.2 CLI Flag & Mode Support**
  - Flags:
    - `--harness <name>`: Target a specific harness (e.g. `antigravity`, `claude-code`, `cursor`).
    - `--all`: Install hooks for all detected harnesses (default behavior if no `--harness` specified).
    - `--trigger <triggers>`: Comma-separated list (`message`, `session-end`, `on-demand`, `all`).
    - `--scope <scope>`: Custom target scope for auto-captured memories (default: `project:<name>` or `global`).
    - `--list`: List detected harnesses, hook target files, and proposed actions without making changes.
    - `--dry-run`: Output all file writes, symlinks, and configuration edits without modifying disk.
    - `--uninstall`: Safely remove installed hooks, shims, traps, and background watchers.
    - `-h`, `--help`: Comprehensive help and usage examples.

- [x] **B.3 Idempotent Hook Registration & File Management**
  - Manage shell trap injection in `.claude/centmem.env` and user profile files using bounded marker blocks (`# BEGIN CENTMEM HOOK` ... `# END CENTMEM HOOK`).
  - Create directory trees with proper permissions (`0755` for scripts/directories, `0644` for markdown/configs).
  - Ensure re-running `install.sh` never creates duplicated entries or syntax errors.
  - Ensure `--uninstall` completely removes marker blocks, shims, and plist/systemd unit files without touching unrelated user configurations.

- [x] **B.4 Background Watcher Daemon / Service Setup**
  - For per-message file watching on file-based harnesses (Antigravity, Trae, Deepseek):
    - **macOS:** Generate and install `launchd` agent plist (`~/Library/LaunchAgents/com.aradenta.centmem.capture-watcher.plist`).
    - **Linux:** Generate and install `systemd` user service unit (`~/.config/systemd/user/centmem-capture-watcher.service`).
    - Support `--no-daemon` flag for environments preferring direct CLI or container-based execution.

---

## 4. Track C — Real-Time Incremental Watcher Engine (`internal/capture/watcher.go`)

*Build a robust, low-overhead incremental file watcher engine capable of tracking active transcript files in real-time.*

- [x] **C.1 Watcher Core Architecture & Data Types**
  - Define `WatcherConfig`:
    - `TranscriptPath string`: Target file to monitor.
    - `Harness string`: Agent harness format parser.
    - `PollInterval time.Duration`: Polling interval for fallback (default: `250ms`).
    - `DebounceDuration time.Duration`: Quiet period before batch classification (default: `100ms`).
    - `Scope string`: Memory target scope.
  - Define `WatcherState`:
    - `FilePath string`: Absolute path to transcript.
    - `LastByteOffset int64`: Byte position up to which the file was parsed.
    - `LastLineNumber int`: Line count processed.
    - `LastModified time.Time`: File timestamp.
    - `SessionID string`: Active capture session ID.

- [x] **C.2 Incremental Read & State Tracking**
  - Implement `StateStore`: persist `WatcherState` to `~/.centmem/watcher-<hash>.json` across restarts.
  - On file modification:
    - Detect file rotation or truncation (file size < `LastByteOffset` -> reset offset with warning log).
    - Open file and seek to `LastByteOffset`.
    - Read only newly appended bytes/lines.
    - Parse new lines into `[]TranscriptMessage` using harness-specific parser.
    - Update and persist `LastByteOffset` and `LastLineNumber` after successful read.

- [x] **C.3 Debouncing & Pipeline Execution**
  - Implement debouncing loop: collect messages over quiet window before triggering classification pass.
  - Pipeline flow per debounced chunk:
    1. Read new `[]TranscriptMessage`.
    2. Run `classifier.Classify(messages, categories)`.
    3. Run `dedup.IsDuplicate()` and filter out existing memories.
    4. Write novel items to store via `writer.Write()`.
    5. Update in-memory session summary statistics.
  - Context cancellation & signal handling: Ensure `SIGINT`/`SIGTERM` gracefully stops the watcher, flushes pending classification, updates summary, and releases locks.

---

## 5. Track D — CLI Integration for `--watch` (`cmd/centmem/handlers_capture.go`)

*Integrate the watcher engine into `centmem capture run` and provide watcher process management.*

- [x] **D.1 CLI Flags for `centmem capture run`**
  - Add `--watch` boolean flag: activates real-time incremental watching mode.
  - Add `--once` boolean flag: single-pass incremental poll.
  - Add `--interval` duration flag: poll/debounce interval override (e.g. `250ms`).
  - Add `--state-file` path flag: optional custom watcher state file.
  - Validation: If `--watch` is specified without `--transcript`, check `CaptureConfig.TranscriptPath` or error cleanly.

- [x] **D.2 Watcher Process & Session Lifecycle**
  - When `--watch` is invoked:
    - Acquire session lock via `capture.StartSession()`.
    - Print startup banner in verbose mode or clean JSON startup status.
    - Run watcher event loop blocking on signal context.
    - On termination: call `capture.EndSession()`, output final summary, and exit 0.

- [x] **D.3 Dedicated Subcommand `centmem capture watch` (Optional / Helper)**
  - Implement `centmem capture watch status`: inspect running watcher PID and state.
  - Implement `centmem capture watch stop`: send graceful termination signal to active watcher PID.

---

## 6. Track E — Multi-Harness Transcript Normalization (`internal/capture/convert.go`)

*Ensure transcript parsing and normalization (`centmem capture convert`) reliably handles the idiosyncrasies of all 7 harness formats.*

- [x] **E.1 Parser Implementations per Harness**
  - `parseAntigravityTranscript`: Parse JSONL records (`USER_INPUT`, `PLANNER_RESPONSE`, `SUBAGENT_RESPONSE`, timestamp, role).
  - `parseClaudeCodeTranscript`: Parse Claude transcript JSON/text stream (`user`, `assistant`, `tool_use`, timestamps).
  - `parseCursorTranscript`: Parse Cursor chat export and prompt logs.
  - `parseTraeTranscript`: Parse Trae structured session logs.
  - `parseCodexTranscript`: Parse Codex CLI streaming output and markdown block structure.
  - `parseDeepseekTranscript`: Parse Deepseek session exports.
  - `parseHermesTranscript`: Parse Hermes event JSON arrays.

- [x] **E.2 Normalization Pipeline & Output Validation**
  - Map every harness payload into standard `TranscriptMessage{Role, Content, Timestamp}`.
  - Sanitize content: strip ANSI escape codes, terminal formatting, and unprintable control characters.
  - In `centmem capture convert --harness <name> --input <in> [--output <out>]`:
    - Output clean `.jsonl` containing normalized `{role, content, timestamp}`.
    - Return JSON result: `{"ok": true, "output": "...", "messages": N, "harness": "..."}`.

---

## 7. Implementation Order

1. **Step 1:** Create `skill/adapters/hooks/` directory and write adapter markdown specifications for all 7 harnesses (`antigravity.md`, `trae.md`, `claude-code.md`, `cursor.md`, `codex.md`, `deepseek.md`, `hermes.md`).
2. **Step 2:** Implement multi-harness transcript normalization parsers in `internal/capture/convert.go` and `reader.go`.
3. **Step 3:** Implement incremental file watcher and state persistence engine in `internal/capture/watcher.go`.
4. **Step 4:** Wire `--watch` flag and streaming execution loop into `cmd/centmem/handlers_capture.go`.
5. **Step 5:** Create `skill/adapters/hooks/install.sh` supporting auto-detection, `--dry-run`, `--uninstall`, trap injection, and daemon generation.
6. **Step 6:** Implement `skill/adapters/hooks/install_test.sh` to test the shell installer across temp directories and simulated environments.
7. **Step 7:** Implement Go unit tests, CLI tests, adapter syntax checkers, and end-to-end watcher tests.
8. **Step 8:** Run `go test -tags fts5 -race ./...`, verify code quality, and run `graphify update .`.

---

## 8. Test Plan (Validation)

*All tests below must pass cleanly with `go test -tags fts5 -race ./...` and `bash skill/adapters/hooks/install_test.sh`.*

### 8.1 Unit Tests (`internal/capture/`)

#### `internal/capture/watcher_test.go`
- [x] `TestWatcher_InitialRead`: Verify watcher starts at beginning of file if no state exists, reads existing messages, and saves state offset.
- [x] `TestWatcher_IncrementalAppend`: Append 5 new lines to an existing transcript file; verify watcher picks up only the 5 new lines and updates byte offset.
- [x] `TestWatcher_FileTruncation`: Truncate file to smaller size; verify watcher detects truncation, resets offset to 0 with warning, and continues without panicking.
- [x] `TestWatcher_StatePersistence`: Stop watcher, restart with existing state file; verify it resumes exactly from `LastByteOffset` without re-reading processed lines.
- [x] `TestWatcher_Debounce`: Rapidly write 10 chunks within 50ms; verify debounce groups them into a single classification batch.
- [x] `TestWatcher_GracefulShutdown`: Cancel context during active watch; verify clean shutdown, state write, and no dangling file locks.

#### `internal/capture/convert_test.go`
- [x] `TestConvert_AllHarnesses`: Test `ConvertTranscript()` against sample fixture files for each of the 7 harnesses (`antigravity`, `claude-code`, `cursor`, `trae`, `codex`, `deepseek`, `hermes`) and verify accurate extraction of `role`, `content`, and `timestamp`.
- [x] `TestConvert_MalformedInput`: Feed corrupted JSON / text to converter; verify structured error returned with line number.

---

### 8.2 Shell Hook Installer Tests (`skill/adapters/hooks/install_test.sh`)

- [x] `TestInstall_AutoDetect_AllHarnesses`:
  - Create mock `$HOME` with simulated harness directories (`.claude/`, `.cursor/`, `.trae/`, `.gemini/antigravity/`, `.codex/`, `.hermes/`, `.deepseek/`).
  - Run `install.sh` and verify hook scripts, rules, and marker blocks are created in each target directory.
- [x] `TestInstall_SpecificHarness`:
  - Run `install.sh --harness claude-code`.
  - Verify only `.claude/` targets are modified, leaving other directories untouched.
- [x] `TestInstall_Idempotency`:
  - Run `install.sh` twice consecutively.
  - Verify zero duplicate marker blocks, zero syntax errors, and exit code 0.
- [x] `TestInstall_DryRun`:
  - Run `install.sh --dry-run`.
  - Assert zero disk modifications in `$HOME` (file count unchanged).
- [x] `TestInstall_Uninstall`:
  - Run `install.sh --uninstall`.
  - Verify all installed hook blocks, shims, and configs are removed, while preserving original user configuration files.
- [x] `TestInstall_DaemonGeneration`:
  - Verify macOS launchd plist syntax and Linux systemd unit syntax generated by installer.

---

### 8.3 CLI & Handler Tests (`cmd/centmem/`)

#### `cmd/centmem/handlers_capture_watch_test.go`
- [x] `TestCLI_CaptureRun_WatchMode`:
  - Launch `centmem capture run --watch --transcript <temp_file>` in test goroutine.
  - Append test messages to `<temp_file>`.
  - Send interrupt signal; verify process exits 0 and captures memories into test DB.
- [x] `TestCLI_CaptureRun_MissingTranscriptInWatch`:
  - Run `centmem capture run --watch` with no transcript configured or specified; verify exit code 1 with actionable error JSON.
- [x] `TestCLI_CaptureConvert_Command`:
  - Run `centmem capture convert --harness antigravity --input <in> --output <out>`.
  - Verify exit 0, JSON response shape, and valid `.jsonl` output file.

#### `cmd/centmem/hook_adapters_test.go` (Adapter Doc Contract Validation)
- [x] `TestHookAdapters_FlagsMatchCLI`:
  - Parse every `centmem` command invocation in `skill/adapters/hooks/*.md`.
  - Verify every subcommand and flag used (`--transcript`, `--scope`, `--harness`, `--watch`, etc.) exists in the CLI registry.
  - Fails build if adapter documentation drifts from actual CLI implementation.

---

### 8.4 End-to-End & Simulation Tests (`cmd/centmem/e2e_hooks_test.go`)

- [x] `TestE2E_ClaudeCode_SessionEndTrap`:
  - Simulate a complete Claude Code session:
    1. Source generated `.claude/centmem.env` in subshell.
    2. Execute simulated agent session writing decisions to transcript.
    3. Trigger `EXIT` trap.
    4. Assert memories are stored in centmem store under correct scope.
    5. Assert `centmem capture summary` returns captured count > 0.
- [x] `TestE2E_Antigravity_RealTimeWatch`:
  - Simulate active Antigravity session:
    1. Start `centmem capture run --watch --transcript <temp.jsonl>`.
    2. Concurrently append agent messages with architecture decisions.
    3. Verify memories appear in store via `centmem recall` while watcher is still running.
    4. Stop watcher and verify final summary file.
- [x] `TestE2E_Codex_StreamingPipe`:
  - Pipe simulated codex transcript stream directly to `centmem capture run --harness codex`.
  - Verify memories are extracted and provenance tags (`source-agent: capture-hook`, `source-session: ...`) are recorded.

---

## 9. Checklist (Definition of Done)

- [x] `skill/adapters/hooks/` contains complete adapter markdown docs for all 7 harnesses (`antigravity.md`, `trae.md`, `claude-code.md`, `cursor.md`, `codex.md`, `deepseek.md`, `hermes.md`).
- [x] `skill/adapters/hooks/install.sh` provides auto-detection, `--dry-run`, `--uninstall`, `--harness`, and `--trigger` support.
- [x] `internal/capture/watcher.go` implements incremental transcript reading with state persistence and debounce.
- [x] `centmem capture run --watch` is fully wired to the watcher engine with signal handling and graceful session shutdown.
- [x] `centmem capture convert` normalizes all 7 harness formats into standard JSONL.
- [x] Shell installer test suite `skill/adapters/hooks/install_test.sh` passes 100%.
- [x] Go unit, CLI, adapter recipe, and end-to-end hook tests pass with race detector enabled (`go test -tags fts5 -race ./...`).
- [x] `go vet -tags fts5 ./...` reports 0 issues.
- [x] `graphify update .` is run to keep the knowledge graph up to date.
