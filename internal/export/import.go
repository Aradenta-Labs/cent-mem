package export

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// ImportOptions configures optional behavior for importing memories.
type ImportOptions struct {
	DryRun bool
}

// Import parses and ingests memories from an export stream into the store.
func Import(ctx context.Context, s *store.Store, r io.Reader) (ImportReport, error) {
	return ImportWithOptions(ctx, s, r, ImportOptions{})
}

// ImportWithOptions parses and ingests memories with optional configuration (e.g. DryRun).
func ImportWithOptions(ctx context.Context, s *store.Store, r io.Reader, opts ImportOptions) (ImportReport, error) {
	env, err := ReadEnvelope(r)
	if err != nil {
		return ImportReport{}, err
	}

	report := ImportReport{
		Total:  len(env.Memories),
		Errors: make([]string, 0),
	}

	seenHashes := make(map[string]bool)

	for i, rec := range env.Memories {
		targetScope := rec.Scope
		if targetScope == "" {
			targetScope = env.Scope
		}
		if targetScope == "" {
			targetScope = "global"
		}

		sc, err := scope.Parse(targetScope)
		if err != nil {
			report.Failed++
			report.Errors = append(report.Errors, fmt.Sprintf("record %d: invalid scope %q: %v", i, targetScope, err))
			continue
		}

		if rec.Type != "fact" && rec.Type != "note" && rec.Type != "log" {
			report.Failed++
			report.Errors = append(report.Errors, fmt.Sprintf("record %d: unsupported memory type %q (expected fact, note, or log)", i, rec.Type))
			continue
		}

		if rec.Type == "fact" && strings.TrimSpace(rec.Content) == "" && rec.Key != "" {
			rec.Content = rec.Key + " = " + rec.ValueJSON
		}

		if strings.TrimSpace(rec.Content) == "" {
			report.Failed++
			report.Errors = append(report.Errors, fmt.Sprintf("record %d: content is required", i))
			continue
		}

		contentHash := store.ContentHash(sc.Path, rec.Type, rec.Key, rec.Content)
		if seenHashes[contentHash] {
			report.Skipped++
			continue
		}

		exists, err := s.HasActiveMemoryExact(ctx, sc.Path, rec.Type, rec.Key, rec.Content)
		if err != nil {
			report.Failed++
			report.Errors = append(report.Errors, fmt.Sprintf("record %d: check duplicate: %v", i, err))
			continue
		}
		if exists {
			seenHashes[contentHash] = true
			report.Skipped++
			continue
		}

		if opts.DryRun {
			seenHashes[contentHash] = true
			report.Imported++
			continue
		}

		if _, err := s.EnsureScope(ctx, sc); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, fmt.Sprintf("record %d: ensure scope: %v", i, err))
			continue
		}

		input := store.MemoryInput{
			Scope:         sc.Path,
			Type:          rec.Type,
			Content:       rec.Content,
			Key:           rec.Key,
			ValueJSON:     rec.ValueJSON,
			Tags:          rec.Tags,
			SourceAgent:   rec.SourceAgent,
			SourceSession: rec.SourceSession,
		}

		_, status, err := s.PutMemory(ctx, input)
		if err != nil {
			report.Failed++
			report.Errors = append(report.Errors, fmt.Sprintf("record %d: put memory: %v", i, err))
			continue
		}

		seenHashes[contentHash] = true
		if status == "merged" {
			report.Skipped++
		} else {
			report.Imported++
		}
	}

	return report, nil
}
