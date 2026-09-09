# Agent Workflow Examples — centmem

This guide demonstrates how AI coding agents interact with cent-mem during a real development session.

---

## Scenario 1: Implementing OAuth Authentication

### Step 1: Session Start (READ)
The agent receives the prompt: *"Add Google OAuth2 login to our backend service."*

Before writing any code, the agent recalls past decisions and architectural constraints:
```bash
centmem recall "OAuth authentication architecture conventions" --scope "project:backend-api" --top 3
```

**Output returned to agent:**
```json
{
  "ok": true,
  "memories": [
    {
      "id": 8,
      "type": "note",
      "content": "All auth tokens must be signed with RS256; JWKS URL exposed at /.well-known/jwks.json",
      "tags": ["auth", "decision"],
      "score": 0.041
    },
    {
      "id": 14,
      "type": "fact",
      "key": "auth.cookie_name",
      "value": "app_session_id"
    }
  ]
}
```

The agent knows to use `RS256` and name the session cookie `app_session_id` without asking the user.

---

### Step 2: Task Execution (DO)
The agent designs the callback handler, tests the token exchange, and wires the user persistence layer.

---

### Step 3: Session Completion (UPDATE)
Before ending the turn, the agent logs new decisions and endpoints to `centmem`:

```bash
# Store the new architecture decision
centmem put \
  --scope "project:backend-api" \
  --type note \
  --content "Google OAuth callback endpoint mounted at /api/v1/auth/google/callback; redirects to /dashboard with session cookie." \
  --tags decision,auth \
  --source-agent "antigravity"

# Store the fact
centmem set \
  --scope "project:backend-api" \
  --key "auth.google.callback_url" \
  --value '"/api/v1/auth/google/callback"' \
  --tags auth,endpoints
```

---

## Scenario 2: Repository Knowledge Ingestion (Pre-Session Bootstrapping)

When an agent joins a new or existing repository, it can immediately build a persistent mental model of the codebase without needing a user to manually summarize prior work.

### Step 1: Ingesting Repository Artifacts

The agent runs developer artifact capture commands during repository onboarding:

```bash
# 1. Ingest recent Git history, architectural commit decisions, and package dependencies
centmem capture git --scope "project:backend-api" --max-commits 50

# 2. Ingest project architecture documentation and Markdown guides
centmem capture docs --scope "project:backend-api" --dir docs

# 3. Ingest active TODOs and FIXMEs across source code
centmem capture comments --scope "project:backend-api" --dir src --ext go,ts
```

**Output returned from Git capture:**
```json
{
  "ok": true,
  "command": "capture git",
  "source": ".git",
  "scanned": 50,
  "commits_scanned": 50,
  "memories_created": 8,
  "memories_updated": 0,
  "cursor": "f4a1c2d3e4b567890abcdef1234567890abcdef1",
  "items": [
    {
      "id": 112,
      "type": "note",
      "tags": ["git", "commit", "decision"],
      "content": "Commit 3b8e1f0: refactor(db): migrate connection pooling from lib/pq to pgx/v5 for native context support",
      "dry_run": false
    },
    {
      "id": 113,
      "type": "fact",
      "tags": ["git", "dependency"],
      "content": "dep.github.com/jackc/pgx/v5 = v5.5.0",
      "dry_run": false
    }
  ]
}
```

### Step 2: Instant Grounded Recall

Now, when asked to implement a database query or fix a connection bug, the agent immediately grounds its work on the captured knowledge:

```bash
centmem recall "database connection pool conventions" --scope "project:backend-api" --top 3
```

The agent retrieves the pgx migration decision and uses `pgx/v5` natively without trial and error.

---

## Scenario 3: Superseding & Refining Decisions (Link Graph)

When project requirements evolve, older architectural decisions may be replaced, refined, or contradicted. Rather than leaving obsolete decisions active in retrieval, agents use typed relationships (`supersedes`, `refines`, `contradicts`, `depends-on`, `supports`) to link memories into an explorable relationship graph.

### Step 1: Writing an Updated Decision & Handling Auto-Suggestions

The agent writes a new decision that obsoletes an earlier choice:
```bash
centmem put \
  --scope "project:backend-api" \
  --type note \
  --content "Switched vector database from pgvector to sqlite-vec for 100% embedded offline operation; pgvector is deprecated." \
  --tags decision,vector,storage
```

During write execution, centmem's heuristic classifier detects the transition cue ("switched ... from ... deprecated") and performs an automatic similarity pass. The JSON response returns the new memory alongside `suggested_links`:

```json
{
  "ok": true,
  "id": 142,
  "scope": "project:backend-api",
  "status": "queued",
  "suggested_links": [
    {
      "id": 18,
      "from_id": 142,
      "to_id": 45,
      "relation": "supersedes",
      "target_content": "Store embeddings in Postgres using pgvector extension.",
      "target_type": "note"
    }
  ]
}
```

### Step 2: Confirming or Establishing Links

The agent reviews the suggested link and promotes it from pending to confirmed:
```bash
centmem link confirm 18
```

Alternatively, if creating an explicit relationship without relying on auto-suggestions, the agent links them directly:
```bash
centmem link 142 45 --relation supersedes
```

If another sub-decision refines this architecture (e.g., embedding model dimensions), the agent links it with `refines`:
```bash
centmem put \
  --scope "project:backend-api" \
  --type note \
  --content "sqlite-vec index uses 384-dimensional cosine distance for bge-small-en-v1.5." \
  --tags decision,vector,sqlite-vec

# Link as a refinement of memory #142
centmem link 143 142 --relation refines
```

### Step 3: Verifying with Graph-Enriched Recall

Future agents recalling vector database conventions pass `--include-links` to receive 1-hop relationship context:

```bash
centmem recall "vector database choice" --scope "project:backend-api" --include-links
```

**Output returned to agent:**
```json
{
  "ok": true,
  "query": "vector database choice",
  "results": [
    {
      "id": 142,
      "type": "note",
      "scope": "project:backend-api",
      "content": "Switched vector database from pgvector to sqlite-vec for 100% embedded offline operation; pgvector is deprecated.",
      "score": 0.412,
      "links": [
        {
          "relation": "supersedes",
          "direction": "outgoing",
          "linked_id": 45,
          "linked_content": "Store embeddings in Postgres using pgvector extension."
        },
        {
          "relation": "refines",
          "direction": "incoming",
          "linked_id": 143,
          "linked_content": "sqlite-vec index uses 384-dimensional cosine distance for bge-small-en-v1.5."
        }
      ]
    }
  ]
}
```

By inspecting `links`, the agent immediately sees that Memory #142 supersedes #45 and is refined by #143, eliminating the risk of acting on the deprecated Postgres/pgvector decision.

---

## Scenario 4: Responding to `/centmem` User Invocations

When the user types:
> `/centmem what database are we using and what were the migration rules?`

The agent recognizes the **Recall Intent** and executes:
```bash
centmem recall "database migration rules" --scope "project:backend-api" --top 5
```
It reads the JSON response and synthesizes a direct answer for the user.

When the user types:
> `/centmem remember: never use raw SQL queries, always use sqlc`

The agent recognizes the **Save Intent** and executes:
```bash
centmem put \
  --scope "project:backend-api" \
  --type note \
  --content "Never use raw SQL queries; all database access must use sqlc." \
  --tags decision,database,convention
```
It confirms to the user that the convention is saved.

When the user types:
> `/centmem link memory 142 to 45 with supersedes`

The agent recognizes the **Relationships / Links Intent** and executes:
```bash
centmem link 142 45 --relation supersedes
```
It returns confirmation to the user.

When the user types:
> `/centmem configure llm backend to ollama with model deepseek-r1:8b`

The agent recognizes the **Configuration / Agent Engine Intent** and executes:
```bash
centmem config set llm.backend ollama
centmem config set llm.model "deepseek-r1:8b"
```
It confirms the updated settings to the user.

---

## Scenario 5: Configuring Built-in AI Agent & Stage 1 Schema v6 (v2.0.0 Preview)

In v2.0.0 Stage 1, centmem introduces a built-in cognitive reasoning engine (`internal/agent`) and Schema v6 staging tables (`agent_proposals`, `agent_conversations`, `agent_messages`).

### Step 1: Inspecting & Configuring the Unified LLM Backend
The agent checks the active LLM backend and configures it to point to a local Ollama instance:

```bash
# Verify current settings
centmem config get llm

# Update provider and model
centmem config set llm.backend ollama
centmem config set llm.endpoint "http://127.0.0.1:11434/v1"
centmem config set llm.model "deepseek-r1:8b"
centmem config set agent.max_reasoning_steps 8
centmem config set agent.confidence_threshold 0.75
```

**Output returned:**
```json
{
  "ok": true,
  "key": "llm.model",
  "value": "deepseek-r1:8b"
}
```

### Step 2: Understanding ReAct Proposals & Human-in-the-Loop Curation
The autonomous curation loop identifies conflicting or duplicate memories using internal store tools (`search_memories`, `read_memory`, `inspect_links`, `detect_knowledge_gaps`). Instead of silently mutating knowledge records, it stages reversible proposals in SQLite (`agent_proposals` table):

- **Link Proposal**: Proposes establishing semantic edges (`supersedes`, `contradicts`, `depends-on`).
- **Merge Proposal**: Proposes consolidating multiple fragmented memories into a single canonical note while archiving the sources.

Once Stage 2 CLI commands (`centmem proposals`) and Stage 3 Web UI are active, agents and developers review, approve, or dismiss these proposals with atomic transaction guarantees.
