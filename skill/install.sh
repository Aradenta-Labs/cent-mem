#!/usr/bin/env bash
# centmem skill installer
# Installs the centmem skill into common AI agent harness locations.
set -euo pipefail

SKILL_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_NAME="centmem"

targets=()

# Claude Code (project-level + user-level)
targets+=("$PWD/.claude/skills/$SKILL_NAME")
targets+=("$HOME/.claude/skills/$SKILL_NAME")

# Cursor
targets+=("$HOME/.cursor/rules/$SKILL_NAME")

# Codex (OpenAI Codex CLI)
targets+=("$HOME/.codex/skills/$SKILL_NAME")

# Generic / custom harnesses
targets+=("$HOME/.config/skills/$SKILL_NAME")

echo "Installing skill '$SKILL_NAME' from $SKILL_SRC"

for dest in "${targets[@]}"; do
  mkdir -p "$dest"
  cp -R "$SKILL_SRC"/. "$dest"/
  echo "  installed -> $dest"
done

echo "Done. Restart your agent/harness if it caches skills."
echo "If you use a different harness, copy $SKILL_SRC into its skill/rules directory."
