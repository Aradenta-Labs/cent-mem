package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestServer_HealthAndStaticServing(t *testing.T) {
	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0, // ephemeral port for test isolation
		NoOpen:  true,
		Version: "1.4.0",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Test /api/health
	healthURL := srv.URL() + "/api/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		t.Fatalf("GET /api/health error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/health status = %d, want 200", resp.StatusCode)
	}

	var healthData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
		t.Fatalf("decode health json: %v", err)
	}
	if healthData["ok"] != true || healthData["status"] != "healthy" || healthData["version"] != "1.4.0" {
		t.Errorf("unexpected health payload: %+v", healthData)
	}

	// 2. Test root index.html
	rootURL := srv.URL() + "/"
	respRoot, err := client.Get(rootURL)
	if err != nil {
		t.Fatalf("GET / error: %v", err)
	}
	defer respRoot.Body.Close()

	if respRoot.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want 200", respRoot.StatusCode)
	}
	bodyRoot, _ := io.ReadAll(respRoot.Body)
	if !strings.Contains(string(bodyRoot), "centmem") {
		t.Errorf("GET / body missing 'centmem': %s", string(bodyRoot))
	}

	// 3. Test SPA fallback route (/ui/design-system)
	spaURL := srv.URL() + "/ui/design-system"
	respSPA, err := client.Get(spaURL)
	if err != nil {
		t.Fatalf("GET /ui/design-system error: %v", err)
	}
	defer respSPA.Body.Close()

	if respSPA.StatusCode != http.StatusOK {
		t.Errorf("GET /ui/design-system status = %d, want 200", respSPA.StatusCode)
	}
	bodySPA, _ := io.ReadAll(respSPA.Body)
	if !strings.Contains(string(bodySPA), "<div id=\"root\">") {
		t.Errorf("GET /ui/design-system expected index.html fallback with root div: %s", string(bodySPA))
	}

	// 4. Test missing API route returns 404
	missingAPIURL := srv.URL() + "/api/nonexistent"
	respMissing, err := client.Get(missingAPIURL)
	if err != nil {
		t.Fatalf("GET /api/nonexistent error: %v", err)
	}
	defer respMissing.Body.Close()

	if respMissing.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/nonexistent status = %d, want 404", respMissing.StatusCode)
	}
}

func TestServer_ScopesAPI(t *testing.T) {
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	// Seed store with memories
	ctx := context.Background()
	_, _, _ = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:myapp",
		Type:    "note",
		Content: "test note in myapp",
	})

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. GET /api/health with store connected
	healthResp, err := client.Get(srv.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health error: %v", err)
	}
	defer healthResp.Body.Close()
	var healthData map[string]any
	_ = json.NewDecoder(healthResp.Body).Decode(&healthData)
	if healthData["store"] != "connected" {
		t.Errorf("expected store 'connected', got %v", healthData["store"])
	}

	// 2. GET /api/scopes
	scopesResp, err := client.Get(srv.URL() + "/api/scopes")
	if err != nil {
		t.Fatalf("GET /api/scopes error: %v", err)
	}
	defer scopesResp.Body.Close()

	if scopesResp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/scopes status = %d, want 200", scopesResp.StatusCode)
	}

	var scopesData struct {
		OK     bool               `json:"ok"`
		Scopes []*store.ScopeNode `json:"scopes"`
	}
	if err := json.NewDecoder(scopesResp.Body).Decode(&scopesData); err != nil {
		t.Fatalf("decode scopes json: %v", err)
	}
	if !scopesData.OK || len(scopesData.Scopes) == 0 {
		t.Fatalf("expected non-empty scopes list, got: %+v", scopesData)
	}
	// Check global root node
	g := scopesData.Scopes[0]
	if g.Path != "global" {
		t.Errorf("expected root 'global', got %q", g.Path)
	}
	if g.TotalCount != 1 {
		t.Errorf("expected global total_count 1, got %d", g.TotalCount)
	}

	// 3. POST /api/scopes with valid scope
	createPayload := []byte(`{"path": "project:newapp"}`)
	postResp, err := client.Post(srv.URL()+"/api/scopes", "application/json", bytes.NewReader(createPayload))
	if err != nil {
		t.Fatalf("POST /api/scopes error: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusCreated {
		t.Errorf("POST /api/scopes status = %d, want 201", postResp.StatusCode)
	}
	var postData map[string]any
	_ = json.NewDecoder(postResp.Body).Decode(&postData)
	if postData["ok"] != true {
		t.Errorf("expected ok=true, got %+v", postData)
	}

	// 4. POST /api/scopes with invalid scope
	badPayload := []byte(`{"path": "invalid::syntax"}`)
	badResp, err := client.Post(srv.URL()+"/api/scopes", "application/json", bytes.NewReader(badPayload))
	if err != nil {
		t.Fatalf("POST /api/scopes invalid error: %v", err)
	}
	defer badResp.Body.Close()

	if badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /api/scopes with invalid path status = %d, want 400", badResp.StatusCode)
	}
}

func TestServer_Memories(t *testing.T) {
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	// Seed store with diverse memories
	noteID, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:         "project:webapp",
		Type:          "note",
		Content:       "Refactored memory browser table for high density",
		Tags:          []string{"ui", "density", "refactor"},
		SourceAgent:   "agent-alpha",
		SourceSession: "sess-1",
	})
	if err != nil {
		t.Fatalf("seed note: %v", err)
	}

	factID, _, err := st.SetFact(ctx, store.FactInput{
		Scope:       "project:webapp",
		Key:         "api_version",
		Value:       `{"major": 1, "minor": 4}`,
		Tags:        []string{"config", "version"},
		SourceAgent: "agent-beta",
	})
	if err != nil {
		t.Fatalf("seed fact: %v", err)
	}

	_, _, err = st.PutMemory(ctx, store.MemoryInput{
		Scope:         "project:webapp/agent:agent-alpha",
		Type:          "log",
		Content:       "Agent alpha session checkpoint log entry",
		Tags:          []string{"log", "checkpoint"},
		SourceAgent:   "agent-alpha",
		SourceSession: "sess-1",
	})
	if err != nil {
		t.Fatalf("seed subscope log: %v", err)
	}

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. GET /api/memories (global, all children)
	resp, err := client.Get(srv.URL() + "/api/memories?scope=global")
	if err != nil {
		t.Fatalf("GET /api/memories error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var data struct {
		OK       bool       `json:"ok"`
		Memories []UIMemory `json:"memories"`
		Total    int64      `json:"total"`
		Limit    int        `json:"limit"`
		Offset   int        `json:"offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode memories json: %v", err)
	}
	if !data.OK || data.Total != 3 || len(data.Memories) != 3 {
		t.Fatalf("expected total 3, got total %d, len %d", data.Total, len(data.Memories))
	}

	// 2. GET /api/memories with direct scope (children=false)
	respDirect, err := client.Get(srv.URL() + "/api/memories?scope=project:webapp&children=false")
	if err != nil {
		t.Fatalf("GET /api/memories direct error: %v", err)
	}
	defer respDirect.Body.Close()
	var directData struct {
		OK       bool       `json:"ok"`
		Memories []UIMemory `json:"memories"`
		Total    int64      `json:"total"`
	}
	_ = json.NewDecoder(respDirect.Body).Decode(&directData)
	if directData.Total != 2 {
		t.Errorf("expected 2 direct memories in project:webapp, got %d", directData.Total)
	}

	// 3. GET /api/memories with type filter (type=fact)
	respFact, err := client.Get(srv.URL() + "/api/memories?scope=global&type=fact")
	if err != nil {
		t.Fatalf("GET /api/memories type=fact error: %v", err)
	}
	defer respFact.Body.Close()
	var factData struct {
		OK       bool       `json:"ok"`
		Memories []UIMemory `json:"memories"`
		Total    int64      `json:"total"`
	}
	_ = json.NewDecoder(respFact.Body).Decode(&factData)
	if factData.Total != 1 || factData.Memories[0].Type != "fact" || factData.Memories[0].Key != "api_version" {
		t.Errorf("expected 1 fact memory with key 'api_version', got %+v", factData)
	}

	// 4. GET /api/memories with tag filter (tags=density)
	respTag, err := client.Get(srv.URL() + "/api/memories?scope=global&tags=density")
	if err != nil {
		t.Fatalf("GET /api/memories tags=density error: %v", err)
	}
	defer respTag.Body.Close()
	var tagData struct {
		OK       bool       `json:"ok"`
		Memories []UIMemory `json:"memories"`
		Total    int64      `json:"total"`
	}
	_ = json.NewDecoder(respTag.Body).Decode(&tagData)
	if tagData.Total != 1 || tagData.Memories[0].ID != noteID {
		t.Errorf("expected noteID %d, got %+v", noteID, tagData)
	}

	// 5. GET /api/memories with agent filter (agent=agent-alpha)
	respAgent, err := client.Get(srv.URL() + "/api/memories?scope=global&agent=agent-alpha")
	if err != nil {
		t.Fatalf("GET /api/memories agent=agent-alpha error: %v", err)
	}
	defer respAgent.Body.Close()
	var agentData struct {
		OK       bool       `json:"ok"`
		Memories []UIMemory `json:"memories"`
		Total    int64      `json:"total"`
	}
	_ = json.NewDecoder(respAgent.Body).Decode(&agentData)
	if agentData.Total != 2 {
		t.Errorf("expected 2 memories for agent-alpha, got %d", agentData.Total)
	}

	// 6. GET /api/memories with search query (?q=density)
	respSearch, err := client.Get(srv.URL() + "/api/memories?scope=global&q=density")
	if err != nil {
		t.Fatalf("GET /api/memories q=density error: %v", err)
	}
	defer respSearch.Body.Close()
	var searchData struct {
		OK       bool       `json:"ok"`
		Memories []UIMemory `json:"memories"`
		Total    int64      `json:"total"`
	}
	_ = json.NewDecoder(respSearch.Body).Decode(&searchData)
	if !searchData.OK || len(searchData.Memories) == 0 {
		t.Fatalf("expected search match for 'density', got %+v", searchData)
	}
	foundNote := false
	for _, m := range searchData.Memories {
		if m.ID == noteID {
			foundNote = true
			if m.Score == nil {
				t.Errorf("expected search result to contain score")
			}
			break
		}
	}
	if !foundNote {
		t.Errorf("expected noteID %d in search results, got %+v", noteID, searchData.Memories)
	}

	// 7. GET /api/memories/:id (valid ID)
	respDetail, err := client.Get(fmt.Sprintf("%s/api/memories/%d", srv.URL(), factID))
	if err != nil {
		t.Fatalf("GET /api/memories/:id error: %v", err)
	}
	defer respDetail.Body.Close()
	if respDetail.StatusCode != http.StatusOK {
		t.Errorf("detail status = %d, want 200", respDetail.StatusCode)
	}
	var detailData struct {
		OK     bool     `json:"ok"`
		Memory UIMemory `json:"memory"`
	}
	_ = json.NewDecoder(respDetail.Body).Decode(&detailData)
	if !detailData.OK || detailData.Memory.ID != factID || detailData.Memory.Key != "api_version" {
		t.Errorf("expected fact memory detail, got %+v", detailData)
	}

	// 8. GET /api/memories/:id (not found ID)
	respNotFound, err := client.Get(srv.URL() + "/api/memories/999999")
	if err != nil {
		t.Fatalf("GET /api/memories/999999 error: %v", err)
	}
	defer respNotFound.Body.Close()
	if respNotFound.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", respNotFound.StatusCode)
	}

	// 9. GET /api/memories/:id (invalid ID)
	respInvalidID, err := client.Get(srv.URL() + "/api/memories/abc")
	if err != nil {
		t.Fatalf("GET /api/memories/abc error: %v", err)
	}
	defer respInvalidID.Body.Close()
	if respInvalidID.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", respInvalidID.StatusCode)
	}
}

func TestServer_StatsAPI(t *testing.T) {
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	_, _, _ = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:backend",
		Type:    "note",
		Content: "Backend architecture note",
	})
	_, _, _ = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:backend",
		Type:    "log",
		Content: "Deployment log",
	})
	_, _, _ = st.SetFact(ctx, store.FactInput{
		Scope: "project:backend",
		Key:   "port",
		Value: `8080`,
	})
	_, _, _ = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:frontend",
		Type:    "note",
		Content: "UI notes",
	})

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. GET /api/stats (overall)
	resp, err := client.Get(srv.URL() + "/api/stats")
	if err != nil {
		t.Fatalf("GET /api/stats error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/stats status = %d, want 200", resp.StatusCode)
	}

	var statsData struct {
		OK    bool `json:"ok"`
		Stats struct {
			Memories         int64            `json:"memories"`
			ByType           map[string]int64 `json:"by_type"`
			ByScope          map[string]int64 `json:"by_scope"`
			PendingEmbedding int64            `json:"pending_embedding"`
			DBSizeMB         float64          `json:"db_size_mb"`
			LastCompactAt    *int64           `json:"last_compact_at"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statsData); err != nil {
		t.Fatalf("decode stats: %v", err)
	}

	if !statsData.OK || statsData.Stats.Memories != 4 {
		t.Errorf("expected 4 memories, got %+v", statsData)
	}
	if statsData.Stats.ByType["note"] != 2 || statsData.Stats.ByType["fact"] != 1 || statsData.Stats.ByType["log"] != 1 {
		t.Errorf("unexpected by_type counts: %+v", statsData.Stats.ByType)
	}

	// 2. GET /api/stats?scope=project:backend
	respScoped, err := client.Get(srv.URL() + "/api/stats?scope=project:backend")
	if err != nil {
		t.Fatalf("GET /api/stats?scope=project:backend error: %v", err)
	}
	defer respScoped.Body.Close()

	var scopedData struct {
		OK    bool `json:"ok"`
		Stats struct {
			Memories       int64            `json:"memories"`
			Scope          string           `json:"scope"`
			ScopedMemories int64            `json:"scoped_memories"`
			ScopedByType   map[string]int64 `json:"scoped_by_type"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(respScoped.Body).Decode(&scopedData); err != nil {
		t.Fatalf("decode scoped stats: %v", err)
	}
	if scopedData.Stats.ScopedMemories != 3 {
		t.Errorf("expected 3 scoped memories for project:backend, got %d", scopedData.Stats.ScopedMemories)
	}
	if scopedData.Stats.ScopedByType["note"] != 1 || scopedData.Stats.ScopedByType["fact"] != 1 || scopedData.Stats.ScopedByType["log"] != 1 {
		t.Errorf("unexpected scoped_by_type: %+v", scopedData.Stats.ScopedByType)
	}
}

func TestServer_HealthAPI_DoctorChecks(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatalf("chmod test dir: %v", err)
	}
	stCfg := config.Config{
		Home:   dir,
		DBPath: dir + "/centmem.db",
	}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
		Config:  stCfg,
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Get(srv.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health error: %v", err)
	}
	defer resp.Body.Close()

	var healthData struct {
		OK       bool          `json:"ok"`
		Status   string        `json:"status"`
		Store    string        `json:"store"`
		Checks   []DoctorCheck `json:"checks"`
		Warnings []string      `json:"warnings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
		t.Fatalf("decode health data: %v", err)
	}

	if healthData.Store != "connected" {
		t.Errorf("expected store 'connected', got %s", healthData.Store)
	}
	if len(healthData.Checks) == 0 {
		t.Errorf("expected non-empty doctor checks list")
	}

	checkNames := map[string]string{}
	for _, c := range healthData.Checks {
		checkNames[c.Name] = c.Status
	}

	for _, reqCheck := range []string{"integrity", "schema_version", "extensions", "embed_queue", "permissions"} {
		if status, exists := checkNames[reqCheck]; !exists || status != "ok" {
			t.Errorf("expected check %q to be 'ok', got exists=%v status=%q", reqCheck, exists, status)
		}
	}
}

func TestServer_ForgetMemory(t *testing.T) {
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	id, _, err := st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:actions",
		Type:    "note",
		Content: "temporary secret to forget",
	})
	if err != nil {
		t.Fatalf("put memory: %v", err)
	}

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Invalid ID -> 400
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/memories/invalid/forget", srv.URL()), nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST forget invalid id: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid id, got %d", resp.StatusCode)
	}

	// 2. Forget valid ID -> 200
	req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/memories/%d/forget", srv.URL(), id), nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST forget valid id: %v", err)
	}
	var forgetData map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&forgetData)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for forget, got %d", resp.StatusCode)
	}
	if forgetData["ok"] != true || int(forgetData["deleted"].(float64)) != 1 {
		t.Errorf("unexpected forget payload: %+v", forgetData)
	}

	// 3. Forget already deleted ID -> 404
	req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/memories/%d/forget", srv.URL(), id), nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST forget already deleted: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 on re-forget, got %d", resp.StatusCode)
	}

	// 4. Verify memory is gone from store
	_, err = st.GetMemory(ctx, id)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound from store, got %v", err)
	}
}

func TestServer_RestoreMemory(t *testing.T) {
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	body := []byte(`{"scope":"project:undo","type":"note","content":"restored content","tags":["undo-tag"],"source_agent":"agent-test"}`)
	resp, err := client.Post(srv.URL()+"/api/memories", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/memories error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d", resp.StatusCode)
	}

	var res map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if res["ok"] != true || res["id"] == nil {
		t.Errorf("unexpected restore response: %+v", res)
	}

	id := int64(res["id"].(float64))
	m, err := st.GetMemory(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to retrieve created memory %d: %v", id, err)
	}
	if m.Content != "restored content" || m.ScopePath != "project:undo" {
		t.Errorf("mismatched restored memory: %+v", m)
	}
}

func TestServer_ExportAPI_JSON_and_CSV(t *testing.T) {
	dir := t.TempDir()
	stCfg := config.Config{DBPath: dir + "/centmem.db"}
	st, err := store.Open(stCfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	_, _, _ = st.PutMemory(ctx, store.MemoryInput{
		Scope:   "project:export/agent:crawler",
		Type:    "note",
		Content: "crawler note",
		Tags:    []string{"scrape", "data"},
	})
	_, _, _ = st.PutMemory(ctx, store.MemoryInput{
		Scope:     "project:export/agent:crawler",
		Type:      "fact",
		Key:       "endpoint.url",
		ValueJSON: `{"url":"https://example.com"}`,
		Tags:      []string{"config"},
	})

	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    0,
		NoOpen:  true,
		Version: "1.4.0",
		Store:   st,
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Test JSON export
	respJSON, err := client.Get(srv.URL() + "/api/export?scope=project:export&format=json")
	if err != nil {
		t.Fatalf("GET /api/export json error: %v", err)
	}
	defer respJSON.Body.Close()

	if respJSON.StatusCode != http.StatusOK {
		t.Errorf("export JSON status = %d, want 200", respJSON.StatusCode)
	}
	cd := respJSON.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".json") {
		t.Errorf("expected Content-Disposition attachment with .json, got: %q", cd)
	}

	var jsonExport struct {
		OK       bool       `json:"ok"`
		Total    int        `json:"total"`
		Memories []UIMemory `json:"memories"`
	}
	if err := json.NewDecoder(respJSON.Body).Decode(&jsonExport); err != nil {
		t.Fatalf("decode json export: %v", err)
	}
	if !jsonExport.OK || jsonExport.Total != 2 || len(jsonExport.Memories) != 2 {
		t.Errorf("unexpected json export total: %d (expected 2)", jsonExport.Total)
	}

	// 2. Test CSV export
	respCSV, err := client.Get(srv.URL() + "/api/export?scope=project:export&format=csv")
	if err != nil {
		t.Fatalf("GET /api/export csv error: %v", err)
	}
	defer respCSV.Body.Close()

	if respCSV.StatusCode != http.StatusOK {
		t.Errorf("export CSV status = %d, want 200", respCSV.StatusCode)
	}
	cdCSV := respCSV.Header.Get("Content-Disposition")
	if !strings.Contains(cdCSV, "attachment") || !strings.Contains(cdCSV, ".csv") {
		t.Errorf("expected Content-Disposition attachment with .csv, got: %q", cdCSV)
	}

	csvBody, _ := io.ReadAll(respCSV.Body)
	csvLines := strings.Split(strings.TrimSpace(string(csvBody)), "\n")
	if len(csvLines) != 3 { // 1 header + 2 rows
		t.Errorf("expected 3 CSV lines, got %d:\n%s", len(csvLines), string(csvBody))
	}
	if !strings.HasPrefix(csvLines[0], "id,scope,type,content,key,value_json,tags") {
		t.Errorf("unexpected CSV header: %s", csvLines[0])
	}

	// 3. Test filter by type on export
	respFiltered, err := client.Get(srv.URL() + "/api/export?scope=project:export&type=fact&format=json")
	if err != nil {
		t.Fatalf("GET /api/export filtered error: %v", err)
	}
	defer respFiltered.Body.Close()

	var filteredExport struct {
		Total    int        `json:"total"`
		Memories []UIMemory `json:"memories"`
	}
	_ = json.NewDecoder(respFiltered.Body).Decode(&filteredExport)
	if filteredExport.Total != 1 || filteredExport.Memories[0].Type != "fact" {
		t.Errorf("expected 1 fact memory, got %d", filteredExport.Total)
	}
}
