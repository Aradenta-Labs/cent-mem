package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

const configHeader = `# cent-mem configuration file
# Schema and reference: docs/architecture.md and docs/plan-v1.3.0.md
#
# Note: Never write raw API keys into this file.
# Set capture.api_key_env to the name of the environment variable containing your key (e.g. "OPENAI_API_KEY").

`

// tomlFileSchema defines the persistent layout of config.toml.
type tomlFileSchema struct {
	Model     ModelConfig   `toml:"model"`
	Retention Retention     `toml:"retention"`
	Capture   CaptureConfig `toml:"capture"`
	Search    SearchConfig  `toml:"search"`
	Daemon    DaemonConfig  `toml:"daemon"`
	LLM       LLMConfig     `toml:"llm"`
	Agent     AgentConfig   `toml:"agent"`
}

// LoadTOML reads a TOML configuration file and overlays it onto the default Config.
// If the file does not exist, it returns the defaults without error.
func LoadTOML(path string) (Config, error) {
	cfg := Config{
		Model: ModelConfig{
			Name: "bge-small-en-v1.5",
			Dims: 384,
		},
		Retention: DefaultRetention(),
		Capture:   DefaultCaptureConfig(),
		Search:    DefaultSearchConfig(),
		Daemon:    DefaultDaemonConfig(),
		LLM:       DefaultLLMConfig(),
		Agent:     DefaultAgentConfig(),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}

	schema := tomlFileSchema{
		Model:     cfg.Model,
		Retention: cfg.Retention,
		Capture:   cfg.Capture,
		Search:    cfg.Search,
		Daemon:    cfg.Daemon,
		LLM:       cfg.LLM,
		Agent:     cfg.Agent,
	}

	if err := toml.Unmarshal(data, &schema); err != nil {
		return cfg, fmt.Errorf("parse toml config %q: %w", path, err)
	}

	cfg.Model = schema.Model
	cfg.Retention = schema.Retention
	cfg.Capture = schema.Capture
	cfg.Search = schema.Search
	cfg.Daemon = schema.Daemon
	cfg.LLM = schema.LLM
	cfg.Agent = schema.Agent

	return cfg, nil
}

// SaveTOML serializes the configuration to TOML and writes it atomically to path.
func SaveTOML(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir %q: %w", dir, err)
	}

	schema := tomlFileSchema{
		Model:     cfg.Model,
		Retention: cfg.Retention,
		Capture:   cfg.Capture,
		Search:    cfg.Search,
		Daemon:    cfg.Daemon,
		LLM:       cfg.LLM,
		Agent:     cfg.Agent,
	}

	data, err := toml.Marshal(schema)
	if err != nil {
		return fmt.Errorf("marshal toml: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(configHeader)
	buf.Write(data)

	tmpPath := path + ".tmp." + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("write temp config %q: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename config to %q: %w", path, err)
	}

	return nil
}

// SaveToHome writes the configuration to ~/.centmem/config.toml (or cfg.Home/config.toml).
func SaveToHome(cfg Config) error {
	if cfg.Home == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		cfg.Home = filepath.Join(homeDir, ".centmem")
	}
	return SaveTOML(filepath.Join(cfg.Home, "config.toml"), cfg)
}
