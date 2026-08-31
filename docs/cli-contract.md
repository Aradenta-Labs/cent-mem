# CLI & API Contract: centmem

**Version:** 1.0
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
centmem put --scope <scope> --type note|log --content <text> [--tags a,b] [--source-agent claude] [--source-session s1] [--ttl 30d]
```
- `--scope` format: `global`, `project:<name>`, `project:<name>/agent:<agent>`, `project:<name>/agent:<agent>/session:<id>`. Auto-creates scope + ancestors.
- `--type log` appends to chronological log; `note` is a free-text memory.

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
centmem recall <query> --scope <scope> [--top 5] [--type note|fact|log] [--tags a,b] [--since 7d] [--until 1d] [--agent claude] [--inherit] [--children]
```

- `--inherit` (default true): include ancestor scopes (global).
- `--children`: include descendant scopes (agents/sessions under a project).
- `--top N` default 5, max 20.

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
      "matched_by": ["semantic", "keyword"]
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

### 3.7 `compact` — retention/summarization
```
centmem compact [--scope <scope>] [--dry-run]
```
Summarizes eligible memories (`summarize_at <= now`), archives originals, inserts consolidated note.

**Output:**
```json
{"ok": true, "summarized": 14, "archived": 14, "new_memory_ids": [201]}
```

---

### 3.8 `list` — browse memories
```
centmem list --scope <scope> [--type note] [--tags a,b] [--limit 20] [--offset 0]
```

---

### 3.9 `forget` — delete a memory
```
centmem forget --id 123 [--scope <scope> --key k] [--scope <scope> --tag tag] [--all-archived]
```
Deletes by id, or by scope+key, or by scope+tag. Archived rows are hard-deleted.

---

### 3.10 `stats`
```
centmem stats
```
**Output:**
```json
{"ok": true, "db_path": "...", "db_size_mb": 4.2, "memories": 1042, "by_type": {"fact": 30, "note": 900, "log": 112}, "by_scope": {...}, "last_compact_at": 1788000000, "pending_embeddings": 3}
```

---

### 3.11 `doctor`
```
centmem doctor
```
Integrity check, model check, config check. Exit 0 healthy.

---

### 3.12 `backup`
```
centmem backup --to ~/backups/centmem-$(date +%F).db
```
`VACUUM INTO`.

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
