package turn

import (
	"os"
	"testing"
)

func TestNewDetector(t *testing.T) {
	// Test with default config - may fail to load models, which is OK for tests
	config := DetectorConfig{
		Model: "english",
	}

	detector, err := NewDetector(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if detector == nil {
		t.Fatal("Expected detector to be created")
	}

	// Note: SupportsLanguage may return false if models aren't available,
	// which is expected in test environment
	t.Logf("Detector supports en-US: %v", detector.SupportsLanguage("en-US"))
}

func TestNewDetectorWithRemote(t *testing.T) {
	// Set remote URL
	remoteURL := "http://localhost:8080/predict"
	config := DetectorConfig{
		Model:     "english",
		RemoteURL: remoteURL,
	}

	detector, err := NewDetector(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should create a remote detector
	if _, ok := detector.(*RemoteDetector); !ok {
		t.Error("Expected RemoteDetector to be created")
	}
}

func TestNewDetectorWithEnvVar(t *testing.T) {
	// Set environment variable
	originalEnv := os.Getenv("LIVEKIT_REMOTE_EOT_URL")
	defer os.Setenv("LIVEKIT_REMOTE_EOT_URL", originalEnv)

	os.Setenv("LIVEKIT_REMOTE_EOT_URL", "http://localhost:8080/predict")

	config := DetectorConfig{
		Model: "english",
	}

	detector, err := NewDetector(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should create a remote detector
	if _, ok := detector.(*RemoteDetector); !ok {
		t.Error("Expected RemoteDetector to be created")
	}
}

func TestNewDetectorInvalidModel(t *testing.T) {
	config := DetectorConfig{
		Model: "invalid",
	}

	_, err := NewDetector(config)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
}

func TestNewDefaultDetector(t *testing.T) {
	detector, err := NewDefaultDetector()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if detector == nil {
		t.Fatal("Expected detector to be created")
	}
}
