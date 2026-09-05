package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

// CaptureItem represents a piece of durable knowledge extracted from a transcript.
type CaptureItem struct {
	Category   string   `json:"category"`
	Content    string   `json:"content"`
	Key        string   `json:"key,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Confidence float64  `json:"confidence"`
	Skip       bool     `json:"skip,omitempty"`
	SkipReason string   `json:"skip_reason,omitempty"`
}

// RecallFunc retrieves existing memory snippets relevant to candidate content.
type RecallFunc func(ctx context.Context, query string) ([]string, error)

// Classifier interface defines methods to classify transcripts into structured memories.
type Classifier interface {
	Classify(ctx context.Context, messages []TranscriptMessage, recallFn RecallFunc) ([]CaptureItem, error)
}

// DefaultPromptTemplate is the prompt used for LLM-based classification.
const DefaultPromptTemplate = `You are a memory classifier for the cent-mem project memory store.
You will receive a list of AI agent conversation messages.
Your job is to identify content worth saving as a long-term memory.

For each piece of content worth saving, output a JSON object with:
- category: one of [{{CATEGORIES}}]
- content: the exact content to store (concise, self-contained)
- key: (for facts and dependencies only) a dot-notation key like "api.base_url" or "dep.react"
- tags: array of relevant tags
- confidence: 0.0–1.0 (only output items with confidence >= {{CONFIDENCE_THRESHOLD}})

Before saving, here are relevant existing memories:
{{RECALL_CONTEXT}}
If an existing memory already captures this exact information, output {"skip": true, "skip_reason": "already exists", "content": "..."}.

Output a JSON array of objects. Output nothing else.`

// BuildPrompt renders the classification prompt with runtime configuration and recall context.
func BuildPrompt(cfg CaptureConfig, recallSnippets []string) string {
	template := DefaultPromptTemplate

	// Check if a custom template exists in home dir or CENTMEM_HOME
	var homeDir string
	if envHome := os.Getenv("CENTMEM_HOME"); envHome != "" {
		homeDir = envHome
	} else if userHome, err := os.UserHomeDir(); err == nil {
		homeDir = filepath.Join(userHome, ".centmem")
	}
	if homeDir != "" {
		promptPath := filepath.Join(homeDir, "capture-prompt.md")
		if data, err := os.ReadFile(promptPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			template = string(data)
		}
	}

	prompt := template
	prompt = strings.ReplaceAll(prompt, "{{CATEGORIES}}", strings.Join(cfg.Categories, ", "))
	prompt = strings.ReplaceAll(prompt, "{{CONFIDENCE_THRESHOLD}}", fmt.Sprintf("%.2f", cfg.ConfidenceThreshold))

	if len(recallSnippets) > 0 {
		prompt = strings.ReplaceAll(prompt, "{{RECALL_CONTEXT}}", "- "+strings.Join(recallSnippets, "\n- "))
	} else {
		prompt = strings.ReplaceAll(prompt, "{{RECALL_CONTEXT}}", "(none)")
	}
	return prompt
}

// HeuristicClassifier is a zero-dependency, rule-based classifier.
type HeuristicClassifier struct {
	cfg CaptureConfig
}

// NewHeuristicClassifier returns a new HeuristicClassifier.
func NewHeuristicClassifier(cfg CaptureConfig) *HeuristicClassifier {
	return &HeuristicClassifier{cfg: cfg}
}

var (
	decisionRegex   = regexp.MustCompile(`(?i)(we decided|we chose|going with|agreed on|architecture decision|decision:|we selected)`)
	preferenceRegex = regexp.MustCompile(`(?i)(i prefer|always use|don't use|my convention|please use|prefer to)`)
	errorRegex      = regexp.MustCompile(`(?i)(bug:|root cause|the fix was|resolved by|error:|failed because|solution was)`)
	logRegex        = regexp.MustCompile(`(?i)^(completed|finished|implemented|added|fixed|migrated|updated)\b`)
	depRegex        = regexp.MustCompile(`(?i)(go get\s+([^\s]+)|npm (?:install|i)\s+([^\s]+)|pip install\s+([^\s]+)|import\s+["']([^"']+)["'])`)
	urlRegex        = regexp.MustCompile(`https?://[^\s)\]]+`)
	versionRegex    = regexp.MustCompile(`\bv[0-9]+\.[0-9]+(?:\.[0-9]+)?\b`)
	keyValueRegex   = regexp.MustCompile(`(?m)^([a-zA-Z0-9_.-]+)\s*=\s*([^\s]+)`)
	codeFenceRegex  = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9_-]+)?\n(.*?)```")
)

func (h *HeuristicClassifier) Classify(ctx context.Context, messages []TranscriptMessage, recallFn RecallFunc) ([]CaptureItem, error) {
	var items []CaptureItem

	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		// 1. Code blocks
		if codeMatches := codeFenceRegex.FindAllStringSubmatch(content, -1); len(codeMatches) > 0 {
			for _, m := range codeMatches {
				codeContent := strings.TrimSpace(m[1])
				if codeContent != "" {
					items = append(items, CaptureItem{
						Category:   "code",
						Content:    codeContent,
						Tags:       []string{"code", "snippet"},
						Confidence: 0.75,
					})
				}
			}
		}

		// 2. Decisions
		if decisionRegex.MatchString(content) {
			items = append(items, CaptureItem{
				Category:   "decision",
				Content:    content,
				Tags:       []string{"decision"},
				Confidence: 0.75,
			})
		}

		// 3. Preferences
		if preferenceRegex.MatchString(content) {
			items = append(items, CaptureItem{
				Category:   "preference",
				Content:    content,
				Tags:       []string{"preference"},
				Confidence: 0.75,
			})
		}

		// 4. Errors & Resolutions
		if errorRegex.MatchString(content) {
			items = append(items, CaptureItem{
				Category:   "error",
				Content:    content,
				Tags:       []string{"error", "resolution"},
				Confidence: 0.75,
			})
		}

		// 5. Dependencies
		if depMatches := depRegex.FindAllStringSubmatch(content, -1); len(depMatches) > 0 {
			for _, dm := range depMatches {
				depName := ""
				for i := 2; i < len(dm); i++ {
					if dm[i] != "" {
						depName = dm[i]
						break
					}
				}
				if depName != "" {
					cleanKey := strings.Trim(depName, `"'/`)
					cleanKey = strings.ReplaceAll(cleanKey, "/", ".")
					items = append(items, CaptureItem{
						Category:   "dependency",
						Key:        "dep." + cleanKey,
						Content:    content,
						Tags:       []string{"dependency", cleanKey},
						Confidence: 0.75,
					})
				}
			}
		}

		// 6. Facts (URLs, Key=Value, Versions)
		if urls := urlRegex.FindAllString(content, -1); len(urls) > 0 {
			for _, u := range urls {
				items = append(items, CaptureItem{
					Category:   "fact",
					Key:        "url." + sanitizeKey(u),
					Content:    u,
					Tags:       []string{"fact", "url"},
					Confidence: 0.75,
				})
			}
		}
		if kvMatches := keyValueRegex.FindAllStringSubmatch(content, -1); len(kvMatches) > 0 {
			for _, kv := range kvMatches {
				items = append(items, CaptureItem{
					Category:   "fact",
					Key:        kv[1],
					Content:    kv[2],
					Tags:       []string{"fact"},
					Confidence: 0.75,
				})
			}
		}

		// 7. Logs
		if logRegex.MatchString(content) {
			items = append(items, CaptureItem{
				Category:   "log",
				Content:    content,
				Tags:       []string{"log"},
				Confidence: 0.75,
			})
		}
	}

	return items, nil
}

func sanitizeKey(s string) string {
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.ReplaceAll(s, "/", ".")
	s = strings.ReplaceAll(s, ":", ".")
	s = strings.ReplaceAll(s, "?", ".")
	s = strings.ReplaceAll(s, "&", ".")
	s = strings.ReplaceAll(s, "=", ".")
	return strings.Trim(s, ".")
}

// LocalLLMClassifier connects to an OpenAI-compatible local LLM endpoint (e.g. Ollama).
type LocalLLMClassifier struct {
	cfg        CaptureConfig
	httpClient *http.Client
	fallback   Classifier
}

// NewLocalLLMClassifier creates a LocalLLMClassifier with fallback to HeuristicClassifier.
func NewLocalLLMClassifier(cfg CaptureConfig) *LocalLLMClassifier {
	return &LocalLLMClassifier{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		fallback:   NewHeuristicClassifier(cfg),
	}
}

// chatCompletionRequest represents the standard OpenAI chat completions JSON payload.
type chatCompletionRequest struct {
	Model       string               `json:"model"`
	Messages    []chatMessagePayload `json:"messages"`
	Temperature float64              `json:"temperature"`
}

type chatMessagePayload struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (l *LocalLLMClassifier) Classify(ctx context.Context, messages []TranscriptMessage, recallFn RecallFunc) ([]CaptureItem, error) {
	endpoint := l.cfg.LocalLLMEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434/v1"
	}
	url := strings.TrimSuffix(endpoint, "/") + "/chat/completions"

	items, err := callOpenAIEndpointWithRetry(ctx, l.httpClient, url, "", l.cfg.LocalLLMModel, l.cfg, messages, recallFn)
	if err != nil {
		// Graceful fallback to heuristic
		return l.fallback.Classify(ctx, messages, recallFn)
	}
	return items, nil
}

// OpenAICompatibleClassifier calls an OpenAI-compatible cloud API with BYOK authentication.
type OpenAICompatibleClassifier struct {
	cfg        CaptureConfig
	httpClient *http.Client
	fallback   Classifier
}

// NewOpenAICompatibleClassifier creates an OpenAICompatibleClassifier.
func NewOpenAICompatibleClassifier(cfg CaptureConfig) *OpenAICompatibleClassifier {
	return &OpenAICompatibleClassifier{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		fallback:   NewHeuristicClassifier(cfg),
	}
}

func (o *OpenAICompatibleClassifier) Classify(ctx context.Context, messages []TranscriptMessage, recallFn RecallFunc) ([]CaptureItem, error) {
	keyEnv := o.cfg.APIKey
	if keyEnv == "" {
		keyEnv = o.cfg.APIKeyEnv
	}
	if keyEnv == "" {
		keyEnv = "OPENAI_API_KEY"
	}
	apiKey, _ := config.ResolveAPIKey(keyEnv)
	if apiKey == "" {
		// Key missing; fallback to heuristic
		return o.fallback.Classify(ctx, messages, recallFn)
	}

	baseURL := o.cfg.APIBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	items, err := callOpenAIEndpointWithRetry(ctx, o.httpClient, url, apiKey, o.cfg.APIModel, o.cfg, messages, recallFn)
	if err != nil {
		return o.fallback.Classify(ctx, messages, recallFn)
	}
	return items, nil
}

func callOpenAIEndpointWithRetry(ctx context.Context, client *http.Client, url, apiKey, model string, cfg CaptureConfig, messages []TranscriptMessage, recallFn RecallFunc) ([]CaptureItem, error) {
	items, err := callOpenAIEndpoint(ctx, client, url, apiKey, model, cfg, messages, recallFn)
	if err == nil {
		return items, nil
	}
	// Attempt one retry on failure
	return callOpenAIEndpoint(ctx, client, url, apiKey, model, cfg, messages, recallFn)
}

func callOpenAIEndpoint(ctx context.Context, client *http.Client, url, apiKey, model string, cfg CaptureConfig, messages []TranscriptMessage, recallFn RecallFunc) ([]CaptureItem, error) {
	if model == "" {
		model = "gpt-4o-mini"
	}

	var recallSnippets []string
	if recallFn != nil {
		// Gather query from first few messages for recall context
		var sampleText []string
		for i, m := range messages {
			if i >= 5 {
				break
			}
			sampleText = append(sampleText, m.Content)
		}
		if len(sampleText) > 0 {
			snippets, _ := recallFn(ctx, strings.Join(sampleText, " "))
			recallSnippets = snippets
		}
	}

	systemPrompt := BuildPrompt(cfg, recallSnippets)

	transcriptBytes, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("marshal transcript messages: %w", err)
	}

	reqBody := chatCompletionRequest{
		Model: model,
		Messages: []chatMessagePayload{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(transcriptBytes)},
		},
		Temperature: 0.0,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	// Strip markdown json fences if present
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var items []CaptureItem
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil, fmt.Errorf("unmarshal capture items: %w (content: %s)", err, content)
	}

	return items, nil
}

// ClassifyWithConfig selects the appropriate classifier backend based on config and filters results by categories and confidence threshold.
func ClassifyWithConfig(ctx context.Context, messages []TranscriptMessage, cfg CaptureConfig, recallFn RecallFunc) ([]CaptureItem, error) {
	var c Classifier
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))

	switch backend {
	case "local-llm":
		c = NewLocalLLMClassifier(cfg)
	case "openai-compatible":
		c = NewOpenAICompatibleClassifier(cfg)
	case "heuristic", "":
		c = NewHeuristicClassifier(cfg)
	default:
		c = NewHeuristicClassifier(cfg)
	}

	rawItems, err := c.Classify(ctx, messages, recallFn)
	if err != nil {
		return nil, err
	}

	allowedCategories := make(map[string]bool)
	for _, cat := range cfg.Categories {
		allowedCategories[strings.ToLower(strings.TrimSpace(cat))] = true
	}

	var filtered []CaptureItem
	for _, item := range rawItems {
		if item.Skip {
			continue
		}
		if len(allowedCategories) > 0 && !allowedCategories[strings.ToLower(item.Category)] {
			continue
		}
		if item.Confidence < cfg.ConfidenceThreshold {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered, nil
}
