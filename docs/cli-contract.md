# CLI & API Contract: centmem

**Version:** 2.0.3
**Binary:** `centmem`, `centmemd`
**Output default:** JSON to stdout; errors to stderr. Use `--pretty` for human-readable output.

---

## 1. Global Flags

| Flag | Description |
|------|-------------|
| `--home <dir>` | Override `CENTMEM_HOME` (default `~/.centmem`) |
| `--db <path>` | Override DB path |
| `--direct` | Bypass `centmemd` daemon IPC and execute directly on SQLite (or `CENTMEM_DIRECT=1`) |
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
Inspects system health across all 7 operational subsystems: database integrity (`PRAGMA integrity_check`), schema migration version, native vector/FTS5 extensions, local ONNX embedding model, embed queue, directory/file permissions, and AI agent LLM connectivity.
- Exit 0 if all checks succeed (`status: "ok"`) or contain warnings only (`status: "warn"`).
- Soft warning semantics for `ai_agent`: when the configured LLM endpoint is unreachable, offline, or timed out, `ai_agent` reports `status: "warn"` and appends an entry to `warnings` without failing the command (exit code 0).
- Exit 1 only if a fatal subsystem check fails (`status: "fail"`).

**Output:**
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
centmem ui [--port <port>] [--host <host>] [--no-open] [--token <secret>]
```

- `--port`: port to listen on (default `4231`, or `CENTMEM_UI_PORT`).
- `--host`: host IP to bind to (default `127.0.0.1`).
- `--no-open`: do not automatically open the browser.
- `--token`: bearer token secret for API authentication (or via `CENTMEM_UI_TOKEN`). Mandatory when binding to non-loopback hosts (min 16 chars).

**Output:**
```json
{"ok": true, "url": "http://127.0.0.1:4231", "host": "127.0.0.1", "port": 4231, "version": "2.0.3", "auth": false}
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

### 3.21 `serve` — Model Context Protocol (MCP) server
Run a zero-dependency stdio Model Context Protocol (MCP) JSON-RPC 2.0 server exposing centmem tools directly to AI coding agents. Defaults to MCP mode without requiring flags.

```
centmem serve [--mcp]
```

- `--mcp`: enable MCP stdio protocol mode (default `true`).

**Supported Methods:** `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`.  
**Exposed Tools:** `centmem_recall`, `centmem_put`, `centmem_set`, `centmem_get`, `centmem_timeline`, `centmem_stats`, `centmem_forget`.

---

### 3.22 `ask` — conversational inquiry & citations
Ask a question grounded in stored memories using ReAct reasoning, citation provenance, and gap detection.

```
centmem ask "<question>" [--scope <scope>] [--top N] [--interactive]
```
- `--scope`: scope path filter (default `global`).
- `--top`: maximum number of candidate citations to inspect (default `5`).
- `--interactive`: enter interactive multi-turn terminal chat session reading from stdin.

**Output (Single-shot):**
```json
{
  "ok": true,
  "answer": "We use SQLite with sqlite-vec for 100% offline agent memory...",
  "citations": [
    {
      "id": 42,
      "type": "note",
      "scope": "project:cent-mem",
      "snippet": "Chose SQLite-vec with local ONNX embeddings...",
      "score": 0.0412
    }
  ],
  "knowledge_gaps": [],
  "reasoning_steps": 2,
  "conversation_id": "conv-1234",
  "fallback_used": false
}
```

---

### 3.23 `curate` — autonomous memory curation
Scan memories to proactively detect contradictions, evolution, and semantic duplicates, staging reversible human-in-the-loop proposals into the `agent_proposals` table.

```
centmem curate [--scope <scope>] [--type contradictions|dedup|all] [--apply] [--dry-run]
```
- `--scope`: scope path filter (default `global`).
- `--type`: curation category (`contradictions`, `dedup`, or `all`; default `all`).
- `--apply`: automatically apply proposals exceeding confidence threshold.
- `--dry-run`: simulate curation without staging proposals to store.

**Output:**
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

---

### 3.24 `summarize` — scope briefings & synthesis
Synthesize architectural pillars, conventions, and developer guides from memories in a target scope.

```
centmem summarize [--scope <scope>] [--focus <topic>] [--format markdown|json] [--save]
```
- `--scope`: scope path filter (default `global`).
- `--focus`: optional focus topic or component.
- `--format`: output format (`json` or `markdown`; default `json`).
- `--save`: save synthesized summary as a new note memory tagged `summary,architecture,digest`.

**Output (`--format json`):**
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

---

### 3.25 `proposals` — human-in-the-loop staged proposals
Manage staged agent curation proposals (`agent_proposals` table).

```
centmem proposals list [--scope <scope>] [--status pending|applied|dismissed] [--limit N] [--offset N]
centmem proposals show <id>
centmem proposals apply <id>
centmem proposals dismiss <id>
```
- `list`: browse staged proposals with optional status filter.
- `show <id>`: display full proposal details and payload.
- `apply <id>`: atomically execute proposal actions (e.g. merge memories or establish relationship link).
- `dismiss <id>`: dismiss proposal without modifying memories.
- Bulk actions: The Web UI and REST API support bulk approval and rejection via `POST /api/proposals/batch` ("Approve All" and "Reject All").

**Output (`proposals list`):**
```json
{
  "ok": true,
  "proposals": [
    {
      "id": 101,
      "scope_id": 1,
      "scope_path": "project:cent-mem",
      "proposal_type": "link",
      "status": "pending",
      "title": "Link memory #12 ──supersedes──► memory #8",
      "reasoning": "Memory #12 updates database architecture decision",
      "payload_json": "{\"from_id\":12,\"to_id\":8,\"relation\":\"supersedes\"}",
      "created_at": "2026-09-09T12:00:00Z"
    }
  ]
}
```

**Output (`proposals apply <id>`):**
```json
{
  "ok": true,
  "applied": true,
  "proposal": {
    "id": 101,
    "status": "applied",
    "applied_at": "2026-09-09T12:05:00Z"
  }
}
```

---

### 3.26 `scope` — hierarchical scope management
Manage hierarchical memory scopes and cascade-delete scope subtrees.

```
centmem scope delete <path> [--force]
centmem scope list
```
- `delete <path>`: Deletes the specified non-global scope (project, agent, or session) and all descendant scopes, memories, vector embeddings, queue jobs, memory links, proposals, and conversations.
- `--force`: Bypass interactive confirmation prompt.
- Root scope `global` cannot be deleted.
- Exit code 0 on success; exit code 2 if target scope does not exist; exit code 1 if deleting `global` or confirmation is unconfirmed/aborted.

**Output (`scope delete <path>`):**
```json
{
  "ok": true,
  "deleted_scope": "project:foo",
  "memories_deleted": 12,
  "scopes_deleted": 3
}
```

**Output (`scope list`):**
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
          "path": "project:foo",
          "parent_path": "global",
          "kind": "project",
          "name": "foo",
          "count": 24,
          "total_count": 42,
          "children": []
        }
      ]
    }
  ]
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

---

## 7. Daemon Service (`centmemd`)

`centmemd` is the companion background service binary providing single-writer SQLite transaction coordination, connection pooling, background embedding queue draining, periodic retention compaction, and multi-agent IPC over Unix domain sockets.

When `centmemd` is running, the `centmem` CLI automatically delegates operations (`put`, `set`, `get`, `recall`, `timeline`, `list`, `forget`, `stats`, `compact`, `link`, `unlink`, `links`) through the daemon socket, falling back transparently to direct SQLite if the daemon is unavailable or stopped.

### Lifecycle Management

- **`centmemd start`**: Spawns the daemon process in the background detached, redirects logs to `centmemd.log`, and verifies socket health.
  ```
  centmemd start [--home <dir>] [--db <path>] [--socket <path>] [--pid-file <path>] [--port <port>] [--pretty]
  ```
  **Output:**
  ```json
  {"ok": true, "status": "started", "pid": 12345, "socket": "/Users/x/.centmem/centmemd.sock", "port": 0}
  ```
  If already running:
  ```json
  {"ok": true, "status": "already_running", "pid": 12345, "socket": "/Users/x/.centmem/centmemd.sock", "port": 0}
  ```

- **`centmemd run`**: Runs the daemon in the foreground. Traps `SIGTERM` and `SIGINT` to flush queues and shut down gracefully. Recommended for process supervisors (`launchd`, `systemd`) or Docker containers.
  ```
  centmemd run [--home <dir>] [--db <path>] [--socket <path>] [--pid-file <path>] [--port <port>]
  ```

- **`centmemd status`**: Probes the daemon socket, checks PID status, and queries current memory statistics via gRPC.
  ```
  centmemd status [--home <dir>] [--socket <path>] [--pid-file <path>] [--pretty]
  ```
  **Output (Running):**
  ```json
  {
    "ok": true,
    "status": "running",
    "pid": 12345,
    "socket": "/Users/x/.centmem/centmemd.sock",
    "port": 0,
    "stats": {
      "db_path": "/Users/x/.centmem/centmem.db",
      "db_size_mb": 1.2,
      "total_memories": 42,
      "by_type": {"note": 30, "fact": 12},
      "by_scope": {"project:cent-mem": 42},
      "pending_embedding": 0,
      "last_compact_at": "2026-09-09T08:00:00Z"
    }
  }
  ```
  **Output (Stopped):**
  ```json
  {"ok": false, "status": "stopped", "socket": "/Users/x/.centmem/centmemd.sock"}
  ```

- **`centmemd stop`**: Signals `SIGTERM` to the daemon PID, waits up to 5s for clean shutdown, unlinks socket and PID files.
  ```
  centmemd stop [--home <dir>] [--socket <path>] [--pid-file <path>] [--pretty]
  ```
  **Output:**
  ```json
  {"ok": true, "status": "stopped", "pid": 12345}
  ```
  If already stopped:
  ```json
  {"ok": true, "status": "already_stopped"}
  ```
