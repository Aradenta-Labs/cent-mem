-- m0002_vec.sql: Vector index hardening for M2 (embeddings + hybrid search).
--
-- m0001 already created `memories_vec` (guarded so the schema is portable).
-- This migration makes the vec index explicit, idempotently, and adds the
-- covering index that hybrid recall relies on to filter by status + scope.
--
-- The vec0 CREATE VIRTUAL TABLE is wrapped so that a missing vec0 extension
-- aborts the migration with a clear error (see the Go migration runner).

-- Vector index (sqlite-vec). Idempotent; existing table from m0001 is kept.
CREATE VIRTUAL TABLE IF NOT EXISTS memories_vec USING vec0(
    memory_id INTEGER PRIMARY KEY,
    embedding float[384]
);

-- Covering index for recall: narrows semantic/keyword scans to active rows
-- within a scope.
CREATE INDEX IF NOT EXISTS idx_memories_status_scope
    ON memories(status, scope_id);
