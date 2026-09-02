# Guide: Auto-capture from Agent Transcripts (Hooks)

cent-mem features an **auto-capture subsystem** that automatically extracts and stores durable knowledge (architectural decisions, user preferences, concrete facts, code patterns, checkpoints, error resolutions, and dependencies) directly from your AI agent conversations.

With capture hooks enabled, your AI agents maintain a shared memory store with **zero manual effort** after initial setup.

---

## 1. Overview & Architecture

Whenever an AI agent interacts in a supported harness, cent-mem intercepts or monitors the conversation transcript, classifies candidate learnings using a three-tier classifier, deduplicates against existing memories, and persists novel entries to the local SQLite database.

```
                  ┌───────────────────────────────────────────────┐
                  │              Agent Conversation               │
                  │  (Antigravity / Claude Code / Cursor / etc.)  │
                  └──────────────────────┬────────────────────────┘
                                         │
                 ┌───────────────────────┼───────────────────────┐
                 │                       │                       │
      [Trigger: per-message]   [Trigger: session-end]   [Trigger: on-demand]
                 │                       │                       │
                 ▼                       ▼                       ▼
      File Watcher (--watch)      Shell Exit Trap       centmem capture run
                 │                       │                       │
                 └───────────────────────┼───────────────────────┘
                                         │
                                         ▼
                     ┌───────────────────────────────────────┐
                     │           internal/capture            │
                     │  1. Transcript Reader / Normalizer   │
                     │  2. 3-Tier Classification Engine      │
                     │  3. Recall-Before-Write Deduplication │
                     │  4. Store Writer (put / set)          │
                     └───────────────────┬───────────────────┘
                                         │
                                         ▼
                     ┌───────────────────────────────────────┐
                     │          SQLite Store Engine          │
                     │   memories, facts, vector embeddings  │
                     └───────────────────────────────────────┘
```

### Core Design Invariants

- **Local-first & privacy-preserving:** By default, classification runs entirely locally via your local LLM (e.g. Ollama) or lightweight heuristic pattern matching. No conversation data ever leaves your machine.
- **Zero API keys stored in config:** When using BYOK cloud endpoints, `config.toml` only stores the *name* of the environment variable holding your key (e.g. `OPENAI_API_KEY`).
- **Asynchronous & non-blocking:** Transcript capture runs incrementally or at session teardown without adding latency to agent prompt responses.
- **Recall-before-write deduplication:** Before saving a memory, cent-mem checks existing store records and skips duplicate items.

---

## 2. Supported Harnesses & Access Matrix

cent-mem includes dedicated hook adapters for 7 major AI agent harnesses:

| Harness | Transcript Access Method | Default Transcript Path / Mechanism | Supported Triggers |
|---|---|---|---|
| **Antigravity** | JSONL file watcher | `~/.gemini/antigravity/brain/<id>/.system_generated/logs/transcript.jsonl` | `message` (`--watch`), `session-end`, `on-demand` |
| **Trae** | Structured session logs | `~/.trae/logs/sessions/<id>/session.log` | `session-end`, `on-demand` |
| **Claude Code** | Shell `EXIT` trap | `.claude/` session directories & shell profile trap | `session-end`, `on-demand` |
| **Cursor** | Composer rules / logs | `.cursorrules` injection & `.cursor/logs/` | `session-end`, `on-demand` |
| **Codex** | Stdin/stdout pipe shim | `codex "$@" \| tee >(centmem capture run --harness codex)` | `message` (streaming pipe), `session-end` |
| **DeepSeek** | Session export watcher | `~/.deepseek/sessions/` | `session-end`, `on-demand` |
| **Hermes** | Event bus plugin | Hermes lifecycle hook API (`on_message_end`, `on_session_end`) | `message`, `session-end` |

---

## 3. Installation & Setup

### Unified Shell Hook Installer (`install.sh`)

The easiest way to register transcript hooks is using the unified installer script:

```bash
# Auto-detect all installed harnesses and install appropriate hooks
bash skill/adapters/hooks/install.sh --all

# Or target a specific harness with a custom scope
bash skill/adapters/hooks/install.sh --harness claude-code --scope "project:myapp"
```

#### Installer CLI Options

| Flag | Argument | Description |
|---|---|---|
| `--all` | _(none)_ | Install hooks for all detected agent harnesses on the machine |
| `--harness` | `<name>` | Target a specific harness (`antigravity`, `claude-code`, `cursor`, `trae`, `codex`, `deepseek`, `hermes`) |
| `--scope` | `<s>` | Default target memory scope for auto-captured items (e.g. `project:myproj`) |
| `--trigger` | `<triggers>` | Comma-separated triggers: `message`, `session-end`, `on-demand`, or `all` |
| `--list` | _(none)_ | List detected harnesses and proposed hook modifications without making changes |
| `--dry-run` | _(none)_ | Preview all disk changes, symlinks, and configurations without modifying files |
| `--uninstall` | _(none)_ | Safely remove installed hooks, shims, traps, and background watcher services |

### Background Service Setup (for Real-Time Watching)

For file-based harnesses (such as Antigravity, Trae, or DeepSeek) that support per-message incremental watching, `install.sh` can generate service configurations:

- **macOS (`launchd`):** Generates `~/Library/LaunchAgents/com.aradenta.centmem.capture-watcher.plist`
- **Linux (`systemd`):** Generates `~/.config/systemd/user/centmem-capture-watcher.service`

You can also run the watcher directly in any terminal:
```bash
centmem capture run --watch --transcript /path/to/transcript.jsonl --scope project:myapp
```

---

## 4. Configuration (Three Interfaces)

You can configure the capture engine through any of three interfaces—all write to the same `~/.centmem/config.toml` file.

### Interface 1: Interactive Wizard (`centmem init`)

When running `centmem init`, if no `[capture]` section exists, the CLI prompts you through interactive setup:

```
Configure auto-capture from AI agent transcripts? [Y/n]
> Y
Which AI agent harness do you primarily use? [antigravity / trae / claude-code / cursor / codex / deepseek / hermes]
> claude-code
Which classification backend do you want to use? [local-llm / heuristic / openai-compatible]
> local-llm
Local LLM endpoint? [default: http://localhost:11434/v1]
> http://localhost:11434/v1
Local LLM model name? [default: llama3.2]
> llama3.2
Default memory scope for captured items? [default: global]
> project:myapp
```

### Interface 2: CLI Key/Value Management (`centmem config set`)

Manage individual configuration keys directly from the command line:

```bash
# Enable auto-capture
centmem config set capture.enabled true

# Configure harness and scope
centmem config set capture.harness claude-code
centmem config set capture.scope "project:myapp"

# Configure classifier backend
centmem config set capture.backend local-llm
centmem config set capture.local_llm_endpoint "http://localhost:11434/v1"
centmem config set capture.local_llm_model "llama3.2"

# Adjust confidence threshold (0.0 to 1.0)
centmem config set capture.confidence_threshold 0.75
```

### Interface 3: Direct `config.toml` Schema

You can edit `~/.centmem/config.toml` directly. Below is the complete `[capture]` schema:

```toml
[capture]
# Enable or disable transcript capture
enabled = true

# Target agent harness
harness = "claude-code"

# Active trigger events: "message", "session-end", "on-demand"
triggers = ["message", "session-end", "on-demand"]

# Default centmem scope for captured memories
scope = "project:myapp"

# Whitelist of active categories to extract
categories = ["decision", "fact", "preference", "code", "log", "error", "dependency"]

# Optional override path to conversation transcript
transcript_path = ""

# Classifier backend: "local-llm" | "heuristic" | "openai-compatible"
backend = "local-llm"

# Local LLM settings (used when backend = "local-llm")
local_llm_endpoint = "http://localhost:11434/v1"
local_llm_model = "llama3.2"

# BYOK OpenAI-compatible settings (used when backend = "openai-compatible")
api_base_url = "https://api.openai.com/v1"
api_key_env = "OPENAI_API_KEY"
api_model = "gpt-4o-mini"

# Classification confidence score threshold (0.0 - 1.0)
confidence_threshold = 0.70
```

#### Environment Variable Overrides

Every configuration key can be overridden at runtime using environment variables:

| Environment Variable | Overrides Key | Example |
|---|---|---|
| `CENTMEM_CAPTURE_ENABLED` | `capture.enabled` | `true` |
| `CENTMEM_CAPTURE_HARNESS` | `capture.harness` | `antigravity` |
| `CENTMEM_CAPTURE_SCOPE` | `capture.scope` | `project:backend` |
| `CENTMEM_CAPTURE_BACKEND` | `capture.backend` | `local-llm` |
| `CENTMEM_CAPTURE_LOCAL_LLM_ENDPOINT` | `capture.local_llm_endpoint` | `http://localhost:11434/v1` |
| `CENTMEM_CAPTURE_LOCAL_LLM_MODEL` | `capture.local_llm_model` | `llama3.2` |
| `CENTMEM_CAPTURE_API_BASE_URL` | `capture.api_base_url` | `https://api.openai.com/v1` |
| `CENTMEM_CAPTURE_API_KEY_ENV` | `capture.api_key_env` | `OPENAI_API_KEY` |
| `CENTMEM_CAPTURE_API_MODEL` | `capture.api_model` | `gpt-4o-mini` |
| `CENTMEM_CAPTURE_CONFIDENCE_THRESHOLD` | `capture.confidence_threshold` | `0.80` |
| `CENTMEM_CAPTURE_CATEGORIES` | `capture.categories` | `decision,fact,code` |
| `CENTMEM_CAPTURE_TRIGGERS` | `capture.triggers` | `message,session-end` |

---

## 5. Classification Backends & Fallback Chain

cent-mem uses a resilient **three-tier classification chain**. If the configured primary backend is unreachable or fails, the engine retries once and gracefully falls back to the next available tier so capture never halts.

```
  [Configured: local-llm]
            │
            ├─► Endpoint reachable & valid JSON? ──► Use local LLM result
            │
            └─► (Offline / 500 error / retry exhausted)
                     │
                     ▼
  [Configured: openai-compatible]
            │
            ├─► API key present & endpoint valid? ──► Use cloud LLM result
            │
            └─► (Missing env var / 401 error / retry exhausted)
                     │
                     ▼
  [Fallback: heuristic] ───────────────────────────► Deterministic keyword patterns
```

### Backend 1: Local LLM (`backend = "local-llm"`)
- Connects to local OpenAI-compatible inference servers (Ollama, LM Studio, vLLM, LocalAI).
- **100% offline and private:** No prompt or conversation data ever leaves your device.
- Temperature is locked to `0.0` for deterministic extraction.
- Malformed JSON outputs are retried once before falling back.

### Backend 2: Lightweight Heuristic (`backend = "heuristic"`)
- Zero dependencies, zero network calls, instant execution.
- Deterministic regex pattern matching across all 7 default categories.
- Assigns a baseline confidence score of `0.75` for matched patterns.

### Backend 3: BYOK OpenAI-Compatible (`backend = "openai-compatible"`)
- Connects to any OpenAI-compatible remote provider (OpenAI, Groq, Together AI, DeepSeek, Anthropic via shim).
- **Zero secrets stored on disk:** Resolves API keys exclusively from the environment variable specified in `api_key_env` (e.g. `$OPENAI_API_KEY`).

---

## 6. Capture Categories & Storage Mappings

cent-mem classifies content into 7 standard categories, mapping each to its optimal storage format and tags:

| Category | Description | Storage Action | Assigned Tags |
|---|---|---|---|
| `decision` | Architecture, design, or framework choices | `put --type note` | `["decision"]` |
| `fact` | Concrete endpoints, versions, URLs, key/value facts | `set --key <sanitized_key>` | `["fact"]` |
| `preference` | User preferences, code style, tooling choices | `put --type note` | `["preference"]` |
| `code` | Code snippets, implementation patterns, algorithms | `put --type note` | `["code", "snippet"]` |
| `log` | Task milestones, completion checkpoints | `put --type log` | `["log"]` |
| `error` | Bugs, root causes, and their resolutions | `put --type note` | `["error", "resolution"]` |
| `dependency` | Discovered libraries, packages, or services | `set --key dep.<pkg>` | `["dependency"]` |

### Managing Categories

Users can inspect and customize active categories via the CLI:

```bash
# List active categories
centmem capture categories --list

# Add a custom category
centmem capture categories --add "security"

# Remove a category
centmem capture categories --remove "log"
```

---

## 7. Transcript Normalization (`centmem capture convert`)

Different agent harnesses produce transcripts in different formats (JSONL streams, markdown logs, JSON arrays). cent-mem provides a built-in normalization utility:

```bash
centmem capture convert --harness cursor --input .cursor/logs/session.json --output ./normalized.jsonl
```

**Output format:**
```json
{"ok": true, "output": "./normalized.jsonl", "messages": 42, "harness": "cursor"}
```

The resulting normalized `.jsonl` file contains one clean JSON object per line with standard fields:
```json
{"role": "user", "content": "Let's use SQLite-vec for local vector search", "timestamp": "2026-09-02T10:00:00Z"}
```

---

## 8. Session Inspection & Telemetry (`centmem capture summary`)

At the conclusion of a session (or on demand), cent-mem writes a comprehensive session summary to `~/.centmem/capture-summary-<session-id>.json`.

You can inspect the summary for the latest session at any time:

```bash
centmem capture summary
```

**Example JSON output:**
```json
{
  "ok": true,
  "session_id": "sess_01j7abc123",
  "harness": "claude-code",
  "backend": "local-llm",
  "started_at": "2026-09-02T10:00:00Z",
  "ended_at": "2026-09-02T10:15:30Z",
  "total_messages": 28,
  "captured": 4,
  "skipped_duplicate": 2,
  "skipped_low_confidence": 1,
  "items": [
    {
      "category": "decision",
      "content": "Selected SQLite-vec over pgvector for local-first embedded vector search",
      "tags": ["decision"],
      "confidence": 0.94
    },
    {
      "category": "fact",
      "key": "api.base_url",
      "content": "https://api.centmem.io/v1",
      "tags": ["fact"],
      "confidence": 0.98
    }
  ]
}
```

To view a summary for a specific historical session:
```bash
centmem capture summary --session sess_01j7abc123
```

---

## 9. Customizing Classification Prompts

cent-mem writes a customizable prompt template to `~/.centmem/capture-prompt.md` during initialization. You can edit this file to adjust extraction guidelines, add domain rules, or tweak categorization logic.

### Dynamic Prompt Variables

At runtime, cent-mem interpolates the following template variables:

| Variable | Description |
|---|---|
| `{{CATEGORIES}}` | Comma-separated list of currently enabled categories (e.g. `decision, fact, code`) |
| `{{CONFIDENCE_THRESHOLD}}` | The configured minimum confidence score threshold (e.g. `0.70`) |
| `{{RECALL_CONTEXT}}` | Formatted bullet list of top-ranked existing memories retrieved via recall for deduplication |

---

## 10. Troubleshooting & Common Pitfalls

### Local LLM Endpoint Offline
- **Symptom:** Logs show warning connecting to `http://localhost:11434/v1`.
- **Behavior:** cent-mem automatically falls back to `heuristic` pattern matching. Captures still succeed with `backend: "heuristic"` in summary.
- **Fix:** Start Ollama (`ollama serve`) or verify endpoint with `curl http://localhost:11434/v1/models`.

### Missing Cloud API Key
- **Symptom:** `capture.backend = "openai-compatible"` but summary indicates `backend: "heuristic"`.
- **Reason:** The environment variable specified in `capture.api_key_env` (default `OPENAI_API_KEY`) is unset.
- **Fix:** `export OPENAI_API_KEY="your-key"` in your shell profile.

### Stale Lockfile
- **Symptom:** Error `session lock active` when starting a capture run.
- **Reason:** An earlier watcher process was killed abruptly with `SIGKILL` without releasing `.centmem/capture-session.lock`.
- **Fix:** Verify no other `centmem` process is running (`pgrep centmem`), then remove `~/.centmem/capture-session.lock`.

---

## Summary of Capture Commands

```bash
centmem capture run [--transcript <path>] [--scope <s>] [--watch]   # Execute capture pass
centmem capture summary [--session <id>]                           # View session capture report
centmem capture categories [--list] [--add <c>] [--remove <c>]     # Manage category whitelist
centmem capture convert --harness <name> --input <path>            # Normalize transcript to JSONL
centmem config set capture.<key> <value>                           # Set capture configuration
centmem config get capture.<key>                                   # Read capture configuration
```
