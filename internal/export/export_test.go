package export

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "centmem.db")
	st, err := store.Open(config.Config{DBPath: dbPath})
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestEnvelopeRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    now,
		Scope:         "project:cent-mem",
		Total:         2,
		Memories: []MemoryRecord{
			{
				ID:            101,
				Scope:         "project:cent-mem",
				Type:          "note",
				Content:       "Test note content",
				Tags:          []string{"test", "roundtrip"},
				SourceAgent:   "agent-a",
				SourceSession: "sess-1",
				CreatedAt:     now.Unix(),
			},
			{
				ID:            102,
				Scope:         "project:cent-mem",
				Type:          "fact",
				Content:       "fact content",
				Key:           "pref.color",
				ValueJSON:     `"blue"`,
				Tags:          []string{"pref"},
				SourceAgent:   "agent-b",
				CreatedAt:     now.Unix(),
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteEnvelope(&buf, env); err != nil {
		t.Fatalf("WriteEnvelope failed: %v", err)
	}

	got, err := ReadEnvelope(&buf)
	if err != nil {
		t.Fatalf("ReadEnvelope failed: %v", err)
	}

	if got.Format != env.Format || got.FormatVersion != env.FormatVersion {
		t.Fatalf("format mismatch: got %s v%d, want %s v%d", got.Format, got.FormatVersion, env.Format, env.FormatVersion)
	}
	if got.Scope != env.Scope || got.Total != env.Total || len(got.Memories) != 2 {
		t.Fatalf("metadata mismatch: got %+v, want %+v", got, env)
	}
	if got.Memories[0].Content != env.Memories[0].Content || got.Memories[1].Key != env.Memories[1].Key {
		t.Fatalf("memory content mismatch: got %+v", got.Memories)
	}
}

func TestExportEmptyScope(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env, err := Export(ctx, st, ExportQuery{Scope: "project:empty"})
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if env.Total != 0 || len(env.Memories) != 0 {
		t.Fatalf("expected 0 memories, got total=%d len=%d", env.Total, len(env.Memories))
	}
	if env.Format != "centmem-export" || env.FormatVersion != 1 {
		t.Fatalf("unexpected format: %s v%d", env.Format, env.FormatVersion)
	}
}

func TestExportPagination(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	// Seed 1,250 memories to cross the 500-item page boundary multiple times
	const count = 1250
	for i := 0; i < count; i++ {
		_, _, err := st.PutMemory(ctx, store.MemoryInput{
			Scope:   "project:pagination",
			Type:    "note",
			Content: fmt.Sprintf("Memory item #%04d for pagination test", i),
		})
		if err != nil {
			t.Fatalf("PutMemory #%d failed: %v", i, err)
		}
	}

	env, err := Export(ctx, st, ExportQuery{Scope: "project:pagination"})
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if env.Total != count || len(env.Memories) != count {
		t.Fatalf("expected %d memories, got total=%d len=%d", count, env.Total, len(env.Memories))
	}
}

func TestImportIdempotent(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         "project:idem",
		Total:         3,
		Memories: []MemoryRecord{
			{
				Scope:   "project:idem",
				Type:    "note",
				Content: "First idempotent note",
			},
			{
				Scope:   "project:idem",
				Type:    "note",
				Content: "Second idempotent note",
			},
			{
				Scope:     "project:idem",
				Type:      "fact",
				Key:       "setting.timeout",
				ValueJSON: "30",
				Content:   "Timeout setting 30",
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteEnvelope(&buf, env); err != nil {
		t.Fatalf("WriteEnvelope failed: %v", err)
	}
	rawJSON := buf.Bytes()

	// First import: all 3 should be imported
	rep1, err := Import(ctx, st, bytes.NewReader(rawJSON))
	if err != nil {
		t.Fatalf("first Import failed: %v", err)
	}
	if rep1.Total != 3 || rep1.Imported != 3 || rep1.Skipped != 0 || rep1.Failed != 0 {
		t.Fatalf("first import report mismatch: %+v", rep1)
	}

	// Second import: all 3 should be skipped as duplicates
	rep2, err := Import(ctx, st, bytes.NewReader(rawJSON))
	if err != nil {
		t.Fatalf("second Import failed: %v", err)
	}
	if rep2.Total != 3 || rep2.Imported != 0 || rep2.Skipped != 3 || rep2.Failed != 0 {
		t.Fatalf("second import report mismatch: %+v", rep2)
	}
}

func TestImportPreservesScope(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         "project:p1",
		Total:         2,
		Memories: []MemoryRecord{
			{
				Scope:   "project:p1/agent:coder",
				Type:    "note",
				Content: "Coder agent note",
			},
			{
				Scope:   "project:p1/agent:tester/session:s1",
				Type:    "log",
				Content: "Tester agent log",
			},
		},
	}

	var buf bytes.Buffer
	_ = WriteEnvelope(&buf, env)

	rep, err := Import(ctx, st, &buf)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if rep.Imported != 2 || rep.Failed != 0 {
		t.Fatalf("expected 2 imported, got %+v", rep)
	}

	// Verify scopes were created and memories can be exported from their specific scopes
	coderExport, err := Export(ctx, st, ExportQuery{Scope: "project:p1/agent:coder"})
	if err != nil {
		t.Fatalf("Export coder scope failed: %v", err)
	}
	if coderExport.Total != 1 || coderExport.Memories[0].Content != "Coder agent note" {
		t.Fatalf("unexpected coder export: %+v", coderExport)
	}
}

func TestImportRejectsBadFormat(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	badFormats := []string{
		`{"format": "invalid-format", "format_version": 1, "memories": [{"content": "a"}]}`,
		`{"format": "centmem-export", "format_version": 2, "memories": [{"content": "a"}]}`,
		`{"format": "centmem-export", "format_version": 1, "memories": []}`,
		`{"not_an_export": true}`,
	}

	for i, bad := range badFormats {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			_, err := Import(ctx, st, strings.NewReader(bad))
			if err == nil {
				t.Fatalf("expected error for bad input %q, got nil", bad)
			}
		})
	}
}

func TestImportRejectsCSV(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	csvContent := "id,scope,type,content,key,value_json,tags\n1,project:test,note,hello,,,\n"
	_, err := Import(ctx, st, strings.NewReader(csvContent))
	if err == nil {
		t.Fatal("expected error importing CSV file, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Fatalf("expected error to mention unsupported file format, got: %v", err)
	}
}

func TestImportPartialFailure(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         "project:partial",
		Total:         3,
		Memories: []MemoryRecord{
			{
				Scope:   "project:partial",
				Type:    "note",
				Content: "Valid note 1",
			},
			{
				Scope:   "invalid scope with spaces and symbols :::",
				Type:    "note",
				Content: "Invalid scope note",
			},
			{
				Scope:   "project:partial",
				Type:    "note",
				Content: "Valid note 2",
			},
		},
	}

	var buf bytes.Buffer
	_ = WriteEnvelope(&buf, env)

	rep, err := Import(ctx, st, &buf)
	if err != nil {
		t.Fatalf("unexpected Import top-level error: %v", err)
	}
	if rep.Total != 3 || rep.Imported != 2 || rep.Failed != 1 || rep.Skipped != 0 {
		t.Fatalf("unexpected partial failure report: %+v", rep)
	}
	if len(rep.Errors) != 1 {
		t.Fatalf("expected 1 error entry, got %d: %v", len(rep.Errors), rep.Errors)
	}
}

func TestImportDryRun(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         "project:dryrun",
		Total:         2,
		Memories: []MemoryRecord{
			{
				Scope:   "project:dryrun",
				Type:    "note",
				Content: "Dry run note 1",
			},
			{
				Scope:   "project:dryrun",
				Type:    "note",
				Content: "Dry run note 2",
			},
		},
	}

	var buf bytes.Buffer
	_ = WriteEnvelope(&buf, env)

	rep, err := ImportWithOptions(ctx, st, bytes.NewReader(buf.Bytes()), ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ImportWithOptions failed: %v", err)
	}
	if rep.Total != 2 || rep.Imported != 2 || rep.Skipped != 0 || rep.Failed != 0 {
		t.Fatalf("unexpected dry-run report: %+v", rep)
	}

	// Verify store is still completely empty
	check, err := Export(ctx, st, ExportQuery{Scope: "project:dryrun"})
	if err != nil {
		t.Fatalf("Export check failed: %v", err)
	}
	if check.Total != 0 {
		t.Fatalf("store should have 0 memories after dry-run, got %d", check.Total)
	}
}

func TestWriteCSV(t *testing.T) {
	records := []MemoryRecord{
		{
			ID:            10,
			Scope:         "project:csv",
			Type:          "note",
			Content:       "CSV export note",
			Tags:          []string{"tag1", "tag2"},
			SourceAgent:   "agent1",
			SourceSession: "sess1",
			CreatedAt:     1789200000,
		},
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, records); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 CSV lines (header + 1 row), got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "id,scope,type,content,key,value_json,tags") {
		t.Fatalf("unexpected CSV header: %s", lines[0])
	}
	if !strings.Contains(lines[1], "CSV export note") || !strings.Contains(lines[1], "tag1,tag2") {
		t.Fatalf("unexpected CSV row: %s", lines[1])
	}
}

func TestImportRejectsUnsupportedType(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         "project:test",
		Total:         1,
		Memories: []MemoryRecord{
			{
				Scope:   "project:test",
				Type:    "unsupported_type",
				Content: "Some content",
			},
		},
	}
	var buf bytes.Buffer
	_ = WriteEnvelope(&buf, env)

	// In dry-run: should fail record upfront
	repDry, err := ImportWithOptions(ctx, st, bytes.NewReader(buf.Bytes()), ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ImportWithOptions dry-run failed: %v", err)
	}
	if repDry.Failed != 1 || repDry.Imported != 0 {
		t.Fatalf("expected dry-run failed=1 imported=0, got %+v", repDry)
	}

	// In normal import: should also fail record
	rep, err := Import(ctx, st, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if rep.Failed != 1 || rep.Imported != 0 {
		t.Fatalf("expected failed=1 imported=0, got %+v", rep)
	}
}

func TestImportEmptyContentAndFactSynthesis(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	env := Envelope{
		Format:        "centmem-export",
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Scope:         "project:test",
		Total:         2,
		Memories: []MemoryRecord{
			{
				Scope:     "project:test",
				Type:      "fact",
				Key:       "app.port",
				ValueJSON: "8080",
				Content:   "", // empty content: should be synthesized
			},
			{
				Scope:   "project:test",
				Type:    "note",
				Content: "   ", // empty whitespace content: should fail
			},
		},
	}
	var buf bytes.Buffer
	_ = WriteEnvelope(&buf, env)

	rep, err := Import(ctx, st, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if rep.Imported != 1 || rep.Failed != 1 {
		t.Fatalf("expected imported=1 failed=1, got %+v", rep)
	}
}
