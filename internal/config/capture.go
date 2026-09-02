package config

import (
	"fmt"
	"strings"
)

// CaptureConfig defines configuration options for transcript auto-capture.
type CaptureConfig struct {
	Enabled             bool     `json:"enabled" toml:"enabled"`
	Harness             string   `json:"harness" toml:"harness"`
	Triggers            []string `json:"triggers" toml:"triggers"`
	Scope               string   `json:"scope" toml:"scope"`
	Categories          []string `json:"categories" toml:"categories"`
	TranscriptPath      string   `json:"transcript_path" toml:"transcript_path"`
	Backend             string   `json:"backend" toml:"backend"` // "local-llm" | "heuristic" | "openai-compatible"
	LocalLLMEndpoint    string   `json:"local_llm_endpoint" toml:"local_llm_endpoint"`
	LocalLLMModel       string   `json:"local_llm_model" toml:"local_llm_model"`
	APIBaseURL          string   `json:"api_base_url" toml:"api_base_url"`
	APIKeyEnv           string   `json:"api_key_env" toml:"api_key_env"`
	APIModel            string   `json:"api_model" toml:"api_model"`
	ConfidenceThreshold float64  `json:"confidence_threshold" toml:"confidence_threshold"`
}

// DefaultCategories returns the default list of memory capture categories.
func DefaultCategories() []string {
	return []string{
		"decision",
		"fact",
		"preference",
		"code",
		"log",
		"error",
		"dependency",
	}
}

// DefaultCaptureConfig returns the default configuration for capture.
func DefaultCaptureConfig() CaptureConfig {
	return CaptureConfig{
		Enabled:             false,
		Harness:             "antigravity",
		Triggers:            []string{"message", "session-end", "on-demand"},
		Scope:               "global",
		Categories:          DefaultCategories(),
		Backend:             "heuristic",
		ConfidenceThreshold: 0.7,
		LocalLLMEndpoint:    "http://localhost:11434/v1",
		LocalLLMModel:       "llama3.2",
		APIBaseURL:          "https://api.openai.com/v1",
		APIKeyEnv:           "OPENAI_API_KEY",
		APIModel:            "gpt-4o-mini",
	}
}

// ValidateCaptureConfig validates that capture configuration parameters are well-formed.
func ValidateCaptureConfig(c CaptureConfig) error {
	if c.ConfidenceThreshold < 0.0 || c.ConfidenceThreshold > 1.0 {
		return fmt.Errorf("capture config: invalid confidence_threshold=%f: must be between 0.0 and 1.0", c.ConfidenceThreshold)
	}
	switch strings.ToLower(strings.TrimSpace(c.Backend)) {
	case "", "heuristic", "local-llm", "openai-compatible":
		// valid
	default:
		return fmt.Errorf("capture config: unknown backend %q (expected heuristic, local-llm, or openai-compatible)", c.Backend)
	}
	return nil
}
