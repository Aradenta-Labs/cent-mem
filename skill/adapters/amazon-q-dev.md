# Amazon Q Developer adapter

Amazon Q Developer (IDE extension / CLI) can call `centmem` via its shell/command execution. The skill `SKILL.md` is the contract; this file describes invocation specifics.

## Setup

1. Ensure `centmem` is on `PATH`.
2. Amazon Q Developer supports custom commands/rules; add the `centmem` skill by placing `SKILL.md` where your Q Developer rules are loaded, or reference it from a custom command.
3. Set env vars in your shell profile so recipes in `SKILL.md` resolve:

```bash
export CENTMEM_AGENT=amazon-q
export CENTMEM_SID="sess_$(date +%s)"
export CENTMEM_PROJ="$(basename "$PWD")"
```

## Canonical recipes

Recall (at task start):

```bash
centmem recall "$TASK" --scope "project:$CENTMEM_PROJ" --top 5 --inherit
```

Store a decision:

```bash
centmem put \
  --scope "project:$CENTMEM_PROJ" \
  --type note \
  --content "$NOTE" \
  --tags decision \
  --source-agent "$CENTMEM_AGENT" \
  --source-session "$CENTMEM_SID"
```

Store a key/value fact:

```bash
centmem set --scope "project:$CENTMEM_PROJ" --key "$KEY" --value "$JSON_VALUE"
```

## Invocation rules

- Parse stdout as JSON (`{"ok":true,"results":[...]}`).
- Branch on exit code: `0` ok, `1` error, `2` not found, `3` conflict.
- Never use `--pretty` for programmatic use.
