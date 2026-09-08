package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// AllTools returns the 7 core tools exposed by centmem over MCP.
func AllTools() []Tool {
	return []Tool{
		{
			Name:        "centmem_recall",
			Description: "Search memory using hybrid vector + keyword + facts + timeline retrieval.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"query":             {Type: "string", Description: "Search query or task context to recall relevant memories"},
					"scope":             {Type: "string", Description: "Memory scope filter (e.g. 'project:my-app' or 'global')"},
					"top":               {Type: "integer", Description: "Maximum number of memories to return (default 5, max 20)"},
					"type":              {Type: "string", Description: "Filter by memory type ('note', 'fact', 'log')"},
					"tags":              {Type: "array", Items: map[string]any{"type": "string"}, Description: "Filter memories matching these tags"},
					"since":             {Type: "string", Description: "Filter memories created after duration (e.g. '7d', '24h')"},
					"until":             {Type: "string", Description: "Filter memories created before duration (e.g. '1d')"},
					"agent":             {Type: "string", Description: "Filter by author agent identifier"},
					"caller_agent":      {Type: "string", Description: "Identifier of the calling agent for personalization boost"},
					"reranker":          {Type: "string", Description: "Reranker algorithm override ('composite', 'none', 'cross_encoder', 'llm')"},
					"inherit":           {Type: "boolean", Description: "Include memories inherited from parent scopes (default true)"},
					"children":          {Type: "boolean", Description: "Include memories from child scopes (default false)"},
					"include_links":     {Type: "boolean", Description: "Include 1-hop connected memory graph relationships"},
					"include_suggested": {Type: "boolean", Description: "Include pending suggested links in relationship graph"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "centmem_put",
			Description: "Store a freeform note or chronological log memory.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"content":        {Type: "string", Description: "Memory content text"},
					"scope":          {Type: "string", Description: "Scope path (e.g. 'project:my-app' or 'global')"},
					"type":           {Type: "string", Enum: []string{"note", "log"}, Description: "Memory type (default 'note')"},
					"tags":           {Type: "array", Items: map[string]any{"type": "string"}, Description: "Tags to categorize the memory"},
					"source_agent":   {Type: "string", Description: "Author agent identifier"},
					"source_session": {Type: "string", Description: "Session identifier"},
					"no_suggest":     {Type: "boolean", Description: "Bypass automatic relationship link suggestions"},
				},
				Required: []string{"content", "scope"},
			},
		},
		{
			Name:        "centmem_set",
			Description: "Upsert a key/value fact memory with hierarchical scoping.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"scope": {Type: "string", Description: "Scope path for the fact"},
					"key":   {Type: "string", Description: "Fact key (dot-separated path, e.g. 'api.port')"},
					"value": {Description: "Fact value (JSON object, array, or string primitive)"},
					"tags":  {Type: "array", Items: map[string]any{"type": "string"}, Description: "Tags to associate with the fact"},
				},
				Required: []string{"scope", "key", "value"},
			},
		},
		{
			Name:        "centmem_get",
			Description: "Retrieve a memory by ID or fact key.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"id":      {Type: "integer", Description: "Memory ID to fetch directly"},
					"scope":   {Type: "string", Description: "Scope path when fetching fact by key"},
					"key":     {Type: "string", Description: "Fact key to fetch"},
					"inherit": {Type: "boolean", Description: "Walk ancestor scopes for fact lookup (default true)"},
				},
			},
		},
		{
			Name:        "centmem_timeline",
			Description: "Retrieve a chronological timeline of logs and memories.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"scope": {Type: "string", Description: "Scope path filter"},
					"since": {Type: "string", Description: "Since duration offset (default '24h', e.g. '7d', '2h')"},
					"until": {Type: "string", Description: "Until duration offset"},
					"limit": {Type: "integer", Description: "Maximum entries to return (default 50)"},
				},
			},
		},
		{
			Name:        "centmem_stats",
			Description: "Retrieve memory store statistics, counts, and database status.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"scope": {Type: "string", Description: "Optional scope filter"},
				},
			},
		},
		{
			Name:        "centmem_forget",
			Description: "Delete or archive a memory by ID, fact key, or tag.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"id":    {Type: "integer", Description: "Memory ID to delete"},
					"scope": {Type: "string", Description: "Scope path when deleting by key or tag"},
					"key":   {Type: "string", Description: "Fact key to delete"},
					"tag":   {Type: "string", Description: "Tag to delete memories by"},
				},
			},
		},
	}
}

// ExecuteTool executes a named centmem tool and returns CallToolResult.
func ExecuteTool(ctx context.Context, st *store.Store, searcher *search.Searcher, cfg config.Config, name string, args map[string]any) CallToolResult {
	if args == nil {
		args = make(map[string]any)
	}

	var res any
	var err error

	switch name {
	case "centmem_recall":
		res, err = executeRecall(ctx, st, searcher, cfg, args)
	case "centmem_put":
		res, err = executePut(ctx, st, searcher, cfg, args)
	case "centmem_set":
		res, err = executeSet(ctx, st, cfg, args)
	case "centmem_get":
		res, err = executeGet(ctx, st, args)
	case "centmem_timeline":
		res, err = executeTimeline(ctx, searcher, args)
	case "centmem_stats":
		res, err = executeStats(ctx, st, args)
	case "centmem_forget":
		res, err = executeForget(ctx, st, args)
	default:
		return CallToolResult{
			Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", name)}},
			IsError: true,
		}
	}

	if err != nil {
		errBytes, _ := json.Marshal(map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return CallToolResult{
			Content: []ToolContent{{Type: "text", Text: string(errBytes)}},
			IsError: true,
		}
	}

	outBytes, err := json.Marshal(res)
	if err != nil {
		return CallToolResult{
			Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("marshal tool response: %v", err)}},
			IsError: true,
		}
	}

	return CallToolResult{
		Content: []ToolContent{{Type: "text", Text: string(outBytes)}},
		IsError: false,
	}
}

func executeRecall(ctx context.Context, st *store.Store, searcher *search.Searcher, cfg config.Config, args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("centmem_recall: query is required")
	}

	scope, _ := args["scope"].(string)
	top := getIntArg(args, "top", 5)
	if top > 20 {
		top = 20
	}
	typ, _ := args["type"].(string)
	tags := getStringSliceArg(args, "tags")
	agent, _ := args["agent"].(string)
	callerAgent, _ := args["caller_agent"].(string)
	if callerAgent == "" {
		callerAgent = os.Getenv("CENTMEM_AGENT")
	}
	inherit := getBoolArg(args, "inherit", true)
	children := getBoolArg(args, "children", false)
	includeSuggested := getBoolArg(args, "include_suggested", false)
	includeLinks := getBoolArg(args, "include_links", false) || includeSuggested

	q := search.Query{
		Text:                  query,
		Scope:                 scope,
		Inherit:               inherit,
		Children:              children,
		Top:                   top,
		Type:                  typ,
		Tags:                  tags,
		Agent:                 agent,
		CallerAgent:           callerAgent,
		IncludeLinks:          includeLinks,
		IncludeSuggestedLinks: includeSuggested,
	}

	now := time.Now()
	if sinceStr, ok := args["since"].(string); ok && sinceStr != "" {
		d, err := parseDuration(sinceStr)
		if err != nil {
			return nil, fmt.Errorf("centmem_recall: bad since duration %q: %w", sinceStr, err)
		}
		q.Since = now.Add(-d)
	}
	if untilStr, ok := args["until"].(string); ok && untilStr != "" {
		d, err := parseDuration(untilStr)
		if err != nil {
			return nil, fmt.Errorf("centmem_recall: bad until duration %q: %w", untilStr, err)
		}
		q.Until = now.Add(-d)
	}

	if searcher == nil && st != nil {
		searcher = search.New(st)
	}
	if searcher == nil {
		return nil, errors.New("centmem_recall: searcher not initialized")
	}

	if reranker, ok := args["reranker"].(string); ok && reranker != "" {
		switch strings.ToLower(reranker) {
		case "composite", "none", "cross_encoder", "llm":
			searcher = searcher.WithRerankerName(strings.ToLower(reranker))
		}
	}

	results, err := searcher.Recall(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(results) > 0 && st != nil {
		ids := make([]int64, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		st.RecordAccessAsync(ids)
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

	return map[string]any{
		"ok":      true,
		"query":   query,
		"results": out,
	}, nil
}

func executePut(ctx context.Context, st *store.Store, searcher *search.Searcher, cfg config.Config, args map[string]any) (any, error) {
	content, _ := args["content"].(string)
	scope, _ := args["scope"].(string)
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("centmem_put: content is required")
	}
	if strings.TrimSpace(scope) == "" {
		return nil, errors.New("centmem_put: scope is required")
	}
	typ, _ := args["type"].(string)
	if typ == "" {
		typ = "note"
	}
	if typ != "note" && typ != "log" {
		return nil, errors.New("centmem_put: type must be 'note' or 'log'")
	}

	tags := getStringSliceArg(args, "tags")
	sourceAgent, _ := args["source_agent"].(string)
	sourceSession, _ := args["source_session"].(string)
	noSuggest := getBoolArg(args, "no_suggest", false)

	var summarizeAt *int64
	retDays := 0
	if typ == "log" {
		retDays = cfg.Retention.LogSummarizeAfterDays
	} else if typ == "note" {
		retDays = cfg.Retention.NoteSummarizeAfterDays
	}
	if retDays > 0 {
		ts := time.Now().Add(time.Duration(retDays) * 24 * time.Hour).UnixMicro()
		summarizeAt = &ts
	}

	id, status, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:         scope,
		Type:          typ,
		Content:       content,
		Tags:          tags,
		SourceAgent:   sourceAgent,
		SourceSession: sourceSession,
		SummarizeAt:   summarizeAt,
	})
	if err != nil {
		return nil, err
	}

	drainMCPQueue(cfg, st)

	out := map[string]any{
		"ok":     true,
		"id":     id,
		"scope":  scope,
		"status": status,
	}

	if !noSuggest && searcher != nil {
		memRow, err := st.GetMemory(ctx, id)
		if err == nil && memRow != nil {
			suggestions, err := searcher.SuggestLinks(ctx, *memRow)
			if err == nil && len(suggestions) > 0 {
				out["suggested_links"] = suggestions
			}
		}
	}

	return out, nil
}

func executeSet(ctx context.Context, st *store.Store, cfg config.Config, args map[string]any) (any, error) {
	scope, _ := args["scope"].(string)
	key, _ := args["key"].(string)
	if strings.TrimSpace(scope) == "" || strings.TrimSpace(key) == "" {
		return nil, errors.New("centmem_set: scope and key are required")
	}

	rawVal, ok := args["value"]
	if !ok || rawVal == nil {
		return nil, errors.New("centmem_set: value is required")
	}

	var norm string
	switch v := rawVal.(type) {
	case string:
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			b, _ := json.Marshal(parsed)
			norm = string(b)
		} else {
			b, _ := json.Marshal(v)
			norm = string(b)
		}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("centmem_set: marshal value: %w", err)
		}
		norm = string(b)
	}

	tags := getStringSliceArg(args, "tags")

	id, status, err := st.SetFact(ctx, store.FactInput{
		Scope: scope,
		Key:   key,
		Value: norm,
		Tags:  tags,
	})
	if err != nil {
		return nil, err
	}

	drainMCPQueue(cfg, st)

	return map[string]any{
		"ok":     true,
		"id":     id,
		"key":    key,
		"status": status,
	}, nil
}

func executeGet(ctx context.Context, st *store.Store, args map[string]any) (any, error) {
	id := getIntArg(args, "id", 0)
	if id > 0 {
		mem, err := st.GetMemory(ctx, int64(id))
		if errors.Is(err, sql.ErrNoRows) || mem == nil {
			return nil, fmt.Errorf("memory with id %d not found", id)
		}
		if err != nil {
			return nil, err
		}
		var lastAccessed any
		if mem.AccessCount > 0 && mem.LastAccessedAt != nil {
			lastAccessed = mem.LastAccessedAt.Unix()
		}
		memMap := map[string]any{
			"id":               mem.ID,
			"type":             mem.Type,
			"scope":            mem.ScopePath,
			"content":          mem.Content,
			"tags":             mem.Tags,
			"source_agent":     mem.SourceAgent,
			"source_session":   mem.SourceSession,
			"status":           mem.Status,
			"access_count":     mem.AccessCount,
			"last_accessed_at": lastAccessed,
			"created_at":       mem.CreatedAt.Unix(),
			"updated_at":       mem.UpdatedAt.Unix(),
		}
		if mem.Type == "fact" {
			memMap["key"] = mem.Key
			var val any
			if err := json.Unmarshal([]byte(mem.ValueJSON), &val); err == nil {
				memMap["value"] = val
			} else {
				memMap["value"] = mem.ValueJSON
			}
		}
		return map[string]any{
			"ok":     true,
			"memory": memMap,
		}, nil
	}

	key, _ := args["key"].(string)
	scope, _ := args["scope"].(string)
	if key == "" || scope == "" {
		return nil, errors.New("centmem_get: either id or scope+key must be provided")
	}

	inherit := getBoolArg(args, "inherit", true)
	f, err := st.GetFact(ctx, scope, key, inherit)
	if errors.Is(err, sql.ErrNoRows) || f == nil {
		return nil, fmt.Errorf("fact %q not found in scope %q", key, scope)
	}
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal([]byte(f.Value), &value); err != nil {
		value = f.Value
	}

	return map[string]any{
		"ok":    true,
		"key":   f.Key,
		"value": value,
		"scope": f.ScopePath,
		"id":    f.ID,
	}, nil
}

func executeTimeline(ctx context.Context, searcher *search.Searcher, args map[string]any) (any, error) {
	if searcher == nil {
		return nil, errors.New("centmem_timeline: searcher not initialized")
	}

	scope, _ := args["scope"].(string)
	limit := getIntArg(args, "limit", 50)
	if limit > 200 {
		limit = 200
	}

	now := time.Now()
	sinceDur := 24 * time.Hour
	if sinceStr, ok := args["since"].(string); ok && sinceStr != "" {
		d, err := parseDuration(sinceStr)
		if err != nil {
			return nil, fmt.Errorf("centmem_timeline: bad since duration %q: %w", sinceStr, err)
		}
		sinceDur = d
	}

	q := search.Query{
		Scope:   scope,
		Inherit: true,
		Top:     limit,
		Since:   now.Add(-sinceDur),
	}
	if untilStr, ok := args["until"].(string); ok && untilStr != "" {
		d, err := parseDuration(untilStr)
		if err != nil {
			return nil, fmt.Errorf("centmem_timeline: bad until duration %q: %w", untilStr, err)
		}
		q.Until = now.Add(-d)
	}

	results, err := searcher.Timeline(ctx, q, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]any, 0, len(results))
	for _, r := range results {
		entries = append(entries, map[string]any{
			"id":         r.ID,
			"type":       r.Type,
			"scope":      r.Scope,
			"content":    r.Content,
			"tags":       r.Tags,
			"created_at": r.CreatedAt.Unix(),
		})
	}

	return map[string]any{
		"ok":       true,
		"entries":  entries,
		"timeline": entries,
	}, nil
}

func executeStats(ctx context.Context, st *store.Store, args map[string]any) (any, error) {
	stats, err := st.Stats(ctx)
	if err != nil {
		return nil, err
	}
	var lastCompact any
	if stats.LastCompactAt != nil {
		lastCompact = *stats.LastCompactAt
	}
	return map[string]any{
		"ok":                      true,
		"stats":                   stats,
		"db_path":                 stats.DBPath,
		"db_size_mb":              round2(stats.DBSizeMB),
		"memories":                stats.Memories,
		"by_type":                 stats.ByType,
		"by_scope":                stats.ByScope,
		"last_compact_at":         lastCompact,
		"pending_embeddings":      stats.PendingEmbedding,
		"importance_distribution": stats.ImportanceDistribution,
	}, nil
}

func executeForget(ctx context.Context, st *store.Store, args map[string]any) (any, error) {
	id := getIntArg(args, "id", 0)
	var ids []int64
	if id > 0 {
		ids = []int64{int64(id)}
	}

	var scopeP, keyP, tagP *string
	if scope, ok := args["scope"].(string); ok && scope != "" {
		scopeP = &scope
	}
	if key, ok := args["key"].(string); ok && key != "" {
		keyP = &key
	}
	if tag, ok := args["tag"].(string); ok && tag != "" {
		tagP = &tag
	}

	deleted, err := st.Forget(ctx, ids, scopeP, keyP, tagP)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":      true,
		"deleted": deleted,
	}, nil
}

// Helpers
func getIntArg(args map[string]any, key string, defaultVal int) int {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	return defaultVal
}

func getBoolArg(args map[string]any, key string, defaultVal bool) bool {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(v, "true") || v == "1"
		}
	}
	return defaultVal
}

func getStringSliceArg(args map[string]any, key string) []string {
	val, ok := args[key]
	if !ok || val == nil {
		return nil
	}
	switch v := val.(type) {
	case []any:
		var res []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				res = append(res, strings.TrimSpace(s))
			}
		}
		return res
	case []string:
		return v
	case string:
		parts := strings.Split(v, ",")
		var res []string
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				res = append(res, s)
			}
		}
		return res
	default:
		return nil
	}
}

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

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func drainMCPQueue(cfg config.Config, s *store.Store) {
	if s == nil || cfg.Model.Path == "" {
		return
	}
	emb, err := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
	if err != nil {
		return
	}
	defer emb.Close()
	q := embed.NewQueue(s, emb)
	q.MaxTime = 200 * time.Millisecond
	_, _ = q.Drain(context.Background())
}

