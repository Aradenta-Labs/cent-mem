package search_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func newSearchTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		Home:   dir,
		DBPath: filepath.Join(dir, "search_test.db"),
	}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestClassifyRelation(t *testing.T) {
	tests := []struct {
		name          string
		srcContent    string
		srcTags       []string
		targetContent string
		targetTags    []string
		semanticScore float64
		rrfScore      float64
		want          string
	}{
		{
			name:          "supersedes with keyword overlap",
			srcContent:    "Switched from Fly.io to AWS ECS for production",
			srcTags:       []string{"infra"},
			targetContent: "Deploy production services to Fly.io",
			targetTags:    []string{"infra"},
			semanticScore: 0.8,
			rrfScore:      0.03,
			want:          "supersedes",
		},
		{
			name:          "contradicts with cue and no overlap",
			srcContent:    "Postgres is deprecated and obsolete here",
			srcTags:       []string{"db"},
			targetContent: "Redis cluster setup details",
			targetTags:    []string{"cache"},
			semanticScore: 0.7,
			rrfScore:      0.02,
			want:          "contradicts",
		},
		{
			name:          "depends-on with dependency cue",
			srcContent:    "Web UI frontend requires backend REST endpoints",
			srcTags:       []string{"ui"},
			targetContent: "REST API endpoints specifications",
			targetTags:    []string{"api"},
			semanticScore: 0.7,
			rrfScore:      0.02,
			want:          "depends-on",
		},
		{
			name:          "refines with 2 shared tags",
			srcContent:    "Use strict camelCase for JSON output serialization",
			srcTags:       []string{"convention", "json", "api"},
			targetContent: "Output contract guidelines for CLI and REST",
			targetTags:    []string{"convention", "json"},
			semanticScore: 0.8,
			rrfScore:      0.02,
			want:          "refines",
		},
		{
			name:          "supports as default high score",
			srcContent:    "SQLite WAL mode ensures concurrent readers",
			srcTags:       []string{"sqlite"},
			targetContent: "Database architecture uses SQLite with WAL enabled",
			targetTags:    []string{"sqlite"},
			semanticScore: 0.7,
			rrfScore:      0.02,
			want:          "supports",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := search.ClassifyRelation(tc.srcContent, tc.srcTags, tc.targetContent, tc.targetTags, tc.semanticScore, tc.rrfScore)
			if got != tc.want {
				t.Errorf("ClassifyRelation() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSuggestLinks_Integration(t *testing.T) {
	s := newSearchTestStore(t)
	ctx := context.Background()

	// Seed an existing memory at global scope
	m1ID, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "Deploy production services to Fly.io",
		Tags:    []string{"infra", "deployment"},
	})
	if err != nil {
		t.Fatalf("PutMemory m1: %v", err)
	}

	searcher := search.New(s)

	// New memory at project scope that supersedes Fly.io
	m2ID, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:web",
		Type:    "note",
		Content: "Switched from Fly.io to AWS ECS for production",
		Tags:    []string{"infra"},
	})
	if err != nil {
		t.Fatalf("PutMemory m2: %v", err)
	}

	m2Row, err := s.GetMemory(ctx, m2ID)
	if err != nil {
		t.Fatalf("GetMemory m2: %v", err)
	}

	suggestions, err := searcher.SuggestLinks(ctx, *m2Row)
	if err != nil {
		t.Fatalf("SuggestLinks: %v", err)
	}

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	sug := suggestions[0]
	if sug.FromID != m2ID || sug.ToID != m1ID {
		t.Errorf("expected link %d -> %d, got %d -> %d", m2ID, m1ID, sug.FromID, sug.ToID)
	}
	if sug.Relation != "supersedes" {
		t.Errorf("expected relation 'supersedes', got %q", sug.Relation)
	}

	// Verify suggestion was stored in DB with suggested = true
	link, err := s.GetLinkByID(ctx, sug.ID)
	if err != nil {
		t.Fatalf("GetLinkByID: %v", err)
	}
	if !link.Suggested {
		t.Errorf("expected stored link to be suggested=true")
	}

	// Calling SuggestLinks again should NOT suggest the same link
	suggestions2, err := searcher.SuggestLinks(ctx, *m2Row)
	if err != nil {
		t.Fatalf("SuggestLinks 2nd run: %v", err)
	}
	if len(suggestions2) != 0 {
		t.Errorf("expected 0 suggestions on 2nd run (already linked), got %d", len(suggestions2))
	}
}

func TestRecall_IncludeLinks(t *testing.T) {
	s := newSearchTestStore(t)
	ctx := context.Background()

	m1ID, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:app",
		Type:    "note",
		Content: "Postgres database schema definition",
	})
	m2ID, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:app",
		Type:    "note",
		Content: "Postgres connection pooling using pgx",
	})

	// Create a confirmed link: m2 depends-on m1
	_, err := s.CreateLink(ctx, m2ID, m1ID, "depends-on", false)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}

	// Create a suggested link: m1 refines m2
	_, err = s.CreateLink(ctx, m1ID, m2ID, "refines", true)
	if err != nil {
		t.Fatalf("CreateLink suggested: %v", err)
	}

	searcher := search.New(s)

	// 1. Recall without IncludeLinks
	res1, err := searcher.Recall(ctx, search.Query{
		Text:         "connection pooling",
		Scope:        "project:app",
		Top:          5,
		IncludeLinks: false,
	})
	if err != nil {
		t.Fatalf("Recall 1: %v", err)
	}
	if len(res1) == 0 {
		t.Fatalf("expected results")
	}
	if len(res1[0].Links) != 0 {
		t.Errorf("expected no links when IncludeLinks is false, got %v", res1[0].Links)
	}

	// 2. Recall with IncludeLinks = true (confirmed only by default)
	res2, err := searcher.Recall(ctx, search.Query{
		Text:                  "connection pooling",
		Scope:                 "project:app",
		Top:                   5,
		IncludeLinks:          true,
		IncludeSuggestedLinks: false,
	})
	if err != nil {
		t.Fatalf("Recall 2: %v", err)
	}
	var m2Res *search.Ranked
	for i := range res2 {
		if res2[i].ID == m2ID {
			m2Res = &res2[i]
			break
		}
	}
	if m2Res == nil {
		t.Fatalf("expected m2 in recall results")
	}
	if len(m2Res.Links) != 1 {
		t.Fatalf("expected exactly 1 confirmed link for m2, got %d", len(m2Res.Links))
	}
	link := m2Res.Links[0]
	if link.Relation != "depends-on" || link.Direction != "outgoing" || link.LinkedID != m1ID {
		t.Errorf("unexpected link: %+v", link)
	}
	if link.Suggested {
		t.Errorf("expected confirmed link (suggested=false)")
	}

	// 3. Recall with IncludeSuggestedLinks = true
	res3, err := searcher.Recall(ctx, search.Query{
		Text:                  "connection pooling",
		Scope:                 "project:app",
		Top:                   5,
		IncludeLinks:          true,
		IncludeSuggestedLinks: true,
	})
	if err != nil {
		t.Fatalf("Recall 3: %v", err)
	}
	for i := range res3 {
		if res3[i].ID == m2ID {
			m2Res = &res3[i]
			break
		}
	}
	if m2Res == nil {
		t.Fatalf("expected m2 in recall results")
	}
	// m2 has 1 outgoing confirmed (depends-on m1) and 1 incoming suggested (m1 refines m2)
	if len(m2Res.Links) != 2 {
		t.Fatalf("expected 2 links when including suggested, got %d", len(m2Res.Links))
	}

	// 4. Recall with IncludeSuggestedLinks = true even when IncludeLinks = false
	res4, err := searcher.Recall(ctx, search.Query{
		Text:                  "connection pooling",
		Scope:                 "project:app",
		Top:                   5,
		IncludeLinks:          false,
		IncludeSuggestedLinks: true,
	})
	if err != nil {
		t.Fatalf("Recall 4: %v", err)
	}
	for i := range res4 {
		if res4[i].ID == m2ID {
			m2Res = &res4[i]
			break
		}
	}
	if m2Res == nil {
		t.Fatalf("expected m2 in recall 4 results")
	}
	if len(m2Res.Links) != 2 {
		t.Fatalf("expected 2 links when IncludeSuggestedLinks=true alone, got %d", len(m2Res.Links))
	}
}
