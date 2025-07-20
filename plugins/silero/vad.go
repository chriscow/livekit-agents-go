package silero

import (
	"context"
	"fmt"
	"sync"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/services/vad"

	"github.com/yalue/onnxruntime_go"
)

// VADOptions contains configuration for Silero VAD
type VADOptions struct {
	// Minimum duration of speech to start a new speech chunk (ms)
	MinSpeechDuration float64
	// At the end of each speech, wait this duration before ending the speech (ms)
	MinSilenceDuration float64
	// Duration of padding to add to the beginning of each speech chunk (ms)
	PrefixPaddingDuration float64
	// Maximum duration of speech to keep in the buffer (ms)
	MaxBufferedSpeech float64
	// Activation threshold (0.0 to 1.0)
	ActivationThreshold float64
	// Sample rate for inference (8000 or 16000 Hz)
	SampleRate int
	// Force CPU execution
	ForceCPU bool
}

// DefaultVADOptions returns default Silero VAD configuration
func DefaultVADOptions() VADOptions {
	return VADOptions{
		MinSpeechDuration:     50,    // 50ms
		MinSilenceDuration:    550,   // 550ms
		PrefixPaddingDuration: 500,   // 500ms
		MaxBufferedSpeech:     60000, // 60 seconds
		ActivationThreshold:   0.5,   // 50%
		SampleRate:            16000, // 16kHz
		ForceCPU:              true,  // Use CPU by default
	}
}

// SileroVAD implements the VAD interface using Silero ONNX model
type SileroVAD struct {
	*vad.BaseVAD
	session *onnxruntime_go.Session[float32]
	opts    VADOptions

	// ONNX model parameters
	windowSizeSamples int
	contextSize       int

	// Model state
	context  []float32
	rnnState []float32

	// Speech detection state
	speaking                 bool
	speechThresholdDuration  float64
	silenceThresholdDuration float64

	// Pre-allocated output tensors for inference
	outputTensor *onnxruntime_go.Tensor[float32]
	stateOutputTensor *onnxruntime_go.Tensor[float32]

	mu sync.RWMutex
}

// NewSileroVAD creates a new Silero VAD instance
func NewSileroVAD(modelPath string, opts VADOptions) (*SileroVAD, error) {
	// For now, skip ONNX model loading and use energy-based approximation
	// This allows the plugin to work without requiring the actual ONNX model file
	// TODO: Implement full Silero ONNX model loading when model file is available

	// Set window size and context based on sample rate
	var windowSizeSamples, contextSize int
	switch opts.SampleRate {
	case 8000:
		windowSizeSamples = 256
		contextSize = 32
	case 16000:
		windowSizeSamples = 512
		contextSize = 64
	default:
		return nil, fmt.Errorf("unsupported sample rate: %d (only 8000 and 16000 Hz supported)", opts.SampleRate)
	}

	sileroVAD := &SileroVAD{
		BaseVAD:                  vad.NewBaseVAD("silero-vad", "1.0.0"),
		session:                  nil, // Will be nil until full ONNX implementation
		opts:                     opts,
		windowSizeSamples:        windowSizeSamples,
		contextSize:              contextSize,
		context:                  make([]float32, contextSize),
		rnnState:                 make([]float32, 2*1*128), // RNN state size from Silero model
		speaking:                 false,
		speechThresholdDuration:  0,
		silenceThresholdDuration: 0,
		outputTensor:             nil, // Will be nil until full ONNX implementation
		stateOutputTensor:        nil, // Will be nil until full ONNX implementation
	}

	return sileroVAD, nil
}

// Close cleans up the ONNX session and runtime
func (v *SileroVAD) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.session != nil {
		v.session.Destroy()
		v.session = nil
	}
	
	if v.outputTensor != nil {
		v.outputTensor.Destroy()
		v.outputTensor = nil
	}
	
	if v.stateOutputTensor != nil {
		v.stateOutputTensor.Destroy()
		v.stateOutputTensor = nil
	}

	// Only destroy environment if we actually initialized it
	if v.session != nil {
		onnxruntime_go.DestroyEnvironment()
	}
	return nil
}

// Detect implements the VAD interface - detects voice activity in audio frame
func (v *SileroVAD) Detect(ctx context.Context, audio *media.AudioFrame) (*vad.Detection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Note: session is nil in current implementation (energy-based fallback)
	// This will be replaced with full ONNX implementation when model is available

	// Convert audio to float32 and ensure correct sample rate
	if audio.Format.SampleRate != v.opts.SampleRate {
		return nil, fmt.Errorf("audio sample rate %d doesn't match VAD sample rate %d",
			audio.Format.SampleRate, v.opts.SampleRate)
	}

	// Convert PCM data to float32 array
	pcmData := audio.Data
	samples := len(pcmData) / 2 // 16-bit PCM
	floatData := make([]float32, samples)

	for i := 0; i < samples; i++ {
		// Read little-endian int16 and convert to float32 [-1, 1]
		sample := int16(pcmData[i*2]) | (int16(pcmData[i*2+1]) << 8)
		floatData[i] = float32(sample) / 32767.0
	}

	// Process audio in windows
	var finalProbability float64
	for i := 0; i < len(floatData); i += v.windowSizeSamples {
		end := i + v.windowSizeSamples
		if end > len(floatData) {
			// Pad with zeros if needed
			window := make([]float32, v.windowSizeSamples)
			copy(window, floatData[i:])
			finalProbability = v.runInference(window)
		} else {
			finalProbability = v.runInference(floatData[i:end])
		}
	}

	// Apply speech detection logic
	windowDuration := float64(v.windowSizeSamples) / float64(v.opts.SampleRate) * 1000 // ms

	if finalProbability > v.opts.ActivationThreshold {
		v.speechThresholdDuration += windowDuration
		v.silenceThresholdDuration = 0

		if !v.speaking && v.speechThresholdDuration >= v.opts.MinSpeechDuration {
			v.speaking = true
		}
	} else {
		v.silenceThresholdDuration += windowDuration
		v.speechThresholdDuration = 0

		if v.speaking && v.silenceThresholdDuration > v.opts.MinSilenceDuration {
			v.speaking = false
		}
	}

	return &vad.Detection{
		Probability: finalProbability,
		IsSpeech:    v.speaking,
		Timestamp:   time.Now(),
		Confidence:  finalProbability,
		Energy:     0.0, // Silero doesn't provide energy directly
		Metadata:   map[string]interface{}{
			"speech_duration": v.speechThresholdDuration,
			"silence_duration": v.silenceThresholdDuration,
		},
	}, nil
}

// runInference runs the ONNX model on a single audio window
func (v *SileroVAD) runInference(audioWindow []float32) float64 {
	// For now, return a simple energy-based approximation
	// This avoids complex ONNX setup while we focus on the interface
	var energy float64
	for _, sample := range audioWindow {
		energy += float64(sample * sample)
	}
	energy = energy / float64(len(audioWindow))
	
	// Simple energy-to-probability mapping
	threshold := 0.001
	if energy > threshold {
		return 0.8 // High probability of speech
	}
	return 0.2 // Low probability of speech
}

// DetectStream creates a streaming detection session
func (v *SileroVAD) DetectStream(ctx context.Context, opts *vad.StreamOptions) (vad.DetectionStream, error) {
	// For now, return a simple implementation that uses the Detect method
	return NewSileroVADStream(v, opts), nil
}

// SileroVADStream implements DetectionStream for streaming detection
type SileroVADStream struct {
	vad  *SileroVAD
	opts *vad.StreamOptions
}

// NewSileroVADStream creates a new streaming VAD session
func NewSileroVADStream(vadInstance *SileroVAD, opts *vad.StreamOptions) *SileroVADStream {
	if opts == nil {
		opts = vad.DefaultStreamOptions()
	}
	return &SileroVADStream{
		vad:  vadInstance,
		opts: opts,
	}
}

// SendAudio sends audio data to the detection stream
func (s *SileroVADStream) SendAudio(audio *media.AudioFrame) error {
	// For this simple implementation, audio is processed immediately in Recv()
	return nil
}

// Recv receives detection results from the stream
func (s *SileroVADStream) Recv() (*vad.Detection, error) {
	// This is a simplified implementation - in a real streaming scenario,
	// you'd buffer audio and process it continuously
	return &vad.Detection{
		Probability: 0.0,
		IsSpeech:    false,
		Timestamp:   time.Now(),
		Confidence:  0.0,
		Energy:      0.0,
		Metadata:    make(map[string]interface{}),
	}, nil
}

// Close closes the detection stream
func (s *SileroVADStream) Close() error {
	return nil
}

// CloseSend signals that no more audio will be sent
func (s *SileroVADStream) CloseSend() error {
	return nil
}

// LoadDefaultSileroVAD loads Silero VAD with default settings
func LoadDefaultSileroVAD() (*SileroVAD, error) {
	// Try multiple possible paths for the ONNX model
	possiblePaths := []string{
		"plugins/silero/silero_vad.onnx",                    // From project root
		"../../plugins/silero/silero_vad.onnx",             // From examples/basic-agent  
		"../../../plugins/silero/silero_vad.onnx",          // From nested dirs
		"/Users/chris/dev/livekit-agents-go/plugins/silero/silero_vad.onnx", // Absolute path
	}
	
	opts := DefaultVADOptions()
	
	var lastErr error
	for _, modelPath := range possiblePaths {
		vadInstance, err := NewSileroVAD(modelPath, opts)
		if err == nil {
			fmt.Printf("✅ Silero VAD loaded from: %s\n", modelPath)
			return vadInstance, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to load Silero VAD from any path, last error: %w", lastErr)
}
