package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// TestE2E_ParaphraseRetrieval writes a memory and verifies a semantically
// related query (sharing few keywords) retrieves it via the full CLI path.
func TestE2E_ParaphraseRetrieval(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, _, code := runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "we deploy via github actions to fly.io", "--tags", "deploy,ci")
	if code != 0 {
		t.Fatalf("put exit code = %d, want 0", code)
	}

	stdout, _, code := runCLI(t, home, "recall", "how do we ship?", "--scope", "global", "--top", "5")
	if code != 0 {
		t.Fatalf("recall exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	results, ok := m["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", m["results"])
	}
	found := false
	for _, r := range results {
		item := r.(map[string]any)
		if strings.Contains(item["content"].(string), "deploy via github actions") {
			found = true
			if matched, ok := item["matched_by"].([]any); !ok || !sliceHas(matched, "semantic") {
				t.Errorf("expected matched_by to include semantic, got %v", item["matched_by"])
			}
		}
	}
	if !found {
		t.Fatal("expected the deploy memory in paraphrase results")
	}
}

// TestE2E_InlineDrain writes 50 memories and verifies pending_embeddings
// returns to 0 within 5s (inline drain on write).
func TestE2E_InlineDrain(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	for i := 0; i < 50; i++ {
		_, _, code := runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", fmt.Sprintf("inline drain memory number %d unique", i))
		if code != 0 {
			t.Fatalf("put#%d exit code = %d, want 0", i, code)
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		stdout, _, code := runCLI(t, home, "stats")
		if code != 0 {
			t.Fatalf("stats exit code = %d, want 0", code)
		}
		st := parseJSON(t, stdout)
		pending, _ := st["pending_embeddings"].(float64)
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending_embeddings still %v after 5s", pending)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestE2E_RecallIsFast seeds 10k memories and verifies 100 recall calls have a
// p95 under 300ms (per the M2 performance target).
func TestE2E_RecallIsFast(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(config.Config{DBPath: dir + "/centmem.db"})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	for i := 0; i < 10000; i++ {
		if _, _, err := s.PutMemory(ctx, store.MemoryInput{
			Scope: "global", Type: "note",
			Content: fmt.Sprintf("deployment topic number %d for fast recall", i),
		}); err != nil {
			t.Fatalf("PutMemory: %v", err)
		}
	}
	q := embed.NewQueue(s, embed.NewStub(384))
	q.MaxTime = time.Hour
	if _, err := q.Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	se := search.New(s).WithEmbedder(embed.NewStub(384))

	const calls = 100
	durs := make([]time.Duration, 0, calls)
	for i := 0; i < calls; i++ {
		start := time.Now()
		if _, err := se.Recall(ctx, search.Query{Text: "how do we deploy topic 5000", Top: 5, Scope: "global", Inherit: true}); err != nil {
			t.Fatalf("Recall: %v", err)
		}
		durs = append(durs, time.Since(start))
	}
	sortDur(durs)
	p95 := durs[len(durs)*95/100]
	if p95 > 300*time.Millisecond {
		t.Errorf("recall p95 = %v, want < 300ms", p95)
	}
}

// TestE2E_RecollisionAfterRestart simulates a crash mid-drain: rows are claimed
// with an old timestamp, then the queue is drained again (the "rerun") and the
// stale claims are recovered and embedded.
func TestE2E_RecollisionAfterRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(config.Config{DBPath: dir + "/centmem.db"})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// Seed 5 memories.
	ids := make([]int64, 5)
	for i := range ids {
		id, _, err := s.PutMemory(ctx, store.MemoryInput{
			Scope: "global", Type: "note", Content: fmt.Sprintf("crash recovery row %d", i),
		})
		if err != nil {
			t.Fatalf("PutMemory: %v", err)
		}
		ids[i] = id
	}

	// Simulate a crash: mark all queue rows as claimed long ago (stale).
	if _, err := s.DB().Exec(`UPDATE embed_queue SET claimed_at = ?`, time.Now().Add(-10*time.Minute).UnixMicro()); err != nil {
		t.Fatalf("mark stale: %v", err)
	}

	// "Rerun" the CLI drain: a fresh queue drains the stale rows.
	q := embed.NewQueue(s, embed.NewStub(384))
	q.MaxTime = time.Hour
	n, err := q.Drain(ctx)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if n != len(ids) {
		t.Errorf("drained %d, want %d", n, len(ids))
	}
	// Embeddings should now exist for all rows.
	var pending int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM embed_queue`).Scan(&pending); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 0 {
		t.Errorf("pending after rerun = %d, want 0", pending)
	}
}

// TestE2E_NoNetwork verifies the embedder and recall still work when no model
// file exists (fully offline): embed.New falls back to the stub embedder.
func TestE2E_NoNetwork(t *testing.T) {
	// Use a home with no model file at all (no stubDownloader, no init).
	dir := t.TempDir()
	s, err := store.Open(config.Config{DBPath: dir + "/centmem.db"})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if _, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope: "global", Type: "note", Content: "offline memory about logging",
	}); err != nil {
		t.Fatalf("PutMemory: %v", err)
	}

	// No model file present -> embed.New must still return a working embedder.
	emb, err := embed.New(dir+"/no-model.onnx", 384, "")
	if err != nil {
		t.Fatalf("embed.New: %v", err)
	}
	defer emb.Close()
	vecs, err := emb.Embed(ctx, []string{"offline query about logging"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 384 {
		t.Fatalf("embedding shape = %d x %d, want 1 x 384", len(vecs), len(vecs[0]))
	}

	q := embed.NewQueue(s, emb)
	q.MaxTime = time.Hour
	if _, err := q.Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	se := search.New(s).WithEmbedder(emb)
	res, err := se.Recall(ctx, search.Query{Text: "offline query about logging", Top: 5, Scope: "global", Inherit: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected recall result with offline embedder")
	}
}

func sliceHas(ss []any, want string) bool {
	for _, v := range ss {
		if v == want {
			return true
		}
	}
	return false
}

func sortDur(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j] < d[j-1]; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}
