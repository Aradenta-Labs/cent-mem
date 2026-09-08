# Auto-Capture Hooks Reference — centmem

This reference describes the auto-capture architecture introduced in cent-mem v1.3.0.

## Overview

Auto-capture monitors agent transcripts to extract durable knowledge without manual user prompting.

```
Agent Conversation
       │
  (Trigger: watcher | exit trap | on-demand)
       ▼
centmem capture run
       ▼
 1. Transcript Ingestion (JSONL / plain text)
 2. Three-Tier Classifier (Local LLM -> Heuristic -> BYOK Cloud)
 3. Novelty Judgment (Recall-before-write)
 4. Store Persistence (put / set)
 5. Session Telemetry (summary report)
```

---

## 1. Triggers

- **Per-message Watcher**: A continuous background file watcher (`centmem capture run --watch`) tracking new lines in the agent transcript.
- **Session-end Trap**: Shell exit trap in agent harness scripts triggering a one-shot extraction on exit.
- **On-demand**: Executed directly via `centmem capture run --transcript <file>`.

---

## 2. Three-Tier Classification Backend

The classifier attempts backends in order:
1. **Local LLM (`backend = "local-llm"`)**:
   - Calls local OpenAI-compatible endpoint (e.g. Ollama at `http://localhost:11434/v1`).
   - Private and 100% on-device.
2. **Deterministic Heuristic (`backend = "heuristic"`)**:
   - Zero-dependency regex pattern matcher.
   - Triggers on indicators like "we decided", "chose X over Y", version strings, and error resolutions.
   - Always succeeds as a fallback if other backends fail.
3. **Bring-Your-Own-Key (`backend = "openai-compatible"`)**:
   - User-provided endpoint (e.g. `https://api.openai.com/v1`).
   - Secret key is read from an environment variable name (e.g. `$OPENAI_API_KEY`), never written to disk.

---

## 3. Recall-Before-Write Deduplication

To prevent duplicate memories:
1. Before saving a candidate memory, `centmem` performs a high-threshold `recall` query.
2. If similar content exists in the project scope, the item is marked as skipped.
3. A session-level dedup lock file (`~/.centmem/session-<id>.dedup.json`) prevents reprocessing within the same session.

---

## 4. Category Whitelist

Default categories:
- `decision`: Architectural and design choices.
- `fact`: Concrete configuration values, ports, endpoints.
- `preference`: User coding style and workflow preferences.
- `code`: Reusable snippets, commands, and patterns.
- `log`: Session checkpoints and progress notes.
- `error`: Bugs, root causes, and resolutions.
- `dependency`: Newly introduced packages and external services.

Manage categories via:
```bash
centmem capture categories --list
centmem capture categories --add "custom-category"
centmem capture categories --remove "preference"
```

---

## 5. Developer Artifact Capture Pipelines (v1.5.1)

In v1.5.1, `centmem capture` extends beyond runtime conversational transcripts to ingest static and historical developer artifacts directly from local project repositories and toolchains.

### Pipelines

1. **Git Commit & Dependency Pipeline (`centmem capture git`)**:
   - Parses Git commits from the repository history.
   - Evaluates commit messages and diffs to distinguish architectural decisions from routine changes.
   - Detects package dependency additions and updates across `go.mod`, `package.json`, `Cargo.toml`, `requirements.txt`, and `pyproject.toml`, creating structured `dep.<package>` facts.
   - Resumes from the last scanned commit SHA using `.centmem/git-cursor`.

2. **Documentation Ingestion Pipeline (`centmem capture docs`)**:
   - Scans project documentation directories (Markdown, plain text, reStructuredText).
   - Chunks documents by Markdown headings and code blocks into self-contained semantic memories.
   - Preserves headings and relative file provenance in tags.
   - Tracks file modification timestamps in `.centmem/docs-cursor`.
   - **Tombstoning**: Automatically marks memories as archived when their corresponding documentation files are deleted.

3. **Developer Shell History Pipeline (`centmem capture shell`)**:
   - Scans shell history files (`~/.zsh_history`, `~/.bash_history`, `~/.local/share/fish/fish_history` or `$HISTFILE`).
   - Strips transient flags, normalizes recurring commands, and computes frequency distributions.
   - Persists top workflows as a structured `shell.frequent_commands` fact.
   - Tracks read offsets in `.centmem/shell-cursor`.

4. **Code Annotation Pipeline (`centmem capture comments`)**:
   - Scans source code (`.go`, `.ts`, `.js`, `.py`, `.rs`, `.sh`, etc.) for actionable developer annotations (`TODO`, `FIXME`, `HACK`, `NOTE`, `OPTIMIZE`, `SECURITY`, `DEPRECATED`).
   - Language-aware comment parsing respects line comments (`//`, `#`) and multi-line block comments (`/* ... */`).
   - Formats memories with explicit line provenance: `[file: <path>:<line>] [<KEYWORD>] <comment text>`.
   - **Line-shift tracking**: Detects when surrounding code changes move an existing comment to a different line number, updating the memory content without creating duplicate entries.

### Cursor Directory & Repository Isolation (`.centmem/`)

Incremental capture pipelines store their synchronization state inside a `.centmem/` directory located at the project root:
- `.centmem/git-cursor`: Plaintext commit SHA representing the latest ingested Git commit.
- `.centmem/docs-cursor`: JSON object mapping relative file paths to Unix modification timestamps.
- `.centmem/shell-cursor`: Line or byte offset within the shell history file.

**Automatic Isolation**:
When `centmem` creates or accesses `.centmem/`, it automatically creates `.centmem/.gitignore` containing `*`. This guarantees that cursor files, temporary buffers, and local capture state are never committed to repository version control. State files are updated using atomic temporary file writes (`rename`) to ensure consistency during interrupted operations.

### High-Entropy Secret Scrubber

All developer artifact capture pipelines run content through the automated secret scrubber (`internal/capture/scrubber.go`) before memories are persisted or evaluated:
- **Detected Patterns**:
  - Bearer tokens (`Bearer <token>`)
  - URLs with embedded credentials (`https://user:password@host`)
  - AWS access keys (`AKIA...`)
  - GitHub Personal Access Tokens and OAuth tokens (`ghp_...`, `github_pat_...`)
  - Slack API tokens (`xoxb-...`, `xoxp-...`)
  - PEM-encoded private keys (`-----BEGIN ... PRIVATE KEY-----`)
  - Generic high-entropy assignments matching `api_key`, `secret`, `token`, `password`, `auth_key`, etc.
- **Redaction Placeholders**: Matched secrets are replaced with deterministic placeholders (e.g. `[REDACTED]`, `[REDACTED_AWS_KEY]`, `[REDACTED_PRIVATE_KEY]`).
- **Safety Guarantee**: Unredacted secrets and credentials never enter SQLite storage, FTS5 full-text indices, or ONNX vector embeddings.

---

## External MCP Context Enrichment (v1.5.3)

In v1.5.3, the capture classification engine can query external Model Context Protocol (MCP) servers to retrieve additional context before categorizing candidate memories.

### Configuration (`config.toml`)

```toml
[capture.mcp]
enabled = false
servers = [
  { name = "docs-search", command = "npx", args = ["-y", "@modelcontextprotocol/server-everything"] }
]
tools = ["search_docs", "fetch_url"]
```

### Execution Flow & Safeguards
1. **Ambiguous Context Resolution**: When evaluating commits or transcript messages that mention external documents, packages, or URLs, the classifier queries configured MCP tools to fetch summary text.
2. **5-Second Hard Timeout**: External MCP tool invocations are bounded by a strict 5-second timeout (`context.WithTimeout(ctx, 5*time.Second)`).
3. **Graceful Fallback**: If an external server times out, fails, or exits with an error, the classification pipeline logs a debug warning and gracefully falls back to direct classification without blocking the capture workflow.


