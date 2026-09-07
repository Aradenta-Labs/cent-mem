-- m0004_importance.sql: Access frequency tracking and importance scoring.
-- Adds access_count and last_accessed_at to memories table.

ALTER TABLE memories ADD COLUMN access_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memories ADD COLUMN last_accessed_at INTEGER; -- unix microseconds

-- Create index on access_count for fast distribution statistics and sorting
CREATE INDEX IF NOT EXISTS idx_memories_access_count ON memories(access_count);
CREATE INDEX IF NOT EXISTS idx_memories_last_accessed_at ON memories(last_accessed_at);

-- Record schema version
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '4');
