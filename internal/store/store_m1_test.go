package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
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

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: dir + "/centmem.db"}

	s1, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.Close()

	s2, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()
}

func TestEnsureScopeCreatesAncestors(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// project:cent-mem/agent:y should create project:cent-mem and global too.
	sc, err := scope.Parse("project:cent-mem/agent:y")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	id, err := s.EnsureScope(ctx, sc)
	if err != nil {
		t.Fatalf("EnsureScope: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	// Verify global + project exist.
	db := s.DB()
	for _, path := range []string{"global", "project:cent-mem", "project:cent-mem/agent:y"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM scopes WHERE path = ?`, path).Scan(&n); err != nil {
			t.Fatalf("query %s: %v", path, err)
		}
		if n != 1 {
			t.Errorf("scope %q count = %d, want 1", path, n)
		}
	}
}

func TestPutMemoryDedup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	in := store.MemoryInput{
		Scope:   "project:cent-mem",
		Type:    "note",
		Content: "we chose sqlite-vec for local-first speed",
		Tags:    []string{"decision", "db"},
	}

	id1, status1, err := s.PutMemory(ctx, in)
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	if status1 != "queued" {
		t.Errorf("first status = %q, want queued", status1)
	}

	id2, status2, err := s.PutMemory(ctx, in)
	if err != nil {
		t.Fatalf("PutMemory (dup): %v", err)
	}
	if status2 != "merged" {
		t.Errorf("dup status = %q, want merged", status2)
	}
	if id2 != id1 {
		t.Errorf("dedup id = %d, want %d", id2, id1)
	}
}

func TestSetFactUpsert(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	in := store.FactInput{
		Scope: "project:cent-mem",
		Key:   "user.timezone",
		Value: `"Asia/Jakarta"`,
		Tags:  []string{"user"},
	}

	id1, status1, err := s.SetFact(ctx, in)
	if err != nil {
		t.Fatalf("SetFact: %v", err)
	}
	if status1 != "created" {
		t.Errorf("first status = %q, want created", status1)
	}

	id2, status2, err := s.SetFact(ctx, in)
	if err != nil {
		t.Fatalf("SetFact (dup): %v", err)
	}
	if status2 != "updated" {
		t.Errorf("dup status = %q, want updated", status2)
	}
	if id2 != id1 {
		t.Errorf("upsert id = %d, want %d", id2, id1)
	}

	// Only one fact row.
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM memories WHERE type='fact'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("fact row count = %d, want 1", n)
	}
}

func TestGetFactInherit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, _, err := s.SetFact(ctx, store.FactInput{Scope: "global", Key: "user.timezone", Value: `"Asia/Jakarta"`}); err != nil {
		t.Fatal(err)
	}

	// Not present in project scope, but inherited from global.
	f, err := s.GetFact(ctx, "project:cent-mem", "user.timezone", true)
	if err != nil {
		t.Fatalf("GetFact inherit: %v", err)
	}
	if f.ScopePath != "global" {
		t.Errorf("inherit returned scope %q, want global", f.ScopePath)
	}
	if f.Value != `"Asia/Jakarta"` {
		t.Errorf("value = %q, want %q", f.Value, `"Asia/Jakarta"`)
	}
}

func TestGetFactNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.GetFact(ctx, "project:cent-mem", "missing.key", true)
	if err != sql.ErrNoRows {
		t.Fatalf("GetFact missing err = %v, want sql.ErrNoRows", err)
	}
}

func TestFTS5SyncOnInsert(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "we deploy via github actions to fly",
	}); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memories_fts MATCH 'github'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("fts match count = %d, want 1", n)
	}
}

func TestList_FiltersAndPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seed := func(content string, tags []string) {
		t.Helper()
		if _, _, err := s.PutMemory(ctx, store.MemoryInput{
			Scope: "global", Type: "note", Content: content, Tags: tags,
		}); err != nil {
			t.Fatal(err)
		}
	}
	seed("alpha note", []string{"a"})
	seed("beta note", []string{"b"})
	seed("gamma note", []string{"a", "b"})

	sc, _ := scope.Parse("global")
	ids, err := s.ResolveScopeIDs(ctx, sc, true, false)
	if err != nil {
		t.Fatal(err)
	}

	// Type filter.
	mems, err := s.List(ctx, store.ListQuery{ScopeIDs: ids, Type: "note"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mems) != 3 {
		t.Errorf("list type=note count = %d, want 3", len(mems))
	}

	// Tag filter: tag "a".
	mems, err = s.List(ctx, store.ListQuery{ScopeIDs: ids, Tags: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(mems) != 2 {
		t.Errorf("list tags=[a] count = %d, want 2", len(mems))
	}

	// Pagination.
	mems, err = s.List(ctx, store.ListQuery{ScopeIDs: ids, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(mems) != 2 {
		t.Errorf("list limit=2 count = %d, want 2", len(mems))
	}
}

func TestForget_ByID_ByKey_ByTag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "forget by id"})
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.Forget(ctx, []int64{id}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("forget by id deleted = %d, want 1", n)
	}

	// by scope+key (fact)
	if _, _, err := s.SetFact(ctx, store.FactInput{Scope: "global", Key: "temp.key", Value: `"v"`}); err != nil {
		t.Fatal(err)
	}
	n, err = s.Forget(ctx, nil, strPtr("global"), strPtr("temp.key"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("forget by key deleted = %d, want 1", n)
	}

	// by scope+tag
	if _, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "tagged", Tags: []string{"junk"}}); err != nil {
		t.Fatal(err)
	}
	n, err = s.Forget(ctx, nil, strPtr("global"), nil, strPtr("junk"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("forget by tag deleted = %d, want 1", n)
	}
}

func TestAppendEvent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.AppendEvent(ctx, store.Event{
		MemoryID: 1, Op: "insert", ScopePath: "global",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("events count = %d, want 1", count)
	}
}

func TestStats(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "stats note"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetFact(ctx, store.FactInput{Scope: "global", Key: "a.key", Value: `"v"`}); err != nil {
		t.Fatal(err)
	}

	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Memories != 2 {
		t.Errorf("stats memories = %d, want 2", st.Memories)
	}
	if st.ByType["note"] != 1 || st.ByType["fact"] != 1 {
		t.Errorf("stats by_type = %v", st.ByType)
	}
	if st.PendingEmbedding != 2 {
		t.Errorf("pending embeddings = %d, want 2", st.PendingEmbedding)
	}
}

func TestResolveScopeIDs_Children(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sc, _ := scope.Parse("project:cent-mem")
	s.EnsureScope(ctx, sc)
	agentSc, _ := scope.Parse("project:cent-mem/agent:claude")
	s.EnsureScope(ctx, agentSc)

	ids, err := s.ResolveScopeIDs(ctx, sc, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("children resolve ids = %d, want 2 (project + agent)", len(ids))
	}
}

func strPtr(s string) *string { return &s }

func TestListScopeTree(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Put memories in different hierarchical scopes
	// 1 memory in global
	if _, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "global", Type: "note", Content: "global note"}); err != nil {
		t.Fatal(err)
	}
	// 2 memories in project:alpha
	if _, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha", Type: "note", Content: "alpha note 1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha", Type: "note", Content: "alpha note 2"}); err != nil {
		t.Fatal(err)
	}
	// 1 memory in project:alpha/agent:bot
	if _, _, err := s.PutMemory(ctx, store.MemoryInput{Scope: "project:alpha/agent:bot", Type: "note", Content: "bot note"}); err != nil {
		t.Fatal(err)
	}
	// Also ensure project:beta (0 memories)
	betaSc, _ := scope.Parse("project:beta")
	if _, err := s.EnsureScope(ctx, betaSc); err != nil {
		t.Fatal(err)
	}

	roots, err := s.ListScopeTree(ctx)
	if err != nil {
		t.Fatalf("ListScopeTree: %v", err)
	}

	if len(roots) != 1 {
		t.Fatalf("expected 1 root (global), got %d", len(roots))
	}

	g := roots[0]
	if g.Path != "global" {
		t.Errorf("root path = %q, want 'global'", g.Path)
	}
	if g.Count != 1 {
		t.Errorf("global direct count = %d, want 1", g.Count)
	}
	// Total count for global: 1 (global) + 2 (alpha) + 1 (bot) + 0 (beta) = 4
	if g.TotalCount != 4 {
		t.Errorf("global total_count = %d, want 4", g.TotalCount)
	}

	// Children of global should be project:alpha and project:beta
	if len(g.Children) != 2 {
		t.Fatalf("global children count = %d, want 2", len(g.Children))
	}

	alpha := g.Children[0]
	if alpha.Path != "project:alpha" {
		t.Errorf("child 0 = %q, want 'project:alpha'", alpha.Path)
	}
	if alpha.Count != 2 {
		t.Errorf("alpha direct count = %d, want 2", alpha.Count)
	}
	if alpha.TotalCount != 3 { // 2 + 1 child
		t.Errorf("alpha total_count = %d, want 3", alpha.TotalCount)
	}

	if len(alpha.Children) != 1 {
		t.Fatalf("alpha children count = %d, want 1", len(alpha.Children))
	}
	bot := alpha.Children[0]
	if bot.Path != "project:alpha/agent:bot" {
		t.Errorf("bot path = %q, want 'project:alpha/agent:bot'", bot.Path)
	}
	if bot.Count != 1 || bot.TotalCount != 1 {
		t.Errorf("bot count = %d, total = %d, want 1, 1", bot.Count, bot.TotalCount)
	}

	beta := g.Children[1]
	if beta.Path != "project:beta" {
		t.Errorf("child 1 = %q, want 'project:beta'", beta.Path)
	}
	if beta.Count != 0 || beta.TotalCount != 0 {
		t.Errorf("beta count = %d, total = %d, want 0, 0", beta.Count, beta.TotalCount)
	}
}
