# Web UI Memory Browser Dashboard (`centmem ui`)

A fast, local-first browser dashboard for exploring, searching, and managing all memories stored in `centmem`.

Served directly from the single `centmem` binary via an embedded HTTP server—zero npm or Node.js runtime dependencies on user machines.

---

## 1. Quick Start

Launch the dashboard in your default browser:

```bash
centmem ui
```

```json
{"ok": true, "url": "http://127.0.0.1:4231", "host": "127.0.0.1", "port": 4231, "version": "2.0.3"}
```

The browser will open automatically to `http://127.0.0.1:4231`. To stop the server, press `Ctrl+C` in your terminal.

### CLI Flags & Environment Variables

| Flag | Env Variable | Default | Description |
|---|---|---|---|
| `--port <number>` | `CENTMEM_UI_PORT` | `4231` | Port to listen on |
| `--host <ip>` | — | `127.0.0.1` | Local IP to bind to (strictly localhost) |
| `--no-open` | `CENTMEM_UI_NO_OPEN` | `false` | Start server without auto-launching browser |

Examples:
```bash
# Start on custom port without opening browser
centmem ui --port 8080 --no-open

# Run in background via environment variables
CENTMEM_UI_PORT=5000 CENTMEM_UI_NO_OPEN=1 centmem ui
```

---

## 2. Design Principles

The centmem dashboard is crafted under two strict design directives:
- **impeccable (Operate Mode)**: High density, scannable tabular typography, calm and focused interface. Components ship with complete states (default, hover, focus, active, disabled, skeleton loading, and informative empty states).
- **antislop-ui**: Zero generic AI gradients or purple glows; restrained single-accent palette (calm teal `#0f766e` in light mode, `#14b8a6` in dark mode); shadows with real offsets; 100% real SQLite data (no filler or invented metrics); and full keyboard accessibility.

---

## 3. Interface Anatomy

```
┌───────────────────────────────────────────────────────────────────────────┐
│ [centmem]  | 🔍 Hybrid search (/ or ⌘K) | [Design] [?] [⚙ Settings] [●] [☼]│
├──────────────┬────────────────────────────────────────────────────────────┤
│              │ Breadcrumb: global > project:cent-mem > agent:antigravity  │
│  SCOPES      │ ┌────────────────────────────────────────────────────────┐ │
│  ──────────  │ │ Filters: [Type ▾] [Tags] [Date ▾] [Agent ▾]   [Reset]  │ │
│  ▸ global (42│ └────────────────────────────────────────────────────────┘ │
│  ▾ project:A │                                                            │
│    ▸ agent:1 │ Memory List (J/K to navigate, Enter to inspect)            │
│    ▸ agent:2 │ ┌──────┬───────────────────────────────┬──────┬──────────┐ │
│  ▸ project:B │ │ TYPE │ CONTENT PREVIEW               │ TAGS │ MODIFIED │ │
│              │ ├──────┼───────────────────────────────┼──────┼──────────┤ │
│  ──────────  │ │ note │ Architecture: SQLite + FTS5   │ arch │ 2m ago   │ │
│  + New Scope │ │ fact │ port=4231                     │ cfg  │ 1h ago   │ │
│              │ └──────┴───────────────────────────────┴──────┴──────────┘ │
│              │ Showing 1-20 of 84 memories                 [< Prev] [Next >]│
└──────────────┴────────────────────────────────────────────────────────────┘
```

---

## 4. Key Capabilities

### 4.1 Hierarchical Scope Navigation
The sidebar renders the full 4-tier memory hierarchy (`global -> project -> agent -> session`).
- Displays live memory counts per scope node.
- Expand or collapse scope branches.
- Click any scope to instantly filter the memory browser and update the deep-linkable URL.
- Responsive design collapses the sidebar into a slide-over drawer on mobile or narrow viewports.

### 4.2 Hybrid Recall Search & Advanced Filters
- **Global Search**: Type `/` or `Cmd+K` anywhere to focus the search input. Searches using hybrid Reciprocal Rank Fusion (RRF $k=60$), fusing SQLite FTS5 bm25 keyword scoring with local ONNX vector embeddings.
- **Filters Panel**: Filter by memory type (`note`, `fact`, `log`), tags, date ranges (`24h`, `7d`, `30d`, or custom timestamps), agent name, and session ID.
- **Teaching Empty States**: If a scope or query has no memories, the empty state displays the exact copy-pasteable `centmem put` CLI command with a one-click copy button.

### 4.3 Detail Drawer & Inspector
- Click any memory row or press `Enter` on a keyboard-selected row to open the side drawer.
- View raw content, formatted timestamps, scope path, tags, and internal IDs.
- Easily copy content to clipboard with one click.

### 4.4 Actions, Safety & Exports
- **Memory Deletion**: Click the trash icon or press `Del`/`Backspace` on a selected memory. An explicit confirmation dialog explains consequences before deletion.
- **Undo Toast**: Forgetting a memory triggers an 8-second floating toast with an **Undo** action that instantly restores the memory.
- **Scoped Exports**: Export memories in the active scope as JSON or CSV directly from the top bar dropdown.

### 4.5 System Health & Doctor Diagnostics
- The top bar displays a real-time status pill (`Healthy`, `Warning`, or `Degraded`).
- Click the pill to open the interactive **Doctor Diagnostics Popover**, displaying the results of all 7 operational subsystems:
  1. **Database Integrity**: `PRAGMA integrity_check` verification.
  2. **Schema Version**: Schema migration check (Schema v6).
  3. **Native Extensions**: `sqlite-vec` vector similarity and SQLite FTS5 full-text indexing.
  4. **Embedding Model**: SHA256 checksum and file validation for the local ONNX model (`bge-small-en-v1.5`).
  5. **Embed Queue**: Number of pending background embedding jobs.
  6. **Permissions**: File mode checks on database (`0600`) and storage root (`0700`).
  7. **AI Agent**: Live probe of the configured LLM backend endpoint and model latency.
- **Soft Warning Semantics**: If the AI Agent LLM is unreachable or offline, the doctor status displays a yellow warning icon (`AlertTriangle`) with latency and actionable advice without blocking core memory operations or failing the overall system.

### 4.6 Settings & System Configuration (`Cmd+,`)
Click the gear icon in the top bar or press `Cmd+,` (`Ctrl+,` on Linux/Windows) to open the **Settings & Configuration** modal. This two-pane dialog manages system configuration keys supported by `centmem` with instant validation and atomic persistence:

- **General Tab**: Inspects the pinned local ONNX embedding model (`bge-small-en-v1.5`, 384 dimensions), local storage directory (`CENTMEM_HOME`), database size, and file write permissions.
- **AI Agent Tab**: Dedicated configuration panel for the built-in AI Memory Agent:
  - **Provider Presets**: 1-click preset buttons for common providers (Ollama local default, OpenAI, Gemini, Anthropic).
  - **LLM Settings**: Interactive inputs for endpoint URL, model identifier, API key / environment variable name, timeout seconds, and max tokens.
  - **Live "Test Connection"**: Probes endpoint reachability via `POST /api/config/test-agent`, checks model availability, and displays round-trip latency in milliseconds.
  - **Reasoning Controls**: Sliders for ReAct reasoning step limits (`max_reasoning_steps`, 1–16), curation confidence threshold (`confidence_threshold`, 0.50–1.00), and auto-apply toggle for safe link proposals.
  - **Live Hot-Reloading**: Changes take effect immediately upon saving without restarting the `centmem ui` server process.
- **Retention Tab**: Visualizes the active retention lifecycle (`Raw Notes/Logs -> Summaries -> Archive -> Dropped`). Provides tactile number steppers to configure retention policies: `fact_keep_days` (set to 0 for indefinite retention), `note_summarize_after_days`, `log_summarize_after_days`, `log_drop_after_days`, and `archive_keep_days`.
- **Auto-Capture Tab**: Master switch for background and transcript auto-capture, target harness selector (`auto`, `claude-code`, `cursor`, `antigravity`, `trae`, `codex`, `generic`), triggers multi-select (`session-end`, `per-message`, `on-demand`), and default capture scope expression.
- **Classifier Tab**: Configures the 3-tier memory classification backend (`heuristic`, `local-llm`, or `openai-compatible`). Provides endpoint URL, model tag, API key environment variable, and confidence cutoff slider (`0.10` to `1.00`). Includes a live **"Test Connection"** probe button verifying classifier latency and health.
- **Categories Tab**: Interactive tag chip manager for whitelisted auto-capture categories with quick-add chips (`decision`, `convention`, `preference`, `learning`, `checkpoint`, `security`, `api`, `arch`).
- **Atomic Persistence & Dirty Tracking**: The footer tracks draft changes against `~/.centmem/config.toml` in real time with a dirty indicator dot. Includes "Revert Changes", "Reset to Defaults" (with confirmation), and prevents accidental dismissal via an unsaved changes confirmation dialog.

### 4.7 Hierarchical Scope Deletion & Project Pruning
Manage and prune memory subtrees directly within the browser interface:
- **UI Entry Points**:
  1. A prominent **"Delete Scope"** button located in the `ScopeOverview` card header.
  2. A quick-action **trash icon** on hover next to scope nodes in the `ScopeTree` sidebar.
- **Type-to-Confirm Modal (`DeleteScopeConfirmDialog`)**:
  - Requires explicitly typing the exact scope path (e.g. `project:my-app`) to unlock the red confirmation button, eliminating accidental deletions.
  - Shows real-time counts of all memories and descendant child scopes that will be pruned.
- **Root Scope Invariant**: The root `global` scope is strictly protected; deletion is disabled in the UI and rejected by the backend.
- **Complete Cascade Cleanup**: Prunes memories, sqlite-vec embeddings, queue jobs, memory links, agent proposals, and conversation threads in an atomic transaction.
- **Reactive Cache Invalidation**: Automatically resets view selection to `global`, invalidates SWR caches, and displays a success toast.

### 4.8 Proposals Review Center & Bulk Actions
Human-in-the-loop review for autonomous memory agent suggestions (`merge`, `link`, `update`, `archive`):
- **Visual Diffs**: Side-by-side comparison of source memories and proposed consolidated text, combined tags, and directional relationship arrows (`supports`, `contradicts`, `supersedes`).
- **Individual Actions**: 1-click **Approve & Apply**, **Dismiss**, and **Undo/Reopen** with live status badge updates.
- **Bulk Actions**: When viewing pending proposals, a dedicated review toolbar provides:
  - **"Approve All"** button: Opens `BatchProposalConfirmDialog` to apply all currently filtered pending proposals in one batch.
  - **"Reject All"** button: Opens confirmation dialog to dismiss all currently filtered pending proposals.
  - **Batch Confirmation Dialog**: Displays the total count, category badges by proposal type, and explicit warning callouts (e.g. merging consolidates and archives original memories).
  - **Transparent Chunking & Resilience**: Handles review sets up to 500 items with transparent client-side chunking and conflict handling.

### 4.9 Assistant Chat Tab (Active Intelligence)
- **Interactive Conversational Inquiry**: Chat directly with the built-in AI Memory Agent in natural language.
- **Grounded Provenance**: Outputs markdown responses with clickable `[#id]` citations linking directly to the memory detail drawer.
- **Multi-Step ReAct Loop**: Coordinates search, reading, and link inspection before answering complex multi-step inquiries.
- **Knowledge Gap Detection**: Surfaces detected project blind spots with 1-click shortcuts to store missing knowledge.
- **Offline Fallback Notice**: If the LLM backend is unconfigured or unreachable, displays a calm notice and provides hybrid search results with a direct shortcut to Agent Settings.

---

## 5. Keyboard Shortcuts

Press `?` anywhere in the dashboard to toggle the keyboard shortcuts overlay.

| Shortcut | Action |
|---|---|
| `/` or `Cmd/Ctrl + K` | Focus memory search input |
| `J` or `↓` | Select next memory row |
| `K` or `↑` | Select previous memory row |
| `Enter` | Open selected memory detail drawer |
| `Backspace` or `Del` | Forget selected memory (with confirmation) |
| `Cmd / Ctrl + ,` | Open Settings & Configuration dialog |
| `Esc` | Close drawer, modal dialog, or popover |
| `?` | Toggle keyboard shortcuts modal |

---

## 6. Embedded REST API Reference

The dashboard communicates with `centmem` via a local-only REST API:

| Endpoint | Method | Description |
|---|---|---|
| `/api/scopes` | `GET` | Returns full scope hierarchy tree with memory counts |
| `/api/scopes` | `POST` | Create a new scope (`{"path": "..."}`) |
| `/api/scopes` | `DELETE` | Cascade-delete a non-global scope subtree (`?path=<scope>`) |
| `/api/memories` | `GET` | List/filter memories (`scope`, `type`, `tags`, `agent`, `session`, `since`, `until`, `q`, `page`, `page_size`) |
| `/api/memories/:id` | `GET` | Retrieve full memory details by integer ID |
| `/api/memories/:id/forget` | `POST` | Delete memory by ID |
| `/api/memories` | `POST` | Restore or insert memory (used by Undo) |
| `/api/config` | `GET` | Retrieve active system configuration and file metadata (`home`, `config_path`, `is_writable`) |
| `/api/config` | `PATCH` | Update configuration with dot-notation validation and atomic write to `~/.centmem/config.toml` |
| `/api/config/test-agent` | `POST` | Live connectivity probe verifying AI Agent LLM endpoint health and round-trip latency |
| `/api/config/test-classifier` | `POST` | Live connectivity probe verifying classifier endpoint health and latency |
| `/api/proposals` | `GET` | List staged agent proposals (`scope`, `status`, `type`) |
| `/api/proposals/:id/apply` | `POST` | Apply an individual staged proposal |
| `/api/proposals/:id/dismiss` | `POST` | Dismiss an individual staged proposal |
| `/api/proposals/:id/reopen` | `POST` | Reopen a previously dismissed proposal |
| `/api/proposals/batch` | `POST` | Batch apply or dismiss multiple proposals (`action: "apply"\|"dismiss", ids: [...]`) |
| `/api/agent/chat` | `POST` | Send natural language inquiry to AI Memory Agent with ReAct reasoning and citations |
| `/api/agent/conversations` | `GET` | List persistent agent conversation threads |
| `/api/agent/conversations` | `POST` | Create a new agent conversation thread |
| `/api/agent/conversations/:id/messages` | `GET` | List chronological messages and tool execution turns for a conversation |
| `/api/export` | `GET` | Download scoped memories (`format=json` or `format=csv`) |
| `/api/stats` | `GET` | Retrieve database metrics (counts by type, size, pending embeddings) |
| `/api/health` | `GET` | Run comprehensive `centmem doctor` checks (including AI Agent probe) |
| `/ui/design-system` | `GET` | Interactive component gallery and token reference |
