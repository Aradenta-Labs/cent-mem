package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuth_NonLoopbackWithoutToken(t *testing.T) {
	_, err := NewServer(ServerConfig{
		Host: "0.0.0.0",
		Port: 0,
	})
	if err == nil {
		t.Fatal("expected error when binding to 0.0.0.0 without token, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to bind to non-loopback host without authentication token") {
		t.Errorf("unexpected error message: %v", err)
	}

	_, err = NewServer(ServerConfig{
		Host: "192.168.1.50",
		Port: 0,
	})
	if err == nil {
		t.Fatal("expected error when binding to 192.168.1.50 without token, got nil")
	}
}

func TestAuth_MinimumEntropy(t *testing.T) {
	_, err := NewServer(ServerConfig{
		Host:  "127.0.0.1",
		Port:  0,
		Token: "short-token",
	})
	if err == nil {
		t.Fatal("expected error when token is less than 16 chars, got nil")
	}
	if !strings.Contains(err.Error(), "token must be at least 16 characters long") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAuth_LoopbackWithoutTokenAllowed(t *testing.T) {
	srv, err := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("expected loopback without token to succeed, got %v", err)
	}
	if srv == nil {
		t.Fatal("expected server instance, got nil")
	}
}

func TestAuth_BearerTokenValidation(t *testing.T) {
	validToken := "sec_9a8f2bc0e194871da938b"
	srv, err := NewServer(ServerConfig{
		Host:  "127.0.0.1",
		Port:  0,
		Token: validToken,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := srv.httpServer.Handler

	// 1. Missing Authorization header on /api/health
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for missing token, got %d", rec.Code)
	}
	var errResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if errResp["ok"] != false || !strings.Contains(errResp["error"].(string), "unauthorized") {
		t.Errorf("expected unauthorized error body, got: %v", errResp)
	}

	// 2. Incorrect token
	req = httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-value-123")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for invalid token, got %d", rec.Code)
	}

	// 3. Valid Bearer token
	req = httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for valid token, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// 4. Static assets (SPA fallback) accessible without token
	req = httptest.NewRequest("GET", "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected root path to be accessible without token, got 401")
	}

	// 5. CORS OPTIONS preflight
	req = httptest.NewRequest("OPTIONS", "/api/health", nil)
	req.Header.Set("Origin", "http://example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for OPTIONS preflight, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("expected CORS origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// 6. Valid token via query param fallback
	req = httptest.NewRequest("GET", "/api/health?token="+validToken, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for token via query param, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}
