package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
)

// CreateProposal inserts a new agent proposal in status 'pending'.
func (s *Store) CreateProposal(ctx context.Context, p *Proposal) (int64, error) {
	if p == nil {
		return 0, fmt.Errorf("store: nil proposal")
	}
	if !slices.Contains(ValidProposalTypes, p.ProposalType) {
		return 0, fmt.Errorf("store: invalid proposal_type %q; must be one of: %s",
			p.ProposalType, strings.Join(ValidProposalTypes, ", "))
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	if !slices.Contains(ValidProposalStatuses, p.Status) {
		return 0, fmt.Errorf("store: invalid proposal status %q; must be one of: %s",
			p.Status, strings.Join(ValidProposalStatuses, ", "))
	}
	if p.ScopeID == 0 {
		if p.ScopePath == "" {
			p.ScopePath = "global"
		}
		sc, err := scope.Parse(p.ScopePath)
		if err != nil {
			return 0, fmt.Errorf("store: parse scope %q: %w", p.ScopePath, err)
		}
		scopeID, err := s.EnsureScope(ctx, sc)
		if err != nil {
			return 0, fmt.Errorf("store: ensure scope %q: %w", p.ScopePath, err)
		}
		p.ScopeID = scopeID
	}
	nowMicros := nowMicro()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.UnixMicro(nowMicros)
	}
	var appliedAtVal *int64
	if p.AppliedAt != nil {
		t := p.AppliedAt.UnixMicro()
		appliedAtVal = &t
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_proposals(scope_id, proposal_type, status, title, reasoning, payload_json, created_at, applied_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ScopeID, p.ProposalType, p.Status, p.Title, p.Reasoning, p.PayloadJSON, p.CreatedAt.UnixMicro(), appliedAtVal)
	if err != nil {
		return 0, fmt.Errorf("store: create proposal: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("store: last insert id: %w", err)
	}
	p.ID = id
	return id, nil
}

// GetProposal retrieves a proposal by ID. Returns ErrNotFound if missing.
func (s *Store) GetProposal(ctx context.Context, id int64) (*Proposal, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.scope_id, s.path, p.proposal_type, p.status, p.title, p.reasoning,
		       p.payload_json, p.created_at, p.applied_at
		FROM agent_proposals p
		JOIN scopes s ON s.id = p.scope_id
		WHERE p.id = ?`, id)

	p := &Proposal{}
	var createdAtMicros int64
	var appliedAtMicros sql.NullInt64
	err := row.Scan(&p.ID, &p.ScopeID, &p.ScopePath, &p.ProposalType, &p.Status,
		&p.Title, &p.Reasoning, &p.PayloadJSON, &createdAtMicros, &appliedAtMicros)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get proposal: %w", err)
	}
	p.CreatedAt = time.UnixMicro(createdAtMicros)
	if appliedAtMicros.Valid {
		t := time.UnixMicro(appliedAtMicros.Int64)
		p.AppliedAt = &t
	}
	return p, nil
}

// ListProposals queries agent proposals with filtering and pagination.
func (s *Store) ListProposals(ctx context.Context, q ProposalListQuery) ([]Proposal, error) {
	var where []string
	var args []any

	if q.ScopeID > 0 {
		where = append(where, "p.scope_id = ?")
		args = append(args, q.ScopeID)
	} else if q.ScopePath != "" {
		sc, err := scope.Parse(q.ScopePath)
		if err == nil {
			var sid int64
			if err := s.db.QueryRowContext(ctx, "SELECT id FROM scopes WHERE path = ?", sc.Path).Scan(&sid); err == nil {
				where = append(where, "p.scope_id = ?")
				args = append(args, sid)
			} else {
				where = append(where, "s.path = ?")
				args = append(args, sc.Path)
			}
		}
	}
	if q.Status != "" {
		where = append(where, "p.status = ?")
		args = append(args, q.Status)
	}
	if q.ProposalType != "" {
		where = append(where, "p.proposal_type = ?")
		args = append(args, q.ProposalType)
	}

	query := `
		SELECT p.id, p.scope_id, s.path, p.proposal_type, p.status, p.title, p.reasoning,
		       p.payload_json, p.created_at, p.applied_at
		FROM agent_proposals p
		JOIN scopes s ON s.id = p.scope_id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY p.created_at DESC"

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, q.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list proposals: %w", err)
	}
	defer rows.Close()

	var proposals []Proposal
	for rows.Next() {
		var p Proposal
		var createdAtMicros int64
		var appliedAtMicros sql.NullInt64
		if err := rows.Scan(&p.ID, &p.ScopeID, &p.ScopePath, &p.ProposalType, &p.Status,
			&p.Title, &p.Reasoning, &p.PayloadJSON, &createdAtMicros, &appliedAtMicros); err != nil {
			return nil, fmt.Errorf("store: scan proposal: %w", err)
		}
		p.CreatedAt = time.UnixMicro(createdAtMicros)
		if appliedAtMicros.Valid {
			t := time.UnixMicro(appliedAtMicros.Int64)
			p.AppliedAt = &t
		}
		proposals = append(proposals, p)
	}
	return proposals, rows.Err()
}

// UpdateProposalStatus updates the status of a proposal.
func (s *Store) UpdateProposalStatus(ctx context.Context, id int64, status string) error {
	if !slices.Contains(ValidProposalStatuses, status) {
		return fmt.Errorf("store: invalid proposal status %q; must be one of: %s",
			status, strings.Join(ValidProposalStatuses, ", "))
	}
	var appliedAt *int64
	if status == "applied" {
		t := nowMicro()
		appliedAt = &t
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE agent_proposals
		SET status = ?, applied_at = ?
		WHERE id = ?`, status, appliedAt, id)
	if err != nil {
		return fmt.Errorf("store: update proposal status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// DismissProposal marks a proposal as dismissed.
func (s *Store) DismissProposal(ctx context.Context, id int64) error {
	return s.UpdateProposalStatus(ctx, id, "dismissed")
}

// ApplyProposal atomically applies the changes proposed in a pending proposal.
func (s *Store) ApplyProposal(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	var p Proposal
	var createdAtMicros int64
	var appliedAtMicros sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT p.id, p.scope_id, s.path, p.proposal_type, p.status, p.title, p.reasoning,
		       p.payload_json, p.created_at, p.applied_at
		FROM agent_proposals p
		JOIN scopes s ON s.id = p.scope_id
		WHERE p.id = ?`, id).Scan(
		&p.ID, &p.ScopeID, &p.ScopePath, &p.ProposalType, &p.Status,
		&p.Title, &p.Reasoning, &p.PayloadJSON, &createdAtMicros, &appliedAtMicros)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: query proposal: %w", err)
	}
	if p.Status == "applied" {
		return fmt.Errorf("store: proposal %d already applied", id)
	}
	if p.Status != "pending" {
		return fmt.Errorf("store: proposal %d has status %q (cannot apply)", id, p.Status)
	}

	nowMicros := nowMicro()

	switch p.ProposalType {
	case "link":
		var payload LinkProposalPayload
		if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("store: invalid link payload: %w", err)
		}
		if payload.FromID <= 0 || payload.ToID <= 0 || payload.FromID == payload.ToID {
			return fmt.Errorf("store: invalid link ids (from=%d, to=%d)", payload.FromID, payload.ToID)
		}
		if !IsValidLinkRelation(payload.Relation) {
			return fmt.Errorf("store: invalid link relation %q", payload.Relation)
		}
		var dummy int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM memories WHERE id = ?", payload.FromID).Scan(&dummy); err != nil {
			return fmt.Errorf("store: source memory %d not found: %w", payload.FromID, err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM memories WHERE id = ?", payload.ToID).Scan(&dummy); err != nil {
			return fmt.Errorf("store: target memory %d not found: %w", payload.ToID, err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO memory_links(from_id, to_id, relation, created_at, suggested)
			VALUES (?, ?, ?, ?, 0)
			ON CONFLICT(from_id, to_id, relation) DO UPDATE SET suggested = 0`,
			payload.FromID, payload.ToID, payload.Relation, nowMicros)
		if err != nil {
			return fmt.Errorf("store: apply link: %w", err)
		}

	case "merge":
		var payload MergeProposalPayload
		if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("store: invalid merge payload: %w", err)
		}
		if len(payload.SourceIDs) == 0 {
			return fmt.Errorf("store: merge requires at least one source id")
		}
		if strings.TrimSpace(payload.TargetContent) == "" {
			return fmt.Errorf("store: merge target_content cannot be empty")
		}

		tagSet := make(map[string]bool)
		for _, t := range payload.TargetTags {
			if tr := strings.TrimSpace(t); tr != "" {
				tagSet[tr] = true
			}
		}
		primaryScopeID := p.ScopeID
		primaryScopePath := p.ScopePath

		for _, sid := range payload.SourceIDs {
			var memID, memScopeID int64
			var memScopePath, memTags, memStatus string
			err := tx.QueryRowContext(ctx, `
				SELECT m.id, m.scope_id, s.path, m.tags, m.status
				FROM memories m
				JOIN scopes s ON s.id = m.scope_id
				WHERE m.id = ?`, sid).Scan(&memID, &memScopeID, &memScopePath, &memTags, &memStatus)
			if err != nil {
				return fmt.Errorf("store: merge source memory %d: %w", sid, err)
			}
			if primaryScopeID == 0 {
				primaryScopeID = memScopeID
				primaryScopePath = memScopePath
			}
			for _, t := range splitTags(memTags) {
				tagSet[t] = true
			}
		}

		var unionTags []string
		for t := range tagSet {
			unionTags = append(unionTags, t)
		}
		sort.Strings(unionTags)

		contentHash := hashContent(primaryScopePath, "note", "", payload.TargetContent)
		res, err := tx.ExecContext(ctx, `
			INSERT INTO memories(
				scope_id, type, content, key, value_json, tags,
				source_agent, source_session, content_hash, status, summarize_at,
				created_at, updated_at
			) VALUES (?, 'note', ?, NULL, NULL, ?, '', '', ?, 'active', NULL, ?, ?)`,
			primaryScopeID, payload.TargetContent, joinTags(unionTags), contentHash, nowMicros, nowMicros)
		if err != nil {
			return fmt.Errorf("store: insert merged memory: %w", err)
		}
		mergedID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("store: merged memory id: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO embed_queue(memory_id, priority, created_at) VALUES (?, 0, ?)`,
			mergedID, nowMicros); err != nil {
			return fmt.Errorf("store: enqueue merged embedding: %w", err)
		}

		for _, sid := range payload.SourceIDs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE memories SET status = 'summarized', updated_at = ? WHERE id = ?`,
				nowMicros, sid); err != nil {
				return fmt.Errorf("store: summarize source memory %d: %w", sid, err)
			}
			_, _ = tx.ExecContext(ctx, `DELETE FROM embeddings WHERE memory_id = ?`, sid)
			_, _ = tx.ExecContext(ctx, `DELETE FROM memories_vec WHERE memory_id = ?`, sid)
			_, _ = tx.ExecContext(ctx, `DELETE FROM embed_queue WHERE memory_id = ?`, sid)

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO memory_links(from_id, to_id, relation, created_at, suggested)
				VALUES (?, ?, 'supersedes', ?, 0)
				ON CONFLICT(from_id, to_id, relation) DO UPDATE SET suggested = 0`,
				mergedID, sid, nowMicros); err != nil {
				return fmt.Errorf("store: link supersedes %d -> %d: %w", mergedID, sid, err)
			}

			evPayload, _ := json.Marshal(map[string]any{
				"memory_id":   sid,
				"merged_into": mergedID,
				"op":          "merge",
			})
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
				VALUES (?, 'merge', ?, ?, ?)`,
				sid, primaryScopePath, string(evPayload), nowMicros)
		}

	case "update":
		var payload UpdateProposalPayload
		if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("store: invalid update payload: %w", err)
		}
		if payload.TargetID <= 0 {
			return fmt.Errorf("store: invalid update target_id %d", payload.TargetID)
		}
		var memType, memScopePath string
		err := tx.QueryRowContext(ctx, `
			SELECT m.type, s.path FROM memories m JOIN scopes s ON s.id = m.scope_id WHERE m.id = ?`,
			payload.TargetID).Scan(&memType, &memScopePath)
		if err != nil {
			return fmt.Errorf("store: update target memory %d: %w", payload.TargetID, err)
		}
		contentHash := hashContent(memScopePath, memType, "", payload.Content)
		tagsStr := joinTags(payload.Tags)
		if _, err := tx.ExecContext(ctx, `
			UPDATE memories
			SET content = ?, tags = ?, content_hash = ?, updated_at = ?
			WHERE id = ?`,
			payload.Content, tagsStr, contentHash, nowMicros, payload.TargetID); err != nil {
			return fmt.Errorf("store: update memory: %w", err)
		}
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO embed_queue(memory_id, priority, created_at) VALUES (?, 1, ?)`,
			payload.TargetID, nowMicros)
		evPayload, _ := json.Marshal(map[string]any{
			"memory_id": payload.TargetID,
			"op":        "update",
		})
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'update', ?, ?, ?)`,
			payload.TargetID, memScopePath, string(evPayload), nowMicros)

	case "archive":
		var payload ArchiveProposalPayload
		if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("store: invalid archive payload: %w", err)
		}
		if payload.TargetID <= 0 {
			return fmt.Errorf("store: invalid archive target_id %d", payload.TargetID)
		}
		var memScopePath string
		err := tx.QueryRowContext(ctx, `
			SELECT s.path FROM memories m JOIN scopes s ON s.id = m.scope_id WHERE m.id = ?`,
			payload.TargetID).Scan(&memScopePath)
		if err != nil {
			return fmt.Errorf("store: archive target memory %d: %w", payload.TargetID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE memories SET status = 'archived', updated_at = ? WHERE id = ?`,
			nowMicros, payload.TargetID); err != nil {
			return fmt.Errorf("store: archive memory: %w", err)
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM embeddings WHERE memory_id = ?`, payload.TargetID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM memories_vec WHERE memory_id = ?`, payload.TargetID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM embed_queue WHERE memory_id = ?`, payload.TargetID)
		evPayload, _ := json.Marshal(map[string]any{
			"memory_id": payload.TargetID,
			"reason":    payload.Reason,
			"op":        "archive",
		})
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'archive', ?, ?, ?)`,
			payload.TargetID, memScopePath, string(evPayload), nowMicros)

	default:
		return fmt.Errorf("store: unsupported proposal type %q", p.ProposalType)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_proposals SET status = 'applied', applied_at = ? WHERE id = ?`,
		nowMicros, id); err != nil {
		return fmt.Errorf("store: update proposal applied status: %w", err)
	}

	return tx.Commit()
}
