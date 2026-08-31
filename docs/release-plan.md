# Release & QA Plan

**Scope:** how v1.0 (and future releases) are cut, validated, and shipped.

---

## 1. Release process

1. **Version bump** — update `cmd/centmem/main.go` version const + `CHANGELOG.md` (add if missing).
2. **Tag** — `git tag vX.Y.Z` (semver; `v0.1.0` at end of M3, `v1.0.0` at end of M4).
3. **CI release workflow** — on tag push:
   - Cross-compile darwin/arm64, darwin/amd64, linux/amd64, linux/arm64.
   - Generate `SHA256SUMS`.
   - Attach binaries + checksums to a GitHub Release.
4. **Skill install docs** — release notes include the one-line installer command.

## 2. Pre-release QA checklist

Run all of the following **before** tagging:

- [ ] `go build ./...` on all 4 targets
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` green
- [ ] `go test ./... -bench=.` meets thresholds (see Phase 2 plan)
- [ ] Contract consistency test (M3) green — docs == code
- [ ] `demo-two-agents.sh` PASS on a clean machine
- [ ] Fresh-user install walkthrough (README/getting-started) executed manually
- [ ] `doctor` reports healthy on a fresh store
- [ ] `backup` → `restore` round-trip verified
- [ ] Offline test: disable network; `recall`/`put` still work (after `init`)

## 3. Post-release smoke

On the released binary (not `go run`):

```bash
centmem init
centmem set --scope project:demo --key k --value '"v"'
centmem put --scope project:demo --type note --content "test" --tags t
centmem recall "test" --scope project:demo --top 1
centmem timeline --scope project:demo
centmem stats
centmem doctor
```

All must exit 0 with valid JSON.

## 4. Rollback criteria

Roll back / yank the release if:
- Any golden/contract test fails on the tagged binary.
- `recall` returns wrong results on the paraphrase test.
- Data loss on `restore` round-trip.
- p95 read > 1.5 s on 10k memories (2x target) on reference hardware.

## 5. Versioning & compatibility

- v1.x: additive-only changes; JSON field names, command names, exit codes are stable.
- Breaking change → v2.0 with migration notes and a compat shim where feasible.
- Schema migrations are forward-only; `doctor` detects version mismatch and instructs.
