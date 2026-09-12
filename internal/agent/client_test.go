package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClient_NonStreamingChatReceivingSSE(t *testing.T) {
	// Tests reverse proxies / routers that return SSE streams (data: ...)
	// even when stream: false is requested.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunk1 := `{"id":"sse-stream-1","choices":[{"index":0,"delta":{"role":"assistant","content":"Streamed "},"finish_reason":null}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk1)
		flusher.Flush()

		chunk2 := `{"id":"sse-stream-1","choices":[{"index":0,"delta":{"content":"response!"},"finish_reason":"stop"}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk2)
		flusher.Flush()

		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := agent.NewClient(config.LLMConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	resp, err := client.Chat(context.Background(), &agent.ChatRequest{
		Messages: []agent.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat failed when receiving SSE: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "Streamed response!" {
		t.Errorf("expected 'Streamed response!', got %+v", resp.Choices[0].Message)
	}
}

func TestProbeEndpoint(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/models") {
				t.Errorf("expected /models path, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("expected Bearer test-key, got %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
		}))
		defer server.Close()

		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: server.URL,
			Model:    "test-model",
			APIKey:   "test-key",
		}, nil)

		if !res.OK {
			t.Fatalf("expected OK true, got false, message: %s", res.Message)
		}
		if res.Status != "connected" {
			t.Errorf("expected status connected, got %s", res.Status)
		}
		if res.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", res.Model)
		}
		if res.Latency <= 0 {
			t.Errorf("expected latency > 0, got %v", res.Latency)
		}
	})

	t.Run("auth_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`invalid api key`))
		}))
		defer server.Close()

		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: server.URL,
			Model:    "test-model",
			APIKey:   "bad-key",
		}, nil)

		if res.OK {
			t.Fatalf("expected OK false, got true")
		}
		if res.Status != "auth_error" {
			t.Errorf("expected status auth_error, got %s", res.Status)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: "http://127.0.0.1:59999/v1",
			Model:    "test-model",
		}, nil)

		if res.OK {
			t.Fatalf("expected OK false, got true")
		}
		if res.Status != "unreachable" {
			t.Errorf("expected status unreachable, got %s", res.Status)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Backend:  "disabled",
			Endpoint: "http://127.0.0.1:11434/v1",
			Model:    "test-model",
		}, nil)

		if res.OK {
			t.Fatalf("expected OK false, got true")
		}
		if res.Status != "disabled" {
			t.Errorf("expected status disabled, got %s", res.Status)
		}
	})

	t.Run("missing_api_key", func(t *testing.T) {
		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: "http://127.0.0.1:11434/v1",
			Model:    "test-model",
			APIKey:   "$NON_EXISTENT_ENV_KEY_12345",
		}, nil)

		if res.OK {
			t.Fatalf("expected OK false, got true")
		}
		if res.Status != "missing_api_key" {
			t.Errorf("expected status missing_api_key, got %s", res.Status)
		}
	})

	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`model loading failed`))
		}))
		defer server.Close()

		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: server.URL,
			Model:    "test-model",
		}, nil)

		if res.OK {
			t.Fatalf("expected OK false, got true")
		}
		if res.Status != "error" {
			t.Errorf("expected status error, got %s", res.Status)
		}
	})

	t.Run("gemini_openai_url", func(t *testing.T) {
		var requestedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"gemini-1.5-flash"}]}`))
		}))
		defer server.Close()

		// Endpoint ending with /openai (like Gemini base URL)
		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: server.URL + "/v1beta/openai",
			Model:    "gemini-1.5-flash",
		}, nil)

		if !res.OK {
			t.Fatalf("expected OK true, got false: %s", res.Message)
		}
		if requestedPath != "/v1beta/openai/models" {
			t.Errorf("expected path /v1beta/openai/models, got %q", requestedPath)
		}
	})

	t.Run("actionable_advice_on_unreachable", func(t *testing.T) {
		res := agent.ProbeEndpoint(context.Background(), config.LLMConfig{
			Endpoint: "http://127.0.0.1:59995/v1",
			Model:    "test-model",
		}, nil)

		if !strings.Contains(res.Message, "ensure local model server is running") {
			t.Errorf("expected local model advice in message, got %q", res.Message)
		}
	})
}

