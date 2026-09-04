package search

import (
	"context"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// semanticDistThreshold is the maximum cosine distance a memory may have
// from the query vector to be included in the semantic candidate set.
// Distance 0 = identical; 1 = orthogonal; 2 = opposite.
// Tune this value empirically; 0.45 filters out clearly unrelated content.
var semanticDistThreshold = 0.45

// Semantic ranks memories by cosine distance against the query embedding using
// the memories_vec (vec0) index. Scope + active-status are pushed into the KNN
// candidate set via a subquery (sqlite-vec requires the LIMIT/k constraint on
// the MATCH itself). Remaining filters (type/tags/since/until/agent) are
// applied in Go.
func (s *Searcher) Semantic(ctx context.Context, q Query, top int) ([]Ranked, error) {
	if q.Text == "" || s.emb == nil {
		return nil, nil
	}

	vecs, err := s.emb.Embed(ctx, []string{q.Text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	queryVec, err := sqlite_vec.SerializeFloat32(vecs[0])
	if err != nil {
		return nil, err
	}

	scopeConds, scopeArgs, err := s.scopeFilter(ctx, q)
	if err != nil {
		return nil, err
	}

	sqlText := `
		SELECT v.memory_id, v.distance
		FROM memories_vec v
		WHERE v.embedding MATCH ?
		  AND v.memory_id IN (
		    SELECT m.id FROM memories m
		    WHERE m.status = 'active'` + scopeConds + `
		  )
		ORDER BY v.distance ASC
		LIMIT ?`
	args := []any{queryVec}
	args = append(args, scopeArgs...)
	args = append(args, top)

	rows, err := s.db().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type cand struct {
		id       int64
		distance float64
	}
	cands := make([]cand, 0, top)
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.distance); err != nil {
			return nil, err
		}
		
		// c.distance from sqlite-vec MATCH is Euclidean (L2) distance.
		// Convert to cosine distance: cos_dist = L2^2 / 2
		cosDist := (c.distance * c.distance) / 2.0
		if cosDist > semanticDistThreshold {
			continue // skip: too semantically distant
		}
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		return nil, nil
	}

	ids := make([]int64, len(cands))
	dist := make(map[int64]float64, len(cands))
	for i, c := range cands {
		ids[i] = c.id
		dist[c.id] = c.distance
	}

	mems, err := s.loadMemoriesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]*Ranked, 0, len(mems))
	for _, m := range mems {
		// dist[m.ID] is L2 distance. Convert to cosine distance for scoring.
		cosDist := (dist[m.ID] * dist[m.ID]) / 2.0
		sc := 1.0 - cosDist
		if sc < 0 {
			sc = 0
		}
		r := &Ranked{
			ID:            m.ID,
			Type:          m.Type,
			Scope:         m.ScopePath,
			ScopeID:       m.ScopeID,
			Content:       m.Content,
			Key:           m.Key,
			ContentHash:   m.ContentHash,
			Tags:          m.Tags,
			SourceAgent:   m.SourceAgent,
			CreatedAt:     m.CreatedAt,
			Score:         sc,
			SemanticScore: sc,
		}
		out = append(out, r)
	}

	filtered := filterResults(out, q)

	result := make([]Ranked, 0, len(filtered))
	for _, r := range filtered {
		result = append(result, *r)
	}
	return result, nil
}
