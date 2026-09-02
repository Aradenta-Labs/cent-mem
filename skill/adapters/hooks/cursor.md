# Cursor Hook Adapter

Cursor agent mode reads `.cursorrules` / `.cursor/rules/` and writes conversation logs.

## Transcript Location

- Workspace log export: `.cursor/logs/conversation.json`
- Global logs: `~/.cursor/logs/`

## Capture Triggers

### 1. `.cursorrules` Post-Session Instruction

Add to `.cursorrules` or `.cursor/rules/centmem.md`:

```markdown
When finishing a major implementation or architecture decision, run:
centmem capture run --transcript ".cursor/logs/conversation.json" --harness cursor --scope "project:${CENTMEM_PROJ:-global}"
```

### 2. File Watcher Mode

Watch exported Cursor logs in real time:

```bash
centmem capture run --watch --transcript ".cursor/logs/conversation.json" --harness cursor --scope "project:${CENTMEM_PROJ:-global}"
```

### 3. Session Summary

Inspect captured memories from Cursor:

```bash
centmem capture summary
```

## Normalization

Convert Cursor JSON transcripts to normalized JSONL:

```bash
centmem capture convert --harness cursor --input ".cursor/logs/conversation.json"
```
