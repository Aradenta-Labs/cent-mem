// Package search implements hybrid retrieval (semantic, FTS5 keyword, facts, timeline).
//
// Phase 1 (M1) implements the non-semantic parts: FTS5 keyword ranking, fact
// key-prefix lookup, and timeline ordering, fused via Reciprocal Rank Fusion.
// The semantic ranker is a stub returning empty results and is completed in M2.
package search

import (
	"context"
	"database/sql"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// rrfK is the smoothing constant used by Reciprocal Rank Fusion.
const rrfK = 60

// Query captures a full search request. It is reused by both M1 (non-semantic)
// and M2 (semantic) rankers.
type Query struct {
	Text        string
	Scope       string
	Inherit     bool
	Children    bool
	Top         int
	Type        string
	Tags        []string
	Since       time.Time
	Until       time.Time
	Agent                 string
	CallerAgent           string
	IncludeLinks          bool
	IncludeSuggestedLinks bool
}

// LinkedMemory describes an edge connected to a ranked memory result.
type LinkedMemory struct {
	LinkID        int64  `json:"link_id,omitempty"`
	Relation      string `json:"relation"`
	Direction     string `json:"direction"` // "outgoing" | "incoming"
	LinkedID      int64  `json:"linked_id"`
	LinkedContent string `json:"linked_content"`
	Suggested     bool   `json:"suggested,omitempty"`
}

// Ranked is a single search result.
type Ranked struct {
	ID             int64
	Type           string
	Scope          string
	ScopeID        int64
	Content        string
	Key            string // fact key or empty
	ContentHash    string // SHA-256 hash from memories.content_hash
	Tags           []string
	SourceAgent    string
	AccessCount    int
	LastAccessedAt *time.Time
	CreatedAt      time.Time
	Score          float64
	MatchedBy      []string
	SemanticScore  float64 // dense semantic similarity (1.0 - cosDist)
	Links          []LinkedMemory
}

// Searcher executes hybrid search against a Store.
type Searcher struct {
	store             *store.Store
	emb               embed.Embedder
	decayHalfLifeDays int
	reranker          ReRanker
	rerankWindow      int
	sessionBoost      float64
	agentBoost        float64
	importanceEnabled bool
	importanceWeight  float64
	importanceCap     float64
}

// New builds a Searcher backed by s. It uses an offline StubEmbedder by
// default; call WithEmbedder to attach a real ONNX embedder.
func New(s *store.Store) *Searcher {
	return &Searcher{
		store:             s,
		emb:               embed.NewStub(384),
		decayHalfLifeDays: 0,
		reranker:          NewCompositeReRanker(),
		rerankWindow:      30,
		sessionBoost:      1.25,
		agentBoost:        1.15,
		importanceEnabled: true,
		importanceWeight:  0.1,
		importanceCap:     2.0,
	}
}

// WithImportance configures importance scoring boost parameters.
func (s *Searcher) WithImportance(enabled bool, weight, cap float64) *Searcher {
	dup := *s
	dup.importanceEnabled = enabled
	dup.importanceWeight = weight
	dup.importanceCap = cap
	return &dup
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

// WithDecayDays configures the recency decay half-life in days.
// If days <= 0, decay is disabled (the default).
// Returns a copy of the Searcher for safe concurrent chaining.
func (s *Searcher) WithDecayDays(days int) *Searcher {
	if days < 0 {
		days = 0
	}
	dup := *s
	dup.decayHalfLifeDays = days
	return &dup
}

// WithReranker configures the re-ranker strategy instance.
func (s *Searcher) WithReranker(r ReRanker) *Searcher {
	dup := *s
	dup.reranker = r
	return &dup
}

// WithRerankerName configures the re-ranker strategy by name.
func (s *Searcher) WithRerankerName(name string) *Searcher {
	dup := *s
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "none":
		dup.reranker = &noneReRanker{}
	case "cross_encoder":
		dup.reranker = NewCrossEncoderReRanker()
	case "llm":
		dup.reranker = NewLLMReRanker()
	case "composite", "":
		dup.reranker = NewCompositeReRanker()
	default:
		dup.reranker = NewCompositeReRanker()
	}
	return &dup
}

// WithRerankWindow sets the candidate window slice size for Stage 2 re-ranking.
func (s *Searcher) WithRerankWindow(w int) *Searcher {
	if w <= 0 {
		w = 30
	}
	dup := *s
	dup.rerankWindow = w
	return &dup
}

// WithSessionBoost sets the score multiplier for memories in matching session scope.
func (s *Searcher) WithSessionBoost(b float64) *Searcher {
	if b <= 0 {
		b = 1.25
	}
	dup := *s
	dup.sessionBoost = b
	return &dup
}

// WithAgentBoost sets the score multiplier for memories authored by caller agent.
func (s *Searcher) WithAgentBoost(b float64) *Searcher {
	if b <= 0 {
		b = 1.15
	}
	dup := *s
	dup.agentBoost = b
	return &dup
}

// db exposes the underlying SQL handle.
func (s *Searcher) db() *sql.DB { return s.store.DB() }

// Keyword ranks memories by FTS5 bm25 over content + tags.
func (s *Searcher) Keyword(ctx context.Context, q Query, top int) ([]Ranked, error) {
	if q.Text == "" {
		return nil, nil
	}
	exact, prefix := ftsQueries(q.Text)
	if exact == "" {
		return nil, nil
	}

	scopeConds, scopeArgs, err := s.scopeFilter(ctx, q)
	if err != nil {
		return nil, err
	}

	runQuery := func(query string, matchedBy string) ([]Ranked, error) {
		sqlText := `
			SELECT m.id, m.type, sc.path, m.scope_id, m.content, m.content_hash, m.tags, m.source_agent, m.created_at,
			       bm25(memories_fts) AS score, COALESCE(m.key, ''), m.access_count, m.last_accessed_at
			FROM memories_fts
			JOIN memories m ON m.id = memories_fts.rowid
			JOIN scopes sc ON sc.id = m.scope_id
			WHERE memories_fts MATCH ? AND m.status = 'active'` +
			scopeConds +
			` ORDER BY score LIMIT ?`
		args := []any{query}
		args = append(args, scopeArgs...)
		args = append(args, top)
		return s.queryRanked(ctx, sqlText, args, matchedBy)
	}

	res, err := runQuery(exact, "keyword")
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return runQuery(prefix, "keyword_prefix")
	}
	return res, nil
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
		SELECT m.id, m.type, sc.path, m.scope_id, m.content, m.content_hash, m.tags, m.source_agent, m.created_at,
		       0.0 AS score, COALESCE(m.key, ''), m.access_count, m.last_accessed_at
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
		SELECT m.id, m.type, sc.path, m.scope_id, m.content, m.content_hash, m.tags, m.source_agent, m.created_at,
		       0.0 AS score, COALESCE(m.key, ''), m.access_count, m.last_accessed_at
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
// filters, dedups by id and content_hash, and returns the top-N ranked results.
func (s *Searcher) Recall(ctx context.Context, q Query) ([]Ranked, error) {
	top := q.Top
	if top <= 0 {
		top = 5
	}
	if top > 200 {
		top = 200
	}
	const candidateMultiplier = 5
	const candidateFloor = 50

	cand := max(top*candidateMultiplier, candidateFloor)

	// Facts receives unexpanded q to preserve key prefix matching (key LIKE prefix%).
	factsQuery := q

	// R1: Query Expansion via tag enrichment
	enrichedQuery, err := s.EnrichQuery(ctx, q)
	if err != nil {
		return nil, err
	}

	kw, err := s.Keyword(ctx, enrichedQuery, cand)
	if err != nil {
		return nil, err
	}
	facts, err := s.Facts(ctx, factsQuery, cand)
	if err != nil {
		return nil, err
	}
	tl, err := s.Timeline(ctx, q, cand)
	if err != nil {
		return nil, err
	}
	sem, err := s.Semantic(ctx, enrichedQuery, cand)
	if err != nil {
		return nil, err
	}

	type rankerDef struct {
		name   string
		items  []Ranked
		weight float64
	}

	var timelineWeight float64 = 1.0
	if q.Text != "" {
		timelineWeight = 0.3 // recency ≠ relevance for text queries
	}

	lists := []rankerDef{
		{"keyword", kw, 1.0},
		{"facts", facts, 1.0},
		{"timeline", tl, timelineWeight},
		{"semantic", sem, 1.2}, // semantic similarity is highest-confidence signal
	}

	scores := map[int64]*Ranked{}
	for _, l := range lists {
		for rank, item := range l.items {
			contrib := l.weight / (float64(rrfK) + float64(rank+1))
			if existing, ok := scores[item.ID]; ok {
				existing.Score += contrib
				if item.SemanticScore > 0 {
					existing.SemanticScore = item.SemanticScore
				}
				if item.Key != "" && existing.Key == "" {
					existing.Key = item.Key
				}
				if item.AccessCount > existing.AccessCount {
					existing.AccessCount = item.AccessCount
				}
				if existing.LastAccessedAt == nil || (item.LastAccessedAt != nil && item.LastAccessedAt.After(*existing.LastAccessedAt)) {
					existing.LastAccessedAt = item.LastAccessedAt
				}
				if !contains(existing.MatchedBy, l.name) {
					existing.MatchedBy = append(existing.MatchedBy, l.name)
				}
				continue
			}
			item.Score = contrib
			item.MatchedBy = []string{l.name}
			clone := item
			scores[item.ID] = &clone
		}
	}

	results := make([]*Ranked, 0, len(scores))
	for _, r := range scores {
		results = append(results, r)
	}

	// Filter by type, tags, time range, agent
	results = filterResults(results, q)

	// Sort candidates descending by Stage 1 RRF score before candidate window slicing
	slices.SortFunc(results, sortRanked)

	// Stage 2 Re-Ranking: Slice candidate window (default 30)
	rerankWindow := s.rerankWindow
	if rerankWindow <= 0 {
		rerankWindow = 30
	}
	windowSize := min(len(results), rerankWindow)
	reranker := s.reranker
	if reranker == nil {
		reranker = NewCompositeReRanker()
	}

	isReranked := false
	if windowSize > 0 && reranker.Name() != "none" && strings.TrimSpace(q.Text) != "" {
		reranked, err := reranker.ReRank(ctx, q, nil, results[:windowSize])
		if err != nil {
			return nil, err
		}
		copy(results[:windowSize], reranked)
		isReranked = true
	}

	// Session & Agent Boosting (Scope Proximity & Caller Affinity) and Importance Boosting
	for _, r := range results {
		applyScopeProximityBoost(r, q, s.sessionBoost)
		applyAgentAffinityBoost(r, q, s.agentBoost)
		applyImportanceBoost(r, s.importanceEnabled, s.importanceWeight, s.importanceCap)
	}

	// Recency Decay (strictly after boosting and before final sort)
	if s.decayHalfLifeDays > 0 {
		halfLife := time.Duration(s.decayHalfLifeDays) * 24 * time.Hour
		now := time.Now()
		for _, r := range results {
			applyDecay(r, now, halfLife)
		}
	}

	// Sort candidates descending by final score with deterministic tie-breaking.
	// When Stage 2 re-ranking is active, candidates within the window are sorted among
	// themselves, while overflow candidates outside the window remain strictly after
	// the window (preserving their Stage 1 order/ranking without leapfrogging).
	if isReranked {
		slices.SortFunc(results[:windowSize], sortRanked)
		if len(results) > windowSize {
			slices.SortFunc(results[windowSize:], sortRanked)
		}
	} else {
		slices.SortFunc(results, sortRanked)
	}

	// Near-Duplicate Collapse (strictly after sorting and before top truncation)
	results = collapseNearDupes(results)

	if len(results) > top {
		results = results[:top]
	}

	if (q.IncludeLinks || q.IncludeSuggestedLinks) && len(results) > 0 {
		ids := make([]int64, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		linksMap, err := s.store.GetLinksForMemories(ctx, ids, q.IncludeSuggestedLinks)
		if err == nil {
			for _, r := range results {
				attached := linksMap[r.ID]
				r.Links = make([]LinkedMemory, 0, len(attached))
				for _, l := range attached {
					lm := LinkedMemory{
						LinkID:    l.ID,
						Relation:  l.Relation,
						Suggested: l.Suggested,
					}
					if l.FromID == r.ID {
						lm.Direction = "outgoing"
						lm.LinkedID = l.ToID
						lm.LinkedContent = l.TargetContent
					} else {
						lm.Direction = "incoming"
						lm.LinkedID = l.FromID
						lm.LinkedContent = l.SourceContent
					}
					r.Links = append(r.Links, lm)
				}
			}
		}
	}

	out := make([]Ranked, 0, len(results))
	for _, r := range results {
		out = append(out, *r)
	}
	return out, nil
}

// applyDecay multiplies r.Score by an exponential decay factor based on age.
// halfLife <= 0 disables decay.
func applyDecay(r *Ranked, now time.Time, halfLife time.Duration) {
	if r == nil || halfLife <= 0 || r.CreatedAt.IsZero() {
		return
	}
	age := now.Sub(r.CreatedAt)
	if age <= 0 {
		return
	}
	// score *= 0.5 ^ (age / halfLife)
	r.Score *= math.Pow(0.5, float64(age)/float64(halfLife))
}

// collapseNearDupes removes entries whose ContentHash has already been seen,
// keeping only the highest-scored copy. Prevents multiple ingestion passes of
// identical content from occupying multiple result slots. Entries with an empty
// ContentHash are preserved without deduplication.
func collapseNearDupes(results []*Ranked) []*Ranked {
	if len(results) <= 1 {
		return results
	}
	seen := make(map[string]bool, len(results))
	out := make([]*Ranked, 0, len(results))
	for _, r := range results {
		if r.ContentHash != "" {
			if seen[r.ContentHash] {
				continue
			}
			seen[r.ContentHash] = true
		}
		out = append(out, r)
	}
	return out
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

// ftsQueries returns an exact-phrase query and a prefix fallback query.
// The caller tries the exact query first; if it returns 0 rows, retries
// with the prefix query.
func ftsQueries(text string) (exact, prefix string) {
	fields := strings.Fields(text)
	var eq, pf []string
	for _, f := range fields {
		f = strings.ReplaceAll(f, "\x00", "")
		f = strings.Trim(f, `"'()*`)
		f = strings.ReplaceAll(f, `"`, `""`)
		if f == "" {
			continue
		}
		eq = append(eq, `"`+f+`"`)
		pf = append(pf, `"`+f+`"*`)
	}
	return strings.Join(eq, " "), strings.Join(pf, " ")
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
		var tags, key string
		var created int64
		var lastAccessed sql.NullInt64
		if err := rows.Scan(
			&r.ID, &r.Type, &r.Scope, &r.ScopeID, &r.Content, &r.ContentHash, &tags, &r.SourceAgent, &created,
			&r.Score, &key, &r.AccessCount, &lastAccessed,
		); err != nil {
			return nil, err
		}
		r.Tags = splitTags(tags)
		r.Key = key
		r.CreatedAt = time.UnixMicro(created)
		if lastAccessed.Valid {
			t := time.UnixMicro(lastAccessed.Int64)
			r.LastAccessedAt = &t
		}
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
		       m.access_count, m.last_accessed_at, m.created_at, m.updated_at
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
	var summarizeAt, lastAccessedAt sql.NullInt64
	var created, updated int64
	err := row.Scan(
		&m.ID, &m.ScopeID, &m.ScopePath, &m.Type, &m.Content, &key, &valueJSON,
		&tags, &sourceAgent, &sourceSession, &m.ContentHash, &m.Status,
		&summarizeAt, &m.AccessCount, &lastAccessedAt, &created, &updated,
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
	if lastAccessedAt.Valid {
		t := time.UnixMicro(lastAccessedAt.Int64)
		m.LastAccessedAt = &t
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
