package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
// ErrProposalConflict indicates a proposal state transition or dependency conflict.
var ErrProposalConflict = errors.New("proposal conflict")

func (s *Store) ListProposals(ctx context.Context, q ProposalListQuery) ([]Proposal, error) {
	var where []string
	var args []any

	if q.ScopeID > 0 {
		where = append(where, "p.scope_id = ?")
		args = append(args, q.ScopeID)
	} else if q.ScopePath != "" {
		sc, err := scope.Parse(q.ScopePath)
		if err != nil {
			return nil, fmt.Errorf("store: parse scope %q: %w", q.ScopePath, err)
		}
		var sid int64
		if err := s.db.QueryRowContext(ctx, "SELECT id FROM scopes WHERE path = ?", sc.Path).Scan(&sid); err == nil {
			where = append(where, "p.scope_id = ?")
			args = append(args, sid)
		} else {
			where = append(where, "s.path = ?")
			args = append(args, sc.Path)
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRowContext(ctx, "SELECT status FROM agent_proposals WHERE id = ?", id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: query proposal status: %w", err)
	}
	if currentStatus == "applied" && status != "applied" {
		return fmt.Errorf("%w: proposal %d already applied (cannot transition to %q)", ErrProposalConflict, id, status)
	}

	var appliedAt *int64
	if status == "applied" {
		t := nowMicro()
		appliedAt = &t
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE agent_proposals
		SET status = ?, applied_at = ?
		WHERE id = ? AND (status != 'applied' OR ? = 'applied')`, status, appliedAt, id, status)
	if err != nil {
		return fmt.Errorf("store: update proposal status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("%w: proposal %d already applied (cannot transition to %q)", ErrProposalConflict, id, status)
	}
	return tx.Commit()
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
		return fmt.Errorf("%w: proposal %d already applied", ErrProposalConflict, id)
	}
	if p.Status != "pending" {
		return fmt.Errorf("%w: proposal %d has status %q (cannot apply)", ErrProposalConflict, id, p.Status)
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
			return fmt.Errorf("%w: source memory %d not found: %w", ErrProposalConflict, payload.FromID, err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM memories WHERE id = ?", payload.ToID).Scan(&dummy); err != nil {
			return fmt.Errorf("%w: target memory %d not found: %w", ErrProposalConflict, payload.ToID, err)
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

		// Deduplicate source IDs
		seen := make(map[int64]bool)
		var uniqueSourceIDs []int64
		for _, sid := range payload.SourceIDs {
			if sid <= 0 {
				return fmt.Errorf("store: invalid merge source id %d", sid)
			}
			if !seen[sid] {
				seen[sid] = true
				uniqueSourceIDs = append(uniqueSourceIDs, sid)
			}
		}

		tagSet := make(map[string]bool)
		for _, t := range payload.TargetTags {
			if tr := strings.TrimSpace(t); tr != "" {
				tagSet[tr] = true
			}
		}
		primaryScopeID := p.ScopeID
		primaryScopePath := p.ScopePath

		for _, sid := range uniqueSourceIDs {
			var memID, memScopeID int64
			var memScopePath, memTags, memStatus string
			err := tx.QueryRowContext(ctx, `
				SELECT m.id, m.scope_id, s.path, m.tags, m.status
				FROM memories m
				JOIN scopes s ON s.id = m.scope_id
				WHERE m.id = ?`, sid).Scan(&memID, &memScopeID, &memScopePath, &memTags, &memStatus)
			if err != nil {
				return fmt.Errorf("%w: merge source memory %d: %w", ErrProposalConflict, sid, err)
			}
			if memStatus != "active" {
				return fmt.Errorf("%w: cannot merge non-active source memory %d (status: %q)", ErrProposalConflict, sid, memStatus)
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

		evMergedPayload, _ := json.Marshal(map[string]any{
			"memory_id":  mergedID,
			"source_ids": uniqueSourceIDs,
			"op":         "insert",
		})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'insert', ?, ?, ?)`,
			mergedID, primaryScopePath, string(evMergedPayload), nowMicros); err != nil {
			return fmt.Errorf("store: insert merged event: %w", err)
		}

		for _, sid := range uniqueSourceIDs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE memories SET status = 'summarized', updated_at = ? WHERE id = ?`,
				nowMicros, sid); err != nil {
				return fmt.Errorf("store: summarize source memory %d: %w", sid, err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE memory_id = ?`, sid); err != nil {
				return fmt.Errorf("store: delete embeddings for source %d: %w", sid, err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM memories_vec WHERE memory_id = ?`, sid); err != nil {
				return fmt.Errorf("store: delete vector for source %d: %w", sid, err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM embed_queue WHERE memory_id = ?`, sid); err != nil {
				return fmt.Errorf("store: delete embed_queue for source %d: %w", sid, err)
			}

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
				"op":          "summarize",
			})
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
				VALUES (?, 'summarize', ?, ?, ?)`,
				sid, primaryScopePath, string(evPayload), nowMicros); err != nil {
				return fmt.Errorf("store: summarize event: %w", err)
			}
		}

	case "update":
		var payload UpdateProposalPayload
		if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("store: invalid update payload: %w", err)
		}
		if payload.TargetID <= 0 {
			return fmt.Errorf("store: invalid update target_id %d", payload.TargetID)
		}
		var memType, memScopePath, memStatus string
		err := tx.QueryRowContext(ctx, `
			SELECT m.type, s.path, m.status FROM memories m JOIN scopes s ON s.id = m.scope_id WHERE m.id = ?`,
			payload.TargetID).Scan(&memType, &memScopePath, &memStatus)
		if err != nil {
			return fmt.Errorf("store: update target memory %d: %w", payload.TargetID, err)
		}
		if memStatus != "active" {
			return fmt.Errorf("store: cannot update non-active memory %d (status: %q)", payload.TargetID, memStatus)
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

		if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE memory_id = ?`, payload.TargetID); err != nil {
			return fmt.Errorf("store: delete embeddings for update: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memories_vec WHERE memory_id = ?`, payload.TargetID); err != nil {
			return fmt.Errorf("store: delete vector for update: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM embed_queue WHERE memory_id = ?`, payload.TargetID); err != nil {
			return fmt.Errorf("store: delete embed_queue for update: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO embed_queue(memory_id, priority, created_at) VALUES (?, 1, ?)`,
			payload.TargetID, nowMicros); err != nil {
			return fmt.Errorf("store: enqueue update embedding: %w", err)
		}

		evPayload, _ := json.Marshal(map[string]any{
			"memory_id": payload.TargetID,
			"op":        "update",
		})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'update', ?, ?, ?)`,
			payload.TargetID, memScopePath, string(evPayload), nowMicros); err != nil {
			return fmt.Errorf("store: update event: %w", err)
		}

	case "archive":
		var payload ArchiveProposalPayload
		if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("store: invalid archive payload: %w", err)
		}
		if payload.TargetID <= 0 {
			return fmt.Errorf("store: invalid archive target_id %d", payload.TargetID)
		}
		var memScopePath, memStatus string
		err := tx.QueryRowContext(ctx, `
			SELECT s.path, m.status FROM memories m JOIN scopes s ON s.id = m.scope_id WHERE m.id = ?`,
			payload.TargetID).Scan(&memScopePath, &memStatus)
		if err != nil {
			return fmt.Errorf("store: archive target memory %d: %w", payload.TargetID, err)
		}
		if memStatus == "archived" {
			return fmt.Errorf("store: memory %d is already archived", payload.TargetID)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE memories SET status = 'archived', updated_at = ? WHERE id = ?`,
			nowMicros, payload.TargetID); err != nil {
			return fmt.Errorf("store: archive memory: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE memory_id = ?`, payload.TargetID); err != nil {
			return fmt.Errorf("store: delete embeddings for archive: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memories_vec WHERE memory_id = ?`, payload.TargetID); err != nil {
			return fmt.Errorf("store: delete vector for archive: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM embed_queue WHERE memory_id = ?`, payload.TargetID); err != nil {
			return fmt.Errorf("store: delete embed_queue for archive: %w", err)
		}

		evPayload, _ := json.Marshal(map[string]any{
			"memory_id": payload.TargetID,
			"reason":    payload.Reason,
			"op":        "archive",
		})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'archive', ?, ?, ?)`,
			payload.TargetID, memScopePath, string(evPayload), nowMicros); err != nil {
			return fmt.Errorf("store: archive event: %w", err)
		}

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
