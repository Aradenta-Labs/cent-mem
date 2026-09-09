package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestClient_NonStreamingChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret-key-123" {
			t.Errorf("expected Authorization header Bearer secret-key-123, got %q", r.Header.Get("Authorization"))
		}

		var req agent.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("expected model test-model, got %q", req.Model)
		}

		resp := agent.ChatResponse{
			ID: "chatcmpl-test-1",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role:    "assistant",
						Content: "Hello world!",
					},
					FinishReason: "stop",
				},
			},
			Usage: agent.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.LLMConfig{
		Backend:        "openai_compatible",
		Endpoint:       server.URL,
		Model:          "test-model",
		APIKey:         "secret-key-123",
		TimeoutSeconds: 5,
	}

	client := agent.NewClient(cfg)
	res, err := client.Chat(context.Background(), &agent.ChatRequest{
		Messages: []agent.ChatMessage{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if len(res.Choices) != 1 || res.Choices[0].Message.Content != "Hello world!" {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestClient_NonStreamingToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := agent.ChatResponse{
			ID: "chatcmpl-tool-1",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role: "assistant",
						ToolCalls: []agent.ToolCall{
							{
								ID:   "call_abc123",
								Type: "function",
								Function: agent.FunctionCall{
									Name:      "search_memories",
									Arguments: `{"query":"auth"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := agent.NewClient(config.LLMConfig{
		Endpoint: server.URL,
		Model:    "test-tool-model",
	})

	res, err := client.Chat(context.Background(), &agent.ChatRequest{
		Messages: []agent.ChatMessage{
			{Role: "user", Content: "search for auth"},
		},
	})
	if err != nil {
		t.Fatalf("Chat tool calls failed: %v", err)
	}
	if len(res.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(res.Choices[0].Message.ToolCalls))
	}
	tc := res.Choices[0].Message.ToolCalls[0]
	if tc.ID != "call_abc123" || tc.Function.Name != "search_memories" {
		t.Errorf("tool call mismatch: %+v", tc)
	}
}

func TestClient_StreamingSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("ResponseWriter does not support Flusher")
		}

		chunks := []string{
			`{"id":"chatcmpl-stream-1","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-stream-1","choices":[{"index":0,"delta":{"content":" stream"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-stream-1","choices":[{"index":0,"delta":{"content":"ing!"},"finish_reason":"stop"}]}`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := agent.NewClient(config.LLMConfig{
		Endpoint: server.URL,
		Model:    "stream-model",
	})

	var collected []string
	accumulated, err := client.StreamChat(context.Background(), &agent.ChatRequest{
		Messages: []agent.ChatMessage{
			{Role: "user", Content: "hello"},
		},
	}, func(chunk *agent.StreamChunk) error {
		if chunk.DeltaContent != "" {
			collected = append(collected, chunk.DeltaContent)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}
	if len(collected) != 3 {
		t.Errorf("expected 3 chunks, got %d: %v", len(collected), collected)
	}
	if accumulated.Choices[0].Message.Content != "Hello streaming!" {
		t.Errorf("expected 'Hello streaming!', got %q", accumulated.Choices[0].Message.Content)
	}
}

func TestClient_RetryTransientErrors(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attempts, 1)
		if att < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		resp := agent.ChatResponse{
			ID: "chatcmpl-retry-success",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role:    "assistant",
						Content: "Recovered after retry",
					},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := agent.NewClient(config.LLMConfig{
		Endpoint: server.URL,
		Model:    "retry-model",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := client.Chat(ctx, &agent.ChatRequest{
		Messages: []agent.ChatMessage{
			{Role: "user", Content: "retry test"},
		},
	})
	if err != nil {
		t.Fatalf("Chat should succeed after retries: %v", err)
	}
	if res.Choices[0].Message.Content != "Recovered after retry" {
		t.Errorf("unexpected response content: %q", res.Choices[0].Message.Content)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_StreamingNonContiguousToolIndices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send tool call at index 0 and tool call at index 2 (simulating sparse/non-contiguous stream)
		chunks := []string{
			`{"id":"sparse-1","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_0","type":"function","function":{"name":"search_memories","arguments":"{\"query\":\"a\"}"}}]},"finish_reason":null}]}`,
			`{"id":"sparse-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":2,"id":"call_2","type":"function","function":{"name":"read_memory","arguments":"{\"id\":42}"}}]},"finish_reason":"tool_calls"}]}`,
		}
		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := agent.NewClient(config.LLMConfig{
		Endpoint: server.URL,
		Model:    "sparse-model",
	})

	accumulated, err := client.StreamChat(context.Background(), &agent.ChatRequest{
		Messages: []agent.ChatMessage{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	toolCalls := accumulated.Choices[0].Message.ToolCalls
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls preserved from non-contiguous stream, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_0" || toolCalls[1].ID != "call_2" {
		t.Errorf("unexpected tool calls: %+v", toolCalls)
	}
}

func TestClient_StreamingLargeLine(t *testing.T) {
	largeContent := make([]byte, 80*1024) // 80KB > 64KB default bufio.MaxScanTokenSize
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunk := fmt.Sprintf(`{"id":"large-1","choices":[{"index":0,"delta":{"role":"assistant","content":%q},"finish_reason":"stop"}]}`, string(largeContent))
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := agent.NewClient(config.LLMConfig{
		Endpoint: server.URL,
		Model:    "large-model",
	})

	accumulated, err := client.StreamChat(context.Background(), &agent.ChatRequest{
		Messages: []agent.ChatMessage{{Role: "user", Content: "give me large data"}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamChat failed on large payload: %v", err)
	}
	if len(accumulated.Choices[0].Message.Content) != len(largeContent) {
		t.Errorf("expected %d bytes, got %d", len(largeContent), len(accumulated.Choices[0].Message.Content))
	}
}
