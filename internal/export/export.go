package export

import (
	"context"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// ExportQuery specifies filters for exporting memories.
type ExportQuery struct {
	Scope   string
	Type    string
	Tags    []string
	Since   time.Time
	Until   time.Time
	Agent   string
	Session string
}

// Export dumps memories matching the query into a canonical Envelope.
// It iterates store.List() in batches of 500 without a hard limit.
func Export(ctx context.Context, s *store.Store, q ExportQuery) (Envelope, error) {
	scopePath := q.Scope
	if scopePath == "" {
		scopePath = "global"
	}
	sc, err := scope.Parse(scopePath)
	if err != nil {
		return Envelope{}, err
	}

	scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
	if err != nil {
		return Envelope{}, err
	}

	records := make([]MemoryRecord, 0)
	if len(scopeIDs) > 0 {
		const pageSize = 500
		offset := 0
		for {
			lq := store.ListQuery{
				ScopeIDs:      scopeIDs,
				Type:          q.Type,
				Tags:          q.Tags,
				SourceAgent:   q.Agent,
				SourceSession: q.Session,
				Since:         q.Since,
				Until:         q.Until,
				Status:        "active",
				Limit:         pageSize,
				Offset:        offset,
			}
			batch, err := s.List(ctx, lq)
			if err != nil {
				return Envelope{}, err
			}
			for _, m := range batch {
				tags := m.Tags
				if tags == nil {
					tags = []string{}
				}
				records = append(records, MemoryRecord{
					ID:            m.ID,
					Scope:         m.ScopePath,
					Type:          m.Type,
					Content:       m.Content,
					Key:           m.Key,
					ValueJSON:     m.ValueJSON,
					Tags:          tags,
					SourceAgent:   m.SourceAgent,
					SourceSession: m.SourceSession,
					CreatedAt:     m.CreatedAt.Unix(),
				})
			}
			if len(batch) < pageSize {
				break
			}
			offset += len(batch)
		}
	}

	return Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         scopePath,
		Total:         len(records),
		Memories:      records,
	}, nil
}
