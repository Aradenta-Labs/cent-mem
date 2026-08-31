#!/usr/bin/env bash
# demo-two-agents.sh — end-to-end test that two DIFFERENT agents share memory.
#
# Agent A (claude) writes a decision; Agent B (codex) recalls it. Because memory
# is shared and recall defaults to inherit across scopes/agents, B sees A's
# memory with no custom integration code.
#
# This demo is fully offline: it does not require the embedding model. put/recall
# auto-create the store and fall back to keyword matching when no model is
# present (real semantic search works after `centmem init` on a networked
# machine). Override the binary with CENTMEM_BIN if centmem is not on PATH.
set -euo pipefail

CENTMEM_BIN="${CENTMEM_BIN:-centmem}"
PROJ="cent-mem"

export CENTMEM_HOME="$(mktemp -d)"
export CENTMEM_DB="$CENTMEM_HOME/centmem.db"
trap 'rm -rf "$CENTMEM_HOME"' EXIT

echo "==> temp store at $CENTMEM_HOME"

# Agent A (claude) stores a decision. The first write creates the schema.
echo "==> Agent A (claude) stores a deploy decision"
"$CENTMEM_BIN" put \
  --scope "project:$PROJ" \
  --type note \
  --content "Deploy to Fly.io via GitHub Actions on merge to main." \
  --tags deploy \
  --source-agent claude \
  --source-session sessA >/dev/null

# Agent B (codex) recalls — must retrieve Agent A's memory (cross-agent, inherit).
echo "==> Agent B (codex) recalls the shared memory"
RESULT="$("$CENTMEM_BIN" recall "how do we deploy?" --scope "project:$PROJ" --top 5)"

if ! echo "$RESULT" | grep -q "Fly.io"; then
  echo "FAIL: Agent B did not retrieve Agent A's memory" >&2
  echo "recall output:" >&2
  echo "$RESULT" >&2
  exit 1
fi

echo "PASS: Agent B retrieved Agent A's memory"
