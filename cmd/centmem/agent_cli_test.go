package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestAgentCLI_Ask_Offline(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Disable LLM in config
	runCLI(t, home, "config", "set", "llm.backend", "disabled")

	// Seed memory
	_, _, code := runCLI(t, home, "put", "--scope", "project:alpha", "--type", "note", "--content", "Architecture uses SQLite with SQLite-vec")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	// Single-shot ask
	stdout, stderr, code := runCLI(t, home, "ask", "what database do we use?", "--scope", "project:alpha", "--top", "3")
	if code != 0 {
		t.Fatalf("ask code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
	if res["fallback_used"] != true {
		t.Errorf("expected fallback_used=true, got %v", res["fallback_used"])
	}
	answer, _ := res["answer"].(string)
	if !strings.Contains(answer, "Offline Mode") {
		t.Errorf("expected answer to contain 'Offline Mode', got: %s", answer)
	}
	if !strings.Contains(answer, "SQLite-vec") {
		t.Errorf("expected answer to contain 'SQLite-vec', got: %s", answer)
	}

	citations, ok := res["citations"].([]any)
	if !ok || len(citations) == 0 {
		t.Errorf("expected at least 1 citation, got: %v", res["citations"])
	}
}

func TestAgentCLI_Ask_MissingQuestion(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "ask", "--scope", "project:alpha")
	if code != 1 {
		t.Fatalf("ask code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "question is required") {
		t.Errorf("expected 'question is required' in stderr, got: %s", stderr)
	}
}

func TestAgentCLI_Ask_MockLLM(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Seed a memory to retrieve
	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	mID, _, _ := s.PutMemory(context.Background(), store.MemoryInput{
		Scope:   "project:react",
		Type:    "note",
		Content: "Authentication is configured with RS256 JWT tokens",
		Tags:    []string{"auth", "jwt"},
	})
	s.Close()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")

		if call == 1 {
			// First turn: model calls search_memories
			resp := agent.ChatResponse{
				ID: "resp-1",
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
										Arguments: `{"query":"Authentication tokens","scope":"project:react"}`,
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

		// Second turn: model returns synthesized answer with citation
		resp := agent.ChatResponse{
			ID: "resp-2",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role:    "assistant",
						Content: fmt.Sprintf("Authentication uses RS256 JWT tokens per [id: %d].\n\nKnowledge Gap: Refresh token expiration is not documented.", mID),
					},
					FinishReason: "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Configure centmem with mock server
	runCLI(t, home, "config", "set", "llm.backend", "openai_compatible")
	runCLI(t, home, "config", "set", "llm.endpoint", server.URL)
	runCLI(t, home, "config", "set", "llm.model", "test-model")

	stdout, stderr, code := runCLI(t, home, "ask", "how is authentication handled?", "--scope", "project:react")
	if code != 0 {
		t.Fatalf("ask code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
	if res["fallback_used"] != false {
		t.Errorf("expected fallback_used=false, got %v", res["fallback_used"])
	}
	steps, _ := res["reasoning_steps"].(float64)
	if steps < 2 {
		t.Errorf("expected >= 2 reasoning steps, got %v", steps)
	}

	gaps, ok := res["knowledge_gaps"].([]any)
	if !ok || len(gaps) == 0 {
		t.Errorf("expected knowledge gaps, got: %v", res["knowledge_gaps"])
	}

	citations, ok := res["citations"].([]any)
	if !ok || len(citations) == 0 {
		t.Errorf("expected citations, got: %v", res["citations"])
	}
}

func TestAgentCLI_Ask_Interactive(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Disable LLM for predictable offline response
	runCLI(t, home, "config", "set", "llm.backend", "disabled")

	input := "how do we deploy?\nexit\n"
	stdout, stderr, code := runCLIWithStdin(t, home, input, "ask", "--interactive", "--scope", "project:demo")
	if code != 0 {
		t.Fatalf("ask --interactive code = %d, want 0, stderr: %s", code, stderr)
	}

	if !strings.Contains(stdout, "centmem> ") {
		t.Errorf("expected interactive prompt 'centmem> ', got: %s", stdout)
	}
}

func TestAgentCLI_Curate_OfflineDedup(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:curate", Type: "note", Content: "Identical duplicate text content"})
	_, _ = s.DB().ExecContext(ctx, "UPDATE memories SET updated_at = ? WHERE id = ?", time.Now().Add(-5*time.Minute).UnixMicro(), m1)
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:curate", Type: "note", Content: "Identical duplicate text content"})
	s.Close()

	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:curate", "--type", "dedup")
	if code != 0 {
		t.Fatalf("curate code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
	created, _ := res["proposals_created"].([]any)
	if len(created) != 1 {
		t.Errorf("expected 1 proposal created, got %v", res["proposals_created"])
	}

	// Verify proposal exists in proposals list
	stdout, _, code = runCLI(t, home, "proposals", "list", "--scope", "project:curate", "--status", "pending")
	if code != 0 {
		t.Fatalf("proposals list code = %d, want 0", code)
	}
	var plist map[string]any
	_ = json.Unmarshal([]byte(stdout), &plist)
	props, _ := plist["proposals"].([]any)
	if len(props) != 1 {
		t.Errorf("expected 1 pending proposal, got %v", props)
	}
}

func TestAgentCLI_Curate_DryRun(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:dry", Type: "note", Content: "Identical content for dry run test"})
	_, _ = s.DB().ExecContext(ctx, "UPDATE memories SET updated_at = ? WHERE id = ?", time.Now().Add(-5*time.Minute).UnixMicro(), m1)
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:dry", Type: "note", Content: "Identical content for dry run test"})
	s.Close()

	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:dry", "--type", "dedup", "--dry-run")
	if code != 0 {
		t.Fatalf("curate dry-run code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	created, _ := res["proposals_created"].([]any)
	if len(created) != 0 {
		t.Errorf("expected 0 proposals created in dry-run, got %v", created)
	}

	// Verify no proposal in DB
	stdout, _, _ = runCLI(t, home, "proposals", "list", "--scope", "project:dry")
	var plist map[string]any
	_ = json.Unmarshal([]byte(stdout), &plist)
	props, _ := plist["proposals"].([]any)
	if len(props) != 0 {
		t.Errorf("expected 0 proposals in store, got %d", len(props))
	}
}

func TestAgentCLI_Curate_AutoApply(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:auto", Type: "note", Content: "Duplicate content for auto-apply test"})
	_, _ = s.DB().ExecContext(ctx, "UPDATE memories SET updated_at = ? WHERE id = ?", time.Now().Add(-5*time.Minute).UnixMicro(), m1)
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:auto", Type: "note", Content: "Duplicate content for auto-apply test"})
	s.Close()

	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:auto", "--type", "dedup", "--apply")
	if code != 0 {
		t.Fatalf("curate apply code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	applied, _ := res["proposals_applied"].([]any)
	if len(applied) != 1 {
		t.Errorf("expected 1 proposal applied, got %v", res["proposals_applied"])
	}

	// Check proposal status is applied
	stdout, _, _ = runCLI(t, home, "proposals", "list", "--scope", "project:auto", "--status", "applied")
	var plist map[string]any
	_ = json.Unmarshal([]byte(stdout), &plist)
	props, _ := plist["proposals"].([]any)
	if len(props) != 1 {
		t.Errorf("expected 1 applied proposal, got %v", props)
	}
}

func TestAgentCLI_Curate_InvalidType(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "curate", "--type", "invalid-type")
	if code != 1 {
		t.Fatalf("curate code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid --type") {
		t.Errorf("expected 'invalid --type' in stderr, got: %s", stderr)
	}
}

func TestAgentCLI_Summarize_FormatsAndSave(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Seed some memories
	runCLI(t, home, "put", "--scope", "project:briefing", "--type", "note", "--content", "Primary database is SQLite-vec")
	runCLI(t, home, "put", "--scope", "project:briefing", "--type", "note", "--content", "Authentication is RS256 JWT")

	// 1. JSON format without save
	stdout, stderr, code := runCLI(t, home, "summarize", "--scope", "project:briefing", "--focus", "Storage", "--format", "json")
	if code != 0 {
		t.Fatalf("summarize json code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}
	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
	title, _ := res["title"].(string)
	if !strings.Contains(title, "Storage") {
		t.Errorf("expected title to contain 'Storage', got: %s", title)
	}
	if res["saved_id"] != nil {
		t.Errorf("expected saved_id=nil when --save not passed, got: %v", res["saved_id"])
	}

	// 2. Raw Markdown format
	stdout, _, code = runCLI(t, home, "summarize", "--scope", "project:briefing", "--format", "markdown")
	if code != 0 {
		t.Fatalf("summarize markdown code = %d, want 0", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "#") {
		t.Errorf("expected markdown output starting with '#', got: %s", stdout[:30])
	}

	// 3. JSON format WITH --save
	stdout, _, code = runCLI(t, home, "summarize", "--scope", "project:briefing", "--save")
	if code != 0 {
		t.Fatalf("summarize --save code = %d, want 0", code)
	}
	var saveRes map[string]any
	_ = json.Unmarshal([]byte(stdout), &saveRes)
	savedID, ok := saveRes["saved_id"].(float64)
	if !ok || savedID <= 0 {
		t.Fatalf("expected saved_id > 0, got: %v", saveRes["saved_id"])
	}

	// Verify memory exists with tags
	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	mem, err := s.GetMemory(context.Background(), int64(savedID))
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if mem.Type != "note" {
		t.Errorf("expected type 'note', got %q", mem.Type)
	}
}

func TestAgentCLI_Summarize_InvalidFormat(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "summarize", "--format", "yaml")
	if code != 1 {
		t.Fatalf("summarize code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid --format") {
		t.Errorf("expected 'invalid --format' in stderr, got: %s", stderr)
	}
}

func TestAgentCLI_Proposals_Lifecycle(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:prop", Type: "note", Content: "Original decision: use MySQL"})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:prop", Type: "note", Content: "Updated decision: use SQLite-vec"})

	payload, _ := json.Marshal(store.LinkProposalPayload{FromID: m2, ToID: m1, Relation: "supersedes"})
	propID1, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:prop",
		ProposalType: "link",
		Title:        fmt.Sprintf("Link memory #%d ──supersedes──► memory #%d", m2, m1),
		Reasoning:    "SQLite-vec supersedes old MySQL note",
		PayloadJSON:  string(payload),
	})
	if err != nil {
		t.Fatalf("CreateProposal 1: %v", err)
	}

	propID2, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:prop",
		ProposalType: "link",
		Title:        "Temporary link proposal",
		Reasoning:    "To be dismissed",
		PayloadJSON:  string(payload),
	})
	if err != nil {
		t.Fatalf("CreateProposal 2: %v", err)
	}
	s.Close()

	// 1. List proposals
	stdout, stderr, code := runCLI(t, home, "proposals", "list", "--scope", "project:prop", "--status", "pending")
	if code != 0 {
		t.Fatalf("proposals list code = %d, want 0, stderr: %s", code, stderr)
	}
	var listRes map[string]any
	_ = json.Unmarshal([]byte(stdout), &listRes)
	props, _ := listRes["proposals"].([]any)
	if len(props) != 2 {
		t.Fatalf("expected 2 pending proposals, got %d", len(props))
	}

	// 2. Show proposal 1
	stdout, stderr, code = runCLI(t, home, "proposals", "show", fmt.Sprintf("%d", propID1))
	if code != 0 {
		t.Fatalf("proposals show code = %d, want 0, stderr: %s", code, stderr)
	}
	var showRes map[string]any
	_ = json.Unmarshal([]byte(stdout), &showRes)
	prop, _ := showRes["proposal"].(map[string]any)
	if prop["status"] != "pending" {
		t.Errorf("expected status 'pending', got %v", prop["status"])
	}

	// 3. Apply proposal 1
	stdout, stderr, code = runCLI(t, home, "proposals", "apply", fmt.Sprintf("%d", propID1))
	if code != 0 {
		t.Fatalf("proposals apply code = %d, want 0, stderr: %s", code, stderr)
	}
	var applyRes map[string]any
	_ = json.Unmarshal([]byte(stdout), &applyRes)
	if applyRes["applied"] != true {
		t.Errorf("expected applied=true, got %v", applyRes["applied"])
	}

	// 4. Apply again -> expect conflict exit code 3
	_, stderr, code = runCLI(t, home, "proposals", "apply", fmt.Sprintf("%d", propID1))
	if code != 3 {
		t.Fatalf("re-apply code = %d, want 3 (conflict)", code)
	}

	// 5. Dismiss proposal 2
	stdout, stderr, code = runCLI(t, home, "proposals", "dismiss", fmt.Sprintf("%d", propID2))
	if code != 0 {
		t.Fatalf("proposals dismiss code = %d, want 0, stderr: %s", code, stderr)
	}
	var dismissRes map[string]any
	_ = json.Unmarshal([]byte(stdout), &dismissRes)
	if dismissRes["dismissed"] != true {
		t.Errorf("expected dismissed=true, got %v", dismissRes["dismissed"])
	}

	// 6. Verify proposal 2 is now dismissed in list
	stdout, _, code = runCLI(t, home, "proposals", "list", "--scope", "project:prop", "--status", "dismissed")
	if code != 0 {
		t.Fatalf("proposals list dismissed code = %d, want 0", code)
	}
	var dismissedList map[string]any
	_ = json.Unmarshal([]byte(stdout), &dismissedList)
	dProps, _ := dismissedList["proposals"].([]any)
	if len(dProps) != 1 {
		t.Errorf("expected 1 dismissed proposal, got %d", len(dProps))
	}
}

func TestAgentCLI_Proposals_Errors(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Missing action
	_, _, code := runCLI(t, home, "proposals")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}

	// Unknown action
	_, _, code = runCLI(t, home, "proposals", "foobar")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}

	// Show missing ID
	_, _, code = runCLI(t, home, "proposals", "show")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}

	// Show non-existent proposal -> exit code 2 (not found)
	_, _, code = runCLI(t, home, "proposals", "show", "999999")
	if code != 2 {
		t.Errorf("code = %d, want 2 (not found)", code)
	}

	// Apply non-existent proposal -> exit code 2 (not found)
	_, _, code = runCLI(t, home, "proposals", "apply", "999999")
	if code != 2 {
		t.Errorf("code = %d, want 2 (not found)", code)
	}

	// Dismiss non-existent proposal -> exit code 2 (not found)
	_, _, code = runCLI(t, home, "proposals", "dismiss", "999999")
	if code != 2 {
		t.Errorf("code = %d, want 2 (not found)", code)
	}

	// Invalid status filter in list
	_, _, code = runCLI(t, home, "proposals", "list", "--status", "invalid-status")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
}

func TestAgentCLI_ScopeValidation(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	badScope := "invalid:::scope!!!"

	// 1. ask
	_, stderr, code := runCLI(t, home, "ask", "hello", "--scope", badScope)
	if code != 1 {
		t.Errorf("ask bad scope code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid --scope") {
		t.Errorf("expected 'invalid --scope' in stderr, got: %s", stderr)
	}

	// 2. curate
	_, stderr, code = runCLI(t, home, "curate", "--scope", badScope)
	if code != 1 {
		t.Errorf("curate bad scope code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid --scope") {
		t.Errorf("expected 'invalid --scope' in stderr, got: %s", stderr)
	}

	// 3. summarize
	_, stderr, code = runCLI(t, home, "summarize", "--scope", badScope)
	if code != 1 {
		t.Errorf("summarize bad scope code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid --scope") {
		t.Errorf("expected 'invalid --scope' in stderr, got: %s", stderr)
	}

	// 4. proposals list
	_, stderr, code = runCLI(t, home, "proposals", "list", "--scope", badScope)
	if code != 1 {
		t.Errorf("proposals list bad scope code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid --scope") {
		t.Errorf("expected 'invalid --scope' in stderr, got: %s", stderr)
	}
}

func TestAgentCLI_Ask_TopCapping(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "config", "set", "llm.backend", "disabled")

	// Seed 5 memories
	for i := 1; i <= 5; i++ {
		runCLI(t, home, "put", "--scope", "project:toptest", "--type", "note",
			"--content", fmt.Sprintf("Architecture component number %d uses microservices", i))
	}

	// Recall with top 2
	stdout, _, code := runCLI(t, home, "ask", "microservices architecture", "--scope", "project:toptest", "--top", "2")
	if code != 0 {
		t.Fatalf("ask code = %d, want 0", code)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}
	citations, ok := res["citations"].([]any)
	if !ok || len(citations) != 2 {
		t.Errorf("expected exactly 2 citations, got: %d", len(citations))
	}

	// Recall with top 0 -> should default to 5
	stdout, _, code = runCLI(t, home, "ask", "microservices architecture", "--scope", "project:toptest", "--top", "0")
	if code != 0 {
		t.Fatalf("ask code = %d, want 0", code)
	}
	var resDef map[string]any
	_ = json.Unmarshal([]byte(stdout), &resDef)
	citationsDef, _ := resDef["citations"].([]any)
	if len(citationsDef) != 5 {
		t.Errorf("expected default 5 citations when --top 0, got: %d", len(citationsDef))
	}
}

func TestAgentCLI_Proposals_ConflictVsError(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:test", Type: "note", Content: "mem1"})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:test", Type: "note", Content: "mem2"})

	payload, _ := json.Marshal(store.LinkProposalPayload{FromID: m1, ToID: m2, Relation: "supersedes"})
	propID, _ := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:test",
		ProposalType: "link",
		Title:        "Link prop",
		PayloadJSON:  string(payload),
	})

	// Apply proposal
	_, _, code := runCLI(t, home, "proposals", "apply", fmt.Sprintf("%d", propID))
	if code != 0 {
		t.Fatalf("apply code = %d, want 0", code)
	}

	// Dismissing an already applied proposal -> must return CONFLICT (exit code 3)
	_, stderr, code := runCLI(t, home, "proposals", "dismiss", fmt.Sprintf("%d", propID))
	if code != 3 {
		t.Errorf("dismiss applied proposal code = %d, want 3 (conflict), stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "CONFLICT") {
		t.Errorf("expected 'CONFLICT' in stderr, got: %s", stderr)
	}

	// Create a proposal with corrupted payload JSON
	badPropID, _ := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:test",
		ProposalType: "link",
		Title:        "Corrupt proposal",
		PayloadJSON:  "{not valid json!!",
	})
	s.Close()

	// Applying corrupt proposal -> must return error (exit code 1), NOT conflict (code 3)
	_, stderr, code = runCLI(t, home, "proposals", "apply", fmt.Sprintf("%d", badPropID))
	if code != 1 {
		t.Errorf("apply corrupt proposal code = %d, want 1 (internal/error), got %d, stderr: %s", code, code, stderr)
	}
}

func TestAgentCLI_Curate_OnlineProposalCounts(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:online", Type: "note", Content: "PostgreSQL is our database"})
	m2, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:online", Type: "note", Content: "MySQL is our database"})
	m3, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:online", Type: "note", Content: "Identical dupe content"})
	_, _ = s.DB().ExecContext(ctx, "UPDATE memories SET updated_at = ? WHERE id = ?", time.Now().Add(-5*time.Minute).UnixMicro(), m3)
	m4, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:online", Type: "note", Content: "Identical dupe content"})
	s.Close()

	var turn int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tCount := atomic.AddInt32(&turn, 1)
		w.Header().Set("Content-Type", "application/json")

		if tCount == 1 {
			// Turn 1: Propose contradiction link
			resp := agent.ChatResponse{
				ID: "resp-1",
				Choices: []agent.Choice{
					{
						Index: 0,
						Message: agent.ChatMessage{
							Role: "assistant",
							ToolCalls: []agent.ToolCall{
								{
									ID:   "call_link_1",
									Type: "function",
									Function: agent.FunctionCall{
										Name:      "propose_link",
										Arguments: fmt.Sprintf(`{"from_id":%d,"to_id":%d,"relation":"contradicts","reasoning":"conflicting database choices"}`, m1, m2),
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

		if tCount == 2 {
			// Turn 2: Propose duplicate merge
			resp := agent.ChatResponse{
				ID: "resp-2",
				Choices: []agent.Choice{
					{
						Index: 0,
						Message: agent.ChatMessage{
							Role: "assistant",
							ToolCalls: []agent.ToolCall{
								{
									ID:   "call_merge_1",
									Type: "function",
									Function: agent.FunctionCall{
										Name:      "propose_merge",
										Arguments: fmt.Sprintf(`{"source_ids":[%d,%d],"title":"Consolidate dupes","content":"Identical dupe content","reasoning":"Duplicate notes"}`, m3, m4),
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

		// Turn 3: Complete
		resp := agent.ChatResponse{
			ID: "resp-3",
			Choices: []agent.Choice{
				{
					Index: 0,
					Message: agent.ChatMessage{
						Role:    "assistant",
						Content: "Curation complete. Identified 1 contradiction and 1 duplicate cluster.",
					},
					FinishReason: "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runCLI(t, home, "config", "set", "llm.backend", "openai_compatible")
	runCLI(t, home, "config", "set", "llm.endpoint", server.URL)
	runCLI(t, home, "config", "set", "llm.model", "curate-model")

	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:online", "--type", "all")
	if code != 0 {
		t.Fatalf("curate code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse: %v", err)
	}

	if res["fallback_used"] != false {
		t.Errorf("expected fallback_used=false, got %v", res["fallback_used"])
	}

	created, _ := res["proposals_created"].([]any)
	if len(created) != 2 {
		t.Errorf("expected 2 created proposals, got %v", res["proposals_created"])
	}

	contra, _ := res["contradictions_found"].(float64)
	if contra != 1 {
		t.Errorf("expected contradictions_found=1, got %v", res["contradictions_found"])
	}

	dupes, _ := res["duplicates_found"].(float64)
	if dupes != 1 {
		t.Errorf("expected duplicates_found=1, got %v", res["duplicates_found"])
	}
}

func TestAgentCLI_Summarize_DeterministicOrder(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Seed multiple types
	runCLI(t, home, "put", "--scope", "project:det", "--type", "note", "--content", "Note 1 about architecture")
	runCLI(t, home, "put", "--scope", "project:det", "--type", "log", "--content", "Log 1 about deployment")
	runCLI(t, home, "set", "--scope", "project:det", "--key", "fact.k", "--value", `"value"`)

	// Run summarize twice
	stdout1, _, code1 := runCLI(t, home, "summarize", "--scope", "project:det", "--format", "markdown")
	stdout2, _, code2 := runCLI(t, home, "summarize", "--scope", "project:det", "--format", "markdown")

	if code1 != 0 || code2 != 0 {
		t.Fatalf("summarize failed: code1=%d, code2=%d", code1, code2)
	}

	if stdout1 != stdout2 {
		t.Errorf("summarize outputs differ across runs!\nRun 1:\n%s\nRun 2:\n%s", stdout1, stdout2)
	}
}

func TestAgentCLI_Ask_InteractiveCommands(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "config", "set", "llm.backend", "disabled")

	input := "/help\n/clear\nhello\nexit\n"
	stdout, stderr, code := runCLIWithStdin(t, home, input, "ask", "--interactive", "--scope", "project:interactive")
	if code != 0 {
		t.Fatalf("interactive code = %d, want 0, stderr: %s", code, stderr)
	}

	if !strings.Contains(stdout, "Interactive Commands:") {
		t.Errorf("expected help output in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Cleared conversation context") {
		t.Errorf("expected clear output in stdout, got: %s", stdout)
	}
}

func TestAgentCLI_Ask_UnreachableEndpoint_Fallback(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Configure an unreachable LLM endpoint with a short timeout
	runCLI(t, home, "config", "set", "llm.backend", "openai_compatible")
	runCLI(t, home, "config", "set", "llm.endpoint", "http://127.0.0.1:59999")
	runCLI(t, home, "config", "set", "llm.timeout_seconds", "1")

	// Seed memory
	_, _, code := runCLI(t, home, "put", "--scope", "project:unreach", "--type", "note", "--content", "Unreachable endpoint fallback test memory")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	// Run ask - should gracefully fall back to hybrid search recall
	stdout, stderr, code := runCLI(t, home, "ask", "fallback test", "--scope", "project:unreach", "--top", "3")
	if code != 0 {
		t.Fatalf("ask code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
	if res["fallback_used"] != true {
		t.Errorf("expected fallback_used=true, got %v", res["fallback_used"])
	}
	answer, _ := res["answer"].(string)
	if !strings.Contains(answer, "Offline Mode") {
		t.Errorf("expected answer to contain 'Offline Mode', got: %s", answer)
	}
	if !strings.Contains(answer, "Unreachable endpoint fallback test memory") {
		t.Errorf("expected answer to contain memory snippet, got: %s", answer)
	}

	citations, ok := res["citations"].([]any)
	if !ok || len(citations) == 0 {
		t.Errorf("expected at least 1 citation, got: %v", res["citations"])
	}
}

func TestAgentCLI_Curate_UnreachableEndpoint_Fallback(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Configure an unreachable LLM endpoint with a short timeout
	runCLI(t, home, "config", "set", "llm.backend", "openai_compatible")
	runCLI(t, home, "config", "set", "llm.endpoint", "http://127.0.0.1:59999")
	runCLI(t, home, "config", "set", "llm.timeout_seconds", "1")

	// Seed identical duplicate memories
	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	m1, _, _ := s.PutMemory(ctx, store.MemoryInput{Scope: "project:curate-unreach", Type: "note", Content: "Identical memory for offline curation"})
	_, _ = s.DB().ExecContext(ctx, "UPDATE memories SET updated_at = ? WHERE id = ?", time.Now().Add(-5*time.Minute).UnixMicro(), m1)
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:curate-unreach", Type: "note", Content: "Identical memory for offline curation"})
	s.Close()

	// Run curate - should gracefully fall back to heuristic exact deduplication
	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:curate-unreach", "--type", "dedup")
	if code != 0 {
		t.Fatalf("curate code = %d, want 0, stderr: %s", code, stderr)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json parse error: %v", err)
	}

	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
	if res["fallback_used"] != true {
		t.Errorf("expected fallback_used=true, got %v", res["fallback_used"])
	}
	created, ok := res["proposals_created"].([]any)
	if !ok || len(created) == 0 {
		t.Fatalf("expected at least 1 proposal created, got: %v", res["proposals_created"])
	}

	propID := fmt.Sprintf("%v", created[0])

	// Show proposal
	stdout, _, code = runCLI(t, home, "proposals", "show", propID)
	if code != 0 {
		t.Fatalf("proposals show code = %d, want 0", code)
	}
	var showRes map[string]any
	if err := json.Unmarshal([]byte(stdout), &showRes); err != nil {
		t.Fatalf("json parse error: %v", err)
	}
	prop, _ := showRes["proposal"].(map[string]any)
	if prop["proposal_type"] != "merge" {
		t.Errorf("expected proposal_type=merge, got %v", prop["proposal_type"])
	}

	// Apply proposal
	stdout, _, code = runCLI(t, home, "proposals", "apply", propID)
	if code != 0 {
		t.Fatalf("proposals apply code = %d, want 0", code)
	}
}

