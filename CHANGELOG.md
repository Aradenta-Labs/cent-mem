# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.0.0
