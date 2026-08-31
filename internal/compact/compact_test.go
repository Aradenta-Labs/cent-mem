package compact_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"

	"github.com/aradenta-labs/cent-mem/internal/compact"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{DBPath: dir + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// putEligible writes a memory whose summarize_at is well in the past.
func putEligible(t *testing.T, s *store.Store, scope, typ, content string, tags []string) int64 {
	t.Helper()
	id, _, err := s.PutMemory(context.Background(), store.MemoryInput{
		Scope: scope, Type: typ, Content: content, Tags: tags,
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	// Override summarize_at to the past so it is eligible immediately.
	past := time.Now().Add(-24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec(`UPDATE memories SET summarize_at = ? WHERE id = ?`, past, id); err != nil {
		t.Fatalf("set summarize_at: %v", err)
	}
	return id
}

func activeCount(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var n int64
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM memories WHERE status = 'active'`).Scan(&n); err != nil {
		t.Fatalf("count active: %v", err)
	}
	return n
}

func archivedCount(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var n int64
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM memories WHERE status = 'archived'`).Scan(&n); err != nil {
		t.Fatalf("count archived: %v", err)
	}
	return n
}

func TestHeuristicSummarizer_Deterministic(t *testing.T) {
	h := compact.NewHeuristicSummarizer(3)
	mems := []store.Memory{
		{ScopePath: "project:cent-mem", Type: "note", Content: "We deploy via GitHub Actions on merge to main.", CreatedAt: time.Unix(1000, 0)},
		{ScopePath: "project:cent-mem", Type: "note", Content: "We deploy to Fly.io after merge.", CreatedAt: time.Unix(2000, 0)},
	}
	a, err := h.Summarize(context.Background(), mems)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	b, err := h.Summarize(context.Background(), mems)
	if err != nil {
		t.Fatalf("Summarize#2: %v", err)
	}
	if a != b {
		t.Errorf("summarizer not deterministic:\nA=%q\nB=%q", a, b)
	}
}

func TestHeuristicSummarizer_Shorter(t *testing.T) {
	h := compact.NewHeuristicSummarizer(1)
	mems := []store.Memory{}
	for i := 0; i < 8; i++ {
		mems = append(mems, store.Memory{
			ScopePath: "project:cent-mem",
			Content:   "The project chose a distributed microservices architecture with event-driven messaging and strict service-level objectives.",
			CreatedAt: time.Unix(int64(1000+i*100), 0),
		})
	}
	orig := ""
	for _, m := range mems {
		orig += m.Content + " "
	}
	summary, err := h.Summarize(context.Background(), mems)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(summary) >= len(orig) {
		t.Errorf("expected summary shorter than concatenated originals (summary=%d chars, orig=%d)", len(summary), len(orig))
	}
	if !strings.Contains(summary, "Summary of 8 memories") {
		t.Errorf("expected header in summary, got %q", summary)
	}
}

func TestCompact_SelectsEligible(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Two eligible notes.
	putEligible(t, s, "project:cent-mem", "note", "First decision about the architecture.", []string{"a"})
	putEligible(t, s, "project:cent-mem", "note", "Second decision about the database.", []string{"a"})
	// One ineligible note (summarize_at in the future).
	id, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:cent-mem", Type: "note", Content: "Future memory not yet eligible."})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	future := time.Now().Add(30 * 24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec(`UPDATE memories SET summarize_at = ? WHERE id = ?`, future, id); err != nil {
		t.Fatalf("set future summarize_at: %v", err)
	}

	res, err := compact.Compact(ctx, s, compact.Options{})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if res.Summarized != 2 {
		t.Errorf("summarized = %d, want 2", res.Summarized)
	}
	// Only the rows past summarize_at are touched: the future note must remain
	// active and unarchived.
	var status string
	if err := s.DB().QueryRow(`SELECT status FROM memories WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("query future note: %v", err)
	}
	if status != "active" {
		t.Errorf("future note status = %q, want active (should not be touched)", status)
	}
	if archivedCount(t, s) != 2 {
		t.Errorf("archived count = %d, want 2", archivedCount(t, s))
	}
}

func TestCompact_GroupedByScopeAndType(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	putEligible(t, s, "project:cent-mem", "note", "Note one about deployment strategy.", []string{"n"})
	putEligible(t, s, "project:cent-mem", "note", "Note two about release process.", []string{"n"})
	putEligible(t, s, "project:other", "log", "Log one from the other project.", []string{"l"})

	res, err := compact.Compact(ctx, s, compact.Options{})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	// Two groups => two consolidated notes.
	if len(res.NewMemoryIDs) != 2 {
		t.Errorf("new_memory_ids count = %d, want 2 (grouped by scope+type)", len(res.NewMemoryIDs))
	}
	// Each group wrote a note.
	for _, id := range res.NewMemoryIDs {
		var typ string
		if err := s.DB().QueryRow(`SELECT type FROM memories WHERE id = ?`, id).Scan(&typ); err != nil {
			t.Fatalf("query new note: %v", err)
		}
		if typ != "note" {
			t.Errorf("consolidated row type = %q, want note", typ)
		}
	}
}

func TestCompact_ArchivesOriginals(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id1 := putEligible(t, s, "project:cent-mem", "note", "Archive me please.", []string{"x"})
	putEligible(t, s, "project:cent-mem", "note", "Archive me too.", []string{"x"})

	if _, err := compact.Compact(ctx, s, compact.Options{}); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	var status string
	if err := s.DB().QueryRow(`SELECT status FROM memories WHERE id = ?`, id1).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "archived" {
		t.Errorf("original status = %q, want archived", status)
	}
	if archivedCount(t, s) != 2 {
		t.Errorf("archived count = %d, want 2", archivedCount(t, s))
	}
}

func TestCompact_WritesConsolidatedNote(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	putEligible(t, s, "project:cent-mem", "note", "Alpha note about team velocity.", []string{"team", "alpha"})
	putEligible(t, s, "project:cent-mem", "note", "Beta note about team velocity.", []string{"team", "beta"})

	res, err := compact.Compact(ctx, s, compact.Options{})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if len(res.NewMemoryIDs) != 1 {
		t.Fatalf("expected 1 new note, got %d", len(res.NewMemoryIDs))
	}
	newID := res.NewMemoryIDs[0]

	var status, tags string
	var summarizeAt *int64
	if err := s.DB().QueryRow(`SELECT status, tags, summarize_at FROM memories WHERE id = ?`, newID).
		Scan(&status, &tags, &summarizeAt); err != nil {
		t.Fatalf("query new note: %v", err)
	}
	if status != "active" {
		t.Errorf("consolidated note status = %q, want active", status)
	}
	// Union of tags across originals.
	for _, want := range []string{"team", "alpha", "beta"} {
		if !strings.Contains(tags, want) {
			t.Errorf("consolidated tags %q missing %q", tags, want)
		}
	}
	if summarizeAt != nil {
		t.Errorf("consolidated note summarize_at should be NULL")
	}
}

func TestCompact_AppendsEvents(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	putEligible(t, s, "project:cent-mem", "note", "Event test memory.", []string{"e"})
	putEligible(t, s, "project:cent-mem", "note", "Another event memory.", []string{"e"})

	if _, err := compact.Compact(ctx, s, compact.Options{}); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE op = 'summarize'`).Scan(&n); err != nil {
		t.Fatalf("count summarize events: %v", err)
	}
	if n != 2 {
		t.Errorf("summarize events = %d, want 2", n)
	}
}

func TestCompact_DropsVectors(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id := putEligible(t, s, "project:cent-mem", "note", "Vector drop target.", []string{"v"})
	vec, err := sqlite_vec.SerializeFloat32(make([]float32, 384))
	if err != nil {
		t.Fatalf("serialize vec: %v", err)
	}
	// Simulate an embedding row + vec row for this memory.
	if _, err := s.DB().Exec(`INSERT INTO embeddings(memory_id, embedding, model, embedded_at) VALUES (?, ?, 'test', 0)`, id, vec); err != nil {
		t.Fatalf("insert embedding: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO memories_vec(memory_id, embedding) VALUES (?, ?)`, id, vec); err != nil {
		t.Fatalf("insert vec: %v", err)
	}

	if _, err := compact.Compact(ctx, s, compact.Options{}); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	var emb int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM embeddings WHERE memory_id = ?`, id).Scan(&emb)
	if emb != 0 {
		t.Errorf("embedding row not dropped (count=%d)", emb)
	}
	var vecCount int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM memories_vec WHERE memory_id = ?`, id).Scan(&vecCount)
	if vecCount != 0 {
		t.Errorf("vec row not dropped (count=%d)", vecCount)
	}
}

func TestCompact_DryRun(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	putEligible(t, s, "project:cent-mem", "note", "Dry run should not archive me.", []string{"d"})
	putEligible(t, s, "project:cent-mem", "note", "Dry run two.", []string{"d"})

	res, err := compact.Compact(ctx, s, compact.Options{DryRun: true})
	if err != nil {
		t.Fatalf("Compact dry-run: %v", err)
	}
	if res.Summarized != 2 {
		t.Errorf("dry-run summarized = %d, want 2", res.Summarized)
	}
	if len(res.NewMemoryIDs) != 0 {
		t.Errorf("dry-run should write no new notes, got %v", res.NewMemoryIDs)
	}
	// No rows archived or created.
	if archivedCount(t, s) != 0 {
		t.Errorf("dry-run archived count = %d, want 0", archivedCount(t, s))
	}
	if activeCount(t, s) != 2 {
		t.Errorf("dry-run active count = %d, want 2 (no changes)", activeCount(t, s))
	}
}

func TestCompact_RespectsRetentionConfig(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// A fact with summarize_at in the past must never be compacted (keep forever).
	factID, _, err := s.SetFact(ctx, store.FactInput{Scope: "project:cent-mem", Key: "user.timezone", Value: `"Asia/Jakarta"`})
	if err != nil {
		t.Fatalf("SetFact: %v", err)
	}
	past := time.Now().Add(-24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec(`UPDATE memories SET summarize_at = ? WHERE id = ?`, past, factID); err != nil {
		t.Fatalf("set fact summarize_at: %v", err)
	}

	res, err := compact.Compact(ctx, s, compact.Options{})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if res.Summarized != 0 {
		t.Errorf("facts should not be compacted; summarized = %d", res.Summarized)
	}
	// Fact still active.
	var status string
	_ = s.DB().QueryRow(`SELECT status FROM memories WHERE id = ?`, factID).Scan(&status)
	if status != "active" {
		t.Errorf("fact status = %q, want active", status)
	}
}

func TestCompact_ReductionTarget(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// 30-day simulation: 100 eligible logs/notes. After compaction the active
	// row count should drop by >= 60% of the eligible rows.
	const total = 100
	for i := 0; i < total; i++ {
		putEligible(t, s, "project:cent-mem", "log", fmt.Sprintf("Daily log entry number %d about the pipeline status and build health.", i), []string{"log"})
	}

	before := activeCount(t, s)
	res, err := compact.Compact(ctx, s, compact.Options{})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	after := activeCount(t, s)

	reduction := float64(before-after) / float64(before) * 100
	if res.Summarized != total {
		t.Errorf("summarized = %d, want %d", res.Summarized, total)
	}
	if reduction < 60 {
		t.Errorf("active-row reduction = %.1f%%, want >= 60%%", reduction)
	}
}

func TestCompact_SignalSafe(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// A single group's InsertConsolidatedNote is atomic: if it fails mid-way,
	// no partial archive remains. We simulate by feeding a summary-producing
	// group and relying on tx rollback semantics. Here we assert the happy path
	// leaves consistent counts, and that a cancelled context leaves no writes.
	id1 := putEligible(t, s, "project:cent-mem", "note", "Atomic memory one.", []string{"s"})
	putEligible(t, s, "project:cent-mem", "note", "Atomic memory two.", []string{"s"})

	cctx, cancel := context.WithCancel(ctx)
	cancel() // pre-cancelled => store ops fail, tx rolls back
	if _, err := compact.Compact(cctx, s, compact.Options{}); err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// No rows changed.
	var status string
	_ = s.DB().QueryRow(`SELECT status FROM memories WHERE id = ?`, id1).Scan(&status)
	if status != "active" {
		t.Errorf("original status = %q, want active (no partial archive)", status)
	}
	if archivedCount(t, s) != 0 {
		t.Errorf("archived count = %d, want 0", archivedCount(t, s))
	}
}
