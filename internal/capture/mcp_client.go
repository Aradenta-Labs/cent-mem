package capture

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MCPClient handles JSON-RPC 2.0 communication over stdio with an external MCP server.
type MCPClient struct {
	cmd         *exec.Cmd
	in          io.WriteCloser
	scanner     *bufio.Scanner
	mu          sync.Mutex
	reqID       int64
	initialized bool
	tools       []string
}

// mcpRequest represents a JSON-RPC 2.0 outgoing request.
type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// mcpResponse represents a JSON-RPC 2.0 incoming response.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ToolCallResultContent represents content items returned by tools/call.
type ToolCallResultContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ToolCallResultPayload represents the MCP tools/call result body.
type ToolCallResultPayload struct {
	Content []ToolCallResultContent `json:"content"`
	IsError bool                    `json:"isError,omitempty"`
}

// NewMCPClient spawns an external MCP server as a subprocess communicating over stdio.
func NewMCPClient(command string, args ...string) (*MCPClient, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start mcp process %s: %w", command, err)
	}

	scanner := bufio.NewScanner(stdout)
	const maxBuf = 2 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxBuf)

	return &MCPClient{
		cmd:     cmd,
		in:      stdin,
		scanner: scanner,
	}, nil
}

// NewMCPClientFromIO creates an MCPClient from existing reader and writer streams (useful for tests).
func NewMCPClientFromIO(in io.WriteCloser, out io.Reader) *MCPClient {
	scanner := bufio.NewScanner(out)
	const maxBuf = 2 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxBuf)

	return &MCPClient{
		in:      in,
		scanner: scanner,
	}
}

// Initialize performs the MCP protocol handshake (initialize request + notifications/initialized).
func (c *MCPClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.reqID, 1)
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name":    "centmem-classifier",
				"version": "1.5.4",
			},
		},
	}

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("mcp initialize rpc error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	// Send notifications/initialized (notification has no ID)
	notif := mcpRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	notifBytes, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal initialized notification: %w", err)
	}
	if _, err := c.in.Write(append(notifBytes, '\n')); err != nil {
		return fmt.Errorf("write initialized notification: %w", err)
	}

	c.initialized = true
	return nil
}

// ListTools queries the MCP server for its exposed tools.
func (c *MCPClient) ListTools(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.reqID, 1)
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/list",
	}

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcp tools/list rpc error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal tools/list result: %w", err)
	}

	var names []string
	for _, t := range result.Tools {
		names = append(names, t.Name)
	}
	c.tools = names
	return names, nil
}

// CallTool invokes an MCP tool by name with arguments and returns text content.
func (c *MCPClient) CallTool(ctx context.Context, tool string, args map[string]any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.reqID, 1)
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      tool,
			"arguments": args,
		},
	}

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return "", fmt.Errorf("mcp tools/call %s: %w", tool, err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("mcp tools/call rpc error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	var result ToolCallResultPayload
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("unmarshal tools/call result: %w", err)
	}

	if result.IsError {
		var errMsg strings.Builder
		for _, item := range result.Content {
			if item.Text != "" {
				errMsg.WriteString(item.Text)
			}
		}
		return "", fmt.Errorf("tool execution failed: %s", errMsg.String())
	}

	var sb strings.Builder
	for _, item := range result.Content {
		if item.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(item.Text)
		}
	}
	return sb.String(), nil
}

func (c *MCPClient) sendRequestLocked(ctx context.Context, req mcpRequest) (*mcpResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := c.in.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response line with context cancellation
	type scanResult struct {
		line []byte
		err  error
	}
	ch := make(chan scanResult, 1)

	go func() {
		if c.scanner.Scan() {
			line := c.scanner.Bytes()
			cpy := make([]byte, len(line))
			copy(cpy, line)
			ch <- scanResult{line: cpy}
		} else {
			if err := c.scanner.Err(); err != nil {
				ch <- scanResult{err: err}
			} else {
				ch <- scanResult{err: io.EOF}
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		var resp mcpResponse
		if err := json.Unmarshal(res.line, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response line %q: %w", string(res.line), err)
		}
		return &resp, nil
	}
}

// Close terminates the client and cleans up process resources.
func (c *MCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	if c.in != nil {
		if err := c.in.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// MCPEnricher coordinates external MCP servers to enrich capture context.
type MCPEnricher struct {
	cfg     CaptureMCPConfig
	clients []*MCPClient
	mu      sync.Mutex
}

// NewMCPEnricher creates an enricher configured with external servers.
func NewMCPEnricher(cfg CaptureMCPConfig) *MCPEnricher {
	return &MCPEnricher{
		cfg: cfg,
	}
}

// AddClient attaches an already instantiated MCPClient to the enricher.
func (e *MCPEnricher) AddClient(client *MCPClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clients = append(e.clients, client)
}

// Start launches and initializes all configured external MCP servers.
func (e *MCPEnricher) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, srv := range e.cfg.Servers {
		if srv.Command == "" {
			continue
		}
		client, err := NewMCPClient(srv.Command, srv.Args...)
		if err != nil {
			return fmt.Errorf("start mcp server %s: %w", srv.Name, err)
		}

		initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = client.Initialize(initCtx)
		cancel()
		if err != nil {
			_ = client.Close()
			return fmt.Errorf("initialize mcp server %s: %w", srv.Name, err)
		}

		e.clients = append(e.clients, client)
	}
	return nil
}

// Enrich invokes configured tools across available MCP clients to gather external context.
func (e *MCPEnricher) Enrich(ctx context.Context, query string) (string, error) {
	e.mu.Lock()
	clients := append([]*MCPClient(nil), e.clients...)
	tools := append([]string(nil), e.cfg.Tools...)
	e.mu.Unlock()

	if len(clients) == 0 || len(tools) == 0 || strings.TrimSpace(query) == "" {
		return "", nil
	}

	var results []string
	for _, client := range clients {
		for _, tool := range tools {
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			content, err := client.CallTool(callCtx, tool, map[string]any{
				"query": query,
				"url":   query,
			})
			cancel()
			if err == nil && strings.TrimSpace(content) != "" {
				results = append(results, fmt.Sprintf("[%s]: %s", tool, strings.TrimSpace(content)))
			}
		}
	}

	if len(results) == 0 {
		return "", nil
	}
	return strings.Join(results, "\n\n"), nil
}

// Close terminates all running MCP clients.
func (e *MCPEnricher) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var firstErr error
	for _, c := range e.clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	e.clients = nil
	return firstErr
}

// EnrichClassifierContext augments candidate message context if MCP enrichment is enabled.
func EnrichClassifierContext(ctx context.Context, cfg CaptureConfig, query string) string {
	if !cfg.MCP.Enabled {
		return ""
	}
	enricher := NewMCPEnricher(cfg.MCP)
	if err := enricher.Start(ctx); err != nil {
		return ""
	}
	defer enricher.Close()

	res, err := enricher.Enrich(ctx, query)
	if err != nil {
		return ""
	}
	return res
}

// ErrMCPClosed indicates the MCP client is already closed.
var ErrMCPClosed = errors.New("mcp client closed")
