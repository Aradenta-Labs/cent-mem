#!/usr/bin/env bash
# check_contract.sh — enforce "the docs are the contract".
#
# Runs the Go contract tests that assert the registered CLI command surface,
# exit codes, and stable JSON fields match docs/cli-contract.md and
# skill/SKILL.md. Exits non-zero if any contract drifts from the code.
#
# Usage:
#   scripts/check_contract.sh
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Running CLI contract consistency tests (docs == code)"
go test -tags fts5 ./cmd/centmem/ -run 'TestContract|TestRegistry' -count=1 -v

echo "==> Contract OK: docs/cli-contract.md, skill/SKILL.md, and CLI are in sync."
