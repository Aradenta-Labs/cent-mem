#!/usr/bin/env bash
# install_test.sh — verifies skill/adapters/hooks/install.sh against a temporary environment.
set -euo pipefail

FAILURES=0
pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1" >&2; FAILURES=$((FAILURES + 1)); }

INSTALLER="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/install.sh"
TMPHOME="$(mktemp -d)"
TMPPROJ="$(mktemp -d)"
trap 'rm -rf "$TMPHOME" "$TMPPROJ"' EXIT

export HOME="$TMPHOME"
cd "$TMPPROJ"

echo "==> Test 1: install to all harnesses with mock environment"
mkdir -p "$TMPHOME/.claude" "$TMPHOME/.cursor" "$TMPHOME/.gemini/antigravity" "$TMPHOME/.trae" "$TMPHOME/.codex" "$TMPHOME/.deepseek" "$TMPHOME/.hermes"
bash "$INSTALLER" --all >/dev/null

if [[ -f "$TMPPROJ/.claude/centmem.env" ]] && grep -q "BEGIN CENTMEM HOOK" "$TMPPROJ/.claude/centmem.env"; then
  pass "Claude Code env hook installed"
else
  fail "Claude Code env hook missing"
fi

if [[ -f "$TMPPROJ/.cursorrules" ]] && grep -q "BEGIN CENTMEM HOOK" "$TMPPROJ/.cursorrules"; then
  pass "Cursor rules hook installed"
else
  fail "Cursor rules hook missing"
fi

if [[ -x "$TMPHOME/.codex/bin/codex-centmem-shim" ]]; then
  pass "Codex shim created and executable"
else
  fail "Codex shim missing or not executable"
fi

if [[ -f "$TMPHOME/.gemini/antigravity/hooks/centmem-capture.md" ]]; then
  pass "Antigravity hook doc installed"
else
  fail "Antigravity hook doc missing"
fi

if [[ -f "$TMPHOME/.trae/hooks/centmem.json" ]]; then
  pass "Trae hook config installed"
else
  fail "Trae hook config missing"
fi

if [[ -f "$TMPHOME/.deepseek/hooks/centmem.json" ]]; then
  pass "Deepseek hook config installed"
else
  fail "Deepseek hook config missing"
fi

if [[ -f "$TMPHOME/.hermes/plugins/centmem.json" ]]; then
  pass "Hermes plugin config installed"
else
  fail "Hermes plugin config missing"
fi

echo "==> Test 2: idempotency (second run produces no duplicate blocks)"
bash "$INSTALLER" --all >/dev/null
block_count="$(grep -c "BEGIN CENTMEM HOOK" "$TMPPROJ/.claude/centmem.env" || true)"
if [[ "$block_count" -eq 1 ]]; then
  pass "Idempotency preserved (single marker block found)"
else
  fail "Duplicate marker blocks found: $block_count"
fi

echo "==> Test 3: dry-run modifies nothing"
file_count_before="$(find "$TMPHOME" "$TMPPROJ" -type f | wc -l | tr -d ' ')"
bash "$INSTALLER" --dry-run >/dev/null
file_count_after="$(find "$TMPHOME" "$TMPPROJ" -type f | wc -l | tr -d ' ')"
if [[ "$file_count_before" == "$file_count_after" ]]; then
  pass "dry-run modified zero files ($file_count_before == $file_count_after)"
else
  fail "dry-run modified file count: $file_count_before -> $file_count_after"
fi

echo "==> Test 4: targeted harness installation"
TMPPROJ2="$(mktemp -d)"
cd "$TMPPROJ2"
bash "$INSTALLER" --harness claude-code >/dev/null
if [[ -f "$TMPPROJ2/.claude/centmem.env" ]] && [[ ! -f "$TMPPROJ2/.cursorrules" ]]; then
  pass "targeted install only modified claude-code"
else
  fail "targeted install touched unexpected files"
fi
rm -rf "$TMPPROJ2"
cd "$TMPPROJ"

echo "==> Test 5: list mode"
list_out="$(bash "$INSTALLER" --list)"
if echo "$list_out" | grep -q "Selected harnesses"; then
  pass "list mode output valid"
else
  fail "list mode failed"
fi

echo "==> Test 6: uninstall cleans up hooks"
bash "$INSTALLER" --uninstall >/dev/null
if ! grep -q "BEGIN CENTMEM HOOK" "$TMPPROJ/.claude/centmem.env" 2>/dev/null && \
   [[ ! -f "$TMPHOME/.codex/bin/codex-centmem-shim" ]] && \
   [[ ! -f "$TMPHOME/.trae/hooks/centmem.json" ]]; then
  pass "uninstall cleaned up hooks and shims"
else
  fail "uninstall left behind artifacts"
fi

echo "==> Test 7: missing HOME error handling"
( unset HOME; bash "$INSTALLER" >/dev/null 2>&1 ) && {
  fail "installer should error when HOME is unset";
} || pass "installer errors cleanly when HOME is unset"

echo ""
if [[ "$FAILURES" -eq 0 ]]; then
  echo "ALL HOOK INSTALLER TESTS PASSED"
  exit 0
else
  echo "$FAILURES test(s) FAILED"
  exit 1
fi
