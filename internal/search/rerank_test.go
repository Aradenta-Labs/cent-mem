package search_test

import (
	"context"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/search"
)

// TestCompositeReRanker_PhraseAndLexical tests that candidate with high lexical
// and exact phrase match moves above a distant candidate that had higher timeline rank.
func TestCompositeReRanker_PhraseAndLexical(t *testing.T) {
	reranker := search.NewCompositeReRanker()
	ctx := context.Background()

	now := time.Now()
	query := search.Query{
		Text: "sqlite connection pooling",
	}

	// Candidate A: Higher initial RRF score (e.g. from timeline rank 1),
	// but only marginal word match ("sqlite") and no phrase match.
	candA := &search.Ranked{
		ID:            1,
		Content:       "general notes about sqlite without pooling",
		Score:         0.040, // higher RRF
		CreatedAt:     now,
		SemanticScore: 0.10,
	}

	// Candidate B: Lower initial RRF score (rank 4),
	// but has exact phrase match "sqlite connection pooling" and 100% lexical match!
	candB := &search.Ranked{
		ID:            2,
		Content:       "recommendations for sqlite connection pooling in production",
		Score:         0.020, // lower RRF
		CreatedAt:     now.Add(-time.Hour),
		SemanticScore: 0.85,
	}

	cands := []*search.Ranked{candA, candB}
	rescored, err := reranker.ReRank(ctx, query, nil, cands)
	if err != nil {
		t.Fatalf("ReRank: %v", err)
	}

	if len(rescored) != 2 {
		t.Fatalf("expected 2 rescored candidates, got %d", len(rescored))
	}

	// Candidate B must now be rank 1 because of high semantic (0.85),
	// 100% lexical coverage, and exact phrase match!
	if rescored[0].ID != 2 {
		t.Errorf("expected candidate B (ID 2) to be ranked first, got ID %d (score A=%f, score B=%f)",
			rescored[0].ID, candA.Score, candB.Score)
	}
	if rescored[0].Score <= rescored[1].Score {
		t.Errorf("expected B score %f > A score %f", rescored[0].Score, rescored[1].Score)
	}
}

// TestCompositeReRanker_EdgeCases tests empty queries, empty candidates, no tags, etc.
func TestCompositeReRanker_EdgeCases(t *testing.T) {
	reranker := search.NewCompositeReRanker()
	ctx := context.Background()

	// 1. Empty candidate list
	out, err := reranker.ReRank(ctx, search.Query{Text: "test"}, nil, nil)
	if err != nil || len(out) != 0 {
		t.Errorf("expected empty result for nil candidates, got %v, err=%v", out, err)
	}

	// 2. Empty query string
	now := time.Now()
	single := []*search.Ranked{{
		ID:        1,
		Content:   "some content",
		Score:     0.05,
		CreatedAt: now,
	}}
	out, err = reranker.ReRank(ctx, search.Query{Text: ""}, nil, single)
	if err != nil {
		t.Fatalf("empty query ReRank: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 result, got %d", len(out))
	}
	// Score should be purely normalized RRF component
	if out[0].Score <= 0 {
		t.Errorf("expected positive normalized score, got %f", out[0].Score)
	}

	// 3. Strategy name
	if reranker.Name() != "composite" {
		t.Errorf("expected strategy 'composite', got %q", reranker.Name())
	}
}
