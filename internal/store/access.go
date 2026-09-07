package store

import (
	"context"
	"strings"
	"time"
)

// RecordAccess increments access_count and sets last_accessed_at for the specified memory IDs.
// Repeated IDs in the input slice are deduplicated.
func (s *Store) RecordAccess(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Deduplicate IDs
	seen := make(map[int64]bool, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil
	}

	nowMicros := time.Now().UnixMicro()
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(unique)), ",")
	query := `UPDATE memories
		SET access_count = access_count + 1,
		    last_accessed_at = ?
		WHERE id IN (` + placeholders + `)`

	args := make([]any, 0, len(unique)+1)
	args = append(args, nowMicros)
	for _, id := range unique {
		args = append(args, id)
	}

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// RecordAccessAsync initiates an asynchronous, non-blocking background write to record access
// for the given memory IDs. In-flight tasks are tracked via Store.wg to ensure clean shutdown.
func (s *Store) RecordAccessAsync(ids []int64) {
	if len(ids) == 0 {
		return
	}

	// Deduplicate and snapshot IDs to avoid data race with caller
	seen := make(map[int64]bool, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return
	}

	s.wg.Add(1)
	go func(targetIDs []int64) {
		defer s.wg.Done()
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.RecordAccess(bgCtx, targetIDs)
	}(unique)
}
