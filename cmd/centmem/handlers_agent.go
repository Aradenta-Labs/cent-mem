package main

import (
	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// initAgentEngine wires up embedder, hybrid searcher, and agent reasoning engine.
func initAgentEngine(cfg config.Config, s *store.Store) *agent.Engine {
	emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
	searcher := search.New(s).
		WithEmbedder(emb).
		WithDecayDays(cfg.Search.DecayHalfLifeDays).
		WithRerankerName(cfg.Search.Reranker).
		WithRerankWindow(cfg.Search.RerankWindow).
		WithSessionBoost(cfg.Search.SessionBoost).
		WithAgentBoost(cfg.Search.AgentBoost).
		WithImportance(cfg.Search.ImportanceBoostEnabled, cfg.Search.ImportanceWeight, cfg.Search.ImportanceCap)
	return agent.NewEngine(cfg, s, searcher)
}
