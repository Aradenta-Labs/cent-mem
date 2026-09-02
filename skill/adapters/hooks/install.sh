#!/usr/bin/env bash
# centmem hook adapter installer
# Installs per-harness hooks, traps, shims, and watchers for AI agent harnesses.
#
# Supported harnesses:
#   antigravity, trae, claude-code, cursor, codex, deepseek, hermes
#
# Flags:
#   --harness <name>   install hook for a specific harness
#   --all              install hooks for all detected harnesses (default)
#   --trigger <list>   comma-separated triggers: message, session-end, on-demand, all (default: all)
#   --scope <scope>    target scope (default: project:<basename> or global)
#   --list             list detected harnesses and target destinations, then exit
#   --dry-run          print actions without modifying disk
#   --uninstall        remove hooks from targets
#   --no-daemon        skip background daemon plist/systemd service generation
#
# Idempotent: safe to run repeatedly.
set -euo pipefail

SKILL_HOOKS_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_ROOT="$(cd "$SKILL_HOOKS_SRC/../.." && pwd)"

MODE="install"
DRY_RUN=0
LIST_ONLY=0
TARGET_HARNESS=""
INSTALL_ALL=1
TRIGGERS="all"
CUSTOM_SCOPE=""
NO_DAEMON=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --harness)
      TARGET_HARNESS="$2"
      INSTALL_ALL=0
      shift 2
      ;;
    --all)
      INSTALL_ALL=1
      shift
      ;;
    --trigger)
      TRIGGERS="$2"
      shift 2
      ;;
    --scope)
      CUSTOM_SCOPE="$2"
      shift 2
      ;;
    --list)
      LIST_ONLY=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --uninstall)
      MODE="uninstall"
      shift
      ;;
    --no-daemon)
      NO_DAEMON=1
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "  --harness <name>   target specific harness (antigravity|trae|claude-code|cursor|codex|deepseek|hermes)"
      echo "  --all              install hooks for all detected harnesses (default)"
      echo "  --trigger <list>   triggers to enable: message, session-end, on-demand, all (default: all)"
      echo "  --scope <scope>    default scope for auto-captured memories"
      echo "  --list             list detected harnesses and targets"
      echo "  --dry-run          show actions without writing to disk"
      echo "  --uninstall        remove hooks from all targets"
      echo "  --no-daemon        skip background daemon service generation"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "${HOME:-}" ]]; then
  echo "ERROR: \$HOME is not set; cannot determine install destinations." >&2
  exit 1
fi

PROJECT_NAME="$(basename "$PWD")"
SCOPE="${CUSTOM_SCOPE:-project:${PROJECT_NAME}}"

ALL_HARNESSES=("antigravity" "claude-code" "cursor" "trae" "codex" "deepseek" "hermes")

# Auto-detect installed harnesses
detect_harnesses() {
  local detected=()
  for h in "${ALL_HARNESSES[@]}"; do
    case "$h" in
      antigravity)
        if [[ -d "$HOME/.gemini/antigravity" || -n "${ANTIGRAVITY_HOME:-}" ]]; then
          detected+=("antigravity")
        fi
        ;;
      claude-code)
        if [[ -d "$HOME/.claude" || -d "$PWD/.claude" || $(command -v claude 2>/dev/null) ]]; then
          detected+=("claude-code")
        fi
        ;;
      cursor)
        if [[ -d "$HOME/.cursor" || -d "$PWD/.cursor" || -f "$PWD/.cursorrules" || $(command -v cursor 2>/dev/null) ]]; then
          detected+=("cursor")
        fi
        ;;
      trae)
        if [[ -d "$HOME/.trae" || -d "$PWD/.trae" || $(command -v trae 2>/dev/null) ]]; then
          detected+=("trae")
        fi
        ;;
      codex)
        if [[ -d "$HOME/.codex" || $(command -v codex 2>/dev/null) ]]; then
          detected+=("codex")
        fi
        ;;
      deepseek)
        if [[ -d "$HOME/.deepseek" || -d "$PWD/.deepseek" || $(command -v deepseek 2>/dev/null) ]]; then
          detected+=("deepseek")
        fi
        ;;
      hermes)
        if [[ -d "$HOME/.hermes" || $(command -v hermes 2>/dev/null) ]]; then
          detected+=("hermes")
        fi
        ;;
    esac
  done
  echo "${detected[@]}"
}

SELECTED_HARNESSES=()
if [[ -n "$TARGET_HARNESS" ]]; then
  SELECTED_HARNESSES=("$TARGET_HARNESS")
elif [[ "$INSTALL_ALL" == "1" ]]; then
  DETECTED=($(detect_harnesses))
  if [[ ${#DETECTED[@]} -eq 0 ]]; then
    # Fallback to all standard harnesses if none detected explicitly
    SELECTED_HARNESSES=("${ALL_HARNESSES[@]}")
  else
    SELECTED_HARNESSES=("${DETECTED[@]}")
  fi
fi

if [[ "$LIST_ONLY" == "1" ]]; then
  echo "cent-mem Hook Adapter Installer"
  echo "Source: $SKILL_HOOKS_SRC"
  echo "Target Scope: $SCOPE"
  echo "Triggers: $TRIGGERS"
  echo ""
  echo "Selected harnesses:"
  for h in "${SELECTED_HARNESSES[@]}"; do
    echo "  - $h"
  done
  exit 0
fi

# Marker blocks for idempotent shell config injection
MARKER_START="# BEGIN CENTMEM HOOK"
MARKER_END="# END CENTMEM HOOK"

inject_hook_block() {
  local target_file="$1"
  local content="$2"

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would inject hook block into $target_file"
    return
  fi

  mkdir -p "$(dirname "$target_file")"
  touch "$target_file"

  # Remove existing block if present
  if grep -q "$MARKER_START" "$target_file" 2>/dev/null; then
    local tmp
    tmp="$(mktemp)"
    sed "/$MARKER_START/,/$MARKER_END/d" "$target_file" > "$tmp"
    mv "$tmp" "$target_file"
  fi

  # Append new block
  {
    echo "$MARKER_START"
    echo "$content"
    echo "$MARKER_END"
  } >> "$target_file"

  echo "  injected hook -> $target_file"
}

remove_hook_block() {
  local target_file="$1"

  if [[ ! -f "$target_file" ]]; then
    return
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would remove hook block from $target_file"
    return
  fi

  if grep -q "$MARKER_START" "$target_file" 2>/dev/null; then
    local tmp
    tmp="$(mktemp)"
    sed "/$MARKER_START/,/$MARKER_END/d" "$target_file" > "$tmp"
    mv "$tmp" "$target_file"
    echo "  removed hook from -> $target_file"
  fi
}

install_claude_code() {
  local env_file="$PWD/.claude/centmem.env"
  local hook_script="export CENTMEM_SCOPE=\"${SCOPE}\"\ntrap 'centmem capture run --transcript \"\${CLAUDE_TRANSCRIPT_PATH:-.claude/latest.jsonl}\" --harness claude-code --scope \"\${CENTMEM_SCOPE}\" && centmem capture summary' EXIT"
  inject_hook_block "$env_file" "$hook_script"
}

uninstall_claude_code() {
  remove_hook_block "$PWD/.claude/centmem.env"
  remove_hook_block "$HOME/.claude/centmem.env"
}

install_cursor() {
  local rules_file="$PWD/.cursorrules"
  local rule_content="# centmem auto-capture hook\n# On session or milestone completion, run:\n# centmem capture run --transcript \".cursor/logs/conversation.json\" --harness cursor --scope \"${SCOPE}\""
  inject_hook_block "$rules_file" "$rule_content"
}

uninstall_cursor() {
  remove_hook_block "$PWD/.cursorrules"
  remove_hook_block "$HOME/.cursorrules"
}

install_codex() {
  local shim_dir="$HOME/.codex/bin"
  local shim_path="$shim_dir/codex-centmem-shim"

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would create shim at $shim_path"
    return
  fi

  mkdir -p "$shim_dir"
  cat << 'EOF' > "$shim_path"
#!/usr/bin/env bash
set -euo pipefail
TMP_TRANSCRIPT="$(mktemp)"
trap 'rm -f "$TMP_TRANSCRIPT"' EXIT

if command -v codex >/dev/null 2>&1; then
  codex "$@" | tee "$TMP_TRANSCRIPT"
  centmem capture run --transcript "$TMP_TRANSCRIPT" --harness codex --scope "${CENTMEM_SCOPE:-global}" || true
fi
EOF
  chmod 0755 "$shim_path"
  echo "  installed shim -> $shim_path"
}

uninstall_codex() {
  local shim_path="$HOME/.codex/bin/codex-centmem-shim"
  if [[ -f "$shim_path" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $shim_path"
    else
      rm -f "$shim_path"
      echo "  removed -> $shim_path"
    fi
  fi
}

install_antigravity() {
  local hook_doc="$HOME/.gemini/antigravity/hooks/centmem-capture.md"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would write adapter doc -> $hook_doc"
    return
  fi
  mkdir -p "$(dirname "$hook_doc")"
  cp "$SKILL_HOOKS_SRC/antigravity.md" "$hook_doc"
  echo "  installed adapter doc -> $hook_doc"
}

uninstall_antigravity() {
  local hook_doc="$HOME/.gemini/antigravity/hooks/centmem-capture.md"
  if [[ -f "$hook_doc" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $hook_doc"
    else
      rm -f "$hook_doc"
      echo "  removed -> $hook_doc"
    fi
  fi
}

install_trae() {
  local hook_cfg="$HOME/.trae/hooks/centmem.json"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would create $hook_cfg"
    return
  fi
  mkdir -p "$(dirname "$hook_cfg")"
  cat << EOF > "$hook_cfg"
{
  "name": "centmem-capture",
  "command": "centmem capture run --harness trae --scope ${SCOPE}",
  "events": ["session_end"]
}
EOF
  echo "  installed hook config -> $hook_cfg"
}

uninstall_trae() {
  local hook_cfg="$HOME/.trae/hooks/centmem.json"
  if [[ -f "$hook_cfg" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $hook_cfg"
    else
      rm -f "$hook_cfg"
      echo "  removed -> $hook_cfg"
    fi
  fi
}

install_deepseek() {
  local hook_cfg="$HOME/.deepseek/hooks/centmem.json"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would create $hook_cfg"
    return
  fi
  mkdir -p "$(dirname "$hook_cfg")"
  cat << EOF > "$hook_cfg"
{
  "name": "centmem-capture",
  "command": "centmem capture run --harness deepseek --scope ${SCOPE}",
  "trigger": "session_end"
}
EOF
  echo "  installed hook config -> $hook_cfg"
}

uninstall_deepseek() {
  local hook_cfg="$HOME/.deepseek/hooks/centmem.json"
  if [[ -f "$hook_cfg" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $hook_cfg"
    else
      rm -f "$hook_cfg"
      echo "  removed -> $hook_cfg"
    fi
  fi
}

install_hermes() {
  local hook_cfg="$HOME/.hermes/plugins/centmem.json"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  [dry-run] would create $hook_cfg"
    return
  fi
  mkdir -p "$(dirname "$hook_cfg")"
  cat << EOF > "$hook_cfg"
{
  "plugin": "centmem-capture",
  "events": ["on_message_end", "on_session_end"],
  "action": "centmem capture run --harness hermes --scope ${SCOPE}"
}
EOF
  echo "  installed plugin config -> $hook_cfg"
}

uninstall_hermes() {
  local hook_cfg="$HOME/.hermes/plugins/centmem.json"
  if [[ -f "$hook_cfg" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $hook_cfg"
    else
      rm -f "$hook_cfg"
      echo "  removed -> $hook_cfg"
    fi
  fi
}

# Daemon / Service setup
install_daemon_service() {
  if [[ "$NO_DAEMON" == "1" ]]; then
    return
  fi

  if [[ "$(uname)" == "Darwin" ]]; then
    local plist_dir="$HOME/Library/LaunchAgents"
    local plist_file="$plist_dir/com.aradenta.centmem.capture-watcher.plist"
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would create launchd plist -> $plist_file"
      return
    fi
    mkdir -p "$plist_dir"
    cat << EOF > "$plist_file"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.aradenta.centmem.capture-watcher</string>
    <key>ProgramArguments</key>
    <array>
        <string>centmem</string>
        <string>capture</string>
        <string>run</string>
        <string>--watch</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
EOF
    echo "  installed launchd plist -> $plist_file"
  elif [[ "$(uname)" == "Linux" ]]; then
    local service_dir="$HOME/.config/systemd/user"
    local service_file="$service_dir/centmem-capture-watcher.service"
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would create systemd unit -> $service_file"
      return
    fi
    mkdir -p "$service_dir"
    cat << EOF > "$service_file"
[Unit]
Description=cent-mem Capture Watcher Service
After=network.target

[Service]
Type=simple
ExecStart=centmem capture run --watch
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
EOF
    echo "  installed systemd service -> $service_file"
  fi
}

uninstall_daemon_service() {
  local plist_file="$HOME/Library/LaunchAgents/com.aradenta.centmem.capture-watcher.plist"
  if [[ -f "$plist_file" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $plist_file"
    else
      rm -f "$plist_file"
      echo "  removed -> $plist_file"
    fi
  fi

  local service_file="$HOME/.config/systemd/user/centmem-capture-watcher.service"
  if [[ -f "$service_file" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      echo "  [dry-run] would remove $service_file"
    else
      rm -f "$service_file"
      echo "  removed -> $service_file"
    fi
  fi
}

# Main dispatch
if [[ "$MODE" == "uninstall" ]]; then
  echo "Uninstalling cent-mem hook adapters..."
  for h in "${SELECTED_HARNESSES[@]}"; do
    case "$h" in
      claude-code) uninstall_claude_code ;;
      cursor)      uninstall_cursor ;;
      codex)       uninstall_codex ;;
      antigravity) uninstall_antigravity ;;
      trae)        uninstall_trae ;;
      deepseek)    uninstall_deepseek ;;
      hermes)      uninstall_hermes ;;
    esac
  done
  uninstall_daemon_service
  echo "Uninstall complete."
  exit 0
fi

echo "Installing cent-mem hook adapters (triggers: $TRIGGERS, scope: $SCOPE)..."
for h in "${SELECTED_HARNESSES[@]}"; do
  echo "==> Configuring harness: $h"
  case "$h" in
    claude-code) install_claude_code ;;
    cursor)      install_cursor ;;
    codex)       install_codex ;;
    antigravity) install_antigravity ;;
    trae)        install_trae ;;
    deepseek)    install_deepseek ;;
    hermes)      install_hermes ;;
  esac
done

if [[ "$TRIGGERS" == *"message"* || "$TRIGGERS" == "all" ]]; then
  echo "==> Configuring background watcher daemon"
  install_daemon_service
fi

if [[ "$DRY_RUN" == "1" ]]; then
  echo ""
  echo "Dry run complete (no files were modified)."
else
  echo ""
  echo "Hooks installed successfully for ${#SELECTED_HARNESSES[@]} harness(es)."
fi
