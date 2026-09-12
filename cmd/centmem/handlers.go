package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/compact"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func cmdInit(args []string) int {
	fs := newFlagSet("init")
	fs.String("model", "bge-small-en-v1.5", "embedding model name")
	fs.Bool("force", false, "re-download model even if present")
	fs.Bool("non-interactive", false, "skip interactive capture setup wizard")
	fs.Bool("wizard", false, "force run interactive capture setup wizard")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		if err := cfg.Ensure(); err != nil {
			return cli.Internalf("init: %v", err)
		}
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("init: %v", err)
		}
		defer s.Close()

		modelName := fs.Lookup("model").Value.String()
		model, ok := embed.ModelCatalog[modelName]
		if !ok {
			return cli.Invalidf("init: unknown model %q", modelName)
		}
		cfg.Model.Name = modelName
		cfg.Model.Dims = model.Dims
		cfg.Model.Path = cfg.Home + "/models/" + modelName + ".onnx"
		force := fs.Lookup("force").Value.String() == "true"
		nonInteractive := fs.Lookup("non-interactive").Value.String() == "true"
		forceWizard := fs.Lookup("wizard").Value.String() == "true"

		modelPath := cfg.Model.Path
		if force {
			os.Remove(modelPath)
		}
		path, err := embed.Downloader(modelName, modelPath)
		if err != nil {
			return cli.E(cli.ExitError, "INTERNAL", fmt.Sprintf("init: download model: %v", err),
				"ensure the model file can be downloaded, or place it manually at the model path; see docs/guides/troubleshooting.md")
		}
		_ = path

		// Interactive capture wizard
		configPath := filepath.Join(cfg.Home, "config.toml")
		_, configErr := os.Stat(configPath)
		shouldRunWizard := forceWizard || (!nonInteractive && os.IsNotExist(configErr))

		if shouldRunWizard {
			_ = runCaptureInitWizard(os.Stdin, os.Stderr, &cfg)
		}

		if err := config.SaveToHome(cfg); err != nil {
			return cli.Internalf("init: save config: %v", err)
		}
		if err := writeDefaultCapturePrompt(cfg.Home); err != nil {
			return cli.Internalf("init: write capture prompt: %v", err)
		}

		return prettyPrint(fs, map[string]any{
			"ok":    true,
			"db":    cfg.DBPath,
			"model": modelName,
			"dims":  cfg.Model.Dims,
		})
	})
}

// ---------------------------------------------------------------------------
// put
// ---------------------------------------------------------------------------

func cmdPut(args []string) int {
	fs := newFlagSet("put")
	fs.String("scope", "", "scope path")
	fs.String("type", "note", "memory type: note|log")
	fs.String("content", "", "memory content")
	fs.String("tags", "", "comma-separated tags")
	fs.String("source-agent", "", "source agent id")
	fs.String("source-session", "", "source session id")
	fs.Bool("no-suggest", false, "bypass relationship auto-suggestion")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		scopePath := fs.Lookup("scope").Value.String()
		typ := fs.Lookup("type").Value.String()
		content := fs.Lookup("content").Value.String()
		if scopePath == "" {
			return cli.E(cli.ExitError, "INVALID", "put: --scope is required",
				"pass --scope project:<name> or --scope global")
		}
		if content == "" {
			return cli.E(cli.ExitError, "INVALID", "put: --content is required",
				"pass --content '<your memory text>'")
		}
		if typ != "note" && typ != "log" {
			return cli.Invalidf("put: --type must be note or log")
		}

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			noSuggest := fs.Lookup("no-suggest") != nil && fs.Lookup("no-suggest").Value.String() == "true"
			resp, err := client.Put(context.Background(), &centmemv1.PutRequest{
				Scope:         scopePath,
				Type:          typ,
				Content:       content,
				Tags:          splitCSV(fs.Lookup("tags").Value.String()),
				SourceAgent:   fs.Lookup("source-agent").Value.String(),
				SourceSession: fs.Lookup("source-session").Value.String(),
				NoSuggest:     noSuggest,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			out := map[string]any{
				"ok":     resp.Ok,
				"id":     resp.Id,
				"scope":  resp.Scope,
				"status": resp.Status,
			}
			if len(resp.SuggestedLinks) > 0 {
				suggs := make([]map[string]any, 0, len(resp.SuggestedLinks))
				for _, sl := range resp.SuggestedLinks {
					suggs = append(suggs, map[string]any{
						"id":             sl.LinkId,
						"from_id":        sl.FromId,
						"to_id":          sl.ToId,
						"relation":       sl.Relation,
						"target_content": sl.TargetContent,
						"target_type":    sl.TargetType,
					})
				}
				out["suggested_links"] = suggs
			}
			return prettyPrint(fs, out)
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("put: %v", err)
		}
		defer s.Close()

		var summarizeAt *int64
		if days := retentionDays(cfg, typ); days > 0 {
			ts := time.Now().Add(time.Duration(days) * 24 * time.Hour).UnixMicro()
			summarizeAt = &ts
		}

		id, status, err := s.PutMemory(context.Background(), store.MemoryInput{
			Scope:         scopePath,
			Type:          typ,
			Content:       content,
			Tags:          splitCSV(fs.Lookup("tags").Value.String()),
			SourceAgent:   fs.Lookup("source-agent").Value.String(),
			SourceSession: fs.Lookup("source-session").Value.String(),
			SummarizeAt:   summarizeAt,
		})
		if err != nil {
			return cli.Internalf("put: %v", err)
		}

		drainQueue(cfg, s)

		out := map[string]any{
			"ok":     true,
			"id":     id,
			"scope":  scopePath,
			"status": status,
		}

		noSuggest := fs.Lookup("no-suggest") != nil && fs.Lookup("no-suggest").Value.String() == "true"
		if !noSuggest {
			memRow, err := s.GetMemory(context.Background(), id)
			if err == nil && memRow != nil {
				emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
				if emb != nil {
					defer emb.Close()
				}
				searcher := search.New(s).WithEmbedder(emb)
				suggestions, err := searcher.SuggestLinks(context.Background(), *memRow)
				if err == nil && len(suggestions) > 0 {
					out["suggested_links"] = suggestions
				}
			}
		}

		return prettyPrint(fs, out)
	})
}

// ---------------------------------------------------------------------------
// set
// ---------------------------------------------------------------------------

func cmdSet(args []string) int {
	fs := newFlagSet("set")
	fs.String("scope", "", "scope path")
	fs.String("key", "", "fact key")
	fs.String("value", "", "JSON value")
	fs.String("tags", "", "comma-separated tags")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		scopePath := fs.Lookup("scope").Value.String()
		key := fs.Lookup("key").Value.String()
		value := fs.Lookup("value").Value.String()
		if scopePath == "" || key == "" {
			return cli.Invalidf("set: --scope and --key are required")
		}

		// Validate/normalize value as JSON; fall back to string.
		norm, err := normalizeValue(value)
		if err != nil {
			return cli.Invalidf("set: invalid --value JSON: %v", err)
		}

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Set(context.Background(), &centmemv1.SetRequest{
				Scope: scopePath,
				Key:   key,
				Value: norm,
				Tags:  splitCSV(fs.Lookup("tags").Value.String()),
			})
			if err != nil {
				return mapRPCErr(err)
			}
			return prettyPrint(fs, map[string]any{
				"ok":     resp.Ok,
				"id":     resp.Id,
				"key":    resp.Key,
				"status": resp.Status,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("set: %v", err)
		}
		defer s.Close()

		id, status, err := s.SetFact(context.Background(), store.FactInput{
			Scope: scopePath, Key: key, Value: norm,
			Tags: splitCSV(fs.Lookup("tags").Value.String()),
		})
		if err != nil {
			return cli.Internalf("set: %v", err)
		}

		drainQueue(cfg, s)

		return prettyPrint(fs, map[string]any{
			"ok":     true,
			"id":     id,
			"key":    key,
			"status": status,
		})
	})
}

// ---------------------------------------------------------------------------
// get
// ---------------------------------------------------------------------------

func cmdGet(args []string) int {
	fs := newFlagSet("get")
	fs.String("scope", "", "scope path")
	fs.String("key", "", "fact key")
	fs.Bool("inherit", true, "walk ancestor scopes")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		scopePath := fs.Lookup("scope").Value.String()
		key := fs.Lookup("key").Value.String()
		inherit := fs.Lookup("inherit").Value.String() == "true"

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Get(context.Background(), &centmemv1.GetRequest{
				Scope:   scopePath,
				Key:     key,
				Inherit: inherit,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			var value any
			if err := json.Unmarshal([]byte(resp.Value), &value); err != nil {
				value = resp.Value
			}
			return prettyPrint(fs, map[string]any{
				"ok":    true,
				"key":   resp.Key,
				"value": value,
				"scope": resp.Scope,
				"id":    resp.Id,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("get: %v", err)
		}
		defer s.Close()

		f, err := s.GetFact(context.Background(), scopePath, key, inherit)
		if errors.Is(err, sql.ErrNoRows) {
			return cli.E(cli.ExitNotFound, "NOT_FOUND",
				fmt.Sprintf("fact %q not found in scope %q", key, scopePath),
				"use centmem set --scope <scope> --key <k> --value <v> to create it first")
		}
		if err != nil {
			return cli.Internalf("get: %v", err)
		}

		// Try to decode fact value as JSON; if valid JSON (object/array), return decoded.
		// If it's a raw string (not valid JSON), return as string.
		var value any
		if err := json.Unmarshal([]byte(f.Value), &value); err != nil {
			value = f.Value
		}
		return prettyPrint(fs, map[string]any{
			"ok":    true,
			"key":   f.Key,
			"value": value,
			"scope": f.ScopePath,
			"id":    f.ID,
		})
	})
}

// ---------------------------------------------------------------------------
// recall
// ---------------------------------------------------------------------------

func cmdRecall(args []string) int {
	fs := newFlagSet("recall")
	fs.String("scope", "", "scope path")
	fs.Int("top", 5, "number of results")
	fs.String("type", "", "memory type filter")
	fs.String("tags", "", "comma-separated tags")
	fs.String("since", "", "since duration (e.g. 7d, 24h)")
	fs.String("until", "", "until duration (e.g. 1d)")
	fs.String("agent", "", "source agent filter")
	fs.String("caller-agent", "", "caller agent identifier for affinity boosting")
	fs.String("reranker", "", "re-ranker strategy override")
	fs.Bool("inherit", true, "include ancestor scopes")
	fs.Bool("children", false, "include descendant scopes")
	fs.Bool("include-links", false, "include 1-hop connected memory links")
	fs.Bool("include-suggested", false, "include pending suggested links in link expansion")
	return runCommandQuery(args, fs, func(cfg config.Config, fs *flag.FlagSet, query string) error {
		callerAgent := fs.Lookup("caller-agent").Value.String()
		if callerAgent == "" {
			callerAgent = os.Getenv("CENTMEM_AGENT")
		}

		rerankerChoice := cfg.Search.Reranker
		if flagReranker := fs.Lookup("reranker").Value.String(); flagReranker != "" {
			switch strings.ToLower(flagReranker) {
			case "composite", "none", "cross_encoder", "llm":
				rerankerChoice = strings.ToLower(flagReranker)
			default:
				return cli.Invalidf("recall: unknown --reranker %q (expected composite, none, cross_encoder, or llm)", flagReranker)
			}
		}

		top := intFlag(fs, "top", 5)
		if top > 20 {
			top = 20
		}

		includeSuggested := fs.Lookup("include-suggested").Value.String() == "true"
		includeLinks := fs.Lookup("include-links").Value.String() == "true" || includeSuggested

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			req := &centmemv1.RecallRequest{
				Text:             query,
				Scope:            fs.Lookup("scope").Value.String(),
				Top:              int32(top),
				Type:             fs.Lookup("type").Value.String(),
				Tags:             splitCSV(fs.Lookup("tags").Value.String()),
				Since:            fs.Lookup("since").Value.String(),
				Until:            fs.Lookup("until").Value.String(),
				Agent:            fs.Lookup("agent").Value.String(),
				Inherit:          fs.Lookup("inherit").Value.String() == "true",
				Children:         fs.Lookup("children").Value.String() == "true",
				CallerAgent:      callerAgent,
				Reranker:         rerankerChoice,
				IncludeLinks:     includeLinks,
				IncludeSuggested: includeSuggested,
			}
			resp, err := client.Recall(context.Background(), req)
			if err != nil {
				return mapRPCErr(err)
			}
			out := make([]map[string]any, 0, len(resp.Results))
			for _, r := range resp.Results {
				var lastAccessed any
				if r.AccessCount > 0 && r.LastAccessedAt > 0 {
					lastAccessed = r.LastAccessedAt
				}
				item := map[string]any{
					"id":               r.Id,
					"type":             r.Type,
					"scope":            r.Scope,
					"content":          r.Content,
					"tags":             r.Tags,
					"created_at":       r.CreatedAt,
					"score":            round4(r.Score),
					"matched_by":       r.MatchedBy,
					"access_count":     r.AccessCount,
					"last_accessed_at": lastAccessed,
				}
				if includeLinks {
					linksList := make([]map[string]any, 0, len(r.Links))
					for _, l := range r.Links {
						lMap := map[string]any{
							"relation":       l.Relation,
							"direction":      l.Direction,
							"linked_id":      l.LinkedId,
							"linked_content": l.LinkedContent,
						}
						if l.Suggested {
							lMap["suggested"] = true
						}
						if l.LinkId > 0 {
							lMap["link_id"] = l.LinkId
						}
						linksList = append(linksList, lMap)
					}
					item["links"] = linksList
				}
				out = append(out, item)
			}
			return prettyPrint(fs, map[string]any{
				"ok":      true,
				"query":   query,
				"results": out,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("recall: %v", err)
		}
		defer s.Close()

		emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
		searcher := search.New(s).
			WithEmbedder(emb).
			WithDecayDays(cfg.Search.DecayHalfLifeDays).
			WithRerankerName(rerankerChoice).
			WithRerankWindow(cfg.Search.RerankWindow).
			WithSessionBoost(cfg.Search.SessionBoost).
			WithAgentBoost(cfg.Search.AgentBoost).
			WithImportance(cfg.Search.ImportanceBoostEnabled, cfg.Search.ImportanceWeight, cfg.Search.ImportanceCap)
		text := query
		q := search.Query{
			Text:                  text,
			Scope:                 fs.Lookup("scope").Value.String(),
			Inherit:               fs.Lookup("inherit").Value.String() == "true",
			Children:              fs.Lookup("children").Value.String() == "true",
			Top:                   top,
			Type:                  fs.Lookup("type").Value.String(),
			Tags:                  splitCSV(fs.Lookup("tags").Value.String()),
			Agent:                 fs.Lookup("agent").Value.String(),
			CallerAgent:           callerAgent,
			IncludeLinks:          includeLinks,
			IncludeSuggestedLinks: includeSuggested,
		}

		now := time.Now()
		if since := fs.Lookup("since").Value.String(); since != "" {
			d, err := parseDuration(since)
			if err != nil {
				return cli.Invalidf("recall: bad --since: %v", err)
			}
			q.Since = now.Add(-d)
		}
		if until := fs.Lookup("until").Value.String(); until != "" {
			d, err := parseDuration(until)
			if err != nil {
				return cli.Invalidf("recall: bad --until: %v", err)
			}
			q.Until = now.Add(-d)
		}

		results, err := searcher.Recall(context.Background(), q)
		if err != nil {
			return cli.Internalf("recall: %v", err)
		}

		if len(results) > 0 {
			returnedIDs := make([]int64, len(results))
			for i, r := range results {
				returnedIDs[i] = r.ID
			}
			s.RecordAccessAsync(returnedIDs)
		}

		out := make([]map[string]any, 0, len(results))
		for _, r := range results {
			var lastAccessed any
			if r.AccessCount > 0 && r.LastAccessedAt != nil {
				lastAccessed = r.LastAccessedAt.Unix()
			}
			item := map[string]any{
				"id":               r.ID,
				"type":             r.Type,
				"scope":            r.Scope,
				"content":          r.Content,
				"tags":             r.Tags,
				"created_at":       r.CreatedAt.Unix(),
				"score":            round4(r.Score),
				"matched_by":       r.MatchedBy,
				"access_count":     r.AccessCount,
				"last_accessed_at": lastAccessed,
			}
			if includeLinks {
				linksList := make([]map[string]any, 0, len(r.Links))
				for _, l := range r.Links {
					lMap := map[string]any{
						"relation":       l.Relation,
						"direction":      l.Direction,
						"linked_id":      l.LinkedID,
						"linked_content": l.LinkedContent,
					}
					if l.Suggested {
						lMap["suggested"] = true
					}
					if l.LinkID > 0 {
						lMap["link_id"] = l.LinkID
					}
					linksList = append(linksList, lMap)
				}
				item["links"] = linksList
			}
			out = append(out, item)
		}

		return prettyPrint(fs, map[string]any{
			"ok":      true,
			"query":   text,
			"results": out,
		})
	})
}

// ---------------------------------------------------------------------------
// timeline
// ---------------------------------------------------------------------------

func cmdTimeline(args []string) int {
	fs := newFlagSet("timeline")
	fs.String("scope", "", "scope path")
	fs.String("since", "24h", "since duration")
	fs.String("until", "", "until duration")
	fs.Int("limit", 50, "max entries")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Timeline(context.Background(), &centmemv1.TimelineRequest{
				Scope: fs.Lookup("scope").Value.String(),
				Since: fs.Lookup("since").Value.String(),
				Until: fs.Lookup("until").Value.String(),
				Limit: int32(intFlag(fs, "limit", 50)),
			})
			if err != nil {
				return mapRPCErr(err)
			}
			entries := make([]map[string]any, 0, len(resp.Items))
			for _, r := range resp.Items {
				entries = append(entries, map[string]any{
					"id":         r.Id,
					"content":    r.Content,
					"created_at": r.CreatedAt,
					"scope":      r.Scope,
					"tags":       r.Tags,
				})
			}
			return prettyPrint(fs, map[string]any{"ok": true, "entries": entries})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("timeline: %v", err)
		}
		defer s.Close()

		searcher := search.New(s)
		now := time.Now()
		q := search.Query{
			Scope:   fs.Lookup("scope").Value.String(),
			Inherit: true,
			Top:     intFlag(fs, "limit", 50),
		}
		if since := fs.Lookup("since").Value.String(); since != "" {
			d, err := parseDuration(since)
			if err != nil {
				return cli.Invalidf("timeline: bad --since: %v", err)
			}
			q.Since = now.Add(-d)
		}
		if until := fs.Lookup("until").Value.String(); until != "" {
			d, err := parseDuration(until)
			if err != nil {
				return cli.Invalidf("timeline: bad --until: %v", err)
			}
			q.Until = now.Add(-d)
		}

		results, err := searcher.Timeline(context.Background(), q, q.Top)
		if err != nil {
			return cli.Internalf("timeline: %v", err)
		}

		entries := make([]map[string]any, 0, len(results))
		for _, r := range results {
			entries = append(entries, map[string]any{
				"id":         r.ID,
				"content":    r.Content,
				"created_at": r.CreatedAt.Unix(),
				"scope":      r.Scope,
				"tags":       r.Tags,
			})
		}
		return prettyPrint(fs, map[string]any{"ok": true, "entries": entries})
	})
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func cmdList(args []string) int {
	fs := newFlagSet("list")
	fs.String("scope", "", "scope path")
	fs.String("type", "", "memory type")
	fs.String("tags", "", "comma-separated tags")
	fs.Int("limit", 20, "max results")
	fs.Int("offset", 0, "offset")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		scopePath := fs.Lookup("scope").Value.String()
		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.List(context.Background(), &centmemv1.ListRequest{
				Scope:  scopePath,
				Type:   fs.Lookup("type").Value.String(),
				Tags:   splitCSV(fs.Lookup("tags").Value.String()),
				Limit:  int32(intFlag(fs, "limit", 20)),
				Offset: int32(intFlag(fs, "offset", 0)),
			})
			if err != nil {
				return mapRPCErr(err)
			}
			out := make([]map[string]any, 0, len(resp.Items))
			for _, m := range resp.Items {
				out = append(out, map[string]any{
					"id":         m.Id,
					"type":       m.Type,
					"scope":      m.Scope,
					"content":    m.Content,
					"tags":       m.Tags,
					"created_at": m.CreatedAt,
				})
			}
			return prettyPrint(fs, out)
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("list: %v", err)
		}
		defer s.Close()
		sc, err := parseScopeOpt(scopePath)
		if err != nil {
			return cli.Invalidf("list: %v", err)
		}
		var scopeIDs []int64
		if sc != nil {
			ids, err := s.ResolveScopeIDs(context.Background(), *sc, true, false)
			if err != nil {
				return cli.Internalf("list: %v", err)
			}
			scopeIDs = ids
		}

		mems, err := s.List(context.Background(), store.ListQuery{
			ScopeIDs: scopeIDs,
			Type:     fs.Lookup("type").Value.String(),
			Tags:     splitCSV(fs.Lookup("tags").Value.String()),
			Limit:    intFlag(fs, "limit", 20),
			Offset:   intFlag(fs, "offset", 0),
		})
		if err != nil {
			return cli.Internalf("list: %v", err)
		}

		out := make([]map[string]any, 0, len(mems))
		for _, m := range mems {
			out = append(out, map[string]any{
				"id":         m.ID,
				"type":       m.Type,
				"scope":      m.ScopePath,
				"content":    m.Content,
				"tags":       m.Tags,
				"created_at": m.CreatedAt.Unix(),
			})
		}
		return prettyPrint(fs, out)
	})
}

// ---------------------------------------------------------------------------
// forget
// ---------------------------------------------------------------------------

func cmdForget(args []string) int {
	fs := newFlagSet("forget")
	fs.Int64("id", 0, "memory id")
	fs.String("scope", "", "scope path")
	fs.String("key", "", "fact key")
	fs.String("tag", "", "tag")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		id := int64Flag(fs, "id", 0)
		var scopePath, key, tag string
		var scopeP, keyP, tagP *string
		if sp := fs.Lookup("scope").Value.String(); sp != "" {
			scopePath = sp
			scopeP = &scopePath
		}
		if k := fs.Lookup("key").Value.String(); k != "" {
			key = k
			keyP = &key
		}
		if tg := fs.Lookup("tag").Value.String(); tg != "" {
			tag = tg
			tagP = &tag
		}

		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Forget(context.Background(), &centmemv1.ForgetRequest{
				Id:    id,
				Scope: scopePath,
				Key:   key,
				Tag:   tag,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			return prettyPrint(fs, map[string]any{"ok": resp.Ok, "deleted": resp.Deleted})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("forget: %v", err)
		}
		defer s.Close()

		var ids []int64
		if id > 0 {
			ids = []int64{id}
		}
		n, err := s.Forget(context.Background(), ids, scopeP, keyP, tagP)
		if err != nil {
			return cli.Internalf("forget: %v", err)
		}
		return prettyPrint(fs, map[string]any{"ok": true, "deleted": n})
	})
}

// ---------------------------------------------------------------------------
// stats
// ---------------------------------------------------------------------------

func cmdStats(args []string) int {
	fs := newFlagSet("stats")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			resp, err := client.Stats(context.Background(), &centmemv1.StatsRequest{})
			if err != nil {
				return mapRPCErr(err)
			}
			var lastCompact any
			if resp.LastCompactAt > 0 {
				lastCompact = resp.LastCompactAt
			}
			var impDist any
			if resp.ImportanceDistribution != nil {
				impDist = map[string]any{
					"zero_access":        resp.ImportanceDistribution.ZeroAccess,
					"low_access_1_5":     resp.ImportanceDistribution.LowAccess_1_5,
					"medium_access_6_20": resp.ImportanceDistribution.MediumAccess_6_20,
					"high_access_21_plus": resp.ImportanceDistribution.HighAccess_21Plus,
					"max_access_count":   resp.ImportanceDistribution.MaxAccessCount,
					"avg_access_count":   resp.ImportanceDistribution.AvgAccessCount,
				}
			}
			return prettyPrint(fs, map[string]any{
				"ok":                      true,
				"db_path":                 resp.DbPath,
				"db_size_mb":              round2(resp.DbSizeMb),
				"memories":                resp.TotalMemories,
				"by_type":                 resp.ByType,
				"by_scope":                resp.ByScope,
				"last_compact_at":         lastCompact,
				"pending_embeddings":      resp.PendingEmbedding,
				"importance_distribution": impDist,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("stats: %v", err)
		}
		defer s.Close()

		st, err := s.Stats(context.Background())
		if err != nil {
			return cli.Internalf("stats: %v", err)
		}

		var lastCompact any
		if st.LastCompactAt != nil {
			lastCompact = *st.LastCompactAt
		} else {
			lastCompact = nil
		}

		return prettyPrint(fs, map[string]any{
			"ok":                      true,
			"db_path":                 st.DBPath,
			"db_size_mb":              round2(st.DBSizeMB),
			"memories":                st.Memories,
			"by_type":                 st.ByType,
			"by_scope":                st.ByScope,
			"last_compact_at":         lastCompact,
			"pending_embeddings":      st.PendingEmbedding,
			"importance_distribution": st.ImportanceDistribution,
		})
	})
}

// ---------------------------------------------------------------------------
// compact
// ---------------------------------------------------------------------------

func cmdCompact(args []string) int {
	fs := newFlagSet("compact")
	fs.String("scope", "", "restrict compaction to a scope + descendants")
	fs.Bool("dry-run", false, "report what would be summarized without writing")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		if client, ok := getDaemonClient(cfg, fs); ok {
			defer client.Close()
			dryRun := fs.Lookup("dry-run").Value.String() == "true"
			resp, err := client.Compact(context.Background(), &centmemv1.CompactRequest{
				Scope:  fs.Lookup("scope").Value.String(),
				DryRun: dryRun,
			})
			if err != nil {
				return mapRPCErr(err)
			}
			ids := resp.NewMemoryIds
			if ids == nil {
				ids = []int64{}
			}
			return prettyPrint(fs, map[string]any{
				"ok":             resp.Ok,
				"summarized":     resp.Summarized,
				"archived":       resp.Archived,
				"new_memory_ids": ids,
				"dry_run":        resp.DryRun,
			})
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("compact: %v", err)
		}
		defer s.Close()

		policy := compact.Policy{
			FactKeepForever:        cfg.Retention.FactKeepDays == 0,
			NoteSummarizeAfterDays: cfg.Retention.NoteSummarizeAfterDays,
			LogSummarizeAfterDays:  cfg.Retention.LogSummarizeAfterDays,
		}

		ctx, cancel := signalContext()
		defer cancel()
		res, err := compact.Compact(ctx, s, compact.Options{
			Scope:  fs.Lookup("scope").Value.String(),
			DryRun: fs.Lookup("dry-run").Value.String() == "true",
			Policy: &policy,
		})
		if err != nil {
			return cli.Internalf("compact: %v", err)
		}

		ids := res.NewMemoryIDs
		if ids == nil {
			ids = []int64{}
		}
		return prettyPrint(fs, map[string]any{
			"ok":             true,
			"summarized":     res.Summarized,
			"archived":       res.Archived,
			"new_memory_ids": ids,
			"dry_run":        fs.Lookup("dry-run").Value.String() == "true",
		})
	})
}

// ---------------------------------------------------------------------------
// doctor / backup / restore
// ---------------------------------------------------------------------------

// doctorCheck is a single named health check result.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "warn" | "fail"
	Detail string `json:"detail,omitempty"`
}

func cmdDoctor(args []string) int {
	fs := newFlagSet("doctor")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		checks := []doctorCheck{}
		var warnings []string

		// 1. DB opens + integrity.
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("doctor: %v", err)
		}
		defer s.Close()

		var integrity string
		if err := s.DB().QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
			checks = append(checks, doctorCheck{Name: "integrity", Status: "fail", Detail: err.Error()})
		} else if integrity != "ok" {
			checks = append(checks, doctorCheck{Name: "integrity", Status: "fail", Detail: integrity})
		} else {
			checks = append(checks, doctorCheck{Name: "integrity", Status: "ok"})
		}

		// 2. Schema version matches binary.
		cur, _ := s.SchemaVersion()
		want := store.LatestSchemaVersion()
		if cur == "" || cur != want {
			checks = append(checks, doctorCheck{Name: "schema_version", Status: "fail",
				Detail: fmt.Sprintf("db=%q binary=%q (run init to migrate)", cur, want)})
		} else {
			checks = append(checks, doctorCheck{Name: "schema_version", Status: "ok", Detail: cur})
		}

		// 3. Extensions present.
		var vecTable int
		_ = s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memories_vec'`).Scan(&vecTable)
		fts5 := false
		if rows, err := s.DB().Query(`PRAGMA compile_options`); err == nil {
			for rows.Next() {
				var opt string
				if rows.Scan(&opt) == nil && strings.Contains(opt, "ENABLE_FTS5") {
					fts5 = true
				}
			}
			rows.Close()
		}
		extsOK := vecTable == 1 && fts5
		detail := fmt.Sprintf("sqlite_vec=%v fts5=%v", vecTable == 1, fts5)
		if !extsOK {
			checks = append(checks, doctorCheck{Name: "extensions", Status: "fail", Detail: detail})
		} else {
			checks = append(checks, doctorCheck{Name: "extensions", Status: "ok", Detail: detail})
		}

		// 4. Model file exists + sha256 matches catalog.
		model, known := embed.ModelCatalog[cfg.Model.Name]
		modelOK := true
		modelDetail := "model=" + cfg.Model.Name
		if !known {
			modelOK = false
			modelDetail += " unknown model in catalog"
		} else if _, err := os.Stat(cfg.Model.Path); err != nil {
			modelOK = false
			modelDetail += " missing (run: centmem init)"
		} else if sum, err := sha256File(cfg.Model.Path); err != nil {
			modelOK = false
			modelDetail += " sha256 error: " + err.Error()
		} else if sum != model.SHA256 {
			// A present-but-mismatched model is usable; flag it as a warning so the
			// user can re-download, but don't block on it.
			modelDetail += " sha256 mismatch (re-run: centmem init --force)"
			warnings = append(warnings, modelDetail)
		}
		if !modelOK {
			checks = append(checks, doctorCheck{Name: "model", Status: "fail", Detail: modelDetail})
		} else {
			checks = append(checks, doctorCheck{Name: "model", Status: "ok", Detail: modelDetail})
		}

		// 5. Embed queue backlog.
		var pending int64
		_ = s.DB().QueryRow(`SELECT COUNT(*) FROM embed_queue WHERE claimed_at IS NULL`).Scan(&pending)
		checks = append(checks, doctorCheck{Name: "embed_queue", Status: "ok", Detail: fmt.Sprintf("pending=%d", pending)})
		if pending > 1000 {
			warnings = append(warnings, fmt.Sprintf("embed queue backlog: %d pending embeddings", pending))
		}

		// 6. Permissions: home dir 0700, DB 0600.
		homeOK, homeDetail := checkPerm(cfg.Home, 0700)
		if !homeOK {
			checks = append(checks, doctorCheck{Name: "permissions", Status: "fail", Detail: homeDetail})
		} else {
			dbOK, dbDetail := checkPerm(cfg.DBPath, 0600)
			if !dbOK {
				checks = append(checks, doctorCheck{Name: "permissions", Status: "fail", Detail: dbDetail})
			} else {
				checks = append(checks, doctorCheck{Name: "permissions", Status: "ok", Detail: homeDetail + "; " + dbDetail})
			}
		}

		// 7. AI Agent connectivity.
		if !cfg.Agent.Enabled || cfg.LLM.Backend == "disabled" {
			checks = append(checks, doctorCheck{Name: "ai_agent", Status: "ok", Detail: "disabled in config"})
		} else {
			probe := agent.ProbeEndpoint(context.Background(), cfg.LLM, nil)
			if probe.OK {
				checks = append(checks, doctorCheck{
					Name:   "ai_agent",
					Status: "ok",
					Detail: fmt.Sprintf("endpoint=%s model=%s (%dms)", probe.Endpoint, probe.Model, probe.Latency.Milliseconds()),
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:   "ai_agent",
					Status: "warn",
					Detail: probe.Message,
				})
				warnings = append(warnings, fmt.Sprintf("AI Agent: %s", probe.Message))
			}
		}

		// Determine overall status (only fail causes exit code 1; warn reports warnings without blocking).
		hasFail := false
		for _, c := range checks {
			if c.Status == "fail" {
				hasFail = true
				break
			}
		}
		if hasFail {
			// Print the full checks report on stdout (with ok=false), then exit 1.
			_ = prettyPrint(fs, map[string]any{
				"ok":       false,
				"checks":   checks,
				"warnings": warnings,
			})
			return cli.E(cli.ExitError, "DOCTOR", "doctor found problems", "run the failing checks and re-run: centmem doctor")
		}
		return prettyPrint(fs, map[string]any{
			"ok":       true,
			"checks":   checks,
			"warnings": warnings,
		})
	})
}

// checkPerm returns ok if path exists with at least the given permission bits
// (not exceeding the mode), plus a human description.
func checkPerm(path string, want os.FileMode) (bool, string) {
	info, err := os.Stat(path)
	if err != nil {
		return false, path + " not accessible: " + err.Error()
	}
	mode := info.Mode().Perm()
	if mode != want {
		return false, fmt.Sprintf("%s mode=%o want=%o", path, mode, want)
	}
	return true, fmt.Sprintf("%s mode=%o", path, mode)
}

// sha256File returns the hex sha256 of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func cmdBackup(args []string) int {
	fs := newFlagSet("backup")
	fs.String("to", "", "destination path")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		to := fs.Lookup("to").Value.String()
		if to == "" {
			return cli.Invalidf("backup: --to is required")
		}
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("backup: %v", err)
		}
		defer s.Close()

		if _, err := s.DB().Exec("VACUUM INTO ?", to); err != nil {
			return cli.Internalf("backup: %v", err)
		}
		var sizeMB float64
		if info, err := os.Stat(to); err == nil {
			sizeMB = round2(float64(info.Size()) / (1024 * 1024))
		}
		return prettyPrint(fs, map[string]any{
			"ok":      true,
			"backup":  to,
			"size_mb": sizeMB,
		})
	})
}

func cmdRestore(args []string) int {
	fs := newFlagSet("restore")
	fs.String("from", "", "backup file to restore from")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		from := fs.Lookup("from").Value.String()
		if from == "" {
			return cli.Invalidf("restore: --from is required")
		}
		info, err := os.Stat(from)
		if err != nil {
			return cli.Invalidf("restore: cannot access %q: %v", from, err)
		}
		if info.Size() == 0 {
			return cli.Invalidf("restore: backup file %q is empty", from)
		}

		// Verify the backup opens and passes integrity check before replacing.
		tmp := cfg.DBPath + ".verify"
		_ = os.Remove(tmp)
		if err := copyFile(from, tmp); err != nil {
			return cli.Internalf("restore: stage backup: %v", err)
		}
		defer os.Remove(tmp)
		if err := checkDBIntegrity(tmp); err != nil {
			return cli.Invalidf("restore: backup failed integrity check: %v", err)
		}

		// Safety copy of the current DB before replacing.
		bak := cfg.DBPath + ".pre-restore.bak"
		if _, err := os.Stat(cfg.DBPath); err == nil {
			if err := copyFile(cfg.DBPath, bak); err != nil {
				return cli.Internalf("restore: create safety copy: %v", err)
			}
		}

		if err := copyFile(from, cfg.DBPath); err != nil {
			return cli.Internalf("restore: replace db: %v", err)
		}

		return prettyPrint(fs, map[string]any{
			"ok":              true,
			"restored_from":   from,
			"pre_restore_bak": bak,
		})
	})
}

// retentionDays returns the number of days after which a memory of the given
// type becomes eligible for summarization, from the retention config. A value
// of 0 means "keep forever" (summarize_at stays NULL).
func retentionDays(cfg config.Config, typ string) int {
	switch typ {
	case "note":
		return cfg.Retention.NoteSummarizeAfterDays
	case "log":
		return cfg.Retention.LogSummarizeAfterDays
	}
	return 0
}

// copyFile copies src to dst, preserving content. Used by backup/restore.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// checkDBIntegrity opens the file at path with the SQLite driver and runs
// PRAGMA integrity_check, returning an error unless it reports "ok".
func checkDBIntegrity(path string) error {
	db, err := sql.Open("sqlite3", "file:"+path+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check failed: %s", integrity)
	}
	return nil
}

// ---------------------------------------------------------------------------
// reindex
// ---------------------------------------------------------------------------

func cmdReindex(args []string) int {
	fs := newFlagSet("reindex")
	fs.Bool("all", false, "re-enqueue all active memories even if already embedded")
	fs.Int("batch", 32, "batch size per ONNX inference run")
	fs.String("max-time", "30s", "maximum execution time before yielding (0 for unlimited)")
	fs.Bool("dry-run", false, "report count of pending embeddings without processing")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		start := time.Now()
		ctx, cancel := signalContext()
		defer cancel()

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("reindex: open store: %v", err)
		}
		defer s.Close()

		all := fs.Lookup("all").Value.String() == "true"
		batch := intFlag(fs, "batch", 32)
		if batch <= 0 {
			return cli.Invalidf("reindex: --batch must be > 0")
		}
		dryRun := fs.Lookup("dry-run").Value.String() == "true"
		maxTimeStr := fs.Lookup("max-time").Value.String()
		maxTime := 30 * time.Second
		if maxTimeStr != "" {
			d, err := parseDuration(maxTimeStr)
			if err != nil {
				return cli.Invalidf("reindex: bad --max-time: %v", err)
			}
			maxTime = d
		}

		modelName := cfg.Model.Name
		if modelName == "" {
			modelName = "bge-small-en-v1.5"
		}

		if dryRun {
			var pending int64
			if all {
				err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE status = 'active'`).Scan(&pending)
			} else {
				pending, err = s.PendingEmbeddings(ctx)
			}
			if err != nil {
				return cli.Internalf("reindex: count pending: %v", err)
			}

			return prettyPrint(fs, map[string]any{
				"ok":          true,
				"reindexed":   0,
				"pending":     pending,
				"duration_ms": time.Since(start).Milliseconds(),
				"model":       modelName,
			})
		}

		if all {
			if _, err := s.EnqueueAllActive(ctx); err != nil {
				return cli.Internalf("reindex: enqueue all: %v", err)
			}
		}

		emb, err := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
		if err != nil {
			return cli.Internalf("reindex: init embedder: %v", err)
		}
		defer emb.Close()

		q := embed.NewQueue(s, emb)
		q.BatchSize = batch
		q.Model = modelName
		if maxTime <= 0 {
			q.MaxTime = -1 // unlimited
		} else {
			q.MaxTime = maxTime
		}

		reindexed, err := q.Drain(ctx)
		if err != nil {
			return cli.Internalf("reindex: drain queue: %v", err)
		}

		pending, err := s.PendingEmbeddings(ctx)
		if err != nil {
			return cli.Internalf("reindex: count pending: %v", err)
		}

		return prettyPrint(fs, map[string]any{
			"ok":          true,
			"reindexed":   reindexed,
			"pending":     pending,
			"duration_ms": time.Since(start).Milliseconds(),
			"model":       modelName,
		})
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func drainQueue(cfg config.Config, s *store.Store) {
	emb, err := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
	if err != nil {
		return
	}
	defer emb.Close()
	q := embed.NewQueue(s, emb)
	q.MaxTime = 200 * time.Millisecond
	// Best-effort background drain; errors are non-fatal (next run retries).
	_, _ = q.Drain(context.Background())
}

func intFlag(fs *flag.FlagSet, name string, def int) int {
	v := fs.Lookup(name)
	if v == nil {
		return def
	}
	if i, err := strconv.Atoi(v.Value.String()); err == nil {
		return i
	}
	return def
}

func int64Flag(fs *flag.FlagSet, name string, def int64) int64 {
	v := fs.Lookup(name)
	if v == nil {
		return def
	}
	if i, err := strconv.ParseInt(v.Value.String(), 10, 64); err == nil {
		return i
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseDuration accepts Go-style durations (24h, 30m) plus 'd' (day) and 'w'
// (week) suffixes used in the CLI contract (e.g. --since 7d).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration")
	}
	last := s[len(s)-1]
	if last == 'd' || last == 'w' {
		num := s[:len(s)-1]
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		mult := time.Hour * 24
		if last == 'w' {
			mult *= 7
		}
		return time.Duration(n) * mult, nil
	}
	return time.ParseDuration(s)
}

// normalizeValue validates s as JSON; if it fails, wraps it as a JSON string.
func normalizeValue(s string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		// Round-trip to canonical compact JSON.
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Not JSON: store as a JSON string.
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func round4(f float64) float64 {
	return float64(int64(f*10000+0.5)) / 10000
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

func parseScopeOpt(s string) (*scope.Scope, error) {
	if s == "" {
		return nil, nil
	}
	sc, err := scope.Parse(s)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}
