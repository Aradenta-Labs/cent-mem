package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/farras/cent-mem/internal/config"
)

func TestConfigDefaults(t *testing.T) {
	os.Clearenv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	expectedHome := filepath.Join(homeDir, ".centmem")

	if cfg.Home != expectedHome {
		t.Errorf("expected home %q, got %q", expectedHome, cfg.Home)
	}

	expectedDB := filepath.Join(expectedHome, "centmem.db")
	if cfg.DBPath != expectedDB {
		t.Errorf("expected DBPath %q, got %q", expectedDB, cfg.DBPath)
	}

	if cfg.Model.Name != "bge-small-en-v1.5" {
		t.Errorf("expected model name %q, got %q", "bge-small-en-v1.5", cfg.Model.Name)
	}

	expectedModelPath := filepath.Join(expectedHome, "models", "bge-small-en-v1.5.onnx")
	if cfg.Model.Path != expectedModelPath {
		t.Errorf("expected model path %q, got %q", expectedModelPath, cfg.Model.Path)
	}

	if cfg.Model.Dims != 384 {
		t.Errorf("expected dimensions 384, got %d", cfg.Model.Dims)
	}
}

func TestConfigEnvOverride(t *testing.T) {
	os.Clearenv()
	os.Setenv("CENTMEM_HOME", "/tmp/custom_home")
	os.Setenv("CENTMEM_DB", "/tmp/custom_db.sqlite")
	os.Setenv("CENTMEM_MODEL", "custom_model")
	os.Setenv("CENTMEM_MODEL_DIMS", "768")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Home != "/tmp/custom_home" {
		t.Errorf("expected custom home, got %q", cfg.Home)
	}

	if cfg.DBPath != "/tmp/custom_db.sqlite" {
		t.Errorf("expected custom DB path, got %q", cfg.DBPath)
	}

	if cfg.Model.Name != "custom_model" {
		t.Errorf("expected custom model name, got %q", cfg.Model.Name)
	}

	expectedModelPath := filepath.Join("/tmp/custom_home", "models", "custom_model.onnx")
	if cfg.Model.Path != expectedModelPath {
		t.Errorf("expected custom model path, got %q", cfg.Model.Path)
	}

	if cfg.Model.Dims != 768 {
		t.Errorf("expected custom dimensions 768, got %d", cfg.Model.Dims)
	}
}

func TestConfigEnsure(t *testing.T) {
	tempHome := filepath.Join(t.TempDir(), "centmem_test_home")
	cfg := config.Config{
		Home:   tempHome,
		DBPath: filepath.Join(tempHome, "centmem.db"),
		Model: config.ModelConfig{
			Name: "test-model",
			Path: filepath.Join(tempHome, "models", "test-model.onnx"),
			Dims: 128,
		},
	}

	if err := cfg.Ensure(); err != nil {
		t.Fatalf("failed to ensure config directories: %v", err)
	}

	info, err := os.Stat(tempHome)
	if err != nil {
		t.Fatalf("home dir not created: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("home is not a directory")
	}

	// Permissions check on UNIX-like filesystems
	mode := info.Mode().Perm()
	if os.PathSeparator == '/' && mode != 0700 {
		t.Errorf("expected permissions 0700 for home, got %o", mode)
	}

	modelDir := filepath.Dir(cfg.Model.Path)
	modelInfo, err := os.Stat(modelDir)
	if err != nil {
		t.Fatalf("models dir not created: %v", err)
	}
	if !modelInfo.IsDir() {
		t.Fatalf("models path is not a directory")
	}
}
