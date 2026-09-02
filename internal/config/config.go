package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Home      string        `json:"home" toml:"home"`
	DBPath    string        `json:"db_path" toml:"db_path"`
	Model     ModelConfig   `json:"model" toml:"model"`
	Retention Retention     `json:"retention" toml:"retention"`
	Capture   CaptureConfig `json:"capture" toml:"capture"`
}

type ModelConfig struct {
	Name string `json:"name" toml:"name"`
	Path string `json:"path" toml:"path"`
	Dims int    `json:"dims" toml:"dims"`
}

// Retention captures per-type retention policies (see docs/data-model.md).
// Values are in days; 0 means "keep forever".
type Retention struct {
	// FactKeepDays is 0 by default (facts are kept forever).
	FactKeepDays int `json:"fact_keep_days" toml:"fact_keep_days"`
	// NoteSummarizeAfterDays: notes are summarized after this many days.
	NoteSummarizeAfterDays int `json:"note_summarize_after_days" toml:"note_summarize_after_days"`
	// LogSummarizeAfterDays: logs are summarized after this many days.
	LogSummarizeAfterDays int `json:"log_summarize_after_days" toml:"log_summarize_after_days"`
	// LogDropAfterDays: raw logs are dropped after this many days.
	LogDropAfterDays int `json:"log_drop_after_days" toml:"log_drop_after_days"`
	// ArchiveKeepDays: archived rows are kept for audit for this many days.
	ArchiveKeepDays int `json:"archive_keep_days" toml:"archive_keep_days"`
}

// DefaultRetention returns the standard retention policy.
func DefaultRetention() Retention {
	return Retention{
		FactKeepDays:           0,
		NoteSummarizeAfterDays: 30,
		LogSummarizeAfterDays:  14,
		LogDropAfterDays:       30,
		ArchiveKeepDays:        365,
	}
}

// Load reads config.toml (if it exists) and applies overrides from env variables:
// CENTMEM_HOME, CENTMEM_DB, CENTMEM_MODEL, CENTMEM_RETENTION_*, and CENTMEM_CAPTURE_*.
func Load() (Config, error) {
	// 1. Establish defaults
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	defaultHome := filepath.Join(homeDir, ".centmem")

	// 2. Apply env overrides for Home
	home := defaultHome
	if envHome := os.Getenv("CENTMEM_HOME"); envHome != "" {
		home = envHome
	}

	// 3. Load from config.toml if it exists
	cfg, err := LoadTOML(filepath.Join(home, "config.toml"))
	if err != nil {
		return Config{}, err
	}
	cfg.Home = home

	// Load categories from home directory if custom file exists
	catFile := filepath.Join(cfg.Home, "capture-categories.json")
	if data, err := os.ReadFile(catFile); err == nil {
		var cats []string
		if err := json.Unmarshal(data, &cats); err == nil && len(cats) > 0 {
			cfg.Capture.Categories = cats
		}
	}

	if envDB := os.Getenv("CENTMEM_DB"); envDB != "" {
		cfg.DBPath = envDB
	} else {
		cfg.DBPath = filepath.Join(cfg.Home, "centmem.db")
	}

	if envModel := os.Getenv("CENTMEM_MODEL"); envModel != "" {
		cfg.Model.Name = envModel
	}

	cfg.Model.Path = filepath.Join(cfg.Home, "models", cfg.Model.Name+".onnx")

	if envDims := os.Getenv("CENTMEM_MODEL_DIMS"); envDims != "" {
		if dims, err := strconv.Atoi(envDims); err == nil {
			cfg.Model.Dims = dims
		}
	}

	// 3. Apply retention env overrides (CENTMEM_RETENTION_*).
	if v := os.Getenv("CENTMEM_RETENTION_FACT_KEEP_DAYS"); v != "" {
		cfg.Retention.FactKeepDays, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("CENTMEM_RETENTION_NOTE_SUMMARIZE_AFTER_DAYS"); v != "" {
		cfg.Retention.NoteSummarizeAfterDays, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("CENTMEM_RETENTION_LOG_SUMMARIZE_AFTER_DAYS"); v != "" {
		cfg.Retention.LogSummarizeAfterDays, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("CENTMEM_RETENTION_LOG_DROP_AFTER_DAYS"); v != "" {
		cfg.Retention.LogDropAfterDays, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("CENTMEM_RETENTION_ARCHIVE_KEEP_DAYS"); v != "" {
		cfg.Retention.ArchiveKeepDays, _ = strconv.Atoi(v)
	}

	// 4. Apply capture env overrides (CENTMEM_CAPTURE_*).
	if v := os.Getenv("CENTMEM_CAPTURE_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Capture.Enabled = b
		}
	}
	if v := os.Getenv("CENTMEM_CAPTURE_HARNESS"); v != "" {
		cfg.Capture.Harness = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_SCOPE"); v != "" {
		cfg.Capture.Scope = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_TRANSCRIPT_PATH"); v != "" {
		cfg.Capture.TranscriptPath = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_BACKEND"); v != "" {
		cfg.Capture.Backend = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_LOCAL_LLM_ENDPOINT"); v != "" {
		cfg.Capture.LocalLLMEndpoint = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_LOCAL_LLM_MODEL"); v != "" {
		cfg.Capture.LocalLLMModel = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_API_BASE_URL"); v != "" {
		cfg.Capture.APIBaseURL = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_API_KEY_ENV"); v != "" {
		cfg.Capture.APIKeyEnv = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_API_MODEL"); v != "" {
		cfg.Capture.APIModel = v
	}
	if v := os.Getenv("CENTMEM_CAPTURE_CONFIDENCE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Capture.ConfidenceThreshold = f
		}
	}
	if v := os.Getenv("CENTMEM_CAPTURE_CATEGORIES"); v != "" {
		cats := strings.Split(v, ",")
		var cleaned []string
		for _, cat := range cats {
			if trimmed := strings.TrimSpace(cat); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			cfg.Capture.Categories = cleaned
		}
	}
	if v := os.Getenv("CENTMEM_CAPTURE_TRIGGERS"); v != "" {
		trigs := strings.Split(v, ",")
		var cleaned []string
		for _, trig := range trigs {
			if trimmed := strings.TrimSpace(trig); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			cfg.Capture.Triggers = cleaned
		}
	}

	// 5. Validate retention and capture values.
	if err := validateRetention(cfg.Retention); err != nil {
		return Config{}, err
	}
	if err := ValidateCaptureConfig(cfg.Capture); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// validateRetention rejects nonsensical retention values.
func validateRetention(r Retention) error {
	neg := func(name string, v int) error {
		if v < 0 {
			return fmt.Errorf("config: invalid %s=%d: must be >= 0", name, v)
		}
		return nil
	}
	if err := neg("fact_keep_days", r.FactKeepDays); err != nil {
		return err
	}
	if err := neg("note_summarize_after_days", r.NoteSummarizeAfterDays); err != nil {
		return err
	}
	if err := neg("log_summarize_after_days", r.LogSummarizeAfterDays); err != nil {
		return err
	}
	if err := neg("log_drop_after_days", r.LogDropAfterDays); err != nil {
		return err
	}
	if err := neg("archive_keep_days", r.ArchiveKeepDays); err != nil {
		return err
	}
	return nil
}

// Ensure creates the home and model directories and sets permissions (0700 for Home).
func (c Config) Ensure() error {
	if err := os.MkdirAll(c.Home, 0700); err != nil {
		return fmt.Errorf("failed to create home dir: %w", err)
	}
	// Explicitly enforce owner-only permissions (0700) on the home directory
	// so `doctor` passes even if the directory was pre-created via umask 0755.
	if err := os.Chmod(c.Home, 0700); err != nil {
		return fmt.Errorf("failed to restrict home dir permissions: %w", err)
	}

	modelDir := filepath.Dir(c.Model.Path)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("failed to create models dir: %w", err)
	}

	return nil
}
