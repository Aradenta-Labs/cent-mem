package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestInit_Wizard_InteractiveComplete(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	// Answers to:
	// 1. Harness: claude-code
	// 2. Triggers: message,session-end
	// 3. Scope: project:my-app
	// 4. Categories: decision,preference,code
	// 5. Backend: heuristic
	// 6. Confidence: 0.85
	input := "claude-code\nmessage,session-end\nproject:my-app\ndecision,preference,code\nheuristic\n0.85\n"

	stdout, stderr, code := runCLIWithStdin(t, home, input, "init", "--wizard")
	if code != 0 {
		t.Fatalf("init with wizard failed (code %d): stdout=%s stderr=%s", code, stdout, stderr)
	}

	loaded, err := config.LoadTOML(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}

	if !loaded.Capture.Enabled {
		t.Errorf("expected capture enabled")
	}
	if loaded.Capture.Harness != "claude-code" {
		t.Errorf("expected harness claude-code, got %q", loaded.Capture.Harness)
	}
	if loaded.Capture.Scope != "project:my-app" {
		t.Errorf("expected scope project:my-app, got %q", loaded.Capture.Scope)
	}
	if loaded.Capture.ConfidenceThreshold != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", loaded.Capture.ConfidenceThreshold)
	}
	if len(loaded.Capture.Categories) != 3 || loaded.Capture.Categories[0] != "decision" {
		t.Errorf("unexpected categories: %+v", loaded.Capture.Categories)
	}

	// Verify capture-prompt.md exists
	promptPath := filepath.Join(home, "capture-prompt.md")
	promptContent, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("capture-prompt.md not found: %v", err)
	}
	if !strings.Contains(string(promptContent), "{{CATEGORIES}}") {
		t.Errorf("capture-prompt.md missing template variables")
	}
}

func TestInit_Wizard_DefaultsOnEnter(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	// Hit Enter on everything
	input := "\n\n\n\n\n\n"

	_, _, code := runCLIWithStdin(t, home, input, "init", "--wizard")
	if code != 0 {
		t.Fatalf("init with wizard failed (code %d)", code)
	}

	loaded, err := config.LoadTOML(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}

	if loaded.Capture.Harness != "antigravity" {
		t.Errorf("expected default harness antigravity, got %q", loaded.Capture.Harness)
	}
	if loaded.Capture.Scope != "global" {
		t.Errorf("expected default scope global, got %q", loaded.Capture.Scope)
	}
	if loaded.Capture.Backend != "heuristic" {
		t.Errorf("expected default backend heuristic, got %q", loaded.Capture.Backend)
	}
	if loaded.Capture.ConfidenceThreshold != 0.7 {
		t.Errorf("expected default confidence 0.7, got %f", loaded.Capture.ConfidenceThreshold)
	}
}

func TestInit_Wizard_Backend_LocalLLM(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	var in bytes.Buffer
	in.WriteString("trae\n")                               // harness
	in.WriteString("all\n")                                // triggers
	in.WriteString("project:local\n")                      // scope
	in.WriteString("\n")                                   // categories (defaults)
	in.WriteString("local-llm\n")                          // backend
	in.WriteString("http://192.168.1.50:11434/v1\n")       // endpoint
	in.WriteString("mistral-small\n")                      // model
	in.WriteString("0.75\n")                               // confidence

	_, _, code := runCLIWithStdin(t, home, in.String(), "init", "--wizard")
	if code != 0 {
		t.Fatalf("init with local-llm wizard failed (code %d)", code)
	}

	loaded, err := config.LoadTOML(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}

	if loaded.Capture.Backend != "local-llm" {
		t.Errorf("expected backend local-llm, got %q", loaded.Capture.Backend)
	}
	if loaded.Capture.LocalLLMEndpoint != "http://192.168.1.50:11434/v1" {
		t.Errorf("expected custom endpoint, got %q", loaded.Capture.LocalLLMEndpoint)
	}
	if loaded.Capture.LocalLLMModel != "mistral-small" {
		t.Errorf("expected custom model, got %q", loaded.Capture.LocalLLMModel)
	}
}

func TestInit_Wizard_Backend_OpenAICompatible(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	var in bytes.Buffer
	in.WriteString("cursor\n")                          // harness
	in.WriteString("session-end\n")                     // triggers
	in.WriteString("project:cloud\n")                   // scope
	in.WriteString("\n")                                // categories
	in.WriteString("openai-compatible\n")               // backend
	in.WriteString("https://openrouter.ai/api/v1\n")    // api url
	in.WriteString("OPENROUTER_API_KEY\n")              // key env
	in.WriteString("anthropic/claude-3.5-sonnet\n")     // model
	in.WriteString("0.8\n")                             // confidence

	_, _, code := runCLIWithStdin(t, home, in.String(), "init", "--wizard")
	if code != 0 {
		t.Fatalf("init with openai wizard failed (code %d)", code)
	}

	loaded, err := config.LoadTOML(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}

	if loaded.Capture.Backend != "openai-compatible" {
		t.Errorf("expected backend openai-compatible, got %q", loaded.Capture.Backend)
	}
	if loaded.Capture.APIBaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected api base url, got %q", loaded.Capture.APIBaseURL)
	}
	if loaded.Capture.APIKeyEnv != "OPENROUTER_API_KEY" {
		t.Errorf("expected api key env var name, got %q", loaded.Capture.APIKeyEnv)
	}
	if loaded.Capture.APIModel != "anthropic/claude-3.5-sonnet" {
		t.Errorf("expected api model, got %q", loaded.Capture.APIModel)
	}
}

func TestInit_Wizard_NonInteractiveBypass(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	stdout, stderr, code := runCLI(t, home, "init", "--non-interactive")
	if code != 0 {
		t.Fatalf("init failed (exit %d): stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"ok":true`) {
		t.Errorf("expected JSON ok output: %s", stdout)
	}
}
