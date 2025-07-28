package turn

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/chriscow/livekit-agents-go/pkg/turn/internal"
)

// Downloader handles downloading turn detection models and associated files.
type Downloader struct {
	modelPath string
	client    *http.Client
}

// NewDownloader creates a new model downloader.
func NewDownloader(modelPath string) *Downloader {
	if modelPath == "" {
		modelPath = getDefaultModelPath()
	}

	return &Downloader{
		modelPath: modelPath,
		client:    &http.Client{},
	}
}

// DownloadAll downloads all available models.
func (d *Downloader) DownloadAll() error {
	for _, model := range internal.AllModels {
		if err := d.DownloadModel(model); err != nil {
			return fmt.Errorf("failed to download model %s: %w", model.Name, err)
		}
	}
	return nil
}

// DownloadModel downloads a specific model and its associated files.
func (d *Downloader) DownloadModel(model internal.ModelInfo) error {
	modelDir := internal.GetModelPath(d.modelPath, model.Revision)

	// Create model directory
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}
	// Download each required file
	for _, filename := range model.Files {
		filePath := filepath.Join(modelDir, filename)

		// Ensure parent directories exist for nested paths (e.g. onnx/model_q8.onnx)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directories for %s: %w", filename, err)
		}

		// Skip if file already exists and is valid
		if d.isValidFile(filePath, model.SHA256Hash, filename) {
			fmt.Printf("✓ %s already exists and is valid\n", filename)
			continue
		}

		fmt.Printf("Downloading %s...\n", filename)
		if err := d.downloadFile(model, filename, filePath); err != nil {
			// Clean up partial download
			os.Remove(filePath)
			return fmt.Errorf("failed to download %s: %w", filename, err)
		}

		fmt.Printf("✓ Downloaded %s\n", filename)
	}

	fmt.Printf("✓ Model '%s' downloaded successfully\n", model.Name)
	return nil
}

// downloadFile downloads a single file from the model repository.
func (d *Downloader) downloadFile(model internal.ModelInfo, filename, destination string) error {
	// Construct download URL (using Hugging Face Hub format)
	url := fmt.Sprintf("https://huggingface.co/%s/resolve/%s/%s",
		model.Repo, model.Revision, filename)

	// Create HTTP request
	resp, err := d.client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Create destination file
	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data with progress (for large files)
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// isValidFile checks if a file exists and has the correct hash.
func (d *Downloader) isValidFile(filePath, expectedHash, filename string) bool {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	// Check file size
	if info.Size() == 0 {
		return false
	}

	// Get expected hash from our mapping
	expectedHash = internal.FileHashes[filename]
	if expectedHash == "" {
		// If no hash is defined, just check existence
		return true
	}

	// Verify SHA-256 hash
	return d.verifyFileHash(filePath, expectedHash)
}

// verifyFileHash computes and verifies the SHA-256 hash of a file.
func (d *Downloader) verifyFileHash(filePath, expectedHash string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	return actualHash == expectedHash
}

// GetModelStatus returns the download status of all models.
func (d *Downloader) GetModelStatus() map[string]bool {
	status := make(map[string]bool)

	for _, model := range internal.AllModels {
		isComplete := true
		for _, filename := range model.Files {
			filePath := internal.GetModelFilePath(d.modelPath, model.Revision, filename)
			if !d.isValidFile(filePath, model.SHA256Hash, filename) {
				isComplete = false
				break
			}
		}
		status[model.Name] = isComplete
	}

	return status
}
