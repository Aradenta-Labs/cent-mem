package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

var citationRegex = regexp.MustCompile(`\[id:\s*(\d+)\]`)

// Engine coordinates the ReAct reasoning loop, tools, prompt catalog, and fallback logic.
type Engine struct {
	client   *Client
	store    *store.Store
	searcher *search.Searcher
	tools    *ToolRegistry
	cfg      config.AgentConfig
	llmCfg   config.LLMConfig
}

// NewEngine creates a new Reasoning Engine.
func NewEngine(cfg config.Config, s *store.Store, searcher *search.Searcher) *Engine {
	client := NewClient(cfg.LLM)
	tools := DefaultToolRegistry(s, searcher)
	return &Engine{
		client:   client,
		store:    s,
		searcher: searcher,
		tools:    tools,
		cfg:      cfg.Agent,
		llmCfg:   cfg.LLM,
	}
}

// Ask performs an interactive conversational inquiry using ReAct reasoning and grounded citations.
func (e *Engine) Ask(ctx context.Context, question string, opts InquiryOptions) (*AskResult, error) {
	if strings.TrimSpace(question) == "" {
		return nil, fmt.Errorf("agent: empty question")
	}

	scope := opts.Scope
	if scope == "" {
		scope = "global"
	}

	// 1. Ensure conversation persistence
	convID := opts.ConversationID
	if convID == "" && e.store != nil {
		title := question
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		conv := &store.Conversation{
			ScopePath: scope,
			Title:     title,
		}
		if err := e.store.CreateConversation(ctx, conv); err == nil {
			convID = conv.ID
		}
	}

	if convID != "" && e.store != nil {
		_, _ = e.store.AppendMessage(ctx, &store.Message{
			ConversationID: convID,
			Role:           "user",
			Content:        question,
		})
	}

	// 2. Check if offline fallback is required
	if e.isOffline() {
		res := e.offlineAsk(ctx, question, scope, opts.Top)
		res.ConversationID = convID
		if opts.StreamCallback != nil {
			_ = opts.StreamCallback(&StreamChunk{
				DeltaRole:    "assistant",
				DeltaContent: res.Answer,
				FinishReason: "stop",
			})
		}
		e.persistAssistantMessage(ctx, convID, res)
		return res, nil
	}

	// 3. Build ReAct conversation messages
	messages := []ChatMessage{
		{Role: "system", Content: AskSystemPrompt},
	}

	// If thread exists, load recent messages for multi-turn context
	if convID != "" && e.store != nil {
		priorMsgs, _ := e.store.GetConversationMessages(ctx, convID)
		// Load up to last 10 prior messages (excluding the one we just added)
		if len(priorMsgs) > 1 {
			startIdx := 0
			if len(priorMsgs)-1 > 10 {
				startIdx = len(priorMsgs) - 1 - 10
			}
			for _, m := range priorMsgs[startIdx : len(priorMsgs)-1] {
				messages = append(messages, ChatMessage{
					Role:    m.Role,
					Content: m.Content,
				})
			}
		}
	}

	userMsg := fmt.Sprintf("Question (Scope: %s):\n%s", scope, question)
	messages = append(messages, ChatMessage{Role: "user", Content: userMsg})

	// 4. Run ReAct reasoning loop
	answer, steps, err := e.RunReActLoop(ctx, messages, opts.StreamCallback)
	if err != nil {
		// LLM endpoint failure → graceful fallback
		// Log the real error to stderr so it's visible in centmem ui output
		fmt.Fprintf(os.Stderr, "[centmem/agent] ReAct loop failed, falling back to offline mode: %v\n", err)
		res := e.offlineAsk(ctx, question, scope, opts.Top)
		res.ConversationID = convID
		e.persistAssistantMessage(ctx, convID, res)
		return res, nil
	}

	// 5. Extract citations & knowledge gaps
	citations := e.resolveCitations(ctx, answer)
	if opts.Top > 0 && len(citations) > opts.Top {
		citations = citations[:opts.Top]
	}
	gaps := e.extractKnowledgeGaps(answer)

	res := &AskResult{
		Answer:         answer,
		Citations:      citations,
		KnowledgeGaps:  gaps,
		ReasoningSteps: steps,
		ConversationID: convID,
		FallbackUsed:   false,
	}

	e.persistAssistantMessage(ctx, convID, res)
	return res, nil
}

// Curate runs autonomous memory curation (contradictions, deduplication, or both).
func (e *Engine) Curate(ctx context.Context, opts CurateOptions) (*CurateResult, error) {
	scope := opts.Scope
	if scope == "" {
		scope = "global"
	}
	curateType := opts.Type
	if curateType == "" {
		curateType = "all"
	}

	if e.isOffline() {
		return e.offlineCurate(ctx, scope, curateType, opts.AutoApply, opts.DryRun)
	}

	var prompt string
	switch curateType {
	case "contradictions":
		prompt = CurateContradictionsPrompt
	case "dedup":
		prompt = CurateDedupPrompt
	default:
		prompt = CurateContradictionsPrompt + "\n\n" + CurateDedupPrompt
	}

	userMsg := fmt.Sprintf("Run memory curation for scope %q. Inspect memories, find conflicts and duplicates, and create proposals.", scope)
	if opts.DryRun {
		ctx = context.WithValue(ctx, dryRunContextKey, true)
		userMsg += " (DRY RUN: do not persist changes to store)"
	}

	messages := []ChatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: userMsg},
	}

	// Track proposals count before loop
	existingProps, _ := e.store.ListProposals(ctx, store.ProposalListQuery{ScopePath: scope, Limit: 10000})
	preExistingMap := make(map[int64]bool)
	for _, p := range existingProps {
		preExistingMap[p.ID] = true
	}

	_, _, err := e.RunReActLoop(ctx, messages, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[centmem/agent] Curate ReAct loop failed, falling back to offline mode: %v\n", err)
		return e.offlineCurate(ctx, scope, curateType, opts.AutoApply, opts.DryRun)
	}

	// Find newly created proposals
	afterProps, _ := e.store.ListProposals(ctx, store.ProposalListQuery{ScopePath: scope, Limit: 10000})
	createdIDs := make([]int64, 0)
	appliedIDs := make([]int64, 0)
	var contradictionsCount, duplicatesCount int

	for _, p := range afterProps {
		if !preExistingMap[p.ID] {
			createdIDs = append(createdIDs, p.ID)
			if p.ProposalType == "merge" {
				duplicatesCount++
			} else if p.ProposalType == "link" {
				var payload store.LinkProposalPayload
				if err := json.Unmarshal([]byte(p.PayloadJSON), &payload); err == nil && payload.Relation == "contradicts" {
					contradictionsCount++
				}
			}
			if opts.AutoApply && !opts.DryRun {
				if err := e.store.ApplyProposal(ctx, p.ID); err == nil {
					appliedIDs = append(appliedIDs, p.ID)
				}
			}
		}
	}

	var scannedCount int
	if count, countErr := e.store.Count(ctx, store.ListQuery{ScopePath: scope, Status: "active"}); countErr == nil {
		scannedCount = int(count)
	}

	return &CurateResult{
		ProposalsCreated:    createdIDs,
		ProposalsApplied:    appliedIDs,
		ScannedMemories:     scannedCount,
		ContradictionsFound: contradictionsCount,
		DuplicatesFound:     duplicatesCount,
		FallbackUsed:        false,
	}, nil
}

// Summarize generates a comprehensive architectural and conventions briefing for a scope.
func (e *Engine) Summarize(ctx context.Context, opts SummarizeOptions) (*SummarizeResult, error) {
	scope := opts.Scope
	if scope == "" {
		scope = "global"
	}

	var res *SummarizeResult
	var err error

	if e.isOffline() {
		res, err = e.offlineSummarize(ctx, scope, opts.Focus)
	} else {
		messages := []ChatMessage{
			{Role: "system", Content: SummarizeScopePrompt},
			{Role: "user", Content: fmt.Sprintf("Synthesize an architectural summary and developer guide for scope %q. Focus: %s", scope, opts.Focus)},
		}

		var summaryMarkdown string
		summaryMarkdown, _, err = e.RunReActLoop(ctx, messages, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[centmem/agent] Summarize ReAct loop failed, falling back to offline mode: %v\n", err)
			res, err = e.offlineSummarize(ctx, scope, opts.Focus)
		} else {
			citations := e.resolveCitations(ctx, summaryMarkdown)
			citedIDs := make([]int64, 0, len(citations))
			for _, c := range citations {
				citedIDs = append(citedIDs, c.ID)
			}

			title := fmt.Sprintf("Architectural Summary — %s", scope)
			if opts.Focus != "" {
				title = fmt.Sprintf("Summary: %s (%s)", opts.Focus, scope)
			}

			res = &SummarizeResult{
				Title:           title,
				SummaryMarkdown: summaryMarkdown,
				CitedMemoryIDs:  citedIDs,
				Scope:           scope,
				FallbackUsed:    false,
			}
		}
	}

	if err != nil {
		return nil, err
	}

	if opts.Save && e.store != nil && res != nil && res.SummaryMarkdown != "" {
		id, _, putErr := e.store.PutMemory(ctx, store.MemoryInput{
			Scope:   scope,
			Type:    "note",
			Content: res.SummaryMarkdown,
			Tags:    []string{"summary", "architecture", "digest"},
		})
		if putErr == nil && id > 0 {
			res.SavedID = &id
		}
	}

	return res, nil
}

// RunReActLoop executes the multi-step "Plan -> Act -> Think" reasoning loop with cycle guards.
func (e *Engine) RunReActLoop(ctx context.Context, initialMessages []ChatMessage, onStream func(*StreamChunk) error) (string, int, error) {
	maxSteps := e.cfg.MaxReasoningSteps
	if maxSteps <= 0 {
		maxSteps = 8
	}

	messages := make([]ChatMessage, len(initialMessages))
	copy(messages, initialMessages)

	steps := 0
	toolDefs := e.tools.Definitions()

	for step := 0; step < maxSteps; step++ {
		steps++

		isLastStep := (step == maxSteps-1)
		req := &ChatRequest{
			Model:    e.llmCfg.Model,
			Messages: messages,
			Tools:    toolDefs,
		}

		if isLastStep {
			// Force synthesis on final step: disable further tool calls
			req.Tools = nil
			req.ToolChoice = "none"
			messages = append(messages, ChatMessage{
				Role:    "user",
				Content: ForceSynthesisPrompt,
			})
			req.Messages = messages
		}

		var resp *ChatResponse
		var err error

		if onStream != nil {
			resp, err = e.client.StreamChat(ctx, req, onStream)
		} else {
			resp, err = e.client.Chat(ctx, req)
		}

		if err != nil {
			return "", steps, err
		}

		if len(resp.Choices) == 0 {
			return "", steps, fmt.Errorf("agent: model returned empty choices")
		}

		choice := resp.Choices[0]

		// If no tool calls requested or final step reached, we have our final synthesized response
		if len(choice.Message.ToolCalls) == 0 || isLastStep {
			return strings.TrimSpace(choice.Message.Content), steps, nil
		}

		// Tool calls requested: execute each and append to message sequence
		messages = append(messages, choice.Message)

		for _, tc := range choice.Message.ToolCalls {
			out, toolErr := e.tools.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if toolErr != nil {
				out = fmt.Sprintf(`{"error": %q}`, toolErr.Error())
			}
			messages = append(messages, ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    out,
			})
		}
	}

	return "", steps, fmt.Errorf("agent: exceeded maximum reasoning steps budget (%d)", maxSteps)
}

func (e *Engine) isOffline() bool {
	return e.llmCfg.Backend == "disabled" || !e.cfg.Enabled
}

func (e *Engine) offlineAsk(ctx context.Context, question, scope string, top int) *AskResult {
	if top <= 0 {
		top = 5
	}
	var citations []store.Citation
	var answerBuilder strings.Builder

	answerBuilder.WriteString("> ℹ️ **Offline Mode**: Built-in LLM inference is disabled or unreachable. Displaying raw hybrid search recall results.\n\n")

	if e.searcher != nil {
		results, err := e.searcher.Recall(ctx, search.Query{
			Text:  question,
			Scope: scope,
			Top:   top,
		})
		if err == nil && len(results) > 0 {
			answerBuilder.WriteString("### Matched Memories\n\n")
			for _, r := range results {
				citations = append(citations, store.Citation{
					ID:      r.ID,
					Type:    r.Type,
					Scope:   r.Scope,
					Snippet: truncateSnippet(r.Content, 160),
					Score:   r.Score,
				})
				answerBuilder.WriteString(fmt.Sprintf("- **[id: %d]** (%s in `%s`):\n  %s\n\n",
					r.ID, r.Type, r.Scope, r.Content))
			}
		} else {
			answerBuilder.WriteString("No relevant memories found matching the inquiry in scope `" + scope + "`.\n")
		}
	}

	return &AskResult{
		Answer:         answerBuilder.String(),
		Citations:      citations,
		KnowledgeGaps:  []string{"LLM reasoning is offline; results reflect raw keyword and vector retrieval."},
		ReasoningSteps: 1,
		FallbackUsed:   true,
	}
}

func (e *Engine) offlineCurate(ctx context.Context, scope, curateType string, autoApply bool, dryRun bool) (*CurateResult, error) {
	if e.store == nil {
		return &CurateResult{
			ProposalsCreated: []int64{},
			ProposalsApplied: []int64{},
			FallbackUsed:     true,
		}, nil
	}

	memories, err := e.store.List(ctx, store.ListQuery{
		ScopePath: scope,
		Status:    "active",
		Limit:     500,
	})
	if err != nil {
		return nil, err
	}

	createdIDs := make([]int64, 0)
	appliedIDs := make([]int64, 0)
	var duplicatesCount int

	if curateType == "dedup" || curateType == "all" {
		// Exact hash and identical content grouping
		hashMap := make(map[string][]store.Memory)
		for _, m := range memories {
			key := m.ContentHash
			if key == "" {
				key = strings.TrimSpace(strings.ToLower(m.Content))
			}
			if key != "" {
				hashMap[key] = append(hashMap[key], m)
			}
		}

		hashKeys := make([]string, 0, len(hashMap))
		for k := range hashMap {
			hashKeys = append(hashKeys, k)
		}
		sort.Strings(hashKeys)

		for _, k := range hashKeys {
			group := hashMap[k]
			if len(group) > 1 {
				duplicatesCount += len(group) - 1
				if dryRun {
					continue
				}
				var sids []int64
				for _, m := range group {
					sids = append(sids, m.ID)
				}
				payload, _ := json.Marshal(store.MergeProposalPayload{
					SourceIDs:     sids,
					TargetTitle:   fmt.Sprintf("Consolidate duplicate memories (%d entries)", len(sids)),
					TargetContent: group[0].Content,
					TargetTags:    group[0].Tags,
				})
				propID, err := e.store.CreateProposal(ctx, &store.Proposal{
					ScopeID:      group[0].ScopeID,
					ScopePath:    group[0].ScopePath,
					ProposalType: "merge",
					Title:        fmt.Sprintf("Deduplicate %d identical memories", len(sids)),
					Reasoning:    "Exact content hash duplicate detected in offline mode",
					PayloadJSON:  string(payload),
				})
				if err == nil {
					createdIDs = append(createdIDs, propID)
					if autoApply {
						if err := e.store.ApplyProposal(ctx, propID); err == nil {
							appliedIDs = append(appliedIDs, propID)
						}
					}
				}
			}
		}
	}

	return &CurateResult{
		ProposalsCreated:    createdIDs,
		ProposalsApplied:    appliedIDs,
		ScannedMemories:     len(memories),
		DuplicatesFound:     duplicatesCount,
		ContradictionsFound: 0,
		FallbackUsed:        true,
	}, nil
}

func (e *Engine) offlineSummarize(ctx context.Context, scope string, focus string) (*SummarizeResult, error) {
	title := fmt.Sprintf("Memory Digest — %s", scope)
	if focus != "" {
		title = fmt.Sprintf("Memory Digest: %s (%s)", focus, scope)
	}

	if e.store == nil {
		return &SummarizeResult{
			Title:           title,
			SummaryMarkdown: "> No store available.",
			Scope:           scope,
			FallbackUsed:    true,
		}, nil
	}

	memories, err := e.store.List(ctx, store.ListQuery{
		ScopePath: scope,
		Status:    "active",
		Limit:     200,
	})
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString("> ℹ️ Generated in offline catalog mode (no LLM synthesis).\n\n")

	byType := make(map[string][]store.Memory)
	var citedIDs []int64
	for _, m := range memories {
		byType[m.Type] = append(byType[m.Type], m)
		citedIDs = append(citedIDs, m.ID)
	}

	types := make([]string, 0, len(byType))
	for typ := range byType {
		types = append(types, typ)
	}
	sort.Strings(types)

	for _, typ := range types {
		list := byType[typ]
		sb.WriteString(fmt.Sprintf("## %s Memories (%d)\n\n", titleCase(typ), len(list)))
		for _, m := range list {
			sb.WriteString(fmt.Sprintf("- **[id: %d]** `%s`: %s\n", m.ID, m.ScopePath, m.Content))
		}
		sb.WriteString("\n")
	}

	return &SummarizeResult{
		Title:           title,
		SummaryMarkdown: sb.String(),
		CitedMemoryIDs:  citedIDs,
		Scope:           scope,
		FallbackUsed:    true,
	}, nil
}

func (e *Engine) resolveCitations(ctx context.Context, text string) []store.Citation {
	matches := citationRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	idMap := make(map[int64]bool)
	var ids []int64
	for _, m := range matches {
		if len(m) > 1 {
			id, err := strconv.ParseInt(m[1], 10, 64)
			if err == nil && !idMap[id] {
				idMap[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var citations []store.Citation
	for _, id := range ids {
		c := store.Citation{ID: id}
		if e.store != nil {
			if mem, err := e.store.GetMemory(ctx, id); err == nil {
				c.Type = mem.Type
				c.Scope = mem.ScopePath
				c.Snippet = truncateSnippet(mem.Content, 160)
			}
		}
		citations = append(citations, c)
	}
	return citations
}

func (e *Engine) extractKnowledgeGaps(text string) []string {
	var gaps []string
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToLower(trimmed), "knowledge gap:") ||
			strings.HasPrefix(strings.ToLower(trimmed), "- knowledge gap:") ||
			strings.HasPrefix(strings.ToLower(trimmed), "* knowledge gap:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
				gaps = append(gaps, strings.TrimSpace(parts[1]))
			}
		}
	}
	return gaps
}

func (e *Engine) persistAssistantMessage(ctx context.Context, convID string, res *AskResult) {
	if convID == "" || e.store == nil || res == nil {
		return
	}
	citBytes, _ := json.Marshal(res.Citations)
	_, _ = e.store.AppendMessage(ctx, &store.Message{
		ConversationID: convID,
		Role:           "assistant",
		Content:        res.Answer,
		CitationsJSON:  string(citBytes),
	})
}

func truncateSnippet(s string, maxLen int) string {
	cleaned := strings.ReplaceAll(s, "\n", " ")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	return cleaned[:maxLen-3] + "..."
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
