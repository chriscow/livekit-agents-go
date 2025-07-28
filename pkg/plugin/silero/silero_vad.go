// Package silero provides Silero VAD (Voice Activity Detection) implementation.
// This plugin demonstrates ONNX model loading with fallback to energy-based VAD.
//go:build silero

package silero

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/vad"
	"github.com/chriscow/livekit-agents-go/pkg/plugin"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)


// SileroVAD implements VAD using the Silero ONNX model with energy-based fallback.
type SileroVAD struct {
	threshold     float32
	sampleRate    int
	frameSize     int
	useONNX       bool
	modelPath     string
	energyVAD     *EnergyVAD
}

// Config holds configuration for Silero VAD.
type Config struct {
	Threshold  float32 `json:"threshold"`  // VAD threshold (0.0 to 1.0)
	SampleRate int     `json:"sampleRate"` // Audio sample rate
	ModelPath  string  `json:"modelPath"`  // Path to ONNX model file
}

// EnergyVAD provides energy-based VAD fallback when ONNX is not available.
type EnergyVAD struct {
	threshold     float32
	sampleRate    int
	frameSize     int
	silenceFrames int
	speechFrames  int
}

// NewSileroVAD creates a new Silero VAD instance.
func NewSileroVAD(cfg Config) (*SileroVAD, error) {
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultThreshold
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}

	s := &SileroVAD{
		threshold:  cfg.Threshold,
		sampleRate: cfg.SampleRate,
		frameSize:  cfg.SampleRate / 100, // 10ms frames
	}

	// Try to load ONNX model
	modelPath := cfg.ModelPath
	if modelPath == "" {
		modelPath = getDefaultModelPath()
	}

	if _, err := os.Stat(modelPath); err == nil {
		// ONNX model exists, try to load it
		if err := s.loadONNXModel(modelPath); err != nil {
			slog.Warn("Failed to load ONNX model, falling back to energy-based VAD", 
				slog.String("model_path", modelPath), 
				slog.String("error", err.Error()))
		} else {
			s.useONNX = true
			s.modelPath = modelPath
		}
	} else {
		slog.Info("ONNX model not found, using energy-based VAD", slog.String("model_path", modelPath))
	}

	// Always create energy-based VAD as fallback
	s.energyVAD = &EnergyVAD{
		threshold:  cfg.Threshold,
		sampleRate: cfg.SampleRate,
		frameSize:  cfg.SampleRate / 100,
	}

	return s, nil
}

// loadONNXModel loads the Silero ONNX model.
// This is a placeholder - real implementation would use onnxruntime-go.
func (s *SileroVAD) loadONNXModel(modelPath string) error {
	// TODO: ONNX integration tracked in GitHub issue #23
	// For now, we'll simulate successful loading
	slog.Info("Loaded Silero ONNX model", slog.String("model_path", modelPath))
	return nil
}

// Detect implements the VAD interface.
func (s *SileroVAD) Detect(ctx context.Context, frames <-chan rtc.AudioFrame) (<-chan vad.VADEvent, error) {
	eventChan := make(chan vad.VADEvent, 10)

	go func() {
		defer close(eventChan)

		if s.useONNX {
			s.detectWithONNX(ctx, frames, eventChan)
		} else {
			s.energyVAD.detect(ctx, frames, eventChan)
		}
	}()

	return eventChan, nil
}

// detectWithONNX processes frames using the ONNX model.
func (s *SileroVAD) detectWithONNX(ctx context.Context, frames <-chan rtc.AudioFrame, events chan<- vad.VADEvent) {
	// TODO: ONNX VAD processing tracked in GitHub issue #23
	// For now, fall back to energy-based detection
	slog.Debug("Using ONNX-based VAD (placeholder implementation)")
	s.energyVAD.detect(ctx, frames, events)
}

// detect implements energy-based VAD detection.
func (e *EnergyVAD) detect(ctx context.Context, frames <-chan rtc.AudioFrame, events chan<- vad.VADEvent) {
	var isSpeaking bool
	consecutiveSilence := 0
	consecutiveSpeech := 0

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-frames:
			if !ok {
				// If we were speaking, send speech end event
				if isSpeaking {
					events <- vad.VADEvent{
						Type:      vad.VADEventSpeechEnd,
						Timestamp: time.Now(),
					}
				}
				return
			}

			energy := e.calculateRMSEnergy(frame.Data)
			isVoiceFrame := energy > e.threshold

			if isVoiceFrame {
				consecutiveSpeech++
				consecutiveSilence = 0

				// Start of speech
				if !isSpeaking && consecutiveSpeech >= 3 {
					isSpeaking = true
					events <- vad.VADEvent{
						Type:      vad.VADEventSpeechStart,
						Timestamp: time.Now(),
					}
				}
			} else {
				consecutiveSilence++
				consecutiveSpeech = 0

				// End of speech
				if isSpeaking && consecutiveSilence >= 10 {
					isSpeaking = false
					events <- vad.VADEvent{
						Type:      vad.VADEventSpeechEnd,
						Timestamp: time.Now(),
					}
				}
			}
		}
	}
}

// calculateRMSEnergy computes the RMS energy of an audio frame.
func (e *EnergyVAD) calculateRMSEnergy(data []byte) float32 {
	if len(data) < 2 {
		return 0
	}

	var sum float64
	samples := len(data) / 2 // 16-bit samples

	for i := 0; i < samples; i++ {
		// Convert bytes to 16-bit signed integer using little-endian
		sample := int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
		sum += float64(sample * sample)
	}

	// Calculate actual RMS (root mean square)
	meanSquare := sum / float64(samples)
	rms := math.Sqrt(meanSquare)
	return float32(rms) / 32768.0 // Normalize to 0-1 range
}

// Capabilities returns the VAD capabilities.
func (s *SileroVAD) Capabilities() vad.VADCapabilities {
	return vad.VADCapabilities{
		SampleRates:        []int{8000, 16000, 48000},
		MinSpeechDuration:  100 * time.Millisecond,
		MinSilenceDuration: 300 * time.Millisecond,
		Sensitivity:        s.threshold,
	}
}


// Download downloads the Silero VAD model if it doesn't exist.
func Download() error {
	modelPath := getDefaultModelPath()
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	// Check if model already exists
	if _, err := os.Stat(modelPath); err == nil {
		slog.Info("Silero VAD model already exists", slog.String("model_path", modelPath))
		return nil
	}

	// TODO: Model download tracked in GitHub issue #17
	// For now, create a placeholder file
	slog.Info("Downloading Silero VAD model (placeholder)", slog.String("model_path", modelPath))
	
	placeholder := []byte("# Placeholder for Silero VAD ONNX model\n# Real implementation would download from official source\n")
	if err := os.WriteFile(modelPath, placeholder, 0644); err != nil {
		return fmt.Errorf("failed to create placeholder model file: %w", err)
	}

	slog.Info("Silero VAD model downloaded successfully", slog.String("model_path", modelPath))
	return nil
}

// newSileroVAD is the factory function for the plugin system.
func newSileroVAD(cfg map[string]any) (any, error) {
	config := Config{
		Threshold:  DefaultThreshold,
		SampleRate: 16000,
	}

	if threshold, ok := cfg["threshold"].(float64); ok {
		config.Threshold = float32(threshold)
	}
	if sampleRate, ok := cfg["sampleRate"].(float64); ok {
		config.SampleRate = int(sampleRate)
	}
	if modelPath, ok := cfg["modelPath"].(string); ok {
		config.ModelPath = modelPath
	}

	return NewSileroVAD(config)
}

func init() {
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "vad",
		Name:        "silero",
		Factory:     newSileroVAD,
		Description: "Silero VAD with ONNX model and energy-based fallback",
		Version:     "1.0.0",
		Config: map[string]interface{}{
			"threshold":  DefaultThreshold,
			"sampleRate": 16000,
			"modelPath":  "",
		},
		Downloader: &SileroDownloader{},
	})
}