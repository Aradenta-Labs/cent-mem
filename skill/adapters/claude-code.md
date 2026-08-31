# Claude Code adapter

Claude Code can invoke `centmem` via the Bash tool. The skill `SKILL.md` is the contract; no plugin is required.

## Setup

1. Ensure `centmem` is on `PATH` (run `which centmem` to confirm).
2. The skill will be auto-discovered when placed at:
   - `~/.claude/skills/centmem/SKILL.md` (user-level), or
   - `./.claude/skills/centmem/SKILL.md` (project-level).
3. Run the installer: `bash skill/install.sh`.

## Recommended environment in your project

Create `.claude/centmem.env` (sourced by hooks if you want, or read directly):

```bash
export CENTMEM_AGENT=claude
export CENTMEM_SID="${CLAUDE_SESSION_ID:-$(uuidgen 2>/dev/null || echo sess_$(date +%s))}"
export CENTMEM_PROJ="$(basename "$PWD")"
```

The agent then uses recipes from `SKILL.md` substituting these vars.

## Optional: hook on session start

Add to `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "type": "command",
        "command": "centmem recall \"${CLAUDE_TASK:-project context}\" --scope project:${CENTMEM_PROJ} --top 5"
      }
    ]
  }
}
```

(The exact hook surface depends on your Claude Code version; the recipes in `SKILL.md` work without hooks.)
