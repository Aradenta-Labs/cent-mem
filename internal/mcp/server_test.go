package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/mcp"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func setupTestServer(t *testing.T, in io.Reader, out io.Writer) (*mcp.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		DBPath: filepath.Join(dir, "centmem.db"),
	}
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	searcher := search.New(st)
	srv := mcp.NewServer(st, searcher, cfg, in, out, "2.0.3")
	return srv, st
}

func runServerLines(t *testing.T, inputLines []string) []map[string]any {
	t.Helper()
	inStr := strings.Join(inputLines, "\n") + "\n"
	in := strings.NewReader(inStr)
	var out bytes.Buffer

	srv, _ := setupTestServer(t, in, &out)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		t.Fatalf("server run error: %v", err)
	}

	scanner := bufio.NewScanner(&out)
	var responses []map[string]any
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("failed to parse output line %q: %v", string(line), err)
		}
		responses = append(responses, m)
	}
	return responses
}

func TestServer_Initialize(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`
	resps := runServerLines(t, []string{req})

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	resp := resps[0]
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}
	if resp["id"] != float64(1) {
		t.Errorf("expected id 1, got %v", resp["id"])
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %v", resp["result"])
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}
	serverInfo, _ := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "centmem" {
		t.Errorf("expected server name centmem, got %v", serverInfo["name"])
	}
	if serverInfo["version"] != "2.0.3" {
		t.Errorf("expected server version 2.0.3, got %v", serverInfo["version"])
	}
}

func TestServer_NotificationInitialized(t *testing.T) {
	// Notification should produce 0 responses
	notif := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	resps := runServerLines(t, []string{notif})

	if len(resps) != 0 {
		t.Fatalf("expected 0 responses for notification, got %d", len(resps))
	}
}

func TestServer_Ping(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":"ping-1","method":"ping"}`
	resps := runServerLines(t, []string{req})

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	resp := resps[0]
	if resp["id"] != "ping-1" {
		t.Errorf("expected id 'ping-1', got %v", resp["id"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	resps := runServerLines(t, []string{req})

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %v", resps[0]["result"])
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %v", result["tools"])
	}

	expectedTools := map[string]bool{
		"centmem_recall":   false,
		"centmem_put":      false,
		"centmem_set":      false,
		"centmem_get":      false,
		"centmem_timeline": false,
		"centmem_stats":    false,
		"centmem_forget":   false,
	}

	for _, toolRaw := range tools {
		toolMap, _ := toolRaw.(map[string]any)
		name, _ := toolMap["name"].(string)
		if _, exists := expectedTools[name]; exists {
			expectedTools[name] = true
		}
	}

	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found in tools/list", toolName)
		}
	}
}

func TestServer_ToolsCall_Workflow(t *testing.T) {
	lines := []string{
		// 1. Put note
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"centmem_put","arguments":{"content":"MCP server integration verified","scope":"project:cent-mem","type":"note","tags":["test","mcp"]}}}`,
		// 2. Set fact
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"centmem_set","arguments":{"scope":"project:cent-mem","key":"system.mcp_mode","value":"enabled","tags":["config"]}}}`,
		// 3. Get fact
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"centmem_get","arguments":{"scope":"project:cent-mem","key":"system.mcp_mode"}}}`,
		// 4. Get note by ID
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"centmem_get","arguments":{"id":1}}}`,
		// 5. Recall
		`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"centmem_recall","arguments":{"query":"MCP server integration","scope":"project:cent-mem","reranker":"composite"}}}`,
		// 6. Timeline
		`{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"centmem_timeline","arguments":{"scope":"project:cent-mem"}}}`,
		// 7. Stats
		`{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"centmem_stats","arguments":{}}}`,
		// 8. Forget fact
		`{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"centmem_forget","arguments":{"scope":"project:cent-mem","key":"system.mcp_mode"}}}`,
	}

	resps := runServerLines(t, lines)
	if len(resps) != 8 {
		t.Fatalf("expected 8 responses, got %d", len(resps))
	}

	// Verify Put response
	putResp := resps[0]
	putResult := putResp["result"].(map[string]any)
	putContent := putResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(putContent, `"ok":true`) || !strings.Contains(putContent, `"id":`) {
		t.Errorf("unexpected put content: %s", putContent)
	}

	// Verify Set response
	setResp := resps[1]
	setResult := setResp["result"].(map[string]any)
	setContent := setResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(setContent, `"ok":true`) || !strings.Contains(setContent, `"key":"system.mcp_mode"`) {
		t.Errorf("unexpected set content: %s", setContent)
	}

	// Verify Get fact response
	getResp := resps[2]
	getResult := getResp["result"].(map[string]any)
	getContent := getResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(getContent, `"ok":true`) || !strings.Contains(getContent, `"value":"enabled"`) {
		t.Errorf("unexpected get content: %s", getContent)
	}

	// Verify Get by ID response (snake_case fields)
	getByIDResp := resps[3]
	getByIDResult := getByIDResp["result"].(map[string]any)
	getByIDContent := getByIDResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(getByIDContent, `"ok":true`) || !strings.Contains(getByIDContent, `"id":1`) || !strings.Contains(getByIDContent, `"content":"MCP server integration verified"`) {
		t.Errorf("unexpected get by id content: %s", getByIDContent)
	}
	if strings.Contains(getByIDContent, `"ID":`) || strings.Contains(getByIDContent, `"ScopePath":`) {
		t.Errorf("expected snake_case keys in memory object, got PascalCase: %s", getByIDContent)
	}

	// Verify Recall response
	recallResp := resps[4]
	recallResult := recallResp["result"].(map[string]any)
	recallContent := recallResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(recallContent, `"ok":true`) || !strings.Contains(recallContent, `MCP server integration verified`) {
		t.Errorf("unexpected recall content: %s", recallContent)
	}

	// Verify Timeline response
	tlResp := resps[5]
	tlResult := tlResp["result"].(map[string]any)
	tlContent := tlResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(tlContent, `"ok":true`) || !strings.Contains(tlContent, `"entries":`) || !strings.Contains(tlContent, `MCP server integration verified`) {
		t.Errorf("unexpected timeline content: %s", tlContent)
	}

	// Verify Stats response
	statsResp := resps[6]
	statsResult := statsResp["result"].(map[string]any)
	statsContent := statsResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(statsContent, `"ok":true`) || !strings.Contains(statsContent, `"memories":`) || !strings.Contains(statsContent, `"db_path":`) {
		t.Errorf("unexpected stats content: %s", statsContent)
	}

	// Verify Forget response
	forgetResp := resps[7]
	forgetResult := forgetResp["result"].(map[string]any)
	forgetContent := forgetResult["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(forgetContent, `"ok":true`) || !strings.Contains(forgetContent, `"deleted":1`) {
		t.Errorf("unexpected forget content: %s", forgetContent)
	}
}

func TestServer_ErrorHandling(t *testing.T) {
	lines := []string{
		`{invalid json`,
		`{"jsonrpc":"1.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":2,"method":"unknown_method"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{}}`,
	}

	resps := runServerLines(t, lines)
	if len(resps) != 4 {
		t.Fatalf("expected 4 error responses, got %d", len(resps))
	}

	// 1. Parse error (-32700)
	err1 := resps[0]["error"].(map[string]any)
	if err1["code"] != float64(-32700) {
		t.Errorf("expected code -32700, got %v", err1["code"])
	}

	// 2. Invalid Request (-32600)
	err2 := resps[1]["error"].(map[string]any)
	if err2["code"] != float64(-32600) {
		t.Errorf("expected code -32600, got %v", err2["code"])
	}

	// 3. Method not found (-32601)
	err3 := resps[2]["error"].(map[string]any)
	if err3["code"] != float64(-32601) {
		t.Errorf("expected code -32601, got %v", err3["code"])
	}

	// 4. Invalid params (-32602)
	err4 := resps[3]["error"].(map[string]any)
	if err4["code"] != float64(-32602) {
		t.Errorf("expected code -32602, got %v", err4["code"])
	}
}
