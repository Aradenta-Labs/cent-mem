# Trae Hook Adapter

Trae IDE agent logs sessions as structured JSON or log streams.

## Transcript Location

- User session directory: `~/.trae/logs/sessions/<session-id>/session.log`
- Workspace session logs: `.trae/logs/session.json`

## Capture Triggers

### 1. Real-Time Incremental Watcher

Watch Trae session logs in real time as the agent responds:

```bash
centmem capture run --watch --transcript "$HOME/.trae/logs/sessions/latest/session.log" --harness trae --scope "project:${CENTMEM_PROJ:-global}"
```

### 2. Session-End / On-Demand

Trigger a capture run after completing a task in Trae:

```bash
centmem capture run --transcript "$HOME/.trae/logs/sessions/latest/session.log" --harness trae --scope "project:${CENTMEM_PROJ:-global}"
```

### 3. Session Summary

Review extracted memories and deduplication stats:

```bash
centmem capture summary
```

## Normalization

Convert Trae session logs to cent-mem standard `.jsonl`:

```bash
centmem capture convert --harness trae --input "$HOME/.trae/logs/sessions/latest/session.log"
```
