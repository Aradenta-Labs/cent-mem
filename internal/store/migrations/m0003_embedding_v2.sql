-- m0003_embedding_v2.sql: Tag-weighted embedding format upgrade.
-- Enqueues all active memories for re-indexing with the new structured format.

-- Idempotently enqueue all existing active memories into embed_queue.
INSERT OR IGNORE INTO embed_queue(memory_id, priority, created_at)
SELECT id, 0, CAST(strftime('%s', 'now') AS INTEGER) * 1000000
FROM memories
WHERE status = 'active';

-- Record embedding format version in meta table
INSERT OR REPLACE INTO meta(key, value) VALUES ('embedding_version', '2');

-- Bump schema version to 3
INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '3');
