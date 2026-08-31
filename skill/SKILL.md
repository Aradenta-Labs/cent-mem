---
name: centmem
description: Shared memory for AI agents. Call `centmem` to recall context and store learnings so every agent shares one brain. Use whenever you need to remember/retrieve project context, user preferences, decisions, or session history.
version: 1.0.0
binary: centmem
homepage: https://github.com/farras/cent-mem
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

## Full command reference

See [docs/cli-contract.md](../docs/cli-contract.md) for the complete, stable contract. Summary:

| Command | Purpose |
|---------|---------|
| `init` | create/open store + download model |
| `put` | write note or log memory |
| `set` | upsert key/value fact |
| `get` | fetch fact by key (with inheritance) |
| `recall <query>` | hybrid search (semantic+keyword+facts+timeline) |
| `timeline` | chronological logs |
| `list` | browse memories |
| `forget` | delete memory |
| `compact` | summarize/archive old memories |
| `stats` | store summary |
| `doctor` | integrity check |
| `backup` | snapshot DB |

## Agent best practices

1. **Recall before you act.** A 50 ms `recall` saves tokens and avoids contradicting past decisions.
2. **Write what matters, not everything.** Store decisions, conventions, preferences, and endpoints — not every step.
3. **Tag generously.** Tags make `recall` and `list` much sharper.
4. **Scope correctly.** Project-wide facts go in `project:X`. Per-agent checkpoints go in `project:X/agent:Y/session:Z`.
5. **Prefer `set` for facts.** Use `put --type note` only for free-text memories.
6. **Log checkpoints at session end** so the next agent has continuity.

## Integration notes per harness

- **Claude Code:** invoke via Bash tool. Set `AGENT=claude` and `SID=<session id>`.
- **Codex / Cursor / Continue:** invoke via their shell-exec capability with the same env vars.
- **Custom harnesses:** shell out to `centmem`; capture stdout JSON, check exit code.

See [skill/adapters/](./adapters/) for harness-specific setup snippets.
