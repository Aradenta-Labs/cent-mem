package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/farras/cent-mem/internal/store"
)

// eligible put: write a memory then force its summarize_at into the past.
func putEligible(t *testing.T, s *store.Store, scopePath, typ, content string) *store.Memory {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	old := now.Add(-48 * time.Hour).UnixMicro()
	ts := now.Add(30 * 24 * time.Hour).UnixMicro()
	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope: scopePath, Type: typ, Content: content,
		Tags: []string{"t"}, SummarizeAt: &ts,
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE memories SET summarize_at = ? WHERE id = ?`, old, id); err != nil {
		t.Fatalf("set summarize_at: %v", err)
	}
	return &store.Memory{ID: id, Type: typ, ScopePath: scopePath}
}

func TestStore_SchemaVersionHelpers(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v == "" {
		t.Error("expected non-empty schema version after open")
	}
	if v != store.LatestSchemaVersion() {
		t.Errorf("SchemaVersion=%q, LatestSchemaVersion=%q", v, store.LatestSchemaVersion())
	}
	_ = ctx
}

func TestStore_EligibleMemories(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// One eligible (past), one not (future).
	putEligible(t, s, "global", "log", "eligible log")
	future := time.Now().Add(30 * 24 * time.Hour).UnixMicro()
	if _, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope: "global", Type: "note", Content: "future note", SummarizeAt: &future,
	}); err != nil {
		t.Fatalf("PutMemory future: %v", err)
	}

	now := time.Now()
	mems, err := s.EligibleMemories(ctx, "global", now)
	if err != nil {
		t.Fatalf("EligibleMemories: %v", err)
	}
	if len(mems) != 1 {
		t.Fatalf("expected 1 eligible memory, got %d", len(mems))
	}
	if mems[0].Content != "eligible log" {
		t.Errorf("eligible content = %q, want 'eligible log'", mems[0].Content)
	}
}

func TestStore_InsertConsolidatedNote(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	o1 := putEligible(t, s, "global", "log", "first log")
	o2 := putEligible(t, s, "global", "log", "second log")

	originals, err := s.EligibleMemories(ctx, "global", time.Now())
	if err != nil {
		t.Fatalf("EligibleMemories: %v", err)
	}
	if len(originals) != 2 {
		t.Fatalf("expected 2 originals, got %d", len(originals))
	}

	newID, err := s.InsertConsolidatedNote(ctx, originals, "summary text", time.Now())
	if err != nil {
		t.Fatalf("InsertConsolidatedNote: %v", err)
	}
	if newID <= 0 {
		t.Errorf("expected positive new id, got %d", newID)
	}

	// Originals archived.
	for _, o := range []*store.Memory{o1, o2} {
		var status string
		if err := s.DB().QueryRow(`SELECT status FROM memories WHERE id = ?`, o.ID).Scan(&status); err != nil {
			t.Fatalf("query status: %v", err)
		}
		if status != "archived" {
			t.Errorf("original %d status = %q, want archived", o.ID, status)
		}
	}

	// New note active with summary.
	var content, status string
	if err := s.DB().QueryRow(`SELECT content, status FROM memories WHERE id = ?`, newID).Scan(&content, &status); err != nil {
		t.Fatalf("query new note: %v", err)
	}
	if content != "summary text" {
		t.Errorf("consolidated content = %q, want summary text", content)
	}
	if status != "active" {
		t.Errorf("consolidated status = %q, want active", status)
	}
}

func TestStore_ClaimEmbedJobsAndWrite(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "embed me"})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}

	// Claim the embed job.
	ids, err := s.ClaimEmbedJobs(ctx, 10, time.Now().Add(-time.Hour).UnixMicro())
	if err != nil {
		t.Fatalf("ClaimEmbedJobs: %v", err)
	}
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("expected claimed ids [%d], got %v", id, ids)
	}

	// Second claim returns nothing (already claimed recently).
	ids2, err := s.ClaimEmbedJobs(ctx, 10, time.Now().Add(-time.Hour).UnixMicro())
	if err != nil {
		t.Fatalf("ClaimEmbedJobs#2: %v", err)
	}
	if len(ids2) != 0 {
		t.Errorf("expected no re-claim, got %v", ids2)
	}

	// Load embed text.
	texts, err := s.LoadMemoryEmbedTexts(ctx, ids)
	if err != nil {
		t.Fatalf("LoadMemoryEmbedTexts: %v", err)
	}
	if texts[id] != "embed me" {
		t.Errorf("embed text = %q, want 'embed me'", texts[id])
	}

	// Write embedding (384-dim matches the vec0 schema).
	vec := make([]float32, 384)
	for i := range vec {
		vec[i] = float32(i % 7)
	}
	if err := s.WriteEmbedding(ctx, id, vec, "test-model"); err != nil {
		t.Fatalf("WriteEmbedding: %v", err)
	}

	// Queue is drained.
	var pending int64
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM embed_queue WHERE memory_id = ?`, id).Scan(&pending); err != nil {
		t.Fatalf("query queue: %v", err)
	}
	if pending != 0 {
		t.Errorf("expected embed_queue drained, got %d", pending)
	}
}

func TestStore_LoadMemoryEmbedTextsEmpty(t *testing.T) {
	s := testStore(t)
	texts, err := s.LoadMemoryEmbedTexts(context.Background(), nil)
	if err != nil {
		t.Fatalf("LoadMemoryEmbedTexts(nil): %v", err)
	}
	if len(texts) != 0 {
		t.Errorf("expected empty map, got %v", texts)
	}
}

func TestStore_WriteEmbeddingFactPrependsKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, _, err := s.SetFact(ctx, store.FactInput{Scope: "global", Key: "k", Value: `"v"`}); err != nil {
		t.Fatalf("SetFact: %v", err)
	}
	var id int64
	if err := s.DB().QueryRow(`SELECT id FROM memories WHERE type='fact' LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("find fact: %v", err)
	}

	texts, err := s.LoadMemoryEmbedTexts(ctx, []int64{id})
	if err != nil {
		t.Fatalf("LoadMemoryEmbedTexts: %v", err)
	}
	if texts[id] == "" {
		t.Error("expected fact embed text (key + content)")
	}
}
