---
name: centmem
description: Shared memory for AI agents. Call `centmem` CLI to recall prior context, store decisions, save facts, and inspect timeline so every agent shares one persistent brain. Use whenever you need to remember or retrieve project context, architecture decisions, user preferences, API conventions, or session checkpoints.
version: 1.5.0
binary: centmem
homepage: https://github.com/aradenta-labs/cent-mem
allowed-tools:
  - Bash(centmem *)
---

# Skill: centmem — Shared Memory for AI Agents

You have access to a fast, local-first **shared memory store** via the `centmem` CLI. All AI agents working on this machine or repository use the same store, scoped hierarchically by `project` → `agent` → `session`.

Use it to **recall** relevant context before acting, and **store** what you learn so future sessions (yours or other agents') never have to re-derive the same context.

---

## The 3-Step Memory Workflow

Whenever you start or work on a task, follow this loop:
1. **READ**: Run `centmem recall "<task context>"` to load prior context and decisions.
2. **DO**: Execute the user's prompt.
3. **UPDATE**: Run `centmem put` or `centmem set` to store new learnings, architectural decisions, or endpoints before finishing.

---

## When to Use This Skill

**Use PROACTIVELY when:**
- **Starting any new session or task** — recall prior decisions, conventions, and user preferences first.
- **You make an architectural or design choice** — log a note with `--tags decision`.
- **You discover or configure a durable fact** — upsert with `centmem set` (e.g., ports, endpoints, credentials references, versions).
- **You finish a logical unit of work or milestone** — log a checkpoint so other agents know what happened.
- **You encounter an error and its resolution** — save it so future agents don't get stuck on the same issue.
- **You are asked to check project context or progress** — run `centmem recall` or `centmem timeline`.

**Do NOT use for:**
- Ephemeral scratchpad state within a single immediate response.
- Large raw binary blobs (store local file paths instead).
- Plaintext secrets (passwords, live tokens) — store environment variable names or vault references, never the raw secret.

---

## Quickstart (Must run once per machine)

```bash
centmem init
```
*Creates `~/.centmem/` and downloads the default local ONNX embedding model (BGE-small 384d, ~100 MB).*

---

## Canonical Recipes

### 1. Load Context at Session Start
```bash
centmem recall "<one-line task summary>" --scope "project:$CENTMEM_PROJ" --top 5
centmem get --scope "project:$CENTMEM_PROJ" --key "project.conventions" --inherit
```

### 2. Store a Decision / Note / Insight
```bash
centmem put \
  --scope "project:$CENTMEM_PROJ" \
  --type note \
  --content "Chose SQLite-vec with local ONNX embeddings for 100% offline agent memory." \
  --tags decision,architecture \
  --source-agent "$CENTMEM_AGENT" \
  --source-session "$CENTMEM_SESSION"
```

### 3. Store a Key/Value Fact (Upsert)
```bash
centmem set --scope "project:$CENTMEM_PROJ" --key "user.timezone" --value '"Asia/Jakarta"' --tags user
centmem set --scope "project:$CENTMEM_PROJ" --key "api.base_url" --value '"https://api.internal.dev"'
```
*Note: `--value` is valid JSON: strings quoted (`'"abc"'`), numbers bare (`123`), objects/arrays JSON-formatted.*

### 4. Append a Progress Log / Checkpoint
```bash
centmem put \
  --scope "project:$CENTMEM_PROJ/agent:$CENTMEM_AGENT/session:$CENTMEM_SESSION" \
  --type log \
  --content "Completed phase 1 implementation; passing all unit tests." \
  --tags checkpoint
```

### 5. Review Timeline of Recent Activity
```bash
centmem timeline --scope "project:$CENTMEM_PROJ" --since 24h --limit 10
```

### 6. Auto-Capture & Transcript Hooks
```bash
# Run on-demand capture pass on a transcript
centmem capture run --transcript "$TRANSCRIPT_PATH" --scope "project:$CENTMEM_PROJ"

# Inspect memories captured in the latest session
centmem capture summary

# List, add, or remove active capture categories
centmem capture categories --list
centmem capture categories --add "security"

# Normalize a foreign harness transcript to standard JSONL
centmem capture convert --harness cursor --input .cursor/logs/session.json
```

### 7. Web UI Memory Browser Dashboard
```bash
# Launch the embedded web UI memory browser dashboard in your browser
centmem ui

# Run on a custom port without automatically opening browser
centmem ui --port 8080 --no-open
```

---

## Scope Grammar & Hierarchy

```
global
project:<name>
project:<name>/agent:<agent>
project:<name>/agent:<agent>/session:<id>
```
- **Names**: `[a-z0-9-_.]+` (case-insensitive, stored lowercase).
- **Auto-create**: Scopes auto-create on first write (including ancestors).
- **Inheritance**: Reads default to **inheriting** ancestors (a project read also sees `global`).
  - Pass `--no-inherit` for strict scoping.
  - Pass `--children` to include descendant scopes (agents/sessions under a project).

---

## Output Contract & Error Handling

- **Default output is JSON to stdout.** Always parse stdout as JSON. Use `--pretty` only when outputting directly to human users.
- **Exit codes:**
  - `0`: Success.
  - `1`: General error or invalid arguments (JSON on stderr).
  - `2`: Key or entity not found.
  - `3`: Conflict / duplicate.
- **Error shape (stderr):**
  ```json
  {"error":{"code":"NOT_FOUND","message":"fact not found for key: foo","hint":"try running recall or list"}}
  ```

---

## CLI Reference Summary

| Command | Syntax | Description |
|---------|--------|-------------|
| `init` | `centmem init [--model <name>] [--force]` | Initialize local store and download embedding model |
| `put` | `centmem put --scope <s> --type <note|log> --content <t> [--tags a,b]` | Store a freeform note or chronological log |
| `set` | `centmem set --scope <s> --key <k> --value <json> [--tags a,b]` | Upsert a key/value fact |
| `get` | `centmem get --scope <s> --key <k> [--inherit]` | Fetch a fact by key with inheritance |
| `recall` | `centmem recall <query> --scope <s> [--top N] [--type t] [--tags a,b]` | Hybrid search (vector + FTS5 bm25 + facts + timeline via RRF) |
| `timeline` | `centmem timeline --scope <s> [--since d] [--until d] [--limit N]` | Chronological view of logs and memories |
| `list` | `centmem list --scope <s> [--type t] [--tags a,b] [--limit N]` | Browse and filter memories |
| `forget` | `centmem forget --id N` or `--scope <s> --key <k>` | Delete memory entries |
| `stats` | `centmem stats` | View memory counts, database size, and status |
| `compact` | `centmem compact [--scope <s>] [--dry-run]` | Consolidate and archive old memories |
| `doctor` | `centmem doctor` | Health check (DB integrity, schema, model, FTS5) |
| `backup` | `centmem backup --to <path>` | Create snapshot backup of SQLite database |
| `restore` | `centmem restore --from <path>` | Restore database from backup snapshot |
| `capture` | `centmem capture <run|summary|categories|convert> [flags]` | Auto-capture engine and session telemetry |
| `config` | `centmem config <get|set> [key] [value]` | Manage configuration settings in `config.toml` |
| `ui` | `centmem ui [--port <port>] [--host <host>] [--no-open]` | Launch embedded Web UI memory browser dashboard |
| `reindex` | `centmem reindex [--all] [--batch N] [--max-time d] [--dry-run]` | Re-embed memories into vector index |

---

## Detailed Skill Artifacts

For in-depth guides, schemas, and runnable code, consult the following bundled skill references:
- **CLI Commands Reference**: [references/cli-commands.md](references/cli-commands.md) — Exhaustive documentation of all flags, types, options, and JSON outputs.
- **Architecture & Scoping**: [references/architecture-and-scoping.md](references/architecture-and-scoping.md) — Deep dive into RRF k=60 hybrid search, vector embeddings, and scoping rules.
- **Auto-Capture Hooks**: [references/capture-hooks.md](references/capture-hooks.md) — Guide to transcript watchers, session traps, and 3-tier classification backends.
- **Agent Workflow Walkthrough**: [examples/agent-workflow-examples.md](examples/agent-workflow-examples.md) — Real-world example transcripts of agents reading and writing shared memory.
- **Shell Recipes**: [examples/recipes.sh](examples/recipes.sh) — Ready-to-use bash snippets for common operations.
- **Helper Script**: [scripts/centmem-helper.sh](scripts/centmem-helper.sh) — Utility to verify environment and test memory recall.
