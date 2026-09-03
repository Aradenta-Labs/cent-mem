# Phase 6 (v1.4.1) — centmem Web UI Configuration & Settings

**Version:** 1.4.1 (target)  
**Owner:** Aradenta Labs  
**Mode:** Operate (dashboard settings) — see [impeccable operate reference]  
**Status:** Design direction locked; ready for phased implementation  

---

## 0. Executive Summary & Objective

In **v1.4.0**, `centmem ui` introduced an embedded, local-first Web UI Memory Browser dashboard for searching, inspecting, filtering, deleting, and exporting memories. However, all system configuration (retention periods, auto-capture triggers, LLM classification backends, and category whitelists) remains CLI-only or requires manually editing `~/.centmem/config.toml`.

**v1.4.1** adds a dedicated, high-density **Settings & Configuration** panel directly within the Web UI. Users can visually review, test, validate, and persist their `centmem` configuration with instant feedback, zero terminal roundtrips, and safe defaults.

**Key Design & Engineering Principles:**
- **impeccable (Operate Mode):** Task-first, high information density, calm typography, predictable layout, consistent affordances, complete component states (default, hover, focus, active, disabled, loading, dirty, saved).
- **antislop-ui:** Zero generic AI slop (no blue-purple gradients, no glassmorphism overload, no arbitrary pill radiuses, no halo glows, no decorative emojis), restrained calm teal accent (`#0f766e` in light, `#14b8a6` in dark), real data only, working keyboard shortcuts (`Cmd+,`).
- **Local-First & Safe:** All changes validate against existing dot-notation rules (`internal/config/keys.go`) and persist to `~/.centmem/config.toml` via atomic writes. Includes dirty state tracking and unsaved change confirmation.

---

## 1. Confirmed Requirements

| Decision | Specification | Rationale |
|---|---|---|
| **Primary Purpose** | Settings & System Configuration | Users are configuring policies and integration points for their memory store. |
| **Interface Format** | Centered Modal Dialog with Tabbed Sidebar Navigation | Escapes layout constraints, focuses attention on configuration, matches macOS/desktop app conventions (`Cmd+,`). |
| **Scope of Settings** | Model, Retention, Auto-Capture, Classifier, Categories | Covers all 19 configuration keys supported by `internal/config/keys.go`. |
| **Backend Integration** | Dedicated REST Endpoints (`GET /api/config`, `PATCH /api/config`, `POST /api/config/test-classifier`) | Native Go standard library HTTP routes on existing `127.0.0.1` embedded server. |
| **Persistence Target** | `~/.centmem/config.toml` via `config.SaveToHome()` | Keeps CLI and UI in 100% sync; edits directly update single source of truth. |
| **Connectivity Testing** | Live "Test Connection" probe for Local LLM / OpenAI backends | Verifies user endpoints and models before saving, preventing silent extraction failures. |
| **Shortcut** | `Cmd+,` (Mac) or `Ctrl+,` (Linux/Windows) | Universal standard shortcut for Settings. |

### Anti-Goals (Explicit)
- Not a multi-tenant cloud settings panel (cent-mem is strictly single-user, local-first).
- No arbitrary file system browsing (config paths are pinned to `CENTMEM_HOME`).
- No direct editing of raw database tables (retention and store integrity remain gated by Go store logic).

---

## 2. Design Direction: Impeccable (Operate Mode) & Antislop-UI

### 2.1 Visual Authority & Color Strategy
- **Mode:** **Operate**. Scanability, clarity, and form ergonomics outrank decoration.
- **Color Strategy:** **Restrained** neutral base (cool slate grays) + **one accent** (calm teal `--accent-primary: #0f766e` in light mode, `#14b8a6` in dark mode).
  - Success badges use soft green (`--color-success-bg: #f0fdf4`, `--color-success-text: #166534`).
  - Warning/Dirty state uses warm amber (`--color-warning-bg: #fffbeb`, `--color-warning-text: #92400e`).
  - Error states use crisp red (`--color-error-bg: #fef2f2`, `--color-error-text: #991b1b`).
- **Surfaces & Elevation:**
  - Dialog overlay uses a neutral backdrop tint (`rgba(15, 23, 42, 0.45)`) with minimal blur (`backdrop-filter: blur(2px)` capped per antislop R-10).
  - Dialog card sits flat on `--surface-primary` with a single deliberate elevation shadow (`box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1)`). No colored halo or zero-offset glowing border (antislop R-13, craft-floor).
- **Radius & Borders:**
  - Small, deliberate radius scale: inputs and buttons `6px`, modal container `8px`, badge tags `4px`. Zero pill buttons except status badges.

### 2.2 Modal Architecture & Layout (Two-Pane Desktop-First)
The Settings Modal uses a two-pane layout:
1. **Left Navigation Sidebar (180px)**:
   - Clean vertical tab list with icons and subtle active indicator.
   - Tabs:
     1. **General** (Model info, DB file path, permissions)
     2. **Retention** (Compaction thresholds, expiration days)
     3. **Auto-Capture** (Triggers, default scope, harness)
     4. **Classifier** (Backend selection, endpoint, confidence threshold, live test)
     5. **Categories** (Whitelisted capture categories tag manager)
2. **Right Content Panel (Flex 1, min-width 520px)**:
   - High-density form layout.
   - Each setting has:
     - Clear title and unit label (e.g. `Note Summarization (Days)`).
     - Informative secondary hint explaining the runtime effect.
     - Accessible input control (number stepper, select, switch, tag input).
3. **Sticky Bottom Action Bar**:
   - Left: "Reset to Defaults" button (with confirmation) and "Config file: `~/.centmem/config.toml`" path indicator.
   - Right: "Revert" button (active when dirty) and "Save Changes" primary button (with loading spinner and dirty indicator dot).

---

## 3. Configuration Surface & Key Mapping

Maps all 19 configuration keys defined in `internal/config/keys.go`:

| Tab | Key | Type | UI Control | Description |
|---|---|---|---|---|
| **General** | `model.name` | `string` | Read-only / Select | Pinned embedding model (e.g. `bge-small-en-v1.5`) |
| **General** | `model.dims` | `int` | Read-only | Embedding dimension vector size (`384`) |
| **Retention** | `retention.fact_keep_days` | `int` | Number Stepper | Days to retain facts before archiving (`0` = keep indefinitely) |
| **Retention** | `retention.note_summarize_after_days` | `int` | Number Stepper | Days before notes are consolidated into summaries (default `30`) |
| **Retention** | `retention.log_summarize_after_days` | `int` | Number Stepper | Days before chronological logs are consolidated (default `14`) |
| **Retention** | `retention.log_drop_after_days` | `int` | Number Stepper | Days before raw logs are pruned from active store (default `30`) |
| **Retention** | `retention.archive_keep_days` | `int` | Number Stepper | Retention window for archived summaries (default `365`) |
| **Auto-Capture** | `capture.enabled` | `bool` | Toggle Switch | Master switch for background and transcript capture |
| **Auto-Capture** | `capture.harness` | `string` | Select Dropdown | Agent harness (`claude-code`, `cursor`, `antigravity`, `trae`, `codex`, `generic`) |
| **Auto-Capture** | `capture.triggers` | `[]string` | Multi-Checkbox / Tag | Triggers: `session-end`, `per-message`, `on-demand` |
| **Auto-Capture** | `capture.scope` | `string` | Text Input | Default target memory scope (e.g. `project:my-project`) |
| **Auto-Capture** | `capture.transcript_path` | `string` | Text Input | Optional custom transcript path override |
| **Classifier** | `capture.backend` | `string` | Radio Segment | Engine: `heuristic`, `local-llm`, `openai-compatible` |
| **Classifier** | `capture.local_llm_endpoint` | `string` | URL Input | Local LLM URL (default `http://localhost:11434/v1`) |
| **Classifier** | `capture.local_llm_model` | `string` | Text Input | Model tag (default `llama3.2`, `qwen2.5-coder`, etc.) |
| **Classifier** | `capture.api_base_url` | `string` | URL Input | OpenAI-compatible endpoint URL |
| **Classifier** | `capture.api_key_env` | `string` | Text Input | Env var name containing API key (e.g. `OPENAI_API_KEY`) |
| **Classifier** | `capture.api_model` | `string` | Text Input | Remote model name (e.g. `gpt-4o-mini`) |
| **Classifier** | `capture.confidence_threshold` | `float64` | Range Slider + Input | Confidence cutoff (`0.0` to `1.0`, default `0.70`) |
| **Categories** | `capture.categories` | `[]string` | Tag Chip Editor | Active classification categories |

---

## 4. Technical Architecture

### 4.1 Backend (Go REST Endpoints in `internal/ui/server.go`)

1. **`GET /api/config`**:
   - Reads active `Config` via `config.LoadTOML()` from `CENTMEM_HOME`.
   - Returns structured JSON:
     ```json
     {
       "ok": true,
       "config": {
         "model": { "name": "bge-small-en-v1.5", "dims": 384 },
         "retention": { ... },
         "capture": { ... }
       },
       "meta": {
         "home": "/Users/user/.centmem",
         "config_path": "/Users/user/.centmem/config.toml",
         "is_writable": true
       }
     }
     ```
2. **`PATCH /api/config`**:
   - Accepts partial or full config JSON payload.
   - Flattens into dot-notation keys and validates using `config.SetConfigValue()`.
   - Persists to disk via `config.SaveToHome(cfg.Home, updatedConfig)`.
   - Returns updated configuration object or 400 Bad Request with field-level errors:
     ```json
     {
       "error": {
         "code": "INVALID_CONFIG",
         "message": "retention.note_summarize_after_days must be greater than 0",
         "field": "retention.note_summarize_after_days"
       }
     }
     ```
3. **`POST /api/config/test-classifier`**:
   - Accepts backend type, endpoint URL, model name, and optional auth header.
   - Dispatches a lightweight probe (5-second timeout) to verify connectivity:
     - For `local-llm`: sends a minimal ping to `/v1/models` or completion check.
     - For `openai-compatible`: verifies API key presence and endpoint reachability.
   - Returns:
     ```json
     {
       "ok": true,
       "status": "connected",
       "latency_ms": 38,
       "model": "llama3.2",
       "message": "Local LLM endpoint is healthy and model is loaded."
     }
     ```

### 4.2 Frontend (React + Vite + TypeScript)

- **Types (`ui/src/types/config.ts`)**:
  - `ConfigData`, `ConfigResponse`, `UpdateConfigPayload`, `TestClassifierResult`.
- **API Client (`ui/src/services/api.ts`)**:
  - `fetchConfig()`, `updateConfig(payload)`, `testClassifierEndpoint(params)`.
- **Components (`ui/src/components/settings/`)**:
  - `SettingsModal.tsx`: Main modal shell with tab switcher, dirty-state bar, and keyboard bindings.
  - `GeneralTab.tsx`: Model metadata, storage directory, permissions review.
  - `RetentionTab.tsx`: Interactive day steppers with summary timeline explanation.
  - `CaptureTab.tsx`: Master toggle, trigger multiselect, harness dropdown.
  - `ClassifierTab.tsx`: 3-tier backend selector, endpoint inputs, confidence slider, and "Test Connection" button.
  - `CategoriesTab.tsx`: Interactive tag pills, quick-add preset badges, deletion buttons.
- **TopBar Integration (`ui/src/components/TopBar.tsx`)**:
  - Add Settings button (gear icon) in the right actions cluster.
  - Shortcut listener: `Cmd+,` or `Ctrl+,`.
  - Update `KeyboardShortcutsModal.tsx` to list the Settings shortcut.

---

## 5. Phased Implementation Plan

### Phase 6.0: Backend Endpoints & Test Suite
- [x] Implement `GET /api/config` in `internal/ui/server.go`.
- [x] Implement `PATCH /api/config` with validation and atomic write to `~/.centmem/config.toml`.
- [x] Implement `POST /api/config/test-classifier` probe handler.
- [x] Add comprehensive test suite in `internal/ui/server_test.go`:
  - Test config fetch returns correct structure and defaults.
  - Test partial config update with validation errors on invalid values.
  - Test persistence to disk (verifying TOML content).
  - Test LLM connectivity probe with mocked HTTP responder.

### Phase 6.1: Frontend API Service & State Management
- [x] Create `ui/src/types/config.ts` with complete type definitions matching backend JSON shapes.
- [x] Add API methods to `ui/src/services/api.ts` (`fetchConfig`, `updateConfig`, `testClassifier`).
- [x] Build custom hook or state manager (`useConfig`) tracking:
  - Original server state vs current draft state.
  - `isDirty` flag and modified keys diff.
  - Loading, saving, testing, and error states.

### Phase 6.2: Settings Modal Shell & Navigation
- [x] Create `ui/src/components/settings/SettingsModal.tsx` using `<Dialog>` base component.
- [x] Implement vertical tab list with icons: General, Retention, Auto-Capture, Classifier, Categories.
- [x] Add sticky action footer: "Revert Changes", "Reset to Defaults", "Save Changes" (with dirty state dot).
- [x] Wire `TopBar.tsx` gear icon and `Cmd+,` shortcut.
- [x] Add unsaved changes prompt if user tries to close while `isDirty`.

### Phase 6.3: Tab Panels Implementation
- [x] **General Tab**:
  - Read-only embedding model display (`bge-small-en-v1.5`, 384 dims).
  - Storage location (`CENTMEM_HOME`), database size, and permissions check.
- [x] **Retention Tab**:
  - Steppers for `fact_keep_days`, `note_summarize_after_days`, `log_summarize_after_days`, `log_drop_after_days`, `archive_keep_days`.
  - Inline explanation for each threshold and visual lifecycle progression.
- [x] **Auto-Capture Tab**:
  - Master switch for `capture.enabled`.
  - Target harness select (`auto`, `claude-code`, `cursor`, `antigravity`, `trae`, `codex`, `generic`).
  - Checkbox group for triggers (`session-end`, `per-message`, `on-demand`).
  - Default capture scope text input with validation.
- [x] **Classifier Tab**:
  - Radio segment control for backend: `heuristic` vs `local-llm` vs `openai-compatible`.
  - Conditional input fields based on selected backend (Endpoint URL, Model name, API key env var).
  - Confidence threshold slider (`0.00` to `1.00`) with live numeric badge.
  - "Test Connection" button with status badge (Testing spinner, Success checkmark + latency, or Error message).
- [x] **Categories Tab**:
  - Interactive tag pills displaying current whitelist categories.
  - Input field to add custom category.
  - "Suggested Categories" quick-add chips (`security`, `bug`, `arch`, `api`, `dependency`).

### Phase 6.4: Design System Gallery & Antislop-UI Polish Pass
- [x] Update `ui/src/pages/DesignSystem.tsx` to include the new Settings components (stepper, switch, slider, tag input).
- [x] Run Impeccable Operate-mode audit: ensure all form inputs have proper focus rings, contrast checks (WCAG AA), and keyboard navigation (`Tab`, `Enter`, `Esc`).
- [x] Run Antislop-UI checklist: verify zero generic gradients, dose-capped shadows, and no decorative emojis.

### Phase 6.5: Build, Test & Release v1.4.1
- [x] Run full frontend test and build: `npm --prefix ui run build`.
- [x] Run Go vet, contract checks, and unit tests: `go test -tags fts5 -race ./...`.
- [x] Update `docs/ui.md` with Settings documentation, API endpoints, and screenshots/diagrams.
- [x] Update `CHANGELOG.md` with `[1.4.1]` release entry.
- [x] Bump version in `cmd/centmem/main.go`, `npm/package.json`, and skill files.
- [x] Commit, tag `v1.4.1`, and push.

---

## 6. Testing & Quality Gates

| Test Layer | Scope | Gate / Target |
|---|---|---|
| **Go Backend Unit Tests** | `internal/ui/server_test.go` | 100% test pass on `GET /api/config`, `PATCH /api/config`, validation failures, and disk write roundtrips. |
| **Frontend TypeScript Build** | `tsc --noEmit` & `npm run build` in `ui/` | Zero TypeScript errors, clean bundle compilation. |
| **Contract Stability** | `cmd/centmem/contract_test.go` | Validates that all REST endpoints match documented contract. |
| **Accessibility & Keyboard** | Chrome DevTools / manual audit | Full `Tab` traversal, `Cmd+,` opens dialog, `Esc` closes dialog, all controls accessible. |
| **Idempotency** | Settings Save | Saving unchanged settings does not alter file hash or break formatting in `config.toml`. |

---

## 7. Definition of Done (v1.4.1)

- [x] 1. Clicking the gear icon in the Web UI top bar or pressing `Cmd+,` opens the Settings modal.
- [x] 2. Users can modify any setting across Retention, Auto-Capture, Classifier, and Categories.
- [x] 3. Clicking "Save Changes" persists updates directly to `~/.centmem/config.toml` and displays a confirmation toast.
- [x] 4. "Test Connection" button accurately probes the specified Local LLM endpoint and reports status.
- [x] 5. All automated unit and contract tests pass (`go test -tags fts5 -race ./...`).
- [x] 6. UI builds cleanly with zero console warnings and adheres strictly to Impeccable Operate mode and Antislop-UI rules.
