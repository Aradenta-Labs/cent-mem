package main

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdProposals manages human-in-the-loop agent proposals (list, show, apply, dismiss).
// Syntax:
//   centmem proposals list [--scope <scope>] [--status pending|applied|dismissed] [--limit N] [--offset N]
//   centmem proposals show <id>
//   centmem proposals apply <id>
//   centmem proposals dismiss <id>
func cmdProposals(args []string) int {
	fs := newFlagSet("proposals")
	fs.String("scope", "", "scope path filter")
	fs.String("status", "", "proposal status filter (pending, applied, dismissed)")
	fs.Int("limit", 50, "maximum number of proposals to list")
	fs.Int("offset", 0, "offset for pagination")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		posArgs := fs.Args()
		if len(posArgs) == 0 {
			return cli.Invalidf("proposals: action required (list, show, apply, dismiss)")
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("proposals: %v", err)
		}
		defer s.Close()

		ctx, cancel := signalContext()
		defer cancel()

		action := strings.ToLower(posArgs[0])

		switch action {
		case "list":
			status := strings.ToLower(fs.Lookup("status").Value.String())
			if status != "" {
				switch status {
				case "pending", "applied", "dismissed":
				default:
					return cli.Invalidf("proposals list: invalid --status %q (expected pending, applied, or dismissed)", status)
				}
			}

			scopePath := fs.Lookup("scope").Value.String()
			if scopePath != "" {
				if _, err := scope.Parse(scopePath); err != nil {
					return cli.Invalidf("proposals list: invalid --scope %q: %v", scopePath, err)
				}
			}
			limit := intFlag(fs, "limit", 50)
			if limit <= 0 {
				limit = 50
			}
			offset := intFlag(fs, "offset", 0)
			if offset < 0 {
				offset = 0
			}

			proposals, err := s.ListProposals(ctx, store.ProposalListQuery{
				ScopePath: scopePath,
				Status:    status,
				Limit:     limit,
				Offset:    offset,
			})
			if err != nil {
				return cli.Internalf("proposals list: %v", err)
			}
			if proposals == nil {
				proposals = []store.Proposal{}
			}

			return prettyPrint(fs, map[string]any{
				"ok":        true,
				"proposals": proposals,
			})

		case "show":
			if len(posArgs) < 2 {
				return cli.Invalidf("proposals show: proposal id required")
			}
			id, err := strconv.ParseInt(posArgs[1], 10, 64)
			if err != nil || id <= 0 {
				return cli.Invalidf("proposals show: invalid proposal id %q", posArgs[1])
			}

			p, err := s.GetProposal(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				return cli.E(cli.ExitNotFound, "NOT_FOUND", fmt.Sprintf("proposal %d not found", id),
					"use 'centmem proposals list' to view available proposals")
			}
			if err != nil {
				return cli.Internalf("proposals show: %v", err)
			}

			return prettyPrint(fs, map[string]any{
				"ok":       true,
				"proposal": p,
			})

		case "apply":
			if len(posArgs) < 2 {
				return cli.Invalidf("proposals apply: proposal id required")
			}
			id, err := strconv.ParseInt(posArgs[1], 10, 64)
			if err != nil || id <= 0 {
				return cli.Invalidf("proposals apply: invalid proposal id %q", posArgs[1])
			}

			if err := s.ApplyProposal(ctx, id); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return cli.E(cli.ExitNotFound, "NOT_FOUND", fmt.Sprintf("proposal %d not found", id),
						"use 'centmem proposals list' to view available proposals")
				}
				if errors.Is(err, store.ErrProposalConflict) || strings.Contains(err.Error(), "already applied") || strings.Contains(err.Error(), "cannot apply") {
					return cli.E(cli.ExitConflict, "CONFLICT", fmt.Sprintf("proposals apply: %v", err),
						"verify proposal status with 'centmem proposals show'")
				}
				return cli.Internalf("proposals apply: %v", err)
			}

			p, _ := s.GetProposal(ctx, id)
			return prettyPrint(fs, map[string]any{
				"ok":       true,
				"applied":  true,
				"proposal": p,
			})

		case "dismiss":
			if len(posArgs) < 2 {
				return cli.Invalidf("proposals dismiss: proposal id required")
			}
			id, err := strconv.ParseInt(posArgs[1], 10, 64)
			if err != nil || id <= 0 {
				return cli.Invalidf("proposals dismiss: invalid proposal id %q", posArgs[1])
			}

			if err := s.DismissProposal(ctx, id); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return cli.E(cli.ExitNotFound, "NOT_FOUND", fmt.Sprintf("proposal %d not found", id),
						"use 'centmem proposals list' to view available proposals")
				}
				if errors.Is(err, store.ErrProposalConflict) || strings.Contains(err.Error(), "already applied") {
					return cli.E(cli.ExitConflict, "CONFLICT", fmt.Sprintf("proposals dismiss: %v", err),
						"proposal cannot be dismissed once applied")
				}
				return cli.Internalf("proposals dismiss: %v", err)
			}

			p, _ := s.GetProposal(ctx, id)
			return prettyPrint(fs, map[string]any{
				"ok":        true,
				"dismissed": true,
				"proposal":  p,
			})

		default:
			return cli.Invalidf("proposals: unknown action %q (expected list, show, apply, or dismiss)", posArgs[0])
		}
	})
}
