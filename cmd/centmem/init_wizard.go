package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

const defaultCapturePromptTemplate = `You are a memory classifier for the cent-mem project memory store.
You will receive a list of AI agent conversation messages.
Your job is to identify content worth saving as a long-term memory.

For each piece of content worth saving, output a JSON object with:
- category: one of [{{CATEGORIES}}]
- content: the exact content to store (concise, self-contained)
- key: (for facts only) a dot-notation key like "api.base_url"
- tags: array of relevant tags
- confidence: 0.0–1.0 (only output items with confidence ≥ {{CONFIDENCE_THRESHOLD}})

Before saving, you will also receive the top-3 recall results for similar content.
If any existing memory already captures this information, output {"skip": true, "reason": "..."}.

Output a JSON array. Output nothing else.
`

// promptInput reads a single line of input after displaying a prompt.
// If input is empty, it returns the provided fallback default.
func promptInput(reader *bufio.Reader, writer io.Writer, prompt, fallback string) (string, error) {
	if writer != nil {
		fmt.Fprint(writer, prompt)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return fallback, nil
	}
	return trimmed, nil
}

// runCaptureInitWizard conducts the interactive capture setup questionnaire.
func runCaptureInitWizard(in io.Reader, out io.Writer, cfg *config.Config) error {
	reader := bufio.NewReader(in)

	if out != nil {
		fmt.Fprintln(out, "\n--- cent-mem Transcript Auto-Capture Setup ---")
	}

	// 1. Harness
	harness, err := promptInput(reader, out,
		"Which AI agent do you use? [antigravity / trae / claude-code / cursor / codex / deepseek / hermes] (default: antigravity): ",
		"antigravity")
	if err != nil {
		return err
	}
	if err := config.SetConfigValue(cfg, "capture.harness", harness); err != nil {
		cfg.Capture.Harness = "antigravity"
	}

	// 2. Triggers
	triggers, err := promptInput(reader, out,
		"Which triggers to enable? [message / session-end / on-demand / all] (default: all): ",
		"all")
	if err != nil {
		return err
	}
	if err := config.SetConfigValue(cfg, "capture.triggers", triggers); err != nil {
		cfg.Capture.Triggers = []string{"message", "session-end", "on-demand"}
	}

	// 3. Scope
	scope, err := promptInput(reader, out,
		"Default scope for captured memories? [e.g. project:myapp] (default: global): ",
		"global")
	if err != nil {
		return err
	}
	if err := config.SetConfigValue(cfg, "capture.scope", scope); err != nil {
		cfg.Capture.Scope = "global"
	}

	// 4. Categories
	defaultCats := strings.Join(config.DefaultCategories(), ",")
	cats, err := promptInput(reader, out,
		fmt.Sprintf("Capture categories (comma-separated, or press Enter for defaults: %s): ", defaultCats),
		defaultCats)
	if err != nil {
		return err
	}
	if err := config.SetConfigValue(cfg, "capture.categories", cats); err != nil {
		cfg.Capture.Categories = config.DefaultCategories()
	}

	// 5. Backend
	backend, err := promptInput(reader, out,
		"Which classifier backend? [local-llm / heuristic / openai-compatible] (default: heuristic): ",
		"heuristic")
	if err != nil {
		return err
	}
	if err := config.SetConfigValue(cfg, "capture.backend", backend); err != nil {
		cfg.Capture.Backend = "heuristic"
	}

	// Backend specifics
	switch strings.ToLower(cfg.Capture.Backend) {
	case "local-llm":
		endpoint, err := promptInput(reader, out,
			"Local LLM endpoint? [default: http://localhost:11434/v1]: ",
			"http://localhost:11434/v1")
		if err == nil {
			_ = config.SetConfigValue(cfg, "capture.local_llm_endpoint", endpoint)
		}
		model, err := promptInput(reader, out,
			"Model name? [default: llama3.2]: ",
			"llama3.2")
		if err == nil {
			_ = config.SetConfigValue(cfg, "capture.local_llm_model", model)
		}

	case "openai-compatible":
		apiURL, err := promptInput(reader, out,
			"API base URL? [default: https://api.openai.com/v1]: ",
			"https://api.openai.com/v1")
		if err == nil {
			_ = config.SetConfigValue(cfg, "capture.api_base_url", apiURL)
		}
		keyEnv, err := promptInput(reader, out,
			"Which env var holds your API key? [default: OPENAI_API_KEY]: ",
			"OPENAI_API_KEY")
		if err == nil {
			_ = config.SetConfigValue(cfg, "capture.api_key_env", keyEnv)
		}
		apiModel, err := promptInput(reader, out,
			"Model? [default: gpt-4o-mini]: ",
			"gpt-4o-mini")
		if err == nil {
			_ = config.SetConfigValue(cfg, "capture.api_model", apiModel)
		}
	}

	// 6. Confidence Threshold
	conf, err := promptInput(reader, out,
		"Confidence threshold (0.0–1.0)? [default: 0.7]: ",
		"0.7")
	if err == nil {
		if f, parseErr := strconv.ParseFloat(conf, 64); parseErr == nil && f >= 0.0 && f <= 1.0 {
			cfg.Capture.ConfidenceThreshold = f
		}
	}

	cfg.Capture.Enabled = true

	if out != nil {
		fmt.Fprintln(out, "Capture configuration updated successfully.")
	}

	return nil
}

// writeDefaultCapturePrompt writes the default prompt template file if absent.
func writeDefaultCapturePrompt(home string) error {
	promptPath := filepath.Join(home, "capture-prompt.md")
	if _, err := os.Stat(promptPath); err == nil {
		return nil // already exists
	}
	return os.WriteFile(promptPath, []byte(defaultCapturePromptTemplate), 0644)
}
