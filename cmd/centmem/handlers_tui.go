package main

import (
	"flag"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"github.com/aradenta-labs/cent-mem/internal/tui"
)

func runInteractiveTUI(cfg config.Config, fs *flag.FlagSet, initQuery string) error {
	s, err := store.Open(cfg)
	if err != nil {
		return cli.Internalf("recall --interactive: %v", err)
	}
	defer s.Close()

	emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
	rerankerChoice := cfg.Search.Reranker
	if flagReranker := fs.Lookup("reranker").Value.String(); flagReranker != "" {
		rerankerChoice = flagReranker
	}

	searcher := search.New(s).
		WithEmbedder(emb).
		WithDecayDays(cfg.Search.DecayHalfLifeDays).
		WithRerankerName(rerankerChoice).
		WithRerankWindow(cfg.Search.RerankWindow).
		WithSessionBoost(cfg.Search.SessionBoost).
		WithAgentBoost(cfg.Search.AgentBoost).
		WithImportance(cfg.Search.ImportanceBoostEnabled, cfg.Search.ImportanceWeight, cfg.Search.ImportanceCap)

	scopePath := fs.Lookup("scope").Value.String()
	top := 20
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "top" {
			top = intFlag(fs, "top", 20)
		}
	})
	if top <= 0 {
		top = 20
	}

	typeFilter := ""
	if f := fs.Lookup("type"); f != nil {
		typeFilter = f.Value.String()
	}
	var tagsFilter []string
	if f := fs.Lookup("tags"); f != nil && f.Value.String() != "" {
		tagsFilter = splitCSV(f.Value.String())
	}

	ctx, cancel := signalContext()
	defer cancel()

	selected, err := tui.Run(ctx, tui.Config{
		Searcher:  searcher,
		Store:     s,
		Scope:     scopePath,
		Top:       top,
		InitQuery: initQuery,
		Type:      typeFilter,
		Tags:      tagsFilter,
	})
	if err != nil {
		return cli.Internalf("tui: %v", err)
	}
	if selected != nil {
		var lastAccessed any
		if selected.AccessCount > 0 && selected.LastAccessedAt != nil {
			lastAccessed = selected.LastAccessedAt.Unix()
		}
		tags := selected.Tags
		if tags == nil {
			tags = []string{}
		}
		memMap := map[string]any{
			"id":               selected.ID,
			"type":             selected.Type,
			"scope":            selected.ScopePath,
			"content":          selected.Content,
			"tags":             tags,
			"created_at":       selected.CreatedAt.Unix(),
			"access_count":     selected.AccessCount,
			"last_accessed_at": lastAccessed,
		}
		if selected.Key != "" {
			memMap["key"] = selected.Key
		}
		if selected.ValueJSON != "" {
			memMap["value"] = selected.ValueJSON
		}
		if selected.SourceAgent != "" {
			memMap["source_agent"] = selected.SourceAgent
		}
		return prettyPrint(fs, map[string]any{
			"ok":     true,
			"id":     selected.ID,
			"memory": memMap,
		})
	}
	return nil
}
