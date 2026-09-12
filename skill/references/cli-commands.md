# CLI Commands Reference — centmem

This reference documents the complete CLI surface for `centmem`, including all flags, options, exit codes, and JSON shapes.

## Global Conventions

- **Default output is JSON on stdout.**
- **Errors are emitted as JSON on stderr:**
  ```json
  {"error":{"code":"<CODE>","message":"<msg>","hint":"<hint>"}}
  ```
- **Exit codes:**
  - `0`: Success
  - `1`: Error (internal error, DB failure, or invalid args)
  - `2`: Key or entity not found (`NOT_FOUND`)
  - `3`: Conflict / duplicate (`CONFLICT`)
- Use `--pretty` to indent JSON output for terminal inspection.
- Use `--direct` (or `CENTMEM_DIRECT=1`) to bypass daemon socket delegation and run directly against SQLite.

---

## 1. `centmem init`
Initializes the cent-mem directory structure (`~/.centmem` or `$CENTMEM_HOME`), runs SQLite migrations, and downloads the default ONNX embedding model.

```bash
centmem init [--model <name>] [--force]
```
- `--model <name>`: Model to use (default: `bge-small-en-v1.5`).
- `--force`: Force re-download and re-initialize existing database.

**Stdout JSON:**
```json
{"ok":true,"home":"/Users/user/.centmem","model":"bge-small-en-v1.5","dims":384}
```

---

## 2. `centmem put`
Stores a freeform text memory (note) or an execution/checkpoint event (log).

```bash
centmem put \
  --scope <scope> \
  --type <note|log> \
  --content "<text>" \
  [--tags <tag1,tag2>] \
  [--source-agent <agent>] \
  [--source-session <session-id>] \
  [--no-suggest]
```
- `--scope`: Required scope path (e.g. `project:my-app`).
- `--type`: `note` (default) or `log`.
- `--content`: Text content to store.
- `--tags`: Comma-separated tags (lowercase, no spaces).
- `--source-agent`: Agent identifier (e.g., `claude`, `antigravity`).
- `--source-session`: Session or conversation identifier.
- `--no-suggest`: Bypass post-write relationship auto-suggestion. When omitted, candidate relationships are evaluated and returned in `"suggested_links": [...]` if detected.

**Stdout JSON:**
```json
{"ok":true,"id":42,"scope":"project:my-app","status":"queued"}
```
*(When auto-suggested relationships are detected, response includes `"suggested_links": [{"id": 18, "from_id": 42, "to_id": 15, "relation": "supersedes", "target_content": "...", "target_type": "note"}]`)*

---

## 3. `centmem set`
Upserts a key/value fact. If a fact with the same key exists in the specified scope, its value is updated.

```bash
centmem set \
  --scope <scope> \
  --key <key> \
  --value '<json-value>' \
  [--tags <tag1,tag2>]
```
- `--scope`: Required scope path.
- `--key`: Identifier (e.g. `api.base_url`, `user.timezone`).
- `--value`: Valid JSON value (string `'"val"'`, number `42`, object `'{"k":"v"}'`).

**Stdout JSON:**
```json
{"ok":true,"id":43,"key":"api.base_url","scope":"project:my-app","status":"queued"}
```

---

## 4. `centmem get`
Fetches a single key/value fact by its exact key.

```bash
centmem get --scope <scope> --key <key> [--inherit]
```
- `--inherit`: Inherit facts from ancestor scopes if not found in given scope (default: `true`).

**Stdout JSON:**
```json
{"ok":true,"key":"api.base_url","value":"https://api.example.com","scope":"project:my-app","created_at":"2026-09-02T10:00:00Z"}
```

---

## 5. `centmem recall`
Hybrid search engine fusing vector semantic similarity, SQLite FTS5 keyword matching, key/value fact lookup, and recency scoring via Reciprocal Rank Fusion (RRF, k=60), followed by Stage 2 composite re-ranking and logarithmic importance boost reinforcement.

```bash
centmem recall "<query>" \
  --scope <scope> \
  [--top <N>] \
  [--type <note|log|fact>] \
  [--tags <tag1,tag2>] \
  [--since <duration|date>] \
  [--until <duration|date>] \
  [--agent <name>] \
  [--caller-agent <agent>] \
  [--reranker <strategy>] \
  [--inherit] \
  [--children] \
  [--include-links] \
  [--include-suggested]
```
- `--top`: Maximum items to return (default: `5`, max: `20`).
- `--inherit`: Search ancestor scopes (default: `true`).
- `--children`: Include descendant scopes in search.
- `--caller-agent`: Calling agent identifier for affinity boosting (+15% when matching author; defaults to `$CENTMEM_AGENT`).
- `--reranker`: Re-ranker strategy override (`composite`, `none`, `cross_encoder`, `llm`; defaults to `composite`).
- `--include-links`: Enrich results with 1-hop connected memory relationships (`links`). Returns confirmed links by default.
- `--include-suggested`: Include auto-suggested links pending confirmation in `links` expansion.

**Stdout JSON:**
```json
{
  "ok": true,
  "query": "how do we deploy?",
  "results": [
    {
      "id": 12,
      "type": "note",
      "scope": "project:my-app",
      "content": "We deploy via GitHub Actions to Fly.io",
      "tags": ["deploy", "ci"],
      "score": 0.8742,
      "matched_by": ["semantic", "keyword"],
      "access_count": 18,
      "last_accessed_at": 1788748000,
      "created_at": 1788000000,
      "links": [
        {
          "relation": "supersedes",
          "direction": "outgoing",
          "linked_id": 4,
          "linked_content": "Legacy deployment used Capistrano to EC2"
        }
      ]
    },
    {
      "id": 45,
      "type": "note",
      "scope": "project:my-app",
      "content": "Fly.io staging environment secrets are synchronized via 1Password CLI",
      "tags": ["deploy", "secrets"],
      "score": 0.7415,
      "matched_by": ["keyword"],
      "access_count": 0,
      "last_accessed_at": null,
      "created_at": 1788700000
    }
  ]
}
```
- `results`: Array of recalled memories ranked by score.
  - `access_count`: Number of times this memory has been recalled (integer $\ge 0$).
  - `last_accessed_at`: Unix timestamp (integer) of the most recent recall, or `null` if never previously recalled.
  - `score`: Calibrated relevance score (incorporating Stage 1 RRF, Stage 2 composite features, affinity/proximity boosts, and importance multipliers).
  - `matched_by`: List of search pipelines that matched this memory (`"semantic"`, `"keyword"`, `"fact"`, `"timeline"`).
  - `links`: (optional, present when `--include-links` is set) Array of 1-hop connected relationships (`relation`, `direction` ["outgoing"|"incoming"], `linked_id`, `linked_content`).

---

## 6. `centmem timeline`
Retrieves chronological memories for an audit trail or activity inspection.

```bash
centmem timeline --scope <scope> [--since <duration>] [--until <duration>] [--limit <N>]
```

**Stdout JSON:**
```json
{
  "ok": true,
  "entries": [
    {
      "id": 14,
      "content": "Completed deployment pipeline configuration",
      "created_at": 1788000000,
      "scope": "project:my-app",
      "tags": ["deploy", "ci"]
    }
  ]
}
```

---

## 7. `centmem capture`
Subcommands for transcript auto-capture and developer artifact ingestion pipelines (git, docs, shell, comments).

### Transcript Auto-Capture Commands
- `centmem capture run [--transcript <path>] [--scope <s>] [--watch]`
  Executes an on-demand or background continuous capture run.
- `centmem capture summary [--session <id>]`
  Returns telemetry and captured memories from the latest or specified session.
- `centmem capture categories [--list] [--add <category>] [--remove <category>]`
  Inspects or manages active extraction categories.
- `centmem capture convert --harness <name> --input <path> [--output <path>]`
  Normalizes harness-specific session logs to cent-mem standard JSONL.

---

### `centmem capture git`
Extracts architectural decisions, major commit narratives, and package dependency diffs from Git history into memory notes and facts.

```bash
centmem capture git [--repo <path>] [--since <sha|date>] [--scope <scope>] [--dry-run] [--max-commits <n>]
```
- `--repo <path>`: Target Git repository path (default: current directory or nearest Git root).
- `--since <sha|date>`: Commit SHA or duration/date (e.g., `24h`, `7d`, `2026-01-01`). If omitted, resumes from `.centmem/git-cursor`.
- `--scope <scope>`: Target memory scope (default: `project:<repo-name>`, or `global`).
- `--dry-run`: Simulate commit analysis without writing to SQLite or advancing cursor.
- `--max-commits <n>`: Maximum number of commits to process in this run (default: `100`).

**Stdout JSON:**
```json
{
  "ok": true,
  "command": "capture git",
  "source": ".git",
  "scanned": 15,
  "commits_scanned": 15,
  "memories_created": 3,
  "memories_updated": 0,
  "cursor": "a1b2c3d4e5f678901234567890abcdef12345678",
  "items": [
    {
      "id": 105,
      "type": "note",
      "tags": ["git", "commit", "decision"],
      "content": "Commit a1b2c3d: feat(auth): switch token signing from HS256 to RS256 with JWKS endpoint",
      "dry_run": false
    },
    {
      "id": 106,
      "type": "fact",
      "tags": ["git", "dependency"],
      "content": "dep.golang.jwt/v5 = v5.2.1",
      "dry_run": false
    }
  ]
}
```

---

### `centmem capture docs`
Parses documentation files, splits them into semantic heading sections, and indexes them as durable reference memories with automatic deletion tombstoning.

```bash
centmem capture docs [--dir <path>] [--scope <scope>] [--ext md,txt,rst] [--dry-run]
```
- `--dir <path>`: Target documentation directory (default: current directory).
- `--scope <scope>`: Target memory scope (default: `project:<repo-name>`, or `global`).
- `--ext <exts>`: Comma-separated file extensions to scan (default: `md,txt,rst`).
- `--dry-run`: Simulate markdown section chunking without persisting to SQLite or updating cursor.

**Stdout JSON:**
```json
{
  "ok": true,
  "command": "capture docs",
  "source": "/path/to/project/docs",
  "scanned": 8,
  "memories_created": 5,
  "memories_updated": 0,
  "cursor": "8 files tracked",
  "items": [
    {
      "id": 107,
      "type": "note",
      "tags": ["docs", "architecture", "overview"],
      "content": "# Architecture Overview\ncent-mem uses a hybrid search pipeline combining dense vector embeddings with SQLite FTS5...",
      "dry_run": false
    }
  ]
}
```

---

### `centmem capture shell`
Analyzes local developer shell history, scrubs sensitive arguments and tokens, and extracts recurring workflow patterns as a structured `shell.frequent_commands` fact.

```bash
centmem capture shell [--history <path>] [--shell <zsh|bash|fish>] [--scope <scope>] [--top <n>] [--dry-run]
```
- `--history <path>`: Custom path to shell history file (default: `$HISTFILE` or detected `~/.zsh_history`, `~/.bash_history`, `~/.local/share/fish/fish_history`).
- `--shell <zsh|bash|fish>`: Shell dialect/type (auto-detected if omitted).
- `--scope <scope>`: Target memory scope (default: `project:<repo-name>`, or `global`).
- `--top <n>`: Number of top recurring command patterns to preserve (default: `15`).
- `--dry-run`: Simulate frequency analysis without saving to SQLite or updating cursor.

**Stdout JSON:**
```json
{
  "ok": true,
  "command": "capture shell",
  "source": "/Users/user/.zsh_history",
  "scanned": 420,
  "memories_created": 1,
  "memories_updated": 0,
  "cursor": "1048576",
  "items": [
    {
      "id": 108,
      "type": "fact",
      "tags": ["shell", "toolchain", "conventions"],
      "content": "shell.frequent_commands = {\"go test -tags fts5 ./... -race\":42,\"git status\":35,\"centmem recall\":28}",
      "dry_run": false
    }
  ]
}
```

---

### `centmem capture comments`
Scans codebase source files for actionable annotations (`TODO`, `FIXME`, `HACK`, `SECURITY`, etc.), records line provenance, tracks line movements without duplicating entries, and scrubs credentials.

```bash
centmem capture comments [--dir <path>] [--ext <exts>] [--keywords <list>] [--scope <scope>] [--dry-run]
```
- `--dir <path>`: Target source code directory (default: current directory).
- `--ext <exts>`: Comma-separated file extensions to scan (default: `go,ts,js,py,rs,sh`).
- `--keywords <list>`: Comma-separated comment keywords (default: `TODO,FIXME,HACK,NOTE,OPTIMIZE,SECURITY,DEPRECATED`).
- `--scope <scope>`: Target memory scope (default: `project:<repo-name>`, or `global`).
- `--dry-run`: Simulate comment scan without writing to SQLite.

**Stdout JSON:**
```json
{
  "ok": true,
  "command": "capture comments",
  "source": "/path/to/project/src",
  "scanned": 45,
  "memories_created": 4,
  "memories_updated": 1,
  "items": [
    {
      "id": 109,
      "type": "note",
      "tags": ["comment", "todo", "jwt.go"],
      "content": "[file: internal/auth/jwt.go:42] [TODO] Rotate JWT signing key every 30 days via KMS",
      "dry_run": false
    }
  ]
}
```

---

## 8. `centmem config`
Inspects and updates configuration settings in `~/.centmem/config.toml`:

```bash
centmem config get [key]
centmem config set <key> <value>
```
- `centmem config get`: Dumps the full active configuration object as JSON.
- `centmem config get <key>`: Returns the value of a specific dot-notation key (`{"ok": true, "key": "...", "value": ...}`).
- `centmem config set <key> <value>`: Updates a dot-notation key and persists it to `config.toml`.

### Supported Configuration Keys

#### Search & Re-Ranking (`search.*`)
- `search.importance_boost_enabled`: (`bool`, default: `true`) Enable frequency-based logarithmic importance score boosting during Stage 2 re-ranking.
- `search.importance_weight`: (`float`, default: `0.1`) Multiplier scaling factor in the importance curve ($1.0 + \ln(1 + c) \times \text{weight}$).
- `search.importance_cap`: (`float`, default: `2.0`) Hard ceiling cap on the maximum allowable importance score multiplier ($2.0\times$).
- `search.decay_half_life_days`: (`int`, default: `0` [disabled]) Half-life in days for exponential recency decay.
- `search.reranker`: (`string`, default: `"composite"`) Stage 2 re-ranking strategy (`composite`, `none`, `cross_encoder`, `llm`).
- `search.rerank_window`: (`int`, default: `30`) Number of Stage 1 candidates passed to the Stage 2 rescorer.
- `search.session_boost`: (`float`, default: `1.25`) Score multiplier (+25%) for memories in the active session scope.
- `search.agent_boost`: (`float`, default: `1.15`) Score multiplier (+15%) for memories authored by the calling agent.

#### Model Settings (`model.*`)
- `model.name`: (`string`, default: `"bge-small-en-v1.5"`) ONNX embedding model name.
- `model.dims`: (`int`, default: `384`) Vector embedding dimension.

#### Retention Settings (`retention.*`)
- `retention.fact_keep_days`: (`int`, default: `0` [indefinite]) Retention period for facts.
- `retention.note_summarize_after_days`: (`int`, default: `30`) Threshold before notes are consolidated.
- `retention.log_summarize_after_days`: (`int`, default: `7`) Threshold before logs are consolidated.
- `retention.log_drop_after_days`: (`int`, default: `90`) Threshold before logs are purged.
- `retention.archive_keep_days`: (`int`, default: `365`) Retention window for archived rows.

#### Auto-Capture Settings (`capture.*`)
- `capture.enabled`: (`bool`, default: `true`) Enable automatic transcript capture.
- `capture.harness`: (`string`, default: `"claude-code"`) Active agent harness adapter.
- `capture.triggers`: (`list`) Capture triggers (`["session_end", "manual"]`).
- `capture.confidence_threshold`: (`float`, default: `0.7`) Minimum classification confidence for saving memories.

#### Unified LLM Provider Settings (`llm.*`) — v2.0.0 Stage 1
- `llm.backend`: (`string`, default: `"ollama"`) Active LLM provider (`"ollama"`, `"openai_compatible"`, `"disabled"`).
- `llm.endpoint`: (`string`, default: `"http://127.0.0.1:11434/v1"`) Base HTTP endpoint URL for completions.
- `llm.model`: (`string`, default: `"deepseek-r1:8b"`) Model identifier used for reasoning, synthesis, and curation.
- `llm.api_key`: (`string`, default: `""`) Raw API key or environment variable reference name (e.g. `"OPENAI_API_KEY"`).
- `llm.timeout_seconds`: (`int`, default: `60`) Timeout for LLM completion requests in seconds.
- `llm.max_tokens`: (`int`, default: `4096`) Maximum completion tokens per generation.
- `llm.temperature`: (`float`, default: `0.2`) Sampling temperature for model generations.

*Note: If `[llm]` is not explicitly configured, centmem automatically inherits legacy backend settings from `[capture]` (`capture.backend`, `capture.local_llm_endpoint`, `capture.api_base_url`, `capture.api_key`).*

#### Memory Agent Engine Settings (`agent.*`) — v2.0.0 Stage 1
- `agent.enabled`: (`bool`, default: `true`) Enable or disable the internal autonomous ReAct memory agent engine.
- `agent.max_reasoning_steps`: (`int`, default: `8`) Safety cycle guard limiting the maximum reasoning & tool-call iterations per agent turn.
- `agent.confidence_threshold`: (`float`, default: `0.75`) Minimum model confidence score required to stage memory proposals.
- `agent.auto_apply_safe_links`: (`bool`, default: `false`) When set to `true`, high-confidence (≥0.90) semantic relationship links bypass the proposals queue and are applied immediately.

#### Environment Variable Overrides
All configuration settings can be overridden at runtime via environment variables:

| Environment Variable | Config Key | Description |
|---|---|---|
| `CENTMEM_LLM_BACKEND` | `llm.backend` | Override LLM backend provider (`ollama`, `openai_compatible`, `disabled`) |
| `CENTMEM_LLM_ENDPOINT` | `llm.endpoint` | Override LLM HTTP endpoint URL |
| `CENTMEM_LLM_MODEL` | `llm.model` | Override LLM model name |
| `CENTMEM_LLM_API_KEY` | `llm.api_key` | Override LLM API key or env reference |
| `CENTMEM_LLM_TIMEOUT_SECONDS` | `llm.timeout_seconds` | Override completion request timeout |
| `CENTMEM_LLM_MAX_TOKENS` | `llm.max_tokens` | Override max completion tokens |
| `CENTMEM_LLM_TEMPERATURE` | `llm.temperature` | Override sampling temperature |
| `CENTMEM_AGENT_ENABLED` | `agent.enabled` | Enable/disable agent engine (`true`/`false`) |
| `CENTMEM_AGENT_MAX_REASONING_STEPS` | `agent.max_reasoning_steps` | Override cycle guard step limit |
| `CENTMEM_AGENT_CONFIDENCE_THRESHOLD` | `agent.confidence_threshold` | Override confidence threshold (0.0–1.0) |
| `CENTMEM_AGENT_AUTO_APPLY_SAFE_LINKS` | `agent.auto_apply_safe_links` | Auto-apply safe link proposals (`true`/`false`) |

---

## 9. Operational Commands

### `centmem stats`
Outputs comprehensive store statistics, database file size, row counts partitioned by scope and type, embedding queue state, and the access-frequency importance telemetry distribution.

```bash
centmem stats [--pretty]
```

**Stdout JSON:**
```json
{
  "ok": true,
  "db_path": "/Users/user/.centmem/centmem.db",
  "db_size_mb": 4.2,
  "memories": 1042,
  "by_type": {
    "fact": 30,
    "note": 900,
    "log": 112
  },
  "by_scope": {
    "project:cent-mem": 50,
    "project:my-app": 992
  },
  "last_compact_at": 1788000000,
  "pending_embeddings": 3,
  "importance_distribution": {
    "zero_access": 727,
    "low_access_1_5": 215,
    "medium_access_6_20": 78,
    "high_access_21_plus": 22,
    "max_access_count": 84,
    "avg_access_count": 1.42
  }
}
```

**Importance Distribution Breakdown:**
- `zero_access`: Count of memories never recalled (`access_count == 0`), sitting at neutral baseline weight ($1.000\times$).
- `low_access_1_5`: Count of memories recalled 1 to 5 times (early reinforcement, $+6.9\%$ to $+17.9\%$ score multiplier).
- `medium_access_6_20`: Count of memories recalled 6 to 20 times (established patterns and workflows, $+19.4\%$ to $+30.4\%$ score multiplier).
- `high_access_21_plus`: Count of memories recalled 21+ times (heavily referenced pillars and conventions, approaching the $2.000\times$ cap).
- `max_access_count`: Peak access count attained by any single active memory.
- `avg_access_count`: Arithmetic mean access count across all active memories in the database.

### Other Operational Commands

- `centmem doctor`: Verifies database integrity (`PRAGMA integrity_check`), migration schema version, vector and FTS5 extensions, ONNX embedding model, embedding queue health, directory/database permissions, and AI agent LLM connectivity.
  - **Subsystems inspected:**
    1. `integrity`: SQLite database consistency via `PRAGMA integrity_check`.
    2. `schema_version`: Database migration status (current: v6).
    3. `extensions`: Vector (`sqlite-vec`) and full-text search (`fts5`) extension verification.
    4. `model`: Local ONNX embedding model presence and sha256 checksum validation.
    5. `embed_queue`: Embedding queue processing status and pending un-embedded memory counts.
    6. `permissions`: Asserts directory (`~/.centmem/` 0700) and database (`centmem.db` 0600) permissions.
    7. `ai_agent`: Probes connectivity to configured LLM endpoint (Ollama, OpenAI-compatible, etc.), reporting active endpoint, model name, and roundtrip latency in milliseconds.
  - **Soft Warning Semantics (`ai_agent`)**: When the configured LLM endpoint is unreachable, offline, or timed out, the `ai_agent` check reports `status: "warn"` and appends a warning to `warnings`. It does **not** fail the doctor command (exit code remains 0), ensuring offline CLI memory operations continue uninterrupted. Only fatal subsystem failures (`status: "fail"`) trigger exit code 1.
  - **Example Output (`centmem doctor`):**
    ```json
    {
      "ok": true,
      "checks": [
        {"name": "integrity", "status": "ok", "detail": "PRAGMA integrity_check passed"},
        {"name": "schema_version", "status": "ok", "detail": "version 6"},
        {"name": "extensions", "status": "ok", "detail": "vec, fts5 loaded"},
        {"name": "model", "status": "ok", "detail": "bge-small-en-v1.5 (sha256 verified)"},
        {"name": "embed_queue", "status": "ok", "detail": "0 pending"},
        {"name": "permissions", "status": "ok", "detail": "home 0700; db 0600"},
        {"name": "ai_agent", "status": "ok", "detail": "endpoint=http://127.0.0.1:11434/v1 model=deepseek-r1:8b (42ms)"}
      ],
      "warnings": []
    }
    ```
- `centmem reindex [--all] [--batch N] [--max-time <duration>] [--dry-run]`: Drains the embedding queue and generates dense vector representations in `memories_vec`. Pass `--all` to force complete re-indexing across all active memories.
- `centmem compact [--scope <s>] [--dry-run]`: Consolidates and archives expired notes and logs past their retention threshold into summary notes.
- `centmem backup --to <path>`: Atomic snapshot backup of the SQLite database via `VACUUM INTO`.
- `centmem restore --from <path>`: Restores SQLite database with integrity verification and automatic `.pre-restore.bak` creation.

---

## 10. `centmem ui`
Launches the embedded local web UI Memory Browser dashboard and REST API.

```bash
centmem ui [--port <port>] [--host <host>] [--no-open] [--token <secret>]
```
- `--port`: Port to listen on (default `4231`, or `CENTMEM_UI_PORT`).
- `--host`: Host IP to bind to (default `127.0.0.1`).
- `--no-open`: Do not automatically open the default browser.
- `--token`: Bearer token secret for authentication (or via `CENTMEM_UI_TOKEN`). Mandatory when binding to non-loopback hosts (min 16 chars).

**Stdout JSON:**
```json
{"ok": true, "url": "http://127.0.0.1:4231", "host": "127.0.0.1", "port": 4231, "version": "2.0.2", "auth": false}
```

---

## 11. Memory Relationships: `link`, `unlink`, `links` (v1.5.2)

Establishes and navigates directional semantic links between memories (`supports`, `refines`, `contradicts`, `depends-on`, `supersedes`).

### 11.1 `centmem link`
Creates a confirmed relationship or confirms/dismisses an auto-suggested link.

```bash
# Create confirmed link
centmem link <from_id> <to_id> --relation <rel>

# Confirm auto-suggested link
centmem link confirm <link_id>

# Dismiss auto-suggested link
centmem link dismiss <link_id>
```

**Stdout JSON (`centmem link 42 87 --relation supersedes`):**
```json
{
  "ok": true,
  "link": {
    "id": 12,
    "from_id": 42,
    "to_id": 87,
    "relation": "supersedes",
    "suggested": false,
    "created_at": 1788749000
  }
}
```

### 11.2 `centmem unlink`
Deletes relationships between memories.

```bash
# Delete all links between two memories
centmem unlink <from_id> <to_id>

# Delete specific relation
centmem unlink <from_id> <to_id> --relation <rel>

# Delete by link ID
centmem unlink --id <link_id>
```

**Stdout JSON:**
```json
{"ok": true, "deleted": 1}
```

### 11.3 `centmem links`
Inspects graph edges for a memory.

```bash
centmem links <memory_id> [--all]
```
- `--all`: includes pending auto-suggested links.

**Stdout JSON:**
```json
{
  "ok": true,
  "memory_id": 42,
  "outgoing": [
    {
      "link_id": 12,
      "relation": "supersedes",
      "target_id": 87,
      "target_type": "note",
      "target_content": "Deprecated: custom Python vector index",
      "suggested": false
    }
  ],
  "incoming": []
}
```

---

## 12. Model Context Protocol: `serve` (v1.5.3)

Launches the zero-dependency stdio Model Context Protocol (MCP) JSON-RPC 2.0 server. Exposes centmem memory tools directly to MCP-compliant AI agents (Claude Code, Cursor, Windsurf, Zed, Antigravity) without requiring subshell command wrappers.

```bash
centmem serve [--mcp]
```

- Defaults to MCP stdio mode without requiring flags (`centmem serve` is an alias for `centmem serve --mcp`).
- Protocol framing: Line-delimited JSON-RPC 2.0 over standard input and standard output.
- Concurrency: Thread-safe response serialization, graceful cancellation on `SIGINT`/`SIGTERM`.

### 12.1 Supported JSON-RPC Methods
- `initialize`: Performs MCP capability handshake (`protocolVersion: "2024-11-05"`).
- `notifications/initialized`: Notification emitted by client when ready.
- `ping`: Health probe (returns `{}`).
- `tools/list`: Lists all 7 exposed tools with full JSON schemas.
- `tools/call`: Executes a tool with provided arguments and returns structured tool results.

### 12.2 Exposed MCP Tools

| MCP Tool | Description | Key Arguments |
|---|---|---|
| `centmem_recall` | Hybrid semantic + keyword + fact retrieval with Stage 2 re-ranking | `query` (str, req), `scope` (str), `top` (int), `type` (str), `tags` (array), `reranker` (str), `include_links` (bool) |
| `centmem_put` | Store free-form note or session log memory with auto-links | `content` (str, req), `scope` (str, req), `type` (str), `tags` (array), `no_suggest` (bool) |
| `centmem_set` | Store or update structured key/value fact | `key` (str, req), `value` (any, req), `scope` (str, req), `tags` (array) |
| `centmem_get` | Retrieve fact by key or memory by ID | `id` (int) or `key` (str) + `scope` (str), `inherit` (bool) |
| `centmem_timeline` | Chronological stream of recent memories | `scope` (str), `since` (str), `until` (str), `limit` (int) |
| `centmem_stats` | Memory counts, database size, and status breakdown | `scope` (str) |
| `centmem_forget` | Soft-delete / tombstone memory by ID, key, or tag | `id` (int), `scope` (str), `key` (str), `tag` (str) |

### 12.3 Agent Configuration Example

**Claude Code (`~/.claude/mcp.json` or `.claude.json`):**
```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

**Cursor (`~/.cursor/mcp.json`):**
```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

---

## 13. `centmemd` — Background Daemon & Multi-Process Architecture (v1.5.4)

`centmemd` is the background service binary responsible for single-writer SQLite coordination, connection pooling, background embedding queue draining, periodic retention compaction, and multi-agent IPC over Unix domain sockets.

### 13.1 `centmemd start`
Spawns the daemon in the background detached, logs to `~/.centmem/centmemd.log`, and verifies socket health.

```bash
centmemd start [--home <dir>] [--db <path>] [--socket <path>] [--pid-file <path>] [--port <port>]
```

**Stdout JSON:**
```json
{"ok": true, "status": "started", "pid": 48215, "socket": "/Users/user/.centmem/centmemd.sock", "port": 0}
```

### 13.2 `centmemd run`
Runs the daemon in foreground mode (for `systemd`, `launchd`, Docker).

```bash
centmemd run [--home <dir>] [--db <path>] [--socket <path>] [--pid-file <path>] [--port <port>]
```

### 13.3 `centmemd status`
Inspects daemon health, PID status, and memory statistics via gRPC.

```bash
centmemd status [--home <dir>] [--socket <path>]
```

**Stdout JSON (Running):**
```json
{
  "ok": true,
  "status": "running",
  "pid": 48215,
  "socket": "/Users/user/.centmem/centmemd.sock",
  "port": 0,
  "stats": {
    "db_path": "/Users/user/.centmem/centmem.db",
    "db_size_mb": 1.2,
    "total_memories": 42,
    "by_type": {"note": 30, "fact": 12},
    "by_scope": {"project:cent-mem": 42},
    "pending_embedding": 0,
    "last_compact_at": "2026-09-09T08:00:00Z"
  }
}
```

### 13.4 `centmemd stop`
Sends `SIGTERM` to the daemon PID, allows up to 5s for clean shutdown, unlinks socket and PID files.

```bash
centmemd stop [--home <dir>] [--socket <path>] [--pid-file <path>]
```

**Stdout JSON:**
```json
{"ok": true, "status": "stopped", "pid": 48215}
```

---

## 14. Built-in AI Memory Agent Engine Architecture (v2.0.0 Stage 1)

In v2.0.0, centmem evolves from a passive storage database into an **active intelligence and autonomous curation layer**. Stage 1 establishes the core agent reasoning engine, Schema v6 database tables, internal tool registry adapters, and human-in-the-loop proposals lifecycle.

### 14.1 Schema v6 Specification
Stage 1 applies migration `m0006_agent_proposals.sql`, creating three foundational tables:

1. **`agent_proposals`**: Human-in-the-loop staging queue for autonomous operations:
   - `id`: Unique proposal identifier (`INTEGER PRIMARY KEY AUTOINCREMENT`).
   - `scope_id`: Target scope reference with `ON DELETE CASCADE`.
   - `proposal_type`: Type of proposed operation (`'link'`, `'merge'`, `'update'`, `'archive'`).
   - `status`: Lifecycle state (`'pending'`, `'applied'`, `'dismissed'`).
   - `title`: Short descriptive title summarizing the proposed change.
   - `reasoning`: Detailed reasoning and evidence produced by the reasoning model.
   - `payload_json`: Structured action payload (e.g. source IDs, target ID, relationship type, consolidated memory content).
   - `created_at` & `applied_at`: Unix timestamps in microseconds.

2. **`agent_conversations`**: Multi-turn chat session threads (for Web UI Assistant & CLI interactive inquiry):
   - `id`: Conversation identifier (UUID/NanoID).
   - `scope_id`: Scope isolation boundary.
   - `title`: Auto-generated or user-provided thread title.
   - `created_at` & `updated_at`: Microsecond timestamps.

3. **`agent_messages`**: Dialogue turns within conversation threads:
   - `id`: Message sequence ID.
   - `conversation_id`: References `agent_conversations(id)`.
   - `role`: `'user'`, `'assistant'`, `'system'`, or `'tool'`.
   - `content`: Text response or instruction.
   - `citations_json`: Structured JSON array of memory citations (`[{"id": 42, "title": "...", "score": 0.041}]`).
   - `tool_calls_json`: Recorded tool invocations and intermediate observations.

### 14.2 Built-in ReAct Agent Engine (`internal/agent`)
The agent engine implements an autonomous reasoning loop following the **ReAct (Reasoning + Acting)** paradigm:
- **Plan**: Evaluates query context and decides whether to search, read, inspect graph relationships, or formulate proposals.
- **Act**: Executes one or more registered store tools synchronously.
- **Think**: Consolidates observations into intermediate reasoning steps before responding or staging proposals.
- **Safety Guards**:
  - **Cycle Limiter**: Halts execution if iterations exceed `agent.max_reasoning_steps` (default `8`).
  - **Offline Fallback**: If the configured LLM backend is unreachable or disabled, falls back to deterministic raw hybrid search with actionable setup hints.

### 14.3 Core Agent Tools Exposed to the Engine
The engine operates on 6 structured store adapters in `internal/agent/tools.go`:
1. `search_memories`: Multi-signal hybrid search scoped to the current project/agent hierarchy.
2. `read_memory`: Fetches full content, metadata, timestamps, and access statistics for a specific memory ID.
3. `inspect_links`: Traverses incoming and outgoing semantic graph relationships (`supersedes`, `contradicts`, `refines`, etc.).
4. `propose_link`: Stages a relationship link proposal between two memories with rationale.
5. `propose_merge`: Stages a consolidation proposal merging redundant memories into a canonical note.
6. `detect_knowledge_gaps`: Analyzes retrieved memories to identify missing context or unaddressed questions.

### 14.4 Atomic Proposal Application
When a proposal is approved (manually via CLI/Web UI or autonomously via `agent.auto_apply_safe_links`), `ApplyProposal` executes inside an **atomic SQLite transaction**:
- For `link`: Inserts or updates confirmed records in `memory_links`.
- For `merge`: Creates the new consolidated memory note, links old memories with `supersedes`, and archives the duplicate sources.
- For `archive`: Safely archives stale or obsolete memories.
- Emits audit events to `events` table for multi-agent daemon replication (`centmemd`).

---

## 15. Agent CLI Commands (v2.0.0 Stage 2)

### 15.1 `centmem ask`
Perform natural-language conversational question answering grounded in stored memories.
```bash
centmem ask "<question>" [--scope <scope>] [--top N] [--interactive]
```
- `<question>`: Natural language inquiry.
- `--scope`: Scope path filter (default `global`).
- `--top N`: Maximum candidate citations to retrieve (default `5`, max `20`).
- `--interactive`: Start a multi-turn terminal chat session reading from stdin (supports `/help`, `/clear`, `exit`).

**Output Shape:**
```json
{
  "ok": true,
  "answer": "...",
  "citations": [
    {
      "id": 42,
      "type": "note",
      "scope": "project:cent-mem",
      "snippet": "...",
      "score": 0.0412
    }
  ],
  "knowledge_gaps": [],
  "reasoning_steps": 2,
  "conversation_id": "conv-1234",
  "fallback_used": false
}
```

### 15.2 `centmem curate`
Scan stored memories to autonomously detect contradictions, evolution, and duplicates, staging reviewable proposals into `agent_proposals`.
```bash
centmem curate [--scope <scope>] [--type contradictions|dedup|all] [--apply] [--dry-run]
```
- `--scope`: Scope path filter (default `global`).
- `--type`: Curation category (`contradictions`, `dedup`, or `all`; default `all`).
- `--apply`: Automatically apply generated proposals exceeding confidence threshold.
- `--dry-run`: Simulate curation without persisting proposals to store.

**Output Shape:**
```json
{
  "ok": true,
  "proposals_created": [101, 102],
  "proposals_applied": [],
  "scanned_memories": 120,
  "contradictions_found": 1,
  "duplicates_found": 1,
  "fallback_used": false
}
```

### 15.3 `centmem summarize`
Synthesize scope briefings, developer guides, and architectural pillars.
```bash
centmem summarize [--scope <scope>] [--focus <topic>] [--format markdown|json] [--save]
```
- `--scope`: Scope path filter (default `global`).
- `--focus`: Optional topic or architectural component.
- `--format`: Output format (`json` or `markdown`; default `json`).
- `--save`: Save generated summary as a new `type=note` memory tagged `summary,architecture,digest`.

**Output Shape (`--format json`):**
```json
{
  "ok": true,
  "title": "Architectural Summary — project:cent-mem",
  "summary_markdown": "# Architectural Summary\n...",
  "cited_memory_ids": [14, 25, 33],
  "scope": "project:cent-mem",
  "saved_id": 99,
  "fallback_used": false
}
```

### 15.4 `centmem proposals`
Manage human-in-the-loop staged curation proposals.
```bash
centmem proposals list [--scope <scope>] [--status pending|applied|dismissed] [--limit N] [--offset N]
centmem proposals show <id>
centmem proposals apply <id>
centmem proposals dismiss <id>
```
- `list`: Browse proposals with status/scope filtering and pagination.
- `show <id>`: Inspect detailed proposal metadata, rationale, and JSON payload.
- `apply <id>`: Atomically execute proposal changes.
- `dismiss <id>`: Mark proposal as dismissed.
- Bulk actions: The Web UI and REST API support bulk approval and rejection via `POST /api/proposals/batch` ("Approve All" and "Reject All" with safety confirmation modal dialogs).

**Output (`proposals list`):**
```json
{
  "ok": true,
  "proposals": [
    {
      "id": 101,
      "scope_id": 1,
      "proposal_type": "link",
      "status": "pending",
      "title": "Link memory #12 ──supersedes──► memory #8",
      "reasoning": "Memory #12 updates database architecture decision",
      "payload_json": "{\"from_id\":12,\"to_id\":8,\"relation\":\"supersedes\"}",
      "created_at": 1726050000000000,
      "applied_at": null
    }
  ]
}
```

**Output (`proposals show <id>`):**
```json
{
  "ok": true,
  "proposal": {
    "id": 101,
    "scope_id": 1,
    "proposal_type": "link",
    "status": "pending",
    "title": "Link memory #12 ──supersedes──► memory #8",
    "reasoning": "Memory #12 updates database architecture decision",
    "payload_json": "{\"from_id\":12,\"to_id\":8,\"relation\":\"supersedes\"}",
    "created_at": 1726050000000000,
    "applied_at": null
  }
}
```

**Output (`proposals apply <id>`):**
```json
{
  "ok": true,
  "applied": true,
  "proposal": {
    "id": 101,
    "scope_id": 1,
    "proposal_type": "link",
    "status": "applied",
    "title": "Link memory #12 ──supersedes──► memory #8",
    "reasoning": "Memory #12 updates database architecture decision",
    "payload_json": "{\"from_id\":12,\"to_id\":8,\"relation\":\"supersedes\"}",
    "created_at": 1726050000000000,
    "applied_at": 1726050005000000
  }
}
```

**Output (`proposals dismiss <id>`):**
```json
{
  "ok": true,
  "dismissed": true,
  "proposal": {
    "id": 101,
    "scope_id": 1,
    "proposal_type": "link",
    "status": "dismissed",
    "title": "Link memory #12 ──supersedes──► memory #8",
    "reasoning": "Memory #12 updates database architecture decision",
    "payload_json": "{\"from_id\":12,\"to_id\":8,\"relation\":\"supersedes\"}",
    "created_at": 1726050000000000,
    "applied_at": null
  }
}
```

---

## 16. `centmem scope` — Hierarchical Scope Management

Manage hierarchical memory scopes, inspect scope trees, and cascade-delete scope subtrees across projects, agents, and sessions.

### 16.1 `centmem scope list`
Traverses the database and returns the full hierarchical scope tree starting from root `global`, including direct and descendant memory counts.

```bash
centmem scope list
```

**Output:**
```json
{
  "ok": true,
  "scopes": [
    {
      "id": 1,
      "path": "global",
      "kind": "global",
      "name": "global",
      "count": 5,
      "total_count": 47,
      "children": [
        {
          "id": 2,
          "path": "project:my-app",
          "parent_path": "global",
          "kind": "project",
          "name": "my-app",
          "count": 24,
          "total_count": 42,
          "children": []
        }
      ]
    }
  ]
}
```

### 16.2 `centmem scope delete <path> [--force]`
Deletes the specified non-global scope (project, agent, or session) and **recursively cascade-deletes** all descendant sub-scopes, memories, vector embeddings, queue jobs, memory links, agent proposals, and conversation threads.

```bash
centmem scope delete <path> [--force]
```
- `<path>`: The target scope path to delete (e.g. `project:old-app`, `project:my-app/agent:worker-1`).
- `--force`: Bypasses the interactive confirmation prompt (`Are you sure you want to delete scope "..." (N memories, M sub-scopes)? [y/N]: `). Required in non-interactive environments (CI, subagents, automated scripts).
- **Safety Safeguard**: The root scope `global` cannot be deleted; attempting to do so returns exit code 1 with an error.
- **Exit Codes**: Returns 0 on successful cascade deletion; returns 2 if the target scope is not found; returns 1 if attempting to delete root `global` or if confirmation prompt is unconfirmed/aborted in non-interactive environments.

**Output:**
```json
{
  "ok": true,
  "deleted_scope": "project:old-app",
  "memories_deleted": 42,
  "scopes_deleted": 5
}
```

---

## 17. Web UI Experience: Assistant Chat, Proposals Review & Scope Management (v2.0.0 Stage 3)

Stage 3 introduces a full-featured browser workspace embedded directly in `centmem ui`, connecting the agent reasoning loop, proposals inbox, and scope hierarchy to an interactive interface:

### 17.1 Interactive Assistant Chat (`AssistantTab.tsx`)
- **Server-Sent Events (SSE) Streaming**: Natural-language conversational inquiry streamed in real time via `POST /api/agent/chat`.
- **Grounded Citations**: Interactive citation cards displaying source memory IDs, scope tags, relevance scores, and direct snippet previews.
- **Knowledge Gap Detection**: Alerts highlighting missing, ambiguous, or contradictory domain context that agents or users should record.
- **Thread Management**: Conversation switching, multi-turn history tracking, and scoped conversations managed via `/api/agent/conversations`.

### 17.2 Proposals Review Center (`ProposalsView.tsx`)
- **Human-in-the-Loop Inbox**: Review queue for proposals generated by `centmem curate`, autonomous background workers, or external agent tools.
- **Visual Merge Diffs**: Side-by-side diff comparing redundant source memories with synthesized consolidated notes.
- **Relationship Link Previews**: Direct inspection of proposed graph relationships (`supersedes`, `contradicts`, `refines`, `depends-on`, `supports`) before applying.
- **1-Click Review Actions**:
  - Apply: Atomically executes proposal changes and updates status to `applied`.
  - Dismiss: Marks proposal as `dismissed`.
  - Reopen: Restores a dismissed proposal back to `pending`.
- **Bulk Proposal Management**:
  - "Approve All" and "Reject All" header buttons for processing all pending proposals in the active filter.
  - Safe warning popup dialog confirming batch execution before dispatching `POST /api/proposals/batch`.

### 17.3 Scope Hierarchy & Subtree Deletion (`ScopeTree.tsx`, `ScopeOverview.tsx`)
- **Interactive Tree Navigation**: Expandable tree structure showing direct and descendant memory counts per scope.
- **Subtree Deletion**: "Delete Scope" button in `ScopeOverview` and trash can action on hovering tree nodes.
- **Type-to-Confirm Dialog**: Safe deletion modal requiring the user to type the exact scope path to confirm cascading deletion (`DELETE /api/scopes?path=<scope>`). Root `global` scope deletion is disallowed.

### 17.4 Doctor Diagnostics & Agent Settings (`TopBar.tsx`, `AgentTab.tsx`)
- **Doctor Diagnostics Popover**: Header badge in TopBar providing instant health check status across all 7 subsystems, including AI agent LLM connectivity.
- **Live Agent Connection Probe**: Dedicated "Test Connection" button in Agent Settings (`POST /api/config/test-agent`) validating endpoint responsiveness, model availability, and latency without saving changes.

### 17.5 Embedded REST & SSE API Reference
The embedded HTTP server exposes the following endpoints for UI and programmatic agent integration:

| Method | Endpoint | Request Body / Query Params | Description |
|--------|----------|-----------------------------|-------------|
| `POST` | `/api/agent/chat` | JSON: `{"conversation_id": "...", "message": "...", "scope": "...", "top": 5}` | Streams SSE events (`delta`, `citations`, `gaps`, `done`, `error`) |
| `GET` | `/api/agent/conversations` | Query: `scope`, `limit` | List persisted conversation threads |
| `GET` | `/api/agent/conversations/{id}/messages` | Path: `id` | Retrieve all chronological messages and citations in a conversation |
| `GET` | `/api/proposals` | Query: `scope`, `status`, `type`, `limit`, `offset` | List proposals filtered by status (`pending`, `applied`, `dismissed`) and type (`merge`, `link`, `update`) |
| `POST` | `/api/proposals/{id}/apply` | Path: `id` | Atomically apply proposal to SQLite store |
| `POST` | `/api/proposals/{id}/dismiss` | Path: `id` | Mark proposal as dismissed |
| `POST` | `/api/proposals/{id}/reopen` | Path: `id` | Undo dismissal and restore proposal to pending status |
| `POST` | `/api/proposals/batch` | JSON: `{"action": "apply"\|"dismiss", "ids": [1, 2]}` | Batch apply or dismiss up to 500 proposals atomically |
| `GET` | `/api/scopes` | None | Retrieve hierarchical scope tree with memory counts |
| `POST` | `/api/scopes` | JSON: `{"path": "..."}` | Explicitly create a new scope node |
| `DELETE` | `/api/scopes` | Query: `path=<scope>` or JSON: `{"path": "..."}` | Cascade-delete a scope and all descendant subtrees and memories |
| `POST` | `/api/config/test-agent` | JSON: `{"backend": "...", "endpoint": "...", "model": "...", "api_key": "...", "timeout_seconds": 8}` | Test live AI agent endpoint connectivity and latency |

#### SSE Event Format (`POST /api/agent/chat`)
- `event: delta`: Streaming text tokens `{"content": "text", "role": "assistant"}`
- `event: citations`: Array of grounded source citations `[{"id": 42, "type": "note", "scope": "...", "snippet": "...", "score": 0.0412}]`
- `event: gaps`: Array of missing information alerts `["Missing deploy documentation for staging"]`
- `event: done`: Completion metadata `{"conversation_id": "conv-1", "reasoning_steps": 2, "fallback_used": false}`
- `event: error`: Error payload `{"error": "error message"}`

---

## 18. Roadmap & Upcoming Milestones (Stage 4+)
With Stage 1 (Agent Engine & Proposals), Stage 2 (CLI Command Suite), and Stage 3 (Web UI Experience, Proposals Inbox & Scope Management) complete:
- **Stage 4**: Background Autonomous Daemon Curation loop (`centmemd`) with idle memory scanning, periodic conflict resolution, and background proposal staging.
