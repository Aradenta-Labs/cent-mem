package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/farras/cent-mem/internal/cli"
	"github.com/farras/cent-mem/internal/config"
	"github.com/farras/cent-mem/internal/embed"
	"github.com/farras/cent-mem/internal/scope"
	"github.com/farras/cent-mem/internal/search"
	"github.com/farras/cent-mem/internal/store"
)

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func cmdInit(args []string) int {
	fs := newFlagSet("init")
	fs.String("model", "bge-small-en-v1.5", "embedding model name")
	fs.Bool("force", false, "re-download model even if present")
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

		modelPath := cfg.Model.Path
		if force {
			os.Remove(modelPath)
		}
		path, err := embed.Downloader(modelName, modelPath)
		if err != nil {
			return cli.Internalf("init: download model: %v", err)
		}
		_ = path

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
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("put: %v", err)
		}
		defer s.Close()

		scopePath := fs.Lookup("scope").Value.String()
		typ := fs.Lookup("type").Value.String()
		content := fs.Lookup("content").Value.String()
		if scopePath == "" {
			return cli.Invalidf("put: --scope is required")
		}
		if content == "" {
			return cli.Invalidf("put: --content is required")
		}
		if typ != "note" && typ != "log" {
			return cli.Invalidf("put: --type must be note or log")
		}

		id, status, err := s.PutMemory(context.Background(), store.MemoryInput{
			Scope:         scopePath,
			Type:          typ,
			Content:       content,
			Tags:          splitCSV(fs.Lookup("tags").Value.String()),
			SourceAgent:   fs.Lookup("source-agent").Value.String(),
			SourceSession: fs.Lookup("source-session").Value.String(),
		})
		if err != nil {
			return cli.Internalf("put: %v", err)
		}

		drainQueue(cfg, s)

		return prettyPrint(fs, map[string]any{
			"ok":     true,
			"id":     id,
			"scope":  scopePath,
			"status": status,
		})
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
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("set: %v", err)
		}
		defer s.Close()

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
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("get: %v", err)
		}
		defer s.Close()

		scopePath := fs.Lookup("scope").Value.String()
		key := fs.Lookup("key").Value.String()
		inherit := fs.Lookup("inherit").Value.String() == "true"

		f, err := s.GetFact(context.Background(), scopePath, key, inherit)
		if errors.Is(err, sql.ErrNoRows) {
			return cli.NotFoundf("fact %q not found in scope %q", key, scopePath)
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
	fs.Bool("inherit", true, "include ancestor scopes")
	fs.Bool("children", false, "include descendant scopes")
	return runCommandQuery(args, fs, func(cfg config.Config, fs *flag.FlagSet, query string) error {
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("recall: %v", err)
		}
		defer s.Close()

		emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
		searcher := search.New(s).WithEmbedder(emb)
		text := query

		q := search.Query{
			Text:     text,
			Scope:    fs.Lookup("scope").Value.String(),
			Inherit:  fs.Lookup("inherit").Value.String() == "true",
			Children: fs.Lookup("children").Value.String() == "true",
			Top:      intFlag(fs, "top", 5),
			Type:     fs.Lookup("type").Value.String(),
			Tags:     splitCSV(fs.Lookup("tags").Value.String()),
			Agent:    fs.Lookup("agent").Value.String(),
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

		out := make([]map[string]any, 0, len(results))
		for _, r := range results {
			out = append(out, map[string]any{
				"id":        r.ID,
				"type":      r.Type,
				"scope":     r.Scope,
				"content":   r.Content,
				"tags":      r.Tags,
				"created_at": r.CreatedAt.Unix(),
				"score":     round4(r.Score),
				"matched_by": r.MatchedBy,
			})
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
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("list: %v", err)
		}
		defer s.Close()

		scopePath := fs.Lookup("scope").Value.String()
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
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("forget: %v", err)
		}
		defer s.Close()

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
			"ok":               true,
			"db_path":          st.DBPath,
			"db_size_mb":       round2(st.DBSizeMB),
			"memories":         st.Memories,
			"by_type":          st.ByType,
			"by_scope":         st.ByScope,
			"last_compact_at":  lastCompact,
			"pending_embeddings": st.PendingEmbedding,
		})
	})
}

// ---------------------------------------------------------------------------
// doctor / backup
// ---------------------------------------------------------------------------

func cmdDoctor(args []string) int {
	fs := newFlagSet("doctor")
	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("doctor: %v", err)
		}
		defer s.Close()

		var integrity string
		if err := s.DB().QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
			return cli.Internalf("doctor: %v", err)
		}
		if integrity != "ok" {
			return cli.Internalf("doctor: integrity check failed: %s", integrity)
		}
		return prettyPrint(fs, map[string]any{"ok": true, "integrity": integrity})
	})
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
		return prettyPrint(fs, map[string]any{"ok": true, "to": to})
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
