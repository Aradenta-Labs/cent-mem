package search

import (
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/scope"
)

// applyScopeProximityBoost applies a scope proximity multiplier to r.Score
// based on hierarchical distance between candidate scope and query scope.
// Multipliers (from spec Section 4.1):
// - 1.25x (or sessionBoost) if r.Scope == q.Scope and q.Scope is Session
// - 1.10x if r.Scope is Agent and Parent of q.Scope
// - 1.00x if r.Scope is Project
// - 0.95x if r.Scope is Global
func applyScopeProximityBoost(r *Ranked, q Query, sessionBoost float64) {
	if r == nil || q.Scope == "" || r.Scope == "" {
		return
	}
	if sessionBoost <= 0 {
		sessionBoost = 1.25
	}

	targetScope, err := scope.Parse(q.Scope)
	if err != nil {
		return
	}
	rScope, err := scope.Parse(r.Scope)
	if err != nil {
		return
	}

	var mult float64 = 1.0
	if targetScope.Kind == scope.Session {
		if rScope.Path == targetScope.Path {
			mult = sessionBoost
		} else if rScope.Path == targetScope.ParentPath {
			mult = 1.10
		} else if rScope.Kind == scope.Project {
			mult = 1.00
		} else if rScope.Kind == scope.Global || rScope.Path == "global" {
			mult = 0.95
		}
	} else if targetScope.Kind == scope.Agent {
		if rScope.Path == targetScope.Path {
			mult = 1.10
		} else if rScope.Kind == scope.Project {
			mult = 1.00
		} else if rScope.Kind == scope.Global || rScope.Path == "global" {
			mult = 0.95
		}
	} else if targetScope.Kind == scope.Project {
		if rScope.Kind == scope.Project {
			mult = 1.00
		} else if rScope.Kind == scope.Global || rScope.Path == "global" {
			mult = 0.95
		}
	}

	r.Score *= mult
}

// applyAgentAffinityBoost multiplies r.Score by agentBoost (+15%) if
// q.CallerAgent is non-empty and matches candidate's r.SourceAgent.
func applyAgentAffinityBoost(r *Ranked, q Query, agentBoost float64) {
	if r == nil || strings.TrimSpace(q.CallerAgent) == "" {
		return
	}
	if agentBoost <= 0 {
		agentBoost = 1.15
	}
	if strings.TrimSpace(r.SourceAgent) != "" && strings.EqualFold(strings.TrimSpace(r.SourceAgent), strings.TrimSpace(q.CallerAgent)) {
		r.Score *= agentBoost
	}
}
