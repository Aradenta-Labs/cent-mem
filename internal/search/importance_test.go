package search

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestApplyImportanceBoost_Progression(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		baseScore float64
		enabled   bool
		weight    float64
		cap       float64
		wantMult  float64
		wantScore float64
		epsilon   float64
	}{
		{
			name:      "zero access neutral baseline",
			count:     0,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  1.0,
			wantScore: 1.0,
			epsilon:   1e-6,
		},
		{
			name:      "1 access recalled once",
			count:     1,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  1.0 + math.Log1p(1.0)*0.1, // ~1.069315
			wantScore: 1.0 * (1.0 + math.Log1p(1.0)*0.1),
			epsilon:   1e-4,
		},
		{
			name:      "15 accesses frequently consulted",
			count:     15,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  1.0 + math.Log1p(15.0)*0.1, // ~1.277259
			wantScore: 1.0 * (1.0 + math.Log1p(15.0)*0.1),
			epsilon:   1e-4,
		},
		{
			name:      "100 accesses heavily referenced",
			count:     100,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  1.0 + math.Log1p(100.0)*0.1, // ~1.461512
			wantScore: 1.0 * (1.0 + math.Log1p(100.0)*0.1),
			epsilon:   1e-4,
		},
		{
			name:      "1000 accesses universal convention",
			count:     1000,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  1.0 + math.Log1p(1000.0)*0.1, // ~1.690875
			wantScore: 1.0 * (1.0 + math.Log1p(1000.0)*0.1),
			epsilon:   1e-4,
		},
		{
			name:      "25000 accesses clamped at cap",
			count:     25000,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  2.0,
			wantScore: 2.0,
			epsilon:   1e-6,
		},
		{
			name:      "100000 accesses hard ceiling at cap",
			count:     100000,
			baseScore: 0.5,
			enabled:   true,
			weight:    0.1,
			cap:       2.0,
			wantMult:  2.0,
			wantScore: 1.0, // 0.5 * 2.0
			epsilon:   1e-6,
		},
		{
			name:      "disabled toggle ignores high access count",
			count:     1000,
			baseScore: 0.75,
			enabled:   false,
			weight:    0.1,
			cap:       2.0,
			wantMult:  1.0,
			wantScore: 0.75,
			epsilon:   1e-6,
		},
		{
			name:      "custom weight and cap",
			count:     10,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.05,
			cap:       1.5,
			wantMult:  1.0 + math.Log1p(10.0)*0.05,
			wantScore: 1.0 * (1.0 + math.Log1p(10.0)*0.05),
			epsilon:   1e-6,
		},
		{
			name:      "custom low cap clamped",
			count:     100,
			baseScore: 1.0,
			enabled:   true,
			weight:    0.1,
			cap:       1.2,
			wantMult:  1.2,
			wantScore: 1.2,
			epsilon:   1e-6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Ranked{
				AccessCount: tc.count,
				Score:       tc.baseScore,
			}
			applyImportanceBoost(r, tc.enabled, tc.weight, tc.cap)
			diff := math.Abs(r.Score - tc.wantScore)
			if diff > tc.epsilon {
				t.Errorf("Score = %f, want %f (diff %e > %e)", r.Score, tc.wantScore, diff, tc.epsilon)
			}
		})
	}
}

func TestApplyImportanceBoost_NilSafety(t *testing.T) {
	// Must not panic on nil Ranked
	applyImportanceBoost(nil, true, 0.1, 2.0)

	// Negative access count is treated as no boost
	r := &Ranked{AccessCount: -1, Score: 1.0}
	applyImportanceBoost(r, true, 0.1, 2.0)
	if r.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", r.Score)
	}
}

func TestRecall_ImportanceScore_Promotion(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "importance_test.db")}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	ctx := context.Background()

	// Insert two notes with identical content relevance
	idLow, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "deployment guidelines using fly io",
		Tags:    []string{"deploy"},
	})
	if err != nil {
		t.Fatalf("put low: %v", err)
	}

	idHigh, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "deployment guidelines using fly io platform",
		Tags:    []string{"deploy"},
	})
	if err != nil {
		t.Fatalf("put high: %v", err)
	}

	// Record 25 accesses on idHigh
	for i := 0; i < 25; i++ {
		if err := s.RecordAccess(ctx, []int64{idHigh}); err != nil {
			t.Fatalf("record access: %v", err)
		}
	}

	searcher := New(s)
	results, err := searcher.Recall(ctx, Query{
		Text:  "deployment guidelines",
		Scope: "global",
		Top:   5,
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// Top result should be idHigh due to importance boost
	if results[0].ID != idHigh {
		t.Errorf("expected top result to be id %d (high access), got %d (score=%f vs %f)",
			idHigh, results[0].ID, results[0].Score, results[1].Score)
	}
	if results[0].AccessCount != 25 {
		t.Errorf("expected access_count 25, got %d", results[0].AccessCount)
	}
	if results[1].ID != idLow {
		t.Errorf("expected second result to be id %d (low access), got %d", idLow, results[1].ID)
	}
	if results[1].AccessCount != 0 {
		t.Errorf("expected access_count 0, got %d", results[1].AccessCount)
	}
}
