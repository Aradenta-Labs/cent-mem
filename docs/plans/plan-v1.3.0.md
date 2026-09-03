# Implementation Plan: Auto-capture from Agent Transcripts (Hooks) — v1.1

**Target version:** 1.1.0
**Status:** Planning
**Goal:** Automatically extract and store durable knowledge from AI agent session transcripts into cent-mem, with zero mandatory effort from the user after initial configuration.

---

## Resolved Requirements

| # | Requirement | Decision |
|---|---|---|
| Triggers | When does capture happen? | All three: after every agent message, at session end, and on-demand |
| Scope | What gets captured? | User-configurable via config key, wizard, and config file |
| Classification | How is content filtered before saving? | Hook runs an AI-agent classification pass first |
| User control | How much human oversight? | Summary shown at session end via `centmem capture summary` |
| Harnesses | Which agents are supported? | Antigravity, Trae, Claude Code, Cursor, Codex, Deepseek, Hermes |
| Transcript access | How does the hook read transcripts? | Per-harness (file-based watcher or API/webhook depending on what each exposes) |
| Deduplication | Same fact across sessions? | AI decides if content is new information worth saving (recall-before-write) |
| Privacy | Filtering sensitive content? | User's discretion; no built-in blocklist |
| Session dedup | Same content captured twice in one session? | Yes — maintain a session-level "already saved" dedup log |
| Config interface | Three modes | `centmem config set`, interactive wizard at `init`, and direct `config.toml` editing |
| Categories | What types does the classifier produce? | Default set + user can add/remove categories |
| **Classifier backend** | What does the classification pass? | Three-tier: **Local LLM** → **Lightweight heuristic (keyword-based)** → **BYOK OpenAI-compatible API**; user selects in config |
| **Confidence threshold** | Configurable save threshold? | Yes — `capture.confidence_threshold` in `config.toml` (default: `0.7`) |
| **Transcript normalization** | Format conversion tool? | Yes — `centmem capture convert --harness <name> --input <path>` subcommand |


---

## Default Capture Categories

Users can add to or remove from this list via config.

| Category | Description | Maps to centmem type |
|---|---|---|
| `decision` | Architecture or design choices | `put --type note --tags decision` |
| `fact` | Concrete facts (URLs, keys, versions) | `set --key <derived>` |
| `preference` | User workflow or style preferences | `put --type note --tags preference` |
| `code` | Code snippets, patterns, or examples | `put --type note --tags code` |
| `log` | Progress notes, session checkpoints | `put --type log` |
| `error` | Bugs, root causes, and their resolutions | `put --type note --tags error,resolution` |
| `dependency` | Library or service dependencies discovered | `set --key dep.<name>` |

---

## Architecture Overview

```
 Agent Session
      │
      ├─ [trigger: per-message]  ──►  Hook Adapter (per harness)
      ├─ [trigger: session-end]  ──►  Hook Adapter (per harness)
      └─ [trigger: on-demand]    ──►  centmem capture run
                                              │
                                    ┌─────────▼──────────┐
                                    │  internal/capture  │
                                    │  ├─ reader.go       │  reads raw transcript
                                    │  ├─ classifier.go   │  AI classification pass
                                    │  ├─ dedup.go        │  session log + recall check
                                    │  └─ writer.go       │  calls centmem put/set
                                    └─────────┬──────────┘
                                              │
                                    centmem store (SQLite)
```

### New package: `internal/capture/`

| File | Responsibility |
|---|---|
| `reader.go` | Reads raw transcript text from the source (file path or stdin pipe) |
| `classifier.go` | Runs the classification prompt against the AI; returns typed `CaptureItem` slice |
| `dedup.go` | Maintains the session-level saved-IDs log; runs `recall` before each write to detect duplicates |
| `writer.go` | Converts `CaptureItem` → `centmem put` / `centmem set` calls |
| `summary.go` | Builds and serializes the per-session capture report |
| `config.go` | `CaptureConfig` struct (categories, triggers, scope, harness) |
| `session.go` | Session lifecycle: start, checkpoint, end |

### Config schema additions (`internal/config/config.go`)

```go
type CaptureConfig struct {
    Enabled             bool
    Harness             string   // antigravity | trae | claude-code | cursor | codex | deepseek | hermes
    Triggers            []string // "message" | "session-end" | "on-demand"
    Scope               string   // centmem scope for captured memories
    Categories          []string // user-configured list
    TranscriptPath      string   // override: path to transcript file (harness-dependent default if empty)

    // Classifier backend — evaluated in order until one succeeds
    Backend             string   // "local-llm" | "heuristic" | "openai-compatible"

    // Local LLM settings (used when Backend = "local-llm")
    LocalLLMEndpoint    string   // e.g. "http://localhost:11434/v1" (Ollama default)
    LocalLLMModel       string   // e.g. "llama3.2"

    // BYOK OpenAI-compatible API settings (used when Backend = "openai-compatible")
    APIBaseURL          string   // e.g. "https://api.openai.com/v1"
    APIKeyEnv           string   // name of the env var holding the key, e.g. "OPENAI_API_KEY"
    APIModel            string   // e.g. "gpt-4o-mini"

    // Classification quality
    ConfidenceThreshold float64  // items below this score are skipped (default: 0.7)
}
```

**Env overrides for new fields:**
- `CENTMEM_CAPTURE_BACKEND` — `local-llm` | `heuristic` | `openai-compatible`
- `CENTMEM_CAPTURE_LOCAL_LLM_ENDPOINT`
- `CENTMEM_CAPTURE_LOCAL_LLM_MODEL`
- `CENTMEM_CAPTURE_API_BASE_URL`
- `CENTMEM_CAPTURE_API_KEY_ENV`
- `CENTMEM_CAPTURE_API_MODEL`
- `CENTMEM_CAPTURE_CONFIDENCE_THRESHOLD`


---

## Milestones

### Phase 1 — Core Capture Infrastructure
*The foundation everything else is built on. No harness wiring yet — just the capture engine itself.*

#### `internal/capture/` package

##### [NEW] `internal/capture/config.go`
- Define `CaptureConfig` struct with `Enabled`, `Harness`, `Triggers`, `Scope`, `Categories`, `TranscriptPath`.
- Implement `DefaultCategories()` returning the 7 default category strings.
- Add `CaptureConfig` as a field on the existing `Config` struct in `internal/config/config.go`.
- Add TOML tag support for `[capture]` block in `config.toml`.
- Env override: `CENTMEM_CAPTURE_ENABLED`, `CENTMEM_CAPTURE_HARNESS`, `CENTMEM_CAPTURE_SCOPE`.

##### [NEW] `internal/capture/reader.go`
- `ReadTranscript(path string) ([]TranscriptMessage, error)` — reads transcript from a file path.
- `ReadTranscriptFromPipe(r io.Reader) ([]TranscriptMessage, error)` — reads from stdin pipe.
- `TranscriptMessage` struct: `{Role string, Content string, Timestamp time.Time}`.
- Supports JSON and plain-text transcript formats.

##### [NEW] `internal/capture/classifier.go`
- `Classify(messages []TranscriptMessage, categories []string) ([]CaptureItem, error)`.
- `CaptureItem` struct: `{Category string, Content string, Key string, Tags []string, Confidence float64}`.
- Classification is done by calling back into the agent via the `centmem` CLI's own context — the hook invokes the shell tool that calls back the running agent.
- Prompt template: configurable, stored in `~/.centmem/capture-prompt.md` (written at `init`).

##### [NEW] `internal/capture/dedup.go`
- Session-level dedup log stored at `~/.centmem/session-<id>.dedup.json`.
- `IsDuplicate(item CaptureItem, store *store.Store) (bool, error)` — runs `recall` against the store; passes result to classifier for novelty judgment.
- `MarkSaved(item CaptureItem)` — records item hash in the session dedup log.
- Session log is cleaned up at session end.

##### [NEW] `internal/capture/writer.go`
- `Write(item CaptureItem, cfg CaptureConfig, store *store.Store) error`.
- Maps each `CaptureItem` category to the correct `centmem put` / `centmem set` call.
- Writes `--source-agent capture-hook` and `--source-session <session-id>` on every write for provenance.

##### [NEW] `internal/capture/summary.go`
- `BuildSummary(sessionID string) (CaptureSummary, error)`.
- `CaptureSummary` struct: `{SessionID, Harness, StartedAt, EndedAt, TotalMessages, CapturedCount, SkippedCount, Items []CaptureItem}`.
- Summary persisted to `~/.centmem/capture-summary-<session-id>.json` at session end.

##### [NEW] `internal/capture/session.go`
- `StartSession(harness string) (string, error)` — generates session ID, writes `.centmem/capture-session.lock`.
- `EndSession(sessionID string)` — finalizes summary, cleans up dedup log.

---

### Phase 2 — CLI Subcommand: `centmem capture`

*Expose the capture engine through the CLI so users can trigger it manually and inspect results.*

#### [MODIFY] `cmd/centmem/commands.go`
- Register `capture` as a top-level command entry.
- Sub-commands: `run`, `summary`, `config`, `categories`, `convert`.

#### [NEW] `cmd/centmem/handlers_capture.go`

| Handler | CLI | Purpose |
|---|---|---|
| `cmdCaptureRun()` | `centmem capture run [--transcript <path>] [--scope <s>]` | Run a capture pass on a transcript (on-demand trigger) |
| `cmdCaptureSummary()` | `centmem capture summary [--session <id>]` | Print the last (or a specific) session's capture summary as JSON |
| `cmdCaptureConfig()` | `centmem config set capture.<key> <value>` | Set a capture config key |
| `cmdCaptureCategories()` | `centmem capture categories [--add <c>] [--remove <c>] [--list]` | Manage the user's category list |
| `cmdCaptureConvert()` | `centmem capture convert --harness <name> --input <path> [--output <path>]` | Normalize a harness-specific transcript file into cent-mem's standard JSON format for scripted or manual use |

**`centmem capture convert` output format:**
```json
{"ok": true, "output": "/path/to/normalized.jsonl", "messages": 42, "harness": "cursor"}
```
Writes a normalized `.jsonl` file with `{role, content, timestamp}` per line. Output path defaults to `<input>.centmem.jsonl` if `--output` is omitted.


**JSON output for `centmem capture summary`:**
```json
{
  "ok": true,
  "session_id": "abc123",
  "harness": "claude-code",
  "started_at": "2026-09-02T10:00:00Z",
  "ended_at": "2026-09-02T11:00:00Z",
  "total_messages": 42,
  "captured": 7,
  "skipped_duplicate": 3,
  "skipped_low_confidence": 2,
  "items": [
    {"category": "decision", "content": "We use SQLite-vec for local embeddings", "tags": ["decision", "db"], "confidence": 0.92}
  ]
}
```

---

### Phase 3 — Config UX (Three Interfaces)

*Users can configure capture three ways — they all write to the same `config.toml`.*

#### Interface 1: Config Key (`centmem config set`)

```bash
centmem config set capture.enabled true
centmem config set capture.harness claude-code
centmem config set capture.scope "project:myapp"
centmem config set capture.triggers "message,session-end,on-demand"
centmem config set capture.categories "decision,fact,preference,code,log,error,dependency"
```

#### [NEW] `cmd/centmem/handlers_config.go`
- `cmdConfigSet(key, value string)` — reads `config.toml`, updates the key, writes back.
- `cmdConfigGet(key string)` — reads `config.toml`, prints the value as JSON.
- Validates known keys; rejects unknown keys with a clear error.

#### Interface 2: Interactive Wizard (extended `centmem init`)

#### [MODIFY] `cmd/centmem/handlers.go` — `cmdInit()`
- After the existing DB + model setup, detect if a `[capture]` section already exists in `config.toml`.
- If absent, prompt the user (stdout questions, stdin answers):
  1. *"Which AI agent do you use? [antigravity / trae / claude-code / cursor / codex / deepseek / hermes]"*
  2. *"Which triggers to enable? [message / session-end / on-demand / all]"*
  3. *"Default scope for captured memories? [e.g. project:myapp]"*
  4. *"Capture categories (comma-separated, or press Enter to keep defaults):"*
- Write the answers directly into `[capture]` in `config.toml`.
- Write `capture-prompt.md` to `~/.centmem/`.

#### Interface 3: Direct `config.toml` editing

#### [MODIFY] `internal/config/config.go`
- Implement full TOML read/write for `config.toml`.
- Ensure `Load()` reads a `[capture]` block if present.
- Document the `[capture]` schema at the top of `config.toml` (written by `init`).

---

### Phase 4 — Per-Harness Hook Adapters

*Wire the capture engine into each agent environment's actual transcript delivery mechanism.*

Each harness adapter is responsible for: detecting a transcript source, watching for new content, and calling `centmem capture run` (or the internal Go API).

#### [NEW] `skill/adapters/hooks/` directory

| File | Harness | Transcript Access Method |
|---|---|---|
| `antigravity.md` | Antigravity | Antigravity exposes conversation transcript files under `~/.gemini/antigravity/brain/<id>/`; hook uses a file watcher on the `.jsonl` transcript |
| `trae.md` | Trae | File-based; Trae writes session logs to a known path; hook polls on session-end signal |
| `claude-code.md` | Claude Code | Reads `.claude/` session log directory; hooks into session-end via shell exit trap |
| `cursor.md` | Cursor | Injects a post-conversation shell command via `.cursorrules`; reads cursor log export |
| `codex.md` | Codex (OpenAI) | Stdin/stdout pipe; wraps `codex` invocation in a thin shell shim |
| `deepseek.md` | Deepseek Harness | File-based; reads Deepseek's local session export |
| `hermes.md` | Hermes | Custom harness; hook installed via Hermes's plugin/hook API |

#### [NEW] `skill/adapters/hooks/install.sh`
- Per-harness auto-detection and hook registration script.
- Installs the correct watcher/trap/shim for the detected harness.
- Idempotent and `--dry-run` safe.

#### Trigger: per-message (file watcher)
- For file-based harnesses: `centmem capture run --watch --transcript <path>` watches the transcript file for new lines and runs classification incrementally.
- Requires a long-running background process (managed as a `launchd` plist on macOS or `systemd` unit on Linux).

#### Trigger: session-end (exit trap / plugin hook)
- For shell-based harnesses: inject `trap 'centmem capture run --transcript $TRANSCRIPT_PATH --scope "$CENTMEM_SCOPE" && centmem capture summary' EXIT` into the harness startup script.

#### Trigger: on-demand
- Already handled by `centmem capture run` from Phase 2.

---

### Phase 5 — Classification Backends & Prompt Engine

*The heart of the system. Supports three backends in a priority chain: Local LLM → Heuristic → BYOK OpenAI-compatible.*

#### Backend Selection Logic (`internal/capture/classifier.go`)

The classifier tries backends in user-defined priority order. If the configured backend fails (e.g., local LLM is offline), it automatically falls back to the next available one.

```
Backend priority chain:
  local-llm  ──► (if unreachable) ──► heuristic
  openai-compatible ──► (if key missing/error) ──► heuristic
  heuristic  ──► always succeeds
```

#### Backend 1: Local LLM (`backend = "local-llm"`)

- Calls an OpenAI-compatible local endpoint (e.g., Ollama at `http://localhost:11434/v1`).
- Config: `capture.local_llm_endpoint`, `capture.local_llm_model`.
- Sends the classification prompt + transcript chunk as a chat completion request.
- Returns JSON array of `CaptureItem`. Any malformed response is retried once, then falls back to heuristic.
- **No data leaves the machine** when using a local endpoint — privacy preserved.

#### Backend 2: Lightweight Heuristic (`backend = "heuristic"`)

- Zero dependencies, no model required. Always available.
- Pattern-matching rules per category:
  | Category | Detection signals |
  |---|---|
  | `decision` | Keywords: "we decided", "we chose", "going with", "agreed on" |
  | `fact` | Patterns: URLs (`https://`), version strings (`v1.2.3`), key=value pairs |
  | `preference` | Keywords: "I prefer", "always use", "don't use", "my convention" |
  | `code` | Fenced code blocks (` ``` `), inline backtick expressions |
  | `log` | Starts with "completed", "finished", "implemented", "added", "fixed" |
  | `error` | Keywords: "bug", "root cause", "the fix was", "resolved by" |
  | `dependency` | Patterns: `import`, `require`, `go get`, `npm install`, `pip install` |
- Confidence is always `0.75` for matched items (below the AI-tier quality but above noise).
- User-visible in summary as `backend: "heuristic"` so they know the quality tier.

#### Backend 3: BYOK OpenAI-Compatible API (`backend = "openai-compatible"`)

- Calls any OpenAI-compatible endpoint (OpenAI, Groq, Anthropic via compatibility shim, etc.).
- Config: `capture.api_base_url`, `capture.api_key_env` (env var name, not the key itself), `capture.api_model`.
- The API key is **never stored in `config.toml`** — only the name of the env var that holds it.
- Same classification prompt as the Local LLM backend.
- Falls back to heuristic if the env var is unset or the API returns an error.

#### [NEW] `~/.centmem/capture-prompt.md` (written at `init`, user-editable)

```markdown
You are a memory classifier for the cent-mem project memory store.
You will receive a list of AI agent conversation messages.
Your job is to identify content worth saving as a long-term memory.

For each piece of content worth saving, output a JSON object with:
- category: one of [{{CATEGORIES}}]
- content: the exact content to store (concise, self-contained)
- key: (for facts only) a dot-notation key like "api.base_url"
- tags: array of relevant tags
- confidence: 0.0–1.0 (only output items with confidence ≥ {{CONFIDENCE_THRESHOLD}})

Before saving, you will also receive the top-3 recall results for similar content.
If any existing memory already captures this information, output {"skip": true, "reason": "..."}.

Output a JSON array. Output nothing else.
```

- `{{CATEGORIES}}` and `{{CONFIDENCE_THRESHOLD}}` are replaced at runtime from `CaptureConfig`.

#### [MODIFY] `internal/capture/classifier.go` — dedup integration
- Before calling any backend, run `recall` for a short summary of the candidate content.
- Pass the recall results as context in the classification prompt.
- If output contains `{"skip": true}`, record in the session dedup log and do not write.
- Confidence threshold filtering: items below `CaptureConfig.ConfidenceThreshold` are logged to the summary as `skipped_low_confidence` but never written.

#### Init wizard additions (Phase 3 extension)

Add these questions to the init wizard:
1. *"Which classifier backend? [local-llm / heuristic / openai-compatible]"*
2. If `local-llm`: *"Local LLM endpoint? [default: http://localhost:11434/v1]"* and *"Model name? [e.g. llama3.2]"*
3. If `openai-compatible`: *"API base URL? [default: https://api.openai.com/v1]"*, *"Which env var holds your API key? [e.g. OPENAI_API_KEY]"*, and *"Model? [e.g. gpt-4o-mini]"*
4. *"Confidence threshold (0.0–1.0)? [default: 0.7]"*



---

### Phase 6 — Testing

#### Unit tests
- `internal/capture/classifier_test.go` — mock AI responses, validate `CaptureItem` parsing.
- `internal/capture/dedup_test.go` — verify session log creation, hash collision detection, recall integration.
- `internal/capture/writer_test.go` — validate that each category maps to the correct `put`/`set` call.
- `internal/capture/summary_test.go` — validate summary JSON shape and field completeness.
- `internal/config/config_test.go` — extended: test `[capture]` block loading from TOML.

#### Golden file tests
- `testdata/golden/capture_summary.golden.json` — expected `centmem capture summary` output shape.
- `testdata/golden/capture_run_decision.golden.json` — capture run with a single decision item.

#### E2E test
- `cmd/centmem/e2e_capture_test.go` — `TestE2E_CaptureRun()`:
  1. Write a fixture transcript file with decisions, facts, and logs.
  2. Run `centmem capture run --transcript <fixture>`.
  3. Run `centmem recall` to confirm memories were stored correctly.
  4. Run `centmem capture summary` and validate the JSON output.

---

### Phase 7 — Documentation & Release

#### [MODIFY] `docs/getting-started.md`
- Add a "Capture Hooks" section after the existing quickstart.

#### [NEW] `docs/guides/capture-hooks.md`
- Full guide: installation, config wizard, per-harness setup, custom categories, and the summary command.

#### [MODIFY] `skill/SKILL.md`
- Add capture recipes:
  - On-demand: `centmem capture run`
  - Check what was saved: `centmem capture summary`
  - Manage categories: `centmem capture categories --list`

#### [MODIFY] `CHANGELOG.md`
- Draft v1.1.0 release notes.

#### [MODIFY] `README.md`
- Update the roadmap to mark `v1.1` as complete.

---

## Verification Plan

### Automated Tests
```bash
go test ./internal/capture/... -race
go test ./internal/config/... -race
go test ./cmd/centmem/... -run TestE2E_Capture -race
```

### Manual Verification Checklist
- [ ] `centmem init` prompts the capture wizard and writes `[capture]` to `config.toml`
- [ ] `centmem config set capture.categories "decision,code"` updates config without corrupting other keys
- [ ] `centmem capture categories --add "insight"` adds a new category and persists it
- [ ] `centmem capture run --transcript testdata/fixture.jsonl` stores memories and prints a summary path
- [ ] `centmem capture summary` returns valid JSON matching the golden schema
- [ ] Running the same transcript twice does not produce duplicate memories (dedup works)
- [ ] Claude Code hook fires at session end and calls `centmem capture run` automatically
- [ ] Antigravity transcript watcher detects new messages and classifies them incrementally

---

## All Requirements Resolved

All open questions have been answered. This plan is ready for implementation.

| Question | Answer | Where implemented |
|---|---|---|
| Classification runtime | Three-tier backend: Local LLM → Heuristic → BYOK OpenAI-compatible | Phase 5, `internal/capture/classifier.go` |
| Confidence threshold | Yes, user-configurable via `capture.confidence_threshold` (default `0.7`) | Phase 3 config, Phase 5 prompt |
| Transcript normalization | Yes, `centmem capture convert --harness <name> --input <path>` | Phase 2, `cmdCaptureConvert()` |

