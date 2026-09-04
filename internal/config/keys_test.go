package config_test

import (
	"reflect"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestConfig_GetConfigValue_AllKeys(t *testing.T) {
	cfg := config.Config{
		Model: config.ModelConfig{
			Name: "bge-small-en-v1.5",
			Dims: 384,
		},
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
	}

	for _, key := range config.KnownConfigKeys {
		val, err := config.GetConfigValue(cfg, key)
		if err != nil {
			t.Errorf("GetConfigValue failed for valid key %q: %v", key, err)
		}
		if val == nil {
			t.Errorf("GetConfigValue returned nil for valid key %q", key)
		}
	}
}

func TestConfig_GetConfigValue_TableNames(t *testing.T) {
	cfg := config.Config{
		Model:     config.ModelConfig{Name: "bge", Dims: 384},
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
		Search:    config.DefaultSearchConfig(),
	}

	for _, tbl := range []string{"model", "retention", "capture", "search"} {
		val, err := config.GetConfigValue(cfg, tbl)
		if err != nil {
			t.Errorf("GetConfigValue failed for table %q: %v", tbl, err)
		}
		if val == nil {
			t.Errorf("GetConfigValue returned nil for table %q", tbl)
		}
	}
}

func TestConfig_GetConfigValue_UnknownKey(t *testing.T) {
	cfg := config.Config{
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
	}

	_, err := config.GetConfigValue(cfg, "nonexistent.key")
	if err == nil {
		t.Error("expected error for unknown config key")
	}
}

func TestConfig_SetConfigValue_Types(t *testing.T) {
	cfg := config.Config{
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
	}

	// Boolean
	if err := config.SetConfigValue(&cfg, "capture.enabled", "true"); err != nil {
		t.Fatalf("set capture.enabled true: %v", err)
	}
	if !cfg.Capture.Enabled {
		t.Errorf("expected Enabled=true")
	}

	if err := config.SetConfigValue(&cfg, "capture.enabled", "0"); err != nil {
		t.Fatalf("set capture.enabled 0: %v", err)
	}
	if cfg.Capture.Enabled {
		t.Errorf("expected Enabled=false")
	}

	if err := config.SetConfigValue(&cfg, "capture.enabled", "invalid-bool"); err == nil {
		t.Error("expected error for invalid boolean value")
	}

	// Harness
	if err := config.SetConfigValue(&cfg, "capture.harness", "claude-code"); err != nil {
		t.Fatalf("set harness: %v", err)
	}
	if cfg.Capture.Harness != "claude-code" {
		t.Errorf("expected claude-code, got %q", cfg.Capture.Harness)
	}

	if err := config.SetConfigValue(&cfg, "capture.harness", "invalid-harness"); err == nil {
		t.Error("expected error for invalid harness")
	}

	// Float (ConfidenceThreshold)
	if err := config.SetConfigValue(&cfg, "capture.confidence_threshold", "0.85"); err != nil {
		t.Fatalf("set confidence: %v", err)
	}
	if cfg.Capture.ConfidenceThreshold != 0.85 {
		t.Errorf("expected 0.85, got %f", cfg.Capture.ConfidenceThreshold)
	}

	if err := config.SetConfigValue(&cfg, "capture.confidence_threshold", "1.5"); err == nil {
		t.Error("expected error for out of range float (> 1.0)")
	}
	if err := config.SetConfigValue(&cfg, "capture.confidence_threshold", "-0.1"); err == nil {
		t.Error("expected error for negative float (< 0.0)")
	}

	// Int (Retention)
	if err := config.SetConfigValue(&cfg, "retention.note_summarize_after_days", "60"); err != nil {
		t.Fatalf("set retention: %v", err)
	}
	if cfg.Retention.NoteSummarizeAfterDays != 60 {
		t.Errorf("expected 60, got %d", cfg.Retention.NoteSummarizeAfterDays)
	}
	if err := config.SetConfigValue(&cfg, "retention.note_summarize_after_days", "-5"); err == nil {
		t.Error("expected error for negative retention days")
	}

	// String slices (Categories & Triggers)
	if err := config.SetConfigValue(&cfg, "capture.categories", "decision,fact,custom_one"); err != nil {
		t.Fatalf("set categories: %v", err)
	}
	expectedCats := []string{"decision", "fact", "custom_one"}
	if !reflect.DeepEqual(cfg.Capture.Categories, expectedCats) {
		t.Errorf("categories expected %+v, got %+v", expectedCats, cfg.Capture.Categories)
	}

	if err := config.SetConfigValue(&cfg, "capture.triggers", "message,session-end"); err != nil {
		t.Fatalf("set triggers: %v", err)
	}
	expectedTrigs := []string{"message", "session-end"}
	if !reflect.DeepEqual(cfg.Capture.Triggers, expectedTrigs) {
		t.Errorf("triggers expected %+v, got %+v", expectedTrigs, cfg.Capture.Triggers)
	}

	if err := config.SetConfigValue(&cfg, "capture.triggers", "all"); err != nil {
		t.Fatalf("set triggers all: %v", err)
	}
	expectedAll := []string{"message", "session-end", "on-demand"}
	if !reflect.DeepEqual(cfg.Capture.Triggers, expectedAll) {
		t.Errorf("triggers all expected %+v, got %+v", expectedAll, cfg.Capture.Triggers)
	}

	// Search (decay_half_life_days)
	if err := config.SetConfigValue(&cfg, "search.decay_half_life_days", "45"); err != nil {
		t.Fatalf("set search.decay_half_life_days: %v", err)
	}
	if cfg.Search.DecayHalfLifeDays != 45 {
		t.Errorf("expected 45, got %d", cfg.Search.DecayHalfLifeDays)
	}
	if err := config.SetConfigValue(&cfg, "search.decay_half_life_days", "-10"); err == nil {
		t.Error("expected error for negative decay_half_life_days")
	}
	if err := config.SetConfigValue(&cfg, "search.decay_half_life_days", "invalid"); err == nil {
		t.Error("expected error for non-integer decay_half_life_days")
	}
}

func TestConfig_SetConfigValue_BackendValidation(t *testing.T) {
	cfg := config.Config{
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
	}

	for _, b := range []string{"local-llm", "heuristic", "openai-compatible"} {
		if err := config.SetConfigValue(&cfg, "capture.backend", b); err != nil {
			t.Errorf("expected backend %q to be valid, got: %v", b, err)
		}
	}

	if err := config.SetConfigValue(&cfg, "capture.backend", "random-llm"); err == nil {
		t.Error("expected error for unsupported backend")
	}
}
