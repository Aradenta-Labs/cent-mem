package agent_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func setupTestStoreAndSearcher(t *testing.T) (*store.Store, *search.Searcher) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := config.Config{
		Home:   t.TempDir(),
		DBPath: dbPath,
	}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	searcher := search.New(s)
	return s, searcher
}

func TestToolRegistry_DefinitionsAndDispatch(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	reg := agent.DefaultToolRegistry(s, searcher)

	defs := reg.Definitions()
	if len(defs) != 6 {
		t.Fatalf("expected 6 tool definitions, got %d", len(defs))
	}

	names := make(map[string]bool)
	for _, d := range defs {
		if d.Type != "function" {
			t.Errorf("expected type 'function', got %q", d.Type)
		}
		names[d.Function.Name] = true
	}

	expected := []string{
		"search_memories",
		"read_memory",
		"inspect_links",
		"propose_link",
		"propose_merge",
		"detect_knowledge_gaps",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool in registry: %s", name)
		}
	}

	// Unknown tool
	_, err := reg.Execute(context.Background(), "unknown_tool", "{}")
	if err == nil {
		t.Errorf("expected error for unknown tool, got nil")
	}
}

func TestTools_Execution(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	// Seed memories
	m1ID, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:demo",
		Type:    "note",
		Content: "Use PostgreSQL for relational storage",
		Tags:    []string{"database", "sql"},
	})
	if err != nil {
		t.Fatalf("PutMemory 1: %v", err)
	}

	m2ID, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:demo",
		Type:    "note",
		Content: "Use SQLite for embedded offline storage",
		Tags:    []string{"database", "sqlite"},
	})
	if err != nil {
		t.Fatalf("PutMemory 2: %v", err)
	}

	reg := agent.DefaultToolRegistry(s, searcher)

	// 1. search_memories
	searchOut, err := reg.Execute(ctx, "search_memories", `{"query":"PostgreSQL"}`)
	if err != nil {
		t.Fatalf("search_memories: %v", err)
	}
	var searchItems []map[string]any
	if err := json.Unmarshal([]byte(searchOut), &searchItems); err != nil {
		t.Fatalf("unmarshal searchOut: %v", err)
	}
	if len(searchItems) == 0 {
		t.Errorf("expected at least 1 search result, got %d", len(searchItems))
	}

	// 2. read_memory
	readOut, err := reg.Execute(ctx, "read_memory", `{"id":`+fmtInt(m1ID)+`}`)
	if err != nil {
		t.Fatalf("read_memory: %v", err)
	}
	var readData map[string]any
	if err := json.Unmarshal([]byte(readOut), &readData); err != nil {
		t.Fatalf("unmarshal readData: %v", err)
	}
	if readData["content"] != "Use PostgreSQL for relational storage" {
		t.Errorf("readData content mismatch: %v", readData["content"])
	}

	// 3. propose_link
	linkArgs := `{"from_id":` + fmtInt(m2ID) + `,"to_id":` + fmtInt(m1ID) + `,"relation":"supersedes","reasoning":"SQLite replaces Postgres for local deployment"}`
	linkOut, err := reg.Execute(ctx, "propose_link", linkArgs)
	if err != nil {
		t.Fatalf("propose_link: %v", err)
	}
	var linkRes struct {
		OK         bool  `json:"ok"`
		ProposalID int64 `json:"proposal_id"`
	}
	if err := json.Unmarshal([]byte(linkOut), &linkRes); err != nil || !linkRes.OK || linkRes.ProposalID <= 0 {
		t.Fatalf("unexpected link response: %s", linkOut)
	}

	// Verify proposal exists in store
	prop, err := s.GetProposal(ctx, linkRes.ProposalID)
	if err != nil || prop.ProposalType != "link" {
		t.Errorf("proposal verification failed: %v, %+v", err, prop)
	}

	// 4. propose_merge
	mergeArgs := `{"source_ids":[` + fmtInt(m1ID) + `,` + fmtInt(m2ID) + `],"title":"Database Strategy","content":"Consolidated DB Decision","reasoning":"Merge PG and SQLite notes"}`
	mergeOut, err := reg.Execute(ctx, "propose_merge", mergeArgs)
	if err != nil {
		t.Fatalf("propose_merge: %v", err)
	}
	var mergeRes struct {
		OK         bool  `json:"ok"`
		ProposalID int64 `json:"proposal_id"`
	}
	if err := json.Unmarshal([]byte(mergeOut), &mergeRes); err != nil || !mergeRes.OK || mergeRes.ProposalID <= 0 {
		t.Fatalf("unexpected merge response: %s", mergeOut)
	}

	// 5. inspect_links
	inspectOut, err := reg.Execute(ctx, "inspect_links", `{"id":`+fmtInt(m1ID)+`}`)
	if err != nil {
		t.Fatalf("inspect_links: %v", err)
	}
	var inspectData map[string]any
	if err := json.Unmarshal([]byte(inspectOut), &inspectData); err != nil {
		t.Fatalf("unmarshal inspectData: %v", err)
	}
	if inspectData["memory_id"] == nil {
		t.Errorf("inspectData missing memory_id: %v", inspectData)
	}

	// 6. detect_knowledge_gaps
	gapOut, err := reg.Execute(ctx, "detect_knowledge_gaps", `{"topic":"Kubernetes deployment","scope":"project:demo"}`)
	if err != nil {
		t.Fatalf("detect_knowledge_gaps: %v", err)
	}
	var gapData map[string]any
	if err := json.Unmarshal([]byte(gapOut), &gapData); err != nil {
		t.Fatalf("unmarshal gapData: %v", err)
	}
	if gapData["coverage"] != "missing" {
		t.Errorf("expected missing coverage for unrecorded topic, got %v", gapData["coverage"])
	}
}

func fmtInt(n int64) string {
	return strconv.FormatInt(n, 10)
}
