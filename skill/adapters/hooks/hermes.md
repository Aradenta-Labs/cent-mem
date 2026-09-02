# Hermes Hook Adapter

Hermes agent framework exposes an extensible event and plugin lifecycle.

## Transcript Location & Event Delivery

- Hermes event log: `~/.hermes/events/<session-id>.json`
- Hook callback payload: structured event JSON stream

## Capture Triggers

### 1. Hermes Plugin Callback (`on_session_end`)

Configure the Hermes cent-mem hook plugin to invoke `centmem capture run` upon session completion:

```bash
centmem capture run --transcript "$HOME/.hermes/events/latest.json" --harness hermes --scope "project:${CENTMEM_PROJ:-global}"
```

### 2. Real-Time Incremental Watcher

Watch Hermes session events in real time:

```bash
centmem capture run --watch --transcript "$HOME/.hermes/events/latest.json" --harness hermes --scope "project:${CENTMEM_PROJ:-global}"
```

### 3. Session Summary

Inspect captured memories:

```bash
centmem capture summary
```

## Normalization

Convert Hermes event logs to standard JSONL:

```bash
centmem capture convert --harness hermes --input "$HOME/.hermes/events/latest.json"
```
