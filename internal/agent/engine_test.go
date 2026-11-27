package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/hooks"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestEngine_ReActLoopWithToolCalling(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	// Seed memory
	mID, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:react",
		Type:    "note",
		Content: "Authentication is configured with RS256 JWT tokens",
		Tags:    []string{"auth", "jwt"},
	})
	if err != nil {
		t.Fatalf("PutMemory: %v", err)
	}

	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)

		var req agent.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")

		if call == 1 {
			// First turn: Model calls search_memories tool
			resp := agent.ChatResponse{
				ID: "resp-step-1",
				Choices: []agent.Choice{
					{
						Index: 0,
						Message: agent.ChatMessage{
							Role: "assistant",
							ToolCalls: []agent.ToolCall{
								{
									ID:   "call_search_1",
									Type: "function",
									Function: agent.FunctionCall{
										Name:      "search_memories",
										Arguments: `{"query":"Authentication tokens"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Second turn: Model receives tool result, synthesizes answer with citation
		resp := agent.ChatResponse{
			ID: "resp-step-2",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role:    "assistant",
						Content: fmt.Sprintf("Authentication uses RS256 JWT tokens as specified in [id: %d].\n\nKnowledge Gap: Refresh token expiration is not specified.", mID),
					},
					FinishReason: "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.Config{
		LLM: config.LLMConfig{
			Backend:        "openai_compatible",
			Endpoint:       server.URL,
			Model:          "test-react-model",
			TimeoutSeconds: 5,
		},
		Agent: config.AgentConfig{
			Enabled:           true,
			MaxReasoningSteps: 8,
		},
	}

	engine := agent.NewEngine(cfg, s, searcher)

	res, err := engine.Ask(ctx, "What algorithm is used for auth tokens?", agent.InquiryOptions{
		Scope: "project:react",
	})
	if err != nil {
		t.Fatalf("engine.Ask: %v", err)
	}

	if res.ReasoningSteps != 2 {
		t.Errorf("expected 2 reasoning steps, got %d", res.ReasoningSteps)
	}
	if len(res.Citations) != 1 || res.Citations[0].ID != mID {
		t.Errorf("expected citation for memory ID %d, got %+v", mID, res.Citations)
	}
	if len(res.KnowledgeGaps) != 1 {
		t.Errorf("expected 1 knowledge gap, got %d (%v)", len(res.KnowledgeGaps), res.KnowledgeGaps)
	}
	if res.ConversationID == "" {
		t.Errorf("expected non-empty conversation ID")
	}

	// Verify messages saved in conversation
	msgs, err := s.GetConversationMessages(ctx, res.ConversationID)
	if err != nil || len(msgs) != 2 {
		t.Errorf("expected 2 persisted messages in conversation, got %d (%v)", len(msgs), err)
	}
}

func TestEngine_CycleGuardMaxSteps(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)

		var req agent.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")

		// Check if force synthesis prompt arrived
		lastMsg := req.Messages[len(req.Messages)-1]
		if lastMsg.Role == "user" && lastMsg.Content == agent.ForceSynthesisPrompt {
			// Final step returns answer
			resp := agent.ChatResponse{
				ID: "resp-forced",
				Choices: []agent.Choice{
					{
						Index: 0,
						Message: agent.ChatMessage{
							Role:    "assistant",
							Content: "Terminated after maximum step budget. No conclusive answer found.",
						},
						FinishReason: "stop",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Keep calling tools to trigger cycle guard
		resp := agent.ChatResponse{
			ID: "resp-loop",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role: "assistant",
						ToolCalls: []agent.ToolCall{
							{
								ID:   "call_infinite",
								Type: "function",
								Function: agent.FunctionCall{
									Name:      "search_memories",
									Arguments: `{"query":"loop"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.Config{
		LLM: config.LLMConfig{
			Backend:        "openai_compatible",
			Endpoint:       server.URL,
			Model:          "test-model",
			TimeoutSeconds: 5,
		},
		Agent: config.AgentConfig{
			Enabled:           true,
			MaxReasoningSteps: 3, // Low budget to quickly test cycle guard
		},
	}

	engine := agent.NewEngine(cfg, s, searcher)
	res, err := engine.Ask(ctx, "Test loop", agent.InquiryOptions{Scope: "global"})
	if err != nil {
		t.Fatalf("engine.Ask: %v", err)
	}

	if res.ReasoningSteps != 3 {
		t.Errorf("expected exactly 3 reasoning steps, got %d", res.ReasoningSteps)
	}
	if res.Answer != "Terminated after maximum step budget. No conclusive answer found." {
		t.Errorf("unexpected forced synthesis answer: %q", res.Answer)
	}
}

func TestEngine_OfflineFallback(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	// Seed memory
	mID, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:fallback",
		Type:    "note",
		Content: "Use gRPC for inter-process communication",
		Tags:    []string{"ipc", "grpc"},
	})

	// 1. Explicitly disabled backend
	cfgDisabled := config.Config{
		LLM: config.LLMConfig{
			Backend: "disabled",
		},
		Agent: config.AgentConfig{
			Enabled: true,
		},
	}
	engineDisabled := agent.NewEngine(cfgDisabled, s, searcher)

	res, err := engineDisabled.Ask(ctx, "What IPC is used?", agent.InquiryOptions{
		Scope: "project:fallback",
	})
	if err != nil {
		t.Fatalf("Ask disabled backend: %v", err)
	}
	if !res.FallbackUsed {
		t.Errorf("expected FallbackUsed=true")
	}
	if len(res.Citations) == 0 || res.Citations[0].ID != mID {
		t.Errorf("expected raw search citation for ID %d, got %+v", mID, res.Citations)
	}

	// 2. Unreachable backend endpoint -> auto fallback
	cfgUnreachable := config.Config{
		LLM: config.LLMConfig{
			Backend:        "openai_compatible",
			Endpoint:       "http://127.0.0.1:54321/v1", // Dead port
			Model:          "offline-model",
			TimeoutSeconds: 1,
		},
		Agent: config.AgentConfig{
			Enabled:           true,
			MaxReasoningSteps: 3,
		},
	}
	engineUnreachable := agent.NewEngine(cfgUnreachable, s, searcher)

	resUnreachable, err := engineUnreachable.Ask(ctx, "What IPC is used?", agent.InquiryOptions{
		Scope: "project:fallback",
	})
	if err != nil {
		t.Fatalf("Ask unreachable backend: %v", err)
	}
	if !resUnreachable.FallbackUsed {
		t.Errorf("expected FallbackUsed=true when server is unreachable")
	}

	// 3. Offline Curate
	curateRes, err := engineDisabled.Curate(ctx, agent.CurateOptions{
		Scope: "project:fallback",
	})
	if err != nil {
		t.Fatalf("Curate offline: %v", err)
	}
	if !curateRes.FallbackUsed {
		t.Errorf("expected Curate FallbackUsed=true")
	}

	// 4. Offline Summarize
	sumRes, err := engineDisabled.Summarize(ctx, agent.SummarizeOptions{
		Scope: "project:fallback",
	})
	if err != nil {
		t.Fatalf("Summarize offline: %v", err)
	}
	if !sumRes.FallbackUsed {
		t.Errorf("expected Summarize FallbackUsed=true")
	}
	if len(sumRes.CitedMemoryIDs) == 0 {
		t.Errorf("expected cited memory IDs in offline summary")
	}
}

func TestEngine_StreamCallbackExecution(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []string{
			`{"id":"stream-turn-1","choices":[{"index":0,"delta":{"role":"assistant","content":"Streaming "},"finish_reason":null}]}`,
			`{"id":"stream-turn-1","choices":[{"index":0,"delta":{"content":"answer tokens."},"finish_reason":"stop"}]}`,
		}
		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	cfg := config.Config{
		LLM: config.LLMConfig{
			Backend:        "openai_compatible",
			Endpoint:       server.URL,
			Model:          "stream-model",
			TimeoutSeconds: 5,
		},
		Agent: config.AgentConfig{
			Enabled:           true,
			MaxReasoningSteps: 8,
		},
	}

	engine := agent.NewEngine(cfg, s, searcher)

	var streamTokens []string
	res, err := engine.Ask(ctx, "Tell me something", agent.InquiryOptions{
		Scope: "global",
		StreamCallback: func(chunk *agent.StreamChunk) error {
			if chunk.DeltaContent != "" {
				streamTokens = append(streamTokens, chunk.DeltaContent)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("engine.Ask with stream failed: %v", err)
	}

	if len(streamTokens) != 2 {
		t.Fatalf("expected 2 streamed token chunks, got %d (%v)", len(streamTokens), streamTokens)
	}
	if res.Answer != "Streaming answer tokens." {
		t.Errorf("expected answer 'Streaming answer tokens.', got %q", res.Answer)
	}
}

func TestEngine_OfflineScopeIsolation(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	// Memory in scope A
	mA, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:scope-a",
		Type:    "note",
		Content: "Secret config for Scope A",
		Tags:    []string{"a"},
	})
	// Memory in scope B
	mB, _, _ := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:scope-b",
		Type:    "note",
		Content: "Secret config for Scope B",
		Tags:    []string{"b"},
	})
	_ = mA
	_ = mB

	cfgDisabled := config.Config{
		LLM: config.LLMConfig{
			Backend: "disabled",
		},
		Agent: config.AgentConfig{
			Enabled: true,
		},
	}
	engine := agent.NewEngine(cfgDisabled, s, searcher)

	// Summarize scope A only
	sumResA, err := engine.Summarize(ctx, agent.SummarizeOptions{
		Scope: "project:scope-a",
	})
	if err != nil {
		t.Fatalf("Summarize scope A: %v", err)
	}

	// Verify only scope-a memory is cited
	if len(sumResA.CitedMemoryIDs) != 1 || sumResA.CitedMemoryIDs[0] != mA {
		t.Errorf("expected only mA (%d) cited, got %v", mA, sumResA.CitedMemoryIDs)
	}

	// Curate scope B only
	curResB, err := engine.Curate(ctx, agent.CurateOptions{
		Scope: "project:scope-b",
	})
	if err != nil {
		t.Fatalf("Curate scope B: %v", err)
	}
	if curResB.ScannedMemories != 1 {
		t.Errorf("expected 1 scanned memory in scope B, got %d", curResB.ScannedMemories)
	}
}

func TestEngine_Curate_OnlineReAct_WebhooksAutoApplyAndDryRun(t *testing.T) {
	s, searcher := setupTestStoreAndSearcher(t)
	ctx := context.Background()

	m1, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:online-hooks",
		Type:    "note",
		Content: "Use PostgreSQL 16 for primary persistence.",
	})
	if err != nil {
		t.Fatalf("PutMemory m1: %v", err)
	}
	m2, _, err := s.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:online-hooks",
		Type:    "note",
		Content: "Use MySQL 8 for primary persistence.",
	})
	if err != nil {
		t.Fatalf("PutMemory m2: %v", err)
	}

	type hookHit struct {
		Event   string
		Sig     string
		RawBody []byte
		Payload hooks.Payload
	}
	var mu sync.Mutex
	var hits []hookHit
	hookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p hooks.Payload
		_ = json.Unmarshal(body, &p)
		mu.Lock()
		hits = append(hits, hookHit{
			Event:   r.Header.Get("X-Centmem-Event"),
			Sig:     r.Header.Get("X-Centmem-Signature-256"),
			RawBody: body,
			Payload: p,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer hookSrv.Close()

	// Mock OpenAI server that stages one link proposal on turn 1, then finishes on turn 2.
	var turn int32
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&turn, 1)
		w.Header().Set("Content-Type", "application/json")
		if n%2 == 1 {
			args := fmt.Sprintf(`{"from_id":%d,"to_id":%d,"relation":"contradicts","reasoning":"Conflicting DB engines"}`, m1, m2)
			resp := map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{
									"id":   fmt.Sprintf("call_%d", n),
									"type": "function",
									"function": map[string]any{
										"name":      "propose_link",
										"arguments": args,
									},
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Curation complete.",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer llmSrv.Close()

	secret := "online-curate-secret-99"
	cfg := config.Config{
		LLM: config.LLMConfig{
			Backend:        "openai_compatible",
			Endpoint:       llmSrv.URL,
			Model:          "mock-model",
			TimeoutSeconds: 10,
		},
		Agent: config.AgentConfig{
			Enabled:           true,
			MaxReasoningSteps: 5,
		},
		Hooks: config.HooksConfig{
			Enabled:          true,
			ProposalsCreated: hookSrv.URL,
			ProposalsApplied: hookSrv.URL,
			Secret:           secret,
			TimeoutSeconds:   5,
		},
	}
	engine := agent.NewEngine(cfg, s, searcher)

	// 1. DryRun must emit ZERO webhooks
	_, err = engine.Curate(ctx, agent.CurateOptions{
		Scope:     "project:online-hooks",
		Type:      "contradictions",
		AutoApply: true,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("Curate dry-run: %v", err)
	}
	flushCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	engine.FlushHooks(flushCtx)
	cancel()

	mu.Lock()
	if len(hits) != 0 {
		mu.Unlock()
		t.Fatalf("expected 0 webhooks on dry-run, got %d", len(hits))
	}
	mu.Unlock()

	// 2. Online Curate with AutoApply: true must emit BOTH proposals.created AND proposals.applied
	res, err := engine.Curate(ctx, agent.CurateOptions{
		Scope:     "project:online-hooks",
		Type:      "contradictions",
		AutoApply: true,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Curate online auto-apply: %v", err)
	}
	if res.FallbackUsed {
		t.Fatalf("expected online ReAct mode, got fallback")
	}
	if len(res.ProposalsCreated) != 1 || len(res.ProposalsApplied) != 1 {
		t.Fatalf("expected 1 created and 1 applied proposal, got created=%v applied=%v", res.ProposalsCreated, res.ProposalsApplied)
	}

	flushCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	engine.FlushHooks(flushCtx2)
	cancel2()

	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 2 {
		t.Fatalf("expected 2 webhook events (proposals.created + proposals.applied), got %d: %+v", len(hits), hits)
	}
	eventsSeen := map[string]bool{}
	for _, h := range hits {
		eventsSeen[h.Event] = true
		if want := hooks.SignPayload(secret, h.RawBody); h.Sig != want {
			t.Errorf("HMAC mismatch for %s: got %q want %q", h.Event, h.Sig, want)
		}
		if h.Payload.Scope != "project:online-hooks" || h.Payload.Count != 1 {
			t.Errorf("unexpected payload for %s: %+v", h.Event, h.Payload)
		}
		if len(h.Payload.Proposals) != 1 || h.Payload.Proposals[0].ID != res.ProposalsCreated[0] {
			t.Errorf("unexpected proposal summary for %s: %+v", h.Event, h.Payload.Proposals)
		}
	}
	if !eventsSeen[hooks.EventProposalsCreated] || !eventsSeen[hooks.EventProposalsApplied] {
		t.Fatalf("expected both proposals.created and proposals.applied, got %v", eventsSeen)
	}
}
