# v1.5.2 — Memory Relationships: Link Graph

**Version:** 1.5.2 (target)  
**Owner:** Aradenta Labs  
**Status:** Approved — Ready for implementation  
**Depends on:** v1.5.0 (Schema v4); backwards-compatible schema migration to v5  
**Roadmap Reference:** [docs/plans/roadmap-v1.5.x.md](roadmap-v1.5.x.md)

---

## 0. Executive Summary & Objective

In **v1.0–v1.5.1**, memories in centmem are stored as independent, isolated rows. While memories can share scopes, tags, and semantic similarity clusters, there is no formal mechanism to represent directional semantic relationships:
- Decision B *supersedes* Decision A.
- Fact Y *depends on* Fact X.
- Guideline M *contradicts* historical Rule N.
- Architecture Note P *refines* high-level Strategy Q.

**v1.5.2 (Memory Relationships: Link Graph)** introduces a lightweight, relational graph layer directly within the local SQLite store:
1. **Relational Schema (`m0005_memory_links.sql`):** Establishes the `memory_links` table with foreign key cascading deletes, bidirectional indexes, and a confirmation lifecycle (`suggested` vs `confirmed`).
2. **Explicit Link Operations:** CLI subcommands `centmem link`, `centmem unlink`, `centmem links`, `centmem link confirm`, and `centmem link dismiss`.
3. **Automated Relationship Suggestions on `put`:** Whenever a memory is written, a post-write similarity pass identifies related memories and infers suggested relations (`refines`, `contradicts`, `supersedes`, `supports`), returning them to agents for one-click confirmation.
4. **Graph-Enriched Recall (`centmem recall --include-links`):** Expands retrieved results with 1-hop connected memories, surfacing contradictions and dependencies before decisions are executed.
5. **Web UI Relationship Explorer:** Visual relationship drawer, confirmation actions for pending links, and an interactive graph topology view.

---

## 1. Problem Statement & Root Causes

| # | Current Limitation | Root Cause in Code | v1.5.2 Solution |
|---|---|---|---|
| 1 | Flat memory isolation | Schema (`internal/store/migrations/m0001_init.sql`): Only `memories` table exists; no edge table connects records. | Schema migration `m0005_memory_links.sql` adds `memory_links` edge table with typed relations. |
| 2 | Silent contradictions across sessions | Retrieval treats all matching memories independently. When an old decision contradicts a newer one, agents receive both without knowing which one superseded the other. | `centmem recall --include-links` annotates retrieved memories with explicit `supersedes` or `contradicts` edges. |
| 3 | High cognitive friction for manual graph building | Agents will not proactively link memories if it requires multiple manual roundtrips. | Post-write auto-suggestion evaluates cosine similarity and linguistic cues on every `centmem put`. |
| 4 | Orphaned links on memory deletion | Deleting a memory via `forget` or retention pruning leaves broken dangling references. | Foreign keys with `ON DELETE CASCADE` guarantee referential integrity at the database engine level. |

---

## 2. Data Model & Schema Migrations

### Migration `internal/store/migrations/m0005_memory_links.sql`

```sql
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
```

### Relation Taxonomy

| Relation | Direction Semantics (`from -> to`) | Example |
|---|---|---|
| `supersedes` | `New` replaces / renders obsolete `Old` | "Use sqlite-vec extension" $\to$ "Use custom Python vector store" |
| `contradicts` | `Memory A` conflicts with `Memory B` | "Port must be 4231" $\to$ "Port must be 8080" |
| `refines` | `Child` adds specific detail / rules to `Parent` | "Format JSON output strictly" $\to$ "Add indentation flag" |
| `depends-on` | `Component X` requires `Component Y` to work | "Web UI Settings" $\to$ "Config PATCH API" |
| `supports` | `Evidence A` corroborates or justifies `Decision B` | "Benchmark showing < 1ms WAL" $\to$ "Decided to adopt WAL" |

---

## 3. Technical Architecture & Lifecycles

```
                centmem put --content "Switched to composite re-ranker"
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │     Write Memory to SQLite      │  (memories table)
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │   Auto-Suggest Similarity Run   │  Recall top 5
                    │   (Score > 0.015, cosDist<0.45) │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │   Relation Heuristic Classifier │  Linguistic & Tag
                    │  (refines/contradicts/supports) │  Pattern Matching
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │  Insert memory_links (suggested=1)
                    └────────────────┬────────────────┘
                                     │
                                     ▼
                    ┌─────────────────────────────────┐
                    │ Return CLI Output with Suggestions
                    │ { "id": 88, "suggested_links": [...] }
                    └─────────────────────────────────┘
                                     │
                 ┌───────────────────┴───────────────────┐
                 ▼                                       ▼
    centmem link confirm <link-id>          centmem link dismiss <link-id>
    (UPDATE suggested = 0)                  (DELETE FROM memory_links)
```

---

## 4. Algorithmic Specifications: Auto-Suggestion

When `centmem put` stores a new memory:

1. **Candidate Retrieval:** Perform an internal recall query using the new memory's content against existing memories in the same or ancestor scopes (top 5 candidates, threshold score $\ge 0.015$).
2. **Exclusion:** Skip self-match (`id != new_id`) and memories that already have an existing link.
3. **Classification Heuristics:**
   - **Contradiction / Supersession:**
     If text contains transition or negation cues:
     `{"no longer", "deprecated", "replaced by", "switched from", "instead of", "obsolete", "discontinued"}`
     - If candidate contains similar keywords/tags $\to$ `supersedes`
     - Otherwise $\to$ `contradicts`
   - **Refinement:**
     If cosine distance $< 0.25$ AND share $\ge 2$ identical tags $\to$ `refines`
   - **Dependency:**
     If text contains requirement cues:
     `{"depends on", "requires", "prerequisite", "built on top of"}` $\to$ `depends-on`
   - **Support (Default):**
     If cosine distance $< 0.35$ $\to$ `supports`
4. **Insertion:** Links created through this pipeline are inserted with `suggested = 1`.

---

## 5. CLI Surface & JSON Output Contracts

### 5.1 Manual Linking (`centmem link`)

```bash
# Create a confirmed relationship
centmem link 42 87 --relation supersedes

# List relationships for a memory
centmem links 42

# Confirm an auto-suggested link
centmem link confirm 12

# Dismiss an auto-suggested link
centmem link dismiss 12

# Remove an existing link
centmem unlink 42 87
```

#### JSON Output (`centmem link 42 87 --relation supersedes`):
```json
{
  "ok": true,
  "link": {
    "id": 12,
    "from_id": 42,
    "to_id": 87,
    "relation": "supersedes",
    "suggested": false,
    "created_at": 1788749000
  }
}
```

#### JSON Output (`centmem links 42`):
```json
{
  "ok": true,
  "memory_id": 42,
  "outgoing": [
    {
      "link_id": 12,
      "relation": "supersedes",
      "target_id": 87,
      "target_type": "note",
      "target_content": "Use custom vector store",
      "suggested": false
    }
  ],
  "incoming": [
    {
      "link_id": 19,
      "relation": "supports",
      "source_id": 105,
      "source_type": "fact",
      "source_content": "SQLite busy_timeout benchmark",
      "suggested": true
    }
  ]
}
```

### 5.2 Recall with Link Context (`centmem recall --include-links`)

```json
{
  "ok": true,
  "query": "vector store implementation",
  "results": [
    {
      "id": 42,
      "type": "note",
      "content": "All vector search operations use sqlite-vec",
      "score": 0.4120,
      "links": [
        {
          "relation": "supersedes",
          "direction": "outgoing",
          "linked_id": 87,
          "linked_content": "Deprecated: custom Python vector index"
        }
      ]
    }
  ]
}
```

---

## 6. Internal Package Architecture

### 6.1 `internal/store` Layer (`internal/store/links.go`)

```go
type Link struct {
    ID        int64
    FromID    int64
    ToID      int64
    Relation  string
    Suggested bool
    CreatedAt time.Time
}

type LinkWithContent struct {
    Link
    SourceContent string
    TargetContent string
    SourceType    string
    TargetType    string
}

// Store methods:
func (s *Store) CreateLink(ctx context.Context, fromID, toID int64, relation string, suggested bool) (*Link, error)
func (s *Store) DeleteLink(ctx context.Context, fromID, toID int64) error
func (s *Store) GetLinksForMemory(ctx context.Context, memoryID int64) (outgoing []LinkWithContent, incoming []LinkWithContent, error)
func (s *Store) ConfirmLink(ctx context.Context, linkID int64) error
func (s *Store) DismissLink(ctx context.Context, linkID int64) error
```

### 6.2 `internal/search` Layer
- Modify `Query` struct: add `IncludeLinks bool`.
- Modify `Ranked` struct: add `Links []LinkedMemory`.
- Batch query `memory_links` for returned IDs when `q.IncludeLinks == true`.

---

## 7. Web UI Integration

1. **Detail Drawer Relationships Tab:**
   - Visual chips for outgoing and incoming links with color coding:
     - Red badge: `contradicts`
     - Orange badge: `supersedes`
     - Blue badge: `refines`
     - Green badge: `supports`
     - Purple badge: `depends-on`
2. **Pending Confirmation Action Banner:**
   - When viewing a memory with `suggested=1` links, show an amber action box:
     `"Auto-suggested relation: supersedes #87 [Confirm] [Dismiss]"`
3. **Interactive Graph Topology View:**
   - Mini canvas/SVG graph node viewer inside the drawer showing direct 1-hop and 2-hop connections.

---

## 8. Testing & Verification Plan

### Test Matrix:
1. **Schema Migration Integrity (`internal/store/migration_test.go`):**
   - Verify `m0005_memory_links.sql` applies cleanly on top of v4.
   - Verify check constraints: self-links (`from_id == to_id`) and invalid relations fail with SQLite errors.
2. **Cascade Deletion (`internal/store/links_test.go`):**
   - Create memory A and memory B; create link A $\to$ B.
   - Delete memory A via `s.Forget(A)`.
   - Verify link is automatically deleted by foreign key cascade.
3. **Auto-Suggest Heuristics (`internal/search/links_suggest_test.go`):**
   - Test rule triggers against test sentences ("Switched from X to Y" $\to$ `supersedes`).
4. **Graph-Enriched Recall (`cmd/centmem/handlers_test.go`):**
   - Query recall with `--include-links`; verify JSON structure and correct relationship payload.
5. **Cycle Resistance:**
   - Ensure graph queries with cyclic links (A $\to$ B $\to$ A) never cause infinite loops during 1-hop expansions.

---

## 9. Files Affected & Implementation Checklist

- [ ] **Migration:** `internal/store/migrations/m0005_memory_links.sql` [NEW]
- [ ] **Data Model:** `internal/store/types.go` & `internal/store/links.go` [NEW/MODIFY]
- [ ] **Search Engine:** `internal/search/search.go` (link expansion in `Recall`) [MODIFY]
- [ ] **Auto-Suggestion:** `internal/search/suggest_links.go` [NEW]
- [ ] **CLI Commands:** `cmd/centmem/handlers_links.go` (`cmdLink`, `cmdUnlink`, `cmdLinks`) [NEW]
- [ ] **CLI Wiring:** `cmd/centmem/main.go` & `commands.go` [MODIFY]
- [ ] **CLI Documentation:** `docs/cli-contract.md` [MODIFY]
- [ ] **Web UI:** `ui/src/components/MemoryDrawer.tsx` (Relations tab) [MODIFY]
- [ ] **Tests:** `internal/store/links_test.go` & `cmd/centmem/handlers_links_test.go` [NEW]
- [ ] **Knowledge Graph:** Update with `graphify update .` after completion
