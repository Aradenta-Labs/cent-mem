package capture

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// mockMCPServer runs a minimal JSON-RPC 2.0 loop responding to initialize, tools/list, and tools/call.
func runMockMCPServer(r io.Reader, w io.WriteCloser) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req mcpRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]string{"name": "mock-mcp-server", "version": "1.0.0"},
				},
			}
			bytes, _ := json.Marshal(resp)
			_, _ = w.Write(append(bytes, '\n'))

		case "notifications/initialized":
			// No response needed for notification

		case "tools/list":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{"name": "search_docs", "description": "Search repository documentation"},
						{"name": "fetch_url", "description": "Fetch webpage content"},
					},
				},
			}
			bytes, _ := json.Marshal(resp)
			_, _ = w.Write(append(bytes, '\n'))

		case "tools/call":
			params, _ := req.Params.(map[string]any)
			toolName, _ := params["name"].(string)
			args, _ := params["arguments"].(map[string]any)

			if toolName == "error_tool" {
				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": ToolCallResultPayload{
						IsError: true,
						Content: []ToolCallResultContent{
							{Type: "text", Text: "simulated tool failure"},
						},
					},
				}
				bytes, _ := json.Marshal(resp)
				_, _ = w.Write(append(bytes, '\n'))
				continue
			}

			q := ""
			if args != nil {
				if v, ok := args["query"].(string); ok {
					q = v
				}
			}

			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": ToolCallResultPayload{
					IsError: false,
					Content: []ToolCallResultContent{
						{Type: "text", Text: fmt.Sprintf("Summary of documentation for %q: architecture decision: we chose sqlite-vec.", q)},
					},
				},
			}
			bytes, _ := json.Marshal(resp)
			_, _ = w.Write(append(bytes, '\n'))
		}
	}
}

func TestMCPClient_HandshakeAndCall(t *testing.T) {
	// Client -> Server pipe
	c2sR, c2sW := io.Pipe()
	// Server -> Client pipe
	s2cR, s2cW := io.Pipe()

	go runMockMCPServer(c2sR, s2cW)

	client := NewMCPClientFromIO(c2sW, s2cR)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Initialize
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 2. List tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 || tools[0] != "search_docs" {
		t.Fatalf("unexpected tools list: %v", tools)
	}

	// 3. Call search_docs tool
	out, err := client.CallTool(ctx, "search_docs", map[string]any{"query": "sqlite vector index"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if !strings.Contains(out, "Summary of documentation for \"sqlite vector index\"") {
		t.Errorf("unexpected tool output: %q", out)
	}

	// 4. Call failing tool
	_, err = client.CallTool(ctx, "error_tool", map[string]any{})
	if err == nil {
		t.Fatal("expected error from error_tool, got nil")
	}
	if !strings.Contains(err.Error(), "simulated tool failure") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMCPEnricher_Enrich(t *testing.T) {
	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()

	go runMockMCPServer(c2sR, s2cW)

	client := NewMCPClientFromIO(c2sW, s2cR)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("client initialize failed: %v", err)
	}

	enricher := NewMCPEnricher(CaptureMCPConfig{
		Enabled: true,
		Tools:   []string{"search_docs"},
	})
	enricher.AddClient(client)

	summary, err := enricher.Enrich(ctx, "auth middleware")
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if !strings.Contains(summary, "[search_docs]:") {
		t.Errorf("expected [search_docs] prefix in enrichment, got: %q", summary)
	}
	if !strings.Contains(summary, "auth middleware") {
		t.Errorf("expected query in enrichment, got: %q", summary)
	}
}

func TestEnrichClassifierContext_Disabled(t *testing.T) {
	cfg := CaptureConfig{
		MCP: CaptureMCPConfig{Enabled: false},
	}
	out := EnrichClassifierContext(context.Background(), cfg, "anything")
	if out != "" {
		t.Fatalf("expected empty string when disabled, got %q", out)
	}
}

func TestHelperMCPServerProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	runMockMCPServer(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestClassifyWithConfig_MCPEnrichment(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	cfg := DefaultCaptureConfig()
	cfg.MCP = CaptureMCPConfig{
		Enabled: true,
		Servers: []MCPServerConfig{
			{
				Name:    "mock-docs",
				Command: os.Args[0],
				Args:    []string{"-test.run=TestHelperMCPServerProcess", "--"},
			},
		},
		Tools: []string{"search_docs"},
	}

	messages := []TranscriptMessage{
		{
			Role:    "user",
			Content: "What did we decide about sqlite vector index?",
		},
	}

	items, err := ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed with MCP: %v", err)
	}

	foundEnrichment := false
	for _, item := range items {
		if strings.Contains(item.Content, "Summary of documentation for") || strings.Contains(item.Content, "sqlite vector index") {
			foundEnrichment = true
			break
		}
	}
	if !foundEnrichment {
		t.Errorf("expected MCP-enriched item in classified results, got items: %+v", items)
	}
}

