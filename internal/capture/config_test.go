package capture_test

import (
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestCaptureConfig_Defaults(t *testing.T) {
	cfg := capture.DefaultCaptureConfig()

	if cfg.Enabled {
		t.Errorf("expected default Enabled to be false, got true")
	}
	if cfg.Harness != "antigravity" {
		t.Errorf("expected default Harness 'antigravity', got %q", cfg.Harness)
	}
	if cfg.Backend != "heuristic" {
		t.Errorf("expected default Backend 'heuristic', got %q", cfg.Backend)
	}
	if cfg.ConfidenceThreshold != 0.7 {
		t.Errorf("expected default ConfidenceThreshold 0.7, got %f", cfg.ConfidenceThreshold)
	}
	if len(cfg.Categories) != 7 {
		t.Errorf("expected 7 default categories, got %d", len(cfg.Categories))
	}
	if err := capture.ValidateCaptureConfig(cfg); err != nil {
		t.Errorf("expected default config to be valid, got %v", err)
	}
}

func TestCaptureConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*capture.CaptureConfig)
		wantErr bool
	}{
		{
			name: "valid heuristic",
			mutate: func(c *capture.CaptureConfig) {
				c.Backend = "heuristic"
			},
			wantErr: false,
		},
		{
			name: "valid local-llm",
			mutate: func(c *capture.CaptureConfig) {
				c.Backend = "local-llm"
			},
			wantErr: false,
		},
		{
			name: "valid openai-compatible",
			mutate: func(c *capture.CaptureConfig) {
				c.Backend = "openai-compatible"
			},
			wantErr: false,
		},
		{
			name: "invalid backend",
			mutate: func(c *capture.CaptureConfig) {
				c.Backend = "unknown-backend"
			},
			wantErr: true,
		},
		{
			name: "invalid threshold high",
			mutate: func(c *capture.CaptureConfig) {
				c.ConfidenceThreshold = 1.5
			},
			wantErr: true,
		},
		{
			name: "invalid threshold negative",
			mutate: func(c *capture.CaptureConfig) {
				c.ConfidenceThreshold = -0.1
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := capture.DefaultCaptureConfig()
			tt.mutate(&cfg)
			err := capture.ValidateCaptureConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCaptureConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
