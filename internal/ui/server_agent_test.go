package ui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func setupTestStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	return st, func() { _ = st.Close() }
}

func TestServer_Proposals_CRUD_And_Apply(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	// Seed memories for merge proposal
	m1ID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:myapp",
		Type:    "note",
		Content: "Source memory 1 about auth architecture",
	})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	m2ID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:myapp",
		Type:    "note",
		Content: "Source memory 2 about auth token verification",
	})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}

	// Create a link proposal and a merge proposal
	mergePayload, _ := json.Marshal(store.MergeProposalPayload{
		SourceIDs:     []int64{m1ID, m2ID},
		TargetTitle:   "Consolidated Auth",
		TargetContent: "Consolidated authentication architecture and token verification",
		TargetTags:    []string{"auth", "security"},
	})
	mergeID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:myapp",
		ProposalType: "merge",
		Title:        "Merge Auth Notes",
		Reasoning:    "Duplicate auth knowledge found",
		PayloadJSON:  string(mergePayload),
	})
	if err != nil {
		t.Fatalf("create merge proposal: %v", err)
	}

	linkPayload, _ := json.Marshal(store.LinkProposalPayload{
		FromID:   m1ID,
		ToID:     m2ID,
		Relation: "supersedes",
	})
	_, err = st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:myapp",
		ProposalType: "link",
		Title:        "Link Auth Notes",
		Reasoning:    "Memory 1 supersedes Memory 2",
		PayloadJSON:  string(linkPayload),
	})
	if err != nil {
		t.Fatalf("create link proposal: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.httpServer.Handler

	// 1. GET /api/proposals (list all)
	req := httptest.NewRequest("GET", "/api/proposals", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/proposals status=%d, want 200", rec.Code)
	}
	var listResp struct {
		Ok        bool             `json:"ok"`
		Proposals []store.Proposal `json:"proposals"`
		Count     int              `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list proposals: %v", err)
	}
	if !listResp.Ok || listResp.Count != 2 {
		t.Fatalf("expected count 2, got ok=%v, count=%d", listResp.Ok, listResp.Count)
	}

	// 2. Filter by status=pending
	req = httptest.NewRequest("GET", "/api/proposals?status=pending", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var pendingResp struct {
		Ok        bool             `json:"ok"`
		Proposals []store.Proposal `json:"proposals"`
		Count     int              `json:"count"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&pendingResp)
	if pendingResp.Count != 2 {
		t.Errorf("expected 2 pending proposals, got %d", pendingResp.Count)
	}

	// 3. POST /api/proposals/{id}/apply (apply merge proposal)
	applyURL := fmt.Sprintf("/api/proposals/%d/apply", mergeID)
	req = httptest.NewRequest("POST", applyURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST %s status=%d, body=%s", applyURL, rec.Code, rec.Body.String())
	}
	var applyResp struct {
		Ok       bool            `json:"ok"`
		Applied  bool            `json:"applied"`
		Proposal *store.Proposal `json:"proposal"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&applyResp); err != nil {
		t.Fatalf("decode apply resp: %v", err)
	}
	if !applyResp.Ok || !applyResp.Applied || applyResp.Proposal == nil || applyResp.Proposal.Status != "applied" {
		t.Fatalf("expected status=applied, got %+v", applyResp.Proposal)
	}

	// Verify underlying memories: sources should be marked 'summarized'
	archived1, _ := st.GetMemory(ctx, m1ID)
	if archived1.Status != "summarized" {
		t.Errorf("expected m1 to be summarized, got %s", archived1.Status)
	}
	archived2, _ := st.GetMemory(ctx, m2ID)
	if archived2.Status != "summarized" {
		t.Errorf("expected m2 to be summarized, got %s", archived2.Status)
	}

	// 4. Double apply -> HTTP 409 Conflict
	req = httptest.NewRequest("POST", applyURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict on double-apply, got %d", rec.Code)
	}

	// 5. Apply non-existent proposal -> HTTP 404
	req = httptest.NewRequest("POST", "/api/proposals/999999/apply", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent proposal, got %d", rec.Code)
	}
}

func TestServer_Proposals_Dismiss(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	pID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:myapp",
		ProposalType: "archive",
		Title:        "Archive Stale Memory",
		Reasoning:    "Memory is 1 year old",
		PayloadJSON:  `{"memory_ids":[1]}`,
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.httpServer.Handler

	// Dismiss proposal
	dismissURL := fmt.Sprintf("/api/proposals/%d/dismiss", pID)
	req := httptest.NewRequest("POST", dismissURL, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST %s status=%d, body=%s", dismissURL, rec.Code, rec.Body.String())
	}
	var dismissResp struct {
		Ok        bool            `json:"ok"`
		Dismissed bool            `json:"dismissed"`
		Proposal  *store.Proposal `json:"proposal"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&dismissResp); err != nil {
		t.Fatalf("decode dismiss resp: %v", err)
	}
	if !dismissResp.Ok || !dismissResp.Dismissed || dismissResp.Proposal.Status != "dismissed" {
		t.Fatalf("expected status=dismissed, got %+v", dismissResp.Proposal)
	}

	// Dismiss non-existent ID -> 404
	req = httptest.NewRequest("POST", "/api/proposals/999999/dismiss", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec.Code)
	}
}

func TestServer_Agent_Conversations_And_Messages(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	conv1 := &store.Conversation{
		ScopePath: "project:alpha",
		Title:     "Architecture Q&A",
	}
	if err := st.CreateConversation(ctx, conv1); err != nil {
		t.Fatalf("create conv1: %v", err)
	}
	conv2 := &store.Conversation{
		ScopePath: "project:beta",
		Title:     "Deployment Notes",
	}
	if err := st.CreateConversation(ctx, conv2); err != nil {
		t.Fatalf("create conv2: %v", err)
	}

	// Add messages to conv1
	_, err := st.AppendMessage(ctx, &store.Message{
		ConversationID: conv1.ID,
		Role:           "user",
		Content:        "What database does centmem use?",
	})
	if err != nil {
		t.Fatalf("append message 1: %v", err)
	}
	_, err = st.AppendMessage(ctx, &store.Message{
		ConversationID: conv1.ID,
		Role:           "assistant",
		Content:        "centmem uses SQLite with WAL mode.",
	})
	if err != nil {
		t.Fatalf("append message 2: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.httpServer.Handler

	// 1. GET /api/agent/conversations (all)
	req := httptest.NewRequest("GET", "/api/agent/conversations", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agent/conversations status=%d", rec.Code)
	}
	var convsResp struct {
		Ok            bool                 `json:"ok"`
		Conversations []store.Conversation `json:"conversations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&convsResp); err != nil {
		t.Fatalf("decode convs: %v", err)
	}
	if !convsResp.Ok || len(convsResp.Conversations) != 2 {
		t.Fatalf("expected 2 conversations, got ok=%v, count=%d", convsResp.Ok, len(convsResp.Conversations))
	}

	// 2. GET /api/agent/conversations?scope=project:alpha
	req = httptest.NewRequest("GET", "/api/agent/conversations?scope=project:alpha", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var alphaResp struct {
		Ok            bool                 `json:"ok"`
		Conversations []store.Conversation `json:"conversations"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&alphaResp)
	if len(alphaResp.Conversations) != 1 || alphaResp.Conversations[0].ID != conv1.ID {
		t.Fatalf("expected conv1 in alpha scope, got %+v", alphaResp.Conversations)
	}

	// 3. GET /api/agent/conversations/{id}/messages
	msgURL := fmt.Sprintf("/api/agent/conversations/%s/messages", conv1.ID)
	req = httptest.NewRequest("GET", msgURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d", msgURL, rec.Code)
	}
	var msgResp struct {
		Ok       bool            `json:"ok"`
		Messages []store.Message `json:"messages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&msgResp); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	if !msgResp.Ok || len(msgResp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got ok=%v, len=%d", msgResp.Ok, len(msgResp.Messages))
	}
	if msgResp.Messages[0].Role != "user" || msgResp.Messages[1].Role != "assistant" {
		t.Errorf("unexpected message roles: %+v", msgResp.Messages)
	}

	// 4. GET messages for non-existent conversation -> 404
	req = httptest.NewRequest("GET", "/api/agent/conversations/non-existent-conv/messages", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent conversation, got %d", rec.Code)
	}
}

func TestServer_Agent_Chat_SSE_Stream(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	_, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:myapp",
		Type:    "note",
		Content: "centmem is a local-first memory store for AI agents",
	})
	if err != nil {
		t.Fatalf("put memory: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Valid request to POST /api/agent/chat
	chatURL := srv.URL() + "/api/agent/chat"
	bodyBytes, _ := json.Marshal(map[string]any{
		"message": "What is centmem?",
		"scope":   "project:myapp",
	})
	resp, err := client.Post(chatURL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("POST /api/agent/chat error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/agent/chat status=%d, want 200", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("expected text/event-stream Content-Type, got %q", contentType)
	}

	// Read and parse SSE events
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	eventsReceived := make(map[string][]string)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			eventsReceived[currentEvent] = append(eventsReceived[currentEvent], data)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		t.Fatalf("reading stream error: %v", err)
	}

	if _, ok := eventsReceived["delta"]; !ok {
		t.Errorf("expected 'delta' events, received: %+v", eventsReceived)
	}
	if _, ok := eventsReceived["done"]; !ok {
		t.Errorf("expected 'done' event, received: %+v", eventsReceived)
	}

	// 2. Empty message -> 400 Bad Request
	emptyBody, _ := json.Marshal(map[string]any{
		"message": "   ",
	})
	respErr, err := client.Post(chatURL, "application/json", bytes.NewReader(emptyBody))
	if err != nil {
		t.Fatalf("empty msg request: %v", err)
	}
	defer respErr.Body.Close()
	if respErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty message, got %d", respErr.StatusCode)
	}
}

func TestServer_Agent_Auth(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	validToken := "sec_test_secret_token_12345"
	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
		Token:   validToken,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.httpServer.Handler

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/proposals", ""},
		{"POST", "/api/proposals/1/apply", ""},
		{"POST", "/api/proposals/1/dismiss", ""},
		{"POST", "/api/proposals/1/reopen", ""},
		{"GET", "/api/agent/conversations", ""},
		{"GET", "/api/agent/conversations/conv-123/messages", ""},
		{"POST", "/api/agent/chat", `{"message":"hello"}`},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			// 1. Missing token -> 401
			var bodyReader io.Reader
			if ep.body != "" {
				bodyReader = strings.NewReader(ep.body)
			}
			req := httptest.NewRequest(ep.method, ep.path, bodyReader)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("missing token: expected 401, got %d", rec.Code)
			}

			// 2. Invalid token -> 401
			if ep.body != "" {
				bodyReader = strings.NewReader(ep.body)
			}
			req = httptest.NewRequest(ep.method, ep.path, bodyReader)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Authorization", "Bearer bad-token-here")
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("invalid token: expected 401, got %d", rec.Code)
			}

			// 3. Valid token -> Not 401 (either 200, 404, or other valid handler response)
			if ep.body != "" {
				bodyReader = strings.NewReader(ep.body)
			}
			req = httptest.NewRequest(ep.method, ep.path, bodyReader)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Authorization", "Bearer "+validToken)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized {
				t.Errorf("valid token: unexpected 401 Unauthorized for %s %s", ep.method, ep.path)
			}
		})
	}
}

func TestServer_Proposals_Reopen(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	pID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:myapp",
		ProposalType: "archive",
		Title:        "Archive Stale Memory",
		Reasoning:    "Memory is 1 year old",
		PayloadJSON:  `{"target_id":1}`,
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.httpServer.Handler

	// Dismiss first
	dismissURL := fmt.Sprintf("/api/proposals/%d/dismiss", pID)
	req := httptest.NewRequest("POST", dismissURL, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dismiss failed: status=%d", rec.Code)
	}

	// 1. Reopen dismissed proposal -> 200 OK with status="pending"
	reopenURL := fmt.Sprintf("/api/proposals/%d/reopen", pID)
	req = httptest.NewRequest("POST", reopenURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}
	var reopenResp struct {
		Ok       bool            `json:"ok"`
		Reopened bool            `json:"reopened"`
		Proposal *store.Proposal `json:"proposal"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&reopenResp); err != nil {
		t.Fatalf("decode reopen resp: %v", err)
	}
	if !reopenResp.Ok || !reopenResp.Reopened || reopenResp.Proposal.Status != "pending" {
		t.Fatalf("expected status=pending, got %+v", reopenResp.Proposal)
	}

	// 2. Reopen non-existent proposal -> 404
	req = httptest.NewRequest("POST", "/api/proposals/999999/reopen", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent proposal, got %d", rec.Code)
	}
}

func TestServer_Proposals_ScopeGlobalFilter(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:project-x",
		ProposalType: "archive",
		Title:        "Project X Proposal",
		Reasoning:    "Reason",
		PayloadJSON:  `{"target_id":1}`,
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := srv.httpServer.Handler

	// GET /api/proposals?scope=global should return proposals from project-x too
	req := httptest.NewRequest("GET", "/api/proposals?scope=global", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/proposals?scope=global status=%d", rec.Code)
	}
	var resp struct {
		Ok        bool             `json:"ok"`
		Proposals []store.Proposal `json:"proposals"`
		Count     int              `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("expected 1 proposal for scope=global (all scopes), got %d", resp.Count)
	}
}

func TestServer_Agent_Chat_Client_Abort(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	srv, err := NewServer(ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Start request with context cancelled immediately after starting
	ctx, cancel := context.WithCancel(context.Background())
	chatURL := srv.URL() + "/api/agent/chat"
	bodyBytes, _ := json.Marshal(map[string]any{
		"message": "Explain architecture",
		"scope":   "global",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Cancel context after short delay to simulate client disconnect mid-stream
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	// The important verification is no server panic or lockups on subsequent requests
	healthResp, err := http.Get(srv.URL() + "/api/health")
	if err != nil {
		t.Fatalf("health check after client abort failed: %v", err)
	}
	_ = healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("health check status=%d after client abort", healthResp.StatusCode)
	}
}
