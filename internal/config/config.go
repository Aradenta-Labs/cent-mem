package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Home   string
	DBPath string
	Model  ModelConfig
}

type ModelConfig struct {
	Name string
	Path string
	Dims int
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

	return cfg, nil
}

// Ensure creates the home and model directories and sets permissions (0700 for Home).
func (c Config) Ensure() error {
	if err := os.MkdirAll(c.Home, 0700); err != nil {
		return fmt.Errorf("failed to create home dir: %w", err)
	}

	modelDir := filepath.Dir(c.Model.Path)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("failed to create models dir: %w", err)
	}

	return nil
}
