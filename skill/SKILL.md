---
name: centmem
description: Shared memory for AI agents. Call `centmem` to recall context and store learnings so every agent shares one brain. Use whenever you need to remember/retrieve project context, user preferences, decisions, or session history.
version: 1.3.0
binary: centmem
homepage: https://github.com/aradenta-labs/cent-mem
---

# Skill: centmem — Shared Memory for AI Agents

You have access to a **shared memory store** via the `centmem` CLI. All AI agents on this machine use the same store, scoped by project → agent → session. Use it to **recall** relevant context before acting and **store** what you learn so future sessions (yours or other agents') benefit.

## When to use this skill

**Use PROACTIVELY when:**
- Starting any new task — recall prior decisions, conventions, and preferences first.
- You learn a durable fact (user preference, project convention, API endpoint, decision).
- You finish a logical unit of work — log a checkpoint so other agents know what happened.
- You are about to re-derive something you suspect was already decided.

**Do NOT use for:**
- Ephemeral scratchpad state within a single response.
- Large binary blobs (use file paths instead).
- Sensitive secrets (passwords/tokens) — store a reference, never the value.

## Quickstart (must run once per machine)

```bash
centmem init
```

## Canonical recipes

### 1. Load context at session start
```bash
centmem recall "<one-line task summary>" --scope "project:$PROJ" --top 5
centmem get --scope "project:$PROJ" --key "project.conventions" --inherit
```

### 2. Store a decision / note / insight
```bash
centmem put \
  --scope "project:$PROJ" \
  --type note \
  --content "We chose sqlite-vec over pgvector for local-first speed." \
  --tags decision,db \
  --source-agent "$AGENT" \
  --source-session "$SID"
```

### 3. Store a key/value fact (upsert)
```bash
centmem set --scope "project:$PROJ" --key "user.timezone" --value '"Asia/Jakarta"' --tags user
centmem set --scope "project:$PROJ" --key "api.base_url" --value '"https://api.example.com"'
```
`--value` is JSON: strings quoted, numbers bare, objects/arrays as-is.

### 4. Append a session log
```bash
centmem put \
  --scope "project:$PROJ/agent:$AGENT/session:$SID" \
  --type log \
  --content "Implemented PRD scaffold; chose hierarchical scoping." \
  --tags checkpoint
```

### 5. Timeline of recent activity
```bash
centmem timeline --scope "project:$PROJ" --since 24h
```

### 6. Auto-capture and transcript hooks
```bash
# Trigger an on-demand capture pass on a transcript
centmem capture run --transcript "$TRANSCRIPT_PATH" --scope "project:$PROJ"

# Inspect what memories were captured in the latest session
centmem capture summary

# List, add, or remove active capture categories
centmem capture categories --list
centmem capture categories --add "security"

# Normalize a foreign harness transcript to standard JSONL
centmem capture convert --harness cursor --input .cursor/logs/session.json
```

## Scope grammar

```
global
project:<name>
project:<name>/agent:<agent>
project:<name>/agent:<agent>/session:<id>
```
- Names: `[a-z0-9-_.]+` (case-insensitive, stored lowercase).
- Scopes auto-create on first write (including ancestors).
- Reads default to **inherit** ancestors (a project read also sees `global`).

## Output contract

- **Default output is JSON to stdout.** Always parse JSON. Use `--pretty` only for human reading.
- **Exit codes:** `0` success, `1` error (JSON on stderr), `2` not found, `3` conflict.
- Error shape (stderr): `{"error":{"code":"NOT_FOUND","message":"...","hint":"..."}}`.

## Commands

The canonical, complete contract is [docs/cli-contract.md](../docs/cli-contract.md). This quick reference matches the CLI exactly:

| Command | Usage | Purpose |
|---------|-------|---------|
| `init` | `init [--model <name>] [--force]` | create/open store + download model |
| `put` | `put --scope <s> --type note\|log --content <t> [--tags a,b] [--source-agent a] [--source-session s]` | write note or log memory |
| `set` | `set --scope <s> --key <k> --value <json> [--tags a,b]` | upsert key/value fact |
| `get` | `get --scope <s> --key <k> [--inherit]` | fetch fact by key (with inheritance) |
| `recall` | `recall <query> --scope <s> [--top N] [--type t] [--tags a,b] [--since d] [--until d] [--agent a] [--inherit] [--children]` | hybrid search (semantic+keyword+facts+timeline) |
| `timeline` | `timeline --scope <s> [--since d] [--until d] [--limit N]` | chronological logs |
| `list` | `list --scope <s> [--type t] [--tags a,b] [--limit N] [--offset N]` | browse memories |
| `forget` | `forget --id N` or `--scope <s> --key <k>` or `--scope <s> --tag <t>` | delete memory |
| `stats` | `stats` | store summary |
| `compact` | `compact [--scope <s>] [--dry-run]` | summarize + archive old memories |
| `doctor` | `doctor` | integrity check |
| `backup` | `backup --to <path>` | snapshot DB |
| `restore` | `restore --from <path>` | restore DB from backup |
| `capture` | `capture <run\|summary\|categories\|convert> [flags]` | transcript auto-capture and session summaries |
| `config` | `config <get\|set> [key] [value]` | get or set configuration keys in config.toml |

## Agent best practices

1. **Recall before you act.** A 50 ms `recall` saves tokens and avoids contradicting past decisions.
2. **Write what matters, not everything.** Store decisions, conventions, preferences, and endpoints — not every step.
3. **Tag generously.** Tags make `recall` and `list` much sharper.
4. **Scope correctly.** Project-wide facts go in `project:X`. Per-agent checkpoints go in `project:X/agent:Y/session:Z`.
5. **Prefer `set` for facts.** Use `put --type note` only for free-text memories.
6. **Log checkpoints at session end** so the next agent has continuity.
7. **Leverage auto-captured knowledge.** Memories ingested via transcript hooks have `source_agent = "capture-hook"`. You can recall them directly or inspect `centmem capture summary`.

Flag usage rules:
- `--scope`: required for writes; use the most specific scope the memory belongs to.
- `--tags`: comma-separated, lowercase, no spaces (`decision,db`).
- `--source-agent` / `--source-session`: always set on writes so provenance is tracked.
- `--inherit` (default true on `recall`/`get`): include ancestor scopes (global).
- `--children` (recall only): include descendant scopes (agents/sessions under a project).

## Integration notes per harness

- **Claude Code:** invoke via Bash tool. Set `AGENT=claude` and `SID=<session id>`.
- **Codex / Cursor / Continue:** invoke via their shell-exec capability with the same env vars.
- **Custom harnesses:** shell out to `centmem`; capture stdout JSON, check exit code.
- **Auto-capture hooks:** see [docs/guides/capture-hooks.md](../docs/guides/capture-hooks.md) and [skill/adapters/hooks/](./adapters/hooks/) for harness-specific hook scripts and file watcher setups.

See [skill/adapters/](./adapters/) for harness-specific setup snippets.

