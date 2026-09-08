-- m0005_memory_links.sql: Memory relationship link graph.

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

-- Fast bidirectional graph traversal
CREATE INDEX IF NOT EXISTS idx_memory_links_from ON memory_links(from_id);
CREATE INDEX IF NOT EXISTS idx_memory_links_to ON memory_links(to_id);
CREATE INDEX IF NOT EXISTS idx_memory_links_suggested ON memory_links(suggested);

-- Update schema version in meta
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '5');
