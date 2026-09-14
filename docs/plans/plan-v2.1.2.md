# v2.1.2 — Memory Export / Import

**Version:** 2.1.2  
**Owner:** Aradenta Labs  
**Status:** Complete — Fully implemented and verified across all phases  
**Depends on:** v2.0.0 (store, `PutMemory`, `List`, Web UI `/api/export`)  
**Source:** [docs/v2-nice-to-have.md](../v2-nice-to-have.md) — Quick Win #3

---

## 0. Executive Summary

`centmem export` dumps memories to a portable JSON or CSV file; `centmem import <file>` ingests them into any centmem instance. The Web UI gains an Import button backed by a new `POST /api/import` endpoint.

**Why this version (v2.1.2)?** There is currently no backup/transfer mechanism:
- `centmem backup` produces a raw SQLite file (`VACUUM INTO`) — not portable across schema versions or embeddable in other tools.
- The Web UI's `GET /api/export` already produces JSON/CSV, but there is no import path at all.

This milestone completes the round-trip: logical export → portable file → logical import. No schema changes. No new Go packages. The core work is:
1. Two new CLI commands: `export` and `import` (+ `commands.go` registry entries).
2. A `POST /api/import` endpoint in `internal/ui/server.go` (file upload + JSON body).
3. A shared export/import codec so CLI and Web UI produce/consume the **same file format**.

### Design decisions (locked via `/grill-me`)

| Question | Decision | Rationale |
|----------|----------|-----------|
| CLI surface | **New `export` + `import` commands** | Matches nice-to-have doc; `backup`/`restore` remain binary-SQLite operations — different purpose |
| Export formats | **JSON** (canonical) + **CSV** (secondary) | Reuses the shapes already served by `GET /api/export`; JSON is the import-compatible format |
| Import duplicates | **Skip duplicates (idempotent)** | Importing the same file twice is a no-op; matches "share a snapshot" use case |
| Scope on import | **Preserve original scope** | Records carry their scope path; importing re-creates scopes via `EnsureScope` |
| Import API | **`POST /api/import`** (Web UI parity) | Completes the UI round-trip; consistent with existing `/api/export` |
| CLI export cap | **No 10,000 cap** | CLI streams pages internally; file-based export should handle large stores |
| Dry-run | **`--dry-run` on import** | Parse + validate + report (inserts vs skips) without writing |

---

## 1. Scope & Anti-goals

### In scope
- `centmem export` CLI: `--scope`, `--format json|csv`, `--type`, `--tags`, `--since`, `--until`, `--output <path>` (default stdout), no hard row cap.
- `centmem import` CLI: positional `<file>` arg, `--dry-run` flag, idempotent insert via `content_hash`, scope auto-creation via `EnsureScope`.
- Shared codec package `internal/export/` used by both CLI handlers and `POST /api/import`.
- `POST /api/import` endpoint accepting `multipart/form-data` file upload and raw JSON body; returns `{imported, skipped, failed}` counts.
- Web UI: "Import" button in the Memory Browser toolbar (file picker → POST → toast with counts).
- Golden-file tests for both CLI commands; endpoint tests for `/api/import`.

### Anti-goals (explicit)
- No export/import of **links** (`memory_links`), **proposals**, or **conversations** — memories only.
- No binary SQLite export/import (that's `backup`/`restore`).
- No embedding-vector export/import — vectors are recomputed on import via the embed queue.
- No cross-instance sync — this is a manual, file-based transfer.
- No NDJSON format in this version.
- No schema migrations.
- No change to existing `GET /api/export` behavior.

---

## 2. File Format (Canonical JSON)

The JSON produced by `centmem export` is the canonical import format. It mirrors the existing `GET /api/export` envelope with one addition (`format_version`):

```json
{
  "format": "centmem-export",
  "format_version": 1,
  "exported_at": "2026-09-14T09:30:00Z",
  "scope": "project:cent-mem",
  "total": 2,
  "memories": [
    {
      "id": 142,
      "scope": "project:cent-mem",
      "type": "note",
      "content": "Decided to use RRF with k=60 for hybrid search fusion.",
      "tags": ["architecture", "search"],
      "source_agent": "claude",
      "source_session": "s-2026-09-10",
      "created_at": 1789200000
    },
    {
      "id": 143,
      "scope": "project:cent-mem/agent:claude",
      "type": "fact",
      "key": "build.tool",
      "value_json": "\"go\"",
      "tags": ["build"],
      "source_agent": "claude",
      "created_at": 1789200100
    }
  ]
}
```

**Rules:**
- `format` must be exactly `"centmem-export"` and `format_version` must be `1`. Import rejects anything else with exit code 1 and a clear error.
- `memories[].id` is included for traceability but **ignored on import** (target store assigns its own IDs).
- `memories[].scope` is the scope **path** (string), not `scope_id`. Import resolves/creates the scope via `EnsureScope`.
- `created_at` is Unix seconds. On import, when absent, `now` is used. When present, it is preserved only for **display purposes** — imported memories get a fresh `created_at` of the import time. (Rationale: preserves the "when did this enter this store" semantics; `original_created_at` is retained inside the `content` for reference only in the `--dry-run` report.) **Decision simplified:** imported records always get `created_at = now`; the original timestamp is logged in the dry-run report.
- `key` + `value_json` present only for `fact` memories.
- CSV export is for human/table use only — **CSV files cannot be imported** (import rejects `.csv` with a clear error).

---

## 3. New Package: `internal/export`

### 3.1 File structure

```
internal/export/
├── doc.go        — package doc
├── codec.go      — Envelope + MemoryRecord types, Marshal/Unmarshal, validation
├── export.go     — Export(ctx, store, ExportQuery) — paginated streaming dump
├── import.go     — Import(ctx, store, reader) — idempotent ingest + report
├── csv.go        — CSV writer for export (read-only format)
└── *_test.go     — unit tests
```

### 3.2 Types

```go
// Envelope is the canonical export file format.
type Envelope struct {
    Format        string         `json:"format"`
    FormatVersion int            `json:"format_version"`
    ExportedAt    time.Time      `json:"exported_at"`
    Scope         string         `json:"scope"`
    Total         int            `json:"total"`
    Memories      []MemoryRecord `json:"memories"`
}

// MemoryRecord is a portable memory (no internal IDs except for traceability).
type MemoryRecord struct {
    ID            int64    `json:"id"`
    Scope         string   `json:"scope"`
    Type          string   `json:"type"`
    Content       string   `json:"content"`
    Key           string   `json:"key,omitempty"`
    ValueJSON     string   `json:"value_json,omitempty"`
    Tags          []string `json:"tags,omitempty"`
    SourceAgent   string   `json:"source_agent,omitempty"`
    SourceSession string   `json:"source_session,omitempty"`
    CreatedAt     int64    `json:"created_at"`
}

// ImportReport is returned by Import and --dry-run.
type ImportReport struct {
    Total    int      `json:"total"`
    Imported int      `json:"imported"`
    Skipped  int      `json:"skipped"` // duplicate content_hash already present
    Failed   int      `json:"failed"`
    Errors   []string `json:"errors,omitempty"`
}
```

### 3.3 `Export(ctx, store, q ExportQuery) (Envelope, error)`

Pagination loop over `store.List()` with a page size of **500**:

```go
type ExportQuery struct {
    Scope     string
    Type      string
    Tags      []string
    Since     time.Time
    Until     time.Time
    Agent     string
    Session   string
    // no Limit field — exports everything
}
```

1. Resolve scope IDs once via `store.ResolveScopeIDs(ctx, sc, inherit, children)` (same semantics as `/api/export`: `inherit=false`, `children=true`).
2. Loop `List()` with `Offset` increments of 500 until fewer than 500 rows return.
3. Map `store.Memory` → `MemoryRecord` (drop `scope_id`, `content_hash`, `status`, `access_count`, `last_accessed_at`, `updated_at`, `score`, `matched_by`).
4. Return `Envelope` with `Total = len(records)`.

### 3.4 `Import(ctx, store, r io.Reader) (ImportReport, error)`

1. `json.Decoder` → `Envelope`. Validate `Format == "centmem-export"` and `FormatVersion == 1`.
2. Reject empty `memories` with `cli.Invalidf`.
3. For each `MemoryRecord`:
   a. `scope.Parse(rec.Scope)` — invalid scope → count as Failed with error entry.
   b. `store.EnsureScope(ctx, sc)` — creates the scope and its ancestors.
   c. `store.PutMemory(ctx, MemoryInput{Scope: rec.Scope, Type: rec.Type, Content: rec.Content, Key: rec.Key, ValueJSON: rec.ValueJSON, Tags: rec.Tags, SourceAgent: rec.SourceAgent, SourceSession: rec.SourceSession})`.
   d. Idempotency: **before** calling `PutMemory`, check for an existing active memory with the same `content_hash` in the same scope. The hash is computed with the same function used by `PutMemory` (`store.HashContent()` — verify exact function name in `internal/store/store.go`; if it's unexported, compute `sha256(content + type + key)` locally in `internal/export` and accept minor divergence risk documented in the code).
   e. `status == "merged"` from `PutMemory` (60-second dedup window) also counts as **Skipped**, not Imported.
4. Return `ImportReport`.

> **Hash check note:** `PutMemory` already dedupes within a 60-second window by `content_hash`. For a correct long-term idempotent import, `Import` must perform its own pre-check. The implementation must reuse the store's hash helper. If that helper is unexported, export a `ContentHash(scope, type, content, key)` function from `internal/store` in this milestone (small, additive change).

### 3.5 `WriteCSV(w io.Writer, records []MemoryRecord) error`

Same columns as `GET /api/export` CSV: `id, scope, type, content, key, value_json, tags, source_agent, source_session, created_at, updated_at` (updated_at empty for exported records — column kept for parity).

---

## 4. CLI Commands

### 4.1 `centmem export`

```
centmem export --scope <scope> [--format json|csv] [--type t] [--tags a,b]
               [--since d] [--until d] [--agent a] [--output <path>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--scope` | `global` | Scope path |
| `--format` | `json` | `json` or `csv` |
| `--type` | `""` | Memory type filter |
| `--tags` | `""` | Comma-separated tags |
| `--since` | `""` | Duration string (`24h`, `7d`) |
| `--until` | `""` | Duration string |
| `--agent` | `""` | Source agent filter |
| `--output` | `""` | Output file path. Empty = stdout |

**Output rules (contract):**
- Default output is JSON (per AGENTS.md ground rule #2). This is a deliberate exception to the file format: **`centmem export` prints a status envelope to stdout, not the raw export file**, unless `--output` is given. Rationale: JSON-on-stdout keeps the CLI contract consistent; the export file itself is written atomically via `--output`.

```json
{"ok": true, "file": "centmem-export-project-cent-mem-20260914T093000Z.json", "total": 312, "size_bytes": 12345}
```

- With `--output <path>`: writes the file, prints the status envelope above to stdout.
- Without `--output`: writes the **export file to stdout** and prints the status envelope to **stderr** as JSON (mirrors the `--quiet`/`--verbose` stderr contract). This makes `centmem export --scope project:x > dump.json` work naturally.
- `--format csv` behaves identically (CSV stream instead of JSON).
- No row cap (internal pagination, page size 500).

### 4.2 `centmem import`

```
centmem import <file> [--dry-run]
```

| Arg/Flag | Description |
|----------|-------------|
| `<file>` | Path to a `centmem-export` JSON file. `-` reads from stdin. |
| `--dry-run` | Parse + validate + report, write nothing |

**Output:**

```json
{"ok": true, "file": "dump.json", "total": 312, "imported": 290, "skipped": 22, "failed": 0}
```

With `--dry-run`, the same shape plus `"dry_run": true`.

**Rules:**
- Invalid format / wrong `format_version` → exit 1 with `cli.Invalidf("import: unsupported file format (expected centmem-export v1)")`.
- CSV input → rejected with the same clear error.
- `-` (stdin) supported for piping: `centmem export --scope project:x | centmem import -`.
- Individual record failures do not abort the whole import; they accumulate in `failed` and `errors`.
- Import is **not transactional** per-file (each `PutMemory` is its own transaction); a crash mid-import can be recovered by re-running (idempotent).

### 4.3 `commands.go` registry additions

```go
"export": {cmdExport, "export --scope <scope> [--format json|csv] [--type t] [--tags a,b] [--since d] [--until d] [--agent a] [--output <path>]", []string{"--scope", "--format", "--type", "--tags", "--since", "--until", "--agent", "--output"}},
"import": {cmdImport, "import <file> [--dry-run]", []string{"--dry-run"}},
```

---

## 5. Web UI: `POST /api/import`

### 5.1 Endpoint

```
POST /api/import
```

Registered in [internal/ui/server.go](../../internal/ui/server.go) next to `GET /api/export`.

**Request — two accepted content types:**

1. `multipart/form-data` with a `file` field (browser file picker).
2. `application/json` raw body = the export envelope itself (API clients).

### 5.2 Response

```json
{"ok": true, "total": 312, "imported": 290, "skipped": 22, "failed": 0, "errors": []}
```

On partial failure: `"ok": false`, HTTP 422, same body shape with `failed > 0`.

### 5.3 Handler sketch

```go
mux.HandleFunc("POST /api/import", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if cfg.Store == nil {
        // 503 store_unavailable
    }
    var reader io.Reader
    switch ct := r.Header.Get("Content-Type"); {
    case strings.HasPrefix(ct, "multipart/form-data"):
        r.ParseMultipartForm(32 << 20) // 32 MB max
        fh, _, err := r.FormFile("file")
        if err != nil { /* 400 invalid_payload */ }
        defer fh.Close()
        reader = fh
    default:
        reader = io.LimitReader(r.Body, 64 << 20) // 64 MB JSON body cap
    }
    report, err := exportpkg.Import(r.Context(), cfg.Store, reader)
    if err != nil {
        // 422 invalid_export: malformed envelope
    }
    status := http.StatusOK
    if report.Failed > 0 {
        status = http.StatusUnprocessableEntity
    }
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(map[string]any{
        "ok":       report.Failed == 0,
        "total":    report.Total,
        "imported": report.Imported,
        "skipped":  report.Skipped,
        "failed":   report.Failed,
        "errors":   report.Errors,
    })
})
```

### 5.4 Web UI button

- **Location:** Memory Browser toolbar, next to the existing Export button.
- **Interaction:** file picker (`<input type="file" accept="application/json">`) → POST `/api/import` with `multipart/form-data` → toast on completion: `"Imported 290 · skipped 22 · failed 0"`.
- **Scope note:** Import preserves scopes from the file. The UI shows a static hint under the picker: "Imported memories keep their original scopes."

---

## 6. Phased Implementation Plan

### Phase 1 — `internal/export` package

1. Create `internal/export/` with `doc.go`, `codec.go`, `export.go`, `import.go`, `csv.go`.
2. Export a content-hash helper from `internal/store` if needed (see §3.4 note).
3. Unit tests:
   - `TestEnvelopeRoundTrip` — marshal → unmarshal preserves fields.
   - `TestExportEmptyScope` — empty store returns `Total: 0`.
   - `TestExportPagination` — 1,250 memories → export returns all (validates page loop).
   - `TestImportIdempotent` — import same file twice → second run `imported: 0, skipped: N`.
   - `TestImportPreservesScope` — nested scope `project:x/agent:a` re-created via `EnsureScope`.
   - `TestImportRejectsBadFormat` — non-centmem JSON → error.
   - `TestImportRejectsCSV` — CSV bytes → error.
   - `TestImportPartialFailure` — one invalid scope among valid records → `failed: 1`, others imported.

### Phase 2 — CLI commands

1. Create `cmd/centmem/handlers_export.go` with `cmdExport` and `cmdImport`.
2. Register both in `commands.go`.
3. Golden-file tests in `cmd/centmem/golden_test.go` (or a new `export_import_test.go`):
   - `export --scope project:x --output tmp.json` → status envelope + file exists + valid envelope.
   - `import tmp.json` → `{ok, imported: N, skipped: 0}`.
   - `import tmp.json` (second run) → `{ok, imported: 0, skipped: N}`.
   - `import --dry-run tmp.json` → `dry_run: true`, store unchanged.
   - `import bad.csv` → exit 1, clear error.
   - Contract test: `export` and `import` appear in `buildRegistry()` with correct usage/flags.
4. Update `docs/cli-contract.md` with the two new commands (no version bump; additive).

### Phase 3 — `POST /api/import` + Web UI

1. Add `POST /api/import` handler to `internal/ui/server.go`.
2. Endpoint tests in `internal/ui/server_test.go`:
   - `TestImportEndpointMultipart` — upload file → counts correct.
   - `TestImportEndpointJSONBody` — raw JSON body → counts correct.
   - `TestImportEndpointStoreNil` → 503.
   - `TestImportEndpointBadEnvelope` → 422.
   - `TestImportEndpointIdempotent` — double POST → second `imported: 0`.
3. Web UI: add Import button + file picker + toast (frontend build session, same constraint as v2.1.0 Phase 2 — bundle built separately and dropped into `internal/ui/dist/`).

### Phase 4 — Polish & verify

1. `go build -tags fts5 ./...` + `go vet -tags fts5 ./...`.
2. `go test -tags fts5 ./... -race`.
3. `graphify update .`.
4. Manual acceptance:
   ```bash
   centmem export --scope project:cent-mem --output /tmp/dump.json
   centmem import /tmp/dump.json
   centmem import /tmp/dump.json            # idempotent: skipped only
   centmem export --scope project:cent-mem | centmem import -   # stdin pipe
   centmem import /tmp/dump.json --dry-run
   ```

---

## 7. Test Strategy

| Layer | Tests |
|-------|-------|
| `internal/export` | 8 unit tests (roundtrip, pagination, idempotency, scope preservation, format rejection, CSV rejection, partial failure, empty) |
| `cmd/centmem` | 6 golden/contract tests (export file, import counts, second-run idempotency, dry-run, bad CSV, registry) |
| `internal/ui` | 5 endpoint tests (multipart, JSON body, 503, 422, idempotent) |
| E2E | Manual acceptance script in Phase 4 |

---

## 8. Dependencies & Risks

| Item | Risk | Mitigation |
|------|------|-----------|
| Content-hash helper unexported in store | Low | Export a `store.ContentHash()` function (additive, no schema change) |
| Import of huge files blocks the UI request | Medium | 64 MB JSON body cap + 32 MB multipart cap; CLI is the recommended path for big files |
| Duplicate detection across scopes | Medium | Idempotency check scoped to `(scope_id, content_hash)` — matches `PutMemory` semantics |
| `created_at` semantics | Low | Imported memories get `now`; original timestamps appear only in dry-run report |
| CSV import confusion | Low | Clear rejection error; docs state CSV is export-only |
| Partial import (crash mid-file) | Low | Idempotent re-run completes the transfer |

---

## 9. Acceptance Criteria (Done Definition)

- [x] `centmem export --scope project:x --output dump.json` produces a valid `centmem-export` v1 JSON file with all memories (no 10k cap).
- [x] `centmem import dump.json` imports with correct `{imported, skipped, failed}` counts.
- [x] Running import twice is idempotent (second run: all skipped).
- [x] `centmem import dump.json --dry-run` reports without writing.
- [x] `centmem export ... | centmem import -` works via stdin.
- [x] Importing a CSV file fails with a clear error message.
- [x] `POST /api/import` works via multipart and raw JSON, returns correct counts, 422 on bad envelope, 503 when store is nil.
- [x] Web UI has an Import button that uploads a file and shows a result toast.
- [x] `go test -tags fts5 ./... -race` passes.
- [x] `docs/cli-contract.md` includes `export` and `import`.

---

## 10. Future Enhancements (out of scope for v2.1.2)

- **NDJSON streaming format** — line-delimited records for very large stores and `jq` pipelines.
- **Link graph export** — include `memory_links` edges in the envelope (schema v2 of the format).
- **Incremental export** — `--since <timestamp>` producing only memories changed after a watermark.
- **Import from URL** — `centmem import https://...` fetching a remote snapshot.
- **Compressed export** — transparent gzip (`.json.gz`) support.

---

## 11. Changelog

| Date | Change |
|------|--------|
| 2026-09-14 | Completed implementation across Phases 1–4. All acceptance criteria met. |
| 2026-09-14 | Initial plan created via `/grill-me` session. Design locked. |
