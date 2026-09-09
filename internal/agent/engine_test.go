package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/config"
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
