package main

import (
	"flag"
	"sort"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdTags inspects and merges tags within a scope subtree.
// Syntax:
//
//	centmem tags list [--scope <scope>]
//	centmem tags merge --scope <scope> --from a,b --to canonical [--dry-run]
func cmdTags(args []string) int {
	fs := newFlagSet("tags")
	fs.String("scope", "", "scope path filter")
	fs.String("from", "", "comma-separated source tags to merge")
	fs.String("to", "", "canonical target tag")
	fs.Bool("dry-run", false, "preview affected memory count without modifying the store")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		posArgs := fs.Args()
		if len(posArgs) == 0 {
			return cli.Invalidf("tags: action required (list or merge)")
		}

		action := strings.ToLower(posArgs[0])
		scopePath := strings.TrimSpace(fs.Lookup("scope").Value.String())

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("tags: %v", err)
		}
		defer s.Close()

		ctx, cancel := signalContext()
		defer cancel()

		switch action {
		case "list":
			if scopePath == "" {
				scopePath = "global"
			}
			sc, err := scope.Parse(scopePath)
			if err != nil {
				return cli.Invalidf("tags list: invalid --scope %q: %v", scopePath, err)
			}
			scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
			if err != nil {
				return cli.Internalf("tags list: %v", err)
			}
			if scopeIDs == nil {
				scopeIDs = []int64{}
			}
			stats, err := s.ListTagStats(ctx, scopeIDs)
			if err != nil {
				return cli.Internalf("tags list: %v", err)
			}
			if stats == nil {
				stats = []store.TagStat{}
			}
			return prettyPrint(fs, map[string]any{
				"ok":    true,
				"scope": sc.Path,
				"tags":  stats,
				"count": len(stats),
			})

		case "merge":
			if scopePath == "" {
				return cli.Invalidf("tags merge: --scope is required")
			}
			sc, err := scope.Parse(scopePath)
			if err != nil {
				return cli.Invalidf("tags merge: invalid --scope %q: %v", scopePath, err)
			}

			toTag := strings.TrimSpace(strings.ToLower(fs.Lookup("to").Value.String()))
			if toTag == "" {
				return cli.Invalidf("tags merge: --to canonical tag is required")
			}
			if strings.Contains(toTag, ",") {
				return cli.Invalidf("tags merge: --to tag %q cannot contain commas", toTag)
			}

			rawFrom := fs.Lookup("from").Value.String()
			seenFrom := make(map[string]bool)
			fromTags := make([]string, 0)
			for _, part := range strings.Split(rawFrom, ",") {
				trimmed := strings.TrimSpace(part)
				if trimmed == "" || trimmed == toTag {
					continue
				}
				// Lowercase unless the tag differs from toTag only by case (e.g. Database -> database).
				key := strings.ToLower(trimmed)
				if key == toTag {
					key = trimmed
				}
				if seenFrom[key] {
					continue
				}
				seenFrom[key] = true
				fromTags = append(fromTags, key)
			}
			if len(fromTags) == 0 {
				return cli.Invalidf("tags merge: --from requires at least one source tag distinct from --to")
			}
			sort.Strings(fromTags)

			scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
			if err != nil {
				return cli.Internalf("tags merge: %v", err)
			}
			if scopeIDs == nil {
				scopeIDs = []int64{}
			}

			dryRun := fs.Lookup("dry-run").Value.String() == "true"
			var affected int
			if dryRun {
				affected, err = s.CountMemoriesWithAnyTag(ctx, scopeIDs, fromTags, toTag)
			} else {
				affected, err = s.BulkRenameTag(ctx, scopeIDs, fromTags, toTag)
			}
			if err != nil {
				return cli.Internalf("tags merge: %v", err)
			}

			return prettyPrint(fs, map[string]any{
				"ok":             true,
				"dry_run":        dryRun,
				"scope":          sc.Path,
				"from_tags":      fromTags,
				"to_tag":         toTag,
				"affected_count": affected,
			})

		default:
			return cli.Invalidf("tags: unknown action %q (expected list or merge)", posArgs[0])
		}
	})
}
