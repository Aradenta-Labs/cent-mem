package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"github.com/aradenta-labs/cent-mem/internal/taxonomy"
)

func openUITestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Config{DBPath: filepath.Join(t.TempDir(), "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestAPI_TagsEndpoint(t *testing.T) {
	ctx := context.Background()
	st := openUITestStore(t)

	_, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-tags",
		Type:    "note",
		Content: "Decision about SQLite",
		Tags:    []string{"decision", "database"},
	})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	_, _, err = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-tags/agent:claude",
		Type:    "note",
		Content: "Child agent decision",
		Tags:    []string{"decisions", "database"},
	})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}
	_, _, err = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:other",
		Type:    "note",
		Content: "Other project",
		Tags:    []string{"unrelated"},
	})
	if err != nil {
		t.Fatalf("put m3: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Store:   st,
		Version: testVersion,
		Config:  config.Config{Agent: config.DefaultAgentConfig()},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// 1. Subtree query detects tags + lexical cluster (decision / decisions).
	req := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-tags", nil)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/tags status = %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		OK       bool               `json:"ok"`
		Scope    string             `json:"scope"`
		Total    int                `json:"total"`
		Tags     []store.TagStat    `json:"tags"`
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK || resp.Scope != "project:ui-tags" || resp.Total != 3 {
		t.Fatalf("unexpected tags response: %+v", resp)
	}
	if resp.Tags[0].Tag != "database" || resp.Tags[0].Count != 2 {
		t.Errorf("top tag = %+v, want database(2)", resp.Tags[0])
	}
	if len(resp.Clusters) != 1 || resp.Clusters[0].Canonical != "decision" {
		t.Errorf("expected 1 lexical cluster with canonical 'decision', got %+v", resp.Clusters)
	}

	// 2. Exact scope (children=false) excludes child agent tags.
	req2 := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-tags&children=false", nil)
	rec2 := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /api/tags children=false status = %d", rec2.Code)
	}
	var resp2 struct {
		Total    int                `json:"total"`
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.Total != 2 || len(resp2.Clusters) != 0 {
		t.Errorf("children=false: got total=%d clusters=%d, want 2 and 0", resp2.Total, len(resp2.Clusters))
	}

	// 3. Nonexistent scope returns empty tags cleanly.
	req3 := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:does-not-exist", nil)
	rec3 := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("nonexistent scope status = %d", rec3.Code)
	}
	var resp3 struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(rec3.Body.Bytes(), &resp3)
	if resp3.Total != 0 {
		t.Errorf("nonexistent scope total = %d, want 0", resp3.Total)
	}

	// 4. Invalid scope returns 400.
	req4 := httptest.NewRequest(http.MethodGet, "/api/tags?scope=bad::scope", nil)
	rec4 := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Errorf("invalid scope status = %d, want 400", rec4.Code)
	}

	// 5. Applying a tag_merge proposal via POST /api/proposals/{id}/apply works end-to-end.
	payloadBytes, _ := json.Marshal(store.TagMergeProposalPayload{
		Kind:          "tag_merge",
		FromTags:      []string{"decisions"},
		ToTag:         "decision",
		AffectedCount: 1,
	})
	propID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ui-tags",
		ProposalType: "update",
		Title:        "Merge tags decisions → decision",
		Reasoning:    "Plural variant",
		PayloadJSON:  string(payloadBytes),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	applyReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/proposals/%d/apply", propID), nil)
	applyRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply tag_merge status = %d, body: %s", applyRec.Code, applyRec.Body.String())
	}

	// Subsequent /api/tags should report 0 clusters and decision(2).
	reqAfter := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-tags", nil)
	recAfter := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recAfter, reqAfter)
	var respAfter struct {
		Total    int                `json:"total"`
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	_ = json.Unmarshal(recAfter.Body.Bytes(), &respAfter)
	if respAfter.Total != 2 || len(respAfter.Clusters) != 0 {
		t.Errorf("after apply: total=%d clusters=%d, want 2 and 0", respAfter.Total, len(respAfter.Clusters))
	}
}

func TestAPI_TagsEndpoint_DeduplicatedAffectedCountAndSSEBatchApply(t *testing.T) {
	ctx := context.Background()
	st := openUITestStore(t)

	// Single memory carrying TWO lexical aliases ("auth_flow", "auth_flows") plus another carrying canonical "auth-flow".
	_, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-sse",
		Type:    "note",
		Content: "Canonical auth flow",
		Tags:    []string{"auth-flow"},
	})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	_, _, err = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-sse",
		Type:    "note",
		Content: "Memory with both alias tags",
		Tags:    []string{"auth_flow", "auth_flows"},
	})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}

	srv, err := NewServer(ServerConfig{
		Store:   st,
		Version: testVersion,
		Config:  config.Config{Agent: config.DefaultAgentConfig()},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// GET /api/tags must report affected_count=1 (distinct memory), not 2 (sum of tag counts).
	req := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-sse", nil)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/tags status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %+v", resp.Clusters)
	}
	if resp.Clusters[0].AffectedCount != 1 {
		t.Errorf("cluster affected_count = %d, want 1 (deduplicated per memory)", resp.Clusters[0].AffectedCount)
	}

	// Stage a tag_merge proposal and apply it via SSE POST /api/proposals/batch/stream.
	payloadBytes, _ := json.Marshal(store.TagMergeProposalPayload{
		Kind:          "tag_merge",
		FromTags:      []string{"auth_flow", "auth_flows"},
		ToTag:         "auth-flow",
		AffectedCount: 1,
	})
	propID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ui-sse",
		ProposalType: "update",
		Title:        "Merge tags auth_flow, auth_flows → auth-flow",
		Reasoning:    "Lexical separator & plural normalization",
		PayloadJSON:  string(payloadBytes),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"action": "apply",
		"ids":    []int64{propID},
	})
	sseReq := httptest.NewRequest(http.MethodPost, "/api/proposals/batch/stream", bytes.NewReader(body))
	sseReq.Header.Set("Content-Type", "application/json")
	sseRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(sseRec, sseReq)
	if sseRec.Code != http.StatusOK {
		t.Fatalf("SSE batch apply status = %d, body: %s", sseRec.Code, sseRec.Body.String())
	}
	sseBody := sseRec.Body.String()
	if !strings.Contains(sseBody, "event: start") || !strings.Contains(sseBody, "event: progress") || !strings.Contains(sseBody, "event: done") || !strings.Contains(sseBody, `"succeeded":1`) {
		t.Errorf("expected SSE start, progress, and done events with succeeded=1, got: %s", sseBody)
	}

	// Verify tags are merged cleanly to a single canonical tag with count 2.
	reqAfter := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-sse", nil)
	recAfter := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recAfter, reqAfter)
	var afterResp struct {
		Total    int                `json:"total"`
		Tags     []store.TagStat    `json:"tags"`
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	_ = json.Unmarshal(recAfter.Body.Bytes(), &afterResp)
	if afterResp.Total != 1 || len(afterResp.Clusters) != 0 || afterResp.Tags[0].Tag != "auth-flow" || afterResp.Tags[0].Count != 2 {
		t.Errorf("after SSE batch apply: %+v", afterResp)
	}
}

func TestAPI_TagsEndpoint_NormalizesLegacyMixedCaseCanonical(t *testing.T) {
	ctx := context.Background()
	st := openUITestStore(t)

	// Seed 2 memories with legacy uppercase "Database" and 1 with lowercase "database"
	// so ClusterTags picks "Database" by count; GET /api/tags must normalize canonical
	// to lowercase "database" with alias ["Database"] and affected_count=2 to match curate --type tags.
	id1, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-mixed",
		Type:    "note",
		Content: "Legacy DB note 1",
		Tags:    []string{"database"},
	})
	if err != nil {
		t.Fatalf("put m1: %v", err)
	}
	id2, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-mixed",
		Type:    "note",
		Content: "Legacy DB note 2",
		Tags:    []string{"database"},
	})
	if err != nil {
		t.Fatalf("put m2: %v", err)
	}
	_, _, err = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:ui-mixed",
		Type:    "note",
		Content: "Lowercase DB note 3",
		Tags:    []string{"database"},
	})
	if err != nil {
		t.Fatalf("put m3: %v", err)
	}
	for _, id := range []int64{id1, id2} {
		if _, err := st.DB().ExecContext(ctx, `UPDATE memories SET tags = 'Database' WHERE id = ?`, id); err != nil {
			t.Fatalf("inject mixed-case tag on %d: %v", id, err)
		}
	}

	srv, err := NewServer(ServerConfig{
		Store:   st,
		Version: testVersion,
		Config:  config.Config{Agent: config.DefaultAgentConfig()},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-mixed", nil)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/tags status = %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %+v", resp.Clusters)
	}
	cl := resp.Clusters[0]
	if cl.Canonical != "database" {
		t.Errorf("canonical = %q, want %q", cl.Canonical, "database")
	}
	if len(cl.Aliases) != 1 || cl.Aliases[0] != "Database" {
		t.Errorf("aliases = %v, want [Database]", cl.Aliases)
	}
	if cl.AffectedCount != 2 {
		t.Errorf("affected_count = %d, want 2", cl.AffectedCount)
	}

	// Also verify a legacy mixed-case cluster where NO lowercase tag exists yet ("Caching" x2 + "Cachings" x1)
	// normalizes to canonical "caching", aliases ["Caching", "Cachings"], affected_count=3, and applies cleanly.
	c1, _, _ := st.PutMemory(ctx, store.MemoryInput{Scope: "project:ui-mixed-nolower", Type: "note", Content: "Cache 1", Tags: []string{"caching"}})
	c2, _, _ := st.PutMemory(ctx, store.MemoryInput{Scope: "project:ui-mixed-nolower", Type: "note", Content: "Cache 2", Tags: []string{"caching"}})
	c3, _, _ := st.PutMemory(ctx, store.MemoryInput{Scope: "project:ui-mixed-nolower", Type: "note", Content: "Cache 3", Tags: []string{"cachings"}})
	_, _ = st.DB().ExecContext(ctx, `UPDATE memories SET tags = 'Caching' WHERE id IN (?, ?)`, c1, c2)
	_, _ = st.DB().ExecContext(ctx, `UPDATE memories SET tags = 'Cachings' WHERE id = ?`, c3)

	req2 := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-mixed-nolower", nil)
	rec2 := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /api/tags (no-lowercase) status = %d, body: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 struct {
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal resp2: %v", err)
	}
	if len(resp2.Clusters) != 1 {
		t.Fatalf("expected 1 cluster for no-lowercase mixed-case tags, got %+v", resp2.Clusters)
	}
	cl2 := resp2.Clusters[0]
	if cl2.Canonical != "caching" {
		t.Errorf("canonical = %q, want %q", cl2.Canonical, "caching")
	}
	if len(cl2.Aliases) != 2 || cl2.Aliases[0] != "Caching" || cl2.Aliases[1] != "Cachings" {
		t.Errorf("aliases = %v, want [Caching Cachings]", cl2.Aliases)
	}
	if cl2.AffectedCount != 3 {
		t.Errorf("affected_count = %d, want 3", cl2.AffectedCount)
	}

	// Applying the corresponding tag_merge proposal collapses all 3 memories to lowercase "caching".
	payloadBytes, _ := json.Marshal(store.TagMergeProposalPayload{
		Kind:          "tag_merge",
		FromTags:      cl2.Aliases,
		ToTag:         cl2.Canonical,
		AffectedCount: cl2.AffectedCount,
	})
	propID, err := st.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:ui-mixed-nolower",
		ProposalType: "update",
		Title:        "Merge tags Caching, Cachings → caching",
		Reasoning:    "Legacy mixed-case normalization",
		PayloadJSON:  string(payloadBytes),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	applyReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/proposals/%d/apply", propID), nil)
	applyRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, body: %s", applyRec.Code, applyRec.Body.String())
	}
	reqAfter := httptest.NewRequest(http.MethodGet, "/api/tags?scope=project:ui-mixed-nolower", nil)
	recAfter := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recAfter, reqAfter)
	var afterResp struct {
		Total    int                `json:"total"`
		Tags     []store.TagStat    `json:"tags"`
		Clusters []taxonomy.Cluster `json:"clusters"`
	}
	_ = json.Unmarshal(recAfter.Body.Bytes(), &afterResp)
	if afterResp.Total != 1 || len(afterResp.Clusters) != 0 || afterResp.Tags[0].Tag != "caching" || afterResp.Tags[0].Count != 3 {
		t.Errorf("after apply: %+v", afterResp)
	}
}
