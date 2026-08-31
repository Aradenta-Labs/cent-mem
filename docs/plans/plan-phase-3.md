# Phase 3 (M3) — Skill Packaging & Harness Adapters

**Goal:** A fresh machine can install the binary + skill and have **two different agents** read and write the same shared memory with zero custom code. The skill becomes the canonical integration contract and is verified end-to-end.

**Exit criteria:**
- `skill/install.sh` idempotently installs the skill into all target harness locations.
- `skill/SKILL.md` exactly matches the CLI contract (automated consistency check).
- A scripted end-to-end test has two simulated agents (different `source_agent`/`source_session`) exchanging memory through `centmem`.
- A distributable binary + checksums is produced (GitHub Release).

---

## 3.1 Prerequisites

- Phase 2 complete (CLI is functionally complete: recall/set/put/get/timeline all work).
- Git remote + GitHub repo configured for releases.

## 3.2 Finalize `skill/SKILL.md`

The SKILL.md exists from the initial doc set. This phase makes it **authoritative and testable**:

1. Re-read [cli-contract.md](../cli-contract.md) and reconcile every command name, flag, JSON field, and exit code in SKILL.md.
2. Remove any forward-looking statements that aren't yet true (e.g. don't promise `centmem ui`).
3. Add a `## Commands` quick-reference table matching the final CLI exactly.
4. Ensure the "agent best practices" section maps 1:1 to actual flags (`--scope`, `--tags`, `--source-agent`, `--source-session`, `--inherit`, `--children`).

## 3.3 Contract consistency check (automated)

Create `scripts/check_contract.sh` + a Go test that fails if the docs drift from code:

- `internal/cli/contract_test.go`:
  - Enumerate the CLI's registered commands at runtime (expose a `commands.Names()` function).
  - Parse `docs/cli-contract.md` and `skill/SKILL.md`; assert every command appears in both docs and vice versa.
  - Assert exit-code constants in code (`0/1/2/3`) match the documented table.
- This makes "the docs are the contract" enforceable in CI.

**Implementation note:** to enumerate commands, the router must register subcommands into a `map[string]Command` with metadata (name, usage, flags). Add `internal/cli/registry.go` returning that map; both `main.go` and the test use it.

## 3.4 Installer hardening

`skill/install.sh`:
- Add `--dry-run` and `--list` flags (print destinations without copying).
- Make it idempotent (overwrite is safe; no duplicate nesting).
- Add a `--uninstall` flag (remove from targets).
- Handle `$HOME` unset gracefully.
- Print a summary of what was installed where.

Add a test `skill/install_test.sh` that:
1. Runs installer against a temp `$HOME`.
2. Asserts the SKILL.md landed in each expected location.
3. Runs again (idempotency) and asserts no errors.
4. Runs `--uninstall` and asserts cleanup.

## 3.5 Harness adapters

Finalize the existing adapter docs; add two more as they're commonly used:

- `skill/adapters/claude-code.md` (exists — finalize with real hook example).
- `skill/adapters/codex-cursor-continue.md` (exists — finalize).
- **New:** `skill/adapters/amazon-q-dev.md` and `skill/adapters/custom-harness.md` (generic `exec` + JSON parse snippet).

For each adapter document:
- Exact install location.
- Env vars to set (`CENTMEM_AGENT`, `CENTMEM_SID`, `CENTMEM_PROJ`).
- The one canonical recall recipe and one store recipe.

## 3.6 Two-agent end-to-end demo script

`scripts/demo-two-agents.sh` — a reproducible integration test:

```bash
#!/usr/bin/env bash
set -euo pipefail
export CENTMEM_HOME="$(mktemp -d)"
centmem init >/dev/null
PROJ=cent-mem

# Agent A (claude) stores a decision
centmem put --scope project:$PROJ --type note \
  --content "Deploy to Fly.io via GitHub Actions on merge to main." \
  --tags deploy --source-agent claude --source-session sessA >/dev/null

# Agent B (codex) recalls — must retrieve Agent A's memory via --inherit/global
RESULT=$(centmem recall "how do we deploy?" --scope project:$PROJ --top 3 --agent codex)

echo "$RESULT" | grep -q "Fly.io" && echo "PASS: Agent B retrieved Agent A's memory"
```

Wrap this as a Go integration test `cmd/centmem/e2e_two_agents_test.go` that shells out to the built binary (via `go run` or a prebuilt path) so it runs in CI.

## 3.7 Distribution

`scripts/build.sh` + `.github/workflows/release.yml`:

- Cross-compile with `goreleaser` (or manual `GOOS/GOARCH` matrix): `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
- Generate `SHA256SUMS`.
- Release workflow: on tag `v*`, build all targets, upload to GitHub Release with checksums.
- Optional: `brew tap` formula (defer — note as backlog).

## 3.8 Implementation order

1. Add `internal/cli/registry.go` (command metadata).
2. Write `contract_test.go` consistency checks.
3. Reconcile SKILL.md + cli-contract.md against the registry.
4. Harden `install.sh` (+ `--dry-run/--list/--uninstall`).
5. Write `install_test.sh`.
6. Finalize + add adapter docs.
7. Write `demo-two-agents.sh` + Go e2e test.
8. Set up release workflow + `build.sh`.

## 3.9 Checklist

- [ ] `commands.Names()` registry implemented
- [ ] Contract consistency test passes (docs == code)
- [ ] SKILL.md reconciled; no false promises
- [ ] `install.sh` idempotent + dry-run/list/uninstall
- [ ] `install_test.sh` green
- [ ] Adapter docs: claude, codex, cursor, continue, amazon-q, custom
- [ ] `demo-two-agents.sh` PASS
- [ ] `e2e_two_agents_test.go` in CI
- [ ] Release workflow builds 4 platforms + SHA256SUMS
- [ ] Tagged `v0.1.0` release produced

## 3.10 Phase 3 test plan (validation)

### Contract / docs

| Test | Validates |
|------|-----------|
| `TestContract_CommandsMatchDocs` | every CLI command appears in both docs; no undocumented command |
| `TestContract_ExitCodesMatchDocs` | code constants (0/1/2/3) == documented |
| `TestContract_JSONFieldsStable` | golden files unchanged from M1/M2 (no field renames) |
| `TestRegistry_NamesUnique` | no duplicate command names |

### Installer

| Test | Validates |
|------|-----------|
| `TestInstall_AllTargets` | SKILL.md present at each harness path |
| `TestInstall_Idempotent` | second run succeeds without error/duplication |
| `TestInstall_DryRun` | dry-run copies nothing |
| `TestInstall_Uninstall` | removes from all targets |

### Adapters (doc correctness)

| Test | Validates |
|------|-----------|
| `TestAdapters_RecipesParse` | each adapter's recipe uses only flags that exist in the CLI registry |

### End-to-end

| Test | Validates |
|------|-----------|
| `TestE2E_TwoAgentsShareMemory` | Agent A writes; Agent B recalls; cross-agent visibility works |
| `TestE2E_SessionIsolation` | session-scoped write not visible at project scope unless `--children` |
| `TestE2E_InstallThenUse` | fresh install + binary on PATH → full recall/set/put works |

### Distribution

| Test | Validates |
|------|-----------|
| `TestBuildMatrix` (CI) | all 4 GOOS/GOARCH compile |
| `TestSHA256` | checksum file matches built binaries |

**Definition of done:** all tests green; a tagged release exists; two-agent demo PASS on a clean machine; then proceed to [plan-phase-4.md](plan-phase-4.md).
