package search

import "math"

// applyImportanceBoost adjusts candidate score by an access-frequency multiplier:
//
//	importance(c) = min(cap, 1.0 + ln(1 + c) * weight)
//
// If disabled, r is nil, or AccessCount <= 0, no change is made.
func applyImportanceBoost(r *Ranked, enabled bool, weight float64, cap float64) {
	if !enabled || r == nil || r.AccessCount <= 0 || weight <= 0 {
		return
	}
	if cap < 1.0 {
		cap = 1.0
	}
	mult := 1.0 + math.Log1p(float64(r.AccessCount))*weight
	if mult > cap {
		mult = cap
	}
	r.Score *= mult
}
