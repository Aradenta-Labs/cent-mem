# Antigravity Hook Adapter

Google Antigravity captures agent interactions as structured JSONL transcripts.

## Transcript Location

Transcripts are located at:
- Conversation log: `~/.gemini/antigravity/brain/<conversation-id>/.system_generated/logs/transcript.jsonl`
- Full transcript: `~/.gemini/antigravity/brain/<conversation-id>/.system_generated/logs/transcript_full.jsonl`

## Capture Triggers

### 1. Per-Message Trigger (Real-Time File Watcher)

Run the watcher in the background to monitor active sessions incrementally:

```bash
centmem capture run --watch --transcript "$HOME/.gemini/antigravity/brain/$CONVERSATION_ID/.system_generated/logs/transcript.jsonl" --harness antigravity --scope "project:${CENTMEM_PROJ:-cent-mem}"
```

### 2. On-Demand Trigger

Classify an entire transcript on demand:

```bash
centmem capture run --transcript "$HOME/.gemini/antigravity/brain/$CONVERSATION_ID/.system_generated/logs/transcript.jsonl" --harness antigravity --scope "project:${CENTMEM_PROJ:-cent-mem}"
```

### 3. Session Summary

Inspect memories extracted from the last or active session:

```bash
centmem capture summary
```

## Normalization Format

Antigravity transcript lines have the shape:
```json
{"step_index": 1, "source": "USER_EXPLICIT", "type": "USER_INPUT", "content": "decision: use sqlite-vec", "created_at": "2026-09-02T10:00:00Z"}
```
Normalizing to standard JSONL:
```bash
centmem capture convert --harness antigravity --input "$HOME/.gemini/antigravity/brain/$CONVERSATION_ID/.system_generated/logs/transcript.jsonl"
```
