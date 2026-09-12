# AI Memory Agent Guide (v2.0.0)

`centmem` v2.0 introduces the **Active Intelligence Layer**: a built-in, local-first AI Memory Agent that transforms static memory retrieval into autonomous inquiry, proactive contradiction detection, semantic duplicate consolidation, and structured architectural synthesis.

---

## 1. Overview & Architecture

Prior to v2.0, `centmem` operated as a fast, passive memory repository: agents stored notes and facts via `centmem put` and retrieved them via hybrid search (`centmem recall`).

With v2.0, `centmem` adds an autonomous **Active Intelligence Engine** (`internal/agent`) powered by a multi-step **ReAct reasoning loop** (Reason + Act + Observe). The agent interacts directly with SQLite and hybrid vector search using a typed tool registry:

```
                      ┌─────────────────────────────────┐
                      │    AI Memory Agent Engine       │
                      │   (ReAct Loop: Plan-Act-Think)  │
                      └────────────────┬────────────────┘
                                       │
            ┌──────────────────────────┼──────────────────────────┐
            ▼                          ▼                          ▼
   ┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐
   │ search_memories │        │  inspect_links  │        │  propose_merge  │
   │  read_memory    │        │  propose_link   │        │detect_knowl_gaps│
   └────────┬────────┘        └────────┬────────┘        └────────┬────────┘
            │                          │                          │
            └──────────────────────────┼──────────────────────────┘
                                       ▼
                      ┌─────────────────────────────────┐
                      │   Human-in-the-Loop Staging     │
                      │        (agent_proposals)        │
                      └────────────────┬────────────────┘
                                       │ (1-click apply / dismiss)
                                       ▼
                      ┌─────────────────────────────────┐
                      │ SQLite + Vector Store + Graph   │
                      │  (memories, memory_links, etc.) │
                      └─────────────────────────────────┘
```

### Core Architectural Principles
1. **Zero External Orchestration Dependencies**: Runs entirely in the single `centmem` Go binary. Does not require Python, LangChain, or heavyweight agent frameworks.
2. **Deterministic Human-in-the-Loop (HITL)**: Curation operations do not silently overwrite or delete memories. They generate structured proposals in the `agent_proposals` table for user or harness approval.
3. **Pluggable BYOK / Local LLMs**: Works out of the box with Ollama, LocalAI, vLLM, or any cloud OpenAI-compatible completion endpoint.
4. **Graceful Offline Degradation**: When no LLM backend is configured or the endpoint is unreachable, all commands fall back to heuristic deduplication, deterministic categorization, and hybrid search recall with zero errors.

---

## 2. Configuration (`[llm]` & `[agent]`)

The agent engine is configured in `~/.centmem/config.toml` (or custom `--home`) and supports runtime environment variable overrides.

### Configuration Schema

```toml
[llm]
backend = "ollama"                   # "ollama", "openai_compatible", or "disabled"
endpoint = "http://127.0.0.1:11434"  # Base API URL
model = "llama3.2:latest"            # Model identifier
api_key = ""                         # Optional API token (or use api_key_env)
timeout_seconds = 60                 # Per-request HTTP timeout
max_tokens = 2048                    # Generation token budget
temperature = 0.2                    # Sampling temperature (lower = more deterministic)

[agent]
enabled = true                       # Enable built-in AI Memory Agent
max_reasoning_steps = 8              # Safety cycle guard on ReAct loop
confidence_threshold = 0.85          # Minimum confidence for autonomous actions
auto_apply_proposals = false         # Require explicit review before applying
inquiry_top_citations = 5            # Default citations cap for inquiries
```

### Environment Variable Overrides

| Environment Variable | Config Key | Description |
|---|---|---|
| `CENTMEM_LLM_BACKEND` | `llm.backend` | Backend provider (`ollama`, `openai_compatible`, `disabled`) |
| `CENTMEM_LLM_ENDPOINT` | `llm.endpoint` | HTTP API base URL |
| `CENTMEM_LLM_MODEL` | `llm.model` | Model name |
| `CENTMEM_LLM_API_KEY` | `llm.api_key` | Raw bearer token or API key |
| `CENTMEM_AGENT_MAX_STEPS`| `agent.max_reasoning_steps` | Maximum tool-calling iterations |
| `CENTMEM_AGENT_AUTO_APPLY`| `agent.auto_apply_proposals`| Auto-apply proposals (`true` / `false`) |

### Inspecting and Updating Configuration via CLI

```bash
# View LLM configuration
centmem config get llm

# Switch backend to Ollama
centmem config set llm.backend ollama
centmem config set llm.endpoint http://127.0.0.1:11434
centmem config set llm.model llama3.2

# Configure OpenAI-compatible endpoint
centmem config set llm.backend openai_compatible
centmem config set llm.endpoint https://api.openai.com
centmem config set llm.model gpt-4o-mini
centmem config set llm.api_key "$OPENAI_API_KEY"
```

---

## 3. CLI Command Reference

### 3.1 `centmem ask` — Conversational Q&A & Inquiry

Performs multi-step conversational question answering synthesized from stored memories with grounded citations provenance and gap detection.

```bash
centmem ask "<question>" [--scope <scope>] [--top N] [--interactive]
```

#### Parameters:
- `<question>`: Natural language inquiry.
- `--scope <scope>`: Restrict memory recall to a hierarchical scope path (e.g. `project:cent-mem`).
- `--top <N>`: Maximum number of citation sources to inspect (1–20, default `5`).
- `--interactive`: Launch an interactive multi-turn terminal session.

#### Example Output (JSON):
```json
{
  "ok": true,
  "answer": "The project uses SQLite in WAL mode with sqlite-vec for dense vector embeddings [id: 12450]. Write locks are serialized via the centmemd daemon [id: 12610].",
  "citations": [
    {
      "id": 12450,
      "type": "note",
      "scope": "project:cent-mem",
      "snippet": "Architecture uses SQLite with SQLite-vec for local dense vector search",
      "score": 0.892
    },
    {
      "id": 12610,
      "type": "note",
      "scope": "project:cent-mem",
      "snippet": "centmemd daemon uses server.LockWrite() mutex serialization for multi-agent writes",
      "score": 0.814
    }
  ],
  "knowledge_gaps": [],
  "reasoning_steps": 2,
  "fallback_used": false,
  "conversation_id": "0191ebc4-912f-7890-a3e1-3829ad0ef581"
}
```

#### Interactive Terminal Mode:
```bash
centmem ask --interactive --scope project:cent-mem
```
Inside the interactive session:
- `/help` — Display session commands.
- `/clear` — Clear thread history and start a new conversation context.
- `/q`, `exit`, `quit` — Exit the inquiry session.

---

### 3.2 `centmem curate` — Autonomous Memory Curation

Scans memories within a scope to detect contradictions, identify outdated conventions, and consolidate redundant or duplicate memories into staged proposals.

```bash
centmem curate [--scope <scope>] [--type contradictions|dedup|all] [--apply] [--dry-run]
```

#### Parameters:
- `--scope <scope>`: Scope path to curate (default `global`).
- `--type <type>`: Curation strategy (`contradictions`, `dedup`, or `all`). Default: `all`.
- `--apply`: Automatically apply proposals that exceed `confidence_threshold`.
- `--dry-run`: Simulate curation without persisting proposals or modifying memories.

#### Example Output (JSON):
```json
{
  "ok": true,
  "proposals_created": [42, 43],
  "proposals_applied": [],
  "scanned_memories": 158,
  "contradictions_found": 1,
  "duplicates_found": 1,
  "fallback_used": false
}
```

---

### 3.3 `centmem summarize` — Scope Synthesis & Briefings

Synthesizes high-level project briefings, conventions, and architectural pillars into a cohesive Markdown document.

```bash
centmem summarize [--scope <scope>] [--focus <topic>] [--format markdown|json] [--save]
```

#### Parameters:
- `--scope <scope>`: Scope path to summarize.
- `--focus <topic>`: Optional topic focus (e.g. `database`, `deployment`, `conventions`).
- `--format <format>`: Output format (`markdown` or `json`). Default: `markdown`.
- `--save`: Persists the generated briefing as a permanent memory (`type: note`, tags: `summary,architecture`).

---

### 3.4 `centmem proposals` — Staged Action Management

Full management surface for human-in-the-loop review of staged agent proposals.

```bash
# List staged proposals
centmem proposals list [--scope <scope>] [--status pending|applied|dismissed]

# Inspect proposal diff and reasoning
centmem proposals show <id>

# Apply a proposal atomically
centmem proposals apply <id>

# Dismiss a proposal
centmem proposals dismiss <id>
```

#### Proposal Types:
1. **`merge`**: Consolidates multiple redundant memories into a single target memory. Source memories are archived and updated with `supersedes` links.
2. **`link`**: Creates a directional semantic relationship (`supports`, `contradicts`, `supersedes`) between two memories.
3. **`update`**: Updates content or tags of an existing memory.
4. **`archive`**: Safely archives obsolete memories while preserving audit logs in the events table.

---

## 4. Web UI Experience

`centmem ui` provides a visual interface for both interactive conversation and proposal review.

```bash
centmem ui
```

### Assistant Tab
- **High-Density Streaming Chat**: Real-time markdown rendering with syntax-highlighted code snippets and copy shortcuts.
- **Clickable Citations**: Grounded `[#id]` badges link directly to the memory drawer for instant context verification.
- **Knowledge Gap Indicators**: Highlights detected blind spots with a 1-click "Record Memory for this topic" shortcut.
- **Offline Setup Notice**: Renders a calm banner when running in offline retrieval mode with a direct 1-click shortcut to the Settings tab.

### Proposals Review Center
- **Visual Merge Diffs**: Displays source memories side-by-side with proposed consolidated text and combined tags.
- **Relationship Previews**: Visual graph arrows (`[from] ── supersedes ──► [to]`).
- **1-Click Actions**: Instant **Approve & Apply**, **Dismiss**, and **Undo/Reopen** actions with live counter badges.
- **Bulk Review Actions**: Dedicated toolbar with **Approve All** and **Reject All** buttons, opening a confirmation modal (`BatchProposalConfirmDialog`) with type breakdowns, consequence warnings, and transparent client-side chunking for large review queues.

### Settings & Diagnostics
- **Agent Settings Tab**: Manage provider presets (Ollama, OpenAI, Gemini, Anthropic), model names, credentials, and ReAct loop parameters with instant hot-reloading.
- **Live "Test Connection"**: Fast, zero-token probe button verifying endpoint reachability and displaying round-trip latency in milliseconds.
- **Doctor Diagnostics**: `centmem doctor` and the Web UI Doctor Popover verify the `"ai_agent"` subsystem, reporting active model and latency with soft warning semantics.

---

## 5. Offline Degradation & Zero-Configuration Mode

When no LLM is configured (`backend = "disabled"` or unreachable endpoint):
1. **`centmem doctor`**: Reports `"ai_agent"` with `status: "warn"` and diagnostic advice without failing with exit code 1.
2. **`centmem ask`**: Automatically runs hybrid search recall and outputs ranked memories with citations, setting `"fallback_used": true` and including a clear guidance notice.
3. **`centmem curate`**: Falls back to content-hash and normalized exact-text grouping to identify duplicates and create merge proposals without requiring AI inference.
4. **`centmem summarize`**: Outputs a deterministic catalog digest grouped by memory type (`fact`, `note`, `log`).
5. **Web UI**: Renders an informative banner with an interactive button navigating straight to Settings to configure your backend.
