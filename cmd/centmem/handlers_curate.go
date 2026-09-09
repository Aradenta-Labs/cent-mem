package main

import (
	"flag"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdCurate handles autonomous curation (deduplication, contradiction & evolution detection).
// Syntax:
//   centmem curate [--scope <scope>] [--type contradictions|dedup|all] [--apply] [--dry-run]
func cmdCurate(args []string) int {
	fs := newFlagSet("curate")
	fs.String("scope", "", "scope path filter")
	fs.String("type", "all", "curation type (contradictions, dedup, all)")
	fs.Bool("apply", false, "automatically apply proposals exceeding confidence threshold")
	fs.Bool("dry-run", false, "simulate curation without staging proposals to store")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		curateType := strings.ToLower(fs.Lookup("type").Value.String())
		if curateType == "" {
			curateType = "all"
		}
		switch curateType {
		case "contradictions", "dedup", "all":
		default:
			return cli.Invalidf("curate: invalid --type %q (expected contradictions, dedup, or all)", curateType)
		}

		scopePath := fs.Lookup("scope").Value.String()
		if scopePath != "" {
			if _, err := scope.Parse(scopePath); err != nil {
				return cli.Invalidf("curate: invalid --scope %q: %v", scopePath, err)
			}
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("curate: %v", err)
		}
		defer s.Close()

		engine := initAgentEngine(cfg, s)

		ctx, cancel := signalContext()
		defer cancel()

		apply := fs.Lookup("apply").Value.String() == "true"
		dryRun := fs.Lookup("dry-run").Value.String() == "true"

		res, err := engine.Curate(ctx, agent.CurateOptions{
			Scope:     scopePath,
			Type:      curateType,
			AutoApply: apply,
			DryRun:    dryRun,
		})
		if err != nil {
			return cli.Internalf("curate: %v", err)
		}

		created := res.ProposalsCreated
		if created == nil {
			created = []int64{}
		}
		applied := res.ProposalsApplied
		if applied == nil {
			applied = []int64{}
		}

		out := map[string]any{
			"ok":                   true,
			"proposals_created":    created,
			"proposals_applied":    applied,
			"scanned_memories":     res.ScannedMemories,
			"contradictions_found": res.ContradictionsFound,
			"duplicates_found":     res.DuplicatesFound,
			"fallback_used":        res.FallbackUsed,
		}

		return prettyPrint(fs, out)
	})
}
