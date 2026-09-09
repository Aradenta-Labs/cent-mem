package config

import (
	"fmt"
	"strconv"
	"strings"
)

// KnownConfigKeys lists all valid dot-notation config keys.
var KnownConfigKeys = []string{
	"model.name",
	"model.dims",
	"retention.fact_keep_days",
	"retention.note_summarize_after_days",
	"retention.log_summarize_after_days",
	"retention.log_drop_after_days",
	"retention.archive_keep_days",
	"capture.enabled",
	"capture.harness",
	"capture.triggers",
	"capture.scope",
	"capture.categories",
	"capture.transcript_path",
	"capture.backend",
	"capture.local_llm_endpoint",
	"capture.local_llm_model",
	"capture.api_base_url",
	"capture.api_key_env",
	"capture.api_key",
	"capture.api_model",
	"capture.confidence_threshold",
	"search.decay_half_life_days",
	"search.reranker",
	"search.rerank_window",
	"search.session_boost",
	"search.agent_boost",
	"search.importance_boost_enabled",
	"search.importance_weight",
	"search.importance_cap",
	"llm.backend",
	"llm.endpoint",
	"llm.model",
	"llm.api_key",
	"llm.timeout_seconds",
	"llm.max_tokens",
	"llm.temperature",
	"agent.enabled",
	"agent.max_reasoning_steps",
	"agent.confidence_threshold",
	"agent.auto_apply_safe_links",
}

// GetConfigValue retrieves a config property by its dot-notation key or table name.
func GetConfigValue(cfg Config, key string) (any, error) {
	norm := strings.ToLower(strings.TrimSpace(key))
	switch norm {
	case "model":
		return cfg.Model, nil
	case "model.name":
		return cfg.Model.Name, nil
	case "model.dims":
		return cfg.Model.Dims, nil

	case "retention":
		return cfg.Retention, nil
	case "retention.fact_keep_days":
		return cfg.Retention.FactKeepDays, nil
	case "retention.note_summarize_after_days":
		return cfg.Retention.NoteSummarizeAfterDays, nil
	case "retention.log_summarize_after_days":
		return cfg.Retention.LogSummarizeAfterDays, nil
	case "retention.log_drop_after_days":
		return cfg.Retention.LogDropAfterDays, nil
	case "retention.archive_keep_days":
		return cfg.Retention.ArchiveKeepDays, nil

	case "capture":
		return cfg.Capture, nil
	case "capture.enabled":
		return cfg.Capture.Enabled, nil
	case "capture.harness":
		return cfg.Capture.Harness, nil
	case "capture.triggers":
		return cfg.Capture.Triggers, nil
	case "capture.scope":
		return cfg.Capture.Scope, nil
	case "capture.categories":
		return cfg.Capture.Categories, nil
	case "capture.transcript_path":
		return cfg.Capture.TranscriptPath, nil
	case "capture.backend":
		return cfg.Capture.Backend, nil
	case "capture.local_llm_endpoint":
		return cfg.Capture.LocalLLMEndpoint, nil
	case "capture.local_llm_model":
		return cfg.Capture.LocalLLMModel, nil
	case "capture.api_base_url":
		return cfg.Capture.APIBaseURL, nil
	case "capture.api_key":
		if cfg.Capture.APIKey != "" {
			return cfg.Capture.APIKey, nil
		}
		return cfg.Capture.APIKeyEnv, nil
	case "capture.api_key_env":
		if cfg.Capture.APIKeyEnv != "" {
			return cfg.Capture.APIKeyEnv, nil
		}
		return cfg.Capture.APIKey, nil
	case "capture.api_model":
		return cfg.Capture.APIModel, nil
	case "capture.confidence_threshold":
		return cfg.Capture.ConfidenceThreshold, nil

	case "search":
		return cfg.Search, nil
	case "search.decay_half_life_days":
		return cfg.Search.DecayHalfLifeDays, nil
	case "search.reranker":
		return cfg.Search.Reranker, nil
	case "search.rerank_window":
		return cfg.Search.RerankWindow, nil
	case "search.session_boost":
		return cfg.Search.SessionBoost, nil
	case "search.agent_boost":
		return cfg.Search.AgentBoost, nil
	case "search.importance_boost_enabled":
		return cfg.Search.ImportanceBoostEnabled, nil
	case "search.importance_weight":
		return cfg.Search.ImportanceWeight, nil
	case "search.importance_cap":
		return cfg.Search.ImportanceCap, nil

	case "llm":
		return cfg.LLM, nil
	case "llm.backend":
		return cfg.LLM.Backend, nil
	case "llm.endpoint":
		return cfg.LLM.Endpoint, nil
	case "llm.model":
		return cfg.LLM.Model, nil
	case "llm.api_key":
		return cfg.LLM.APIKey, nil
	case "llm.timeout_seconds":
		return cfg.LLM.TimeoutSeconds, nil
	case "llm.max_tokens":
		return cfg.LLM.MaxTokens, nil
	case "llm.temperature":
		return cfg.LLM.Temperature, nil

	case "agent":
		return cfg.Agent, nil
	case "agent.enabled":
		return cfg.Agent.Enabled, nil
	case "agent.max_reasoning_steps":
		return cfg.Agent.MaxReasoningSteps, nil
	case "agent.confidence_threshold":
		return cfg.Agent.ConfidenceThreshold, nil
	case "agent.auto_apply_safe_links":
		return cfg.Agent.AutoApplySafeLinks, nil

	default:
		return nil, fmt.Errorf("unknown config key %q", key)
	}
}

// SetConfigValue updates a config property by its dot-notation key.
func SetConfigValue(cfg *Config, key, rawVal string) error {
	norm := strings.ToLower(strings.TrimSpace(key))
	val := strings.TrimSpace(rawVal)

	switch norm {
	case "model.name":
		if val == "" {
			return fmt.Errorf("model.name cannot be empty")
		}
		cfg.Model.Name = val

	case "model.dims":
		d, err := strconv.Atoi(val)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid model.dims %q: must be a positive integer", val)
		}
		cfg.Model.Dims = d

	case "retention.fact_keep_days":
		days, err := strconv.Atoi(val)
		if err != nil || days < 0 {
			return fmt.Errorf("invalid retention.fact_keep_days %q: must be >= 0", val)
		}
		cfg.Retention.FactKeepDays = days

	case "retention.note_summarize_after_days":
		days, err := strconv.Atoi(val)
		if err != nil || days < 0 {
			return fmt.Errorf("invalid retention.note_summarize_after_days %q: must be >= 0", val)
		}
		cfg.Retention.NoteSummarizeAfterDays = days

	case "retention.log_summarize_after_days":
		days, err := strconv.Atoi(val)
		if err != nil || days < 0 {
			return fmt.Errorf("invalid retention.log_summarize_after_days %q: must be >= 0", val)
		}
		cfg.Retention.LogSummarizeAfterDays = days

	case "retention.log_drop_after_days":
		days, err := strconv.Atoi(val)
		if err != nil || days < 0 {
			return fmt.Errorf("invalid retention.log_drop_after_days %q: must be >= 0", val)
		}
		cfg.Retention.LogDropAfterDays = days

	case "retention.archive_keep_days":
		days, err := strconv.Atoi(val)
		if err != nil || days < 0 {
			return fmt.Errorf("invalid retention.archive_keep_days %q: must be >= 0", val)
		}
		cfg.Retention.ArchiveKeepDays = days

	case "capture.enabled":
		b, err := parseBoolFlexible(val)
		if err != nil {
			return fmt.Errorf("invalid boolean value for capture.enabled: %q (expected true/false)", val)
		}
		cfg.Capture.Enabled = b

	case "capture.harness":
		switch strings.ToLower(val) {
		case "antigravity", "trae", "claude-code", "cursor", "codex", "deepseek", "hermes", "generic":
			cfg.Capture.Harness = strings.ToLower(val)
		default:
			return fmt.Errorf("unknown harness %q (expected antigravity, trae, claude-code, cursor, codex, deepseek, hermes)", val)
		}

	case "capture.triggers":
		parts := strings.Split(val, ",")
		var cleaned []string
		for _, p := range parts {
			t := strings.ToLower(strings.TrimSpace(p))
			if t == "all" {
				cleaned = []string{"message", "session-end", "on-demand"}
				break
			}
			if t != "" {
				switch t {
				case "message", "session-end", "on-demand":
					cleaned = append(cleaned, t)
				default:
					return fmt.Errorf("invalid trigger %q (expected message, session-end, on-demand, or all)", t)
				}
			}
		}
		if len(cleaned) == 0 {
			return fmt.Errorf("capture.triggers cannot be empty")
		}
		cfg.Capture.Triggers = cleaned

	case "capture.scope":
		if val == "" {
			val = "global"
		}
		cfg.Capture.Scope = val

	case "capture.categories":
		parts := strings.Split(val, ",")
		var cleaned []string
		for _, p := range parts {
			c := strings.ToLower(strings.TrimSpace(p))
			if c != "" {
				cleaned = append(cleaned, c)
			}
		}
		if len(cleaned) == 0 {
			return fmt.Errorf("capture.categories cannot be empty")
		}
		cfg.Capture.Categories = cleaned

	case "capture.transcript_path":
		cfg.Capture.TranscriptPath = val

	case "capture.backend":
		switch strings.ToLower(val) {
		case "local-llm", "heuristic", "openai-compatible":
			cfg.Capture.Backend = strings.ToLower(val)
		default:
			return fmt.Errorf("unknown backend %q (expected local-llm, heuristic, or openai-compatible)", val)
		}

	case "capture.local_llm_endpoint":
		cfg.Capture.LocalLLMEndpoint = val

	case "capture.local_llm_model":
		cfg.Capture.LocalLLMModel = val

	case "capture.api_base_url":
		cfg.Capture.APIBaseURL = val

	case "capture.api_key":
		cfg.Capture.APIKey = val
		cfg.Capture.APIKeyEnv = val

	case "capture.api_key_env":
		cfg.Capture.APIKeyEnv = val

	case "capture.api_model":
		cfg.Capture.APIModel = val

	case "capture.confidence_threshold":
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || f < 0.0 || f > 1.0 {
			return fmt.Errorf("invalid confidence_threshold %q: must be a float between 0.0 and 1.0", val)
		}
		cfg.Capture.ConfidenceThreshold = f

	case "search.decay_half_life_days":
		days, err := strconv.Atoi(val)
		if err != nil || days < 0 {
			return fmt.Errorf("invalid search.decay_half_life_days %q: must be >= 0", val)
		}
		cfg.Search.DecayHalfLifeDays = days

	case "search.reranker":
		switch strings.ToLower(val) {
		case "composite", "none", "cross_encoder", "llm":
			cfg.Search.Reranker = strings.ToLower(val)
		default:
			return fmt.Errorf("invalid search.reranker %q (expected composite, none, cross_encoder, or llm)", val)
		}

	case "search.rerank_window":
		w, err := strconv.Atoi(val)
		if err != nil || w < 0 {
			return fmt.Errorf("invalid search.rerank_window %q: must be >= 0", val)
		}
		cfg.Search.RerankWindow = w

	case "search.session_boost":
		b, err := strconv.ParseFloat(val, 64)
		if err != nil || b < 0 {
			return fmt.Errorf("invalid search.session_boost %q: must be >= 0", val)
		}
		cfg.Search.SessionBoost = b

	case "search.agent_boost":
		b, err := strconv.ParseFloat(val, 64)
		if err != nil || b < 0 {
			return fmt.Errorf("invalid search.agent_boost %q: must be >= 0", val)
		}
		cfg.Search.AgentBoost = b

	case "search.importance_boost_enabled":
		b, err := parseBoolFlexible(val)
		if err != nil {
			return fmt.Errorf("invalid search.importance_boost_enabled %q: must be a boolean", val)
		}
		cfg.Search.ImportanceBoostEnabled = b

	case "search.importance_weight":
		w, err := strconv.ParseFloat(val, 64)
		if err != nil || w < 0 {
			return fmt.Errorf("invalid search.importance_weight %q: must be >= 0", val)
		}
		cfg.Search.ImportanceWeight = w

	case "search.importance_cap":
		c, err := strconv.ParseFloat(val, 64)
		if err != nil || c < 1.0 {
			return fmt.Errorf("invalid search.importance_cap %q: must be >= 1.0", val)
		}
		cfg.Search.ImportanceCap = c

	case "llm.backend":
		switch strings.ToLower(val) {
		case "ollama", "openai_compatible", "openai-compatible", "disabled":
			cfg.LLM.Backend = strings.ToLower(val)
		default:
			return fmt.Errorf("invalid llm.backend %q (expected ollama, openai_compatible, or disabled)", val)
		}

	case "llm.endpoint":
		cfg.LLM.Endpoint = val

	case "llm.model":
		cfg.LLM.Model = val

	case "llm.api_key":
		cfg.LLM.APIKey = val

	case "llm.timeout_seconds":
		s, err := strconv.Atoi(val)
		if err != nil || s < 0 {
			return fmt.Errorf("invalid llm.timeout_seconds %q: must be >= 0", val)
		}
		cfg.LLM.TimeoutSeconds = s

	case "llm.max_tokens":
		m, err := strconv.Atoi(val)
		if err != nil || m < 0 {
			return fmt.Errorf("invalid llm.max_tokens %q: must be >= 0", val)
		}
		cfg.LLM.MaxTokens = m

	case "llm.temperature":
		t, err := strconv.ParseFloat(val, 64)
		if err != nil || t < 0.0 {
			return fmt.Errorf("invalid llm.temperature %q: must be >= 0.0", val)
		}
		cfg.LLM.Temperature = t

	case "agent.enabled":
		b, err := parseBoolFlexible(val)
		if err != nil {
			return fmt.Errorf("invalid agent.enabled %q: must be a boolean", val)
		}
		cfg.Agent.Enabled = b

	case "agent.max_reasoning_steps":
		s, err := strconv.Atoi(val)
		if err != nil || s < 0 {
			return fmt.Errorf("invalid agent.max_reasoning_steps %q: must be >= 0", val)
		}
		cfg.Agent.MaxReasoningSteps = s

	case "agent.confidence_threshold":
		c, err := strconv.ParseFloat(val, 64)
		if err != nil || c < 0.0 || c > 1.0 {
			return fmt.Errorf("invalid agent.confidence_threshold %q: must be between 0.0 and 1.0", val)
		}
		cfg.Agent.ConfidenceThreshold = c

	case "agent.auto_apply_safe_links":
		b, err := parseBoolFlexible(val)
		if err != nil {
			return fmt.Errorf("invalid agent.auto_apply_safe_links %q: must be a boolean", val)
		}
		cfg.Agent.AutoApplySafeLinks = b

	default:
		return fmt.Errorf("unknown config key %q", key)
	}

	if err := validateRetention(cfg.Retention); err != nil {
		return err
	}
	if err := ValidateCaptureConfig(cfg.Capture); err != nil {
		return err
	}
	if err := validateSearchConfig(cfg.Search); err != nil {
		return err
	}
	if err := validateLLMConfig(cfg.LLM); err != nil {
		return err
	}
	if err := validateAgentConfig(cfg.Agent); err != nil {
		return err
	}

	return nil
}

func parseBoolFlexible(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on", "t":
		return true, nil
	case "false", "0", "no", "off", "f":
		return false, nil
	default:
		return false, fmt.Errorf("not a boolean")
	}
}
