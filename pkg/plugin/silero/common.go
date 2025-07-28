package silero

import (
	"os"
	"path/filepath"
)

const (
	// ModelFileName is the expected ONNX model file name
	ModelFileName = "silero_vad.onnx"
	// DefaultThreshold is the default VAD threshold
	DefaultThreshold = 0.5
)

// getDefaultModelPath returns the default path for the Silero model.
func getDefaultModelPath() string {
	modelPath := os.Getenv("LK_MODEL_PATH")
	if modelPath == "" {
		homeDir, _ := os.UserHomeDir()
		modelPath = filepath.Join(homeDir, ".livekit", "models")
	}
	return filepath.Join(modelPath, ModelFileName)
}