# v2.0.0 — Built-in AI Memory Agent: Inquiry, Autonomous Curation & Web UI Assistant

**Version:** 2.0.0 (target)  
**Owner:** Aradenta Labs  
**Status:** Design locked via `/grill-me` — Ready for phased implementation  
**Depends on:** v1.5.0–v1.5.4 (Schema v5, Link Graph, `centmemd` gRPC Daemon)  
**Session Reference:** Derived from `/grill-me` architectural review on 2026-09-09

---

## 0. Executive Summary & Objective

Throughout the **v1.x series**, centmem established a high-performance, local-first shared memory store for AI agents:
- **v1.0–v1.3**: Hybrid search (RRF $k=60$), vector embeddings (ONNX BGE-small 384d), and auto-capture transcript watchers.
- **v1.4**: Embedded Web UI Memory Browser (`centmem ui`) and two-stage re-ranking.
- **v1.5**: Access-frequency importance scoring, developer artifact capture (`git`, `docs`, `shell`, `comments`), memory link graph (`memory_links`), native Model Context Protocol (MCP) server, and background `centmemd` daemon.

Despite these storage and retrieval milestones, **centmem remains a passive database**: it stores and retrieves records, but cannot synthesize knowledge, reason across disparate memory clusters, detect historical contradictions, or proactively clean and reorganize accumulated context.

**v2.0.0 (Built-in AI Memory Agent)** elevates centmem from a passive store into an **active intelligence layer**:
1. **Interactive Cognitive Inquiry (`centmem ask`)**: An on-demand conversational Q&A capability that answers natural language questions by synthesizing multiple memories, providing clickable citations with line/file provenance, and flagging knowledge gaps.
2. **Autonomous Memory Curation (`centmem curate`)**: A proactive curation engine running on a ReAct tool-calling loop that identifies semantic duplicates, suggests consolidation merges, and discovers evolution/contradictions to propose typed relationship links (`supersedes`, `contradicts`).
3. **Architectural & Onboarding Synthesis (`centmem summarize`)**: Generates comprehensive, scope-aware developer guides, project conventions, and architecture briefings from stored memories.
4. **Human-in-the-Loop Staged Proposals (`centmem proposals`)**: All curation operations stage reversible, structured proposals into SQLite (`agent_proposals` table) for explicit user review and 1-click confirmation via CLI or Web UI, with an `--auto-apply` flag for headless automation.
5. **Web UI Assistant & Proposals Inbox**: An interactive streaming Chat tab in `centmem ui` with interactive citation cards and a dedicated Proposals review center.

### Roadmap Staging
- **v2.0.0 (This Milestone)**:
  1. Interactive Q&A (`centmem ask`) with citations and gap detection.
  2. Contradiction & evolution link detection (`centmem curate --type contradictions`).
  3. Intelligent deduplication & merge proposals (`centmem curate --type dedup`).
  4. Scope briefings and architecture digests (`centmem summarize`).
  5. Proposal management queue (`centmem proposals`) and Web UI Chat & Inbox.
- **v2.0.1 (Next)**: Taxonomy & Tag Clustering (normalizing fragmented tags like `db` vs `database` and clustering by topic).
- **v2.0.2 (Next)**: Memory Health & Quality Scoring (detecting stale, vague, or low-information memories for pruning).

---

## 1. Architectural Design

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                 User & Agent Interfaces                                │
│   CLI: ask / curate / summarize    │   Web UI: Chat & Proposals   │   centmemd Cron    │
└───────────────────────────────────────────┬────────────────────────────────────────────┘
                                            │
                                            ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                           internal/agent — Memory Agent Engine                         │
│                                                                                        │
│  ┌────────────────────────┐      ┌────────────────────────┐      ┌──────────────────┐  │
│  │   Unified LLM Client   │◄────►│      ReAct Loop        │◄────►│   Prompt Catalog │  │
│  │ (Ollama / OpenAI-cloud)│      │  (Plan ─► Act ─► Think)│      │  (Ask/Curate/Sum)│  │
│  └────────────────────────┘      └───────────┬────────────┘      └──────────────────┘  │
│                                              │ Tool Execution                          │
│                   ┌──────────────────────────┴──────────────────────────┐              │
│                   ▼                          ▼                          ▼              │
│          ┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐     │
│          │ search_memories │        │   read_memory   │        │  inspect_links  │     │
│          └─────────────────┘        └─────────────────┘        └─────────────────┘     │
│                   ▼                          ▼                          ▼              │
│          ┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐     │
│          │  propose_link   │        │  propose_merge  │        │ summarize_scope │     │
│          └─────────────────┘        └─────────────────┘        └─────────────────┘     │
└───────────────────────────────────────────┬────────────────────────────────────────────┘
                                            │
                                            ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                              Persistence & Storage Layer                               │
│                                                                                        │
│  ┌───────────────────────┐   ┌────────────────────────┐   ┌─────────────────────────┐  │
│  │       memories        │   │      memory_links      │   │     agent_proposals     │  │
│  │ (notes, facts, logs)  │   │  (graph relationship)  │   │ (pending/applied merges)│  │
│  └───────────────────────┘   └────────────────────────┘   └─────────────────────────┘  │
│  ┌───────────────────────┐   ┌────────────────────────┐   ┌─────────────────────────┐  │
│  │  agent_conversations  │   │     agent_messages     │   │     events (sync)       │  │
│  │   (chat thread state) │   │     (dialog turns)     │   │   (gRPC replication)    │  │
│  └───────────────────────┘   └────────────────────────┘   └─────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Database Schema Specification (Migration v6)

File: `internal/store/migrations/m0006_agent_proposals.sql`

```sql
-- Migration 0006: Agent Proposals & Conversation Threads

-- 1. Agent Proposals Queue (Human-in-the-loop staging for merges, links, and updates)
CREATE TABLE IF NOT EXISTS agent_proposals (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    scope_id      INTEGER NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    proposal_type TEXT NOT NULL CHECK(proposal_type IN ('link', 'merge', 'update', 'archive')),
    status        TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'applied', 'dismissed')),
    title         TEXT NOT NULL,
    reasoning     TEXT NOT NULL,
    payload_json  TEXT NOT NULL, -- Structured details (e.g. source_ids, target_id, relation, consolidated_content)
    created_at    INTEGER NOT NULL, -- unix microseconds
    applied_at    INTEGER           -- unix microseconds (nullable)
);

CREATE INDEX IF NOT EXISTS idx_proposals_scope_status 
    ON agent_proposals(scope_id, status, created_at DESC);

-- 2. Agent Conversations (Web UI and CLI chat thread persistence)
CREATE TABLE IF NOT EXISTS agent_conversations (
    id            TEXT PRIMARY KEY, -- uuid or nanoid
    scope_id      INTEGER NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    title         TEXT NOT NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversations_scope 
    ON agent_conversations(scope_id, updated_at DESC);

-- 3. Agent Conversation Messages
CREATE TABLE IF NOT EXISTS agent_messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'system', 'tool')),
    content         TEXT NOT NULL,
    citations_json  TEXT, -- JSON array of cited memory IDs: [{"id": 42, "title": "...", "score": 0.035}]
    tool_calls_json TEXT, -- JSON array of tool execution records if role='assistant' or role='tool'
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation 
    ON agent_messages(conversation_id, created_at ASC);
```

---

## 3. Configuration Specification (`config.toml`)

Unified LLM configuration section supporting both local zero-configuration endpoints (Ollama/LocalAI/vLLM) and OpenAI-compatible cloud providers:

```toml
[llm]
# Provider backend: "ollama", "openai_compatible", or "disabled"
backend = "ollama"
endpoint = "http://127.0.0.1:11434/v1"
model = "deepseek-r1:8b"
api_key = ""                  # Optional raw key or ENV variable name (e.g. "OPENAI_API_KEY")
timeout_seconds = 60
max_tokens = 4096
temperature = 0.2

[agent]
enabled = true
max_reasoning_steps = 8       # Maximum iterations for ReAct tool loop
confidence_threshold = 0.75   # Minimum confidence for autonomous proposals
auto_apply_safe_links = false # When true, high-confidence (>=0.90) links bypass proposal queue
```

*Backward Compatibility Note:* If `[llm]` is not specified, centmem automatically inherits existing values from `[capture]` (`capture.classifier_backend`, `capture.endpoint`, `capture.model`, `capture.api_key`).

---

## 4. Phased Implementation Plan

```
Stage 1: Core Engine & DB ──► Stage 2: CLI Suite ──► Stage 3: Web UI Experience ──► Stage 4: Hardening & Docs
```

---

### Stage 1: Core Agent Engine & Database Layer

**Objective:** Implement the ReAct reasoning engine, schema migration, and store APIs.

#### Tasks:
1. **Schema Migration v6**:
   * Create `internal/store/migrations/m0006_agent_proposals.sql`.
   * Register migration in `internal/store/migrate.go`.
   * Add Store data structures and CRUD methods in `internal/store/proposals.go`:
     * `CreateProposal(ctx, p *Proposal) (int64, error)`
     * `ListProposals(ctx, scopeID int64, status string, limit int) ([]Proposal, error)`
     * `GetProposal(ctx, id int64) (*Proposal, error)`
     * `UpdateProposalStatus(ctx, id int64, status string) error`
     * `ApplyProposal(ctx, id int64) error` (Executes the underlying merge/link/archive within an atomic transaction).
   * Add Conversation persistence in `internal/store/conversations.go`:
     * `CreateConversation`, `ListConversations`, `GetConversationMessages`, `AppendMessage`.
2. **Unified LLM Client (`internal/agent/client.go`)**:
   * Implement a lightweight, zero-dependency streaming and non-streaming HTTP client compatible with OpenAI `/v1/chat/completions`.
   * Support tool definitions (`tools` and `tool_calls` parameter format).
   * Integrate environment variable resolution (`config.ResolveAPIKey`).
3. **Tool Definition & Store Adapters (`internal/agent/tools.go`)**:
   * Define structured tools exposed to the agent:
     * `search_memories(query string, scope string, limit int)`
     * `read_memory(id int64)`
     * `inspect_links(id int64)`
     * `propose_link(from_id, to_id int64, relation, reasoning string)`
     * `propose_merge(source_ids []int64, title, content, tags, reasoning string)`
     * `detect_knowledge_gaps(topic, scope string)`
4. **ReAct Reasoning Loop (`internal/agent/engine.go`)**:
   * Implement step execution: Prompt generation $\to$ Model call $\to$ Tool evaluation $\to$ Observation generation $\to$ Termination / Response synthesis.
   * Add cycle guard (`max_reasoning_steps`, default 8).
   * Add deterministic fallback: if LLM is offline or disabled, return structured guidance directing the user to setup with raw hybrid search fallback.

#### Verification for Stage 1:
- Unit tests in `internal/store/proposals_test.go` verifying proposal creation, listing, and atomic application.
- Mock server tests in `internal/agent/engine_test.go` verifying ReAct loop terminates, correctly invokes tools, handles tool errors, and falls back when the LLM is unreachable.

---

### Stage 2: CLI Command Suite

**Objective:** Implement developer-facing CLI commands for inquiry, curation, and proposal review.

#### Tasks:
1. **`centmem ask` (`cmd/centmem/handlers_ask.go`)**:
   * Syntax:
     ```bash
     centmem ask "<question>" [--scope <scope>] [--top N] [--interactive]
     ```
   * Flow: Runs `agent.Ask(ctx, question, scope)`.
   * Standard output JSON:
     ```json
     {
       "ok": true,
       "answer": "We use SQLite with sqlite-vec for 100% offline agent memory...",
       "citations": [
         {
           "id": 42,
           "type": "note",
           "scope": "project:cent-mem",
           "snippet": "Chose SQLite-vec with local ONNX embeddings...",
           "score": 0.0412
         }
       ],
       "knowledge_gaps": [],
       "reasoning_steps": 2
     }
     ```
   * `--interactive` mode: Enters a multi-turn terminal chat session using `readline`.
2. **`centmem curate` (`cmd/centmem/handlers_curate.go`)**:
   * Syntax:
     ```bash
     centmem curate [--scope <scope>] [--type contradictions|dedup|all] [--apply] [--dry-run]
     ```
   * Analyzes memory clusters using Stage 1 ReAct tools.
   * Generates proposals in `agent_proposals` table.
   * If `--apply` is set, automatically applies proposals that exceed `confidence_threshold`.
3. **`centmem summarize` (`cmd/centmem/handlers_summarize.go`)**:
   * Syntax:
     ```bash
     centmem summarize [--scope <scope>] [--focus <topic>] [--format markdown|json] [--save]
     ```
   * Synthesizes high-level project briefings, conventions, and architectural pillars.
   * If `--save` is set, writes the resulting summary as a new `type=note` memory tagged `summary,architecture`.
4. **`centmem proposals` (`cmd/centmem/handlers_proposals.go`)**:
   * Syntax:
     ```bash
     centmem proposals list [--scope <scope>] [--status pending|applied|dismissed]
     centmem proposals show <id>
     centmem proposals apply <id>
     centmem proposals dismiss <id>
     ```
   * Full management surface for staged agent actions.

#### Verification for Stage 2:
- Golden JSON contract tests in `cmd/centmem/testdata/` for `ask`, `curate`, `summarize`, and `proposals`.
- CLI end-to-end integration tests in `cmd/centmem/agent_cli_test.go`.

---

### Stage 3: Web UI Experience (Assistant Chat & Proposals Inbox)

**Objective:** Deliver an integrated visual experience in the embedded Web UI for conversation and proposal review.

#### Tasks:
1. **REST Endpoints in `internal/ui/server.go`**:
   * `POST /api/agent/chat` — Accepts `{ conversation_id, message, scope }`. Supports Server-Sent Events (SSE) streaming for real-time response chunks and citations.
   * `GET /api/agent/conversations` — Lists conversation threads for the current scope.
   * `GET /api/agent/conversations/:id/messages` — Retrieves thread message history.
   * `GET /api/proposals` — Lists proposals with status filter (`pending`, `applied`, `dismissed`).
   * `POST /api/proposals/:id/apply` — Applies a staged proposal (atomic transaction).
   * `POST /api/proposals/:id/dismiss` — Dismisses a proposal.
   * All endpoints honor `--token` Bearer authentication.
2. **Web UI Frontend Components (`ui/src/`)**:
   * **Assistant View / Tab (`ui/src/components/AssistantTab.tsx`)**:
     * High-density chat interface matching Impeccable & Antislop-UI design principles.
     * Streaming markdown rendering with syntax-highlighted code blocks.
     * Clickable citation badges: clicking a citation opens that memory directly in the drawer.
     * Knowledge gap alerts with one-click "Record Memory for this topic" shortcut.
   * **Proposals Review Center (`ui/src/components/ProposalsView.tsx`)**:
     * Visual diff display for merge proposals: shows source memories side-by-side with proposed consolidated text.
     * Directional relationship preview for link proposals (`[from] ── supersedes ──► [to]`).
     * 1-click **Approve & Apply** and **Dismiss** actions with undo confirmation toasts.
   * **Sidebar Navigation & Header Integration**:
     * Live badge showing pending proposal count (e.g., `Proposals [3]`).
     * Quick assistant drawer trigger accessible via keyboard shortcut (`Cmd+K` or `?`).

#### Verification for Stage 3:
- REST API unit tests in `internal/ui/server_agent_test.go` verifying streaming SSE and proposal application.
- Frontend build validation (`cd ui && npm run build`).

---

### Stage 4: Hardening, Benchmarking & Documentation

**Objective:** Ensure offline resilience, stress-test concurrency, update documentation, and update graphify.

#### Tasks:
1. **Offline & Graceful Degradation Hardening**:
   * Verify behavior when LLM endpoint is down:
     * `centmem ask` outputs hybrid search results with a clear configuration hint.
     * `centmem curate` falls back to heuristic keyword/hash deduplication.
     * Web UI Chat displays a setup alert linking directly to Settings.
2. **Daemon Integration & Concurrency**:
   * Verify `centmemd` handles background curation passes without locking SQLite during concurrent agent writes.
3. **Documentation Updates**:
   * Update `docs/cli-contract.md` with `ask`, `curate`, `summarize`, and `proposals` specifications.
   * Update `docs/data-model.md` with migration v6 tables.
   * Update `docs/architecture.md` documenting `internal/agent`.
   * Create `docs/guides/agent-guide.md`.
   * Update `README.md` and `CHANGELOG.md`.
4. **Knowledge Graph Update**:
   * Run `graphify update .` to index the new packages and symbols.

---

## 5. Milestone Verification Checklist

### Stage 1: Core Engine & DB
- [ ] Schema migration `m0006_agent_proposals.sql` applies cleanly on top of v5
- [ ] `Proposal` and `Conversation` CRUD operations tested with 100% test coverage
- [ ] Unified LLM client verified against Ollama mock and OpenAI mock endpoints
- [ ] ReAct reasoning loop verified with tool-calling step sequence

### Stage 2: CLI Commands
- [ ] `centmem ask "how do we ship?"` returns synthesized answer with valid citation IDs
- [ ] `centmem curate --type contradictions` detects conflicting memories and creates pending link proposals
- [ ] `centmem curate --type dedup` detects redundant memories and creates merge proposals
- [ ] `centmem summarize` produces structured Markdown architectural overview
- [ ] `centmem proposals list/apply/dismiss` successfully transitions proposal states and modifies underlying memories
- [ ] CLI contract golden files passing `scripts/check_contract.sh`

### Stage 3: Web UI Experience
- [ ] `GET /api/proposals` and `POST /api/proposals/:id/apply` functional
- [ ] SSE streaming chat endpoint `POST /api/agent/chat` functional
- [ ] Web UI Assistant Chat tab renders messages, citations, and gap indicators cleanly
- [ ] Proposals Review Center renders merge diffs and relationship previews with 1-click apply
- [ ] UI builds cleanly without TypeScript or bundle errors (`npm run build`)

### Stage 4: Release & Documentation
- [ ] Graceful fallback tested with zero configured LLMs
- [ ] Documentation updated (`cli-contract.md`, `data-model.md`, `architecture.md`, `README.md`)
- [ ] Full Go test suite passing (`go test -tags fts5 ./... -race`)
- [ ] Version bumped to `2.0.0`
- [ ] `graphify update .` executed
