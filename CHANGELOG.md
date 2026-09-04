# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.3] - 2026-09-04

Recall Accuracy Enhancements: Phase A & B. Major improvements to search accuracy, query expansion, and duplicate detection without breaking the CLI contract.

### Added

- **Query Expansion via Tag Enrichment**: Search queries are now automatically augmented with top tags from the closest keyword matches for broader semantic recall.
- **Recency Decay**: Introduced an exponential penalty factor for older memories. Configurable via `search.decay_half_life_days` (default off).
- **FTS5 Prefix Fallback**: Search automatically retries with prefix matching if exact keyword matches return 0 rows.
- **Weighted RRF Fusion**: Lowered timeline influence for text queries to prevent recency bias from overshadowing semantic relevance.

### Fixed

- **Mathematical Conversion of Distance**: Correctly configured semantic search to convert native SQLite L2 Euclidean distances to Cosine distances prior to applying the semantic distance threshold.
- **Near-Duplicate Collapse**: After score fusion, duplicate memories (by `content_hash`) are collapsed into a single best result to prevent result-window crowding.

## [1.4.2] - 2026-09-04

Patch release resolving a React `SyntheticEvent` circular structure serialization error when testing model connections in the Web UI Settings panel.

### Fixed

- **Web UI Live Connection Probe Error**:
  - Resolved `Converting circular structure to JSON` TypeError triggered when clicking "Test Connection" in the Settings > Classifier tab.
  - Sanitized `testClassifier` and `testClassifierEndpoint` parameters to filter out synthetic mouse events and extract only known, strongly-typed configuration keys before serialization.
  - Decoupled `Button` `onClick` handlers across `ClassifierTab`, `SettingsModal`, and `GeneralTab` to prevent leaking event arguments to async action dispatchers.

## [1.4.1] - 2026-09-03

Web UI Configuration & Settings Panel: A dedicated, high-density settings dialog in `centmem ui` allowing users to configure retention rules, auto-capture parameters, classifier backends, and category whitelists with live connectivity probing and atomic persistence directly to `~/.centmem/config.toml`.

### Added

- **Settings & Configuration Modal (`Cmd+,`)**:
  - Accessible two-pane dialog opened via `Cmd+,` (`Ctrl+,`) or the top-bar settings gear button.
  - Full support for all 19 configuration keys across Model, Retention, Auto-Capture, Classifier, and Categories.
  - Live dirty-state tracking with deep diffing, "Revert Changes", "Reset to Defaults" confirmation, and unsaved changes exit dialog.
- **REST Configuration API Endpoints**:
  - `GET /api/config`: Reads active configuration with home directory and file metadata.
  - `PATCH /api/config`: Updates settings with dot-notation field validation and atomic write to `~/.centmem/config.toml`.
  - `POST /api/config/test-classifier`: 5-second connectivity probe verifying model health, responsiveness, and latency for Local LLM and OpenAI-compatible backends.
- **Modular Settings Tab Panels**:
  - **General Tab**: Read-only display of embedding model (`bge-small-en-v1.5`, 384 dims), local directory permissions, and database sizing.
  - **Retention Tab**: Number steppers for facts, note/log summarization, and log pruning, alongside a visual `RetentionLifecycle` progression diagram.
  - **Auto-Capture Tab**: Master toggle, agent harness selector, triggers multi-select, and default scope validator.
  - **Classifier Tab**: 3-tier backend selector (`heuristic`, `local-llm`, `openai-compatible`), endpoint URL/model/API key inputs, confidence threshold slider, and live "Test Connection" status badge.
  - **Categories Tab**: Interactive tag chip input with presets for managing capture whitelists.
- **Accessible Design System Primitives (`ui/src/components/`)**:
  - `Switch`, `Stepper`, `Slider`, `SegmentedControl`, and `TagInput` built under Impeccable Operate-mode rules with zero generic AI slop and 100% keyboard accessibility.
  - Interactive live demos added to `/ui/design-system`.

## [1.4.0] - 2026-09-03

Web UI Memory Browser Dashboard (`centmem ui`): A fast, local-first browser interface served directly from the `centmem` single binary via an embedded HTTP server and REST API, enabling visual browsing, hybrid recall search, and safe management of AI agent memories.

### Added

- **Embedded Web UI Server & CLI command** (`centmem ui`):
  - Subcommand `centmem ui [--port <port>] [--host <host>] [--no-open]` launching an embedded HTTP server serving compiled React/Vite/TypeScript assets via `go:embed`.
  - Automatic browser launching on startup (configurable via `--no-open` or `CENTMEM_UI_NO_OPEN=1`), and configurable listening port via `--port` or `CENTMEM_UI_PORT`.
  - Local-first JSON REST API bound strictly to `127.0.0.1` exposing `/api/scopes`, `/api/memories`, `/api/memories/:id`, `/api/stats`, `/api/health`, `/api/memories/:id/forget`, and `/api/export`.
- **Impeccable & Antislop-UI Design System** (`ui/`):
  - Crafted for **Operate mode**: high-density, calm, scannable layout with tabular data views and zero generic AI slop (no generic blue-purple gradients, no glassmorphism stacking, no zero-offset halo glows).
  - Restrained color strategy with calm teal primary accent (`#0f766e` in light mode, `#14b8a6` in dark mode) meeting WCAG AA contrast standards.
  - Fully persistent light/dark theme toggle (`localStorage` + `prefers-color-scheme` initial detection).
  - Complete component state coverage (default, hover, focus, active, disabled, skeleton loading, and informative teaching empty states with copy-pasteable CLI commands).
- **Hierarchical Scope Tree Navigation**:
  - Expandable/collapsible tree sidebar showing live memory counts across the 4-tier hierarchy (`global -> project -> agent -> session`).
  - Deep-linking URL synchronization (`?scope=...&q=...`) with breadcrumb navigation.
  - Responsive layout reflowing the sidebar into a mobile-friendly slide-over drawer on narrow viewports.
- **Memory Browser & Hybrid Search**:
  - High-density tabular memory list with sorting, pagination, and instant detail inspection drawer.
  - Global hybrid search input (`/` or `Cmd+K`) leveraging Reciprocal Rank Fusion (FTS5 bm25 + ONNX vector embeddings).
  - Advanced filters panel: filtering by memory type (`note`, `fact`, `log`), tags, date ranges (`24h`, `7d`, `30d`, custom), agent name, and session ID.
- **Safety & Actions**:
  - Memory deletion modal (`ForgetConfirmDialog`) clearly articulating consequences before removal.
  - Floating 8-second undo toast allowing instantaneous memory restoration.
  - Scoped data export supporting both JSON and CSV downloads.
- **Diagnostics & Health**:
  - Live Doctor Status popover in the top bar reporting store connectivity, schema version, sqlite-vec extensions, ONNX model presence, and file permissions.
  - High-density system metrics panel (`OverviewPanel`) reporting totals by memory type and recent write activity.
  - Comprehensive keyboard navigation (`/`, `J`, `K`, `Enter`, `Del`, `Esc`, `?` shortcut modal).
- **Comprehensive Documentation**:
  - New [Web UI Guide](docs/ui.md) with complete architecture details, CLI options, REST API reference, and keyboard shortcuts table.
  - Updated CI and release workflows to build frontend assets from source and package with binary releases.

## [1.3.1] - 2026-09-03

Skill installer enhancements: standard `.agents/skills` directory support, comprehensive bundled skill artifacts, guaranteed `AGENTS.md` memory loop injection, and script permission preservation.

### Added

- **Standard `.agents/skills` target support**: `@aradenta.labs/centmem-skills` now automatically installs the skill into `.agents/skills/centmem` (at project level) and `~/.agents/skills/centmem` (at user home level), matching modern AI agent skill conventions across Google Antigravity, Claude Code, Trae, Cursor, and Codex.
- **Detailed bundled skill artifacts**:
  - `references/cli-commands.md`: Exhaustive reference of every `centmem` CLI command with all flags, options, exit codes, and JSON response shapes.
  - `references/architecture-and-scoping.md`: Deep dive on 4-tier scoping (`global -> project -> agent -> session`), inheritance semantics, and RRF $k=60$ hybrid search (SQLite FTS5 + vector embeddings).
  - `references/capture-hooks.md`: Technical documentation on auto-capture file watchers, exit traps, and 3-tier classification backends.
  - `examples/agent-workflow-examples.md`: Practical end-to-end conversation walkthroughs of an agent reading, executing, and updating shared memory.
  - `examples/recipes.sh`: Ready-to-use copy-pasteable bash recipes for common memory queries and maintenance.
  - `scripts/centmem-helper.sh`: Executable diagnostic utility to verify binary presence, detect active project scope, and run health checks.
- **Automated release notes workflow**: Enhanced `.github/workflows/release.yml` to automatically extract the relevant section from `CHANGELOG.md` and publish it directly to the GitHub Release body.

### Changed

- **Guaranteed `AGENTS.md` injection**: In addition to detecting existing instruction files (`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`), the installer now always updates (or creates) `AGENTS.md` in the project root with the 3-step loop (`READ -> DO -> UPDATE`), essential commands cheatsheet, and the `/centmem` smart routing command.
- **Executable permissions preservation**: The skill installer recursively sets `0755` executable permissions on all bundled shell scripts (`.sh`, `.js`, `.mjs`) during deployment.
- **Tracked npm bin files**: Fixed `.gitignore` root `/bin/` pathing so `npm/bin/install.js` is properly version-controlled and packaged.

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

[1.4.3]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.3
[1.4.2]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.2
[1.4.1]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.1
[1.4.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.0
[1.3.1]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.3.1
[1.3.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.3.0
[1.2.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.2.0
[1.1.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.1.0
[1.0.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.0.0
