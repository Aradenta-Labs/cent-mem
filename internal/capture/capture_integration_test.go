package capture_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestE2E_CoreCapturePipeline(t *testing.T) {
	tempHome := t.TempDir()
	dbPath := filepath.Join(tempHome, "centmem.db")

	cfg := config.Config{
		Home:   tempHome,
		DBPath: dbPath,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// 1. Create a realistic multi-turn transcript file with mixed categories
	transcriptContent := `{"role":"user","content":"Let's decide on the database architecture."}
{"role":"assistant","content":"We decided to use SQLite with WAL mode enabled."}
{"role":"user","content":"What is our API endpoint and version?"}
{"role":"assistant","content":"Documentation is at https://api.centmem.io/v1 for release v1.3.0."}
{"role":"user","content":"How should we install the dependencies?"}
{"role":"assistant","content":"Run go get github.com/mattn/go-sqlite3 and npm install @cent-mem/sdk."}
{"role":"assistant","content":"Here is the initialization snippet:\n` + "```go\nfunc InitStore() error {\n  return nil\n}\n```" + `"}
{"role":"assistant","content":"Completed initialization of core modules."}
{"role":"assistant","content":"The root cause was missing SQLite foreign keys, resolved by enabling PRAGMA foreign_keys."}
`
	transcriptPath := filepath.Join(tempHome, "agent-session.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	captureCfg := capture.DefaultCaptureConfig()
	captureCfg.Scope = "project:cent-mem"
	captureCfg.Backend = "heuristic"
	captureCfg.ConfidenceThreshold = 0.5

	// 2. Start capture session
	sess, err := capture.StartSession(tempHome, "antigravity", captureCfg)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// 3. Read transcript
	messages, err := capture.ReadTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ReadTranscript: %v", err)
	}
	sess.Tracker.SetTotalMessages(len(messages))

	// 4. Recall helper for classifier
	recallFn := func(ctx context.Context, q string) ([]string, error) {
		mems, err := st.List(ctx, store.ListQuery{
			ScopePath: captureCfg.Scope,
			Status:    "active",
			Limit:     5,
		})
		if err != nil {
			return nil, err
		}
		var snippets []string
		for _, m := range mems {
			snippets = append(snippets, m.Content)
		}
		return snippets, nil
	}

	// 5. Classify transcript
	items, err := capture.ClassifyWithConfig(ctx, messages, captureCfg, recallFn)
	if err != nil {
		t.Fatalf("ClassifyWithConfig: %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("expected classified items, got 0")
	}

	// 6. Deduplicate and write items to store
	writer := capture.NewWriter(st, captureCfg)

	for _, item := range items {
		if sess.Dedup.IsSessionDuplicate(item) {
			sess.Tracker.RecordSkippedDuplicate(item)
			continue
		}

		isStoreDup, err := capture.IsStoreDuplicate(ctx, item, captureCfg.Scope, st)
		if err != nil {
			t.Fatalf("IsStoreDuplicate: %v", err)
		}
		if isStoreDup {
			sess.Tracker.RecordSkippedDuplicate(item)
			continue
		}

		id, _, err := writer.Write(ctx, item, sess.ID)
		if err != nil {
			t.Fatalf("writer.Write failed for item %+v: %v", item, err)
		}
		if id <= 0 {
			t.Fatalf("expected valid id > 0, got %d", id)
		}

		if err := sess.Dedup.MarkSaved(item); err != nil {
			t.Fatalf("MarkSaved: %v", err)
		}
		sess.Tracker.RecordCaptured(item)
	}

	// 7. End session
	summary, err := capture.EndSession(sess)
	if err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	if summary.Captured == 0 {
		t.Errorf("expected captured memories > 0, got 0")
	}

	// 8. Verify stored memories in SQLite store
	mems, err := st.List(ctx, store.ListQuery{
		ScopePath: captureCfg.Scope,
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("List memories: %v", err)
	}

	if len(mems) == 0 {
		t.Fatalf("expected stored memories in store, got 0")
	}

	for _, m := range mems {
		if m.SourceAgent != "capture-hook" {
			t.Errorf("expected SourceAgent 'capture-hook', got %q", m.SourceAgent)
		}
		if m.Type != "fact" && m.SourceSession != sess.ID {
			t.Errorf("expected SourceSession %q for type %q, got %q", sess.ID, m.Type, m.SourceSession)
		}
	}

	// 9. Run a second pass with the exact same transcript — all should be skipped as duplicates
	sess2, err := capture.StartSession(tempHome, "antigravity", captureCfg)
	if err != nil {
		t.Fatalf("StartSession 2: %v", err)
	}

	items2, err := capture.ClassifyWithConfig(ctx, messages, captureCfg, recallFn)
	if err != nil {
		t.Fatalf("ClassifyWithConfig 2: %v", err)
	}

	for _, item := range items2 {
		isStoreDup, err := capture.IsStoreDuplicate(ctx, item, captureCfg.Scope, st)
		if err != nil {
			t.Fatalf("IsStoreDuplicate pass 2: %v", err)
		}
		if isStoreDup {
			sess2.Tracker.RecordSkippedDuplicate(item)
			continue
		}

		id, _, err := writer.Write(ctx, item, sess2.ID)
		if err != nil {
			t.Fatalf("writer.Write pass 2: %v", err)
		}
		sess2.Tracker.RecordCaptured(item)
		_ = id
	}

	summary2, err := capture.EndSession(sess2)
	if err != nil {
		t.Fatalf("EndSession 2: %v", err)
	}

	if summary2.Captured != 0 {
		t.Errorf("expected 0 new items captured in second pass (all duplicates), got %d", summary2.Captured)
	}
	if summary2.SkippedDuplicate == 0 {
		t.Errorf("expected duplicates skipped in second pass, got 0")
	}

	// 10. Verify summary file exists and can be loaded
	loadedSummary, err := capture.LoadSummary(tempHome, sess.ID)
	if err != nil {
		t.Fatalf("LoadSummary: %v", err)
	}
	if loadedSummary.SessionID != sess.ID || loadedSummary.Captured != summary.Captured {
		t.Errorf("loaded summary does not match: %+v vs %+v", loadedSummary, summary)
	}
}
