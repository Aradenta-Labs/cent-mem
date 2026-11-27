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
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/hooks"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"github.com/aradenta-labs/cent-mem/internal/taxonomy"
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
	hooks    *hooks.Dispatcher
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
		hooks:    hooks.NewDispatcher(cfg.Hooks),
	}
}

// FlushHooks waits for in-flight webhook notifications to finish up to ctx's deadline.
func (e *Engine) FlushHooks(ctx context.Context) {
	if e != nil && e.hooks != nil {
		e.hooks.Flush(ctx)
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

// Curate runs autonomous memory curation (contradictions, deduplication, tag clustering, or all).
func (e *Engine) Curate(ctx context.Context, opts CurateOptions) (*CurateResult, error) {
	scopePath := opts.Scope
	if scopePath == "" {
		scopePath = "global"
	}
	curateType := opts.Type
	if curateType == "" {
		curateType = "all"
	}

	if e.isOffline() {
		return e.offlineCurate(ctx, scopePath, curateType, opts.AutoApply, opts.DryRun)
	}

	if curateType == "tags" {
		tagClusters, createdIDs, appliedIDs, createdSummaries, appliedSummaries, err := e.curateTags(ctx, scopePath, opts.AutoApply, opts.DryRun, true)
		if err != nil {
			return nil, err
		}
		if !opts.DryRun && e.hooks != nil {
			if len(createdSummaries) > 0 {
				e.hooks.Emit(hooks.EventProposalsCreated, hooks.Payload{
					Scope:     scopePath,
					Count:     len(createdSummaries),
					Proposals: createdSummaries,
				})
			}
			if len(appliedSummaries) > 0 {
				e.hooks.Emit(hooks.EventProposalsApplied, hooks.Payload{
					Scope:     scopePath,
					Count:     len(appliedSummaries),
					Proposals: appliedSummaries,
				})
			}
		}
		var scannedCount int
		if e.store != nil {
			if sc, parseErr := scope.Parse(scopePath); parseErr == nil {
				if ids, idErr := e.store.ResolveScopeIDs(ctx, sc, false, true); idErr == nil && len(ids) > 0 {
					if count, countErr := e.store.Count(ctx, store.ListQuery{ScopeIDs: ids, Status: "active"}); countErr == nil {
						scannedCount = int(count)
					}
				}
			}
		}
		return &CurateResult{
			ProposalsCreated: createdIDs,
			ProposalsApplied: appliedIDs,
			ScannedMemories:  scannedCount,
			TagClustersFound: tagClusters,
			FallbackUsed:     false,
		}, nil
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

	userMsg := fmt.Sprintf("Run memory curation for scope %q. Inspect memories, find conflicts and duplicates, and create proposals.", scopePath)
	if opts.DryRun {
		ctx = context.WithValue(ctx, dryRunContextKey, true)
		userMsg += " (DRY RUN: do not persist changes to store)"
	}

	messages := []ChatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: userMsg},
	}

	// Track proposals count before loop
	existingProps, _ := e.store.ListProposals(ctx, store.ProposalListQuery{ScopePath: scopePath, Limit: 10000})
	preExistingMap := make(map[int64]bool)
	for _, p := range existingProps {
		preExistingMap[p.ID] = true
	}

	_, _, err := e.RunReActLoop(ctx, messages, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[centmem/agent] Curate ReAct loop failed, falling back to offline mode: %v\n", err)
		return e.offlineCurate(ctx, scopePath, curateType, opts.AutoApply, opts.DryRun)
	}

	// Find newly created proposals
	afterProps, _ := e.store.ListProposals(ctx, store.ProposalListQuery{ScopePath: scopePath, Limit: 10000})
	createdIDs := make([]int64, 0)
	appliedIDs := make([]int64, 0)
	createdSummaries := make([]hooks.ProposalSummary, 0)
	appliedSummaries := make([]hooks.ProposalSummary, 0)
	var contradictionsCount, duplicatesCount, tagClustersCount int

	for _, p := range afterProps {
		if !preExistingMap[p.ID] {
			createdIDs = append(createdIDs, p.ID)
			summary := hooks.ProposalSummary{ID: p.ID, Type: p.ProposalType, Title: p.Title}
			createdSummaries = append(createdSummaries, summary)
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
					appliedSummaries = append(appliedSummaries, summary)
				}
			}
		}
	}

	if curateType == "all" {
		tc, tCreated, tApplied, tCreatedSum, tAppliedSum, tagErr := e.curateTags(ctx, scopePath, opts.AutoApply, opts.DryRun, true)
		if tagErr != nil {
			return nil, tagErr
		}
		tagClustersCount = tc
		createdIDs = append(createdIDs, tCreated...)
		appliedIDs = append(appliedIDs, tApplied...)
		createdSummaries = append(createdSummaries, tCreatedSum...)
		appliedSummaries = append(appliedSummaries, tAppliedSum...)
	}

	if !opts.DryRun && e.hooks != nil {
		if len(createdSummaries) > 0 {
			e.hooks.Emit(hooks.EventProposalsCreated, hooks.Payload{
				Scope:     scopePath,
				Count:     len(createdSummaries),
				Proposals: createdSummaries,
			})
		}
		if len(appliedSummaries) > 0 {
			e.hooks.Emit(hooks.EventProposalsApplied, hooks.Payload{
				Scope:     scopePath,
				Count:     len(appliedSummaries),
				Proposals: appliedSummaries,
			})
		}
	}

	var scannedCount int
	if count, countErr := e.store.Count(ctx, store.ListQuery{ScopePath: scopePath, Status: "active"}); countErr == nil {
		scannedCount = int(count)
	}

	return &CurateResult{
		ProposalsCreated:    createdIDs,
		ProposalsApplied:    appliedIDs,
		ScannedMemories:     scannedCount,
		ContradictionsFound: contradictionsCount,
		DuplicatesFound:     duplicatesCount,
		TagClustersFound:    tagClustersCount,
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

func (e *Engine) offlineCurate(ctx context.Context, scopePath, curateType string, autoApply bool, dryRun bool) (*CurateResult, error) {
	if e.store == nil {
		return &CurateResult{
			ProposalsCreated: []int64{},
			ProposalsApplied: []int64{},
			FallbackUsed:     true,
		}, nil
	}

	memories, err := e.store.List(ctx, store.ListQuery{
		ScopePath: scopePath,
		Status:    "active",
		Limit:     500,
	})
	if err != nil {
		return nil, err
	}

	createdIDs := make([]int64, 0)
	appliedIDs := make([]int64, 0)
	createdSummaries := make([]hooks.ProposalSummary, 0)
	appliedSummaries := make([]hooks.ProposalSummary, 0)
	var duplicatesCount, tagClustersCount int

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
				propTitle := fmt.Sprintf("Deduplicate %d identical memories", len(sids))
				propID, err := e.store.CreateProposal(ctx, &store.Proposal{
					ScopeID:      group[0].ScopeID,
					ScopePath:    group[0].ScopePath,
					ProposalType: "merge",
					Title:        propTitle,
					Reasoning:    "Exact content hash duplicate detected in offline mode",
					PayloadJSON:  string(payload),
				})
				if err == nil {
					createdIDs = append(createdIDs, propID)
					summary := hooks.ProposalSummary{ID: propID, Type: "merge", Title: propTitle}
					createdSummaries = append(createdSummaries, summary)
					if autoApply {
						if err := e.store.ApplyProposal(ctx, propID); err == nil {
							appliedIDs = append(appliedIDs, propID)
							appliedSummaries = append(appliedSummaries, summary)
						}
					}
				}
			}
		}
	}

	if curateType == "tags" || curateType == "all" {
		tc, tCreated, tApplied, tCreatedSum, tAppliedSum, tagErr := e.curateTags(ctx, scopePath, autoApply, dryRun, false)
		if tagErr != nil {
			return nil, tagErr
		}
		tagClustersCount = tc
		createdIDs = append(createdIDs, tCreated...)
		appliedIDs = append(appliedIDs, tApplied...)
		createdSummaries = append(createdSummaries, tCreatedSum...)
		appliedSummaries = append(appliedSummaries, tAppliedSum...)
	}

	if !dryRun && e.hooks != nil {
		if len(createdSummaries) > 0 {
			e.hooks.Emit(hooks.EventProposalsCreated, hooks.Payload{
				Scope:     scopePath,
				Count:     len(createdSummaries),
				Proposals: createdSummaries,
			})
		}
		if len(appliedSummaries) > 0 {
			e.hooks.Emit(hooks.EventProposalsApplied, hooks.Payload{
				Scope:     scopePath,
				Count:     len(appliedSummaries),
				Proposals: appliedSummaries,
			})
		}
	}

	scannedMemories := len(memories)
	if curateType == "tags" {
		if sc, parseErr := scope.Parse(scopePath); parseErr == nil {
			if ids, idErr := e.store.ResolveScopeIDs(ctx, sc, false, true); idErr == nil {
				if len(ids) == 0 {
					scannedMemories = 0
				} else if count, countErr := e.store.Count(ctx, store.ListQuery{ScopeIDs: ids, Status: "active"}); countErr == nil {
					scannedMemories = int(count)
				}
			}
		}
	}

	return &CurateResult{
		ProposalsCreated:    createdIDs,
		ProposalsApplied:    appliedIDs,
		ScannedMemories:     scannedMemories,
		DuplicatesFound:     duplicatesCount,
		ContradictionsFound: 0,
		TagClustersFound:    tagClustersCount,
		FallbackUsed:        true,
	}, nil
}

func (e *Engine) curateTags(
	ctx context.Context,
	scopePath string,
	autoApply bool,
	dryRun bool,
	useLLMRefine bool,
) (int, []int64, []int64, []hooks.ProposalSummary, []hooks.ProposalSummary, error) {
	createdIDs := make([]int64, 0)
	appliedIDs := make([]int64, 0)
	createdSummaries := make([]hooks.ProposalSummary, 0)
	appliedSummaries := make([]hooks.ProposalSummary, 0)

	if e.store == nil {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, nil
	}

	sc, err := scope.Parse(scopePath)
	if err != nil {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, err
	}
	scopeIDs, err := e.store.ResolveScopeIDs(ctx, sc, false, true)
	if err != nil {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, err
	}
	if len(scopeIDs) == 0 {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, nil
	}

	stats, err := e.store.ListTagStats(ctx, scopeIDs)
	if err != nil {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, err
	}
	if len(stats) < 2 {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, nil
	}

	threshold := e.cfg.TagSimilarityThreshold
	if threshold <= 0 {
		threshold = taxonomy.DefaultSimilarityThreshold
	}
	var emb embed.Embedder
	if e.searcher != nil {
		emb = e.searcher.Embedder()
	}

	clusters, err := taxonomy.ClusterTags(ctx, stats, taxonomy.Options{
		SimilarityThreshold: threshold,
		Embedder:            emb,
	})
	if err != nil {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, err
	}
	if len(clusters) == 0 {
		return 0, createdIDs, appliedIDs, createdSummaries, appliedSummaries, nil
	}

	// Build default reasons; optionally let online LLM refine cluster canonical/reasoning.
	reasons := make([]string, len(clusters))
	for i, cl := range clusters {
		switch cl.MatchMethod {
		case "semantic":
			reasons[i] = fmt.Sprintf("Semantic tag cluster detected (cosine similarity %.2f >= %.2f)", cl.Similarity, threshold)
		case "hybrid":
			reasons[i] = fmt.Sprintf("Lexical and semantic tag cluster detected (cosine similarity %.2f)", cl.Similarity)
		default:
			reasons[i] = "Lexical tag normalization cluster detected (case, separator, singular/plural, or typo match)"
		}
	}

	if useLLMRefine && !dryRun && !e.isOffline() && e.client != nil {
		e.refineTagClustersWithLLM(ctx, clusters, reasons)
	}

	// Deduplicate against already pending tag_merge proposals in this scope.
	pendingKeys := make(map[string]bool)
	if !dryRun {
		existing, _ := e.store.ListProposals(ctx, store.ProposalListQuery{
			ScopePath:    scopePath,
			Status:       "pending",
			ProposalType: "update",
			Limit:        1000,
		})
		for _, p := range existing {
			var tm store.TagMergeProposalPayload
			if json.Unmarshal([]byte(p.PayloadJSON), &tm) == nil && tm.Kind == "tag_merge" {
				pendingKeys[tagMergeKey(tm.ToTag, tm.FromTags)] = true
			}
		}
	}

	clustersFound := 0
	for i, cl := range clusters {
		// Store.BulkRenameTag always writes lowercase tags via joinTags; ensure canonical is lowercase
		// and all members differing from it (including case-only variants) are listed in fromTags.
		toTag := strings.TrimSpace(strings.ToLower(cl.Canonical))
		if toTag == "" {
			continue
		}
		fromTags := make([]string, 0, len(cl.Members))
		for _, m := range cl.Members {
			if m != toTag {
				fromTags = append(fromTags, m)
			}
		}
		sort.Strings(fromTags)
		if len(fromTags) == 0 {
			continue
		}

		affected, err := e.store.CountMemoriesWithAnyTag(ctx, scopeIDs, fromTags, toTag)
		if err != nil || affected == 0 {
			continue
		}
		clustersFound++
		if dryRun {
			continue
		}
		key := tagMergeKey(toTag, fromTags)
		if pendingKeys[key] {
			continue
		}
		pendingKeys[key] = true

		payloadBytes, _ := json.Marshal(store.TagMergeProposalPayload{
			Kind:          "tag_merge",
			FromTags:      fromTags,
			ToTag:         toTag,
			AffectedCount: affected,
		})
		title := fmt.Sprintf("Merge tags %s → %s", strings.Join(fromTags, ", "), toTag)
		propID, err := e.store.CreateProposal(ctx, &store.Proposal{
			ScopePath:    scopePath,
			ProposalType: "update",
			Title:        title,
			Reasoning:    reasons[i],
			PayloadJSON:  string(payloadBytes),
		})
		if err != nil {
			continue
		}
		createdIDs = append(createdIDs, propID)
		summary := hooks.ProposalSummary{ID: propID, Type: "update", Title: title}
		createdSummaries = append(createdSummaries, summary)
		if autoApply {
			if err := e.store.ApplyProposal(ctx, propID); err == nil {
				appliedIDs = append(appliedIDs, propID)
				appliedSummaries = append(appliedSummaries, summary)
			}
		}
	}

	return clustersFound, createdIDs, appliedIDs, createdSummaries, appliedSummaries, nil
}

func (e *Engine) refineTagClustersWithLLM(ctx context.Context, clusters []taxonomy.Cluster, reasons []string) {
	input, err := json.Marshal(clusters)
	if err != nil {
		return
	}
	resp, err := e.client.Chat(ctx, &ChatRequest{
		Model: e.llmCfg.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: `You refine tag canonicalization proposals. Given a JSON array of tag clusters, return ONLY a JSON array of objects [{"index": 0, "canonical": "<one of members>", "reasoning": "<concise 1-sentence reason>"}]. Do not add commentary.`,
			},
			{
				Role:    "user",
				Content: string(input),
			},
		},
	})
	if err != nil || len(resp.Choices) == 0 {
		return
	}
	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Strip optional markdown code fences if present.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var refinements []struct {
		Index     int    `json:"index"`
		Canonical string `json:"canonical"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(raw), &refinements); err != nil {
		return
	}
	for _, ref := range refinements {
		if ref.Index < 0 || ref.Index >= len(clusters) {
			continue
		}
		if r := strings.TrimSpace(ref.Reasoning); r != "" {
			reasons[ref.Index] = r
		}
		cand := strings.TrimSpace(strings.ToLower(ref.Canonical))
		if cand == "" || cand == clusters[ref.Index].Canonical {
			continue
		}
		// Only accept canonical if it matches one of the cluster's members (case-insensitively).
		found := false
		for _, m := range clusters[ref.Index].Members {
			if strings.ToLower(m) == cand {
				found = true
				break
			}
		}
		if found {
			var newAliases []string
			for _, m := range clusters[ref.Index].Members {
				if m != cand {
					newAliases = append(newAliases, m)
				}
			}
			sort.Strings(newAliases)
			clusters[ref.Index].Canonical = cand
			clusters[ref.Index].Aliases = newAliases
		}
	}
}

func tagMergeKey(toTag string, fromTags []string) string {
	cp := make([]string, len(fromTags))
	for i, t := range fromTags {
		cp[i] = strings.TrimSpace(strings.ToLower(t))
	}
	sort.Strings(cp)
	return strings.TrimSpace(strings.ToLower(toTag)) + "|" + strings.Join(cp, ",")
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
