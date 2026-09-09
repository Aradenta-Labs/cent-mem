package capture_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

// 1. Heuristic Classifier Tests

func TestHeuristic_AllSevenCategories(t *testing.T) {
	cfg := capture.DefaultCaptureConfig()
	cfg.ConfidenceThreshold = 0.5
	classifier := capture.NewHeuristicClassifier(cfg)

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We decided to implement three-tier classification priority.", Timestamp: time.Now()},
		{Role: "assistant", Content: "I prefer YAML and TOML over XML.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Code implementation:\n```go\nfunc Classify() {}\n```", Timestamp: time.Now()},
		{Role: "assistant", Content: "Implemented retry on malformed JSON.", Timestamp: time.Now()},
		{Role: "assistant", Content: "The root cause was an unhandled EOF on empty pipe.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Run go get github.com/pelletier/go-toml/v2 to parse TOML.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Check documentation at https://github.com/aradenta-labs/cent-mem/docs.", Timestamp: time.Now()},
	}

	items, err := classifier.Classify(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}

	found := make(map[string]capture.CaptureItem)
	for _, item := range items {
		found[item.Category] = item
		if item.Confidence != 0.75 {
			t.Errorf("expected confidence 0.75 for heuristic item %s, got %f", item.Category, item.Confidence)
		}
	}

	expected := []string{"decision", "preference", "code", "log", "error", "dependency", "fact"}
	for _, cat := range expected {
		if _, ok := found[cat]; !ok {
			t.Errorf("expected category %q was not found in heuristic output", cat)
		}
	}
}

func TestHeuristic_KeySanitization(t *testing.T) {
	cfg := capture.DefaultCaptureConfig()
	cfg.ConfidenceThreshold = 0.5
	classifier := capture.NewHeuristicClassifier(cfg)

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "See https://api.openai.com/v1/models?foo=bar for available models.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Run npm install @anthropic-ai/sdk for TypeScript.", Timestamp: time.Now()},
		{Role: "assistant", Content: "database_url = postgres://user:pass@localhost:5432/db", Timestamp: time.Now()},
	}

	items, err := classifier.Classify(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}

	var hasURLKey, hasDepKey, hasFactKV bool
	for _, item := range items {
		if strings.HasPrefix(item.Key, "url.") {
			hasURLKey = true
			if strings.Contains(item.Key, "https://") || strings.Contains(item.Key, "/") {
				t.Errorf("url key not properly sanitized: %q", item.Key)
			}
		}
		if strings.HasPrefix(item.Key, "dep.") {
			hasDepKey = true
			if strings.Contains(item.Key, "/") {
				t.Errorf("dep key has slashes: %q", item.Key)
			}
		}
		if item.Key == "database_url" {
			hasFactKV = true
		}
	}

	if !hasURLKey {
		t.Error("expected sanitized url key")
	}
	if !hasDepKey {
		t.Error("expected sanitized dep key")
	}
	if !hasFactKV {
		t.Error("expected key=value pair parsed")
	}
}

func TestHeuristic_EmptyAndWhitespace(t *testing.T) {
	cfg := capture.DefaultCaptureConfig()
	classifier := capture.NewHeuristicClassifier(cfg)

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "   \n\t  "},
		{Role: "assistant", Content: ""},
	}

	items, err := classifier.Classify(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Classify should not error on whitespace: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

// 2. Local LLM Classifier Tests

func TestLocalLLM_Success(t *testing.T) {
	mockJSON := `{"choices": [{"message": {"content": "[{\"category\":\"decision\",\"content\":\"Use bge-small-en-v1.5 embeddings\",\"confidence\":0.92}]"}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL
	cfg.LocalLLMModel = "llama3.2"

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We should use bge-small-en-v1.5 embeddings for fast local inference."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}
	if len(items) != 1 || items[0].Category != "decision" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestLocalLLM_MalformedJSON_RetrySuccess(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			// First call: return corrupted / malformed JSON
			w.Write([]byte(`{"choices": [{"message": {"content": "INVALID JSON HERE..."}}]}`))
			return
		}
		// Second call (retry): return valid JSON array
		w.Write([]byte("{\"choices\": [{\"message\": {\"content\": \"```json\\n[{\\\"category\\\":\\\"decision\\\",\\\"content\\\":\\\"Retry worked\\\",\\\"confidence\\\":0.88}]\\n```\"}}]}"))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Testing retry resilience."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed after retry: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 calls (1 failure + 1 retry), got %d", atomic.LoadInt32(&callCount))
	}
	if len(items) != 1 || items[0].Content != "Retry worked" {
		t.Errorf("unexpected items after retry: %+v", items)
	}
}

func TestLocalLLM_MalformedJSON_ExhaustedFallback(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		// Always return invalid content
		w.Write([]byte(`{"choices": [{"message": {"content": "{not valid json}"}}]}`))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We decided to fall back to heuristic when model is corrupt."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig should fall back gracefully, got error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 calls before fallback, got %d", atomic.LoadInt32(&callCount))
	}
	if len(items) == 0 || items[0].Category != "decision" {
		t.Fatalf("expected fallback to heuristic items, got %+v", items)
	}
}

func TestLocalLLM_EndpointOffline_FallbackToHeuristic(t *testing.T) {
	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = "http://127.0.0.1:54329/offline"
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We decided on offline fallback behavior."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("expected graceful fallback, got: %v", err)
	}
	if len(items) == 0 || items[0].Category != "decision" {
		t.Fatalf("expected heuristic fallback, got: %+v", items)
	}
}

func TestLocalLLM_500InternalError_FallbackToHeuristic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We chose graceful degradation on 500 errors."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("expected graceful fallback on 500 error: %v", err)
	}
	if len(items) == 0 || items[0].Category != "decision" {
		t.Fatalf("expected heuristic items, got: %+v", items)
	}
}

// 3. OpenAI-Compatible Classifier Tests

func TestOpenAICompatible_Success(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices": [{"message": {"content": "[{\"category\":\"fact\",\"content\":\"Port 8080\",\"key\":\"server.port\",\"confidence\":0.95}]"}}]}`))
	}))
	defer server.Close()

	t.Setenv("TEST_OPENAI_KEY", "sk-mock-12345")

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "openai-compatible"
	cfg.APIBaseURL = server.URL
	cfg.APIKeyEnv = "TEST_OPENAI_KEY"
	cfg.APIModel = "gpt-4o-mini"

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Server listens on port 8080."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}
	if authHeader != "Bearer sk-mock-12345" {
		t.Errorf("expected Authorization header 'Bearer sk-mock-12345', got %q", authHeader)
	}
	if len(items) != 1 || items[0].Key != "server.port" {
		t.Errorf("unexpected items: %+v", items)
	}
}

func TestOpenAICompatible_DirectAPIKey(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `[{"category":"fact","key":"api.provider","content":"Using direct key","confidence":0.95}]`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "openai-compatible"
	cfg.APIBaseURL = server.URL
	cfg.APIKeyEnv = "sk-f7c1cf87505b4556-yi1kjc-2435b0d4" // User's direct key format
	cfg.APIModel = "deepseek-v4-flash"

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We are configuring direct provider keys."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}
	if authHeader != "Bearer sk-f7c1cf87505b4556-yi1kjc-2435b0d4" {
		t.Errorf("expected Authorization header with direct key, got %q", authHeader)
	}
	if len(items) != 1 || items[0].Key != "api.provider" {
		t.Errorf("unexpected items: %+v", items)
	}
}

func TestOpenAICompatible_MissingAPIKeyEnv_Fallback(t *testing.T) {
	os.Unsetenv("TEST_UNSET_KEY_VAR")

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "openai-compatible"
	cfg.APIKeyEnv = "TEST_UNSET_KEY_VAR"
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We decided to keep API keys in environment variables only."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("expected instant fallback without error: %v", err)
	}
	if len(items) == 0 || items[0].Category != "decision" {
		t.Errorf("expected heuristic items on missing key, got: %+v", items)
	}
}

func TestOpenAICompatible_AuthError_Fallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	t.Setenv("TEST_INVALID_KEY", "sk-invalid")

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "openai-compatible"
	cfg.APIBaseURL = server.URL
	cfg.APIKeyEnv = "TEST_INVALID_KEY"
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We chose fallback on 401 unauthorized."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("expected fallback on auth failure: %v", err)
	}
	if len(items) == 0 || items[0].Category != "decision" {
		t.Errorf("expected heuristic fallback on 401, got: %+v", items)
	}
}

// 4. Prompt Engine & Template Customization Tests

func TestPrompt_DefaultInterpolation(t *testing.T) {
	t.Setenv("CENTMEM_HOME", t.TempDir())
	cfg := capture.DefaultCaptureConfig()
	cfg.Categories = []string{"decision", "code", "fact"}
	cfg.ConfidenceThreshold = 0.85

	snippets := []string{
		"Decision: Use WAL mode in SQLite",
		"Fact: port = 8080",
	}

	prompt := capture.BuildPrompt(cfg, snippets)

	if !strings.Contains(prompt, "decision, code, fact") {
		t.Errorf("prompt missing categories interpolation: %s", prompt)
	}
	if !strings.Contains(prompt, "0.85") {
		t.Errorf("prompt missing confidence threshold interpolation: %s", prompt)
	}
	if !strings.Contains(prompt, "- Decision: Use WAL mode in SQLite") {
		t.Errorf("prompt missing recall snippets: %s", prompt)
	}
}

func TestPrompt_CustomTemplateFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CENTMEM_HOME", tempHome)

	customTemplate := "CUSTOM CLASSIFIER PROMPT: Categories={{CATEGORIES}}, Threshold={{CONFIDENCE_THRESHOLD}}, Context={{RECALL_CONTEXT}}"
	if err := os.WriteFile(filepath.Join(tempHome, "capture-prompt.md"), []byte(customTemplate), 0644); err != nil {
		t.Fatalf("failed to write custom prompt: %v", err)
	}

	cfg := capture.DefaultCaptureConfig()
	cfg.Categories = []string{"decision", "preference"}
	cfg.ConfidenceThreshold = 0.75

	prompt := capture.BuildPrompt(cfg, nil)
	if !strings.HasPrefix(prompt, "CUSTOM CLASSIFIER PROMPT:") {
		t.Errorf("expected custom prompt template to be loaded, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Categories=decision, preference") {
		t.Errorf("expected custom prompt categories interpolation: %s", prompt)
	}
	if !strings.Contains(prompt, "Context=(none)") {
		t.Errorf("expected (none) for empty recall context: %s", prompt)
	}
}

// 5. Filtering & Deduplication Tests

func TestClassifyWithConfig_ConfidenceThresholdFilter(t *testing.T) {
	mockResponse := `{
		"choices": [{
			"message": {
				"content": "[{\"category\":\"decision\",\"content\":\"High conf\",\"confidence\":0.95},{\"category\":\"decision\",\"content\":\"Low conf\",\"confidence\":0.55}]"
			}
		}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL
	cfg.ConfidenceThreshold = 0.80

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Test message."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}
	if len(items) != 1 || items[0].Content != "High conf" {
		t.Fatalf("expected only High conf item to pass threshold, got: %+v", items)
	}
}

func TestClassifyWithConfig_CategoryWhitelist(t *testing.T) {
	mockResponse := `{
		"choices": [{
			"message": {
				"content": "[{\"category\":\"decision\",\"content\":\"Keep this\",\"confidence\":0.95},{\"category\":\"unwanted\",\"content\":\"Drop this\",\"confidence\":0.95}]"
			}
		}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL
	cfg.Categories = []string{"decision"}

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Test message."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}
	if len(items) != 1 || items[0].Category != "decision" {
		t.Fatalf("expected only whitelisted 'decision' item, got: %+v", items)
	}
}

func TestClassifyWithConfig_SkipItemsFiltered(t *testing.T) {
	mockResponse := `{
		"choices": [{
			"message": {
				"content": "[{\"category\":\"decision\",\"content\":\"Existing info\",\"confidence\":0.95,\"skip\":true,\"skip_reason\":\"already exists\"},{\"category\":\"decision\",\"content\":\"New info\",\"confidence\":0.95}]"
			}
		}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = server.URL

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Test message."},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}
	if len(items) != 1 || items[0].Content != "New info" {
		t.Fatalf("expected skip items to be filtered out, got: %+v", items)
	}
}
