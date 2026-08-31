#!/usr/bin/env bash
# centmem skill installer
# Installs the centmem skill into common AI agent harness locations.
#
# Flags:
#   --list       print the target destinations and exit (no changes)
#   --dry-run    print what would be copied/removed without touching disk
#   --uninstall  remove the skill from all targets
#
# Idempotent: safe to run repeatedly (overwrite, no duplicate nesting).
set -euo pipefail

SKILL_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_NAME="centmem"

MODE="install"   # install | uninstall
DRY_RUN=0
LIST_ONLY=0

for arg in "$@"; do
  case "$arg" in
    --list)      LIST_ONLY=1 ;;
    --dry-run)   DRY_RUN=1 ;;
    --uninstall) MODE="uninstall" ;;
    -h|--help)
      echo "Usage: $0 [--list] [--dry-run] [--uninstall]"
      echo "  --list       print target destinations and exit"
      echo "  --dry-run    show actions without modifying disk"
      echo "  --uninstall  remove the skill from all targets"
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

if [[ -z "${HOME:-}" ]]; then
  echo "ERROR: \$HOME is not set; cannot determine install destinations." >&2
  echo "Set HOME (or run with --list) and try again." >&2
  exit 1
fi

# Build the target list. $PWD is the project-level Claude Code location and is
# only meaningful when run from a project; it is always included.
targets=()
targets+=("$PWD/.claude/skills/$SKILL_NAME")
targets+=("$HOME/.claude/skills/$SKILL_NAME")
targets+=("$HOME/.cursor/rules/$SKILL_NAME")
targets+=("$HOME/.codex/skills/$SKILL_NAME")
targets+=("$HOME/.config/skills/$SKILL_NAME")

if [[ "$LIST_ONLY" == "1" ]]; then
  echo "Skill source: $SKILL_SRC"
  echo "Target destinations ($SKILL_NAME):"
  for dest in "${targets[@]}"; do
    printf '  %s\n' "$dest"
  done
  exit 0
fi

if [[ "$MODE" == "uninstall" ]]; then
  echo "Uninstalling skill '$SKILL_NAME'"
  for dest in "${targets[@]}"; do
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove -> $dest"
      continue
    fi
    if [[ -d "$dest" ]]; then
      rm -rf "$dest"
      echo "  removed -> $dest"
    else
      echo "  (not present) $dest"
    fi
  done
  echo "Uninstall done."
  exit 0
fi

echo "Installing skill '$SKILL_NAME' from $SKILL_SRC"
installed=0
for dest in "${targets[@]}"; do
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would install -> $dest"
    continue
  fi
  mkdir -p "$dest"
  cp -R "$SKILL_SRC"/. "$dest"/
  echo "  installed -> $dest"
  installed=$((installed + 1))
done

if [[ "$DRY_RUN" == "1" ]]; then
  echo "Dry run complete (no files were changed)."
  exit 0
fi

echo "Done. Installed to $installed location(s)."
echo "Restart your agent/harness if it caches skills."
echo "If you use a different harness, copy $SKILL_SRC into its skill/rules directory."
