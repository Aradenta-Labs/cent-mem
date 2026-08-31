// Package search implements hybrid retrieval (semantic, FTS5 keyword, facts, timeline).
//
// Phase 1 (M1) implements the non-semantic parts: FTS5 keyword ranking, fact
// key-prefix lookup, and timeline ordering, fused via Reciprocal Rank Fusion.
// The semantic ranker is a stub returning empty results and is completed in M2.
package search

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"time"

	"github.com/farras/cent-mem/internal/embed"
	"github.com/farras/cent-mem/internal/scope"
	"github.com/farras/cent-mem/internal/store"
)

// rrfK is the smoothing constant used by Reciprocal Rank Fusion.
const rrfK = 60

// Query captures a full search request. It is reused by both M1 (non-semantic)
// and M2 (semantic) rankers.
type Query struct {
	Text     string
	Scope    string
	Inherit  bool
	Children bool
	Top      int
	Type     string
	Tags     []string
	Since    time.Time
	Until    time.Time
	Agent    string
}

// Ranked is a single search result.
type Ranked struct {
	ID          int64
	Type        string
	Scope       string
	ScopeID     int64
	Content     string
	Tags        []string
	SourceAgent string
	CreatedAt   time.Time
	Score       float64
	MatchedBy   []string
}

// Searcher executes hybrid search against a Store.
type Searcher struct {
	store *store.Store
	emb   embed.Embedder
}

// New builds a Searcher backed by s. It uses an offline StubEmbedder by
// default; call WithEmbedder to attach a real ONNX embedder.
func New(s *store.Store) *Searcher {
	return &Searcher{store: s, emb: embed.NewStub(384)}
}

// WithEmbedder returns a Searcher that uses e for semantic queries. The
// embedder is wrapped in an LRU cache keyed by query text. It returns the
// receiver for chaining.
func (s *Searcher) WithEmbedder(e embed.Embedder) *Searcher {
	if e != nil {
		s.emb = embed.NewCachingEmbedder(e, 256)
	}
	return s
}

// db exposes the underlying SQL handle.
func (s *Searcher) db() *sql.DB { return s.store.DB() }

// Keyword ranks memories by FTS5 bm25 over content + tags.
func (s *Searcher) Keyword(ctx context.Context, q Query, top int) ([]Ranked, error) {
	if q.Text == "" {
		return nil, nil
	}
	query := ftsQuery(q.Text)
	if query == "" {
		return nil, nil
	}

	scopeConds, scopeArgs, err := s.scopeFilter(ctx, q)
	if err != nil {
		return nil, err
	}

	sqlText := `
		SELECT m.id, m.type, sc.path, m.scope_id, m.content, m.tags, m.source_agent, m.created_at,
		       bm25(memories_fts) AS score
		FROM memories_fts
		JOIN memories m ON m.id = memories_fts.rowid
		JOIN scopes sc ON sc.id = m.scope_id
		WHERE memories_fts MATCH ? AND m.status = 'active'` +
		scopeConds +
		` ORDER BY score LIMIT ?`
	args := []any{query}
	args = append(args, scopeArgs...)
	args = append(args, top)

	return s.queryRanked(ctx, sqlText, args, "keyword")
}

// Facts ranks by key-prefix lookup (key LIKE '<text>%').
func (s *Searcher) Facts(ctx context.Context, q Query, top int) ([]Ranked, error) {
	if q.Text == "" {
		return nil, nil
	}
	prefix := strings.ToLower(q.Text) + "%"

	scopeConds, scopeArgs, err := s.scopeFilter(ctx, q)
	if err != nil {
		return nil, err
	}

	sqlText := `
		SELECT m.id, m.type, sc.path, m.scope_id, m.content, m.tags, m.source_agent, m.created_at,
		       0.0 AS score
		FROM memories m
		JOIN scopes sc ON sc.id = m.scope_id
		WHERE m.type = 'fact' AND m.status = 'active' AND m.key LIKE ?` +
		scopeConds +
		` ORDER BY m.key LIMIT ?`
	args := []any{prefix}
	args = append(args, scopeArgs...)
	args = append(args, top)

	return s.queryRanked(ctx, sqlText, args, "facts")
}

// Timeline orders memories by created_at desc within the scope/time window.
func (s *Searcher) Timeline(ctx context.Context, q Query, top int) ([]Ranked, error) {
	scopeConds, scopeArgs, err := s.scopeFilter(ctx, q)
	if err != nil {
		return nil, err
	}

	sqlText := `
		SELECT m.id, m.type, sc.path, m.scope_id, m.content, m.tags, m.source_agent, m.created_at,
		       0.0 AS score
		FROM memories m
		JOIN scopes sc ON sc.id = m.scope_id
		WHERE m.status = 'active'` +
		scopeConds +
		` ORDER BY m.created_at DESC LIMIT ?`
	args := []any(scopeArgs)
	args = append(args, top)

	results, err := s.queryRanked(ctx, sqlText, args, "timeline")
	if err != nil {
		return nil, err
	}
	// Apply since/until filters (these must not change ordering semantics of
	// the SQL above; applying here is simplest and correct for M1).
	return filterRanked(results, q), nil
}

// filterRanked applies since/until filters to a raw timeline result list.
func filterRanked(rs []Ranked, q Query) []Ranked {
	if q.Since.IsZero() && q.Until.IsZero() {
		return rs
	}
	out := make([]Ranked, 0, len(rs))
	for _, r := range rs {
		if !q.Since.IsZero() && r.CreatedAt.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && r.CreatedAt.After(q.Until) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Recall fuses keyword + facts + timeline + semantic via RRF, applies
// filters, dedups by id, and returns the top-N ranked results.
func (s *Searcher) Recall(ctx context.Context, q Query) ([]Ranked, error) {
	top := q.Top
	if top <= 0 {
		top = 5
	}
	if top > 20 {
		top = 20
	}
	cand := top * 3

	kw, err := s.Keyword(ctx, q, cand)
	if err != nil {
		return nil, err
	}
	facts, err := s.Facts(ctx, q, cand)
	if err != nil {
		return nil, err
	}
	tl, err := s.Timeline(ctx, q, cand)
	if err != nil {
		return nil, err
	}
	sem, err := s.Semantic(ctx, q, cand)
	if err != nil {
		return nil, err
	}

	lists := []struct {
		name  string
		items []Ranked
	}{
		{"keyword", kw},
		{"facts", facts},
		{"timeline", tl},
		{"semantic", sem},
	}

	scores := map[int64]*Ranked{}
	for _, l := range lists {
		for rank, item := range l.items {
			item.Score += 1.0 / (float64(rrfK) + float64(rank+1))
			if existing, ok := scores[item.ID]; ok {
				existing.Score += 1.0 / (float64(rrfK) + float64(rank+1))
				if !contains(existing.MatchedBy, l.name) {
					existing.MatchedBy = append(existing.MatchedBy, l.name)
				}
				continue
			}
			item.MatchedBy = []string{l.name}
			scores[item.ID] = &item
		}
	}

	results := make([]*Ranked, 0, len(scores))
	for _, r := range scores {
		results = append(results, r)
	}

	results = filterResults(results, q)

	slices.SortFunc(results, func(a, b *Ranked) int {
		if b.Score > a.Score {
			return 1
		}
		if b.Score < a.Score {
			return -1
		}
		return 0
	})

	if len(results) > top {
		results = results[:top]
	}

	out := make([]Ranked, 0, len(results))
	for _, r := range results {
		out = append(out, *r)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// filterResults applies type/tags/since/until/agent filters.
func filterResults(rs []*Ranked, q Query) []*Ranked {
	out := make([]*Ranked, 0, len(rs))
	for _, r := range rs {
		if q.Type != "" && r.Type != q.Type {
			continue
		}
		if len(q.Tags) > 0 && !hasAnyTag(r.Tags, q.Tags) {
			continue
		}
		if !q.Since.IsZero() && r.CreatedAt.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && r.CreatedAt.After(q.Until) {
			continue
		}
		if q.Agent != "" && r.SourceAgent != q.Agent {
			continue
		}
		out = append(out, r)
	}
	return out
}

func hasAnyTag(tags, qtags []string) bool {
	for _, t := range tags {
		for _, qt := range qtags {
			if t == qt {
				return true
			}
		}
	}
	return false
}

// scopeFilter builds SQL conditions + args to restrict results to the target
// scope (plus inheritance/children as per the query).
func (s *Searcher) scopeFilter(ctx context.Context, q Query) (string, []any, error) {
	if q.Scope == "" {
		return "", nil, nil
	}
	sc, err := scope.Parse(q.Scope)
	if err != nil {
		return "", nil, err
	}

	ids, err := s.store.ResolveScopeIDs(ctx, sc, q.Inherit, q.Children)
	if err != nil {
		return "", nil, err
	}
	if len(ids) == 0 {
		return " AND 1 = 0", nil, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return " AND m.scope_id IN (" + placeholders + ")", args, nil
}

// ftsQuery builds a safe FTS5 MATCH expression by quoting each whitespace token.
func ftsQuery(text string) string {
	fields := strings.Fields(text)
	var quoted []string
	for _, f := range fields {
		f = strings.Trim(f, `"'()*`)
		if f == "" {
			continue
		}
		quoted = append(quoted, `"`+f+`"`)
	}
	return strings.Join(quoted, " ")
}

// queryRanked scans the common Ranked result shape.
func (s *Searcher) queryRanked(ctx context.Context, sqlText string, args []any, matchedBy string) ([]Ranked, error) {
	rows, err := s.db().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Ranked
	for rows.Next() {
		var r Ranked
		var tags string
		var created int64
		if err := rows.Scan(&r.ID, &r.Type, &r.Scope, &r.ScopeID, &r.Content, &tags, &r.SourceAgent, &created, &r.Score); err != nil {
			return nil, err
		}
		r.Tags = splitTags(tags)
		r.CreatedAt = time.UnixMicro(created)
		r.MatchedBy = []string{matchedBy}
		out = append(out, r)
	}
	return out, rows.Err()
}

// loadMemoriesByIDs loads full memory rows (with scope path) for the given ids.
func (s *Searcher) loadMemoriesByIDs(ctx context.Context, ids []int64) ([]store.Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db().QueryContext(ctx, `
		SELECT m.id, m.scope_id, sc.path, m.type, m.content, m.key, m.value_json, m.tags,
		       m.source_agent, m.source_session, m.content_hash, m.status, m.summarize_at,
		       m.created_at, m.updated_at
		FROM memories m
		JOIN scopes sc ON sc.id = m.scope_id
		WHERE m.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Memory, 0, len(ids))
	for rows.Next() {
		m, err := scanMemoryRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// scanMemoryRow scans a full memory row (must match loadMemoriesByIDs column order).
func scanMemoryRow(row interface{ Scan(...any) error }) (*store.Memory, error) {
	var m store.Memory
	var tags, valueJSON, key, sourceAgent, sourceSession sql.NullString
	var summarizeAt sql.NullInt64
	var created, updated int64
	err := row.Scan(
		&m.ID, &m.ScopeID, &m.ScopePath, &m.Type, &m.Content, &key, &valueJSON,
		&tags, &sourceAgent, &sourceSession, &m.ContentHash, &m.Status,
		&summarizeAt, &created, &updated,
	)
	if err != nil {
		return nil, err
	}
	m.Key = key.String
	m.ValueJSON = valueJSON.String
	m.SourceAgent = sourceAgent.String
	m.SourceSession = sourceSession.String
	m.Tags = splitTags(tags.String)
	if summarizeAt.Valid {
		v := summarizeAt.Int64
		m.SummarizeAt = &v
	}
	m.CreatedAt = time.UnixMicro(created)
	m.UpdatedAt = time.UnixMicro(updated)
	return &m, nil
}

// splitTags parses a comma-separated tag string into a slice.
func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
