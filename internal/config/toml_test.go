package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestConfig_TOML_MarshalUnmarshal(t *testing.T) {
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "config.toml")

	initial := config.Config{
		Home:   tempDir,
		DBPath: filepath.Join(tempDir, "centmem.db"),
		Model: config.ModelConfig{
			Name: "custom-model",
			Path: filepath.Join(tempDir, "models", "custom-model.onnx"),
			Dims: 512,
		},
		Retention: config.Retention{
			FactKeepDays:           0,
			NoteSummarizeAfterDays: 45,
			LogSummarizeAfterDays:  20,
			LogDropAfterDays:       40,
			ArchiveKeepDays:        180,
		},
		Capture: config.CaptureConfig{
			Enabled:             true,
			Harness:             "claude-code",
			Triggers:            []string{"message", "session-end"},
			Scope:               "project:alpha",
			Categories:          []string{"decision", "preference", "fact"},
			TranscriptPath:      "/tmp/transcript.jsonl",
			Backend:             "local-llm",
			LocalLLMEndpoint:    "http://127.0.0.1:11434/v1",
			LocalLLMModel:       "llama3.2",
			APIBaseURL:          "https://api.openai.com/v1",
			APIKeyEnv:           "OPENAI_API_KEY",
			APIModel:            "gpt-4o-mini",
			ConfidenceThreshold: 0.8,
		},
	}

	if err := config.SaveTOML(tomlPath, initial); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}

	loaded, err := config.LoadTOML(tomlPath)
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}

	if loaded.Model.Name != "custom-model" || loaded.Model.Dims != 512 {
		t.Errorf("mismatched Model: %+v", loaded.Model)
	}
	if loaded.Retention.NoteSummarizeAfterDays != 45 || loaded.Retention.ArchiveKeepDays != 180 {
		t.Errorf("mismatched Retention: %+v", loaded.Retention)
	}
	if !loaded.Capture.Enabled || loaded.Capture.Harness != "claude-code" || loaded.Capture.ConfidenceThreshold != 0.8 {
		t.Errorf("mismatched Capture: %+v", loaded.Capture)
	}
	if len(loaded.Capture.Categories) != 3 || loaded.Capture.Categories[0] != "decision" {
		t.Errorf("mismatched Categories: %+v", loaded.Capture.Categories)
	}
}

func TestConfig_TOML_HeaderAndComments(t *testing.T) {
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "config.toml")

	cfg := config.Config{
		Home:      tempDir,
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
	}

	if err := config.SaveTOML(tomlPath, cfg); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}

	content, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "# cent-mem configuration file") {
		t.Errorf("missing header comment in TOML file: %s", s)
	}
	if !strings.Contains(s, "Never write raw API keys") {
		t.Errorf("missing API key security notice in TOML file: %s", s)
	}
}

func TestConfig_TOML_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "config.toml")

	cfg := config.Config{
		Home:      tempDir,
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
	}

	if err := config.SaveTOML(tomlPath, cfg); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}

	info, err := os.Stat(tomlPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if os.PathSeparator == '/' {
		mode := info.Mode().Perm()
		if mode != 0600 {
			t.Errorf("expected permissions 0600, got %o", mode)
		}
	}
}

func TestConfig_TOML_DefaultsFallback(t *testing.T) {
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "empty.toml")

	// Write empty file
	if err := os.WriteFile(tomlPath, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	loaded, err := config.LoadTOML(tomlPath)
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}

	if loaded.Retention.NoteSummarizeAfterDays != 30 {
		t.Errorf("expected default retention note_summarize_after_days=30, got %d", loaded.Retention.NoteSummarizeAfterDays)
	}
	if loaded.Capture.ConfidenceThreshold != 0.7 {
		t.Errorf("expected default capture confidence_threshold=0.7, got %f", loaded.Capture.ConfidenceThreshold)
	}
}

func TestConfig_TOML_EnvPrecedence(t *testing.T) {
	os.Clearenv()
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "config.toml")

	fileCfg := config.Config{
		Home:      tempDir,
		Retention: config.DefaultRetention(),
		Capture: config.CaptureConfig{
			Enabled:             false,
			Harness:             "cursor",
			Backend:             "heuristic",
			ConfidenceThreshold: 0.6,
		},
	}
	if err := config.SaveTOML(tomlPath, fileCfg); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}

	os.Setenv("CENTMEM_HOME", tempDir)
	os.Setenv("CENTMEM_CAPTURE_HARNESS", "antigravity")
	os.Setenv("CENTMEM_CAPTURE_ENABLED", "true")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !loaded.Capture.Enabled {
		t.Errorf("expected env override Enabled=true")
	}
	if loaded.Capture.Harness != "antigravity" {
		t.Errorf("expected env override Harness=antigravity, got %q", loaded.Capture.Harness)
	}
	// Untouched in env, should come from TOML
	if loaded.Capture.ConfidenceThreshold != 0.6 {
		t.Errorf("expected TOML ConfidenceThreshold=0.6, got %f", loaded.Capture.ConfidenceThreshold)
	}
}
