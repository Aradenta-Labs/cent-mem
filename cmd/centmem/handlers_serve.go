package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/mcp"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdServe runs the centmem stdio Model Context Protocol (MCP) server.
func cmdServe(args []string) int {
	fs := newFlagSet("serve")
	fs.Bool("mcp", true, "run MCP stdio server (default true)")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		st, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("serve store open: %v", err)
		}
		defer st.Close()

		emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
		if emb != nil {
			defer emb.Close()
		}

		searcher := search.New(st).
			WithEmbedder(emb).
			WithDecayDays(cfg.Search.DecayHalfLifeDays).
			WithRerankerName(cfg.Search.Reranker).
			WithRerankWindow(cfg.Search.RerankWindow).
			WithSessionBoost(cfg.Search.SessionBoost).
			WithAgentBoost(cfg.Search.AgentBoost).
			WithImportance(cfg.Search.ImportanceBoostEnabled, cfg.Search.ImportanceWeight, cfg.Search.ImportanceCap)

		srv := mcp.NewServer(st, searcher, cfg, os.Stdin, os.Stdout, version)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		if err := srv.Run(ctx); err != nil && err != context.Canceled {
			return cli.Internalf("mcp server: %v", err)
		}
		return nil
	})
}
