# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.2] - 2026-09-11

Web UI Agent Settings, Live Engine Hot-Reloading & Assistant Widget Offline State Synchronization.

### Added

- **Web UI AI Agent Configuration Panel (`ui/src/components/settings/AgentTab.tsx`)**:
  - Dedicated **AI Agent** tab in the Settings modal to configure LLM provider settings (`llm.*`) and memory agent reasoning loop behaviour (`agent.*`).
  - Quick-pick preset buttons for common providers: Ollama (local default), OpenAI, Gemini, and Anthropic.
  - Interactive inputs with visibility toggling for endpoint URL, model identifier, API key / environment variable name, timeout, and max tokens.
  - Interactive sliders for ReAct reasoning step limits (`max_reasoning_steps`, 1–16), curation confidence threshold (`confidence_threshold`, 0.50–1.00), and auto-apply safe link proposals toggle.
  - Real-time dirty-diff detection and save status indicators across all agent and LLM settings.

### Fixed

- **Live Agent Engine Hot-Reloading (`internal/ui/server.go`)**:
  - Re-instantiates `s.agent = agent.NewEngine(candidateConfig, s.cfg.Store, s.cfg.Searcher)` immediately upon saving settings via `PATCH /api/config`.
  - Ensures newly configured LLM endpoints, credentials, and reasoning options take effect instantly without restarting the `centmem ui` server process.
  - Returns `llm` and `agent` sections in the `PATCH /api/config` JSON response to keep client-side draft states synchronized.
- **Dynamic Assistant Widget Offline State (`ui/src/components/AssistantTab.tsx`)**:
  - Dispatches and listens for `centmem:config-updated` window events so the Assistant chat tab automatically refreshes its backend status upon saving settings.
  - Clears `isOfflineBackend` to `false` when subsequent assistant queries complete successfully (`fallback_used: false`).
  - Refines `showOfflineBanner` condition to only activate when the LLM backend is actually offline or unconfigured, preventing historical offline conversation turns from triggering persistent warning banners in active threads.
- **Agent ReAct Error Diagnostics (`internal/agent/engine.go`)**:
  - Logs actual ReAct loop failure reasons to `stderr` when falling back to offline retrieval mode, enabling immediate root-cause identification in UI server logs and CLI outputs.
- **Proxy SSE Stream Decoding in Non-Streaming Requests (`internal/agent/client.go`)**:
  - Automatically detects when reverse proxies or gateways return Server-Sent Events (`data: ...` / `text/event-stream`) even when `stream: false` was requested.
  - Assembles streaming chunks and reconstructs fragmented `tool_calls` arrays, eliminating `invalid character 'd' looking for beginning of value` errors during large curation and reasoning runs.

## [2.0.0] - 2026-09-11

Built-in AI Memory Agent: Inquiry, Autonomous Curation, Synthesis, Web UI Assistant & Hardening.

### Added

- **Active Intelligence Layer & Core Agent Engine (`internal/agent`)**:
  - Multi-step ReAct reasoning loop (Reason + Act + Observe) with cycle guards (`max_reasoning_steps`, default 8), citation extraction `[id: 42]`, real-time token streaming, and deterministic offline fallbacks.
  - 6 memory tools exposed as OpenAI-compatible function definitions: `search_memories`, `read_memory`, `inspect_links`, `propose_link`, `propose_merge`, `detect_knowledge_gaps`.
  - Unified, zero-dependency streaming HTTP client for OpenAI-compatible `/v1/chat/completions` (Ollama, LocalAI, vLLM, and cloud BYOK providers) supporting function calling, SSE chunks, and exponential backoff retry.
- **Schema Migration v6 (`m0006_agent_proposals.sql`)**:
  - `agent_proposals`: Human-in-the-loop staging queue for merges, links, updates, and archives with status tracking (`pending`, `applied`, `dismissed`).
  - `agent_conversations`: Persistent threads for Web UI and CLI interactive inquiry sessions with cascade constraints.
  - `agent_messages`: Chronological dialog turns supporting roles (`user`, `assistant`, `system`, `tool`), citations JSON, and tool execution logs.
- **Store Proposals Engine & Atomic Execution (`internal/store/proposals.go`)**:
  - Full CRUD lifecycle methods: `CreateProposal`, `GetProposal`, `ListProposals`, `UpdateProposalStatus`, `DismissProposal`.
  - Atomic `ApplyProposal` transaction runner executing confirmed graph links, consolidated memory merges with `supersedes` links, content updates, and memory archiving, emitting typed events and managing vector indexing.
- **CLI Command Suite (`cmd/centmem`)**:
  - `centmem ask`: Conversational Q&A with grounded citation provenance, knowledge gap detection, and interactive terminal mode (`--interactive`) with `/help` and `/clear` commands.
  - `centmem curate`: Autonomous memory curation detecting conflicts and semantic redundancies to stage link/merge proposals (`--type contradictions|dedup|all`, `--apply`, `--dry-run`).
  - `centmem summarize`: High-level architectural briefings and convention guides with `--save` and format options.
  - `centmem proposals`: Comprehensive management commands (`list`, `show`, `apply`, `dismiss`) for staged actions.
- **Web UI Experience (`ui/src`)**:
  - **Assistant Tab**: Streaming markdown chat with syntax-highlighted code blocks, clickable citation badges linking to the memory drawer, knowledge gap record alerts, and calm offline retrieval mode notice with 1-click Settings shortcut.
  - **Proposals Review Center**: Side-by-side visual merge diffs, directional relationship preview cards, and 1-click **Approve & Apply**, **Dismiss**, and **Undo/Reopen** actions with live counter badges.
  - REST/SSE endpoints in `internal/ui`: `POST /api/agent/chat`, `GET /api/agent/conversations`, `GET /api/agent/conversations/:id/messages`, `GET /api/proposals`, `POST /api/proposals/:id/apply`, `POST /api/proposals/:id/dismiss`, `POST /api/proposals/:id/reopen`.
- **Extended Configuration & Settings**:
  - Configuration tables `[llm]` and `[agent]` in TOML with environment variable overrides (`CENTMEM_LLM_*`, `CENTMEM_AGENT_*`) and live settings modification in Web UI.
- **Documentation & Guides**:
  - Dedicated AI Memory Agent Guide (`docs/guides/agent-guide.md`) detailing ReAct architecture, configuration, CLI commands, and offline fallbacks.

### Hardened

- **Offline & Graceful Degradation**:
  - Zero-configuration and unreachable endpoint fallback: `ask` outputs hybrid search results with clear guidance notice; `curate` executes heuristic hash/exact deduplication staging merge proposals; Web UI renders calm alert banner.
- **Daemon Concurrency & Lock-Free Multi-Agent Writes**:
  - Stress-tested under 30+ concurrent IPC write workers alongside simultaneous curation and proposal application transactions with zero SQLite lock errors (`_txlock=immediate`).
  - Atomic proposal application conflict resolution returning `ErrProposalConflict` on concurrent application races.

## [1.5.4] - 2026-09-09

Sync & `centmemd` Daemon: Multi-Process Architecture, High-Performance Local IPC, and Real-time Event Streaming.

### Added

- **Standalone Daemon Service (`cmd/centmemd`)**:
  - Dedicated long-running `centmemd` binary holding exclusive SQLite connection handles, vector embedding queue draining, and background compaction loops.
  - High-performance local IPC over Unix Domain Sockets (`~/.centmem/centmemd.sock`) or Windows Named Pipes (`\\.\pipe\centmemd`) with sub-1.5ms latency.
  - Optional TCP gRPC server (`--port <port>`, e.g. `:50051`) for remote multi-machine access.
- **Transparent CLI Delegation**:
  - `centmem` CLI automatically probes for a running daemon. When active, operations (`put`, `set`, `get`, `recall`, `timeline`, `list`, `forget`, `stats`, `compact`, `link`, `unlink`, `links`) transparently route over gRPC.
  - Instant zero-configuration fallback to embedded SQLite access when the daemon is offline.
  - `--direct` CLI flag and `CENTMEM_DIRECT=1` environment variable to bypass daemon IPC when direct database access is explicitly desired.
- **Single-Writer Serialization & Concurrency**:
  - `server.LockWrite()` mutex serialization in the daemon eliminates SQLite WAL `busy_timeout` errors and lock contention during high-concurrency multi-agent parallel operations.
  - Atomic scope creation (`INSERT INTO scopes(path) VALUES(?) ON CONFLICT(path) DO NOTHING`) preventing concurrent scope registration race conditions.
- **Daemon Lifecycle Management**:
  - Commands: `centmemd start` (background daemon spawn), `centmemd run` (foreground supervisor mode for Docker/systemd/launchd), `centmemd status` (live health and statistics probe), `centmemd stop` (graceful shutdown with PID/socket cleanup).
- **gRPC Sync & Replication Service**:
  - `SyncService` streaming events from the `events` table over gRPC for real-time synchronization across instances.

### Fixed

- Preserved socket and PID file path scoping when a custom `--home` directory flag is provided.

## [1.5.3] - 2026-09-08

Integrations: Model Context Protocol (MCP) Server & Client, Remote Web UI REST with Token Auth, and VS Code Extension.

### Added

- **Model Context Protocol (MCP) Server (`centmem serve --mcp`)**:
  - Zero-dependency, stdio-based JSON-RPC 2.0 MCP server natively exposing centmem memory tools to Claude Code, Cursor, Windsurf, Zed, and Google Antigravity.
  - Supported MCP tools: `centmem_recall`, `centmem_put`, `centmem_set`, `centmem_get`, `centmem_timeline`, `centmem_stats`, `centmem_forget`, `centmem_link`.
- **MCP Client Classifier Enrichment**:
  - Auto-capture classifier queries external MCP servers before categorization to retrieve supplemental project context.
- **Remote REST API with Token Authentication**:
  - `centmem ui` supports binding to `0.0.0.0` securely guarded by mandatory `Authorization: Bearer <token>` validation via `--token` flag or `CENTMEM_UI_TOKEN`.
  - Frontend `TokenAuthModal` in Web UI with password visibility toggle and authentication persistence.
- **Official VS Code Extension (`editors/vscode/centmem-memory/`)**:
  - Sidebar panel providing real-time selection context recall and one-click memory capture from the editor.

### Fixed

- Direct API key resolution in OpenAI-compatible classifier probe and UI input component sanitization.

## [1.5.2] - 2026-09-08

Memory Relationships: Link Graph, Relationship Management, and Graph-Aware Recall.

### Added

- **Relational Graph Schema (Migration v5)**:
  - Database schema migration `m0005_memory_links.sql` introducing `memory_links` table with directional typed relationships (`supports`, `refines`, `contradicts`, `depends-on`, `supersedes`).
- **Relationship Management CLI**:
  - `centmem link <from_id> <to_id> --relation <rel>` to establish verified memory relationships.
  - `centmem link confirm <link_id>` and `centmem link dismiss <link_id>` to confirm or reject auto-suggested relationships.
  - `centmem unlink` to remove links by link ID or source/target pair.
  - `centmem links <id> [--all]` to inspect incoming and outgoing graph edges.
- **Auto-Suggestion Engine**:
  - Analyzes semantic similarity on `centmem put` to automatically suggest relationship edges (`suggested=true`).
- **Graph-Aware Recall**:
  - `centmem recall --include-links` expands search results with 1-hop relationship edges.
  - Web UI relationship badge and drawer link navigation.

### Fixed

- Resolved flag parsing, error mapping, and recall edge cases for memory link operations.

## [1.5.1] - 2026-09-08

Richer Capture: Incremental Developer Artifact Ingestion Pipelines.

### Added

- **`centmem capture git`**:
  - Incremental Git commit and PR description ingestion tracking commit SHA cursor (`.centmem/git-cursor`).
- **`centmem capture docs`**:
  - Markdown and documentation file indexing with heading breadcrumb chunking and mtime tracking (`.centmem/docs-cursor`).
- **`centmem capture shell`**:
  - Shell command history pattern extraction (`.zsh_history`, `.bash_history`) with secret scrubbing.
- **`centmem capture comments`**:
  - Codebase source code annotation scanner indexing `TODO`, `FIXME`, `HACK`, `NOTE`, `OPTIMIZE`, and `SECURITY` tags with file and line provenance.

## [1.5.0] - 2026-09-07

Search & Recall: Importance Scoring & Access Frequency Tracking.

### Added

- **Access Frequency Tracking (Migration v4)**:
  - Database schema migration `m0004_importance.sql` adding `access_count` and `last_accessed_at` columns to `memories` table.
  - Asynchronous, non-blocking recall access tracking via `RecordAccessAsync` updating access counts for recalled memories.
- **Logarithmic Importance Score Boost**:
  - Importance multiplier applied during RRF scoring: `multiplier = 1.0 + ln(1 + access_count) * weight`, capped at configurable threshold (default 2.0x).
  - Configuration keys: `search.importance_boost_enabled`, `search.importance_weight`, `search.importance_cap`.
- **Importance Telemetry**:
  - Distribution metrics in `centmem stats` output (`zero_access`, `low_access_1_5`, `medium_access_6_20`, `high_access_21_plus`, `max_access_count`, `avg_access_count`).

## [1.4.5] - 2026-09-05

Smart Skills Installer: Re-engineered `@aradenta.labs/centmem-skills` (`npm/bin/install.js`) with filesystem config directory harness detection, Git-aware scope inference, and interactive disambiguation using Node.js built-in `readline`.

### Added

- **Harness Detection (`npm/bin/harnesses.js`)**:
  - Declarative `HARNESSES` registry mapping supported AI agent harnesses (Antigravity, Claude Code, Cursor, OpenAI Codex, Generic `.agents/skills`, Trae, Hermes) to filesystem config directory detection signals and installation target paths.
  - `detectHarnesses(home, cwd)` to dynamically discover installed harnesses on the user's system rather than scattering files blindly.
- **Scope Inference**:
  - Automatic detection of project vs. global scope: defaults to `project` scope when inside a Git repository (walk-up `.git` discovery) and `global` scope when outside.
  - `--global` and `--project` CLI flags to explicitly override scope inference from any directory.
- **Interactive Disambiguation**:
  - Interactive CLI prompting via Node.js built-in `readline` when multiple harnesses are detected or when no supported harnesses are found.
  - `--yes`, `-y` flags to bypass interactive prompts and accept all detected defaults.
  - CI / non-TTY safety with silent default fallback and zero third-party dependencies.
- **Enhanced CLI Output & List Mode**:
  - `list` command and installer output display resolved scope, detected harnesses, and formatted destination paths.

## [1.4.4] - 2026-09-04

Recall Accuracy Enhancements: Phase C. Comprehensive Stage 2 re-ranking pipeline, scope proximity and agent affinity boosting, tag-weighted structured embeddings, background re-indexing command, and synthetic evaluation suite.

### Added

- **Two-Stage Re-Ranking Pipeline (`CompositeReRanker`)**:
  - Implemented Stage 2 scoring over a candidate window combining semantic score ($S_{\text{sem}}$), lexical token coverage ($S_{\text{lex}}$), exact phrase match bonus ($S_{\text{phrase}}$), and normalized Stage 1 RRF rank ($S_{\text{rrf\_norm}}$).
  - Configurable re-ranking backend via `search.reranker` (`none`, `composite`, `cross-encoder`, `llm`) and `--reranker` CLI flag on `centmem recall`.
  - Configurable candidate window via `search.rerank_window` (default 30).
- **Session & Agent Proximity / Affinity Boosting**:
  - Scope proximity multiplier boosts current session memories by +25% (`search.session_boost`), agent memories by +10% (`search.agent_boost`), preserves project score at 1.0x, and softly penalizes global memories (-5%).
  - Caller agent affinity boost adds +15% when memory `source_agent` matches `--caller-agent` or the `$CENTMEM_AGENT` environment variable.
- **Tag-Weighted Embeddings (v2)**:
  - Formatted embedding canonical text with structured `Key: ...`, `Tags: ...`, and `Content: ...` headers, prioritizing explicit metadata during semantic vector generation.
  - Migration `m0003_embedding_v2.sql` bumps `schema_version` to 3, records `embedding_version = 2`, and enqueues existing active memories for background re-indexing.
- **Background Re-Indexing Command (`centmem reindex`)**:
  - `centmem reindex [--all] [--batch N] [--max-time D] [--dry-run]` to process pending embedding migrations with rate-limiting, batching, and crash recovery.
- **Recall Evaluation Suite**:
  - Benchmark evaluation suite in `internal/search/eval_test.go` verifying accuracy metrics against synthetic test memories (`MRR@5 >= 0.85`, `NDCG@5 >= 0.80`, `Session Precision@1 >= 0.90`).

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

[2.0.2]: https://github.com/aradenta-labs/cent-mem/releases/tag/v2.0.2
[2.0.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v2.0.0
[1.5.4]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.5.4
[1.5.3]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.5.3
[1.5.2]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.5.2
[1.5.1]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.5.1
[1.5.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.5.0
[1.4.5]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.5
[1.4.4]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.4
[1.4.3]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.3
[1.4.2]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.2
[1.4.1]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.1
[1.4.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.4.0
[1.3.1]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.3.1
[1.3.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.3.0
[1.2.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.2.0
[1.1.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.1.0
[1.0.0]: https://github.com/aradenta-labs/cent-mem/releases/tag/v1.0.0
