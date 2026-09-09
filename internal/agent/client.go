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
		if !strings.HasSuffix(endpoint, "/v1") {
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
	req.Stream = false
	if req.Model == "" {
		req.Model = c.cfg.Model
	}
	if req.Temperature == nil && c.cfg.Temperature >= 0 {
		temp := c.cfg.Temperature
		req.Temperature = &temp
	}
	if req.MaxTokens == nil && c.cfg.MaxTokens > 0 {
		maxT := c.cfg.MaxTokens
		req.MaxTokens = &maxT
	}

	bodyBytes, err := json.Marshal(req)
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

		var chatResp ChatResponse
		if decodeErr := json.NewDecoder(httpResp.Body).Decode(&chatResp); decodeErr != nil {
			return false, fmt.Errorf("agent: decode response: %w", decodeErr)
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
	req.Stream = true
	if req.Model == "" {
		req.Model = c.cfg.Model
	}
	if req.Temperature == nil && c.cfg.Temperature >= 0 {
		temp := c.cfg.Temperature
		req.Temperature = &temp
	}
	if req.MaxTokens == nil && c.cfg.MaxTokens > 0 {
		maxT := c.cfg.MaxTokens
		req.MaxTokens = &maxT
	}

	bodyBytes, err := json.Marshal(req)
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

	scanner := bufio.NewScanner(httpResp.Body)
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

	var toolCalls []ToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if tc, ok := toolCallsMap[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
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
