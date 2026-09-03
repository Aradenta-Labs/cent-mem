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
Hybrid search engine fusing vector semantic similarity, SQLite FTS5 keyword matching, key/value fact lookup, and recency scoring via Reciprocal Rank Fusion (RRF, k=60).

```bash
centmem recall "<query>" \
  --scope <scope> \
  --top <N> \
  [--type <note|log|fact>] \
  [--tags <tag1,tag2>] \
  [--since <duration|date>] \
  [--until <duration|date>] \
  [--agent <name>] \
  [--inherit] \
  [--children]
```
- `--top`: Maximum items to return (default: `5`).
- `--inherit`: Search ancestor scopes (default: `true`).
- `--children`: Include descendant scopes in search.

**Stdout JSON:**
```json
{
  "ok": true,
  "query": "how do we deploy?",
  "scope": "project:my-app",
  "memories": [
    {
      "id": 12,
      "type": "note",
      "content": "We deploy via GitHub Actions to Fly.io",
      "tags": ["deploy", "ci"],
      "score": 0.0328,
      "created_at": "2026-09-01T15:00:00Z"
    }
  ]
}
```

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
  "scope": "project:my-app",
  "memories": [...]
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
centmem config get <key>
centmem config set <key> <value>
```

---

## 9. Operational Commands

- `centmem doctor`: Verifies database integrity, model availability, vector extensions, permissions, and embedding queue health.
- `centmem stats`: Outputs store statistics (row counts per type, DB file size).
- `centmem compact [--scope <s>] [--dry-run]`: Consolidates and archives expired notes and logs.
- `centmem backup --to <path>`: Atomic snapshot backup of the SQLite database.
- `centmem restore --from <path>`: Restores SQLite database with safety backup creation.
- `centmem ui [--port <port>] [--host <host>] [--no-open]`: Launches the embedded local Web UI memory browser dashboard.
