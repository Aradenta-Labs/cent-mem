package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
)

// ListTagStats returns each distinct tag, active memory count, and last-used timestamp
// across active memories matching scopeIDs.
// Passing nil scopeIDs queries all scopes; passing a non-nil empty slice returns no tags.
func (s *Store) ListTagStats(ctx context.Context, scopeIDs []int64) ([]TagStat, error) {
	if scopeIDs != nil && len(scopeIDs) == 0 {
		return []TagStat{}, nil
	}

	query := `SELECT tags, updated_at, created_at FROM memories WHERE status = 'active' AND tags IS NOT NULL AND tags != ''`
	var args []any
	if len(scopeIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(scopeIDs)), ",")
		query += " AND scope_id IN (" + placeholders + ")"
		for _, id := range scopeIDs {
			args = append(args, id)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list tag stats: %w", err)
	}
	defer rows.Close()

	type agg struct {
		count    int
		lastUsed int64
	}
	byTag := make(map[string]*agg)

	for rows.Next() {
		var rawTags string
		var updatedAt, createdAt int64
		if err := rows.Scan(&rawTags, &updatedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("store: scan tag stats: %w", err)
		}
		ts := updatedAt
		if createdAt > ts {
			ts = createdAt
		}
		seenInRow := make(map[string]bool)
		for _, tag := range splitTags(rawTags) {
			if seenInRow[tag] {
				continue
			}
			seenInRow[tag] = true
			a, ok := byTag[tag]
			if !ok {
				a = &agg{lastUsed: ts}
				byTag[tag] = a
			}
			a.count++
			if ts > a.lastUsed {
				a.lastUsed = ts
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate tag stats: %w", err)
	}

	out := make([]TagStat, 0, len(byTag))
	for tag, a := range byTag {
		out = append(out, TagStat{
			Tag:      tag,
			Count:    a.count,
			LastUsed: time.UnixMicro(a.lastUsed).UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out, nil
}

// CountMemoriesWithAnyTag returns the number of distinct active memories in scopeIDs
// that have at least one tag in fromTags (excluding toTag if provided).
func (s *Store) CountMemoriesWithAnyTag(ctx context.Context, scopeIDs []int64, fromTags []string, toTag string) (int, error) {
	if scopeIDs != nil && len(scopeIDs) == 0 {
		return 0, nil
	}
	matcher := newTagRenameMatcher(fromTags, toTag)
	if matcher.empty() {
		return 0, nil
	}

	query := `SELECT tags FROM memories WHERE status = 'active' AND tags IS NOT NULL AND tags != ''`
	var args []any
	if len(scopeIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(scopeIDs)), ",")
		query += " AND scope_id IN (" + placeholders + ")"
		for _, id := range scopeIDs {
			args = append(args, id)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("store: count memories with tags: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var rawTags string
		if err := rows.Scan(&rawTags); err != nil {
			return 0, fmt.Errorf("store: scan tags for count: %w", err)
		}
		for _, t := range splitTags(rawTags) {
			if matcher.matches(t) {
				count++
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("store: iterate tags for count: %w", err)
	}
	return count, nil
}

// BulkRenameTag replaces all occurrences of fromTags with toTag across active memories
// in scopeIDs inside a single transaction, deduplicating resulting tag arrays and preserving updated_at.
// Returns the number of memories updated.
func (s *Store) BulkRenameTag(ctx context.Context, scopeIDs []int64, fromTags []string, toTag string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: begin tx for bulk rename tag: %w", err)
	}
	defer tx.Rollback()

	affected, err := bulkRenameTagTx(ctx, tx, scopeIDs, fromTags, toTag)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store: commit bulk rename tag: %w", err)
	}
	return affected, nil
}

func bulkRenameTagTx(ctx context.Context, tx *sql.Tx, scopeIDs []int64, fromTags []string, toTag string) (int, error) {
	normTo := strings.TrimSpace(strings.ToLower(toTag))
	if normTo == "" {
		return 0, fmt.Errorf("store: target tag cannot be empty")
	}
	if strings.Contains(normTo, ",") {
		return 0, fmt.Errorf("store: target tag %q cannot contain commas", toTag)
	}
	if scopeIDs != nil && len(scopeIDs) == 0 {
		return 0, nil
	}

	matcher := newTagRenameMatcher(fromTags, toTag)
	if matcher.empty() {
		return 0, nil
	}

	query := `
		SELECT m.id, m.tags, s.path
		FROM memories m
		JOIN scopes s ON s.id = m.scope_id
		WHERE m.status = 'active' AND m.tags IS NOT NULL AND m.tags != ''`
	var args []any
	if len(scopeIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(scopeIDs)), ",")
		query += " AND m.scope_id IN (" + placeholders + ")"
		for _, id := range scopeIDs {
			args = append(args, id)
		}
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("store: query memories for tag rename: %w", err)
	}

	type candidate struct {
		id        int64
		newTags   string
		scopePath string
	}
	var updates []candidate
	for rows.Next() {
		var id int64
		var rawTags, scopePath string
		if err := rows.Scan(&id, &rawTags, &scopePath); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: scan memory for tag rename: %w", err)
		}
		orig := splitTags(rawTags)
		matched := false
		replaced := make([]string, 0, len(orig))
		for _, t := range orig {
			if matcher.matches(t) {
				matched = true
				replaced = append(replaced, normTo)
			} else {
				replaced = append(replaced, t)
			}
		}
		if !matched {
			continue
		}
		newJoined := joinTags(replaced)
		if newJoined == rawTags {
			continue
		}
		updates = append(updates, candidate{
			id:        id,
			newTags:   newJoined,
			scopePath: scopePath,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("store: iterate memories for tag rename: %w", err)
	}
	rows.Close()

	if len(updates) == 0 {
		return 0, nil
	}

	fromList := matcher.sourceList()

	now := nowMicro()
	for _, u := range updates {
		// Deliberately preserve updated_at so tag canonicalization does not skew recency decay.
		if _, err := tx.ExecContext(ctx, `UPDATE memories SET tags = ? WHERE id = ?`, u.newTags, u.id); err != nil {
			return 0, fmt.Errorf("store: update tags for memory %d: %w", u.id, err)
		}
		evPayload, _ := json.Marshal(map[string]any{
			"memory_id": u.id,
			"op":        "tag_merge",
			"from_tags": fromList,
			"to_tag":    normTo,
		})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'update', ?, ?, ?)`,
			u.id, u.scopePath, string(evPayload), now); err != nil {
			return 0, fmt.Errorf("store: insert tag_merge event for memory %d: %w", u.id, err)
		}
	}

	return len(updates), nil
}

type tagRenameMatcher struct {
	normTo              string
	exactFrom           map[string]bool
	lowerFrom           map[string]bool
	normalizeTargetCase bool
}

func newTagRenameMatcher(fromTags []string, toTag string) tagRenameMatcher {
	trimmedTo := strings.TrimSpace(toTag)
	normTo := strings.ToLower(trimmedTo)
	m := tagRenameMatcher{
		normTo:              normTo,
		exactFrom:           make(map[string]bool, len(fromTags)),
		lowerFrom:           make(map[string]bool, len(fromTags)),
		normalizeTargetCase: trimmedTo != "" && trimmedTo != normTo,
	}
	for _, raw := range fromTags {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || trimmed == normTo {
			continue
		}
		m.exactFrom[trimmed] = true
		lower := strings.ToLower(trimmed)
		if lower != normTo {
			m.lowerFrom[lower] = true
		}
	}
	return m
}

func (m tagRenameMatcher) empty() bool {
	return len(m.exactFrom) == 0 && len(m.lowerFrom) == 0 && !m.normalizeTargetCase
}

func (m tagRenameMatcher) matches(tag string) bool {
	if tag == "" || tag == m.normTo {
		return false
	}
	if m.exactFrom[tag] {
		return true
	}
	lower := strings.ToLower(tag)
	if m.lowerFrom[lower] {
		return true
	}
	return m.normalizeTargetCase && lower == m.normTo
}

func (m tagRenameMatcher) sourceList() []string {
	out := make([]string, 0, len(m.exactFrom))
	for t := range m.exactFrom {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// resolveSubtreeScopeIDsTx resolves scopePath and all its descendants inside an existing transaction.
func resolveSubtreeScopeIDsTx(ctx context.Context, tx *sql.Tx, scopePath string) ([]int64, error) {
	if scopePath == "" {
		scopePath = "global"
	}
	sc, err := scope.Parse(scopePath)
	if err != nil {
		return nil, fmt.Errorf("store: parse scope %q: %w", scopePath, err)
	}

	set := map[int64]bool{}
	var ids []int64

	var rootID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM scopes WHERE path = ?`, sc.Path).Scan(&rootID); err == nil {
		set[rootID] = true
		ids = append(ids, rootID)
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	if sc.Kind != scope.Session {
		var rows *sql.Rows
		if sc.Kind == scope.Global {
			rows, err = tx.QueryContext(ctx, `SELECT id FROM scopes WHERE path != 'global'`)
		} else {
			rows, err = tx.QueryContext(ctx, `SELECT id FROM scopes WHERE path LIKE ? AND path != ?`, scope.DescendantPrefix(sc), sc.Path)
		}
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if !set[id] {
				set[id] = true
				ids = append(ids, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	if ids == nil {
		ids = []int64{}
	}
	return ids, nil
}
