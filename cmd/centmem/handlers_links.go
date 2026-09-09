package main

import (
	"context"
	"errors"
	"flag"
	"strconv"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// reorderFlags moves flags and their arguments before positional arguments
// so Go's standard flag.FlagSet parses flags even when positioned after arguments.
func reorderFlags(args []string, fs *flag.FlagSet) []string {
	var flagArgs []string
	var posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if strings.Contains(arg, "=") {
				flagArgs = append(flagArgs, arg)
				continue
			}
			name := strings.TrimLeft(arg, "-")
			f := fs.Lookup(name)
			if f != nil {
				if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
					flagArgs = append(flagArgs, arg)
					continue
				}
				flagArgs = append(flagArgs, arg)
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flagArgs = append(flagArgs, args[i])
				}
				continue
			}
			flagArgs = append(flagArgs, arg)
		} else {
			posArgs = append(posArgs, arg)
		}
	}
	return append(flagArgs, posArgs...)
}

// cmdLink handles manual link creation and confirmation/dismissal of suggestions.
// Syntax:
//   centmem link <from_id> <to_id> --relation <rel>
//   centmem link confirm <link_id>
//   centmem link dismiss <link_id>
func cmdLink(args []string) int {
	fs := newFlagSet("link")
	fs.String("relation", "", "relationship type (supports, refines, contradicts, depends-on, supersedes)")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		posArgs := fs.Args()
		if len(posArgs) == 0 {
			return cli.E(cli.ExitError, "INVALID", "link: missing arguments",
				"usage: centmem link <from_id> <to_id> --relation <rel> | link <confirm|dismiss> <link_id>")
		}

		ctx := context.Background()

		// Subcommand: link confirm <link_id> | link dismiss <link_id>
		action := posArgs[0]
		if action == "confirm" || action == "dismiss" {
			if len(posArgs) < 2 {
				return cli.Invalidf("link %s: missing link_id", action)
			}
			linkID, err := strconv.ParseInt(posArgs[1], 10, 64)
			if err != nil || linkID <= 0 {
				return cli.Invalidf("link %s: invalid link_id %q", action, posArgs[1])
			}

			if client, ok := getDaemonClient(cfg, fs); ok {
				defer client.Close()
				resp, err := client.Link(ctx, &centmemv1.LinkRequest{
					Action: action,
					LinkId: linkID,
				})
				if err != nil {
					return mapRPCErr(err)
				}
				if action == "confirm" {
					return prettyPrint(fs, map[string]any{
						"ok":        resp.Ok,
						"confirmed": linkID,
					})
				}
				return prettyPrint(fs, map[string]any{
					"ok":        resp.Ok,
					"dismissed": linkID,
				})
			}

			s, err := store.Open(cfg)
			if err != nil {
				return cli.Internalf("link: %v", err)
			}
			defer s.Close()

			if action == "confirm" {
				if err := s.ConfirmLink(ctx, linkID); err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return cli.NotFoundf("link %d not found", linkID)
					}
					return cli.Internalf("confirm link: %v", err)
				}
				return prettyPrint(fs, map[string]any{
					"ok":        true,
					"confirmed": linkID,
				})
			}

			if action == "dismiss" {
				if err := s.DismissLink(ctx, linkID); err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return cli.NotFoundf("suggested link %d not found", linkID)
					}
					return cli.Internalf("dismiss link: %v", err)
				}
				return prettyPrint(fs, map[string]any{
					"ok":        true,
					"dismissed": linkID,
				})
			}
		}

		// Standard link: centmem link <from_id> <to_id> --relation <rel>
		if len(posArgs) < 2 {
			return cli.Invalidf("link: requires <from_id> <to_id> and --relation")
		}

		fromID, err := strconv.ParseInt(posArgs[0], 10, 64)
		if err != nil || fromID <= 0 {
			return cli.Invalidf("link: invalid from_id %q", posArgs[0])
		}
		toID, err := strconv.ParseInt(posArgs[1], 10, 64)
		if err != nil || toID <= 0 {
			return cli.Invalidf("link: invalid to_id %q", posArgs[1])
		}

		relation := fs.Lookup("relation").Value.String()
		if relation == "" {
			return cli.Invalidf("link: --relation is required (supports, refines, contradicts, depends-on, supersedes)")
		}

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Link(ctx, &centmemv1.LinkRequest{
				FromId:   fromID,
				ToId:     toID,
				Relation: relation,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			linkMap := map[string]any{}
			if resp.Link != nil {
				linkMap = map[string]any{
					"id":         resp.Link.Id,
					"from_id":    resp.Link.FromId,
					"to_id":      resp.Link.ToId,
					"relation":   resp.Link.Relation,
					"suggested":  resp.Link.Suggested,
					"created_at": resp.Link.CreatedAt,
				}
			}
			return prettyPrint(fs, map[string]any{
				"ok":   true,
				"link": linkMap,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("link: %v", err)
		}
		defer s.Close()

		link, err := s.CreateLink(ctx, fromID, toID, relation, false)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return cli.NotFoundf("%v", err)
			}
			return cli.Invalidf("link: %v", err)
		}

		return prettyPrint(fs, map[string]any{
			"ok": true,
			"link": map[string]any{
				"id":         link.ID,
				"from_id":    link.FromID,
				"to_id":      link.ToID,
				"relation":   link.Relation,
				"suggested":  link.Suggested,
				"created_at": link.CreatedAt.Unix(),
			},
		})
	})
}

// cmdUnlink handles link deletion.
// Syntax:
//   centmem unlink <from_id> <to_id> [--relation <rel>]
//   centmem unlink --id <link_id>
func cmdUnlink(args []string) int {
	fs := newFlagSet("unlink")
	fs.Int64("id", 0, "link id to delete")
	fs.String("relation", "", "specific relation type to delete")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		ctx := context.Background()
		linkID := int64Flag(fs, "id", 0)

		if linkID > 0 {
			if client, ok := getDaemonClient(cfg, fs); ok {
				defer client.Close()
				resp, err := client.Unlink(ctx, &centmemv1.UnlinkRequest{LinkId: linkID})
				if err != nil {
					return mapRPCErr(err)
				}
				return prettyPrint(fs, map[string]any{
					"ok":      true,
					"deleted": resp.Deleted,
				})
			}

			s, err := store.Open(cfg)
			if err != nil {
				return cli.Internalf("unlink: %v", err)
			}
			defer s.Close()

			n, err := s.DeleteLinkByID(ctx, linkID)
			if err != nil {
				return cli.Internalf("unlink by id: %v", err)
			}
			return prettyPrint(fs, map[string]any{
				"ok":      true,
				"deleted": n,
			})
		}

		posArgs := fs.Args()
		if len(posArgs) < 2 {
			return cli.E(cli.ExitError, "INVALID", "unlink: missing arguments",
				"usage: centmem unlink <from_id> <to_id> [--relation <rel>] | unlink --id <link_id>")
		}

		fromID, err := strconv.ParseInt(posArgs[0], 10, 64)
		if err != nil || fromID <= 0 {
			return cli.Invalidf("unlink: invalid from_id %q", posArgs[0])
		}
		toID, err := strconv.ParseInt(posArgs[1], 10, 64)
		if err != nil || toID <= 0 {
			return cli.Invalidf("unlink: invalid to_id %q", posArgs[1])
		}

		relation := fs.Lookup("relation").Value.String()
		if relation != "" && !store.IsValidLinkRelation(relation) {
			return cli.Invalidf("unlink: invalid relation %q; must be one of: %s",
				relation, strings.Join(store.ValidLinkRelations, ", "))
		}

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Unlink(ctx, &centmemv1.UnlinkRequest{
				FromId:   fromID,
				ToId:     toID,
				Relation: relation,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			return prettyPrint(fs, map[string]any{
				"ok":      true,
				"deleted": resp.Deleted,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("unlink: %v", err)
		}
		defer s.Close()

		n, err := s.DeleteLink(ctx, fromID, toID, relation)
		if err != nil {
			return cli.Internalf("unlink: %v", err)
		}

		return prettyPrint(fs, map[string]any{
			"ok":      true,
			"deleted": n,
		})
	})
}

// cmdLinks lists relationships for a specified memory.
// Syntax:
//   centmem links <memory_id> [--all] [--include-suggested]
func cmdLinks(args []string) int {
	fs := newFlagSet("links")
	fs.Bool("all", false, "include auto-suggested links pending confirmation")
	fs.Bool("include-suggested", false, "include auto-suggested links pending confirmation")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		posArgs := fs.Args()
		if len(posArgs) == 0 {
			return cli.E(cli.ExitError, "INVALID", "links: missing memory_id",
				"usage: centmem links <memory_id> [--all]")
		}

		memoryID, err := strconv.ParseInt(posArgs[0], 10, 64)
		if err != nil || memoryID <= 0 {
			return cli.Invalidf("links: invalid memory_id %q", posArgs[0])
		}

		includeSuggested := fs.Lookup("all").Value.String() == "true" ||
			fs.Lookup("include-suggested").Value.String() == "true"

		ctx := context.Background()

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Links(ctx, &centmemv1.LinksRequest{
				MemoryId:         memoryID,
				All:              includeSuggested,
				IncludeSuggested: includeSuggested,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			outList := make([]map[string]any, 0, len(resp.Outgoing))
			for _, l := range resp.Outgoing {
				outList = append(outList, map[string]any{
					"link_id":        l.Id,
					"relation":       l.Relation,
					"target_id":      l.ToId,
					"target_type":    l.TargetType,
					"target_content": l.TargetContent,
					"suggested":      l.Suggested,
				})
			}
			inList := make([]map[string]any, 0, len(resp.Incoming))
			for _, l := range resp.Incoming {
				inList = append(inList, map[string]any{
					"link_id":        l.Id,
					"relation":       l.Relation,
					"source_id":      l.FromId,
					"source_type":    l.SourceType,
					"source_content": l.SourceContent,
					"suggested":      l.Suggested,
				})
			}
			return prettyPrint(fs, map[string]any{
				"ok":        true,
				"memory_id": memoryID,
				"outgoing":  outList,
				"incoming":  inList,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("links: %v", err)
		}
		defer s.Close()

		// Verify memory exists before querying its links
		if _, err := s.GetMemory(ctx, memoryID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return cli.NotFoundf("memory %d not found", memoryID)
			}
			return cli.Internalf("get memory %d: %v", memoryID, err)
		}

		outgoing, incoming, err := s.GetLinksForMemory(ctx, memoryID, includeSuggested)
		if err != nil {
			return cli.Internalf("get links: %v", err)
		}

		outList := make([]map[string]any, 0, len(outgoing))
		for _, l := range outgoing {
			outList = append(outList, map[string]any{
				"link_id":        l.ID,
				"relation":       l.Relation,
				"target_id":      l.ToID,
				"target_type":    l.TargetType,
				"target_content": l.TargetContent,
				"suggested":      l.Suggested,
			})
		}

		inList := make([]map[string]any, 0, len(incoming))
		for _, l := range incoming {
			inList = append(inList, map[string]any{
				"link_id":        l.ID,
				"relation":       l.Relation,
				"source_id":      l.FromID,
				"source_type":    l.SourceType,
				"source_content": l.SourceContent,
				"suggested":      l.Suggested,
			})
		}

		return prettyPrint(fs, map[string]any{
			"ok":        true,
			"memory_id": memoryID,
			"outgoing":  outList,
			"incoming":  inList,
		})
	})
}
