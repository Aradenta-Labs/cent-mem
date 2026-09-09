#!/usr/bin/env bash
# centmem-helper.sh — Quick diagnostic and helper tool for agents & users

set -eo pipefail

echo "== cent-mem Diagnostic & Scope Helper =="

# Check binary in PATH
if ! command -v centmem &> /dev/null; then
  echo "WARNING: 'centmem' command not found in PATH."
  echo "Make sure Go bin is in your PATH:"
  echo "  export PATH=\$PATH:\$(go env GOPATH)/bin"
  exit 1
fi

echo "✓ centmem binary found at: $(command -v centmem)"

# Detect project name
if [ -n "${CENTMEM_PROJ:-}" ]; then
  PROJECT_NAME="$CENTMEM_PROJ"
elif [ -d ".git" ]; then
  PROJECT_NAME=$(basename "$(git rev-parse --show-toplevel 2>/dev/null || pwd)")
else
  PROJECT_NAME=$(basename "$PWD")
fi

echo "✓ Active Project Scope: project:$PROJECT_NAME"

# Check doctor
echo -n "Checking centmem doctor... "
if centmem doctor > /dev/null 2>&1; then
  echo "✓ HEALTHY"
else
  echo "✗ Doctor reported issues. Run 'centmem doctor --pretty' for details."
fi

# Check LLM and Agent engine configuration
echo -n "Checking LLM backend... "
if LLM_JSON=$(centmem config get llm 2>/dev/null); then
  LLM_BACKEND=$(echo "$LLM_JSON" | grep -o '"backend":"[^"]*"' | cut -d':' -f2 | tr -d '"')
  LLM_MODEL=$(echo "$LLM_JSON" | grep -o '"model":"[^"]*"' | cut -d':' -f2 | tr -d '"')
  echo "✓ Configured ($LLM_BACKEND, model: $LLM_MODEL)"
else
  echo "✗ Could not query LLM config"
fi

# Quick recall test
echo -e "\nRecent memories for project:$PROJECT_NAME:"
centmem recall "conventions decisions" --scope "project:$PROJECT_NAME" --top 3 --pretty 2>/dev/null || true

