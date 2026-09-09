package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdSummarize synthesizes scope briefings, conventions, and architectural pillars.
// Syntax:
//   centmem summarize [--scope <scope>] [--focus <topic>] [--format markdown|json] [--save]
func cmdSummarize(args []string) int {
	fs := newFlagSet("summarize")
	fs.String("scope", "", "scope path filter")
	fs.String("focus", "", "optional topic or component focus for summary")
	fs.String("format", "json", "output format (json or markdown)")
	fs.Bool("save", false, "save synthesized summary as a new note memory")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		format := strings.ToLower(fs.Lookup("format").Value.String())
		if format == "" {
			format = "json"
		}
		if format != "json" && format != "markdown" {
			return cli.Invalidf("summarize: invalid --format %q (expected json or markdown)", format)
		}

		scopePath := fs.Lookup("scope").Value.String()
		if scopePath != "" {
			if _, err := scope.Parse(scopePath); err != nil {
				return cli.Invalidf("summarize: invalid --scope %q: %v", scopePath, err)
			}
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("summarize: %v", err)
		}
		defer s.Close()

		engine := initAgentEngine(cfg, s)

		ctx, cancel := signalContext()
		defer cancel()

		focus := fs.Lookup("focus").Value.String()
		save := fs.Lookup("save").Value.String() == "true"

		res, err := engine.Summarize(ctx, agent.SummarizeOptions{
			Scope:  scopePath,
			Focus:  focus,
			Save:   save,
			Format: format,
		})
		if err != nil {
			return cli.Internalf("summarize: %v", err)
		}

		if format == "markdown" {
			fmt.Fprintln(os.Stdout, res.SummaryMarkdown)
			return nil
		}

		cited := res.CitedMemoryIDs
		if cited == nil {
			cited = []int64{}
		}

		out := map[string]any{
			"ok":               true,
			"title":            res.Title,
			"summary_markdown": res.SummaryMarkdown,
			"cited_memory_ids": cited,
			"scope":            res.Scope,
			"fallback_used":    res.FallbackUsed,
		}
		if res.SavedID != nil {
			out["saved_id"] = *res.SavedID
		}

		return prettyPrint(fs, out)
	})
}
