package main

import (
	"bufio"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdScope handles hierarchical scope operations (delete, list).
// Syntax:
//   centmem scope delete <path> [--force]
//   centmem scope list
func cmdScope(args []string) int {
	fs := newFlagSet("scope")
	fs.Bool("force", false, "bypass confirmation prompt when deleting a scope")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		posArgs := fs.Args()
		if len(posArgs) == 0 {
			return cli.Invalidf("scope: action required (delete, list)")
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("scope: %v", err)
		}
		defer s.Close()

		ctx, cancel := signalContext()
		defer cancel()

		action := strings.ToLower(posArgs[0])
		switch action {
		case "delete":
			if len(posArgs) < 2 {
				return cli.Invalidf("scope delete: scope path required (e.g. centmem scope delete project:foo)")
			}
			targetPath := strings.TrimSpace(posArgs[1])
			if targetPath == "" {
				return cli.Invalidf("scope delete: scope path cannot be empty")
			}
			if targetPath == "global" {
				return cli.Invalidf("cannot delete root scope 'global'")
			}

			sc, err := scope.Parse(targetPath)
			if err != nil {
				return cli.Invalidf("scope delete: %v", err)
			}
			if sc.Kind == scope.Global {
				return cli.Invalidf("cannot delete root scope 'global'")
			}

			// Verify scope exists in database
			var rootID int64
			err = s.DB().QueryRowContext(ctx, "SELECT id FROM scopes WHERE path = ?", sc.Path).Scan(&rootID)
			if errors.Is(err, sql.ErrNoRows) {
				return cli.NotFoundf("scope %q not found", sc.Path)
			}
			if err != nil {
				return cli.Internalf("scope delete: lookup scope: %v", err)
			}

			// Calculate preview metrics (memories and sub-scopes)
			scopeIDs, err := s.ResolveScopeIDs(ctx, sc, false, true)
			if err != nil {
				return cli.Internalf("scope delete: resolve scopes: %v", err)
			}

			var memCount int
			subScopesCount := 0
			if len(scopeIDs) > 0 {
				placeholders := strings.TrimSuffix(strings.Repeat("?,", len(scopeIDs)), ",")
				scopeArgs := make([]any, len(scopeIDs))
				for i, id := range scopeIDs {
					scopeArgs[i] = id
				}
				_ = s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM memories WHERE scope_id IN ("+placeholders+")", scopeArgs...).Scan(&memCount)
				subScopesCount = len(scopeIDs) - 1
				if subScopesCount < 0 {
					subScopesCount = 0
				}
			}

			force := fs.Lookup("force") != nil && fs.Lookup("force").Value.String() == "true"
			if !force {
				isTerm := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
				if !isTerm {
					return cli.Invalidf("confirmation required: pass --force to delete scope in non-interactive environment")
				}
				fmt.Fprintf(os.Stderr, "Are you sure you want to delete scope %q (%d memories, %d sub-scopes)? [y/N]: ", sc.Path, memCount, subScopesCount)
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				line = strings.TrimSpace(strings.ToLower(line))
				if line != "y" && line != "yes" {
					return cli.Invalidf("aborted")
				}
			}

			summary, err := s.DeleteScopeTree(ctx, sc.Path)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return cli.NotFoundf("scope %q not found", sc.Path)
				}
				return cli.Internalf("scope delete: %v", err)
			}

			return prettyPrint(fs, map[string]any{
				"ok":               true,
				"deleted_scope":    summary.ScopePath,
				"memories_deleted": summary.MemoriesDeleted,
				"scopes_deleted":   summary.ScopesDeleted,
			})

		case "list":
			tree, err := s.ListScopeTree(ctx)
			if err != nil {
				return cli.Internalf("scope list: %v", err)
			}
			return prettyPrint(fs, map[string]any{
				"ok":     true,
				"scopes": tree,
			})

		default:
			return cli.Invalidf("scope: unknown action %q (expected delete, list)", action)
		}
	})
}
