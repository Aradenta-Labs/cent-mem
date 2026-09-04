package search

func SetSemanticDistThresholdForTest(v float64) func() {
	old := semanticDistThreshold
	semanticDistThreshold = v
	return func() { semanticDistThreshold = old }
}
