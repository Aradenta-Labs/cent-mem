# Deepseek Hook Adapter

Deepseek CLI and local harnesses store session history in local JSON or text session exports.

## Transcript Location

- Session directory: `~/.deepseek/sessions/<session-id>.json`
- Project exports: `.deepseek/transcript.jsonl`

## Capture Triggers

### 1. Incremental File Watcher

Monitor active Deepseek session transcripts:

```bash
centmem capture run --watch --transcript "$HOME/.deepseek/sessions/latest.json" --harness deepseek --scope "project:${CENTMEM_PROJ:-global}"
```

### 2. On-Demand Capture

Ingest exported Deepseek sessions:

```bash
centmem capture run --transcript "$HOME/.deepseek/sessions/latest.json" --harness deepseek --scope "project:${CENTMEM_PROJ:-global}"
```

### 3. Session Summary

Verify captured items:

```bash
centmem capture summary
```

## Normalization

Convert Deepseek transcript files to standard JSONL:

```bash
centmem capture convert --harness deepseek --input "$HOME/.deepseek/sessions/latest.json"
```
