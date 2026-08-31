# Custom harness adapter

Any harness that can run a shell command can integrate `centmem`. The skill `SKILL.md` is the contract; this file shows the generic invocation pattern.

## Setup

1. Ensure `centmem` is on `PATH`.
2. Set env vars to identify this harness as the source:

```bash
export CENTMEM_AGENT=custom        # your agent id
export CENTMEM_SID="<session-id>" # stable per session/conversation
export CENTMEM_PROJ="$(basename "$PWD")"
```

## Generic invocation

Execute `centmem` as a subprocess, capture stdout JSON, and branch on the exit code:

```js
// Pseudocode (any language)
const proc = spawn("centmem", [
  "recall", query,
  "--scope", "project:" + process.env.CENTMEM_PROJ,
  "--top", "5",
  "--inherit"
]);
const stdout = await readAll(proc.stdout);   // JSON
const stderr = await readAll(proc.stderr);   // JSON error object
const code = proc.exitCode;                  // 0|1|2|3

if (code === 0) {
  const memories = JSON.parse(stdout).results; // [{id,type,scope,content,tags,score,matched_by}]
} else {
  const err = JSON.parse(stderr).error;        // {code,message,hint}
}
```

## Canonical recipes

Recall (at task start):

```bash
centmem recall "$TASK" --scope "project:$PROJ" --top 5 --inherit
```

Store a note:

```bash
centmem put \
  --scope "project:$PROJ" \
  --type note \
  --content "$NOTE" \
  --tags decision \
  --source-agent "$AGENT" \
  --source-session "$SID"
```

Store a fact:

```bash
centmem set --scope "project:$PROJ" --key "$KEY" --value "$JSON_VALUE"
```

## Invocation rules

1. Always parse stdout JSON.
2. Branch on exit code (0/1/2/3).
3. Treat `--pretty` as forbidden for programmatic use.
4. Errors are reported on stderr as `{"error":{"code":...,"message":...,"hint":...}}`.
