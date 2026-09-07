package store_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "test.db")}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStore_RecordAccess_Basic(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "test note 1",
	})
	if err != nil {
		t.Fatalf("put note 1: %v", err)
	}

	id2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "test note 2",
	})
	if err != nil {
		t.Fatalf("put note 2: %v", err)
	}

	// Verify initial state: access_count = 0, last_accessed_at = nil
	m1, err := s.GetMemory(ctx, id1)
	if err != nil {
		t.Fatalf("get note 1: %v", err)
	}
	if m1.AccessCount != 0 {
		t.Errorf("initial access_count = %d, want 0", m1.AccessCount)
	}
	if m1.LastAccessedAt != nil {
		t.Errorf("initial last_accessed_at = %v, want nil", m1.LastAccessedAt)
	}

	// Record access for id1 twice, id2 once
	before := time.Now().Add(-1 * time.Second)
	if err := s.RecordAccess(ctx, []int64{id1, id2}); err != nil {
		t.Fatalf("record access 1: %v", err)
	}
	if err := s.RecordAccess(ctx, []int64{id1, id1}); err != nil { // test deduplication in single call
		t.Fatalf("record access 2: %v", err)
	}

	m1After, err := s.GetMemory(ctx, id1)
	if err != nil {
		t.Fatalf("get note 1 after: %v", err)
	}
	if m1After.AccessCount != 2 {
		t.Errorf("m1 access_count = %d, want 2", m1After.AccessCount)
	}
	if m1After.LastAccessedAt == nil || m1After.LastAccessedAt.Before(before) {
		t.Errorf("m1 last_accessed_at = %v, want >= %v", m1After.LastAccessedAt, before)
	}

	m2After, err := s.GetMemory(ctx, id2)
	if err != nil {
		t.Fatalf("get note 2 after: %v", err)
	}
	if m2After.AccessCount != 1 {
		t.Errorf("m2 access_count = %d, want 1", m2After.AccessCount)
	}

	// Also verify List returns access_count
	listed, err := s.List(ctx, store.ListQuery{ScopePath: "global"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) < 2 {
		t.Fatalf("list returned %d, want >= 2", len(listed))
	}
	for _, m := range listed {
		if m.ID == id1 && m.AccessCount != 2 {
			t.Errorf("listed m1 access_count = %d, want 2", m.AccessCount)
		}
		if m.ID == id2 && m.AccessCount != 1 {
			t.Errorf("listed m2 access_count = %d, want 1", m.AccessCount)
		}
	}
}

func TestStore_RecordAccess_Concurrent50(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "concurrent access note",
	})
	if err != nil {
		t.Fatalf("put note: %v", err)
	}

	const goroutines = 50
	const iterationsPerGoroutine = 5
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterationsPerGoroutine; i++ {
				if err := s.RecordAccess(context.Background(), []int64{id}); err != nil {
					t.Errorf("concurrent RecordAccess error: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	m, err := s.GetMemory(ctx, id)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	wantCount := goroutines * iterationsPerGoroutine
	if m.AccessCount != wantCount {
		t.Errorf("final access_count = %d, want %d", m.AccessCount, wantCount)
	}
}

func TestStore_RecordAccessAsync_WaitedOnClose(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "async.db")}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ctx := context.Background()
	id, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "async note",
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	// Trigger async record
	s.RecordAccessAsync([]int64{id})

	// Close must wait for the async goroutine to finish and commit
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen and check
	s2, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	m, err := s2.GetMemory(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m.AccessCount != 1 {
		t.Errorf("access_count after reopen = %d, want 1", m.AccessCount)
	}
}

func TestStore_ImportanceDistribution(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Put 4 active memories
	// m0: 0 accesses (zero_access)
	// m1: 3 accesses (low_access_1_5)
	// m2: 10 accesses (medium_access_6_20)
	// m3: 25 accesses (high_access_21_plus)
	id0, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "zero"})
	id1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "low"})
	id2, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "med"})
	id3, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "high"})

	for i := 0; i < 3; i++ {
		_ = s.RecordAccess(ctx, []int64{id1})
	}
	for i := 0; i < 10; i++ {
		_ = s.RecordAccess(ctx, []int64{id2})
	}
	for i := 0; i < 25; i++ {
		_ = s.RecordAccess(ctx, []int64{id3})
	}

	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	dist := stats.ImportanceDistribution
	if dist.ZeroAccess != 1 {
		t.Errorf("ZeroAccess = %d, want 1", dist.ZeroAccess)
	}
	if dist.LowAccess1to5 != 1 {
		t.Errorf("LowAccess1to5 = %d, want 1", dist.LowAccess1to5)
	}
	if dist.MedAccess6to20 != 1 {
		t.Errorf("MedAccess6to20 = %d, want 1", dist.MedAccess6to20)
	}
	if dist.HighAccess21Plus != 1 {
		t.Errorf("HighAccess21Plus = %d, want 1", dist.HighAccess21Plus)
	}
	if dist.MaxAccessCount != 25 {
		t.Errorf("MaxAccessCount = %d, want 25", dist.MaxAccessCount)
	}
	expectedAvg := float64(0+3+10+25) / 4.0 // 38 / 4 = 9.5
	if dist.AvgAccessCount != expectedAvg {
		t.Errorf("AvgAccessCount = %f, want %f", dist.AvgAccessCount, expectedAvg)
	}
	_ = id0
}
