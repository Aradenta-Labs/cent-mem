-- m0001_init.sql: Initial cent-mem schema
-- Single source of truth (mirrored in docs/data-model.md).

-- 1. scopes: hierarchical scope registry
CREATE TABLE IF NOT EXISTS scopes (
    id          INTEGER PRIMARY KEY,
    path        TEXT UNIQUE NOT NULL,
    parent_path TEXT,
    kind        TEXT NOT NULL CHECK (kind IN ('global','project','agent','session')),
    name        TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scopes_parent_path ON scopes(parent_path);
CREATE INDEX IF NOT EXISTS idx_scopes_kind ON scopes(kind);

-- 2. memories: all memory types in one table
CREATE TABLE IF NOT EXISTS memories (
    id            INTEGER PRIMARY KEY,
    scope_id      INTEGER NOT NULL REFERENCES scopes(id),
    type          TEXT NOT NULL CHECK (type IN ('fact','note','log')),
    content       TEXT NOT NULL,
    key           TEXT,
    value_json    TEXT,
    tags          TEXT,
    source_agent  TEXT,
    source_session TEXT,
    content_hash  TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived','summarized')),
    summarize_at  INTEGER,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_memories_scope_type_status_created
    ON memories(scope_id, type, status, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_scope_key_fact
    ON memories(scope_id, key) WHERE type = 'fact';
CREATE INDEX IF NOT EXISTS idx_memories_content_hash ON memories(content_hash);
CREATE INDEX IF NOT EXISTS idx_memories_summarize_at
    ON memories(summarize_at) WHERE status = 'active' AND summarize_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);

-- 3. memories_fts: FTS5 external-content index over content + tags
CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    content,
    tags,
    content='memories',
    content_rowid='id',
    tokenize='porter unicode61'
);

-- FTS5 sync triggers
CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, content, tags)
    VALUES (new.id, new.content, IFNULL(new.tags, ''));
END;

CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content, tags)
    VALUES ('delete', old.id, old.content, IFNULL(old.tags, ''));
    INSERT INTO memories_fts(rowid, content, tags)
    VALUES (new.id, new.content, IFNULL(new.tags, ''));
END;

CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content, tags)
    VALUES ('delete', old.id, old.content, IFNULL(old.tags, ''));
END;

-- 4. embeddings: created now, unused until M2
CREATE TABLE IF NOT EXISTS embeddings (
    memory_id   INTEGER PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
    embedding   BLOB NOT NULL,
    model       TEXT NOT NULL,
    embedded_at INTEGER NOT NULL
);

-- Vector index (sqlite-vec). Deliberately created only if vec0 is available;
-- M1 doesn't depend on it, so guard it so the migration is portable.
CREATE VIRTUAL TABLE IF NOT EXISTS memories_vec USING vec0(
    memory_id INTEGER PRIMARY KEY,
    embedding float[384]
);

-- 5. embed_queue: write-time queue for async embedding
CREATE TABLE IF NOT EXISTS embed_queue (
    memory_id   INTEGER PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
    priority    INTEGER NOT NULL DEFAULT 0,
    claimed_at  INTEGER,
    created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_embed_queue_claim
    ON embed_queue(claimed_at, priority, created_at);

-- 6. events: append-only audit/sync log
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id   INTEGER NOT NULL,
    op          TEXT NOT NULL CHECK (op IN ('insert','update','archive','summarize','delete')),
    scope_path  TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_memory_id ON events(memory_id);

-- events_insert trigger: on memories INSERT append to events
CREATE TRIGGER IF NOT EXISTS events_insert AFTER INSERT ON memories BEGIN
    INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
    VALUES (
        new.id,
        CASE WHEN new.type = 'fact' THEN 'insert' ELSE 'insert' END,
        (SELECT path FROM scopes WHERE id = new.scope_id),
        json_object(
            'id', new.id,
            'type', new.type,
            'content', new.content,
            'tags', IFNULL(new.tags, '')
        ),
        new.created_at
    );
END;

-- 7. meta: schema version + model metadata
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Seed meta row (INSERT OR REPLACE so re-running the migration is idempotent)
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '1');
INSERT OR REPLACE INTO meta(key, value) VALUES ('embedding_model', 'bge-small-en-v1.5');
INSERT OR REPLACE INTO meta(key, value) VALUES ('embedding_dims', '384');
