-- Migration 0006: Agent Proposals & Conversation Threads

-- 1. Agent Proposals Queue (Human-in-the-loop staging for merges, links, updates, archives)
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
    id            TEXT PRIMARY KEY,
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
    citations_json  TEXT,
    tool_calls_json TEXT,
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation 
    ON agent_messages(conversation_id, created_at ASC);

-- Update schema version in meta
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '6');
