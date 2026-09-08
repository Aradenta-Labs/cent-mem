package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CreateLink creates or updates a relationship link between two memories.
// If a link with (from_id, to_id, relation) already exists, it updates suggested to 0
// if the new link is confirmed (suggested == false).
func (s *Store) CreateLink(ctx context.Context, fromID, toID int64, relation string, suggested bool) (*Link, error) {
	if fromID <= 0 || toID <= 0 {
		return nil, fmt.Errorf("store: invalid memory ids (%d, %d)", fromID, toID)
	}
	if fromID == toID {
		return nil, fmt.Errorf("store: cannot link memory %d to itself", fromID)
	}
	if !IsValidLinkRelation(relation) {
		return nil, fmt.Errorf("store: invalid link relation %q; must be one of: %s",
			relation, strings.Join(ValidLinkRelations, ", "))
	}

	// Verify both memories exist before inserting
	var dummy int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM memories WHERE id = ?", fromID).Scan(&dummy); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("store: source memory %d not found: %w", fromID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("store: check source memory %d: %w", fromID, err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM memories WHERE id = ?", toID).Scan(&dummy); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("store: target memory %d not found: %w", toID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("store: check target memory %d: %w", toID, err)
	}

	createdAt := nowMicro()
	suggestedInt := 0
	if suggested {
		suggestedInt = 1
	}

	query := `
INSERT INTO memory_links (from_id, to_id, relation, created_at, suggested)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(from_id, to_id, relation) DO UPDATE SET
  suggested = CASE WHEN excluded.suggested = 0 THEN 0 ELSE memory_links.suggested END`

	if _, err := s.db.ExecContext(ctx, query, fromID, toID, relation, createdAt, suggestedInt); err != nil {
		return nil, fmt.Errorf("store: create link: %w", err)
	}

	var link Link
	var linkCreatedAt int64
	var linkSuggested int
	err := s.db.QueryRowContext(ctx, `
SELECT id, from_id, to_id, relation, created_at, suggested
FROM memory_links
WHERE from_id = ? AND to_id = ? AND relation = ?`, fromID, toID, relation).
		Scan(&link.ID, &link.FromID, &link.ToID, &link.Relation, &linkCreatedAt, &linkSuggested)
	if err != nil {
		return nil, fmt.Errorf("store: get created link: %w", err)
	}

	link.Suggested = (linkSuggested != 0)
	link.CreatedAt = time.UnixMicro(linkCreatedAt)
	return &link, nil
}

// GetLinkByID retrieves a link by its primary key ID.
func (s *Store) GetLinkByID(ctx context.Context, linkID int64) (*Link, error) {
	var link Link
	var linkCreatedAt int64
	var linkSuggested int
	err := s.db.QueryRowContext(ctx, `
SELECT id, from_id, to_id, relation, created_at, suggested
FROM memory_links
WHERE id = ?`, linkID).
		Scan(&link.ID, &link.FromID, &link.ToID, &link.Relation, &linkCreatedAt, &linkSuggested)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("store: get link %d: %w", linkID, err)
	}

	link.Suggested = (linkSuggested != 0)
	link.CreatedAt = time.UnixMicro(linkCreatedAt)
	return &link, nil
}

// DeleteLink removes a link between two memories, optionally filtered by relation.
// If relation is empty, all links between fromID and toID are deleted.
// Returns the count of deleted rows.
func (s *Store) DeleteLink(ctx context.Context, fromID, toID int64, relation string) (int64, error) {
	var res sql.Result
	var err error
	if relation != "" {
		res, err = s.db.ExecContext(ctx, "DELETE FROM memory_links WHERE from_id = ? AND to_id = ? AND relation = ?", fromID, toID, relation)
	} else {
		res, err = s.db.ExecContext(ctx, "DELETE FROM memory_links WHERE from_id = ? AND to_id = ?", fromID, toID)
	}
	if err != nil {
		return 0, fmt.Errorf("store: delete link: %w", err)
	}
	return res.RowsAffected()
}

// DeleteLinkByID removes a link by its primary key.
func (s *Store) DeleteLinkByID(ctx context.Context, linkID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM memory_links WHERE id = ?", linkID)
	if err != nil {
		return 0, fmt.Errorf("store: delete link %d: %w", linkID, err)
	}
	return res.RowsAffected()
}

// ConfirmLink marks an auto-suggested link as confirmed (suggested = 0).
func (s *Store) ConfirmLink(ctx context.Context, linkID int64) error {
	res, err := s.db.ExecContext(ctx, "UPDATE memory_links SET suggested = 0 WHERE id = ?", linkID)
	if err != nil {
		return fmt.Errorf("store: confirm link %d: %w", linkID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: confirm link rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DismissLink removes a pending suggested link (suggested = 1).
func (s *Store) DismissLink(ctx context.Context, linkID int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM memory_links WHERE id = ? AND suggested = 1", linkID)
	if err != nil {
		return fmt.Errorf("store: dismiss link %d: %w", linkID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: dismiss link rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetLinksForMemory returns outgoing and incoming links for a memory, including content metadata.
func (s *Store) GetLinksForMemory(ctx context.Context, memoryID int64, includeSuggested bool) (outgoing []LinkWithContent, incoming []LinkWithContent, err error) {
	outgoing = []LinkWithContent{}
	incoming = []LinkWithContent{}

	// Outgoing links (memoryID -> to_id)
	outQuery := `
SELECT l.id, l.from_id, l.to_id, l.relation, l.suggested, l.created_at,
       mf.content, mt.content,
       mf.type, mt.type,
       sf.path, st.path
FROM memory_links l
JOIN memories mf ON l.from_id = mf.id
JOIN memories mt ON l.to_id = mt.id
JOIN scopes sf ON mf.scope_id = sf.id
JOIN scopes st ON mt.scope_id = st.id
WHERE l.from_id = ?`
	if !includeSuggested {
		outQuery += " AND l.suggested = 0"
	}
	outQuery += " ORDER BY l.created_at DESC"

	outRows, err := s.db.QueryContext(ctx, outQuery, memoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("store: query outgoing links for %d: %w", memoryID, err)
	}
	defer outRows.Close()

	for outRows.Next() {
		var item LinkWithContent
		var createdAt int64
		var suggestedInt int
		if err := outRows.Scan(
			&item.ID, &item.FromID, &item.ToID, &item.Relation, &suggestedInt, &createdAt,
			&item.SourceContent, &item.TargetContent,
			&item.SourceType, &item.TargetType,
			&item.SourceScope, &item.TargetScope,
		); err != nil {
			return nil, nil, fmt.Errorf("store: scan outgoing link: %w", err)
		}
		item.Suggested = (suggestedInt != 0)
		item.CreatedAt = time.UnixMicro(createdAt)
		outgoing = append(outgoing, item)
	}
	if err := outRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: iterate outgoing links: %w", err)
	}

	// Incoming links (from_id -> memoryID)
	inQuery := `
SELECT l.id, l.from_id, l.to_id, l.relation, l.suggested, l.created_at,
       mf.content, mt.content,
       mf.type, mt.type,
       sf.path, st.path
FROM memory_links l
JOIN memories mf ON l.from_id = mf.id
JOIN memories mt ON l.to_id = mt.id
JOIN scopes sf ON mf.scope_id = sf.id
JOIN scopes st ON mt.scope_id = st.id
WHERE l.to_id = ?`
	if !includeSuggested {
		inQuery += " AND l.suggested = 0"
	}
	inQuery += " ORDER BY l.created_at DESC"

	inRows, err := s.db.QueryContext(ctx, inQuery, memoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("store: query incoming links for %d: %w", memoryID, err)
	}
	defer inRows.Close()

	for inRows.Next() {
		var item LinkWithContent
		var createdAt int64
		var suggestedInt int
		if err := inRows.Scan(
			&item.ID, &item.FromID, &item.ToID, &item.Relation, &suggestedInt, &createdAt,
			&item.SourceContent, &item.TargetContent,
			&item.SourceType, &item.TargetType,
			&item.SourceScope, &item.TargetScope,
		); err != nil {
			return nil, nil, fmt.Errorf("store: scan incoming link: %w", err)
		}
		item.Suggested = (suggestedInt != 0)
		item.CreatedAt = time.UnixMicro(createdAt)
		incoming = append(incoming, item)
	}
	if err := inRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: iterate incoming links: %w", err)
	}

	return outgoing, incoming, nil
}

// GetLinksForMemories fetches all links attached to any of the specified memory IDs in batch.
// The returned map is keyed by memoryID, containing all links (outgoing and incoming) relevant to that memory.
func (s *Store) GetLinksForMemories(ctx context.Context, memoryIDs []int64, includeSuggested bool) (map[int64][]LinkWithContent, error) {
	out := make(map[int64][]LinkWithContent)
	if len(memoryIDs) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(memoryIDs))
	args := make([]any, 0, len(memoryIDs)*2)
	for i, id := range memoryIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, args...) // duplicate for both from_id and to_id
	inClause := strings.Join(placeholders, ", ")

	query := fmt.Sprintf(`
SELECT l.id, l.from_id, l.to_id, l.relation, l.suggested, l.created_at,
       mf.content, mt.content,
       mf.type, mt.type,
       sf.path, st.path
FROM memory_links l
JOIN memories mf ON l.from_id = mf.id
JOIN memories mt ON l.to_id = mt.id
JOIN scopes sf ON mf.scope_id = sf.id
JOIN scopes st ON mt.scope_id = st.id
WHERE (l.from_id IN (%s) OR l.to_id IN (%s))`, inClause, inClause)

	if !includeSuggested {
		query += " AND l.suggested = 0"
	}
	query += " ORDER BY l.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query batch links: %w", err)
	}
	defer rows.Close()

	idSet := make(map[int64]bool, len(memoryIDs))
	for _, id := range memoryIDs {
		idSet[id] = true
	}

	for rows.Next() {
		var item LinkWithContent
		var createdAt int64
		var suggestedInt int
		if err := rows.Scan(
			&item.ID, &item.FromID, &item.ToID, &item.Relation, &suggestedInt, &createdAt,
			&item.SourceContent, &item.TargetContent,
			&item.SourceType, &item.TargetType,
			&item.SourceScope, &item.TargetScope,
		); err != nil {
			return nil, fmt.Errorf("store: scan batch link: %w", err)
		}
		item.Suggested = (suggestedInt != 0)
		item.CreatedAt = time.UnixMicro(createdAt)

		if idSet[item.FromID] {
			out[item.FromID] = append(out[item.FromID], item)
		}
		if idSet[item.ToID] {
			out[item.ToID] = append(out[item.ToID], item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate batch links: %w", err)
	}

	return out, nil
}
