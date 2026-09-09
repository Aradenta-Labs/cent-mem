package capture_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestHeuristicClassifier_AllCategories(t *testing.T) {
	cfg := capture.DefaultCaptureConfig()
	cfg.ConfidenceThreshold = 0.5
	classifier := capture.NewHeuristicClassifier(cfg)

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We decided to use SQLite-vec for local vector search.", Timestamp: time.Now()},
		{Role: "assistant", Content: "I prefer tabs over spaces for Go files.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Here is the snippet:\n```go\nfunc hello() string {\n  return \"world\"\n}\n```", Timestamp: time.Now()},
		{Role: "assistant", Content: "Completed database migration step 1.", Timestamp: time.Now()},
		{Role: "assistant", Content: "The root cause of the crash was a nil pointer in Store.Open.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Run go get github.com/mattn/go-sqlite3 to install the driver.", Timestamp: time.Now()},
		{Role: "assistant", Content: "Documentation is available at https://github.com/aradenta-labs/cent-mem.", Timestamp: time.Now()},
	}

	items, err := classifier.Classify(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}

	categoriesFound := make(map[string]bool)
	for _, item := range items {
		categoriesFound[item.Category] = true
	}

	expectedCategories := []string{"decision", "preference", "code", "log", "error", "dependency", "fact"}
	for _, expected := range expectedCategories {
		if !categoriesFound[expected] {
			t.Errorf("expected category %q was not detected by heuristic classifier", expected)
		}
	}
}

func TestLocalLLMClassifier_MockSuccess(t *testing.T) {
	mockResponse := `{
		"choices": [{
			"message": {
				"content": "[{\"category\":\"decision\",\"content\":\"Use WAL mode\",\"confidence\":0.95}]"
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
	cfg.LocalLLMModel = "mock-model"

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Let's turn on WAL mode for SQLite.", Timestamp: time.Now()},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Category != "decision" || items[0].Content != "Use WAL mode" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestLocalLLMClassifier_FallbackToHeuristic(t *testing.T) {
	// Point to an unavailable endpoint to trigger fallback
	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "local-llm"
	cfg.LocalLLMEndpoint = "http://127.0.0.1:54321/unreachable"
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We decided to migrate to version 2.", Timestamp: time.Now()},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig should fallback gracefully without error: %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("expected fallback heuristic items, got 0")
	}
	if items[0].Category != "decision" {
		t.Errorf("expected category 'decision' from fallback, got %q", items[0].Category)
	}
}

func TestOpenAICompatibleClassifier_MissingKeyFallback(t *testing.T) {
	os.Unsetenv("TEST_MISSING_API_KEY")

	cfg := capture.DefaultCaptureConfig()
	cfg.Backend = "openai-compatible"
	cfg.APIKeyEnv = "TEST_MISSING_API_KEY"
	cfg.ConfidenceThreshold = 0.5

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "We chose the MIT license.", Timestamp: time.Now()},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}

	if len(items) == 0 || items[0].Category != "decision" {
		t.Errorf("expected fallback to heuristic classifier, got %+v", items)
	}
}

func TestClassifier_ConfidenceThresholdFilter(t *testing.T) {
	mockResponse := `{
		"choices": [{
			"message": {
				"content": "[{\"category\":\"decision\",\"content\":\"Item 1\",\"confidence\":0.9},{\"category\":\"decision\",\"content\":\"Item 2\",\"confidence\":0.4}]"
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
	cfg.ConfidenceThreshold = 0.7

	messages := []capture.TranscriptMessage{
		{Role: "user", Content: "Some conversation.", Timestamp: time.Now()},
	}

	items, err := capture.ClassifyWithConfig(context.Background(), messages, cfg, nil)
	if err != nil {
		t.Fatalf("ClassifyWithConfig failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item (high confidence only), got %d", len(items))
	}
	if items[0].Content != "Item 1" {
		t.Errorf("expected 'Item 1', got %q", items[0].Content)
	}
}

func TestPromptBuilder_RecallContext(t *testing.T) {
	t.Setenv("CENTMEM_HOME", t.TempDir())
	cfg := capture.DefaultCaptureConfig()
	snippets := []string{
		"Existing memory 1: port 8080",
		"Existing memory 2: WAL mode",
	}

	prompt := capture.BuildPrompt(cfg, snippets)
	if !strings.Contains(prompt, "Existing memory 1: port 8080") {
		t.Errorf("expected prompt to contain recall snippets, got: %s", prompt)
	}
	if !strings.Contains(prompt, "decision, fact") {
		t.Errorf("expected prompt to contain default categories, got: %s", prompt)
	}
}
