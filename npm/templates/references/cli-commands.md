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
  [--source-session <session-id>]
```
- `--scope`: Required scope path (e.g. `project:my-app`).
- `--type`: `note` (default) or `log`.
- `--content`: Text content to store.
- `--tags`: Comma-separated tags (lowercase, no spaces).
- `--source-agent`: Agent identifier (e.g., `claude`, `antigravity`).
- `--source-session`: Session or conversation identifier.

**Stdout JSON:**
```json
{"ok":true,"id":42,"scope":"project:my-app","status":"queued"}
```

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
  [--children]
```
- `--top`: Maximum items to return (default: `5`, max: `20`).
- `--inherit`: Search ancestor scopes (default: `true`).
- `--children`: Include descendant scopes in search.
- `--caller-agent`: Calling agent identifier for affinity boosting (+15% when matching author; defaults to `$CENTMEM_AGENT`).
- `--reranker`: Re-ranker strategy override (`composite`, `none`, `cross_encoder`, `llm`; defaults to `composite`).

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
      "created_at": 1788000000
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

- `centmem doctor`: Verifies database integrity (`PRAGMA integrity_check`), ONNX model availability, vector extensions, directory permissions, and embedding queue health.
- `centmem reindex [--all] [--batch N] [--max-time <duration>] [--dry-run]`: Drains the embedding queue and generates dense vector representations in `memories_vec`. Pass `--all` to force complete re-indexing across all active memories.
- `centmem compact [--scope <s>] [--dry-run]`: Consolidates and archives expired notes and logs past their retention threshold into summary notes.
- `centmem backup --to <path>`: Atomic snapshot backup of the SQLite database via `VACUUM INTO`.
- `centmem restore --from <path>`: Restores SQLite database with integrity verification and automatic `.pre-restore.bak` creation.

---

## 10. `centmem ui`
Launches the embedded local web UI Memory Browser dashboard and REST API.

```bash
centmem ui [--port <port>] [--host <host>] [--no-open]
```
- `--port`: Port to listen on (default `4231`, or `CENTMEM_UI_PORT`).
- `--host`: Host IP to bind to (default `127.0.0.1`).
- `--no-open`: Do not automatically open the default browser.

**Stdout JSON:**
```json
{"ok": true, "url": "http://127.0.0.1:4231", "host": "127.0.0.1", "port": 4231, "version": "1.5.0"}
```
