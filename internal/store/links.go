package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
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
	if relation != "" && !IsValidLinkRelation(relation) {
		return 0, fmt.Errorf("store: invalid link relation %q; must be one of: %s",
			relation, strings.Join(ValidLinkRelations, ", "))
	}
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

// ListSuggestedLinks returns pending auto-suggested links (suggested = 1) matching
// the optional scope subtree and relation filters, along with the total matching count.
func (s *Store) ListSuggestedLinks(ctx context.Context, q SuggestedLinkListQuery) ([]LinkWithContent, int, error) {
	where := []string{"l.suggested = 1"}
	var args []any

	if rel := strings.TrimSpace(q.Relation); rel != "" {
		if !IsValidLinkRelation(rel) {
			return nil, 0, fmt.Errorf("store: invalid link relation %q; must be one of: %s",
				rel, strings.Join(ValidLinkRelations, ", "))
		}
		where = append(where, "l.relation = ?")
		args = append(args, rel)
	}

	if scopePath := strings.TrimSpace(q.ScopePath); scopePath != "" && scopePath != "global" {
		sc, err := scope.Parse(scopePath)
		if err != nil {
			return nil, 0, fmt.Errorf("store: parse scope %q: %w", scopePath, err)
		}
		scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
		if err != nil {
			return nil, 0, fmt.Errorf("store: resolve scope %q: %w", scopePath, err)
		}
		if len(scopeIDs) == 0 {
			return []LinkWithContent{}, 0, nil
		}
		placeholders := make([]string, len(scopeIDs))
		for i, id := range scopeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		for _, id := range scopeIDs {
			args = append(args, id)
		}
		inClause := strings.Join(placeholders, ", ")
		where = append(where, fmt.Sprintf("(mf.scope_id IN (%s) OR mt.scope_id IN (%s))", inClause, inClause))
	}

	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`
SELECT COUNT(*)
FROM memory_links l
JOIN memories mf ON l.from_id = mf.id
JOIN memories mt ON l.to_id = mt.id
WHERE %s`, whereSQL)

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store: count suggested links: %w", err)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	listQuery := fmt.Sprintf(`
SELECT l.id, l.from_id, l.to_id, l.relation, l.suggested, l.created_at,
       mf.content, mt.content,
       mf.type, mt.type,
       sf.path, st.path
FROM memory_links l
JOIN memories mf ON l.from_id = mf.id
JOIN memories mt ON l.to_id = mt.id
JOIN scopes sf ON mf.scope_id = sf.id
JOIN scopes st ON mt.scope_id = st.id
WHERE %s
ORDER BY l.created_at DESC, l.id DESC
LIMIT ? OFFSET ?`, whereSQL)

	listArgs := append(append([]any(nil), args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list suggested links: %w", err)
	}
	defer rows.Close()

	items := make([]LinkWithContent, 0)
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
			return nil, 0, fmt.Errorf("store: scan suggested link: %w", err)
		}
		item.Suggested = (suggestedInt != 0)
		item.CreatedAt = time.UnixMicro(createdAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: iterate suggested links: %w", err)
	}

	return items, total, nil
}

// GetGraph returns active memory nodes and their relationship edges for the
// selected scope subtree, prioritizing connected nodes when capped by Limit.
// For non-global scopes, linked active memories outside the scope subtree are
// included as External nodes so cross-scope edges remain visible.
func (s *Store) GetGraph(ctx context.Context, q GraphQuery) (*GraphResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}

	var scopeIDs []int64
	scopedFilter := false
	if scopePath := strings.TrimSpace(q.ScopePath); scopePath != "" && scopePath != "global" {
		sc, err := scope.Parse(scopePath)
		if err != nil {
			return nil, fmt.Errorf("store: parse scope %q: %w", scopePath, err)
		}
		ids, err := s.ResolveScopeIDs(ctx, sc, false, true)
		if err != nil {
			return nil, fmt.Errorf("store: resolve scope %q: %w", scopePath, err)
		}
		if len(ids) == 0 {
			return &GraphResult{
				Nodes: []GraphNode{},
				Edges: []GraphEdge{},
			}, nil
		}
		scopeIDs = ids
		scopedFilter = true
	}

	whereParts := []string{"m.status = 'active'"}
	var scopeArgs []any
	if scopedFilter {
		placeholders := make([]string, len(scopeIDs))
		for i, id := range scopeIDs {
			placeholders[i] = "?"
			scopeArgs = append(scopeArgs, id)
		}
		whereParts = append(whereParts, fmt.Sprintf("m.scope_id IN (%s)", strings.Join(placeholders, ", ")))
	}
	whereSQL := strings.Join(whereParts, " AND ")

	var totalNodes int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM memories m WHERE %s", whereSQL)
	if err := s.db.QueryRowContext(ctx, countQuery, scopeArgs...).Scan(&totalNodes); err != nil {
		return nil, fmt.Errorf("store: count graph nodes: %w", err)
	}

	linkFilter := ""
	if !q.IncludeSuggested {
		linkFilter = " AND l.suggested = 0"
	}

	nodeQuery := fmt.Sprintf(`
SELECT m.id, s.path, m.type, m.content, COALESCE(m.key, ''), COALESCE(m.tags, ''),
       m.access_count, m.created_at,
       (SELECT COUNT(*) FROM memory_links l WHERE (l.from_id = m.id OR l.to_id = m.id)%s) AS deg
FROM memories m
JOIN scopes s ON s.id = m.scope_id
WHERE %s
ORDER BY (CASE WHEN deg > 0 THEN 1 ELSE 0 END) DESC, deg DESC, m.created_at DESC, m.id DESC
LIMIT ?`, linkFilter, whereSQL)

	nodeArgs := append(append([]any(nil), scopeArgs...), limit)
	rows, err := s.db.QueryContext(ctx, nodeQuery, nodeArgs...)
	if err != nil {
		return nil, fmt.Errorf("store: query graph nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]GraphNode, 0)
	nodeIndex := make(map[int64]int)
	inScopeIDs := make([]int64, 0)

	for rows.Next() {
		var n GraphNode
		var tagsStr string
		var createdAtMicros int64
		var deg int
		if err := rows.Scan(
			&n.ID, &n.Scope, &n.Type, &n.Content, &n.Key, &tagsStr,
			&n.AccessCount, &createdAtMicros, &deg,
		); err != nil {
			return nil, fmt.Errorf("store: scan graph node: %w", err)
		}
		n.Content = truncateRunes(n.Content, 500)
		n.Tags = splitTags(tagsStr)
		if n.Tags == nil {
			n.Tags = []string{}
		}
		n.CreatedAt = time.UnixMicro(createdAtMicros).Unix()
		nodeIndex[n.ID] = len(nodes)
		nodes = append(nodes, n)
		inScopeIDs = append(inScopeIDs, n.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate graph nodes: %w", err)
	}

	if len(inScopeIDs) == 0 {
		return &GraphResult{
			Nodes:      nodes,
			Edges:      []GraphEdge{},
			TotalNodes: totalNodes,
			Truncated:  totalNodes > limit,
		}, nil
	}

	idPlaceholders := make([]string, len(inScopeIDs))
	edgeArgs := make([]any, 0, len(inScopeIDs)*2)
	for i, id := range inScopeIDs {
		idPlaceholders[i] = "?"
		edgeArgs = append(edgeArgs, id)
	}
	for _, id := range inScopeIDs {
		edgeArgs = append(edgeArgs, id)
	}
	inClause := strings.Join(idPlaceholders, ", ")

	edgeQuery := fmt.Sprintf(`
SELECT l.id, l.from_id, l.to_id, l.relation, l.suggested, l.created_at
FROM memory_links l
WHERE (l.from_id IN (%s) OR l.to_id IN (%s))%s
ORDER BY l.created_at DESC, l.id DESC`, inClause, inClause, linkFilter)

	edgeRows, err := s.db.QueryContext(ctx, edgeQuery, edgeArgs...)
	if err != nil {
		return nil, fmt.Errorf("store: query graph edges: %w", err)
	}
	defer edgeRows.Close()

	rawEdges := make([]GraphEdge, 0)
	missingEndpoints := make(map[int64]bool)
	for edgeRows.Next() {
		var e GraphEdge
		var suggestedInt int
		var createdAtMicros int64
		if err := edgeRows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Relation, &suggestedInt, &createdAtMicros); err != nil {
			return nil, fmt.Errorf("store: scan graph edge: %w", err)
		}
		e.Suggested = (suggestedInt != 0)
		e.CreatedAt = time.UnixMicro(createdAtMicros).Unix()
		rawEdges = append(rawEdges, e)

		if _, ok := nodeIndex[e.FromID]; !ok {
			missingEndpoints[e.FromID] = true
		}
		if _, ok := nodeIndex[e.ToID]; !ok {
			missingEndpoints[e.ToID] = true
		}
	}
	if err := edgeRows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate graph edges: %w", err)
	}

	// For scoped queries, pull in active external-scope endpoints so cross-scope links stay visible.
	if scopedFilter && len(missingEndpoints) > 0 {
		extIDs := make([]int64, 0, len(missingEndpoints))
		for id := range missingEndpoints {
			extIDs = append(extIDs, id)
		}
		extPlaceholders := make([]string, len(extIDs))
		extArgs := make([]any, 0, len(extIDs)+len(scopeIDs))
		for i, id := range extIDs {
			extPlaceholders[i] = "?"
			extArgs = append(extArgs, id)
		}
		scopePlaceholders := make([]string, len(scopeIDs))
		for i, sid := range scopeIDs {
			scopePlaceholders[i] = "?"
			extArgs = append(extArgs, sid)
		}

		extQuery := fmt.Sprintf(`
SELECT m.id, s.path, m.type, m.content, COALESCE(m.key, ''), COALESCE(m.tags, ''),
       m.access_count, m.created_at
FROM memories m
JOIN scopes s ON s.id = m.scope_id
WHERE m.status = 'active'
  AND m.id IN (%s)
  AND m.scope_id NOT IN (%s)`, strings.Join(extPlaceholders, ", "), strings.Join(scopePlaceholders, ", "))

		extRows, err := s.db.QueryContext(ctx, extQuery, extArgs...)
		if err != nil {
			return nil, fmt.Errorf("store: query external graph nodes: %w", err)
		}
		defer extRows.Close()

		for extRows.Next() {
			var n GraphNode
			var tagsStr string
			var createdAtMicros int64
			if err := extRows.Scan(
				&n.ID, &n.Scope, &n.Type, &n.Content, &n.Key, &tagsStr,
				&n.AccessCount, &createdAtMicros,
			); err != nil {
				return nil, fmt.Errorf("store: scan external graph node: %w", err)
			}
			n.Content = truncateRunes(n.Content, 500)
			n.Tags = splitTags(tagsStr)
			if n.Tags == nil {
				n.Tags = []string{}
			}
			n.CreatedAt = time.UnixMicro(createdAtMicros).Unix()
			n.External = true
			nodeIndex[n.ID] = len(nodes)
			nodes = append(nodes, n)
		}
		if err := extRows.Err(); err != nil {
			return nil, fmt.Errorf("store: iterate external graph nodes: %w", err)
		}
	}

	// Keep only edges whose endpoints are both present in nodes, and compute exact visible degree.
	edges := make([]GraphEdge, 0, len(rawEdges))
	for _, e := range rawEdges {
		fromIdx, hasFrom := nodeIndex[e.FromID]
		toIdx, hasTo := nodeIndex[e.ToID]
		if !hasFrom || !hasTo {
			continue
		}
		edges = append(edges, e)
		nodes[fromIdx].Degree++
		nodes[toIdx].Degree++
	}

	return &GraphResult{
		Nodes:      nodes,
		Edges:      edges,
		TotalNodes: totalNodes,
		Truncated:  totalNodes > limit,
	}, nil
}

func truncateRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
