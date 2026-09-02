# Claude Code Hook Adapter

Claude Code CLI logs session interactions in the `.claude/` project directory and supports shell lifecycle hooks.

## Transcript Location

- Project logs: `.claude/transcripts/<session-id>.jsonl`
- User logs: `~/.claude/logs/<session-id>.jsonl`

## Capture Triggers

### 1. Session-End Shell Exit Trap

Inject a shell exit trap into `.claude/centmem.env` or your shell configuration:

```bash
# BEGIN CENTMEM HOOK
trap 'centmem capture run --transcript "${CLAUDE_TRANSCRIPT_PATH:-.claude/latest.jsonl}" --harness claude-code --scope "project:${CENTMEM_PROJ:-global}" && centmem capture summary' EXIT
# END CENTMEM HOOK
```

### 2. On-Demand Capture

Run capture manually against any completed Claude Code transcript:

```bash
centmem capture run --transcript ".claude/transcripts/latest.jsonl" --harness claude-code --scope "project:${CENTMEM_PROJ:-global}"
```

### 3. Real-Time Watcher

Continuously watch active Claude Code transcripts:

```bash
centmem capture run --watch --transcript ".claude/transcripts/latest.jsonl" --harness claude-code --scope "project:${CENTMEM_PROJ:-global}"
```

## Normalization

Convert Claude Code transcripts to cent-mem format:

```bash
centmem capture convert --harness claude-code --input ".claude/transcripts/latest.jsonl"
```
