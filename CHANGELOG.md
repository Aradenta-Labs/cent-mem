# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0] - 2026-09-02

Auto-capture from agent transcripts: automatic extraction and storage of durable knowledge (decisions, facts, preferences, code, logs, errors, dependencies) directly from AI agent conversations with zero mandatory effort after setup.

### Added

- **Auto-capture engine** (`internal/capture`): Core extraction pipeline including transcript ingestion (`reader.go`), multi-backend classification (`classifier.go`), recall-before-write deduplication (`dedup.go`), store persistence (`writer.go`), session telemetry (`summary.go`), lifecycle locking (`session.go`), real-time file watcher (`watcher.go`), and multi-harness transcript normalization (`convert.go`).
- **Three-tier classification engine**: Resilient fallback chain supporting Local LLM (Ollama/vLLM/LocalAI at `localhost:11434`), lightweight deterministic heuristic pattern matching (zero dependencies), and Bring-Your-Own-Key OpenAI-compatible cloud endpoints with one-time retry on malformed JSON responses and confidence threshold gating (`capture.confidence_threshold`).
- **Hook adapters & recipes** (`skill/adapters/hooks/`): Per-harness hook specifications, shell exit traps, and file watcher configurations for Google Antigravity, Trae, Claude Code, Cursor, OpenAI Codex, DeepSeek, and Hermes.
- **Unified hook installer** (`skill/adapters/hooks/install.sh`): Multi-platform shell installer supporting harness auto-detection, `--all`, `--harness`, `--scope`, `--trigger`, `--list`, `--dry-run`, `--uninstall`, and macOS `launchd` plist / Linux `systemd` user service unit generation.
- **Real-time incremental watcher** (`centmem capture run --watch`): Low-overhead continuous file watcher with persistent byte-offset state tracking across restarts and configurable debouncing.
- **CLI commands & subcommands**:
  - `centmem capture run [--transcript <path>] [--scope <s>] [--watch]` — execute on-demand or continuous capture.
  - `centmem capture summary [--session <id>]` — inspect captured items, telemetry, and skip reasons as structured JSON.
  - `centmem capture categories [--list] [--add <c>] [--remove <c>]` — manage category whitelist.
  - `centmem capture convert --harness <name> --input <path>` — normalize foreign transcripts into standard `.jsonl`.
  - `centmem config set capture.<key> <value>` and `centmem config get capture.<key>` — manage `config.toml` settings.
- **Custom prompt templates**: User-editable prompt at `~/.centmem/capture-prompt.md` with runtime interpolation of `{{CATEGORIES}}`, `{{CONFIDENCE_THRESHOLD}}`, and `{{RECALL_CONTEXT}}`.
- **Recall-before-write deduplication**: Queries store recall context prior to classification to prevent duplicate memory insertion across sessions.
- **Comprehensive documentation**: New [Capture Hooks Guide](docs/guides/capture-hooks.md), updated [Getting Started Guide](docs/guides/getting-started.md), and refreshed runtime recipes in [SKILL.md](skill/SKILL.md).

### Changed

- **Interactive `centmem init` wizard**: Extended initialization flow to interactively configure capture harness, triggers, classifier backends, endpoints, and default scopes.
- **Configuration schema** (`internal/config`): Added `[capture]` configuration block and full suite of `CENTMEM_CAPTURE_*` environment variable overrides.

### Security

- **100% local-first default**: Transcript processing and classification run on-device with zero network calls.
- **Zero API keys on disk**: `config.toml` stores only the name of the environment variable holding cloud credentials (e.g. `OPENAI_API_KEY`), never the plaintext secret.

## [1.2.0] - 2026-08-31

Frictionless UX release: `npx @aradenta.labs/centmem-skills` standalone installer, automated project workflow loop injection, and smart routing `/centmem` slash command integration across major AI agent harnesses.

### Added

- **NPM installer package** (`@aradenta.labs/centmem-skills`): Zero-dependency Node installer executable via `npx @aradenta.labs/centmem-skills` (or `npx @aradenta.labs/centmem-skills add <url>`), supporting cross-platform target discovery across macOS, Linux, and Windows.
- **Automated workflow injection**: Automatically detects project instruction files (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`) and injects the **Read → Do → Update** memory loop (using `<!-- centmem:start -->` tags for idempotency). Automatically creates `AGENTS.md` at the project root if no instruction files exist.
- **Smart routing `/centmem` slash command**: Built-in slash command integrations and templates for Google Antigravity, Trae, Claude Code, Cursor, OpenAI Codex, DeepSeek, and Hermes. Automatically routes between recall, put/set, and maintenance operations.
- **Node.js test suite**: Automated testing with `node:test` covering arguments parsing, idempotent block injections, and end-to-end multi-target installations.

## [1.1.0] - 2026-08-31

### Changed

- Relocated the project to the `github.com/aradenta-labs/cent-mem` module and
  org. All internal imports and every install/docs URL now point to the new
  path. No functional or CLI-contract changes.

[1.1.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.1.0

## [1.0.0] - 2026-08-31

First stable release. Single static binary + installable skill that gives any
AI agent (Claude Code, Codex, Cursor, custom harnesses) a shared, local-first,
hierarchical memory store.

### Added

- **CLI** (`centmem`): `init`, `put`, `set`, `get`, `recall`, `timeline`,
  `list`, `forget`, `stats`, `compact`, `doctor`, `backup`, `restore`.
- **Hybrid search** (`recall`): semantic (local ONNX embedding, BGE-small 384d)
  + keyword (SQLite FTS5 bm25) + facts + timeline, fused via Reciprocal Rank
  Fusion. Retrieves memories by paraphrase with no keyword overlap.
- **Hierarchical scoping**: `global → project → agent → session`, with
  inheritance and `--children`.
- **Async embeddings**: write-time `embed_queue` + inline drain worker, so
  writes return in < 50 ms p95.
- **Retention & compaction** (`compact`): auto-summarizes old notes/logs into
  consolidated notes (heuristic summarizer, pluggable LLM hook) and archives
  originals.
- **Ops tooling**: `doctor` (integrity, schema version, extensions, model,
  embed queue, permissions), `backup` (`VACUUM INTO`) and `restore` (with a
  `.pre-restore.bak` safety copy).
- **Skill package** (`skill/`): canonical `SKILL.md`, idempotent `install.sh`
  (with `--dry-run`/`--list`/`--uninstall`), and adapter docs for Claude Code,
  Codex, Cursor, Amazon Q, and custom harnesses.
- **Stable CLI contract**: deterministic JSON output on stdout, errors to
  stderr as `{"error":{...}}`, and exit codes `0/1/2/3`. Enforced by golden
  files + a contract-consistency test (docs == code).
- **Distribution**: `scripts/build.sh` and a GitHub Actions release workflow
  that cross-compiles `darwin/arm64`, `darwin/amd64`, `linux/amd64`,
  `linux/arm64`, generates `SHA256SUMS`, and attaches them to a GitHub Release.

### Changed

- Corrected the pinned SHA256 checksum for the default embedding model
  (`bge-small-en-v1.5`) to match the file served by Hugging Face. The previous
  checksum caused `centmem init` to fail verification on fresh installs.

### Security

- Fully local-first; no telemetry. The only network call is the one-time model
  download during `init` (over HTTPS, checksum-pinned).
- `~/.centmem` permissions enforced (`0700` dir, `0600` DB), verified by
  `doctor`.

[1.3.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.3.0
[1.2.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.2.0
[1.1.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.1.0
[1.0.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.0.0
