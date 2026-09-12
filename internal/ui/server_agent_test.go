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
	"os"
	"path/filepath"
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

func TestServer_Proposals_Batch(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Seed memories
	m1ID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:batch",
		Type:    "note",
		Content: "Batch memory 1",
	})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	m2ID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:batch",
		Type:    "note",
		Content: "Batch memory 2",
	})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}
	m3ID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:batch",
		Type:    "note",
		Content: "Batch memory 3",
	})
	if err != nil {
		t.Fatalf("put m3: %v", err)
	}
	m4ID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:batch",
		Type:    "note",
		Content: "Batch memory 4",
	})
	if err != nil {
		t.Fatalf("put m4: %v", err)
	}

	// Create 2 proposals for batch apply
	linkPayload1, _ := json.Marshal(store.LinkProposalPayload{
		FromID:   m1ID,
		ToID:     m2ID,
		Relation: "supersedes",
	})
	p1ID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:batch",
		ProposalType: "link",
		Title:        "Link 1 and 2",
		PayloadJSON:  string(linkPayload1),
	})
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}

	linkPayload2, _ := json.Marshal(store.LinkProposalPayload{
		FromID:   m3ID,
		ToID:     m4ID,
		Relation: "supersedes",
	})
	p2ID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:batch",
		ProposalType: "link",
		Title:        "Link 3 and 4",
		PayloadJSON:  string(linkPayload2),
	})
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	// Create 2 proposals for batch dismiss
	p3ID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:batch",
		ProposalType: "archive",
		Title:        "Archive p3",
		PayloadJSON:  `{"target_id":1}`,
	})
	if err != nil {
		t.Fatalf("create p3: %v", err)
	}
	p4ID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:batch",
		ProposalType: "archive",
		Title:        "Archive p4",
		PayloadJSON:  `{"target_id":2}`,
	})
	if err != nil {
		t.Fatalf("create p4: %v", err)
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

	// 1. Batch Apply p1 and p2
	applyBody, _ := json.Marshal(map[string]any{
		"action": "apply",
		"ids":    []int64{p1ID, p2ID},
	})
	req := httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(applyBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("batch apply status=%d, body=%s", rec.Code, rec.Body.String())
	}
	var batchApplyResp struct {
		Ok        bool     `json:"ok"`
		Action    string   `json:"action"`
		Total     int      `json:"total"`
		Succeeded []int64  `json:"succeeded"`
		Failed    []struct {
			ID    int64  `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&batchApplyResp); err != nil {
		t.Fatalf("decode batch apply: %v", err)
	}
	if !batchApplyResp.Ok || batchApplyResp.Action != "apply" || batchApplyResp.Total != 2 {
		t.Errorf("unexpected batch apply response: %+v", batchApplyResp)
	}
	if len(batchApplyResp.Succeeded) != 2 || len(batchApplyResp.Failed) != 0 {
		t.Errorf("expected 2 succeeded, 0 failed, got %+v", batchApplyResp)
	}

	p1After, _ := st.GetProposal(ctx, p1ID)
	p2After, _ := st.GetProposal(ctx, p2ID)
	if p1After.Status != "applied" || p2After.Status != "applied" {
		t.Errorf("expected p1 & p2 status 'applied', got p1=%s, p2=%s", p1After.Status, p2After.Status)
	}

	// 2. Batch Dismiss p3 and p4
	dismissBody, _ := json.Marshal(map[string]any{
		"action": "dismiss",
		"ids":    []int64{p3ID, p4ID},
	})
	req = httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(dismissBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("batch dismiss status=%d, body=%s", rec.Code, rec.Body.String())
	}
	var batchDismissResp struct {
		Ok        bool     `json:"ok"`
		Action    string   `json:"action"`
		Total     int      `json:"total"`
		Succeeded []int64  `json:"succeeded"`
		Failed    []struct {
			ID    int64  `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&batchDismissResp); err != nil {
		t.Fatalf("decode batch dismiss: %v", err)
	}
	if !batchDismissResp.Ok || batchDismissResp.Action != "dismiss" || batchDismissResp.Total != 2 {
		t.Errorf("unexpected batch dismiss response: %+v", batchDismissResp)
	}
	if len(batchDismissResp.Succeeded) != 2 || len(batchDismissResp.Failed) != 0 {
		t.Errorf("expected 2 succeeded, 0 failed, got %+v", batchDismissResp)
	}

	p3After, _ := st.GetProposal(ctx, p3ID)
	p4After, _ := st.GetProposal(ctx, p4ID)
	if p3After.Status != "dismissed" || p4After.Status != "dismissed" {
		t.Errorf("expected p3 & p4 status 'dismissed', got p3=%s, p4=%s", p3After.Status, p4After.Status)
	}

	// 3. Partial Failure: valid ID + non-existent ID + negative ID
	p5ID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:batch",
		ProposalType: "archive",
		Title:        "Archive p5",
		PayloadJSON:  `{"target_id":3}`,
	})
	if err != nil {
		t.Fatalf("create p5: %v", err)
	}

	partialBody, _ := json.Marshal(map[string]any{
		"action": "dismiss",
		"ids":    []int64{p5ID, 999999, -10},
	})
	req = httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(partialBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("partial failure batch status=%d, body=%s", rec.Code, rec.Body.String())
	}
	var partialResp struct {
		Ok        bool     `json:"ok"`
		Total     int      `json:"total"`
		Succeeded []int64  `json:"succeeded"`
		Failed    []struct {
			ID    int64  `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&partialResp); err != nil {
		t.Fatalf("decode partial response: %v", err)
	}
	if len(partialResp.Succeeded) != 1 || partialResp.Succeeded[0] != p5ID {
		t.Errorf("expected succeeded=[%d], got %+v", p5ID, partialResp.Succeeded)
	}
	if len(partialResp.Failed) != 2 {
		t.Errorf("expected 2 failed items, got %+v", partialResp.Failed)
	}

	// 4. Validation: Empty IDs
	emptyBody, _ := json.Marshal(map[string]any{
		"action": "apply",
		"ids":    []int64{},
	})
	req = httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(emptyBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty IDs, got %d", rec.Code)
	}

	// 5. Validation: Invalid Action
	badActionBody, _ := json.Marshal(map[string]any{
		"action": "invalid_action",
		"ids":    []int64{1},
	})
	req = httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(badActionBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid action, got %d", rec.Code)
	}

	// 6. Validation: Exceeding batch cap (>500 IDs)
	tooManyIDs := make([]int64, 501)
	for i := range tooManyIDs {
		tooManyIDs[i] = int64(i + 1)
	}
	tooManyBody, _ := json.Marshal(map[string]any{
		"action": "apply",
		"ids":    tooManyIDs,
	})
	req = httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(tooManyBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for >500 IDs, got %d", rec.Code)
	}

	// 7. Conflict & Idempotency: re-applying p1 (already applied) alongside a newly created p6
	p6ID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:batch",
		ProposalType: "archive",
		Title:        "Archive p6",
		PayloadJSON:  `{"target_id":4}`,
	})
	if err != nil {
		t.Fatalf("create p6: %v", err)
	}
	conflictBody, _ := json.Marshal(map[string]any{
		"action": "apply",
		"ids":    []int64{p1ID, p6ID},
	})
	req = httptest.NewRequest("POST", "/api/proposals/batch", bytes.NewReader(conflictBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for partial conflict apply, got %d", rec.Code)
	}
	var conflictResp struct {
		Ok        bool     `json:"ok"`
		Action    string   `json:"action"`
		Total     int      `json:"total"`
		Succeeded []int64  `json:"succeeded"`
		Failed    []struct {
			ID    int64  `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&conflictResp); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if len(conflictResp.Succeeded) != 1 || conflictResp.Succeeded[0] != p6ID {
		t.Errorf("expected succeeded=[%d], got %+v", p6ID, conflictResp.Succeeded)
	}
	if len(conflictResp.Failed) != 1 || conflictResp.Failed[0].ID != p1ID {
		t.Errorf("expected failed=[%d], got %+v", p1ID, conflictResp.Failed)
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

func TestServer_TestAgent(t *testing.T) {
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
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("success_connected", func(t *testing.T) {
		mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/models") {
				t.Errorf("unexpected probe request: %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer secret-123" {
				t.Errorf("expected Bearer secret-123, got %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"qwen-2.5"}]}`))
		}))
		defer mockLLM.Close()

		payload, _ := json.Marshal(map[string]any{
			"endpoint": mockLLM.URL,
			"model":    "qwen-2.5",
			"api_key":  "secret-123",
		})
		resp, err := client.Post(srv.URL()+"/api/config/test-agent", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("POST /api/config/test-agent: %v", err)
		}
		defer resp.Body.Close()

		var res struct {
			Ok        bool   `json:"ok"`
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms"`
			Model     string `json:"model"`
			Endpoint  string `json:"endpoint"`
			Message   string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if !res.Ok {
			t.Errorf("expected ok=true, got false: %s", res.Message)
		}
		if res.Status != "connected" {
			t.Errorf("expected status=connected, got %s", res.Status)
		}
		if res.Model != "qwen-2.5" {
			t.Errorf("expected model qwen-2.5, got %s", res.Model)
		}
	})

	t.Run("auth_error", func(t *testing.T) {
		mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`Invalid token`))
		}))
		defer mockLLM.Close()

		payload, _ := json.Marshal(map[string]any{
			"endpoint": mockLLM.URL,
			"model":    "qwen-2.5",
			"api_key":  "wrong-key",
		})
		resp, err := client.Post(srv.URL()+"/api/config/test-agent", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("POST /api/config/test-agent: %v", err)
		}
		defer resp.Body.Close()

		var res struct {
			Ok     bool   `json:"ok"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if res.Ok {
			t.Errorf("expected ok=false on auth error, got true")
		}
		if res.Status != "auth_error" {
			t.Errorf("expected status=auth_error, got %s", res.Status)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]any{
			"endpoint": "http://127.0.0.1:59998/v1",
			"model":    "qwen-2.5",
		})
		resp, err := client.Post(srv.URL()+"/api/config/test-agent", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("POST /api/config/test-agent: %v", err)
		}
		defer resp.Body.Close()

		var res struct {
			Ok     bool   `json:"ok"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if res.Ok {
			t.Errorf("expected ok=false on unreachable, got true")
		}
		if res.Status != "unreachable" {
			t.Errorf("expected status=unreachable, got %s", res.Status)
		}
	})
}

func TestServer_Health_AIAgentCheck(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. When agent is disabled in config: status is ok, detail "disabled in config"
	cfgDisabled := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
		Config: config.Config{
			Agent: config.AgentConfig{Enabled: false},
			LLM:   config.LLMConfig{Backend: "disabled"},
		},
	}
	srvDisabled, err := NewServer(cfgDisabled)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srvDisabled.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srvDisabled.Shutdown(shutCtx)
	}()

	resp, err := http.Get(srvDisabled.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	var healthRes struct {
		Ok     bool          `json:"ok"`
		Status string        `json:"status"`
		Checks []DoctorCheck `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&healthRes); err != nil {
		t.Fatalf("decode health: %v", err)
	}

	var aiCheck *DoctorCheck
	for i := range healthRes.Checks {
		if healthRes.Checks[i].Name == "ai_agent" {
			aiCheck = &healthRes.Checks[i]
			break
		}
	}
	if aiCheck == nil {
		t.Fatalf("expected ai_agent check in health response")
	}
	if aiCheck.Status != "ok" {
		t.Errorf("expected ai_agent status ok when disabled, got %s", aiCheck.Status)
	}

	// 2. When agent is enabled with offline endpoint: status is warn, composite status is degraded
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	mockServer.Close() // Immediately close to simulate unreachable endpoint

	cfgUnreachable := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
		Config: config.Config{
			Agent: config.AgentConfig{Enabled: true},
			LLM: config.LLMConfig{
				Backend:  "ollama",
				Endpoint: mockServer.URL,
			},
		},
	}
	srvUnreachable, err := NewServer(cfgUnreachable)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srvUnreachable.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srvUnreachable.Shutdown(shutCtx)
	}()

	resp2, err := http.Get(srvUnreachable.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp2.Body.Close()

	var healthRes2 struct {
		Ok       bool          `json:"ok"`
		Status   string        `json:"status"`
		Checks   []DoctorCheck `json:"checks"`
		Warnings []string      `json:"warnings"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&healthRes2); err != nil {
		t.Fatalf("decode health: %v", err)
	}

	var aiCheck2 *DoctorCheck
	for i := range healthRes2.Checks {
		if healthRes2.Checks[i].Name == "ai_agent" {
			aiCheck2 = &healthRes2.Checks[i]
			break
		}
	}
	if aiCheck2 == nil {
		t.Fatalf("expected ai_agent check in health response")
	}
	if aiCheck2.Status != "warn" {
		t.Errorf("expected ai_agent status warn when unreachable, got %s", aiCheck2.Status)
	}
	if healthRes2.Status != "degraded" {
		t.Errorf("expected composite status degraded, got %s", healthRes2.Status)
	}
	if !healthRes2.Ok {
		t.Errorf("expected ok=true when status is degraded, got false")
	}
}

func TestServer_Health_AIAgentCheck_DynamicConfigUpdate(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	dbPath := filepath.Join(dir, "centmem.db")
	stCfg := config.Config{
		Home:   dir,
		DBPath: dbPath,
	}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	_ = os.Chmod(dbPath, 0600)

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
		Config: config.Config{
			Home:   dir,
			DBPath: dbPath,
			Agent:  config.AgentConfig{Enabled: false},
			LLM:    config.LLMConfig{Backend: "disabled"},
		},
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	// Initial health: disabled
	resp, err := http.Get(srv.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	var h1 struct {
		Status string        `json:"status"`
		Checks []DoctorCheck `json:"checks"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&h1)

	var initialStatus string
	for _, c := range h1.Checks {
		if c.Name == "ai_agent" {
			initialStatus = c.Status
			break
		}
	}
	if initialStatus != "ok" {
		t.Errorf("expected initial ai_agent status ok (disabled), got %s", initialStatus)
	}

	modelsDir := filepath.Join(dir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	_ = os.WriteFile(filepath.Join(modelsDir, "bge-small-en-v1.5.onnx"), []byte("model-data"), 0644)

	// Dynamic update: enable agent and point to an offline endpoint
	patchPayload := []byte(`{"agent":{"enabled":true},"llm":{"backend":"ollama","endpoint":"http://127.0.0.1:59997/v1"}}`)
	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL()+"/api/config", bytes.NewReader(patchPayload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("PATCH /api/config: %v", err)
	}
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(patchResp.Body)
		t.Fatalf("PATCH /api/config status = %d: %s", patchResp.StatusCode, string(body))
	}

	// Verify GET /api/health immediately sees the updated config and reports warn/degraded
	resp2, err := http.Get(srv.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health after update: %v", err)
	}
	defer resp2.Body.Close()

	var h2 struct {
		Status string        `json:"status"`
		Checks []DoctorCheck `json:"checks"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&h2)

	var updatedStatus string
	for _, c := range h2.Checks {
		if c.Name == "ai_agent" {
			updatedStatus = c.Status
			break
		}
	}
	if updatedStatus != "warn" {
		t.Errorf("expected updated ai_agent status warn, got %s", updatedStatus)
	}
	if h2.Status != "degraded" {
		t.Errorf("expected composite status degraded, got %s; checks = %+v", h2.Status, h2.Checks)
	}
}

func TestServer_TestAgent_WhenActiveConfigDisabled(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: testVersion,
		Store:   st,
		Config: config.Config{
			Agent: config.AgentConfig{Enabled: false},
			LLM:   config.LLMConfig{Backend: "disabled"},
		},
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen-2.5"}]}`))
	}))
	defer mockLLM.Close()

	payload, _ := json.Marshal(map[string]any{
		"endpoint": mockLLM.URL,
		"model":    "qwen-2.5",
	})
	resp, err := http.Post(srv.URL()+"/api/config/test-agent", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/config/test-agent: %v", err)
	}
	defer resp.Body.Close()

	var res struct {
		Ok     bool   `json:"ok"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !res.Ok {
		t.Errorf("expected ok=true when testing endpoint with active backend disabled, got false")
	}
	if res.Status != "connected" {
		t.Errorf("expected status=connected, got %s", res.Status)
	}
}

