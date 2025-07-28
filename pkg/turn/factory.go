package turn

import (
	"fmt"
	"os"
)

// DetectorConfig holds configuration for creating turn detectors.
type DetectorConfig struct {
	Model     string // "english" or "multilingual"
	ModelPath string // Path to model files (optional, uses default if empty)
	RemoteURL string // Remote inference URL (optional)
}

// NewDetector creates a turn detector based on the provided configuration.
// If RemoteURL is set, creates a RemoteDetector with local fallback.
// Otherwise, creates an ONNX-based local detector.
func NewDetector(config DetectorConfig) (Detector, error) {
	// Check for remote URL in config or environment
	remoteURL := config.RemoteURL
	if remoteURL == "" {
		remoteURL = os.Getenv("LIVEKIT_REMOTE_EOT_URL")
	}

	// Validate model name
	if config.Model == "" {
		config.Model = "english" // Default to English model
	}

	switch config.Model {
	case "english", "multilingual":
		// valid
	default:
		return nil, fmt.Errorf("invalid model name: %s (supported: english|multilingual)", config.Model)
	}

	// Create local detector (used directly or as fallback)
	localDetector, err := NewONNXDetector(config.Model, config.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX detector: %w", err)
	}

	// If remote URL is configured, create remote detector with local fallback
	if remoteURL != "" {
		return NewRemoteDetector(remoteURL, localDetector), nil
	}

	// Use local detector directly
	return localDetector, nil
}

// NewDefaultDetector creates a detector with default configuration.
func NewDefaultDetector() (Detector, error) {
	return NewDetector(DetectorConfig{Model: "english"})
}
