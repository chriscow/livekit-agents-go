package internal

import "path/filepath"

// ModelInfo holds metadata for a turn-detection model revision.
type ModelInfo struct {
	Name       string // "english", "multilingual"
	Repo       string
	Revision   string
	Size       int64
	SHA256Hash string // SHA of the ONNX file, empty until Phase 5.5-B
	Files      []string
}

// NOTE: SHA-256 hashes are filled in Phase 5.5-B once the final artefacts are cut.
// For Phase 5.5-A the downloader verifies hashes only if provided.

var (
	EnglishModel = ModelInfo{
		Name:       "english",
		Repo:       "livekit/turn-detector",
		Revision:   "v1.2.2-en",
		Size:       66 * 1024 * 1024, // ~66 MB based on docs
		SHA256Hash: "fdd695a99bda01155fb0b5ce71d34cb9fd3902c62496db7a6c2c7bdeac310ac7",
		Files: []string{
			"onnx/model_q8.onnx",
			"tokenizer.json",
			"languages.json",
		},
	}

	MultilingualModel = ModelInfo{
		Name:       "multilingual",
		Repo:       "livekit/turn-detector",
		Revision:   "v0.3.0-intl",
		Size:       281 * 1024 * 1024, // ~281 MB based on docs
		SHA256Hash: "",                // to be populated in Phase 5.5-B
		Files: []string{
			"onnx/model_q8.onnx",
			"tokenizer.json",
			"languages.json",
		},
	}

	// AllModels enumerates every model the downloader must handle.
	AllModels = []ModelInfo{EnglishModel, MultilingualModel}
)

// FileHashes maps filenames (including path) to their SHA-256 hashes.
var FileHashes = map[string]string{
	"onnx/model_q8.onnx": "fdd695a99bda01155fb0b5ce71d34cb9fd3902c62496db7a6c2c7bdeac310ac7",
	"tokenizer.json":     "c8219a662de786c94771323c3500377970f5eaa3afbeaef9390c9a51db9f7884",
	"languages.json":     "a9b71f62240293b05e6fa2b75ffc997ae00cefcc8da8b9567e39e3c356b7ee1",
}

// GetModelPath returns the directory where a revision is stored.
func GetModelPath(basePath, revision string) string {
	return filepath.Join(basePath, "turn-detector", revision)
}

// GetModelFilePath returns the absolute path to a specific file for a revision.
func GetModelFilePath(basePath, revision, filename string) string {
	return filepath.Join(GetModelPath(basePath, revision), filename)
}
