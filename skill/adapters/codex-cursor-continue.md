# Codex / Cursor / Continue adapters

These harnesses expose a shell-exec or rules mechanism. The `centmem` CLI contract in [SKILL.md](../SKILL.md) is harness-agnostic; only the invocation details differ.

## Codex (OpenAI Codex CLI)

- Copy the skill to `~/.codex/skills/centmem/` (the installer does this).
- Codex reads `SKILL.md` as instructions and can run the CLI via its shell tool.
- Set env in your shell profile:
  ```bash
  export CENTMEM_AGENT=codex
  export CENTMEM_SID="sess_$(date +%s)"
  export CENTMEM_PROJ="$(basename "$PWD")"
  ```

## Cursor

- Copy the skill to `~/.cursor/rules/centmem/`.
- Cursor's agent mode reads `.cursor/rules/**` as project rules; `SKILL.md` becomes part of the agent's context.
- To invoke the CLI, the agent uses its built-in terminal capability with the same recipes.

### Native Cursor MCP Setup (v1.5.3+)

Cursor supports Model Context Protocol (MCP) servers. Configure `centmem` in `~/.cursor/mcp.json` or project `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "centmem": {
      "command": "centmem",
      "args": ["serve"]
    }
  }
}
```

## Continue (VS Code / JetBrains)

- Add `SKILL.md` contents to a Continue "rules" file, or reference the binary in `~/.continue/config.json` under custom commands.
- Example custom command:
  ```json
  {
    "name": "recall",
    "prompt": "centmem recall \"{{input}}\" --scope project:${CENTMEM_PROJ} --top 5",
    "description": "Recall shared memory"
  }
  ```

## Generic custom harness

Any harness that can run a shell command can integrate:

```
result = exec(["centmem", "recall", query, "--scope", scope, "--top", "5"])
if result.exit_code == 0:
    memories = json.loads(result.stdout)["results"]
else:
    # handle error from result.stderr JSON
```

Rules:
1. Always parse stdout JSON.
2. Branch on exit code (0/1/2/3).
3. Treat `--pretty` as forbidden for programmatic use.
