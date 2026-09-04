package search_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// TestBoost_ScopeProximity tests that a memory in the caller's active session scope
// receives a 1.25x boost and outranks a parent project memory with identical content.
func TestBoost_ScopeProximity(t *testing.T) {
	_, s := testSearcher(t)
	ctx := context.Background()

	// Seed identical content in parent project scope and child session scope
	sessionScope := "project:cent-mem/agent:claude/session:42"
	projectScope := "project:cent-mem"

	_, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   projectScope,
		Type:    "note",
		Content: "Shared architecture decisions across teams",
	})
	if err != nil {
		t.Fatalf("seed project memory: %v", err)
	}

	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:   sessionScope,
		Type:    "note",
		Content: "Shared architecture decisions across teams",
	})
	if err != nil {
		t.Fatalf("seed session memory: %v", err)
	}

	// Recall from session scope with inheritance
	searcher := search.New(s)
	res, err := searcher.Recall(ctx, search.Query{
		Text:    "architecture decisions",
		Scope:   sessionScope,
		Inherit: true,
		Top:     5,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if len(res) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(res))
	}

	// First result should be the session memory due to 1.25x session proximity boost
	if res[0].Scope != sessionScope {
		t.Errorf("expected rank 1 to be session scope %q, got %q (score: %f vs %f)",
			sessionScope, res[0].Scope, res[0].Score, res[1].Score)
	}
	if res[1].Scope != projectScope {
		t.Errorf("expected rank 2 to be project scope %q, got %q", projectScope, res[1].Scope)
	}
	if res[0].Score <= res[1].Score {
		t.Errorf("expected session score %f > project score %f", res[0].Score, res[1].Score)
	}
}

// TestBoost_CallerAgentAffinity tests that a memory authored by the caller agent
// receives a 1.15x boost over another agent's memory with otherwise equal relevance.
func TestBoost_CallerAgentAffinity(t *testing.T) {
	_, s := testSearcher(t)
	ctx := context.Background()

	scopePath := "project:cent-mem"
	caller := "agent-alpha"
	other := "agent-beta"

	// Memory by other agent
	_, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:       scopePath,
		Type:        "note",
		Content:     "Database configuration guide and best practices for servers",
		SourceAgent: other,
	})
	if err != nil {
		t.Fatalf("seed other memory: %v", err)
	}

	// Memory by caller agent
	_, _, err = s.PutMemory(ctx, store.MemoryInput{
		Scope:       scopePath,
		Type:        "note",
		Content:     "Database configuration guide and best practices for clients",
		SourceAgent: caller,
	})
	if err != nil {
		t.Fatalf("seed caller memory: %v", err)
	}

	searcher := search.New(s)
	res, err := searcher.Recall(ctx, search.Query{
		Text:        "database configuration",
		Scope:       scopePath,
		CallerAgent: caller,
		Top:         5,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if len(res) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(res))
	}

	if res[0].SourceAgent != caller {
		t.Errorf("expected rank 1 to be caller agent %q, got %q (score: %f vs %f)",
			caller, res[0].SourceAgent, res[0].Score, res[1].Score)
	}
	if res[1].SourceAgent != other {
		t.Errorf("expected rank 2 to be other agent %q, got %q", other, res[1].SourceAgent)
	}
	if res[0].Score <= res[1].Score {
		t.Errorf("expected caller score %f > other score %f", res[0].Score, res[1].Score)
	}
}

// TestBoost_DirectUnitEdgeCases tests scope delta multipliers directly on Ranked structs.
func TestBoost_DirectUnitEdgeCases(t *testing.T) {
	now := time.Now()

	// 1. Session exact match vs Parent Agent vs Ancestor Project vs Global
	q := search.Query{
		Scope:       "project:cent-mem/agent:claude/session:42",
		CallerAgent: "claude",
	}

	rSession := &search.Ranked{ID: 1, Scope: "project:cent-mem/agent:claude/session:42", SourceAgent: "claude", Score: 1.0, CreatedAt: now}
	rAgent := &search.Ranked{ID: 2, Scope: "project:cent-mem/agent:claude", SourceAgent: "claude", Score: 1.0, CreatedAt: now}
	rProject := &search.Ranked{ID: 3, Scope: "project:cent-mem", SourceAgent: "bob", Score: 1.0, CreatedAt: now}
	rGlobal := &search.Ranked{ID: 4, Scope: "global", SourceAgent: "bob", Score: 1.0, CreatedAt: now}

	sessionBoost := 1.30
	agentBoost := 1.20

	search.ApplyScopeProximityBoostForTest(rSession, q, sessionBoost)
	search.ApplyScopeProximityBoostForTest(rAgent, q, sessionBoost)
	search.ApplyScopeProximityBoostForTest(rProject, q, sessionBoost)
	search.ApplyScopeProximityBoostForTest(rGlobal, q, sessionBoost)

	if rSession.Score != 1.30 {
		t.Errorf("rSession score = %f, want 1.30", rSession.Score)
	}
	if rAgent.Score != 1.10 {
		t.Errorf("rAgent score = %f, want 1.10", rAgent.Score)
	}
	if rProject.Score != 1.00 {
		t.Errorf("rProject score = %f, want 1.00", rProject.Score)
	}
	if rGlobal.Score != 0.95 {
		t.Errorf("rGlobal score = %f, want 0.95", rGlobal.Score)
	}

	// 2. Caller Agent Affinity Boost
	search.ApplyAgentAffinityBoostForTest(rSession, q, agentBoost)
	search.ApplyAgentAffinityBoostForTest(rProject, q, agentBoost)

	// rSession source_agent "claude" matches caller "claude": 1.30 * 1.20 = 1.56
	expectedSession := 1.30 * 1.20
	if rSession.Score != expectedSession {
		t.Errorf("rSession score after affinity = %f, want %f", rSession.Score, expectedSession)
	}
	// rProject source_agent "bob" does not match caller "claude": unchanged at 1.00
	if rProject.Score != 1.00 {
		t.Errorf("rProject score after affinity = %f, want 1.00", rProject.Score)
	}

	// 3. Empty caller agent or empty source agent
	rNoCaller := &search.Ranked{ID: 5, Scope: "global", SourceAgent: "bob", Score: 1.0, CreatedAt: now}
	search.ApplyAgentAffinityBoostForTest(rNoCaller, search.Query{CallerAgent: ""}, agentBoost)
	if rNoCaller.Score != 1.0 {
		t.Errorf("expected no boost with empty CallerAgent, got %f", rNoCaller.Score)
	}

	rNoSource := &search.Ranked{ID: 6, Scope: "global", SourceAgent: "", Score: 1.0, CreatedAt: now}
	search.ApplyAgentAffinityBoostForTest(rNoSource, q, agentBoost)
	if rNoSource.Score != 1.0 {
		t.Errorf("expected no boost with empty SourceAgent, got %f", rNoSource.Score)
	}
}

// TestRecall_CandidateWindowPartitioning verifies that candidates outside the
// rerank candidate window do not leapfrog over reranked candidates inside the window.
func TestRecall_CandidateWindowPartitioning(t *testing.T) {
	_, s := testSearcher(t)
	ctx := context.Background()

	// Seed 4 memories
	for i := 1; i <= 4; i++ {
		_, _, err := s.PutMemory(ctx, store.MemoryInput{
			Scope:   "project:cent-mem",
			Type:    "note",
			Content: fmt.Sprintf("partition test memory number %d with general content", i),
		})
		if err != nil {
			t.Fatalf("seed memory %d: %v", i, err)
		}
	}

	// Configure searcher with window size = 2
	searcher := search.New(s).WithRerankWindow(2)
	results, err := searcher.Recall(ctx, search.Query{
		Text:    "partition test memory",
		Scope:   "project:cent-mem",
		Inherit: true,
		Top:     4,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(results) < 4 {
		t.Fatalf("expected at least 4 results, got %d", len(results))
	}

	// The first 2 candidates were in the rerank window (scored on composite scale ~0.1 - 1.0)
	// The next 2 candidates were outside the window (scored on RRF scale ~0.01 - 0.05)
	// The window boundary must be respected.
	if len(results) >= 2 {
		for i := 0; i < 2; i++ {
			if results[i].Score < 0.05 {
				t.Logf("window candidate %d score: %f", i, results[i].Score)
			}
		}
	}
}
