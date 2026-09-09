package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// Tool defines an executable agent function.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// ToolRegistry manages registered tools and generates OpenAI-compatible schemas.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get finds a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns OpenAI-compatible function descriptors sorted by name.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)

	defs := make([]ToolDefinition, 0, len(names))
	for _, n := range names {
		t := r.tools[n]
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// Execute dispatches a tool call with the provided JSON arguments.
func (r *ToolRegistry) Execute(ctx context.Context, name string, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("agent: unknown tool %q", name)
	}
	return t.Execute(ctx, argsJSON)
}

// DefaultToolRegistry wires up the 6 standard memory tools.
func DefaultToolRegistry(s *store.Store, searcher *search.Searcher) *ToolRegistry {
	r := NewToolRegistry()
	r.Register(&SearchMemoriesTool{searcher: searcher})
	r.Register(&ReadMemoryTool{store: s})
	r.Register(&InspectLinksTool{store: s})
	r.Register(&ProposeLinkTool{store: s})
	r.Register(&ProposeMergeTool{store: s})
	r.Register(&DetectKnowledgeGapsTool{searcher: searcher})
	return r
}

// 1. SearchMemoriesTool
type SearchMemoriesTool struct {
	searcher *search.Searcher
}

func (t *SearchMemoriesTool) Name() string { return "search_memories" }
func (t *SearchMemoriesTool) Description() string {
	return "Search for stored memories matching a natural language query or keywords via hybrid search (semantic, keyword, facts, timeline)."
}
func (t *SearchMemoriesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query or keywords to retrieve relevant memories.",
			},
			"scope": map[string]any{
				"type":        "string",
				"description": "Optional scope filter (e.g. 'project:my-project' or 'global'). Defaults to searching the active scope.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of memories to return (1-50, default 5).",
			},
		},
		"required": []string{"query"},
	}
}
func (t *SearchMemoriesTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
		Scope string `json:"scope"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if t.searcher == nil || strings.TrimSpace(args.Query) == "" {
		return "[]", nil
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	results, err := t.searcher.Recall(ctx, search.Query{
		Text:         args.Query,
		Scope:        args.Scope,
		Top:          limit,
		IncludeLinks: true,
	})
	if err != nil {
		return "", fmt.Errorf("search error: %w", err)
	}

	type searchResultItem struct {
		ID        int64     `json:"id"`
		Type      string    `json:"type"`
		Scope     string    `json:"scope"`
		Content   string    `json:"content"`
		Key       string    `json:"key,omitempty"`
		Tags      []string  `json:"tags,omitempty"`
		Score     float64   `json:"score"`
		MatchedBy []string  `json:"matched_by,omitempty"`
		Links     []any     `json:"links,omitempty"`
	}

	items := make([]searchResultItem, 0, len(results))
	for _, r := range results {
		var links []any
		for _, l := range r.Links {
			links = append(links, map[string]any{
				"relation":  l.Relation,
				"linked_id": l.LinkedID,
				"direction": l.Direction,
			})
		}
		items = append(items, searchResultItem{
			ID:        r.ID,
			Type:      r.Type,
			Scope:     r.Scope,
			Content:   r.Content,
			Key:       r.Key,
			Tags:      r.Tags,
			Score:     r.Score,
			MatchedBy: r.MatchedBy,
			Links:     links,
		})
	}

	outBytes, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(outBytes), nil
}

// 2. ReadMemoryTool
type ReadMemoryTool struct {
	store *store.Store
}

func (t *ReadMemoryTool) Name() string { return "read_memory" }
func (t *ReadMemoryTool) Description() string {
	return "Retrieve complete details, content, tags, metadata, and connected relationship links for a specific memory ID."
}
func (t *ReadMemoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "integer",
				"description": "Unique identifier of the memory to inspect.",
			},
		},
		"required": []string{"id"},
	}
}
func (t *ReadMemoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.ID <= 0 {
		return "", fmt.Errorf("invalid memory id %d", args.ID)
	}
	if t.store == nil {
		return "", fmt.Errorf("store not available")
	}

	m, err := t.store.GetMemory(ctx, args.ID)
	if err != nil {
		return "", fmt.Errorf("get memory %d: %w", args.ID, err)
	}

	outgoing, incoming, _ := t.store.GetLinksForMemory(ctx, args.ID, false)

	result := map[string]any{
		"id":             m.ID,
		"type":           m.Type,
		"scope":          m.ScopePath,
		"content":        m.Content,
		"key":            m.Key,
		"value_json":     m.ValueJSON,
		"tags":           m.Tags,
		"source_agent":   m.SourceAgent,
		"source_session": m.SourceSession,
		"status":         m.Status,
		"access_count":   m.AccessCount,
		"created_at":     m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"outgoing_links": outgoing,
		"incoming_links": incoming,
	}

	outBytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(outBytes), nil
}

// 3. InspectLinksTool
type InspectLinksTool struct {
	store *store.Store
}

func (t *InspectLinksTool) Name() string { return "inspect_links" }
func (t *InspectLinksTool) Description() string {
	return "Inspect directional relationship links (supports, refines, contradicts, depends-on, supersedes) for a memory node."
}
func (t *InspectLinksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "integer",
				"description": "Memory ID to inspect links for.",
			},
		},
		"required": []string{"id"},
	}
}
func (t *InspectLinksTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.ID <= 0 {
		return "", fmt.Errorf("invalid memory id %d", args.ID)
	}
	if t.store == nil {
		return "", fmt.Errorf("store not available")
	}

	outgoing, incoming, err := t.store.GetLinksForMemory(ctx, args.ID, true)
	if err != nil {
		return "", fmt.Errorf("inspect links for memory %d: %w", args.ID, err)
	}

	result := map[string]any{
		"memory_id": args.ID,
		"outgoing":  outgoing,
		"incoming":  incoming,
	}
	outBytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(outBytes), nil
}

// 4. ProposeLinkTool
type ProposeLinkTool struct {
	store *store.Store
}

func (t *ProposeLinkTool) Name() string { return "propose_link" }
func (t *ProposeLinkTool) Description() string {
	return "Stage a human-in-the-loop proposal to create a typed relationship link between two memories (e.g. supersedes or contradicts)."
}
func (t *ProposeLinkTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"from_id": map[string]any{
				"type":        "integer",
				"description": "Source memory ID.",
			},
			"to_id": map[string]any{
				"type":        "integer",
				"description": "Target memory ID.",
			},
			"relation": map[string]any{
				"type":        "string",
				"enum":        []string{"supports", "refines", "contradicts", "depends-on", "supersedes"},
				"description": "Relationship type between the two memories.",
			},
			"reasoning": map[string]any{
				"type":        "string",
				"description": "Explanation of why this link should be established.",
			},
		},
		"required": []string{"from_id", "to_id", "relation", "reasoning"},
	}
}
func (t *ProposeLinkTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		FromID    int64  `json:"from_id"`
		ToID      int64  `json:"to_id"`
		Relation  string `json:"relation"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.FromID <= 0 || args.ToID <= 0 || args.FromID == args.ToID {
		return "", fmt.Errorf("invalid memory ids (from=%d, to=%d)", args.FromID, args.ToID)
	}
	if !store.IsValidLinkRelation(args.Relation) {
		return "", fmt.Errorf("invalid relation %q", args.Relation)
	}
	if t.store == nil {
		return "", fmt.Errorf("store not available")
	}

	fromMem, err := t.store.GetMemory(ctx, args.FromID)
	if err != nil {
		return "", fmt.Errorf("source memory %d not found: %w", args.FromID, err)
	}
	if _, err := t.store.GetMemory(ctx, args.ToID); err != nil {
		return "", fmt.Errorf("target memory %d not found: %w", args.ToID, err)
	}

	payload, _ := json.Marshal(store.LinkProposalPayload{
		FromID:   args.FromID,
		ToID:     args.ToID,
		Relation: args.Relation,
	})

	propID, err := t.store.CreateProposal(ctx, &store.Proposal{
		ScopeID:      fromMem.ScopeID,
		ScopePath:    fromMem.ScopePath,
		ProposalType: "link",
		Title:        fmt.Sprintf("Link memory #%d ──%s──► memory #%d", args.FromID, args.Relation, args.ToID),
		Reasoning:    args.Reasoning,
		PayloadJSON:  string(payload),
	})
	if err != nil {
		return "", fmt.Errorf("create link proposal: %w", err)
	}

	return fmt.Sprintf(`{"ok":true,"proposal_id":%d,"proposal_type":"link","status":"pending"}`, propID), nil
}

// 5. ProposeMergeTool
type ProposeMergeTool struct {
	store *store.Store
}

func (t *ProposeMergeTool) Name() string { return "propose_merge" }
func (t *ProposeMergeTool) Description() string {
	return "Stage a human-in-the-loop proposal to consolidate multiple overlapping or redundant memories into a single synthesis."
}
func (t *ProposeMergeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_ids": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "integer",
				},
				"description": "IDs of source memories to consolidate.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Descriptive title for the proposed merge.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Consolidated content synthesizing knowledge from all source memories.",
			},
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "Tags to apply to the consolidated memory.",
			},
			"reasoning": map[string]any{
				"type":        "string",
				"description": "Explanation of why these memories should be merged.",
			},
		},
		"required": []string{"source_ids", "title", "content", "reasoning"},
	}
}
func (t *ProposeMergeTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		SourceIDs []int64  `json:"source_ids"`
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
		Reasoning string   `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	seen := make(map[int64]bool)
	var uniqueSourceIDs []int64
	for _, sid := range args.SourceIDs {
		if sid <= 0 {
			return "", fmt.Errorf("invalid source id %d", sid)
		}
		if !seen[sid] {
			seen[sid] = true
			uniqueSourceIDs = append(uniqueSourceIDs, sid)
		}
	}
	if len(uniqueSourceIDs) < 2 {
		return "", fmt.Errorf("propose_merge requires at least two distinct source memories to consolidate")
	}
	args.SourceIDs = uniqueSourceIDs

	if strings.TrimSpace(args.Content) == "" {
		return "", fmt.Errorf("content cannot be empty")
	}
	if t.store == nil {
		return "", fmt.Errorf("store not available")
	}

	firstMem, err := t.store.GetMemory(ctx, args.SourceIDs[0])
	if err != nil {
		return "", fmt.Errorf("source memory %d: %w", args.SourceIDs[0], err)
	}

	for _, sid := range args.SourceIDs[1:] {
		if _, err := t.store.GetMemory(ctx, sid); err != nil {
			return "", fmt.Errorf("source memory %d: %w", sid, err)
		}
	}

	payload, _ := json.Marshal(store.MergeProposalPayload{
		SourceIDs:     args.SourceIDs,
		TargetTitle:   args.Title,
		TargetContent: args.Content,
		TargetTags:    args.Tags,
	})

	propID, err := t.store.CreateProposal(ctx, &store.Proposal{
		ScopeID:      firstMem.ScopeID,
		ScopePath:    firstMem.ScopePath,
		ProposalType: "merge",
		Title:        args.Title,
		Reasoning:    args.Reasoning,
		PayloadJSON:  string(payload),
	})
	if err != nil {
		return "", fmt.Errorf("create merge proposal: %w", err)
	}

	return fmt.Sprintf(`{"ok":true,"proposal_id":%d,"proposal_type":"merge","status":"pending"}`, propID), nil
}

// 6. DetectKnowledgeGapsTool
type DetectKnowledgeGapsTool struct {
	searcher *search.Searcher
}

func (t *DetectKnowledgeGapsTool) Name() string { return "detect_knowledge_gaps" }
func (t *DetectKnowledgeGapsTool) Description() string {
	return "Analyze memory coverage for a specific topic, identifying missing conventions, architectural gaps, or unrecorded decisions."
}
func (t *DetectKnowledgeGapsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topic": map[string]any{
				"type":        "string",
				"description": "Topic, feature, or component to inspect for knowledge completeness.",
			},
			"scope": map[string]any{
				"type":        "string",
				"description": "Optional scope filter.",
			},
		},
		"required": []string{"topic"},
	}
}
func (t *DetectKnowledgeGapsTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Topic string `json:"topic"`
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Topic) == "" {
		return "", fmt.Errorf("topic cannot be empty")
	}

	var results []search.Ranked
	if t.searcher != nil {
		var err error
		results, err = t.searcher.Recall(ctx, search.Query{
			Text:  args.Topic,
			Scope: args.Scope,
			Top:   10,
		})
		if err != nil {
			return "", fmt.Errorf("search error: %w", err)
		}
	}

	coverage := "adequate"
	var gaps []string

	// Count topical matches (excluding results that only matched timeline recency)
	var topicalMatches int
	for _, r := range results {
		isTopical := false
		for _, m := range r.MatchedBy {
			if m == "keyword" || m == "keyword_prefix" || m == "semantic" || m == "facts" {
				isTopical = true
				break
			}
		}
		if isTopical {
			topicalMatches++
		}
	}

	if topicalMatches == 0 {
		coverage = "missing"
		gaps = append(gaps, fmt.Sprintf("No stored memories found for topic %q in scope %q", args.Topic, args.Scope))
	} else if topicalMatches < 3 {
		coverage = "sparse"
		gaps = append(gaps, fmt.Sprintf("Sparse coverage (%d memories) found for %q; key architectural decisions or runbooks may be unrecorded", topicalMatches, args.Topic))
	}

	report := map[string]any{
		"topic":       args.Topic,
		"scope":       args.Scope,
		"found_count": topicalMatches,
		"coverage":    coverage,
		"gaps":        gaps,
	}

	outBytes, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	return string(outBytes), nil
}
