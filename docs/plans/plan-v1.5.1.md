# v1.5.1 — Richer Capture: Git, Docs, Shell, Comments

**Version:** 1.5.1 (target)  
**Owner:** Aradenta Labs  
**Status:** Approved — Ready for implementation  
**Depends on:** v1.5.0 shipped; builds on v1.3 transcript capture and 3-tier classifier infrastructure  
**Roadmap Reference:** [docs/plans/roadmap-v1.5.x.md](roadmap-v1.5.x.md)

---

## 0. Executive Summary & Objective

In **v1.3**, centmem introduced auto-capture from live AI agent transcripts (`centmem capture run --transcript ...`), using a 3-tier classifier (local LLM $\to$ deterministic regex heuristics $\to$ cloud fallback) to extract notes, decisions, and facts from multi-turn dialogues.

However, a software project's most valuable institutional knowledge lives outside agent chat logs: in Git commit histories, architecture markdown files, developer shell patterns, and inline code annotations. Today, bringing this context into centmem requires tedious, manual `centmem put` scripts.

**v1.5.1 (Richer Capture)** transforms centmem into a self-populating developer memory system by introducing four dedicated, incremental capture pipelines:
1. **`centmem capture git`**: Incrementally extracts decisions, breaking changes, and architectural rationale from Git commits using cursor tracking in `.centmem/git-cursor`.
2. **`centmem capture docs`**: Ingests markdown, reStructuredText, and plain-text documentation with semantic heading-based chunking and mtime cursor invalidation.
3. **`centmem capture shell`**: Aggregates developer CLI patterns from Zsh, Bash, and Fish history files with strict secret redaction to store verified toolchain conventions as facts.
4. **`centmem capture comments`**: Scans source code repositories for actionable annotations (`TODO`, `FIXME`, `HACK`, `SECURITY`) and links them to exact file locations.

**Core Tenets:**
- **Incremental & Idempotent:** Cursors ensure repeat runs are fast no-ops ($< 20$ ms when nothing changed).
- **Quality-Filtered:** All unstructured text flows through the existing 3-tier classifier and `recall-before-write` deduplication.
- **Privacy & Safety:** High-entropy secret scrubbing by default; zero leakage of tokens, passwords, or keys.
- **Zero Configuration Necessary:** Sensible defaults infer current Git repo, repo docs, and user shell.

---

## 1. Problem Statement & Architecture Opportunities

| # | Knowledge Source | Current Limitation | v1.5.1 Solution |
|---|---|---|---|
| 1 | Git History | Rich commit explanations ("*refactor: switch SQLite WAL mode to fix lock contention*") are lost after merge. | `centmem capture git` traverses `cursor..HEAD`, classifies commit messages, and extracts structured decisions. |
| 2 | Documentation | Architecture documents (`docs/architecture.md`, `README.md`) must be manually memorized or manually injected into prompt context. | `centmem capture docs` indexes docs by Markdown heading chunks into searchable memories, automatically updating on edit. |
| 3 | Shell Toolchains | Custom build commands, test flags, and environment aliases must be re-discovered by every new agent session. | `centmem capture shell` parses history, redacts secrets, and saves top tool patterns as `fact` memories. |
| 4 | Inline Code Annotations | `// SECURITY:` or `// HACK:` comments left in code are invisible to agents unless the agent happens to view that specific file. | `centmem capture comments` inventories codebase annotations into searchable memory notes with file/line provenance. |

---

## 2. Technical Architecture & Shared Infrastructure

```
                      ┌────────────────────────────────────────┐
                      │            Developer Inputs            │
                      │  (Git commits, Docs, Shell, Comments)  │
                      └───────────────────┬────────────────────┘
                                          │
                                          ▼
                      ┌────────────────────────────────────────┐
                      │       Shared Cursor Engine (.centmem/) │
                      │   - git-cursor (SHA)                   │
                      │   - docs-cursor (mtime JSON map)       │
                      │   - shell-cursor (offset/timestamp)    │
                      └───────────────────┬────────────────────┘
                                          │
                                          ▼
                      ┌────────────────────────────────────────┐
                      │    Secret Scrubber & Sanitizer         │
                      │   - Strips API keys, passwords, bearer │
                      └───────────────────┬────────────────────┘
                                          │
                                          ▼
                      ┌────────────────────────────────────────┐
                      │       3-Tier Classifier Engine         │
                      │  Tier 1: Local Ollama / Embedded LLM   │
                      │  Tier 2: Fast Deterministic Heuristics │
                      │  Tier 3: Cloud Provider (optional)     │
                      └───────────────────┬────────────────────┘
                                          │
                                          ▼
                      ┌────────────────────────────────────────┐
                      │     Recall-Before-Write Deduplicator   │
                      │   - Cosine threshold < 0.20 rejection  │
                      │   - Content hash collapse              │
                      └───────────────────┬────────────────────┘
                                          │
                                          ▼
                      ┌────────────────────────────────────────┐
                      │             Store / SQLite             │
                      │  type=note / type=fact / type=log      │
                      └────────────────────────────────────────┘
```

### Shared Cursor Directory (`.centmem/`)
All capture commands store their state inside a `.centmem/` directory located at the repository root:
- `.centmem/.gitignore` is created automatically containing `*`, ensuring cursor state is never committed to Git.
- Cursors are stored in deterministic, human-readable formats (plaintext SHA, JSON maps).

---

## 3. Subcommand Specifications

### 3.1 `centmem capture git`

#### Command Syntax
```bash
centmem capture git [--repo <path>] [--since <sha|date>] [--scope <scope>] [--dry-run] [--max-commits <n>]
```

#### Flags:
- `--repo`: Target Git repository directory (default: current working directory or Git root).
- `--since`: Commit SHA or relative duration (e.g. `30d`, `2026-01-01`) to initialize cursor if not present.
- `--scope`: Scope path to assign captured memories (default: `project:<repo-name>`).
- `--dry-run`: Evaluate and print memories that would be generated without writing to SQLite.
- `--max-commits`: Cap batch processing size per run (default: `100`).

#### Cursor Logic:
1. Path: `.centmem/git-cursor`.
2. If cursor exists: query commits via `git log <cursor>..HEAD --reverse --format="%H%x1f%an%x1f%at%x1f%s%x1f%b%x1e"`.
3. If cursor does not exist: query either `--since` or default to the last 50 commits (`git log -n 50 --reverse ...`).
4. On successful completion: write latest processed commit SHA to `.centmem/git-cursor`.

#### Classification Rules:
- **`decision` (type=`note`, tag `decision`):**
  - Commit subject starts with conventional commit verbs: `refactor:`, `break:`, `switch:`, `deprecate:`, `arch:`, `perf:`.
  - Body contains intent indicators: `"decided to"`, `"chosen because"`, `"switched from"`, `"in order to"`, `"tradeoff"`.
- **`fact` (type=`fact`):**
  - Dependency changes: `go.mod`, `package.json`, `Cargo.toml`, `requirements.txt` diffs extracted.
  - Stored as key: `deps.<package-name>`, value: version.
- **`log` (type=`log`):**
  - Bug fixes, chore, documentation updates not meeting decision threshold.

---

### 3.2 `centmem capture docs`

#### Command Syntax
```bash
centmem capture docs [--dir <path>] [--scope <scope>] [--ext md,txt,rst] [--dry-run]
```

#### Chunking Algorithm:
1. **Target Identification:** Walks `--dir` (default: current directory, prioritizing `docs/`, `README.md`, `CHANGELOG.md`, `ARCHITECTURE.md`).
2. **Exclusions:** Automatically ignores `.git`, `node_modules`, `vendor`, `.centmem`, `dist`, `build`.
3. **Cursor:** `.centmem/docs-cursor` maps `{ "docs/architecture.md": 1788740000 }`.
4. **Boundary Splitting:**
   - Splits file content at Markdown headers: `# `, `## `, `### `.
   - Preserves breadcrumb hierarchy (e.g., `Architecture > Database > WAL Mode`).
   - If a section exceeds 800 tokens, splits at paragraph boundaries (`\n\n`).
5. **Memory Attributes:**
   - `type`: `note`
   - `tags`: `["docs", "<filename>", "<top-heading>"]`
   - `content`: `"[docs/architecture.md # Storage Engine]\n\n..."`
   - `source_agent`: `"docs-capture"`
6. **Tombstoning:** If a previously indexed file in `.centmem/docs-cursor` no longer exists on disk, query memories with `source_agent = "docs-capture"` and `tags LIKE '%<filename>%'` and mark them `status = 'archived'`.

---

### 3.3 `centmem capture shell`

#### Command Syntax
```bash
centmem capture shell [--history <path>] [--shell <zsh|bash|fish>] [--scope <scope>] [--top <n>] [--dry-run]
```

#### History Formats Supported:
- **Zsh:** `~/.zsh_history` (both standard and extended `: <timestamp>:0;<command>`).
- **Bash:** `~/.bash_history` (with `#<timestamp>` line handling).
- **Fish:** `~/.local/share/fish/fish_history` or `~/.fish/fish_history` (`- cmd: ... \n   when: ...`).

#### Secret Scrubber Engine:
Before analyzing commands, run through high-entropy regex sanitization:
- API tokens: `Bearer [A-Za-z0-9_\-\.]{20,}`
- Keys: `(?i)(api[_-]?key|secret|token|password|auth)\s*[:=]\s*[^\s]+`
- SSH/Git creds: `https?://[^:]+:[^@]+@`
- AWS/Cloud creds: `AKIA[0-9A-Z]{16}`
Commands matching known secret patterns are redacted to `[REDACTED]` or omitted entirely.

#### Pattern Extraction & Storage:
- Normalizes commands: strips directory paths, sudo, env prefixes (e.g. `FOO=1 bar` $\to$ `bar`).
- Groups commands by root tool and subcommand: `git push`, `go test`, `docker compose`, `npm run`.
- Computes frequency distribution.
- Stores top $N$ (default 15) patterns as a fact memory:
  - `key`: `shell.frequent_commands`
  - `value`: `{"go test -tags fts5 ./...": 84, "git status": 65, "graphify query": 42}`
  - `tags`: `["shell", "toolchain", "conventions"]`

---

### 3.4 `centmem capture comments`

#### Command Syntax
```bash
centmem capture comments [--dir <path>] [--ext go,ts,js,py,rs,sh] [--scope <scope>] [--dry-run]
```

#### Extraction Rules:
- Supports single-line comments (`//`, `#`) and multi-line doc comments (`/* ... */`, `""" ... """`).
- Keywords recognized: `TODO`, `FIXME`, `HACK`, `NOTE`, `OPTIMIZE`, `SECURITY`, `DEPRECATED`.
- Format extracted:
  ```
  [file: internal/store/store.go:142] [SECURITY] Always validate file permissions before chmod
  ```
- **Deduplication:** Hash constructed from `file_path + ":" + line_content`. If a comment moves lines but retains identical content, updates existing memory rather than duplicating.

---

## 4. CLI Output Contracts

### JSON Output (`--dry-run` or standard)

```json
{
  "ok": true,
  "command": "capture git",
  "source": ".git",
  "commits_scanned": 24,
  "memories_created": 3,
  "memories_updated": 0,
  "cursor": "a8f3bc1994d8721c0e352b9921",
  "items": [
    {
      "id": 149,
      "type": "note",
      "tags": ["git", "commit", "decision", "sqlite"],
      "content": "Commit a8f3bc1: Switched SQLite to WAL journal mode and busy_timeout=5000 to resolve concurrent write contention across agent processes.",
      "dry_run": false
    }
  ]
}
```

---

## 5. Internal Package Architecture

```
internal/capture/
├── cursor.go          # File-based cursor manager (.centmem/*-cursor)
├── scrubber.go        # High-entropy secret and token sanitizer
├── git.go             # Git commit log parser and diff analyzer
├── docs.go            # Markdown/RST AST walker and heading chunker
├── shell.go           # Zsh, Bash, and Fish history parser and frequency counter
└── comments.go        # Code comment regex scanner with language support
```

### Key Structs (`internal/capture/git.go`):
```go
type GitCaptureConfig struct {
    RepoPath   string
    Since      string
    Scope      string
    DryRun     bool
    MaxCommits int
}

type GitCommit struct {
    SHA       string
    Author    string
    Timestamp time.Time
    Subject   string
    Body      string
}

func CaptureGit(ctx context.Context, st *store.Store, cfg GitCaptureConfig) (*CaptureResult, error)
```

### Key Structs (`internal/capture/docs.go`):
```go
type DocChunk struct {
    FilePath    string
    HeadingPath []string
    Content     string
    Tokens      int
    ModTime     time.Time
}

func CaptureDocs(ctx context.Context, st *store.Store, cfg DocsCaptureConfig) (*CaptureResult, error)
```

---

## 6. Testing & Verification Plan

### Test Suites:
1. **`internal/capture/git_test.go`:**
   - Create synthetic git repo using `exec.Command("git", "init", ...)`.
   - Commit series of formatted commits (conventional commits, chore, breaking change).
   - Test cursor advancement: second run processes 0 commits.
2. **`internal/capture/docs_test.go`:**
   - Chunk markdown documents with varying heading depths (`#`, `##`, `###`).
   - Test mtime caching: unedited files skipped; edited files updated; deleted files archived.
3. **`internal/capture/shell_test.go`:**
   - Parse sample `.zsh_history`, `.bash_history`, and fish history files.
   - Verify secrets (e.g. `export AWS_SECRET_ACCESS_KEY=...`) are cleanly redacted.
4. **`internal/capture/comments_test.go`:**
   - Scan test fixtures containing Go, Python, and TypeScript comments.
   - Verify line numbers, tags, and content extraction.
5. **E2E Integration Test (`cmd/centmem/handlers_capture_test.go`):**
   - Execute CLI subcommands with `--dry-run` and standard mode; verify JSON shapes against golden files.

---

## 7. Files Affected & Implementation Checklist

- [ ] **Shared Capture Logic:** `internal/capture/cursor.go` & `scrubber.go` [NEW]
- [ ] **Git Capture:** `internal/capture/git.go` & `git_test.go` [NEW]
- [ ] **Docs Capture:** `internal/capture/docs.go` & `docs_test.go` [NEW]
- [ ] **Shell Capture:** `internal/capture/shell.go` & `shell_test.go` [NEW]
- [ ] **Comment Capture:** `internal/capture/comments.go` & `comments_test.go` [NEW]
- [ ] **CLI Dispatch:** `cmd/centmem/handlers_capture.go` (wire `git`, `docs`, `shell`, `comments`) [MODIFY]
- [ ] **CLI Documentation:** `docs/cli-contract.md` & `skill/SKILL.md` [MODIFY]
- [ ] **Integration Tests:** `cmd/centmem/e2e_capture_rich_test.go` [NEW]
- [ ] **Knowledge Graph:** Update with `graphify update .` after completion
