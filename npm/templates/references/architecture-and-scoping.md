# Architecture & Scoping — centmem

This reference explains the internal architecture, scope resolution rules, and search fusion engine of `centmem`.

## System Overview

```
AI Agent (any harness) ──► centmem CLI (Go) ──► SQLite (WAL) + sqlite-vec + FTS5
                                                   └── local ONNX model (BGE-small 384d)
```

- **Local-First & Offline**: 100% on-device. No telemetry, no external API calls at runtime.
- **Embedded Database**: Single SQLite database file at `~/.centmem/centmem.db` with permissions `0600` in directory `0700`.
- **Fast Search**: p95 read < 300 ms across 100k memories; p95 write overhead < 50 ms.

---

## Scoping Grammar & Inheritance

cent-mem uses a 4-tier hierarchical scope model:

```
global
  └── project:<project-name>
        └── project:<project-name>/agent:<agent-name>
              └── project:<project-name>/agent:<agent-name>/session:<session-id>
```

### Path Syntax
- Identifiers must match `[a-z0-9-_.]+` (case-insensitive, normalized to lowercase).
- Examples:
  - `global`
  - `project:cent-mem`
  - `project:cent-mem/agent:claude`
  - `project:cent-mem/agent:antigravity/session:sess-9481`

### Inheritance Rules
1. **Ancestry Walk (Reads)**:
   - When querying `project:cent-mem/agent:claude`, the default `--inherit` flag walks up the tree to include memories in `project:cent-mem` and `global`.
   - To disable ancestor inheritance, pass `--no-inherit`.
2. **Descendant Search (Recall)**:
   - When searching at `project:cent-mem`, pass `--children` to also include all sub-agents and session memories beneath that project.
3. **Write Scoping**:
   - Writes always require an explicit `--scope`.
   - A write creates any non-existent ancestor scope records automatically.

---

## Hybrid Search Engine (Two-Stage Retrieval & Importance Scoring)

`centmem recall` implements a high-precision, two-stage retrieval pipeline combining multi-signal candidate generation, calibrated composite re-ranking, contextual affinity boosts, and dynamic importance scoring.

```
                    ┌─────────────────────────────────┐
                    │  Query Expansion (Tags/Context) │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │   Stage 1: Multi-Signal Search   │
                    │   (Vec + FTS5 + Facts + Timeline)│
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │  Stage 1 RRF Fusion & Filtering │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │  Stage 2: Composite Re-Ranking  │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │ Session & Agent Proximity Boost │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
            ┌─────────────────────────────────────────────────┐
            │        Importance Boost Multiplier (v1.5.0)     │
            │  importance = min(2.0, 1.0 + ln(1 + count)*0.1) │
            │                score *= importance              │
            └────────────────────────┬────────────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │    Recency Decay (if enabled)   │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │ Near-Dup Collapse & Top Slice   │
                    └────────────────┬────────────────┘
                                     │
                  ┌──────────────────┴──────────────────┐
                  │                                     │
                  ▼ (synchronous return)                ▼ (asynchronous background)
      Return Top-N JSON to caller             RecordAccessAsync Goroutine
      {"id": 42, "access_count": 14, ...}                │
                                                         ▼
                                              Batch Update SQLite:
                                              UPDATE memories
                                              SET access_count = access_count + 1,
                                                  last_accessed_at = ?
                                              WHERE id IN (...)
```

### Stage 1: Multi-Signal Candidate Generation & RRF Fusion
Four parallel search pipelines retrieve candidate memories:
1. **Semantic Vector Search**: Generates 384-dimensional dense vectors using local BGE-small-en-v1.5 via ONNX Runtime, computing cosine distance via `sqlite-vec`.
2. **Keyword Full-Text Search (FTS5)**: Tokenizes queries with SQLite FTS5 using the Porter stemmer and BM25 ranking.
3. **Exact Fact Matcher**: Evaluates exact key/value fact names, string values, and attached tags.
4. **Timeline Recency Scorer**: Generates candidate matches from recent chronological activity.

Candidates are fused using **Reciprocal Rank Fusion (RRF, $k=60$)**:
$$RRF(m) = \sum_{r \in Rankers} \frac{1}{60 + rank_r(m)}$$

This ensures balanced retrieval: memories that match either semantic intent or exact technical tokens are promoted without score skew.

### Stage 2: Composite Re-Ranking Pipeline
Stage 1 RRF produces a broad candidate pool. The Stage 2 re-ranker evaluates the top candidate window (configurable via `search.rerank_window`, default `30`) using in-process lexical and semantic feature scoring:
- **Dense Semantic Similarity ($S_{\text{sem}}$)**: Exact cosine similarity of query and content embeddings.
- **Lexical Token Coverage ($S_{\text{lex}}$)**: Fraction of query tokens present in the target content and tags.
- **Exact Phrase Bonus ($S_{\text{phrase}}$)**: Multiplier boost for contiguous substring matches.
- **Stage 1 Rank Signal ($S_{\text{rrf\_norm}}$)**: Preserves initial multi-signal fusion consensus.

#### Contextual Proximity & Affinity Boosts
Immediately following composite scoring:
- **Session Proximity Boost**: 1.25x (+25%) multiplier for memories created within the current active session scope.
- **Agent Affinity Boost**: 1.15x (+15%) multiplier for memories authored by the caller agent (matched via `--caller-agent` or `$CENTMEM_AGENT`).

### Access Frequency Importance Scoring
To reinforce memories that prove practically valuable to AI agents over time, centmem applies a sub-linear importance multiplier based on historical access count:

$$\text{importance}(c) = \min\left(2.0,\, 1.0 + \ln(1 + c) \times 0.1\right)$$

where $c = \text{access\_count} \ge 0$.

The candidate score is scaled directly:
$$\text{score} \leftarrow \text{score} \times \text{importance}(c)$$

#### Progression Curve
| Access Count ($c$) | $\ln(1 + c)$ | Multiplier | Boost (%) | Semantic Meaning |
|---|---|---|---|---|
| **0** | 0.000 | **1.000×** | +0.0% | Fresh / unaccessed memory (neutral baseline) |
| **1** | 0.693 | **1.069×** | +6.9% | Recalled once |
| **3** | 1.386 | **1.139×** | +13.9% | Moderately reinforced |
| **7** | 2.079 | **1.208×** | +20.8% | Established team pattern |
| **15** | 2.773 | **1.277×** | +27.7% | Frequently consulted decision |
| **35** | 3.584 | **1.358×** | +35.8% | Core architectural pillar |
| **100** | 4.615 | **1.462×** | +46.2% | Heavily referenced standard |
| **1,000** | 6.909 | **1.691×** | +69.1% | Universal convention |
| **$\ge 22,025$** | $\ge 10.0$ | **2.000×** | +100.0% | Hard ceiling cap ($2.0\times$) |

#### Core Properties & Guarantees
1. **Sub-linear Diminishing Returns**: Early recalls provide immediate reinforcement (+6.9% on first access), but the natural logarithmic curve mathematically bounds runaway score inflation.
2. **Bounded Ceiling ($2.0\times$)**: A hard cap prevents high-frequency memories from overpowering direct lexical or semantic query relevance on mismatched searches.
3. **Zero-Starvation Guarantee**: Unaccessed memories start at $1.0\times$ baseline. A fresh, highly relevant memory ($> 0.85$ semantic similarity) will readily outrank an irrelevant older memory despite high historical access counts.
4. **Recency Decay**: When configured (`search.decay_half_life_days > 0`), exponential decay is applied after importance boosting.

### Asynchronous Feedback Loop (Agent Recall Reinforcement)
When `centmem recall` finishes ranking and slices the top-$N$ winning results:
1. The JSON response containing `access_count` and `last_accessed_at` is returned synchronously to the caller with zero latency overhead.
2. A non-blocking background goroutine (`RecordAccessAsync`) is launched and tracked via `sync.WaitGroup` (`Store.wg`), avoiding data races by snapshotting and deduplicating IDs.
3. The store executes an atomic batch SQLite update within a 2-second timeout:
   ```sql
   UPDATE memories
   SET access_count = access_count + 1,
       last_accessed_at = ?
   WHERE id IN (...)
   ```
4. Upon shutdown, `Store.Close()` awaits `s.wg.Wait()`, guaranteeing all in-flight access updates are persisted without data loss.
5. Over time, memories actively recalled across agent workflows naturally float higher in future queries, while unused memories remain at baseline until pruned or summarized by compaction.

---

## Memory Relationships: Link Graph (v1.5.2)

In v1.5.2, centmem introduces a relational graph layer on top of SQLite, allowing memories to form directional semantic links. This addresses memory obsolescence, historical divergence, and prerequisite dependencies across agent sessions.

### Relational Schema (`memory_links`)

Relationship edges are stored in the `memory_links` table with strict referential integrity:

```sql
CREATE TABLE IF NOT EXISTS memory_links (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id     INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    to_id       INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    relation    TEXT NOT NULL CHECK(relation IN ('supports', 'refines', 'contradicts', 'depends-on', 'supersedes')),
    created_at  INTEGER NOT NULL, -- unix microseconds
    suggested   INTEGER NOT NULL DEFAULT 0, -- 0 = confirmed, 1 = auto-suggested pending confirmation
    UNIQUE(from_id, to_id, relation),
    CHECK(from_id != to_id)
);

CREATE INDEX IF NOT EXISTS idx_memory_links_from ON memory_links(from_id);
CREATE INDEX IF NOT EXISTS idx_memory_links_to ON memory_links(to_id);
CREATE INDEX IF NOT EXISTS idx_memory_links_suggested ON memory_links(suggested);
```

#### Key Properties:
- **Foreign Key Cascade (`ON DELETE CASCADE`)**: When a memory is pruned or forgotten (`centmem forget`), any connecting relationship links are deleted automatically by SQLite, preventing dangling references.
- **Cycle-Safe Traversal**: Indexes on `from_id` and `to_id` support fast bidirectional lookup without deep recursive traversal overhead.
- **Confirmation Lifecycle (`suggested`)**: Auto-suggested links created during `centmem put` start with `suggested = 1`. Agents or users can promote them to confirmed (`suggested = 0`) via `centmem link confirm <id>` or remove them via `centmem link dismiss <id>`.

---

### The 5 Directional Relations

Every relationship link is directed (`from_id` $\to$ `to_id`) and governed by explicit semantic rules:

| Relation | Direction Semantics (`from -> to`) | Use Case & Meaning | Example |
|---|---|---|---|
| `supersedes` | `New` replaces / obsoletes `Old` | Indicates that the source memory replaces the target memory, rendering previous guidelines or architectures obsolete. | Memory 142 ("Use sqlite-vec") $\to$ Memory 45 ("Use pgvector") |
| `refines` | `Child` adds specific detail to `Parent` | Indicates that the source memory elaborates, constrains, or specializes an existing broader decision. | Memory 143 ("sqlite-vec 384d cosine metric") $\to$ Memory 142 ("Use sqlite-vec") |
| `contradicts` | `Memory A` conflicts with `Memory B` | Flags a direct inconsistency or behavioral disagreement across agent sessions that requires reconciliation. | Memory 90 ("Port 8080 required") $\to$ Memory 52 ("Port 4231 required") |
| `depends-on` | `Component X` requires `Component Y` | Expresses a prerequisite or architectural dependency between decisions, configurations, or facts. | Memory 104 ("Web UI Settings") $\to$ Memory 88 ("Config PATCH API") |
| `supports` | `Evidence A` corroborates `Decision B` | Links supporting benchmarks, research, or audit logs that justify an architectural choice. | Memory 65 ("Benchmark: <1ms WAL latency") $\to$ Memory 40 ("Adopt SQLite WAL") |

---

### 1-Hop Graph-Enriched Recall Expansion

Standard recall returns isolated memory matches. By passing `--include-links` to `centmem recall`, retrieval performs a fast 1-hop bidirectional graph expansion:

```
[Candidate Memory from Recall]
       │
       ├─ (outgoing) ──[relation]──► [Target Memory]
       └─ (incoming) ◄──[relation]── [Source Memory]
```

#### Retrieval Behavior:
1. **Candidate Retrieval**: Hybrid search generates the top-$N$ ranked memories.
2. **1-Hop Link Expansion**: For each returned memory ID, `memory_links` is queried for all adjacent incoming and outgoing edges.
3. **Confirmed vs. Suggested Links**:
   - By default, `--include-links` returns only confirmed edges (`suggested = 0`).
   - Adding `--include-suggested` surfaces unconfirmed candidate links (`suggested = 1`) as well.
4. **Enriched Result Shape**:
   Each item in `results` includes a `links` array:
   ```json
   {
     "id": 142,
     "content": "Switched vector database from pgvector to sqlite-vec...",
     "score": 0.412,
     "links": [
       {
         "relation": "supersedes",
         "direction": "outgoing",
         "linked_id": 45,
         "linked_content": "Store embeddings in Postgres using pgvector extension."
       }
     ]
   }
   ```
5. **Obsolescence Guard**: When an agent recalls a decision, any outgoing `supersedes` or incoming/outgoing `contradicts` edges immediately notify the agent of newer or conflicting context before it takes action.

---

## Model Context Protocol (MCP) Integration (v1.5.3)

In v1.5.3, centmem introduces native Model Context Protocol (MCP) support over `stdio` (`centmem serve`), enabling direct agent tool calls without shell command overhead.

### MCP Stdio Architecture

```
┌────────────────────────────────────────────────────────┐
│                   AI Agent Harness                     │
│         (Claude Code, Cursor, Windsurf, Zed)           │
└───────────────────────────┬────────────────────────────┘
                            │
               JSON-RPC 2.0 over stdio
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│             centmem serve (MCP Server)                 │
│  - Methods: initialize, tools/list, tools/call         │
│  - Tools: recall, put, set, get, timeline, stats, forget│
└───────────────────────────┬────────────────────────────┘
                            │
              Direct SQLite Handle & Embedder
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│          SQLite Store (memories, links, facts)         │
└────────────────────────────────────────────────────────┘
```

#### Key Architecture Benefits:
1. **Zero-Dependency Stdio Protocol**: Built entirely on Go standard library (`encoding/json`, `bufio`, `os`), requiring no external daemon or network ports.
2. **Process Lifecycle**: The agent harness spawns `centmem serve` as a dedicated child process. It connects directly to the local SQLite database and embedding pipeline.
3. **Capture Context Enrichment**: External MCP tools (configured in `[capture.mcp]`) can be invoked by the classifier with a strict 5-second timeout and non-blocking fallback to enrich ambiguous commits and docs before memory classification.
4. **Remote REST Security**: The embedded Web UI server (`centmem ui`) supports secure remote deployment with mandatory constant-time Bearer token verification on non-loopback host bindings.

---

## Built-in AI Memory Agent Engine Architecture (v2.0.0 Stages 1–3)

In v2.0.0, centmem evolves from a passive storage database into an **active cognitive intelligence and autonomous curation layer**. Stage 1 established the core ReAct reasoning engine and Schema v6 persistence; Stage 2 introduced the native CLI command suite (`ask`, `curate`, `summarize`, `proposals`); and Stage 3 brings an interactive browser workspace with real-time SSE streaming and human-in-the-loop review.

### Cognitive Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────┐
│                        AI Agent & Client Invocations                   │
│   CLI: ask / curate / summarize   │   Web UI: Assistant & Proposals    │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                  internal/agent — Memory Agent Engine                  │
│                                                                        │
│   ┌────────────────────────┐         ┌──────────────────────────────┐  │
│   │   Unified LLM Client   │◄───────►│          ReAct Loop          │  │
│   │ (Ollama / OpenAI-cloud)│         │     (Plan ──► Act ──► Think) │  │
│   └────────────────────────┘         └──────────────┬───────────────┘  │
│                                                     │ Tool Calls       │
│                  ┌──────────────────────────────────┴───────────────┐  │
│                  ▼                                                  ▼  │
│         ┌─────────────────┐                                ┌─────────┐ │
│         │ search_memories │                                │ propose │ │
│         │ read_memory     │ ◄── Store Tool Adapters ──►    │  _link  │ │
│         │ inspect_links   │                                │ propose │ │
│         │ knowledge_gaps  │                                │  _merge │ │
│         └─────────────────┘                                └─────────┘ │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Atomic Transactions
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                      SQLite Persistence Layer (WAL)                    │
│                                                                        │
│  ┌───────────────────────┐   ┌────────────────────────┐                │
│  │       memories        │   │      memory_links      │                │
│  │ (notes, facts, logs)  │   │  (semantic link graph) │                │
│  └───────────────────────┘   └────────────────────────┘                │
│  ┌───────────────────────┐   ┌────────────────────────┐                │
│  │    agent_proposals    │   │  agent_conversations   │                │
│  │ (staged merges/links) │   │    & agent_messages    │                │
│  └───────────────────────┘   └────────────────────────┘                │
└────────────────────────────────────────────────────────────────────────┘
```

### 1. Schema v6 Specification (`m0006_agent_proposals.sql`)

Stage 1 introduces three dedicated tables supporting agentic workflows and interactive dialog:

```sql
-- 1. Agent Proposals Queue (Human-in-the-loop staging for merges, links, and updates)
CREATE TABLE IF NOT EXISTS agent_proposals (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    scope_id      INTEGER NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    proposal_type TEXT NOT NULL CHECK(proposal_type IN ('link', 'merge', 'update', 'archive')),
    status        TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'applied', 'dismissed')),
    title         TEXT NOT NULL,
    reasoning     TEXT NOT NULL,
    payload_json  TEXT NOT NULL,
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

-- 3. Agent Conversation Messages (Dialogue turns with structured citations)
CREATE TABLE IF NOT EXISTS agent_messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'system', 'tool')),
    content         TEXT NOT NULL,
    citations_json  TEXT, -- JSON array: [{"id": 42, "title": "...", "score": 0.035}]
    tool_calls_json TEXT, -- JSON array of tool calls/observations
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation 
    ON agent_messages(conversation_id, created_at ASC);
```

### 2. Built-in ReAct Reasoning Engine (`internal/agent/engine.go`)

The core engine drives autonomous cognitive operations using the **Plan ─► Act ─► Think** cycle:
- **Plan**: Evaluates query context and decides whether to search, read, inspect graph relationships, or formulate proposals.
- **Act**: Executes one or more registered store tools synchronously.
- **Think**: Consolidates observations into intermediate reasoning steps before responding or staging proposals.
- **Safety Guards**:
  - **Cycle Limiter**: Halts execution if iterations exceed `agent.max_reasoning_steps` (default `8`).
  - **Offline Fallback**: If the configured LLM backend is unreachable or disabled, falls back to deterministic raw hybrid search with actionable setup hints.

### 3. Store Tool Adapters (`internal/agent/tools.go`)

The engine interacts with persistent memory through 6 structured Go tool adapters:
1. `search_memories(query, scope, limit)`: Runs hybrid search and returns ranked memories with snippets.
2. `read_memory(id)`: Fetches complete content, metadata, timestamps, and access counts for an individual memory.
3. `inspect_links(id)`: Inspects incoming and outgoing graph edges (`supersedes`, `contradicts`, `refines`, etc.).
4. `propose_link(from_id, to_id, relation, reasoning)`: Creates a staged link proposal in `agent_proposals`.
5. `propose_merge(source_ids, title, content, tags, reasoning)`: Creates a staged consolidation proposal merging redundant memories into a canonical note.
6. `detect_knowledge_gaps(topic, scope)`: Analyzes retrieved memories to identify missing context or unaddressed questions.

### 4. Atomic Proposal Application (`ApplyProposal`)

When a proposal is approved:
- An atomic SQLite transaction is initiated.
- For `merge` proposals: The new consolidated note is inserted, bidirectional `supersedes` links are created, and the redundant source memories are marked as archived.
- For `link` proposals: The confirmed relationship is inserted into `memory_links`.
- Proposal status is updated to `applied` with `applied_at` timestamp.
- Sync events are appended to `events` table for multi-agent daemon replication (`centmemd`).

### 5. Unified LLM Configuration & Fallback Hierarchy

The agent engine is configured via `[llm]` and `[agent]` in `~/.centmem/config.toml`:
```toml
[llm]
backend = "ollama"                   # "ollama", "openai_compatible", or "disabled"
endpoint = "http://127.0.0.1:11434/v1"
model = "deepseek-r1:8b"
api_key = ""                         # Optional key or ENV reference (e.g. "OPENAI_API_KEY")
timeout_seconds = 60
max_tokens = 4096
temperature = 0.2

[agent]
enabled = true
max_reasoning_steps = 8
confidence_threshold = 0.75
auto_apply_safe_links = false
```

*Inheritance Rule:* If `[llm]` is not specified, centmem automatically inherits configuration from `[capture]` (`capture.backend`, `capture.local_llm_endpoint`, `capture.api_base_url`, `capture.api_key`), ensuring complete backward compatibility.

### 6. Agent CLI Suite Architecture (v2.0.0 Stage 2)

Stage 2 operationalized the agent reasoning engine into four dedicated CLI subcommands wired into the core router:

- **`centmem ask`**: Executes natural language inquiry through `agent.Engine.Ask()`.
  - In single-shot mode, it retrieves candidate memories via hybrid search, feeds them into the ReAct cycle, and outputs grounded answers with structured citation objects and knowledge gaps.
  - In `--interactive` mode, it initiates a terminal REPL supporting persistent multi-turn conversations, terminal commands (`/help`, `/clear`, `exit`), and streaming text output.
- **`centmem curate`**: Runs autonomous memory curation algorithms:
  - Contradiction Detection: Identifies semantic conflicts between memories within a scope and stages `link` or `archive` proposals.
  - Semantic Deduplication: Detects duplicate or heavily overlapping memories via embedding cosine similarity and FTS5 bm25, generating `merge` proposals.
  - `--apply` flag allows high-confidence proposals to be committed immediately, while `--dry-run` simulates curation passes without mutating the staging queue.
- **`centmem summarize`**: Generates high-level architectural briefs and scope summaries.
  - Uses `agent.Engine` to synthesize developer context, architectural pillars, and operational conventions from stored memories.
  - With `--save`, the generated summary is stored as a new `type=note` memory tagged `summary,architecture,digest`.
- **`centmem proposals`**: Provides a complete administrative surface (`list`, `show`, `apply`, `dismiss`) for the `agent_proposals` table, allowing developers and automated agents to inspect, approve, or discard pending actions.

### 7. Web UI Experience & Embedded REST/SSE Architecture (v2.0.0 Stage 3)

Stage 3 bridges the memory store and cognitive agent engine to a local Web UI browser workspace embedded directly in `centmem ui`:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Web UI Browser Client (React + Vite)                 │
│                                                                         │
│   ┌────────────────────────┐              ┌──────────────────────────┐  │
│   │   Assistant Chat Tab   │              │  Proposals Review Center │  │
│   │   (SSE Token Stream)   │              │  (Merge Diffs & Actions) │  │
│   └───────────┬────────────┘              └────────────┬─────────────┘  │
└───────────────┼────────────────────────────────────────┼────────────────┘
                │ EventSource / fetch                    │ REST JSON
                ▼                                        ▼
┌─────────────────────────────────────────────────────────────────────────┐
│               centmem ui — Embedded Go HTTP Server (internal/ui)        │
│                                                                         │
│   SSE Handler:                              REST Handlers:              │
│   • POST /api/agent/chat                    • GET  /api/agent/conversat… │
│     (Chunk flusher, citations, gaps)        • GET  /api/proposals       │
│                                             • POST /api/proposals/{id}… │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      Core Engine & SQLite Store                         │
│       internal/agent.Engine    │    internal/store.Store (WAL)          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 7.1 Assistant Chat & Streaming Protocol
- **Endpoint**: `POST /api/agent/chat`
- **Headers**: `Accept: text/event-stream`, `Cache-Control: no-cache`
- **Chunk Flusher**: Uses `http.ResponseController` and `http.Flusher` to push incremental token deltas and event payloads without buffer delay.
- **Event Lifecycle**:
  1. `event: delta`: Streaming text tokens `{"content": "...", "role": "assistant"}`
  2. `event: citations`: Array of grounded memory citations `[{"id": 1, "type": "note", "scope": "...", "snippet": "...", "score": 0.0412}]`
  3. `event: gaps`: Detected knowledge gaps where context is missing `["..."]`
  4. `event: done`: Completion metadata `{"conversation_id": "...", "reasoning_steps": 2, "fallback_used": false}`
  5. `event: error`: Emitted on agent or model failure `{"error": "..."}`
- **Thread Persistence**: Conversations and individual messages are tracked in `agent_conversations` and `agent_messages`. Threads can be listed via `GET /api/agent/conversations` and individual message histories retrieved via `GET /api/agent/conversations/{id}/messages`.

#### 7.2 Proposals Review Center & Lifecycle
- **Queue Inspection**: `GET /api/proposals` queries `agent_proposals` with filtering by `scope`, `status` (`pending`, `applied`, `dismissed`), and `type` (`merge`, `link`, `update`).
- **Visual Merge Diffs**: Redundant source memories and synthesized candidate content are presented side-by-side with color-coded additions/deletions before confirmation.
- **State Machine Transitions**:
  ```
                ┌──────────────┐
                │   pending    │
                └──────┬───────┘
                       │
             ┌─────────┴─────────┐
             ▼                   ▼
       POST .../apply      POST .../dismiss
             │                   │
             ▼                   ▼
      ┌─────────────┐     ┌─────────────┐
      │   applied   │     │  dismissed  │
      └─────────────┘     └──────┬──────┘
                                 │
                           POST .../reopen
                                 │
                                 ▼
                          ┌─────────────┐
                          │   pending   │
                          └─────────────┘
  ```
- **1-Click Actions**:
  - `POST /api/proposals/{id}/apply`: Commits the proposal changes inside an atomic SQLite transaction and records `applied_at`.
  - `POST /api/proposals/{id}/dismiss`: Sets proposal status to `dismissed`.
  - `POST /api/proposals/{id}/reopen`: Reverts a `dismissed` proposal back to `pending`.

#### 7.3 Embedded REST & SSE API Endpoint Reference

| Method | Endpoint | Query / Path / Body | Description |
|--------|----------|---------------------|-------------|
| `POST` | `/api/agent/chat` | Body: `{"conversation_id": "...", "message": "...", "scope": "...", "top": 5}` | Streams agent reasoning, citations, and answers over SSE |
| `GET` | `/api/agent/conversations` | Query: `scope`, `limit` (default 50, max 200) | Returns persisted conversation threads ordered by `updated_at DESC` |
| `GET` | `/api/agent/conversations/{id}/messages` | Path: `id` (string) | Returns chronological user and assistant messages with citation metadata |
| `GET` | `/api/proposals` | Query: `scope`, `status`, `type`, `limit`, `offset` | Returns staged proposals with pagination and filtering |
| `POST` | `/api/proposals/{id}/apply` | Path: `id` (int64) | Atomically executes staged changes (merge note creation, links, archives) |
| `POST` | `/api/proposals/{id}/dismiss` | Path: `id` (int64) | Updates proposal status to `dismissed` |
| `POST` | `/api/proposals/{id}/reopen` | Path: `id` (int64) | Restores a dismissed proposal to `pending` status |

