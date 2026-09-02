# Codex Hook Adapter

OpenAI Codex and terminal scripting harnesses interact via standard stream pipes.

## Capture Triggers

### 1. Streaming Stdin Pipe Wrapper

Wrap Codex CLI execution with a streaming pipe:

```bash
codex "$@" | tee >(centmem capture run --harness codex --scope "project:${CENTMEM_PROJ:-global}")
```

### 2. Direct Transcript Ingestion

Capture from a saved session output log:

```bash
centmem capture run --transcript "codex-session.log" --harness codex --scope "project:${CENTMEM_PROJ:-global}"
```

### 3. Session Summary

View memories captured from Codex sessions:

```bash
centmem capture summary
```

## Normalization

Convert raw Codex stream logs to standard JSONL:

```bash
centmem capture convert --harness codex --input "codex-session.log"
```
