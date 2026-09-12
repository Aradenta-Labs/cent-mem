package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

// FunctionDefinition defines an OpenAI-compatible function schema.
type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolDefinition wraps a function definition in OpenAI tool format.
type ToolDefinition struct {
	Type     string             `json:"type"` // always "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionCall describes an invocation of a tool function.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a tool call requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// ToolCallChunk represents a streaming delta for a tool call.
type ToolCallChunk struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function,omitempty"`
}

// ChatMessage represents a single turn in a chat conversation.
type ChatMessage struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest parameterizes an OpenAI-compatible completion request.
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"` // "auto", "none", or object
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// Usage captures token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice represents a completion choice in ChatResponse.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatResponse is the payload returned by non-streaming completions.
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// StreamChunk contains incremental delta tokens from an SSE stream.
type StreamChunk struct {
	ID           string          `json:"id,omitempty"`
	DeltaRole    string          `json:"delta_role,omitempty"`
	DeltaContent string          `json:"delta_content,omitempty"`
	DeltaTools   []ToolCallChunk `json:"delta_tools,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

// Client is a zero-dependency HTTP client for OpenAI-compatible LLMs.
type Client struct {
	cfg        config.LLMConfig
	httpClient *http.Client
	url        string
	apiKey     string
}

// NewClient initializes an LLM client from configuration.
func NewClient(cfg config.LLMConfig) *Client {
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434/v1"
	}
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		if !strings.HasSuffix(endpoint, "/v1") && !strings.HasSuffix(endpoint, "/openai") {
			endpoint += "/v1"
		}
		endpoint += "/chat/completions"
	}

	apiKey, _ := config.ResolveAPIKey(cfg.APIKey)

	timeout := 60 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
		url:        endpoint,
		apiKey:     apiKey,
	}
}

// Config returns the underlying LLMConfig.
func (c *Client) Config() config.LLMConfig {
	return c.cfg
}

// Chat sends a non-streaming chat request with retries for transient errors.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	r := *req
	r.Stream = false
	if r.Model == "" {
		r.Model = c.cfg.Model
	}
	if r.Temperature == nil && c.cfg.Temperature >= 0 {
		temp := c.cfg.Temperature
		r.Temperature = &temp
	}
	if r.MaxTokens == nil && c.cfg.MaxTokens > 0 {
		maxT := c.cfg.MaxTokens
		r.MaxTokens = &maxT
	}

	bodyBytes, err := json.Marshal(&r)
	if err != nil {
		return nil, fmt.Errorf("agent: marshal chat request: %w", err)
	}

	var resp *ChatResponse
	err = c.retry(ctx, func() (bool, error) {
		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return false, reqErr
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json, text/event-stream")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		httpResp, reqErr := c.httpClient.Do(httpReq)
		if reqErr != nil {
			// Network error, transient retry
			return true, reqErr
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode == http.StatusTooManyRequests ||
			httpResp.StatusCode == http.StatusBadGateway ||
			httpResp.StatusCode == http.StatusServiceUnavailable ||
			httpResp.StatusCode == http.StatusGatewayTimeout {
			body, _ := io.ReadAll(httpResp.Body)
			return true, fmt.Errorf("agent: http status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
		}

		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			body, _ := io.ReadAll(httpResp.Body)
			return false, fmt.Errorf("agent: llm request failed (status %d): %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
		}

		bodyBytes, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return false, fmt.Errorf("agent: read response: %w", readErr)
		}

		trimmed := strings.TrimSpace(string(bodyBytes))
		// Handle reverse proxies / gateways that return SSE stream ("data: ...") even when stream: false was requested
		if strings.HasPrefix(trimmed, "data:") || strings.Contains(httpResp.Header.Get("Content-Type"), "text/event-stream") {
			sseResp, sseErr := parseSSEReader(bytes.NewReader(bodyBytes), nil)
			if sseErr != nil {
				return false, fmt.Errorf("agent: decode sse response: %w", sseErr)
			}
			resp = sseResp
			return false, nil
		}

		var chatResp ChatResponse
		if decodeErr := json.Unmarshal(bodyBytes, &chatResp); decodeErr != nil {
			preview := trimmed
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			return false, fmt.Errorf("agent: decode response (%q): %w", preview, decodeErr)
		}
		resp = &chatResp
		return false, nil
	})

	if err != nil {
		return nil, err
	}
	return resp, nil
}

// StreamChat sends a streaming chat completion request, calling onChunk for each incoming SSE delta,
// and returns the fully accumulated ChatResponse upon completion.
func (c *Client) StreamChat(ctx context.Context, req *ChatRequest, onChunk func(chunk *StreamChunk) error) (*ChatResponse, error) {
	r := *req
	r.Stream = true
	if r.Model == "" {
		r.Model = c.cfg.Model
	}
	if r.Temperature == nil && c.cfg.Temperature >= 0 {
		temp := c.cfg.Temperature
		r.Temperature = &temp
	}
	if r.MaxTokens == nil && c.cfg.MaxTokens > 0 {
		maxT := c.cfg.MaxTokens
		r.MaxTokens = &maxT
	}

	bodyBytes, err := json.Marshal(&r)
	if err != nil {
		return nil, fmt.Errorf("agent: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("agent: stream request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("agent: stream request failed (status %d): %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	return parseSSEReader(httpResp.Body, onChunk)
}

// parseSSEReader reads and decodes an SSE event stream from r, invoking onChunk if non-nil,
// and returns the fully assembled ChatResponse.
func parseSSEReader(r io.Reader, onChunk func(chunk *StreamChunk) error) (*ChatResponse, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // up to 10MB per line to avoid ErrTooLong on large payloads
	var accumulatedContent strings.Builder
	var accumulatedRole string
	var finishReason string
	var streamID string
	toolCallsMap := make(map[int]*ToolCall)

	type rawStreamDelta struct {
		Role      string          `json:"role,omitempty"`
		Content   string          `json:"content,omitempty"`
		ToolCalls []ToolCallChunk `json:"tool_calls,omitempty"`
	}
	type rawStreamChoice struct {
		Index        int            `json:"index"`
		Delta        rawStreamDelta `json:"delta"`
		FinishReason *string        `json:"finish_reason"`
	}
	type rawStreamChunk struct {
		ID      string            `json:"id"`
		Choices []rawStreamChoice `json:"choices"`
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk rawStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if streamID == "" && chunk.ID != "" {
			streamID = chunk.ID
		}

		for _, ch := range chunk.Choices {
			sc := &StreamChunk{
				ID:           chunk.ID,
				DeltaRole:    ch.Delta.Role,
				DeltaContent: ch.Delta.Content,
				DeltaTools:   ch.Delta.ToolCalls,
			}
			if ch.FinishReason != nil {
				sc.FinishReason = *ch.FinishReason
				finishReason = *ch.FinishReason
			}

			if ch.Delta.Role != "" {
				accumulatedRole = ch.Delta.Role
			}
			if ch.Delta.Content != "" {
				accumulatedContent.WriteString(ch.Delta.Content)
			}

			for _, tc := range ch.Delta.ToolCalls {
				existing, ok := toolCallsMap[tc.Index]
				if !ok {
					existing = &ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
					toolCallsMap[tc.Index] = existing
				} else {
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Function.Name += tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						existing.Function.Arguments += tc.Function.Arguments
					}
				}
			}

			if onChunk != nil {
				if err := onChunk(sc); err != nil {
					return nil, fmt.Errorf("agent: onChunk error: %w", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("agent: reading stream: %w", err)
	}

	keys := make([]int, 0, len(toolCallsMap))
	for k := range toolCallsMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var toolCalls []ToolCall
	for _, k := range keys {
		toolCalls = append(toolCalls, *toolCallsMap[k])
	}

	if accumulatedRole == "" {
		accumulatedRole = "assistant"
	}

	return &ChatResponse{
		ID: streamID,
		Choices: []Choice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:      accumulatedRole,
					Content:   accumulatedContent.String(),
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
	}, nil
}

// retry executes op up to 2 retries with exponential backoff if retryable is true.
func (c *Client) retry(ctx context.Context, op func() (retryable bool, err error)) error {
	backoff := 100 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		retryable, err := op()
		if err == nil {
			return nil
		}
		if !retryable || attempt == 2 {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return errors.New("agent: max retries exceeded")
}

// ProbeResult captures the outcome of an LLM connectivity check.
type ProbeResult struct {
	OK        bool          `json:"ok"`
	Status    string        `json:"status"` // "connected" | "unreachable" | "auth_error" | "missing_api_key" | "disabled" | "error"
	Latency   time.Duration `json:"latency"`
	LatencyMs int64         `json:"latency_ms"`
	Model     string        `json:"model"`
	Endpoint  string        `json:"endpoint"`
	Message   string        `json:"message"`
}

// ProbeEndpoint tests connectivity to the configured LLM endpoint without incurring generation tokens.
// It issues a lightweight GET request to the /models endpoint.
func ProbeEndpoint(ctx context.Context, cfg config.LLMConfig, httpClient *http.Client) ProbeResult {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434/v1"
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "default"
	}

	if cfg.Backend == "disabled" {
		return ProbeResult{
			OK:       false,
			Status:   "disabled",
			Model:    model,
			Endpoint: endpoint,
			Message:  "AI agent LLM is disabled in configuration",
		}
	}

	apiKey, fromEnv := config.ResolveAPIKey(cfg.APIKey)
	if cfg.APIKey != "" && apiKey == "" {
		msg := fmt.Sprintf("API key or environment variable %q is not set or empty (set it via 'centmem config set llm.api_key <key>' or export %s)", cfg.APIKey, cfg.APIKey)
		if !fromEnv {
			msg = "API key is empty (set it via 'centmem config set llm.api_key <key>' or leave blank for Ollama)"
		}
		return ProbeResult{
			OK:       false,
			Status:   "missing_api_key",
			Model:    model,
			Endpoint: endpoint,
			Message:  msg,
		}
	}

	probeURL := strings.TrimRight(endpoint, "/")
	probeURL = strings.TrimSuffix(probeURL, "/chat/completions")
	if !strings.HasSuffix(probeURL, "/models") {
		if !strings.HasSuffix(probeURL, "/v1") && !strings.HasSuffix(probeURL, "/openai") {
			probeURL += "/v1"
		}
		probeURL += "/models"
	}

	timeout := 5 * time.Second
	if cfg.TimeoutSeconds > 0 && cfg.TimeoutSeconds <= 10 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, probeURL, nil)
	if err != nil {
		return ProbeResult{
			OK:       false,
			Status:   "error",
			Model:    model,
			Endpoint: endpoint,
			Message:  fmt.Sprintf("Invalid probe URL %q: %v", probeURL, err),
		}
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	latencyMs := latency.Milliseconds()

	if err != nil {
		advice := "check network connectivity or endpoint URL"
		if strings.Contains(endpoint, "127.0.0.1") || strings.Contains(endpoint, "localhost") {
			advice = "ensure local model server is running (e.g. 'ollama serve') or update endpoint URL"
		} else if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") {
			advice = "endpoint probe timed out; check network connectivity or increase timeout"
		}
		return ProbeResult{
			OK:        false,
			Status:    "unreachable",
			Latency:   latency,
			LatencyMs: latencyMs,
			Model:     model,
			Endpoint:  endpoint,
			Message:   fmt.Sprintf("Failed to connect to LLM endpoint: %v (%s)", err, advice),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return ProbeResult{
			OK:        true,
			Status:    "connected",
			Latency:   latency,
			LatencyMs: latencyMs,
			Model:     model,
			Endpoint:  endpoint,
			Message:   "Connected to LLM endpoint successfully",
		}
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	bodyStr := strings.TrimSpace(string(bodyBytes))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		msg := fmt.Sprintf("Authentication failed (HTTP %d): verify your API key or environment variable", resp.StatusCode)
		if bodyStr != "" {
			msg += ": " + bodyStr
		}
		return ProbeResult{
			OK:        false,
			Status:    "auth_error",
			Latency:   latency,
			LatencyMs: latencyMs,
			Model:     model,
			Endpoint:  endpoint,
			Message:   msg,
		}
	}

	advice := "check endpoint URL configuration"
	if resp.StatusCode == http.StatusNotFound {
		advice = "verify endpoint supports OpenAI-compatible /models API"
	}
	msg := fmt.Sprintf("LLM endpoint returned HTTP %d (%s)", resp.StatusCode, advice)
	if bodyStr != "" {
		msg += ": " + bodyStr
	}
	return ProbeResult{
		OK:        false,
		Status:    "error",
		Latency:   latency,
		LatencyMs: latencyMs,
		Model:     model,
		Endpoint:  endpoint,
		Message:   msg,
	}
}

