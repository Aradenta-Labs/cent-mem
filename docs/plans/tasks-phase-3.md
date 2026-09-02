# Phase 3 — Config UX (Three Interfaces) Task List

> **Source:** `docs/plan-v1.3.0.md` (§ Phase 3 & § Phase 5 Init Extensions), `docs/architecture.md`, `docs/cli-contract.md`, `docs/data-model.md`  
> **Target Version:** `v1.3.0`  
> **Status:** COMPLETE  
> **Goal:** Provide a seamless, multi-modal configuration experience where users can configure capture via CLI commands (`centmem config set/get`), an interactive wizard during `centmem init`, or direct `config.toml` editing—all reading and writing to the identical single-source-of-truth configuration file.

---

## 1. Prerequisites & Dependencies

- [x] **1.1** Verify Phase 1 (`internal/capture` engine) and Phase 2 (`cmd/centmem/handlers_capture.go`) primitives are functional.
- [x] **1.2** Review `internal/config/config.go` and `internal/config/capture.go` data models to ensure all fields have corresponding TOML tags, JSON tags, and default values.
- [x] **1.3** Confirm TOML parser strategy (`github.com/pelletier/go-toml/v2`) that preserves file integrity and supports standard table sections (`[model]`, `[retention]`, `[capture]`).

---

## 2. Track A — TOML Persistence Engine (`internal/config/`)

*Implement robust read, write, validation, and schema generation for `~/.centmem/config.toml`.*

- [x] **A.1 TOML Read/Write Core (`internal/config/toml.go`)**
  - Implement `LoadTOML(path string) (Config, error)` to parse `config.toml` into `Config` struct.
  - Implement `SaveTOML(path string, cfg Config) error` and `SaveToHome(cfg Config) error` to serialize `Config` to formatted TOML.
  - Implement atomic write pattern: write to `.tmp` file in the target directory and `os.Rename` to target path with `0600` permissions.
- [x] **A.2 Configuration Layering & Precedence**
  - Ensure resolution order strictly follows:
    1. Hardcoded Defaults (`DefaultRetention()`, `DefaultCaptureConfig()`, `ModelConfig`)
    2. `~/.centmem/config.toml` file (if present)
    3. Environment variables (`CENTMEM_*`, `CENTMEM_CAPTURE_*`, `CENTMEM_RETENTION_*`)
    4. Runtime CLI flags (`--home`, `--db`, etc.)
- [x] **A.3 Schema Documentation Header Generator**
  - When creating or formatting `config.toml`, generate top-level comments documenting all sections:
    - `[model]`: model name, path, dimension
    - `[retention]`: fact, note, log, archive retention days
    - `[capture]`: enabled, harness, triggers, scope, categories, backend, endpoints, credentials env var name, confidence threshold
    - Explicit note: *API keys must never be stored directly in `config.toml`; use `api_key_env` to reference environment variable names.*
- [x] **A.4 Key Reflection & Getter/Setter Helper (`internal/config/keys.go`)**
  - Implement `GetConfigValue(cfg Config, dotKey string) (any, error)` supporting all dot-notation paths:
    - `capture.enabled`, `capture.harness`, `capture.triggers`, `capture.scope`, `capture.categories`, `capture.transcript_path`
    - `capture.backend`, `capture.local_llm_endpoint`, `capture.local_llm_model`
    - `capture.api_base_url`, `capture.api_key_env`, `capture.api_model`, `capture.confidence_threshold`
    - `model.name`, `model.dims`
    - `retention.fact_keep_days`, `retention.note_summarize_after_days`, `retention.log_summarize_after_days`, `retention.log_drop_after_days`, `retention.archive_keep_days`
  - Implement `SetConfigValue(cfg *Config, dotKey string, rawVal string) error`:
    - Parse raw string into appropriate target type (bool, int, float64, string, `[]string` from comma-separated input).
    - Validate updated struct with `validateRetention()` and `ValidateCaptureConfig()`.
    - Return actionable errors for unknown keys or invalid values.

---

## 3. Track B — Interface 1: Config CLI (`cmd/centmem/handlers_config.go`)

*Expose `centmem config set` and `centmem config get` commands for programmatic and scriptable configuration.*

- [x] **B.1 Command Registration (`cmd/centmem/commands.go` & `main.go`)**
  - Register `"config"` in `commands` map:
    - Usage: `config <get|set> [key] [value]`
    - Flags: standard global flags (`--home`, `--db`, `--pretty`, `--verbose`, `--quiet`).
  - Update `printUsage()` in `cmd/centmem/main.go` to include `config`.
- [x] **B.2 Config Command Router (`cmd/centmem/handlers_config.go`)**
  - Implement `cmdConfig(args []string) int` router:
    - Subcommands: `get`, `set`, and `-h`/`--help`.
    - Return `cli.Invalidf` for missing or unrecognized subcommands.
- [x] **B.3 `cmdConfigSet` Implementation**
  - Signature: `centmem config set <key> <value>`
  - Load active `config.toml` (or default if file does not exist).
  - Apply `SetConfigValue(&cfg, key, value)`.
  - Save back to `~/.centmem/config.toml` atomically.
  - Return JSON to stdout on success:
    ```json
    {
      "ok": true,
      "key": "capture.harness",
      "value": "claude-code"
    }
    ```
  - Emit error JSON to stderr on failure (exit 1) with descriptive hint.
- [x] **B.4 `cmdConfigGet` Implementation**
  - Signature: `centmem config get [key]`
  - If `<key>` is provided:
    - Fetch value via `GetConfigValue(cfg, key)`.
    - Return JSON: `{"ok": true, "key": "capture.harness", "value": "claude-code"}`.
    - If key does not exist: return exit code 2 (`NOT_FOUND`) with `{"error": {"code": "NOT_FOUND", "message": "unknown config key '...'", "hint": "run 'centmem config get' to view all keys"}}`.
  - If no `<key>` is provided:
    - Return full configuration object: `{"ok": true, "config": { ... }}`.

---

## 4. Track C — Interface 2: Interactive Wizard (`centmem init` Capture Extension)

*Extend `centmem init` with an interactive, terminal-friendly onboarding wizard for transcript auto-capture.*

- [x] **C.1 Wizard Detection & TTY Check (`cmd/centmem/handlers.go` / `init_wizard.go`)**
  - After database initialization and model verification in `cmdInit()`:
    - Check if `[capture]` table is already configured in `config.toml`.
    - Check if `--non-interactive` flag is passed or stdin is not a TTY (bypass wizard in CI/scripts).
    - If absent and interactive (or `--wizard` flag explicitly passed), launch wizard.
- [x] **C.2 Interactive Prompt Flow (`internal/cli/wizard.go` or `cmd/centmem/init_wizard.go`)**
  - Design modular prompt prompter with custom `io.Reader` and `io.Writer` for testability.
  - Step 1 — **Harness Selection:**
    - Prompt: *"Which AI agent do you use? [antigravity / trae / claude-code / cursor / codex / deepseek / hermes] (default: antigravity):"*
    - Validation: Accept case-insensitive recognized harness names.
  - Step 2 — **Triggers Selection:**
    - Prompt: *"Which triggers to enable? [message / session-end / on-demand / all] (default: all):"*
    - Mapping: `"all"` expands to `["message", "session-end", "on-demand"]`.
  - Step 3 — **Scope Configuration:**
    - Prompt: *"Default scope for captured memories? [e.g. project:myapp] (default: global):"*
    - Default to `"global"` if empty.
  - Step 4 — **Categories Configuration:**
    - Prompt: *"Capture categories (comma-separated, or press Enter for defaults: decision,fact,preference,code,log,error,dependency):"*
    - Parse comma-separated items; fallback to 7 defaults if empty.
  - Step 5 — **Classifier Backend Selection:**
    - Prompt: *"Which classifier backend? [local-llm / heuristic / openai-compatible] (default: heuristic):"*
    - Step 5a (if `local-llm`):
      - Prompt: *"Local LLM endpoint? [default: http://localhost:11434/v1]:"*
      - Prompt: *"Model name? [default: llama3.2]:"*
    - Step 5b (if `openai-compatible`):
      - Prompt: *"API base URL? [default: https://api.openai.com/v1]:"*
      - Prompt: *"Which env var holds your API key? [default: OPENAI_API_KEY]:"*
      - Prompt: *"Model? [default: gpt-4o-mini]:"*
  - Step 6 — **Confidence Threshold:**
    - Prompt: *"Confidence threshold (0.0–1.0)? [default: 0.7]:"*
    - Validation: Must be float between 0.0 and 1.0.
- [x] **C.3 Wizard Persistence & Prompt Template Creation**
  - Set `capture.enabled = true`.
  - Write updated configuration to `~/.centmem/config.toml`.
  - Generate default `~/.centmem/capture-prompt.md` containing the memory classification prompt template with `{{CATEGORIES}}` and `{{CONFIDENCE_THRESHOLD}}` interpolation variables.
  - Emit completion notice on stdout with summary of configuration.

---

## 5. Track D — Interface 3: Direct `config.toml` Editing & Subsystem Sync

*Ensure external manual edits to `config.toml` work smoothly and remain in sync with runtime commands.*

- [x] **D.1 Subcommand Synchronization**
  - Ensure `centmem capture categories --add/--remove` modifies the `[capture]` table inside `config.toml` directly (retiring standalone `capture-categories.json` or keeping it as fallback).
  - Ensure `centmem capture run` reads all parameters dynamically from `config.toml` unless overridden by CLI flags.
- [x] **D.2 Resilient Parsing & Error Handling**
  - Gracefully handle comments, formatting whitespace, and optional keys in manually edited `config.toml` files.
  - Provide human-readable syntax error messages on corrupted TOML files with line numbers and corrective guidance.

---

## 6. Implementation Order

1. **Step 1:** Implement TOML serialization/deserialization, atomic writer, and key getters/setters in `internal/config/`.
2. **Step 2:** Write comprehensive unit tests for `internal/config/` TOML and key validation.
3. **Step 3:** Implement `cmd/centmem/handlers_config.go` (`centmem config set` and `centmem config get`).
4. **Step 4:** Register `config` in `cmd/centmem/commands.go` and `main.go`.
5. **Step 5:** Implement interactive wizard engine and prompt writer for `centmem init`.
6. **Step 6:** Implement `capture-prompt.md` template writer during init.
7. **Step 7:** Wire `capture categories` command to persist directly to `config.toml`.
8. **Step 8:** Execute full validation test suite (Unit, Integration, CLI Golden, and E2E tests).

---

## 7. Test Plan (Validation)

*All tests below pass cleanly with `go test -tags fts5 -race ./...`.*

### 7.1 Unit Tests

#### `internal/config/toml_test.go`
- [x] `TestConfig_TOML_MarshalUnmarshal`: Verify round-trip conversion of full `Config` struct (including `[model]`, `[retention]`, `[capture]`).
- [x] `TestConfig_TOML_DefaultsFallback`: Verify that an empty or partial TOML file loads defaults for missing keys.
- [x] `TestConfig_TOML_HeaderAndComments`: Verify generated TOML contains documentation comments and warning about API keys.
- [x] `TestConfig_TOML_AtomicWrite`: Verify file is written atomically with `0600` permissions and survives simulated write interruptions.
- [x] `TestConfig_TOML_EnvPrecedence`: Verify environment variables (`CENTMEM_CAPTURE_HARNESS`, etc.) override values in `config.toml`.

#### `internal/config/keys_test.go`
- [x] `TestConfig_GetConfigValue_AllKeys`: Test fetching every valid dot-notation key.
- [x] `TestConfig_GetConfigValue_UnknownKey`: Test error returned when querying non-existent key.
- [x] `TestConfig_SetConfigValue_Types`:
  - Boolean parsing (`"true"`, `"false"`, `"1"`, `"0"`).
  - Float parsing (confidence threshold: valid `0.85`, invalid `-0.1`, invalid `1.5`).
  - Integer parsing (retention days: valid `30`, invalid `-5`).
  - String slices (categories, triggers: comma-separated `"decision,code"`).
- [x] `TestConfig_SetConfigValue_BackendValidation`: Test accepted backend values (`local-llm`, `heuristic`, `openai-compatible`) and rejection of invalid backends.

### 7.2 CLI & Handlers Unit Tests

#### `cmd/centmem/handlers_config_test.go`
- [x] `TestCLI_ConfigSet_ValidKey`: Run `centmem config set capture.harness claude-code` and verify JSON response `{"ok": true, ...}` and updated `config.toml`.
- [x] `TestCLI_ConfigSet_Categories`: Run `centmem config set capture.categories "decision,code,error"` and verify array structure.
- [x] `TestCLI_ConfigSet_InvalidKey`: Run `centmem config set capture.unknown_key foo` -> verify exit code 1 and error JSON on stderr.
- [x] `TestCLI_ConfigSet_InvalidValue`: Run `centmem config set capture.confidence_threshold 2.5` -> verify exit code 1 and validation error.
- [x] `TestCLI_ConfigGet_SingleKey`: Run `centmem config get capture.scope` -> verify JSON `{"ok": true, "key": "capture.scope", "value": "..."}`.
- [x] `TestCLI_ConfigGet_All`: Run `centmem config get` with no args -> verify full JSON output containing `model`, `retention`, and `capture` blocks.
- [x] `TestCLI_ConfigGet_NotFound`: Run `centmem config get nonexistent.key` -> verify exit code 2 and `NOT_FOUND` error code.

### 7.3 Interactive Wizard Tests

#### `cmd/centmem/init_wizard_test.go`
- [x] `TestInit_Wizard_InteractiveComplete`:
  - Feed mock stdin with responses for all 6 questions (harness, triggers, scope, categories, backend, confidence).
  - Verify `centmem init` creates `config.toml` containing exact responses.
  - Verify `~/.centmem/capture-prompt.md` is created with proper template content.
- [x] `TestInit_Wizard_DefaultsOnEnter`:
  - Feed mock stdin with empty newlines `\n\n\n\n\n\n`.
  - Verify `config.toml` is written with all default values.
- [x] `TestInit_Wizard_Backend_LocalLLM`:
  - Feed mock stdin selecting `local-llm`, endpoint `http://127.0.0.1:11434/v1`, model `llama3.2`.
  - Verify `config.toml` contains `[capture]` table with `backend = "local-llm"` and local LLM fields populated.
- [x] `TestInit_Wizard_Backend_OpenAICompatible`:
  - Feed mock stdin selecting `openai-compatible`, endpoint `https://api.openai.com/v1`, env `MY_API_KEY`, model `gpt-4o-mini`.
  - Verify `config.toml` contains `api_key_env = "MY_API_KEY"` and NO raw secrets.
- [x] `TestInit_Wizard_NonInteractiveBypass`:
  - Pass `--non-interactive` flag or simulate non-TTY.
  - Verify `centmem init` completes without prompting or hanging.

### 7.4 End-to-End & Golden Contract Tests

#### `cmd/centmem/e2e_config_test.go`
- [x] `TestE2E_ConfigThreeInterfacesRoundTrip`:
  1. Initialize with `centmem init` using wizard defaults.
  2. Modify harness via `centmem config set capture.harness cursor`.
  3. Verify via `centmem config get capture.harness`.
  4. Directly edit `config.toml` to change scope to `project:testapp`.
  5. Run `centmem config get capture.scope` and verify updated value.
  6. Run `centmem capture run` and verify it uses the updated scope and harness.

#### Golden Tests (`cmd/centmem/golden_test.go`)
- [x] `TestGolden_ConfigGet_Key`: Match `centmem config get capture.backend` against `testdata/golden/config_get_key.golden.json`.
- [x] `TestGolden_ConfigGet_All`: Match `centmem config get` against `testdata/golden/config_get_all.golden.json`.
- [x] `TestGolden_ConfigSet`: Match `centmem config set capture.scope project:demo` against `testdata/golden/config_set.golden.json`.

---

## 8. Checklist (Definition of Done)

- [x] `internal/config` provides full TOML read/write support with schema documentation headers.
- [x] `centmem config set <key> <value>` works for all supported keys with strict type validation.
- [x] `centmem config get [key]` outputs valid JSON (single key or full dump) and returns exit 2 on unknown key.
- [x] `centmem init` includes interactive prompt wizard covering harness, triggers, scope, categories, backend, and confidence.
- [x] `~/.centmem/capture-prompt.md` template is generated at initialization.
- [x] Direct manual edits to `~/.centmem/config.toml` are immediately reflected across all CLI commands.
- [x] All unit, integration, wizard, e2e, and golden tests pass.
- [x] `go vet -tags fts5 ./...` and `go test -tags fts5 -race ./...` pass with 0 errors.
- [x] `graphify update .` is executed after code updates.
