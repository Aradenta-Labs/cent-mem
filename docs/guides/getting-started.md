# Getting started with cent-mem

This guide takes a brand-new user from zero to a working shared memory in under
10 minutes. cent-mem runs fully locally — no account, no API keys for core use.

## Prerequisites

- **Go 1.22+** (or a prebuilt release binary from the GitHub releases page).
- A shell (bash/zsh) on macOS or Linux. Windows is supported via WSL.

## 1. Install the binary

Build from source:

```bash
git clone https://github.com/aradenta-labs/cent-mem.git
cd cent-mem
go build -tags fts5 -o centmem ./cmd/centmem
sudo mv centmem /usr/local/bin/
centmem --help
```

> The `-tags fts5` flag is required: it compiles SQLite's full-text search
> (FTS5) into the driver that cent-mem uses for keyword recall.

Alternatively, download a prebuilt binary from the **Releases** page and place
it on your `PATH`.

## 2. Initialize

```bash
centmem init
```

This creates `~/.centmem/` (permissions `0700`), the SQLite database, and
downloads the default embedding model `bge-small-en-v1.5` (one-time, ~130 MB).
You only run `init` once per machine.

Output looks like:

```json
{"ok":true,"db":"/Users/you/.centmem/centmem.db","model":"bge-small-en-v1.5","dims":384}
```

## 3. Store your first memory

Facts (structured, upserted by key):

```bash
centmem set --scope project:myapp --key user.timezone --value '"Asia/Jakarta"'
```

Notes (free-form, searchable):

```bash
centmem put --scope project:myapp --type note \
  --content "we deploy via github actions on merge to main" \
  --tags decision --source-agent claude
```

## 4. Recall

```bash
centmem recall "how do we deploy" --scope project:myapp --top 5
```

cent-mem runs **hybrid search**: semantic embeddings + FTS5 keyword + facts +
timeline, fused with Reciprocal Rank Fusion (RRF). Results are JSON:

```json
{"ok":true,"query":"how do we deploy","results":[
  {"id":1,"type":"note","scope":"project:myapp","content":"we deploy via github actions...","tags":["decision"],"created_at":1788000000,"score":0.9,"matched_by":["semantic","keyword"]}
]}
```

## 5. Keep it lean with `compact`

Old notes and logs are summarized automatically per your retention policy.
Run it manually (dry-run first to see what would change):

```bash
centmem compact --dry-run
centmem compact
```

## 6. Check health & back up

```bash
centmem doctor                 # health checks (exit 0 = healthy)
centmem backup --to ~/backups/centmem-$(date +%F).db
centmem restore --from ~/backups/centmem-2026-08-31.db
```

## 7. Configure retention

Retention defaults are shown in [data-model.md](../data-model.md). Override them per-machine
with environment variables, e.g.:

```bash
export CENTMEM_RETENTION_NOTE_SUMMARIZE_AFTER_DAYS=60
export CENTMEM_RETENTION_LOG_SUMMARIZE_AFTER_DAYS=21
```

## Where to go next

- Read [cli-contract.md](../cli-contract.md) for the exact CLI and JSON contract.
- Read [skill/SKILL.md](../../skill/SKILL.md) if you want to integrate an AI agent via the skill.
- See [troubleshooting.md](troubleshooting.md) if anything goes wrong.

