package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
)

// ValidMessageRoles defines the allowed values for message roles.
var ValidMessageRoles = []string{"user", "assistant", "system", "tool"}

// GenerateConversationID returns a random prefixed conversation identifier.
func GenerateConversationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "conv_" + hex.EncodeToString(b)
}

// CreateConversation inserts a new conversation thread.
func (s *Store) CreateConversation(ctx context.Context, c *Conversation) error {
	if c == nil {
		return fmt.Errorf("store: nil conversation")
	}
	if c.ID == "" {
		c.ID = GenerateConversationID()
	}
	if c.ScopeID == 0 {
		if c.ScopePath == "" {
			c.ScopePath = "global"
		}
		sc, err := scope.Parse(c.ScopePath)
		if err != nil {
			return fmt.Errorf("store: parse scope %q: %w", c.ScopePath, err)
		}
		scopeID, err := s.EnsureScope(ctx, sc)
		if err != nil {
			return fmt.Errorf("store: ensure scope %q: %w", c.ScopePath, err)
		}
		c.ScopeID = scopeID
	}
	nowMicros := nowMicro()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.UnixMicro(nowMicros)
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_conversations(id, scope_id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.ScopeID, c.Title, c.CreatedAt.UnixMicro(), c.UpdatedAt.UnixMicro())
	if err != nil {
		return fmt.Errorf("store: create conversation: %w", err)
	}
	return nil
}

// GetConversation retrieves a conversation thread by ID.
func (s *Store) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.scope_id, s.path, c.title, c.created_at, c.updated_at
		FROM agent_conversations c
		JOIN scopes s ON s.id = c.scope_id
		WHERE c.id = ?`, id)

	var c Conversation
	var createdAtMicros, updatedAtMicros int64
	err := row.Scan(&c.ID, &c.ScopeID, &c.ScopePath, &c.Title, &createdAtMicros, &updatedAtMicros)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get conversation: %w", err)
	}
	c.CreatedAt = time.UnixMicro(createdAtMicros)
	c.UpdatedAt = time.UnixMicro(updatedAtMicros)
	return &c, nil
}

// ListConversations retrieves conversation threads, optionally filtered by scope.
func (s *Store) ListConversations(ctx context.Context, scopeID int64, limit int) ([]Conversation, error) {
	var where []string
	var args []any

	if scopeID > 0 {
		where = append(where, "c.scope_id = ?")
		args = append(args, scopeID)
	}

	query := `
		SELECT c.id, c.scope_id, s.path, c.title, c.created_at, c.updated_at
		FROM agent_conversations c
		JOIN scopes s ON s.id = c.scope_id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY c.updated_at DESC"

	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		var createdAtMicros, updatedAtMicros int64
		if err := rows.Scan(&c.ID, &c.ScopeID, &c.ScopePath, &c.Title, &createdAtMicros, &updatedAtMicros); err != nil {
			return nil, fmt.Errorf("store: scan conversation: %w", err)
		}
		c.CreatedAt = time.UnixMicro(createdAtMicros)
		c.UpdatedAt = time.UnixMicro(updatedAtMicros)
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

// DeleteConversation removes a conversation thread and cascades its messages.
func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agent_conversations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete conversation: %w", err)
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

// AppendMessage appends a dialog message to an existing conversation thread and updates thread updated_at.
func (s *Store) AppendMessage(ctx context.Context, m *Message) (int64, error) {
	if m == nil {
		return 0, fmt.Errorf("store: nil message")
	}
	if m.ConversationID == "" {
		return 0, fmt.Errorf("store: empty conversation_id")
	}
	if !slices.Contains(ValidMessageRoles, m.Role) {
		return 0, fmt.Errorf("store: invalid message role %q; must be one of: %s",
			m.Role, strings.Join(ValidMessageRoles, ", "))
	}
	nowMicros := nowMicro()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.UnixMicro(nowMicros)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Verify conversation exists
	var dummy string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM agent_conversations WHERE id = ?", m.ConversationID).Scan(&dummy); err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("store: conversation %q not found: %w", m.ConversationID, ErrNotFound)
		}
		return 0, fmt.Errorf("store: check conversation: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO agent_messages(conversation_id, role, content, citations_json, tool_calls_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.ConversationID, m.Role, m.Content, m.CitationsJSON, m.ToolCallsJSON, m.CreatedAt.UnixMicro())
	if err != nil {
		return 0, fmt.Errorf("store: insert message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("store: last insert id: %w", err)
	}
	m.ID = id

	// Update conversation updated_at
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_conversations SET updated_at = ? WHERE id = ?`,
		nowMicros, m.ConversationID); err != nil {
		return 0, fmt.Errorf("store: update conversation updated_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store: commit message: %w", err)
	}
	return id, nil
}

// GetConversationMessages retrieves all messages in a thread in chronological order.
func (s *Store) GetConversationMessages(ctx context.Context, conversationID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, conversation_id, role, content, citations_json, tool_calls_json, created_at
		FROM agent_messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC, id ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: get conversation messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var citationsJSON, toolCallsJSON sql.NullString
		var createdAtMicros int64
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &citationsJSON, &toolCallsJSON, &createdAtMicros); err != nil {
			return nil, fmt.Errorf("store: scan message: %w", err)
		}
		if citationsJSON.Valid {
			m.CitationsJSON = citationsJSON.String
		}
		if toolCallsJSON.Valid {
			m.ToolCallsJSON = toolCallsJSON.String
		}
		m.CreatedAt = time.UnixMicro(createdAtMicros)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
