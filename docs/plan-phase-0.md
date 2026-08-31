# Phase 0 (M0) — Foundation & Toolchain

**Goal:** A compilable Go repo with CI green, SQLite + sqlite-vec proven working, and the local embedding strategy validated. Nothing user-facing yet — this phase de-risks the two biggest technical unknowns (vec extension, ONNX Go bindings) and sets up the skeleton every later phase builds on.

**Exit criteria (all must pass):**
- `go build ./...` succeeds on darwin/arm64.
- `go test ./... -race` passes.
- A spike test inserts + queries a vector via `sqlite-vec` and gets the correct nearest neighbor.
- A spike test embeds one sentence with the ONNX model and asserts the output shape (e.g. 384 floats).
- CI (lint, vet, test, build) is green.

---

## 0.1 Prerequisites

- Go ≥ 1.22 installed (`go version`).
- `git` and GitHub repo created (for CI).
- Decide once, record in this plan's checklist:
  - [x] SQLite driver: **modernc.org/sqlite** (pure Go, no cgo) — recommended. Fall back to `mattn/go-sqlite3` only if benchmarks demand it.
  - [x] Embedding model default: `bge-small-en-v1.5` (384 dims).

## 0.2 Repo scaffold

Create the directory skeleton per [AGENTS.md](../AGENTS.md):

```
cent-mem/
├── cmd/centmem/main.go
├── internal/
│   ├── config/config.go
│   ├── store/
│   │   ├── store.go
│   │   ├── sql.go
│   │   └── migrations/m0001_init.sql
│   ├── scope/scope.go
│   ├── embed/embed.go
│   ├── search/search.go
│   └── compact/compact.go
├── docs/           (existing)
├── skill/          (existing)
├── testdata/
├── scripts/build.sh
├── .github/workflows/ci.yml
├── .gitignore
└── go.mod
```

**Steps:**
1. `go mod init github.com/aradenta-labs/cent-mem`
2. Add `.gitignore` with Go-standard entries (`bin/`, `*.db`, `.env`, `~/.centmem` paths are outside repo anyway).
3. Create placeholder `main.go` that prints version (JSON to stdout):
   ```go
   // cmd/centmem/main.go
   fmt.Println(`{"ok":true,"version":"0.0.0-dev"}`)
   ```
4. Create empty packages with a `doc.go` each so they compile and are importable.

## 0.3 Config package

`internal/config/config.go` — TOML loader with env overrides.

```go
package config

type Config struct {
    Home    string // default ~/.centmem
    DBPath  string // default <home>/centmem.db
    Model   ModelConfig
}

type ModelConfig struct {
    Name string // e.g. bge-small-en-v1.5
    Path string // <home>/models/<name>.onnx
    Dims int    // 384
}

func Load() (Config, error)   // reads config.toml, then applies CENTMEM_* env overrides
func (c Config) Ensure() error // creates Home, sets 0700 perms
```

**Env overrides:** `CENTMEM_HOME`, `CENTMEM_DB`, `CENTMEM_MODEL`.

## 0.4 SQLite + sqlite-vec spike (CRITICAL)

Prove vec works before writing real code.

**Files:**
- `internal/store/store_test.go` — spike tests only in M0.
- Add deps: `modernc.org/sqlite` and the Go wrapper for sqlite-vec.

**Spike test (`TestVec0Roundtrip`):**
```go
// 1. Open in-memory DB with vec0 extension loaded.
// 2. CREATE VIRTUAL TABLE t_vec USING vec0(id INTEGER PRIMARY KEY, embedding float[4]);
// 3. Insert two vectors: [1,0,0,0] (id 1), [0,1,0,0] (id 2).
// 4. Query nearest to [1,0.1,0,0] with LIMIT 1.
// 5. Assert id == 1.
```
If vec0 fails to load with the pure-Go driver, try the cgo driver and record the decision. **Do not proceed to M1 until this test passes.**

**Additional spike assertions:**
- FTS5 available: `CREATE VIRTUAL TABLE t_fts USING fts5(x);` then insert + MATCH works.
- WAL pragma accepted: `PRAGMA journal_mode=WAL;`.

## 0.5 Embedding spike (CRITICAL)

Prove we can embed text locally in Go.

**Files:** `internal/embed/embed.go`, `internal/embed/embed_test.go`.

**Approach options (pick the one that compiles + runs on darwin/arm64):**
1. `gonnx` (ONNX Runtime Go bindings) — preferred.
2. `k2onnx` or another maintained ONNX Go library.
3. Fallback: shell out to a tiny embedder binary (documented as a stopgap).

**Spike test (`TestEmbedShape`):**
```go
// 1. Load a bundled tiny ONNX model (commit a <5MB test fixture, NOT the full bge-small).
// 2. Embed("hello world") -> []float32
// 3. Assert len(vec) == expected dims for the fixture model.
// 4. Assert all values finite (no NaN/Inf).
```
**Note:** The full `bge-small-en-v1.5` (~100 MB) is NOT committed. It is downloaded at runtime by `centmem init` (M1) from a pinned URL with a sha256 checksum recorded in `internal/embed/models.go`.

**Record in `internal/embed/models.go`:**
```go
var ModelCatalog = map[string]ModelInfo{
    "bge-small-en-v1.5": {URL: "...", SHA256: "...", Dims: 384, File: "bge-small-en-v1.5.onnx"},
}
```

## 0.6 CI setup

`.github/workflows/ci.yml`:
- Trigger: push + PR to `main`.
- Matrix: `go-version: [1.22.x]`, `os: [macos-latest]` (add `ubuntu-latest` later).
- Steps: `actions/checkout` → `actions/setup-go` → `go vet ./...` → `go test ./... -race` → `go build ./...`.
- Optional: `golangci-lint` action.

## 0.7 Checklist

- [x] `go mod init` + deps added
- [x] Directory skeleton created; all packages compile
- [x] `config.Load()` + env overrides implemented + unit tested
- [x] **Vec spike test passes** (nearest neighbor correct)
- [x] **FTS5 spike test passes**
- [x] **Embed spike test passes** (correct dims, finite values)
- [x] Model catalog with pinned sha256 recorded
- [x] CI green

## 0.8 Risks & fallbacks (resolve HERE, not later)

| Risk | Fallback |
|------|----------|
| sqlite-vec won't load with pure-Go driver | Switch to `mattn/go-sqlite3` (cgo) behind a build tag |
| ONNX Go binding fails on darwin/arm64 | Use a subprocess embedder for M0; revisit binding in M2 |
| FTS5 missing in driver build | Use driver build with FTS5 enabled; verify in spike |
| Model download blocked in CI | Skip download in CI; test embedder only with tiny committed fixture |

## 0.9 Phase 0 test plan (validation)

| Test | Type | Validates |
|------|------|-----------|
| `TestVec0Roundtrip` | unit | sqlite-vec loads + nearest neighbor correct |
| `TestFTS5Available` | unit | FTS5 extension present |
| `TestWALMode` | unit | WAL pragma accepted |
| `TestEmbedShape` | unit | local embedding returns correct dims |
| `TestEmbedDeterministic` | unit | same input → same vector |
| `TestConfigDefaults` | unit | default home/db paths correct |
| `TestConfigEnvOverride` | unit | `CENTMEM_HOME` etc. override |
| CI build matrix | integration | compiles on target OS |

**Definition of done:** all tests above pass locally and in CI. Then proceed to [plan-phase-1.md](plan-phase-1.md).
