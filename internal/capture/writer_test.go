package capture_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestWriter_CategoryMappingsAndProvenance(t *testing.T) {
	tempHome := t.TempDir()
	cfg := config.Config{
		Home:   tempHome,
		DBPath: filepath.Join(tempHome, "test.db"),
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	captureCfg := capture.DefaultCaptureConfig()
	captureCfg.Scope = "project:myproj"
	sessionID := "session-xyz-789"

	writer := capture.NewWriter(st, captureCfg)

	testCases := []struct {
		category     string
		content      string
		key          string
		expectedType string
		requiredTag  string
		isFact       bool
	}{
		{category: "decision", content: "Adopt SQLite", expectedType: "note", requiredTag: "decision"},
		{category: "preference", content: "Use 2 spaces", expectedType: "note", requiredTag: "preference"},
		{category: "code", content: "fmt.Println()", expectedType: "note", requiredTag: "code"},
		{category: "log", content: "Completed step", expectedType: "log"},
		{category: "error", content: "Fixed nil panic", expectedType: "note", requiredTag: "error"},
		{category: "fact", content: "https://centmem.io", key: "url.centmem", isFact: true},
		{category: "dependency", content: "github.com/mattn/go-sqlite3", key: "go-sqlite3", isFact: true},
		{category: "custom_note", content: "Custom category note", expectedType: "note", requiredTag: "custom_note"},
	}

	for _, tc := range testCases {
		t.Run(tc.category, func(t *testing.T) {
			item := capture.CaptureItem{
				Category: tc.category,
				Content:  tc.content,
				Key:      tc.key,
			}

			id, _, err := writer.Write(ctx, item, sessionID)
			if err != nil {
				t.Fatalf("Write failed for %s: %v", tc.category, err)
			}
			if id <= 0 {
				t.Fatalf("expected positive memory id, got %d", id)
			}

			if tc.isFact {
				factKey := tc.key
				if tc.category == "dependency" && !strings.HasPrefix(factKey, "dep.") {
					factKey = "dep." + factKey
				}
				fact, err := st.GetFact(ctx, captureCfg.Scope, factKey, false)
				if err != nil {
					t.Fatalf("GetFact: %v", err)
				}
				if fact == nil || fact.Value != tc.content {
					t.Errorf("fact value mismatch: got %+v", fact)
				}
			} else {
				mems, err := st.List(ctx, store.ListQuery{
					ScopePath: captureCfg.Scope,
					Status:    "active",
				})
				if err != nil {
					t.Fatalf("List: %v", err)
				}

				var found *store.Memory
				for _, m := range mems {
					if m.ID == id {
						found = &m
						break
					}
				}
				if found == nil {
					t.Fatalf("memory id %d not found in list", id)
				}

				if found.Type != tc.expectedType {
					t.Errorf("expected type %q, got %q", tc.expectedType, found.Type)
				}
				if found.SourceAgent != "capture-hook" {
					t.Errorf("expected SourceAgent 'capture-hook', got %q", found.SourceAgent)
				}
				if found.SourceSession != sessionID {
					t.Errorf("expected SourceSession %q, got %q", sessionID, found.SourceSession)
				}
				if tc.requiredTag != "" {
					hasTag := false
					for _, tag := range found.Tags {
						if tag == tc.requiredTag {
							hasTag = true
							break
						}
					}
					if !hasTag {
						t.Errorf("expected tag %q in %+v", tc.requiredTag, found.Tags)
					}
				}
			}
		})
	}
}
