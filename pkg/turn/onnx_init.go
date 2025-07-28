package turn

import (
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	ortOnce    sync.Once
	ortInitErr error
)

// ensureOrtEnv initializes the ONNX runtime environment exactly once per process.
// This prevents duplicate schema registration warnings that occur when multiple
// detectors initialize the environment concurrently.
func ensureOrtEnv() error {
	ortOnce.Do(func() {
		// Set library path if specified via environment variable
		if libPath := os.Getenv("ONNXRUNTIME_LIB"); libPath != "" {
			ort.SetSharedLibraryPath(libPath)
		} else if runtime.GOOS == "darwin" {
			// Default to Homebrew path on macOS as fallback
			ort.SetSharedLibraryPath("/opt/homebrew/lib/libonnxruntime.dylib")
		}

		// Initialize the environment once
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}