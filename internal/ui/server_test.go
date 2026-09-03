package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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
