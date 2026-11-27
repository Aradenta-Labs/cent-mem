package config_test

import (
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestHooksConfig_DefaultsTOMLAndKeys(t *testing.T) {
	tmp := t.TempDir()
	tomlPath := filepath.Join(tmp, "config.toml")

	// Missing file returns disabled defaults with 5s timeout
	cfg, err := config.LoadTOML(tomlPath)
	if err != nil {
		t.Fatalf("LoadTOML missing: %v", err)
	}
	if cfg.Hooks.Enabled {
		t.Errorf("expected hooks.enabled=false by default")
	}
	if cfg.Hooks.TimeoutSeconds != 5 {
		t.Errorf("expected hooks.timeout_seconds=5 by default, got %d", cfg.Hooks.TimeoutSeconds)
	}

	// Set via keys API and persist
	if err := config.SetConfigValue(&cfg, "hooks.enabled", "true"); err != nil {
		t.Fatalf("set hooks.enabled: %v", err)
	}
	if err := config.SetConfigValue(&cfg, "hooks.proposals_created", "https://example.com/created"); err != nil {
		t.Fatalf("set hooks.proposals_created: %v", err)
	}
	if err := config.SetConfigValue(&cfg, "hooks.proposals_applied", "http://127.0.0.1:8080/applied"); err != nil {
		t.Fatalf("set hooks.proposals_applied: %v", err)
	}
	if err := config.SetConfigValue(&cfg, "hooks.proposals_dismissed", "https://example.com/dismissed"); err != nil {
		t.Fatalf("set hooks.proposals_dismissed: %v", err)
	}
	if err := config.SetConfigValue(&cfg, "hooks.secret", "MY_HOOK_SECRET"); err != nil {
		t.Fatalf("set hooks.secret: %v", err)
	}
	if err := config.SetConfigValue(&cfg, "hooks.timeout_seconds", "10"); err != nil {
		t.Fatalf("set hooks.timeout_seconds: %v", err)
	}

	if err := config.SaveTOML(tomlPath, cfg); err != nil {
		t.Fatalf("SaveTOML: %v", err)
	}

	reloaded, err := config.LoadTOML(tomlPath)
	if err != nil {
		t.Fatalf("LoadTOML reload: %v", err)
	}
	if reloaded.Hooks != cfg.Hooks {
		t.Errorf("reloaded hooks mismatch: got %+v, want %+v", reloaded.Hooks, cfg.Hooks)
	}

	val, err := config.GetConfigValue(reloaded, "hooks")
	if err != nil {
		t.Fatalf("GetConfigValue(hooks): %v", err)
	}
	if h, ok := val.(config.HooksConfig); !ok || !h.Enabled || h.TimeoutSeconds != 10 {
		t.Errorf("unexpected hooks table value: %#v", val)
	}
}

func TestHooksConfig_ValidationRejectsInvalidURLsAndTimeouts(t *testing.T) {
	cfg := config.Config{
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
		Hooks:     config.DefaultHooksConfig(),
	}

	for _, bad := range []string{
		"ftp://example.com/hook",
		"file:///tmp/hook",
		"ws://localhost:9000",
		"http://",
		"http://:8080/hook",
		"https://:443",
		"not-a-url",
	} {
		if err := config.SetConfigValue(&cfg, "hooks.proposals_created", bad); err == nil {
			t.Errorf("expected error for invalid URL %q", bad)
		}
		if err := config.SetConfigValue(&cfg, "hooks.proposals_applied", bad); err == nil {
			t.Errorf("expected error for invalid URL %q", bad)
		}
		if err := config.SetConfigValue(&cfg, "hooks.proposals_dismissed", bad); err == nil {
			t.Errorf("expected error for invalid URL %q", bad)
		}
	}

	// Empty URL is allowed (clears hook)
	if err := config.SetConfigValue(&cfg, "hooks.proposals_created", ""); err != nil {
		t.Errorf("expected empty URL to be allowed, got: %v", err)
	}

	// Timeout <= 0 rejected
	for _, badTimeout := range []string{"0", "-3", "abc"} {
		if err := config.SetConfigValue(&cfg, "hooks.timeout_seconds", badTimeout); err == nil {
			t.Errorf("expected error for timeout %q", badTimeout)
		}
	}
}

func TestHooksConfig_EnvOverridesConflictWithTOML(t *testing.T) {
	home := t.TempDir()
	tomlPath := filepath.Join(home, "config.toml")

	// Seed config.toml with conflicting [hooks] values
	fileCfg, err := config.LoadTOML(tomlPath)
	if err != nil {
		t.Fatalf("LoadTOML: %v", err)
	}
	fileCfg.Hooks = config.HooksConfig{
		Enabled:            true,
		ProposalsCreated:   "https://toml.example/created",
		ProposalsApplied:   "https://toml.example/applied",
		ProposalsDismissed: "https://toml.example/dismissed",
		Secret:             "toml-secret",
		TimeoutSeconds:     9,
	}
	if err := config.SaveTOML(tomlPath, fileCfg); err != nil {
		t.Fatalf("SaveTOML: %v", err)
	}

	t.Setenv("CENTMEM_HOME", home)
	t.Setenv("CENTMEM_HOOKS_ENABLED", "false")
	t.Setenv("CENTMEM_HOOKS_PROPOSALS_CREATED", "https://env.example/created")
	t.Setenv("CENTMEM_HOOKS_PROPOSALS_APPLIED", "https://env.example/applied")
	t.Setenv("CENTMEM_HOOKS_PROPOSALS_DISMISSED", "https://env.example/dismissed")
	t.Setenv("CENTMEM_HOOKS_SECRET", "env-secret")
	t.Setenv("CENTMEM_HOOKS_TIMEOUT_SECONDS", "12")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load with env overrides: %v", err)
	}
	if loaded.Hooks.Enabled {
		t.Errorf("expected CENTMEM_HOOKS_ENABLED=false to override TOML true")
	}
	if loaded.Hooks.ProposalsCreated != "https://env.example/created" {
		t.Errorf("ProposalsCreated = %q, want env value", loaded.Hooks.ProposalsCreated)
	}
	if loaded.Hooks.ProposalsApplied != "https://env.example/applied" {
		t.Errorf("ProposalsApplied = %q, want env value", loaded.Hooks.ProposalsApplied)
	}
	if loaded.Hooks.ProposalsDismissed != "https://env.example/dismissed" {
		t.Errorf("ProposalsDismissed = %q, want env value", loaded.Hooks.ProposalsDismissed)
	}
	if loaded.Hooks.Secret != "env-secret" {
		t.Errorf("Secret = %q, want env-secret", loaded.Hooks.Secret)
	}
	if loaded.Hooks.TimeoutSeconds != 12 {
		t.Errorf("TimeoutSeconds = %d, want 12", loaded.Hooks.TimeoutSeconds)
	}

	// Invalid env timeout must fail Load()
	t.Setenv("CENTMEM_HOOKS_TIMEOUT_SECONDS", "0")
	if _, err := config.Load(); err == nil {
		t.Errorf("expected config.Load to reject CENTMEM_HOOKS_TIMEOUT_SECONDS=0")
	}

	// Invalid env URL scheme must fail Load()
	t.Setenv("CENTMEM_HOOKS_TIMEOUT_SECONDS", "5")
	t.Setenv("CENTMEM_HOOKS_PROPOSALS_CREATED", "http://:8080/bad")
	if _, err := config.Load(); err == nil {
		t.Errorf("expected config.Load to reject hostless port URL")
	}
}
