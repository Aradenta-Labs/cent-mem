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

## Scenario 3: Responding to `/centmem` User Invocations

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
