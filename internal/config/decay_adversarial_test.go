package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestAdversarial_SearchConfig_KnownKeysAndGet(t *testing.T) {
	// 1. Verify search.decay_half_life_days is registered in KnownConfigKeys
	found := false
	for _, k := range config.KnownConfigKeys {
		if k == "search.decay_half_life_days" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("search.decay_half_life_days not found in KnownConfigKeys")
	}

	cfg := config.Config{
		Search: config.SearchConfig{
			DecayHalfLifeDays: 30,
		},
	}

	// 2. Exact match
	v, err := config.GetConfigValue(cfg, "search.decay_half_life_days")
	if err != nil {
		t.Fatalf("GetConfigValue exact key failed: %v", err)
	}
	if days, ok := v.(int); !ok || days != 30 {
		t.Errorf("expected int 30, got %T: %v", v, v)
	}

	// 3. Table retrieval
	vTable, err := config.GetConfigValue(cfg, "search")
	if err != nil {
		t.Fatalf("GetConfigValue table failed: %v", err)
	}
	sc, ok := vTable.(config.SearchConfig)
	if !ok || sc.DecayHalfLifeDays != 30 {
		t.Errorf("expected SearchConfig with DecayHalfLifeDays=30, got %v", vTable)
	}

	// 4. Case-insensitivity and whitespace tolerance
	for _, keyVariant := range []string{
		"SEARCH.DECAY_HALF_LIFE_DAYS",
		"Search.Decay_Half_Life_Days",
		"  search.decay_half_life_days  ",
		"\tsearch.decay_half_life_days\n",
	} {
		vVar, err := config.GetConfigValue(cfg, keyVariant)
		if err != nil {
			t.Errorf("GetConfigValue variant %q failed: %v", keyVariant, err)
		}
		if days, ok := vVar.(int); !ok || days != 30 {
			t.Errorf("variant %q: expected 30, got %v", keyVariant, vVar)
		}
	}
}

func TestAdversarial_SearchConfig_SetConfigValue_AdversarialInputs(t *testing.T) {
	cfg := config.Config{
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
		Search:    config.DefaultSearchConfig(),
	}

	validCases := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"15", 15},
		{"30", 30},
		{"365", 365},
		{" 100 ", 100},
	}

	for _, tc := range validCases {
		if err := config.SetConfigValue(&cfg, "search.decay_half_life_days", tc.input); err != nil {
			t.Errorf("SetConfigValue(%q) failed: %v", tc.input, err)
		}
		if cfg.Search.DecayHalfLifeDays != tc.want {
			t.Errorf("SetConfigValue(%q): expected %d, got %d", tc.input, tc.want, cfg.Search.DecayHalfLifeDays)
		}
	}

	invalidCases := []struct {
		name  string
		input string
	}{
		{"negative_one", "-1"},
		{"negative_large", "-999"},
		{"float_decimal", "3.14"},
		{"float_one_dot_zero", "1.0"},
		{"scientific_notation", "1e5"},
		{"empty_string", ""},
		{"whitespace_only", "   "},
		{"alphabetic", "thirty"},
		{"boolean_true", "true"},
		{"boolean_false", "false"},
		{"int_overflow", "9999999999999999999999999999999999999999"},
		{"symbols", "$@#!"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			origVal := cfg.Search.DecayHalfLifeDays
			err := config.SetConfigValue(&cfg, "search.decay_half_life_days", tc.input)
			if err == nil {
				t.Errorf("expected error for invalid input %q, but got nil", tc.input)
			}
			// Verify config was not corrupted
			if cfg.Search.DecayHalfLifeDays != origVal {
				t.Errorf("config was corrupted on invalid input %q: was %d, now %d", tc.input, origVal, cfg.Search.DecayHalfLifeDays)
			}
		})
	}
}

func TestAdversarial_SearchConfig_PrecedenceAndValidationHierarchy(t *testing.T) {
	os.Unsetenv("CENTMEM_CAPTURE_BACKEND")
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "config.toml")

	// 1. Defaults when neither TOML nor ENV exist
	t.Run("default_level", func(t *testing.T) {
		emptyHome := t.TempDir()
		os.Setenv("CENTMEM_HOME", emptyHome)
		os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")
		os.Unsetenv("CENTMEM_CAPTURE_BACKEND")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load with defaults failed: %v", err)
		}
		if cfg.Search.DecayHalfLifeDays != 0 {
			t.Errorf("expected default 0, got %d", cfg.Search.DecayHalfLifeDays)
		}
	})

	// 2. TOML file overrides defaults
	t.Run("toml_overrides_default", func(t *testing.T) {
		os.Setenv("CENTMEM_HOME", tempDir)
		os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")

		fileCfg := config.Config{
			Home:      tempDir,
			Retention: config.DefaultRetention(),
			Capture:   config.DefaultCaptureConfig(),
			Search: config.SearchConfig{
				DecayHalfLifeDays: 25,
			},
		}
		if err := config.SaveTOML(tomlPath, fileCfg); err != nil {
			t.Fatalf("SaveTOML: %v", err)
		}

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Search.DecayHalfLifeDays != 25 {
			t.Errorf("expected TOML value 25, got %d", cfg.Search.DecayHalfLifeDays)
		}
	})

	// 3. Env variable overrides TOML
	t.Run("env_overrides_toml", func(t *testing.T) {
		os.Setenv("CENTMEM_HOME", tempDir)
		os.Setenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS", "45")
		defer os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Search.DecayHalfLifeDays != 45 {
			t.Errorf("expected ENV value 45 to override TOML 25, got %d", cfg.Search.DecayHalfLifeDays)
		}
	})

	// 4. Empty env variable leaves TOML intact
	t.Run("empty_env_preserves_toml", func(t *testing.T) {
		os.Setenv("CENTMEM_HOME", tempDir)
		os.Setenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS", "")
		defer os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Search.DecayHalfLifeDays != 25 {
			t.Errorf("expected empty ENV to preserve TOML value 25, got %d", cfg.Search.DecayHalfLifeDays)
		}
	})

	// 5. Invalid string in env variable ignored, preserves TOML
	t.Run("invalid_string_env_preserves_toml", func(t *testing.T) {
		os.Setenv("CENTMEM_HOME", tempDir)
		os.Setenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS", "not-a-number")
		defer os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Search.DecayHalfLifeDays != 25 {
			t.Errorf("expected invalid ENV to preserve TOML value 25, got %d", cfg.Search.DecayHalfLifeDays)
		}
	})

	// 6. Negative env variable triggers validation rejection in Load()
	t.Run("negative_env_fails_validation", func(t *testing.T) {
		os.Setenv("CENTMEM_HOME", tempDir)
		os.Setenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS", "-7")
		defer os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected Load() to fail on negative decay days from env")
		}
		if !strings.Contains(err.Error(), "search.decay_half_life_days") || !strings.Contains(err.Error(), "must be >= 0") {
			t.Errorf("expected validation error mentioning search.decay_half_life_days and >= 0, got: %v", err)
		}
	})

	// 7. Directly crafted TOML with negative decay triggers validation rejection in Load()
	t.Run("negative_toml_fails_validation", func(t *testing.T) {
		badHome := t.TempDir()
		os.Setenv("CENTMEM_HOME", badHome)
		os.Unsetenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS")

		badToml := `
[search]
decay_half_life_days = -20
`
		if err := os.WriteFile(filepath.Join(badHome, "config.toml"), []byte(badToml), 0600); err != nil {
			t.Fatalf("write bad toml: %v", err)
		}

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected Load() to reject negative decay_half_life_days in TOML")
		}
		if !strings.Contains(err.Error(), "search.decay_half_life_days") || !strings.Contains(err.Error(), "must be >= 0") {
			t.Errorf("expected validation error, got: %v", err)
		}
	})

	// 8. TOML round-trip fidelity
	t.Run("toml_roundtrip_fidelity", func(t *testing.T) {
		roundHome := t.TempDir()
		roundToml := filepath.Join(roundHome, "config.toml")

		orig := config.Config{
			Home:      roundHome,
			Retention: config.DefaultRetention(),
			Capture:   config.DefaultCaptureConfig(),
			Search: config.SearchConfig{
				DecayHalfLifeDays: 90,
			},
		}

		if err := config.SaveTOML(roundToml, orig); err != nil {
			t.Fatalf("SaveTOML: %v", err)
		}

		readBack, err := config.LoadTOML(roundToml)
		if err != nil {
			t.Fatalf("LoadTOML: %v", err)
		}

		if readBack.Search.DecayHalfLifeDays != 90 {
			t.Errorf("roundtrip failed: expected 90, got %d", readBack.Search.DecayHalfLifeDays)
		}
	})
}
