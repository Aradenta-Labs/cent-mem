# Agent Workflow Examples — centmem

This guide demonstrates how AI coding agents interact with cent-mem during a real development session.

---

## Scenario: Implementing OAuth Authentication

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

## Scenario: Responding to `/centmem` User Invocations

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
