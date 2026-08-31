# Phase 1 (M1) — Core Store & CRUD (keyword + facts + timeline)

**Goal:** Every write/read command works **without embeddings**. By the end of this phase you can `init`, `put`, `set`, `get`, `recall` (keyword-only), `timeline`, `list`, `forget`, and `stats` end-to-end against a real SQLite DB. Semantic ranking is deferred to [Phase 2](plan-phase-2.md) — `recall` here still works via FTS5 + fact lookup + timeline.

**Exit criteria:**
- All commands in [cli-contract.md](../cli-contract.md) except semantic ranking produce correct JSON.
- Golden-file tests lock the JSON contract (stdout shape + exit code) for every command.
- Scope inheritance: a `project:X` read returns matching `global` rows when `--inherit` (default).
- `init` downloads the embedding model file (sha256 verified) — the model isn't *used* yet, just stored for M2.

---

## 1.1 Prerequisites

- Phase 0 complete (vec0 + FTS5 + ONNX binding proven, CI green).
- Driver decision recorded in `internal/store/store.go` header comment.

## 1.2 Schema & migrations

Implement `internal/store/migrations/m0001_init.sql` per [data-model.md](../data-model.md):

Tables: `scopes`, `memories`, `memories_fts` (FTS5 external content), `embeddings` (created now, unused until M2), `embed_queue`, `events`, `meta`.

**Triggers** (place in same migration file):
- `memories_ai`, `memories_au`, `memories_ad` → keep `memories_fts` in sync.
- `events_insert` trigger → on `memories INSERT` append to `events`.

**Migration runner** (`internal/store/store.go`):
- `Open(cfg config.Config) (*Store, error)` — opens DB, sets WAL/busy_timeout/foreign_keys, loads vec0 + FTS5, runs pending migrations, records `meta.schema_version`.

**`meta` seed row:** `schema_version=1`, `embedding_model='bge-small-en-v1.5'`, `embedding_dims=384`.

## 1.3 Scope package — `internal/scope/scope.go`

Grammar from [cli-contract.md](../cli-contract.md):

```go
type Scope struct {
    Path       string // e.g. project:cent-mem/agent:claude
    ParentPath string
    Kind        string // global|project|agent|session
    Name       string
}

func Parse(s string) (Scope, error)        // validate grammar; lowercase
func Ancestors(s Scope) []string           // chain up to global (for inheritance)
func DescendantPrefix(s Scope) string      // for LIKE 'project:X%' children
```

**Validation rules:**
- `global` has no parent.
- `project:X` parent = `global`.
- `agent` segment requires a `project:` prefix.
- `session` segment requires both project and agent prefixes.
- Names match `[a-z0-9-_.]+`; reject on bad input → exit 1 with clear error.

## 1.4 Store layer — `internal/store/store.go`

Pure data access. No business logic, no JSON I/O.

Methods:
```go
func (s *Store) EnsureScope(ctx, sc scope.Scope) (id int64, err error)
func (s *Store) PutMemory(ctx, m Memory) (id int64, status string, err error)   // status: created|merged|queued
func (s *Store) SetFact(ctx, f Fact) (id int64, status string, err error)        // upsert
func (s *Store) GetFact(ctx, scopePath, key string, inherit bool) (*Fact, error)
func (s *Store) List(ctx, q ListQuery) ([]Memory, error)
func (s *Store) Forget(ctx, ids []int64, scopePath, key, tag *string) (int, error)
func (s *Store) Stats(ctx) (Stats, error)
func (s *Store) AppendEvent(ctx, e Event) error
func (s *Store) Close() error
```

Where `Memory`/`Fact`/`Event`/`ListQuery`/`Stats` are structs in `internal/store/types.go`.

**PutMemory dedup rule** (FR-8): compute `content_hash = sha256(scope_path||type||key||content)`; if an `active` row with the same hash exists in the last 60 s in the same scope, update `updated_at` and tags instead of inserting — returns `status="merged"`.

**SetFact**: `INSERT ... ON CONFLICT(scope_id, key) WHERE type='fact' DO UPDATE`.

**Transparency:** every write is a single `BEGIN...COMMIT` tx; FTS5 triggers fire inside the tx.

## 1.5 Keyword search — `internal/search/search.go` (M1 slice)

Implement only the non-semantic parts of `Hybrid`:

```go
func (s *Searcher) Keyword(ctx, q Query) ([]Ranked, error)   // FTS5 bm25
func (s *Searcher) Facts(ctx, q Query) ([]Ranked, error)     // key prefix
func (s *Searcher) Timeline(ctx, q Query) ([]Ranked, error)  // ORDER BY created_at
func (s *Searcher) Recall(ctx, q Query) ([]Ranked, error)    // fuses keyword+facts+timeline via RRF; semantic stub returns empty
```

**RRF** with `k=60`. Each contributing ranker returns up to `top*3` candidates; fusion dedups by `id` and slices to `top`.

**Filters** (apply post-fusion): `scope` (with inheritance/children), `type`, `tags`, `--since/--until`, `--agent`.

**`Query` struct** (one type, reused in M2):
```go
type Query struct {
    Text        string
    Scope       string
    Inherit     bool
    Children    bool
    Top         int
    Type        string
    Tags        []string
    Since, Until time.Time
    Agent       string
}
```

## 1.6 CLI commands — `cmd/centmem/`

One file per command in `cmd/centmem/commands/`:

| File | Command | Behavior |
|------|---------|----------|
| `init.go` | `init` | Ensure home + DB; download model if missing; verify sha256; print `{"ok":true,"db":...,"model":...,"dims":...}` |
| `put.go` | `put` | Parse scope/type/content/tags/source; `store.PutMemory`; queue embedding row; print `{"ok":true,"id":N,"scope":...,"status":...}` |
| `set.go` | `set` | Parse value as JSON (fallback string); `store.SetFact`; print `{"ok":true,"id":N,"key":...,"status":...}` |
| `get.go` | `get` | `store.GetFact`; exit 2 if not found; print `{"ok":true,"key":...,"value":...,"scope":...}` |
| `recall.go` | `recall` | Build `search.Query`; `search.Recall`; print `{"ok":true,"query":...,"results":[...]}` |
| `timeline.go` | `timeline` | Filter by `--since/--until/--limit`; print `{"ok":true,"entries":[...]}` |
| `list.go` | `list` | Pagination `--limit/--offset`; print array |
| `forget.go` | `forget` | Accept `--id` or `--scope/--key` or `--scope/--tag`; print `{"ok":true,"deleted":N}` |
| `stats.go` | `stats` | Print aggregate stats |
| `root.go` | main router | Subcommand dispatch; global flags; `--pretty` formatting; stderr error JSON |

**Error shape** (`internal/cli/errors.go`):
```go
type ErrOut struct {
    Code    string `json:"code"`     // NOT_FOUND|CONFLICT|INVALID|INTERNAL
    Message string `json:"message"`
    Hint    string `json:"hint,omitempty"`
}
```
Exit codes: 0 ok, 1 generic, 2 not found, 3 conflict.

## 1.7 Model download for `init`

`internal/embed/download.go`:
- Reads URL + sha256 from `ModelCatalog`.
- Streams to `<home>/models/<name>.onnx.tmp`, verifies sha256, renames to final path.
- Skip download if file exists with matching sha256.

**Note:** M1 just stores the model file; M2 is where it gets loaded. This keeps `init` honest about preparing the store.

## 1.8 Implementation order (recommended)

1. `migrations/m0001_init.sql` + `store.Open` + `EnsureScope` + smoke test.
2. `scope.Parse/Ancestors` + tests.
3. `store.PutMemory` + `SetFact` + `GetFact` + tests.
4. `search.Keyword` (FTS5) + `Facts` + `Timeline` + tests.
5. `search.Recall` (RRF over those three) + tests.
6. `cmd/centmem` router + `init`/`put`/`set`/`get` commands.
7. `recall`/`timeline`/`list`/`forget`/`stats` commands.
8. Golden file tests for every command.

## 1.9 Checklist

- [ ] `m0001_init.sql` written; `store.Open` runs it idempotently
- [ ] Scope grammar parser + validator with tests
- [ ] `PutMemory` dedup + `SetFact` upsert with tests
- [ ] `GetFact` with `--inherit` walking ancestors
- [ ] FTS5 trigger sync tests (insert/update/delete)
- [ ] Keyword + Facts + Timeline rankers with tests
- [ ] `Recall` RRF fusion returns deduped top-N
- [ ] All 9 commands produce the locked JSON shapes
- [ ] Exit codes match contract (0/1/2/3)
- [ ] `init` downloads model with sha256 verification
- [ ] Golden files committed under `testdata/`
- [ ] Coverage ≥ 70% on `internal/store`, `internal/scope`, `internal/search`

## 1.10 Phase 1 test plan (validation)

### Unit tests (`*_test.go`)

| Test | Package | Validates |
|------|---------|-----------|
| `TestMigrationsIdempotent` | store | running Open twice doesn't error; `meta.schema_version` correct |
| `TestEnsureScopeCreatesAncestors` | store | `project:X/agent:Y` creates both + `global` if missing |
| `TestPutMemoryDedup` | store | identical content within 60s → `status="merged"` |
| `TestSetFactUpsert` | store | same key twice → 1 row, `status="updated"` second time |
| `TestGetFactInherit` | store | missing in `project:X` → found in `global` |
| `TestGetFactNotFound` | store | returns sql.ErrNoRows (CLI maps to exit 2) |
| `TestFTS5SyncOnInsert/Update/Delete` | store | trigger keeps FTS in sync; MATCH reflects changes |
| `TestScopeParse_Valid/Invalid` | scope | accepts valid; rejects malformed names |
| `TestScopeAncestors` | scope | chain from session → agent → project → global |
| `TestKeywordRank_BM25` | search | higher term frequency ranks first |
| `TestFactsRank_Prefix` | search | `key LIKE 'user.%'` returns matches |
| `TestTimelineOrder` | search | results sorted by created_at |
| `TestRecall_Dedup` | search | same id from keyword+facts appears once |
| `TestRecall_Filters` | search | type/tags/scope filters applied |
| `TestRecall_InheritScope` | search | project read sees global memories |

### Golden file tests (`testdata/golden/`)

Each is a fixture run + assertion against a committed `.golden.json`:

| Test | Input | Asserts |
|------|-------|---------|
| `TestCLI_Init` | `centmem init` (in temp home) | stdout JSON shape; exit 0; model file present |
| `TestCLI_Put_Note` | `put --type note ...` | `{"ok":true,"id":N,"status":"queued"}` |
| `TestCLI_Put_Log` | `put --type log ...` | shape; created row type=log |
| `TestCLI_Set_Fact` | `set --key k --value '"v"'` | `status:"created"` then `updated` on repeat |
| `TestCLI_Get_Found` | `get --key k` | JSON with value |
| `TestCLI_Get_NotFound` | `get --key missing` | exit 2; stderr JSON `code:"NOT_FOUND"` |
| `TestCLI_Recall_Keyword` | seed + `recall "term"` | ranked results, `matched_by:["keyword"]` |
| `TestCLI_Timeline` | seed logs + `timeline --since 24h` | chronological entries |
| `TestCLI_List` | seed + `list` | paginated |
| `TestCLI_Forget` | seed + `forget --id N` | `deleted:1`; row gone |
| `TestCLI_Stats` | seed + `stats` | counts match seed |
| `TestCLI_ExitCodes` | various | 0/1/2/3 mapped correctly |
| `TestCLI_Pretty` | `--pretty` | pretty output differs from JSON (no parsing) |

### Integration test

| Test | Validates |
|------|-----------|
| `TestE2E_WriteThenRecallAcrossScopes` | write in `global`, recall from `project:X` with `--inherit` → returns it |

**Definition of done:** all tests above pass; `go test ./... -race` green; then proceed to [plan-phase-2.md](plan-phase-2.md).
