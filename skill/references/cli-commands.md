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
Subcommands for the auto-capture transcript system:

- `centmem capture run [--transcript <path>] [--scope <s>] [--watch]`
  Executes an on-demand or background continuous capture run.
- `centmem capture summary [--session <id>]`
  Returns telemetry and captured memories from the latest or specified session.
- `centmem capture categories [--list] [--add <category>] [--remove <category>]`
  Inspects or manages active extraction categories.
- `centmem capture convert --harness <name> --input <path> [--output <path>]`
  Normalizes harness-specific session logs to cent-mem standard JSONL.

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
