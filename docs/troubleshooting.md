# Troubleshooting

Common issues when installing or running cent-mem, and how to fix them.

## `init` fails: model download blocked

```
{"error":{"code":"INTERNAL","message":"init: download model: ..."}}
```

**Cause:** the one-time model download from Hugging Face is blocked (corporate
proxy, offline machine, firewall). `init` makes no other network calls.

**Fix:**

1. Download the ONNX model manually and place it at
   `~/.centmem/models/bge-small-en-v1.5.onnx`:
   - `https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main/onnx/model.onnx`
2. Re-run `centmem init` (it will skip the download if the file exists and the
   sha256 matches).
3. Or, if you are fully offline, configure a proxy and retry.

## `doctor` reports `extensions` fail: `sqlite_vec=false`

**Cause:** the `sqlite-vec` extension could not be loaded when the database
opened (a shared-library issue on the platform).

**Fix:** reinstall/rebuild with the documented build steps
(`go build -tags fts5 ./cmd/centmem`). Ensure the binary wasn't built without
the vector bindings. This is almost always a build/packaging issue, not data
loss — your SQLite file is intact.

## `doctor` reports `extensions` fail: `fts5=false`

**Cause:** the binary was built without the `fts5` tag, so full-text search is
not compiled in.

**Fix:** rebuild with `go build -tags fts5 -o centmem ./cmd/centmem`, or install
a release binary that was built with the tag.

## `doctor` reports `model` fail: missing

```
"model=bge-small-en-v1.5 missing (run: centmem init)"
```

**Cause:** no model file at `~/.centmem/models/`.

**Fix:** run `centmem init` to download it. If the file exists but was placed
manually, verify its sha256 matches the catalog.

## `doctor` reports `model` fail: sha256 mismatch (warning)

A warning is emitted if the model file's hash differs from the catalog. The
store is still usable, but re-download the official model to be safe:

```bash
centmem init --force
```

## `doctor` reports `permissions` fail

**Cause:** `~/.centmem` is not `0700` or the DB file is not `0600`, so other
local users could read your memory.

**Fix:**

```bash
chmod 0700 ~/.centmem
chmod 0600 ~/.centmem/centmem.db
```

## `doctor` reports `schema_version` fail

**Cause:** the DB's schema version is older than the binary expects (an old DB
opened by a newer binary, or a partially-applied migration).

**Fix:** run `centmem init` to apply any pending migrations. cent-mem migrates
in place; your data is preserved.

## `database is locked` / `SQLITE_BUSY`

**Cause:** two processes opened the DB with conflicting locks, or a long
transaction (e.g. `compact` / `backup`) is running.

**Fix:** retry after a moment. cent-mem sets a 5 s busy timeout. Do not run two
`compact` or `restore` operations concurrently.

## Recall returns nothing even though I stored a memory

**Cause:** scope mismatch. Recall only returns memories from the given scope and
its **ancestors** (with `--inherit`, the default). It does **not** search
sibling or descendant scopes unless you pass `--children`.

**Fix:** recall from a broader scope, or pass `--children` to include
descendants:

```bash
centmem recall "deploy" --scope project:myapp --children
```

## I ran `restore` and lost recent data

**Cause:** `restore` replaces the live DB with the backup. Recent writes since
the backup are not in the backup.

**Fix:** a safety copy of your previous DB is saved at
`~/.centmem/centmem.db.pre-restore.bak`. To undo the restore, copy it back:

```bash
cp ~/.centmem/centmem.db.pre-restore.bak ~/.centmem/centmem.db
```

## Where are my files?

| Item | Default location |
|------|------------------|
| Home dir (0700) | `~/.centmem/` |
| Database | `~/.centmem/centmem.db` |
| Embedding model | `~/.centmem/models/bge-small-en-v1.5.onnx` |

Override with `CENTMEM_HOME` and `CENTMEM_DB`, or the `--home` / `--db` flags.
