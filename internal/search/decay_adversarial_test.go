package search

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestAdversarial_ApplyDecay_Math(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	t.Run("1_day_half_life_horizons", func(t *testing.T) {
		halfLife := 24 * time.Hour
		initialScore := 1.0

		cases := []struct {
			name           string
			age            time.Duration
			expectedFactor float64
			tolerance      float64
		}{
			{"age_zero", 0, 1.0, 1e-12},
			{"age_half_day", 12 * time.Hour, math.Sqrt(0.5), 1e-9},
			{"age_1_day", 24 * time.Hour, 0.5, 1e-12},
			{"age_2_days", 48 * time.Hour, 0.25, 1e-12},
			{"age_3_days", 72 * time.Hour, 0.125, 1e-12},
			{"age_5_days", 120 * time.Hour, 0.03125, 1e-12},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := &Ranked{
					ID:        1,
					CreatedAt: now.Add(-tc.age),
					Score:     initialScore,
				}
				applyDecay(r, now, halfLife)
				expected := initialScore * tc.expectedFactor
				if math.Abs(r.Score-expected) > tc.tolerance {
					t.Errorf("age %v: expected score %f, got %f (diff %e)", tc.age, expected, r.Score, math.Abs(r.Score-expected))
				}
				if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
					t.Fatalf("score is NaN or Inf: %f", r.Score)
				}
			})
		}
	})

	t.Run("7_day_half_life_horizons", func(t *testing.T) {
		halfLife := 7 * 24 * time.Hour
		initialScore := 4.0

		cases := []struct {
			name           string
			age            time.Duration
			expectedFactor float64
			tolerance      float64
		}{
			{"age_zero", 0, 1.0, 1e-12},
			{"age_3_5_days", 3*24*time.Hour + 12*time.Hour, math.Sqrt(0.5), 1e-9},
			{"age_7_days", 7 * 24 * time.Hour, 0.5, 1e-12},
			{"age_14_days", 14 * 24 * time.Hour, 0.25, 1e-12},
			{"age_21_days", 21 * 24 * time.Hour, 0.125, 1e-12},
			{"age_28_days", 28 * 24 * time.Hour, 0.0625, 1e-12},
			{"age_49_days", 49 * 24 * time.Hour, 0.0078125, 1e-12},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := &Ranked{
					ID:        1,
					CreatedAt: now.Add(-tc.age),
					Score:     initialScore,
				}
				applyDecay(r, now, halfLife)
				expected := initialScore * tc.expectedFactor
				if math.Abs(r.Score-expected) > tc.tolerance {
					t.Errorf("age %v: expected score %f, got %f", tc.age, expected, r.Score)
				}
			})
		}
	})

	t.Run("30_day_half_life_horizons", func(t *testing.T) {
		halfLife := 30 * 24 * time.Hour
		initialScore := 10.0

		cases := []struct {
			name           string
			age            time.Duration
			expectedFactor float64
			tolerance      float64
		}{
			{"age_zero", 0, 1.0, 1e-12},
			{"age_1_day", 24 * time.Hour, math.Pow(0.5, 1.0/30.0), 1e-9},
			{"age_15_days", 15 * 24 * time.Hour, math.Sqrt(0.5), 1e-9},
			{"age_30_days", 30 * 24 * time.Hour, 0.5, 1e-12},
			{"age_60_days", 60 * 24 * time.Hour, 0.25, 1e-12},
			{"age_90_days", 90 * 24 * time.Hour, 0.125, 1e-12},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := &Ranked{
					ID:        1,
					CreatedAt: now.Add(-tc.age),
					Score:     initialScore,
				}
				applyDecay(r, now, halfLife)
				expected := initialScore * tc.expectedFactor
				if math.Abs(r.Score-expected) > tc.tolerance {
					t.Errorf("age %v: expected score %f, got %f", tc.age, expected, r.Score)
				}
			})
		}
	})

	t.Run("365_day_half_life_horizons", func(t *testing.T) {
		halfLife := 365 * 24 * time.Hour
		initialScore := 2.5

		cases := []struct {
			name           string
			age            time.Duration
			expectedFactor float64
			tolerance      float64
		}{
			{"age_zero", 0, 1.0, 1e-12},
			{"age_30_days", 30 * 24 * time.Hour, math.Pow(0.5, 30.0/365.0), 1e-9},
			{"age_182_5_days", 182*24*time.Hour + 12*time.Hour, math.Sqrt(0.5), 1e-9},
			{"age_365_days", 365 * 24 * time.Hour, 0.5, 1e-12},
			{"age_730_days", 730 * 24 * time.Hour, 0.25, 1e-12},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := &Ranked{
					ID:        1,
					CreatedAt: now.Add(-tc.age),
					Score:     initialScore,
				}
				applyDecay(r, now, halfLife)
				expected := initialScore * tc.expectedFactor
				if math.Abs(r.Score-expected) > tc.tolerance {
					t.Errorf("age %v: expected score %f, got %f", tc.age, expected, r.Score)
				}
			})
		}
	})

	t.Run("zero_and_negative_half_life_noop", func(t *testing.T) {
		for _, hl := range []time.Duration{0, -1, -24 * time.Hour, -365 * 24 * time.Hour} {
			r := &Ranked{
				ID:        1,
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				Score:     1.234,
			}
			applyDecay(r, now, hl)
			if r.Score != 1.234 {
				t.Errorf("halfLife %v: expected score 1.234 unchanged, got %f", hl, r.Score)
			}
		}
	})

	t.Run("future_timestamps_and_clock_skew", func(t *testing.T) {
		halfLife := 30 * 24 * time.Hour

		futureDurations := []time.Duration{
			1 * time.Nanosecond,
			1 * time.Millisecond,
			1 * time.Second,
			1 * time.Minute,
			1 * time.Hour,
			24 * time.Hour,
			365 * 24 * time.Hour,
		}

		for _, fut := range futureDurations {
			r := &Ranked{
				ID:        1,
				CreatedAt: now.Add(fut),
				Score:     1.0,
			}
			applyDecay(r, now, halfLife)
			// Must not amplify score, must not become negative, must not become NaN/Inf
			if r.Score != 1.0 {
				t.Errorf("future CreatedAt (+%v): expected score 1.0 unchanged, got %f", fut, r.Score)
			}
			if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
				t.Fatalf("future CreatedAt (+%v): score is NaN or Inf: %f", fut, r.Score)
			}
		}
	})

	t.Run("sub_second_ages_and_microsecond_precision", func(t *testing.T) {
		halfLife := 7 * 24 * time.Hour
		initialScore := 1.0

		ages := []time.Duration{
			1 * time.Nanosecond,
			500 * time.Microsecond,
			1 * time.Millisecond,
			50 * time.Millisecond,
			500 * time.Millisecond,
		}

		for _, age := range ages {
			r := &Ranked{
				ID:        1,
				CreatedAt: now.Add(-age),
				Score:     initialScore,
			}
			applyDecay(r, now, halfLife)
			expected := initialScore * math.Pow(0.5, float64(age)/float64(halfLife))
			if math.Abs(r.Score-expected) > 1e-12 {
				t.Errorf("sub-second age %v: expected %f, got %f", age, expected, r.Score)
			}
			if r.Score > initialScore {
				t.Errorf("sub-second age %v amplified score: %f > %f", age, r.Score, initialScore)
			}
			if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
				t.Fatalf("sub-second age %v: score is NaN or Inf: %f", age, r.Score)
			}
		}
	})

	t.Run("boundary_and_defensive_edge_cases", func(t *testing.T) {
		halfLife := 15 * 24 * time.Hour

		// Nil ranked should not panic
		applyDecay(nil, now, halfLife)

		// Zero CreatedAt should be no-op
		rZero := &Ranked{ID: 1, CreatedAt: time.Time{}, Score: 1.0}
		applyDecay(rZero, now, halfLife)
		if rZero.Score != 1.0 {
			t.Errorf("zero CreatedAt: expected score 1.0 unchanged, got %f", rZero.Score)
		}

		// Initial score zero remains zero
		rZeroScore := &Ranked{ID: 1, CreatedAt: now.Add(-10 * 24 * time.Hour), Score: 0.0}
		applyDecay(rZeroScore, now, halfLife)
		if rZeroScore.Score != 0.0 {
			t.Errorf("initial score 0: expected 0, got %f", rZeroScore.Score)
		}

		// Negative initial score decays towards zero
		rNeg := &Ranked{ID: 1, CreatedAt: now.Add(-15 * 24 * time.Hour), Score: -2.0}
		applyDecay(rNeg, now, halfLife)
		if math.Abs(rNeg.Score-(-1.0)) > 1e-12 {
			t.Errorf("negative score: expected -1.0, got %f", rNeg.Score)
		}

		// Extremely old memory (Unix epoch 1970)
		rEpoch := &Ranked{ID: 1, CreatedAt: time.Unix(0, 0), Score: 1.0}
		applyDecay(rEpoch, now, 1*24*time.Hour)
		if math.IsNaN(rEpoch.Score) || math.IsInf(rEpoch.Score, 0) {
			t.Fatalf("epoch memory: score is NaN or Inf: %f", rEpoch.Score)
		}
		if rEpoch.Score < 0 {
			t.Errorf("epoch memory: negative score %f", rEpoch.Score)
		}
		// Over 20,000 days with 1-day half-life -> underflows to 0.0
		if rEpoch.Score != 0.0 {
			t.Errorf("epoch memory: expected score to underflow to 0.0, got %e", rEpoch.Score)
		}

		// Extremely small half-life (1 nanosecond)
		rTinyHL := &Ranked{ID: 1, CreatedAt: now.Add(-1 * time.Second), Score: 1.0}
		applyDecay(rTinyHL, now, 1*time.Nanosecond)
		if math.IsNaN(rTinyHL.Score) || math.IsInf(rTinyHL.Score, 0) {
			t.Fatalf("tiny half-life: score is NaN or Inf: %f", rTinyHL.Score)
		}
		if rTinyHL.Score != 0.0 {
			t.Errorf("tiny half-life: expected underflow to 0.0, got %e", rTinyHL.Score)
		}
	})

	t.Run("strict_monotonicity", func(t *testing.T) {
		halfLife := 10 * 24 * time.Hour
		prevScore := 1.0
		initialScore := 1.0

		for days := 0; days <= 100; days++ {
			r := &Ranked{
				ID:        1,
				CreatedAt: now.Add(-time.Duration(days) * 24 * time.Hour),
				Score:     initialScore,
			}
			applyDecay(r, now, halfLife)
			if r.Score > prevScore {
				t.Fatalf("monotonicity violation at %d days: prev %f, current %f", days, prevScore, r.Score)
			}
			prevScore = r.Score
		}
	})
}

func TestAdversarial_WithDecayDays_Searcher(t *testing.T) {
	cfg := config.Config{DBPath: t.TempDir() + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	se := New(s)

	// 1. Default value
	if se.decayHalfLifeDays != 0 {
		t.Errorf("expected default decayHalfLifeDays=0, got %d", se.decayHalfLifeDays)
	}

	// 2. Chaining positive days returns a copy
	ret := se.WithDecayDays(30)
	if ret == se {
		t.Errorf("WithDecayDays should return a copy for safe concurrent chaining")
	}
	if ret.decayHalfLifeDays != 30 {
		t.Errorf("expected decayHalfLifeDays=30, got %d", ret.decayHalfLifeDays)
	}
	if se.decayHalfLifeDays != 0 {
		t.Errorf("original Searcher should remain unchanged (decayHalfLifeDays=0), got %d", se.decayHalfLifeDays)
	}

	// 3. Reset to zero
	ret = se.WithDecayDays(0)
	if ret.decayHalfLifeDays != 0 {
		t.Errorf("expected decayHalfLifeDays=0, got %d", ret.decayHalfLifeDays)
	}

	// 4. Clamping negative to zero
	ret = se.WithDecayDays(-15)
	if ret.decayHalfLifeDays != 0 {
		t.Errorf("expected negative days to clamp to 0, got %d", ret.decayHalfLifeDays)
	}
}

func TestAdversarial_Recall_DecayReRankingAndCollapsing(t *testing.T) {
	cfg := config.Config{DBPath: t.TempDir() + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// 1. Seed two duplicate memories with identical content
	// Memory 1: older (45 days old)
	idOldDupe, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "authentication token rotation policy secret rotation",
	})
	if err != nil {
		t.Fatalf("PutMemory idOldDupe: %v", err)
	}

	// Backdate old dupe by 45 days (both created_at and updated_at to bypass 60s ingestion dedup window)
	past45 := time.Now().Add(-45 * 24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec("UPDATE memories SET created_at = ?, updated_at = ? WHERE id = ?", past45, past45, idOldDupe); err != nil {
		t.Fatalf("UPDATE created_at: %v", err)
	}

	// Memory 2: brand new (same exact content)
	idNewDupe, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "authentication token rotation policy secret rotation",
	})
	if err != nil {
		t.Fatalf("PutMemory idNewDupe: %v", err)
	}

	if idOldDupe == idNewDupe {
		t.Fatalf("expected distinct IDs for duplicates, but got idOldDupe == idNewDupe == %d", idOldDupe)
	}

	// Memory 3: different content, medium age (15 days old)
	idUnique, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "authentication token rotation database backup procedure",
	})
	if err != nil {
		t.Fatalf("PutMemory idUnique: %v", err)
	}
	past15 := time.Now().Add(-15 * 24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec("UPDATE memories SET created_at = ?, updated_at = ? WHERE id = ?", past15, past15, idUnique); err != nil {
		t.Fatalf("UPDATE created_at: %v", err)
	}

	q := Query{
		Text:    "authentication token rotation",
		Scope:   "global",
		Inherit: true,
		Top:     5,
	}

	t.Run("decay_disabled_retains_older_if_tied_or_higher_id", func(t *testing.T) {
		se := New(s).WithDecayDays(0)
		res, err := se.Recall(ctx, q)
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		// Duplicates must be collapsed to 1 result
		var dupeCount int
		for _, r := range res {
			if r.Content == "authentication token rotation policy secret rotation" {
				dupeCount++
			}
		}
		if dupeCount != 1 {
			t.Errorf("expected exactly 1 duplicate preserved without decay, got %d", dupeCount)
		}
	})

	t.Run("decay_enabled_promotes_newer_dupe_and_preserves_it", func(t *testing.T) {
		// Half-life = 15 days.
		// idOldDupe age = 45 days (3 half-lives -> score * 0.125)
		// idNewDupe age = 0 days (score * 1.0)
		se := New(s).WithDecayDays(15)
		res, err := se.Recall(ctx, q)
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		if len(res) < 2 {
			t.Fatalf("expected at least 2 results, got %d", len(res))
		}

		// The new duplicate must rank #1 and be the preserved instance of that content
		if res[0].ID != idNewDupe {
			t.Errorf("expected new duplicate (ID %d) to be preserved and rank #1, got ID %d (score %f)",
				idNewDupe, res[0].ID, res[0].Score)
		}

		// Ensure old duplicate was collapsed out
		for _, r := range res {
			if r.ID == idOldDupe {
				t.Errorf("expected old duplicate (ID %d) to be collapsed out, but found in results", idOldDupe)
			}
		}

		// Ensure the other unique memory (idUnique) is preserved and ranks #2
		if res[1].ID != idUnique {
			t.Errorf("expected unique memory (ID %d) as rank #2, got ID %d", idUnique, res[1].ID)
		}
	})
}

func TestAdversarial_Recall_DecayRankReordering_OlderHighScoreVsNewerLowScore(t *testing.T) {
	cfg := config.Config{DBPath: t.TempDir() + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Memory 1: Older memory (21 days old) with high keyword relevance
	// Contains "kubernetes ingress controller envoy proxy rate limiting gateway architecture"
	idOldHigh, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "kubernetes ingress controller envoy proxy rate limiting gateway architecture",
		Tags:    []string{"k8s", "gateway"},
	})
	if err != nil {
		t.Fatalf("PutMemory idOldHigh: %v", err)
	}
	past21 := time.Now().Add(-21 * 24 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec("UPDATE memories SET created_at = ?, updated_at = ? WHERE id = ?", past21, past21, idOldHigh); err != nil {
		t.Fatalf("UPDATE created_at idOldHigh: %v", err)
	}

	// Memory 2: Brand new memory (1 hour old) with lower keyword relevance
	// Only contains "kubernetes ingress note"
	idNewLow, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "global",
		Type:    "note",
		Content: "kubernetes ingress note",
		Tags:    []string{"k8s"},
	})
	if err != nil {
		t.Fatalf("PutMemory idNewLow: %v", err)
	}
	past1h := time.Now().Add(-1 * time.Hour).UnixMicro()
	if _, err := s.DB().Exec("UPDATE memories SET created_at = ?, updated_at = ? WHERE id = ?", past1h, past1h, idNewLow); err != nil {
		t.Fatalf("UPDATE created_at idNewLow: %v", err)
	}

	q := Query{
		Text:    "kubernetes ingress controller envoy proxy rate limiting",
		Scope:   "global",
		Inherit: true,
		Top:     5,
	}

	// Baseline: Without decay (decay=0)
	// idOldHigh matches all query terms, so it has much higher relevance and ranks #1.
	// idNewLow matches only 2 terms, so it ranks #2.
	t.Run("without_decay_older_high_score_wins", func(t *testing.T) {
		se := New(s).WithDecayDays(0)
		res, err := se.Recall(ctx, q)
		if err != nil {
			t.Fatalf("Recall without decay: %v", err)
		}
		if len(res) < 2 {
			t.Fatalf("expected at least 2 results, got %d", len(res))
		}
		if res[0].ID != idOldHigh {
			t.Errorf("without decay: expected older high-relevance memory (ID %d) at rank #1, got ID %d (score %f vs %f)",
				idOldHigh, res[0].ID, res[0].Score, res[1].Score)
		}
		if res[1].ID != idNewLow {
			t.Errorf("without decay: expected newer low-relevance memory (ID %d) at rank #2, got ID %d",
				idNewLow, res[1].ID)
		}
		if res[0].Score <= res[1].Score {
			t.Errorf("without decay: expected rank #1 score > rank #2 score, got %f <= %f", res[0].Score, res[1].Score)
		}
	})

	// Challenge: With decay enabled (half-life = 7 days)
	// idOldHigh age = 21 days (3 half-lives). Score is multiplied by 0.5^3 = 0.125.
	// idNewLow age = 1 hour (0.00595 half-lives). Score is multiplied by ~0.9959.
	// Decay heavily penalizes the old memory so the new memory overtakes it and ranks #1!
	t.Run("with_decay_newer_overtakes_older_high_score", func(t *testing.T) {
		se := New(s).WithDecayDays(7)
		res, err := se.Recall(ctx, q)
		if err != nil {
			t.Fatalf("Recall with decay: %v", err)
		}
		if len(res) < 2 {
			t.Fatalf("expected at least 2 results, got %d", len(res))
		}
		if res[0].ID != idNewLow {
			t.Errorf("with decay: expected newer memory (ID %d) to overtake and rank #1, got ID %d (scores: #1=%f, #2=%f)",
				idNewLow, res[0].ID, res[0].Score, res[1].Score)
		}
		if res[1].ID != idOldHigh {
			t.Errorf("with decay: expected older memory (ID %d) to drop to rank #2, got ID %d",
				idOldHigh, res[1].ID)
		}
		if res[0].Score <= res[1].Score {
			t.Errorf("with decay: expected rank #1 score > rank #2 score, got %f <= %f", res[0].Score, res[1].Score)
		}
	})
}
