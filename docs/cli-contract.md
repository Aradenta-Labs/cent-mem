# CLI & API Contract: centmem

**Version:** 1.5.2
**Binary:** `centmem`
**Output default:** JSON to stdout; errors to stderr. Use `--pretty` for human-readable output.

---

## 1. Global Flags

| Flag | Description |
|------|-------------|
| `--home <dir>` | Override `CENTMEM_HOME` (default `~/.centmem`) |
| `--db <path>` | Override DB path |
| `--pretty` | Pretty-print JSON |
| `--verbose` | Debug logging to stderr |
| `--json` | Force JSON (default) |
| `--quiet` | Suppress non-essential stderr |

## 2. Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic error (see stderr JSON) |
| 2 | Not found |
| 3 | Conflict (duplicate fact key, version mismatch) |

**Error JSON shape** (stderr):
```json
{"error": {"code": "NOT_FOUND", "message": "...", "hint": "..."}}
```

## 3. Commands

### 3.1 `init`
Create/open the store and download the embedding model if missing.

```
centmem init [--model bge-small-en-v1.5] [--force]
```

**Output:**
```json
{"ok": true, "db": "/Users/x/.centmem/centmem.db", "model": "bge-small-en-v1.5", "dims": 384}
```

---

### 3.2 `put` — write a note/log memory
```
centmem put --scope <scope> --type note|log --content <text> [--tags a,b] [--source-agent claude] [--source-session s1] [--no-suggest]
```
- `--scope` format: `global`, `project:<name>`, `project:<name>/agent:<agent>`, `project:<name>/agent:<agent>/session:<id>`. Auto-creates scope + ancestors.
- `--type log` appends to chronological log; `note` is a free-text memory.
- `--no-suggest`: bypass post-write relationship auto-suggestion. When omitted, candidate relationships are evaluated and returned in `"suggested_links": [...]` if detected.

**Output:**
```json
{"ok": true, "id": 123, "scope": "project:cent-mem", "status": "queued"}
```

---

### 3.3 `set` — upsert a fact
```
centmem set --scope <scope> --key user.timezone --value '"Asia/Jakarta"' [--tags user]
```
- `--value` is JSON (string/number/object/array/bool). Non-JSON is stored as string.
- Upsert within scope; returns `status: "created" | "updated"`.

**Output:**
```json
{"ok": true, "id": 124, "key": "user.timezone", "status": "created"}
```

---

### 3.4 `get` — fetch fact by key
```
centmem get --scope <scope> --key user.timezone [--inherit]
```
- `--inherit` (default true) walks ancestor scopes if key missing in target scope.

**Output:**
```json
{"ok": true, "key": "user.timezone", "value": "Asia/Jakarta", "scope": "global", "id": 9}
```
Not found → exit 2.

---

### 3.5 `recall` — hybrid search (THE main read command)
```
centmem recall <query> --scope <scope> [--top 5] [--type note|fact|log] [--tags a,b] [--since 7d] [--until 1d] [--agent claude] [--inherit] [--children] [--caller-agent a] [--reranker r] [--include-links] [--include-suggested]
```

- `--inherit` (default true): include ancestor scopes (global).
- `--children`: include descendant scopes (agents/sessions under a project).
- `--top N` default 5, max 20.
- `--caller-agent <name>`: calling agent identifier for affinity boosting (+15% when matching author). Defaults to `$CENTMEM_AGENT`.
- `--reranker <strategy>`: re-ranker strategy override (`composite`, `none`, `cross_encoder`, `llm`). Defaults to `composite`.
- `--include-links`: enrich results with 1-hop connected memory relationships (`links`). Returns confirmed links by default.
- `--include-suggested`: include auto-suggested links pending confirmation in `links` expansion.

**Output:**
```json
{
  "ok": true,
  "query": "how do we deploy",
  "results": [
    {
      "id": 42,
      "type": "note",
      "scope": "project:cent-mem",
      "content": "We deploy via GitHub Actions to Fly.io after merge.",
      "tags": ["deploy", "ci"],
      "source_agent": "claude",
      "created_at": 1788000000,
      "score": 0.87,
      "matched_by": ["semantic", "keyword"],
      "access_count": 18,
      "last_accessed_at": 1788748000
    }
  ]
}
```

---

### 3.6 `timeline` — chronological logs
```
centmem timeline --scope <scope> [--since 24h] [--until 0] [--limit 50]
```

**Output:**
```json
{"ok": true, "entries": [{"id": 7, "content": "...", "created_at": 1788000000, "scope": "...", "tags": []}]}
```

---

### 3.7 `list` — browse memories
```
centmem list --scope <scope> [--type note] [--tags a,b] [--limit 20] [--offset 0]
```

---

### 3.8 `forget` — delete a memory
```
centmem forget --id 123 [--scope <scope> --key k] [--scope <scope> --tag tag] [--all-archived]
```
Deletes by id, or by scope+key, or by scope+tag. Archived rows are hard-deleted.

---

### 3.9 `stats`
```
centmem stats
```
**Output:**
```json
{"ok": true, "db_path": "...", "db_size_mb": 4.2, "memories": 1042, "by_type": {"fact": 30, "note": 900, "log": 112}, "by_scope": {...}, "last_compact_at": 1788000000, "pending_embeddings": 3, "importance_distribution": {"zero_access": 1105, "low_access_1_5": 215, "medium_access_6_20": 78, "high_access_21_plus": 22, "max_access_count": 84, "avg_access_count": 1.42}}
```

---

### 3.10 `doctor`
```
centmem doctor
```
Integrity check, model check, config check. Exit 0 healthy.

---

### 3.11 `backup`
```
centmem backup --to ~/backups/centmem-$(date +%F).db
```
`VACUUM INTO`. Output: `{"ok": true, "backup": "<path>", "size_mb": 4.2}`.

---

### 3.12 `compact` — summarize + archive old memories
```
centmem compact [--scope <scope>] [--dry-run]
```
Summarizes eligible memories (past their `summarize_at`) into consolidated
`note` rows and archives the originals. `--dry-run` reports what would happen
without writing. Output: `{"ok": true, "summarized": N, "archived": N, "new_memory_ids": [...], "dry_run": false}`.

---

### 3.13 `restore`
```
centmem restore --from path.db
```
Verifies the backup (integrity check), writes a `.pre-restore.bak` safety copy
of the current DB, then replaces the live DB. Output: `{"ok": true, "restored_from": "...", "pre_restore_bak": "..."}`.

---

### 3.14 `capture`
Automatic transcript capture, session summaries, category management, format normalization, and developer artifact capture (Git, docs, shell, comments).

```
centmem capture run [--transcript <path>] [--scope <scope>] [--harness <name>]
centmem capture summary [--session <id>]
centmem capture categories [--list] [--add <c>] [--remove <c>]
centmem capture convert --harness <name> --input <path> [--output <path>]
centmem capture git [--repo <path>] [--since <sha|date>] [--scope <scope>] [--dry-run] [--max-commits <n>]
centmem capture docs [--dir <path>] [--scope <scope>] [--ext md,txt,rst] [--dry-run]
centmem capture shell [--history <path>] [--shell <zsh|bash|fish>] [--scope <scope>] [--top <n>] [--dry-run]
centmem capture comments [--dir <path>] [--ext go,ts,js,py,rs,sh] [--keywords TODO,FIXME...] [--scope <scope>] [--dry-run]
```

- `run`: processes a transcript file or stdin pipe, classifies items, deduplicates, writes to store, and outputs `CaptureSummary` JSON.
- `summary`: prints the last (or specified session's) capture summary as JSON. Exit 2 if not found.
- `categories`: inspects or updates the active capture categories list.
- `convert`: normalizes a harness transcript file into standard `.centmem.jsonl` format.
- `git`: incrementally extracts architectural decisions and dependency facts from Git commits using `.centmem/git-cursor`.
- `docs`: indexes markdown, plaintext, and RST documentation into heading-based chunks with mtime caching and tombstoning.
- `shell`: extracts top frequent shell toolchain commands from zsh, bash, or fish history with high-entropy secret scrubbing.
- `comments`: scans source code for actionable annotations (TODO, FIXME, HACK, SECURITY) with line-shift tracking.

**Output (`capture run` / `capture summary`):**
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

**Output (`capture convert`):**
```json
{"ok": true, "output": "/path/to/normalized.jsonl", "messages": 42, "harness": "cursor"}
```

**Output (`capture categories`):**
```json
{"ok": true, "categories": ["decision", "fact", "preference", "code", "log", "error", "dependency"]}
```

**Output (`capture git` / `capture docs` / `capture shell` / `capture comments`):**
```json
{
  "ok": true,
  "command": "capture git",
  "source": ".git",
  "scanned": 24,
  "commits_scanned": 24,
  "memories_created": 3,
  "memories_updated": 0,
  "cursor": "a8f3bc1994d8721c0e352b9921",
  "items": [
    {
      "id": 149,
      "type": "note",
      "tags": ["git", "commit", "decision"],
      "content": "Commit a8f3bc1: Switch SQLite to WAL journal mode to improve multi-process concurrency.",
      "dry_run": false
    }
  ]
}
```

---

### 3.15 `config`
Read or update configuration in `~/.centmem/config.toml`.

```
centmem config set <key> <value>
centmem config get [key]
```

- `set`: updates a dot-notation config key in `config.toml` (e.g. `capture.harness`, `capture.categories`, `retention.note_summarize_after_days`).
- `get`: retrieves the value of a specific key (or full configuration if no key specified). Exit 2 if key not found.

**Output (`config set` / `config get <key>`):**
```json
{"ok": true, "key": "capture.harness", "value": "claude-code"}
```

**Output (`config get` full dump):**
```json
{"ok": true, "config": {"home": "/Users/x/.centmem", "model": {"name": "bge-small-en-v1.5", "dims": 384}, "capture": {"enabled": true, "harness": "claude-code"}}}
```

---

### 3.16 `ui` — Web UI Memory Browser dashboard
Launch the embedded local web UI dashboard and health API.

```
centmem ui [--port <port>] [--host <host>] [--no-open]
```

- `--port`: port to listen on (default `4231`, or `CENTMEM_UI_PORT`).
- `--host`: host IP to bind to (default `127.0.0.1`).
- `--no-open`: do not automatically open the browser.

**Output:**
```json
{"ok": true, "url": "http://127.0.0.1:4231", "host": "127.0.0.1", "port": 4231, "version": "1.4.4"}
```

---

### 3.17 `reindex` — re-embed memories into vector index
Drains the embedding queue and updates dense vector representations in `memories_vec`.

```
centmem reindex [--all] [--batch 32] [--max-time 30s] [--dry-run]
```

- `--all`: re-enqueues all active memories into `embed_queue`, even if already indexed in `memories_vec`.
- `--batch <int>`: batch size per inference iteration (default 32).
- `--max-time <duration>`: maximum execution time before yielding (default 30s; 0 for unlimited).
- `--dry-run`: reports pending embeddings count without processing.

**Output:**
```json
{
  "ok": true,
  "reindexed": 142,
  "pending": 0,
  "duration_ms": 1284,
  "model": "bge-small-en-v1.5"
}
```

---

### 3.18 `link` — create or manage memory relationships
Create a confirmed relationship or confirm/dismiss auto-suggested links.

```
centmem link <from_id> <to_id> --relation <rel>
centmem link confirm <link_id>
centmem link dismiss <link_id>
```

- `--relation`: relationship type (`supports`, `refines`, `contradicts`, `depends-on`, `supersedes`). Required when creating a link.
- `link confirm <link_id>`: promote an auto-suggested link (`suggested = 1`) to confirmed (`suggested = 0`).
- `link dismiss <link_id>`: remove a pending auto-suggested link.

**Output (`centmem link 42 87 --relation supersedes`):**
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

---

### 3.19 `unlink` — delete memory relationships
Remove relationship links between two memories or by primary key link ID.

```
centmem unlink <from_id> <to_id> [--relation <rel>]
centmem unlink --id <link_id>
```

- `--relation`: optionally restrict deletion to a specific relation type. When omitted, all links between `<from_id>` and `<to_id>` are removed.
- `--id <link_id>`: delete a specific link by its primary key ID.

**Output:**
```json
{
  "ok": true,
  "deleted": 1
}
```

---

### 3.20 `links` — inspect memory relationships
List outgoing and incoming relationship graph edges for a given memory.

```
centmem links <memory_id> [--all] [--include-suggested]
```

- `--all` / `--include-suggested`: include auto-suggested links pending confirmation (defaults to confirmed links only).

**Output (`centmem links 42`):**
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
      "target_content": "Use custom vector store",
      "suggested": false
    }
  ],
  "incoming": []
}
```

---

## 4. Scope Grammar

```
scope := "global"
       | "project:" NAME
       | "project:" NAME "/agent:" NAME
       | "project:" NAME "/agent:" NAME "/session:" NAME
NAME  := [a-z0-9-_.]+   (case-insensitive, stored lowercase)
```

Examples:
- `global`
- `project:cent-mem`
- `project:cent-mem/agent:claude`
- `project:cent-mem/agent:codex/session:sess_abc123`

## 5. Canonical Agent Recipes

These are the exact sequences agents should use (mirrored in SKILL.md):

**At session start — load context:**
```bash
centmem recall "<task summary>" --scope "project:$PROJ" --top 5
centmem get --scope "project:$PROJ" --key "project.conventions" --inherit
```

**During work — store learnings:**
```bash
centmem put --scope "project:$PROJ" --type note --content "..." --tags decision,db --source-agent "$AGENT" --source-session "$SID"
centmem set --scope "project:$PROJ" --key "api.base_url" --value '"https://api.example.com"'
```

**At session end — checkpoint:**
```bash
centmem put --scope "project:$PROJ/agent:$AGENT/session:$SID" --type log --content "session summary..." --tags checkpoint
```

## 6. Stability Guarantees

- JSON field names and exit codes are **stable within v1.x**.
- New fields may be added (additive only). No removals without v2.
- `--pretty` is for humans only; agents must parse default JSON.
