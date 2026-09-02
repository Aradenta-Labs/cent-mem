package capture

import (
	"context"
	"fmt"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// Writer writes classified capture items into the cent-mem SQLite store.
type Writer struct {
	store *store.Store
	cfg   CaptureConfig
}

// NewWriter creates a new Writer.
func NewWriter(st *store.Store, cfg CaptureConfig) *Writer {
	return &Writer{
		store: st,
		cfg:   cfg,
	}
}

// Write persists a CaptureItem into the persistent store with proper type mapping and provenance.
func (w *Writer) Write(ctx context.Context, item CaptureItem, sessionID string) (id int64, status string, err error) {
	if w.store == nil {
		return 0, "", fmt.Errorf("writer: nil store")
	}

	scope := strings.TrimSpace(w.cfg.Scope)
	if scope == "" {
		scope = "global"
	}

	category := strings.ToLower(strings.TrimSpace(item.Category))
	sourceAgent := "capture-hook"

	switch category {
	case "fact":
		key := strings.TrimSpace(item.Key)
		if key == "" {
			key = "fact." + sanitizeKey(item.Content)
		}
		tags := mergeTags(item.Tags, "fact")
		return w.store.SetFact(ctx, store.FactInput{
			Scope:       scope,
			Key:         key,
			Value:       item.Content,
			Tags:        tags,
			SourceAgent: sourceAgent,
		})

	case "dependency":
		key := strings.TrimSpace(item.Key)
		if key == "" {
			key = "dep." + sanitizeKey(item.Content)
		} else if !strings.HasPrefix(key, "dep.") {
			key = "dep." + key
		}
		tags := mergeTags(item.Tags, "dependency")
		return w.store.SetFact(ctx, store.FactInput{
			Scope:       scope,
			Key:         key,
			Value:       item.Content,
			Tags:        tags,
			SourceAgent: sourceAgent,
		})

	case "log":
		return w.store.PutMemory(ctx, store.MemoryInput{
			Scope:         scope,
			Type:          "log",
			Content:       item.Content,
			Tags:          item.Tags,
			SourceAgent:   sourceAgent,
			SourceSession: sessionID,
		})

	case "decision":
		tags := mergeTags(item.Tags, "decision")
		return w.store.PutMemory(ctx, store.MemoryInput{
			Scope:         scope,
			Type:          "note",
			Content:       item.Content,
			Tags:          tags,
			SourceAgent:   sourceAgent,
			SourceSession: sessionID,
		})

	case "preference":
		tags := mergeTags(item.Tags, "preference")
		return w.store.PutMemory(ctx, store.MemoryInput{
			Scope:         scope,
			Type:          "note",
			Content:       item.Content,
			Tags:          tags,
			SourceAgent:   sourceAgent,
			SourceSession: sessionID,
		})

	case "code":
		tags := mergeTags(item.Tags, "code")
		return w.store.PutMemory(ctx, store.MemoryInput{
			Scope:         scope,
			Type:          "note",
			Content:       item.Content,
			Tags:          tags,
			SourceAgent:   sourceAgent,
			SourceSession: sessionID,
		})

	case "error":
		tags := mergeTags(item.Tags, "error", "resolution")
		return w.store.PutMemory(ctx, store.MemoryInput{
			Scope:         scope,
			Type:          "note",
			Content:       item.Content,
			Tags:          tags,
			SourceAgent:   sourceAgent,
			SourceSession: sessionID,
		})

	default:
		// Custom categories default to note with the category tag
		tags := mergeTags(item.Tags, category)
		return w.store.PutMemory(ctx, store.MemoryInput{
			Scope:         scope,
			Type:          "note",
			Content:       item.Content,
			Tags:          tags,
			SourceAgent:   sourceAgent,
			SourceSession: sessionID,
		})
	}
}

func mergeTags(existing []string, extra ...string) []string {
	seen := make(map[string]bool)
	var merged []string
	for _, t := range existing {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			merged = append(merged, t)
		}
	}
	for _, t := range extra {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			merged = append(merged, t)
		}
	}
	return merged
}
