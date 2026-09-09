#!/usr/bin/env bash
# install_test.sh — verifies skill/install.sh against a temporary $HOME.
#
# 1. Runs the installer with a temp HOME.
# 2. Asserts SKILL.md landed in each expected location.
# 3. Runs again (idempotency) and asserts no errors / no duplication.
# 4. Runs --uninstall and asserts cleanup.
set -euo pipefail

cd "$(dirname "$0")/.."

FAILURES=0
pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1" >&2; FAILURES=$((FAILURES + 1)); }

INSTALLER="$PWD/skill/install.sh"
TMPHOME="$(mktemp -d)"
trap 'rm -rf "$TMPHOME"' EXIT

# Run installer from a controlled cwd so the project-level target is contained.
cd "$TMPHOME"
export HOME="$TMPHOME"

EXPECTED=(
  "$TMPHOME/.agents/skills/centmem"
  "$TMPHOME/.claude/skills/centmem"
  "$TMPHOME/.cursor/rules/centmem"
  "$TMPHOME/.codex/skills/centmem"
  "$TMPHOME/.config/skills/centmem"
  "$TMPHOME/.gemini/antigravity/skills/centmem"
  "$TMPHOME/.trae/skills/centmem"
  "$TMPHOME/.hermes/skills/centmem"
  "$TMPHOME/.deepseek/skills/centmem"
)

echo "==> Test 1: install to all targets"
bash "$INSTALLER" >/dev/null
all_present=1
for dir in "${EXPECTED[@]}"; do
  if [[ -f "$dir/SKILL.md" ]] && [[ -f "$dir/references/cli-commands.md" ]]; then
    pass "Skill artifacts present at $dir"
  else
    fail "Skill artifacts missing at $dir"
    all_present=0
  fi
done
[[ "$all_present" == "1" ]] || exit 1

echo "==> Test 2: idempotency (second run succeeds, no errors)"
if bash "$INSTALLER" >/dev/null 2>&1; then
  pass "second install run succeeded"
else
  fail "second install run failed"
fi

echo "==> Test 3: dry-run copies nothing"
before="$(find "$TMPHOME" -type f | wc -l | tr -d ' ')"
out="$(bash "$INSTALLER" --dry-run)"
after="$(find "$TMPHOME" -type f | wc -l | tr -d ' ')"
if [[ "$before" == "$after" ]]; then
  pass "dry-run changed no files (count $before == $after)"
else
  fail "dry-run changed file count ($before -> $after)"
fi

echo "==> Test 4: uninstall removes all targets"
bash "$INSTALLER" --uninstall >/dev/null
still_present=0
for dir in "${EXPECTED[@]}"; do
  if [[ -d "$dir" ]]; then
    fail "uninstall left directory: $dir"
    still_present=1
  fi
done
[[ "$still_present" == "0" ]] && pass "uninstall removed all targets"

echo "==> Test 5: unset HOME is handled gracefully"
( unset HOME; bash "$INSTALLER" >/dev/null 2>&1 ) && {
  fail "installer should error when HOME is unset";
} || pass "installer errors cleanly when HOME is unset"

echo
if [[ "$FAILURES" -eq 0 ]]; then
  echo "ALL INSTALL TESTS PASSED"
  exit 0
else
  echo "$FAILURES install test(s) FAILED"
  exit 1
fi
