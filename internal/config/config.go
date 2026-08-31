package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Home      string
	DBPath    string
	Model     ModelConfig
	Retention Retention
}

type ModelConfig struct {
	Name string
	Path string
	Dims int
}

// Retention captures per-type retention policies (see docs/data-model.md).
// Values are in days; 0 means "keep forever".
type Retention struct {
	// FactKeepDays is 0 by default (facts are kept forever).
	FactKeepDays int
	// NoteSummarizeAfterDays: notes are summarized after this many days.
	NoteSummarizeAfterDays int
	// LogSummarizeAfterDays: logs are summarized after this many days.
	LogSummarizeAfterDays int
	// LogDropAfterDays: raw logs are dropped after this many days.
	LogDropAfterDays int
	// ArchiveKeepDays: archived rows are kept for audit for this many days.
	ArchiveKeepDays int
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
// CENTMEM_HOME, CENTMEM_DB, and CENTMEM_MODEL.
func Load() (Config, error) {
	// 1. Establish defaults
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	defaultHome := filepath.Join(homeDir, ".centmem")

	cfg := Config{
		Home:   defaultHome,
		DBPath: "", // Will be computed after Home is final
		Model: ModelConfig{
			Name: "bge-small-en-v1.5",
			Path: "", // Will be computed after Home/Model.Name are final
			Dims: 384,
		},
		Retention: DefaultRetention(),
	}

	// TODO/Future Phase: In M1/M4 we load a config.toml file if it exists.
	// For Phase 0 / baseline M1, we proceed with defaults + env overrides.

	// 2. Apply env overrides
	if envHome := os.Getenv("CENTMEM_HOME"); envHome != "" {
		cfg.Home = envHome
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

	// 4. Validate retention values (negative days are nonsensical).
	if err := validateRetention(cfg.Retention); err != nil {
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
