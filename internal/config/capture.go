package config

import (
	"fmt"
	"os"
	"strings"
)

// MCPServerConfig defines an external MCP server process command and arguments.
type MCPServerConfig struct {
	Name    string   `json:"name" toml:"name"`
	Command string   `json:"command" toml:"command"`
	Args    []string `json:"args" toml:"args"`
}

// CaptureMCPConfig holds MCP client enrichment settings.
type CaptureMCPConfig struct {
	Enabled bool              `json:"enabled" toml:"enabled"`
	Servers []MCPServerConfig `json:"servers" toml:"servers"`
	Tools   []string          `json:"tools" toml:"tools"`
}

// CaptureConfig defines configuration options for transcript auto-capture.
type CaptureConfig struct {
	Enabled             bool             `json:"enabled" toml:"enabled"`
	Harness             string           `json:"harness" toml:"harness"`
	Triggers            []string         `json:"triggers" toml:"triggers"`
	Scope               string           `json:"scope" toml:"scope"`
	Categories          []string         `json:"categories" toml:"categories"`
	TranscriptPath      string           `json:"transcript_path" toml:"transcript_path"`
	Backend             string           `json:"backend" toml:"backend"` // "local-llm" | "heuristic" | "openai-compatible"
	LocalLLMEndpoint    string           `json:"local_llm_endpoint" toml:"local_llm_endpoint"`
	LocalLLMModel       string           `json:"local_llm_model" toml:"local_llm_model"`
	APIBaseURL          string           `json:"api_base_url" toml:"api_base_url"`
	APIKeyEnv           string           `json:"api_key_env" toml:"api_key_env"`
	APIKey              string           `json:"api_key,omitempty" toml:"api_key,omitempty"`
	APIModel            string           `json:"api_model" toml:"api_model"`
	ConfidenceThreshold float64          `json:"confidence_threshold" toml:"confidence_threshold"`
	MCP                 CaptureMCPConfig `json:"mcp" toml:"mcp"`
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
		MCP: CaptureMCPConfig{
			Enabled: false,
			Servers: nil,
			Tools:   []string{"search_docs", "fetch_url"},
		},
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

// ResolveAPIKey extracts the actual secret token from a string which may either be:
// 1. The name of an environment variable (e.g. "OPENAI_API_KEY", "MY_API_KEY").
// 2. A direct API key token (e.g. "sk-...", "sk-proj-...", or any raw token).
//
// It returns the resolved API key and a boolean `fromEnv` indicating whether it
// was resolved from an environment variable (or was intended as an unset env var).
func ResolveAPIKey(keyOrEnv string) (apiKey string, fromEnv bool) {
	trimmed := strings.TrimSpace(keyOrEnv)
	if trimmed == "" {
		return "", false
	}

	// Clean up surrounding quotes if accidentally supplied (e.g. "sk-..." or 'sk-...')
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') ||
			(trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}

	// Clean up optional "Bearer " or "bearer " prefix if supplied
	if len(trimmed) > 7 && strings.EqualFold(trimmed[:7], "bearer ") {
		trimmed = strings.TrimSpace(trimmed[7:])
	}

	// Clean up surrounding quotes again if nested inside "Bearer ..."
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') ||
			(trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}

	// Clean up optional "$" or "${...}" prefix if supplied (e.g. $OPENAI_API_KEY or ${OPENAI_API_KEY})
	if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") {
		trimmed = strings.TrimSpace(trimmed[2 : len(trimmed)-1])
	} else if strings.HasPrefix(trimmed, "$") {
		trimmed = strings.TrimSpace(trimmed[1:])
	}

	if trimmed == "" {
		return "", false
	}

	// 1. If an environment variable with this exact name exists and is non-empty, use its value.
	if envVal := os.Getenv(trimmed); envVal != "" {
		return strings.TrimSpace(envVal), true
	}

	// 2. Check if the input is likely a direct API key secret token rather than an env var name.
	if isLikelyDirectKey(trimmed) {
		return trimmed, false
	}

	// 3. Otherwise, treat it as an unset or empty environment variable.
	return "", true
}

func isLikelyDirectKey(s string) bool {
	if s == "" {
		return false
	}

	// 1. If it contains characters illegal in POSIX environment variable names,
	// it is definitely a direct key token (e.g. contains hyphens, colons, slashes, spaces, dots, pluses).
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return true
		}
	}

	lower := strings.ToLower(s)
	// 2. Well-known secret token prefixes
	for _, prefix := range []string{"sk-", "sk_", "gsk_", "hf_", "xai-", "ghr-", "key-", "api-", "co-", "pplx-", "glpat-", "ghp_"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	// 3. Standard environment variable naming convention is UPPERCASE with underscores and digits (e.g. OPENAI_API_KEY).
	// If the string contains lowercase letters and was not found in the environment,
	// it is almost certainly a direct API key / secret token (e.g. "secret", "mytoken123", "a1b2c3d4...").
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}

	// 4. Long random hash strings (>= 32 chars) without standard env var descriptor words
	// like "KEY", "TOKEN", "SECRET", "API" are treated as raw keys.
	if len(s) >= 32 && !strings.Contains(s, "KEY") && !strings.Contains(s, "TOKEN") && !strings.Contains(s, "SECRET") && !strings.Contains(s, "API") {
		return true
	}

	return false
}

