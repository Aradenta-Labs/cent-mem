package capture

import (
	"github.com/aradenta-labs/cent-mem/internal/config"
)

// CaptureConfig is aliased from internal/config for package capture consumers.
type CaptureConfig = config.CaptureConfig

var (
	// DefaultCategories returns the default list of memory capture categories.
	DefaultCategories = config.DefaultCategories
	// DefaultCaptureConfig returns the default configuration for capture.
	DefaultCaptureConfig = config.DefaultCaptureConfig
	// ValidateCaptureConfig validates that capture configuration parameters are well-formed.
	ValidateCaptureConfig = config.ValidateCaptureConfig
)
