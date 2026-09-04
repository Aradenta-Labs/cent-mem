package search

func SetSemanticDistThresholdForTest(v float64) func() {
	old := semanticDistThreshold
	semanticDistThreshold = v
	return func() { semanticDistThreshold = old }
}

func ApplyScopeProximityBoostForTest(r *Ranked, q Query, sessionBoost float64) {
	applyScopeProximityBoost(r, q, sessionBoost)
}

func ApplyAgentAffinityBoostForTest(r *Ranked, q Query, agentBoost float64) {
	applyAgentAffinityBoost(r, q, agentBoost)
}
