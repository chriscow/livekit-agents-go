package audio

import (
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// Audio processor-specific error variables for backward compatibility
var (
	// ErrRecoverable indicates a temporary audio processor failure that may succeed if retried.
	// Examples: resource shortage, temporary processing overload.
	ErrRecoverable = ai.ErrRecoverable
	
	// ErrFatal indicates a permanent audio processor failure that will not succeed if retried.
	// Examples: unsupported audio format, invalid configuration, hardware failure.
	ErrFatal = ai.ErrFatal
)

// ProcessorConfig toggles individual WebRTC sub-modules.
type ProcessorConfig struct {
	EchoCancellation bool
	NoiseSuppression bool
	HighPassFilter   bool
	AutoGainControl  bool
}

// NewProcessorConfig creates a new ProcessorConfig with recommended defaults.
// All processing features are enabled by default for optimal audio quality.
func NewProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		EchoCancellation: true,
		NoiseSuppression: true,
		HighPassFilter:   true,
		AutoGainControl:  true,
	}
}

// NewProcessorConfigDisabled creates a new ProcessorConfig with all features disabled.
// Use this for testing or when you want to apply custom processing.
func NewProcessorConfigDisabled() ProcessorConfig {
	return ProcessorConfig{
		EchoCancellation: false,
		NoiseSuppression: false,
		HighPassFilter:   false,
		AutoGainControl:  false,
	}
}

// WithEchoCancellation returns a copy of the config with echo cancellation toggled.
func (c ProcessorConfig) WithEchoCancellation(enabled bool) ProcessorConfig {
	c.EchoCancellation = enabled
	return c
}

// WithNoiseSuppression returns a copy of the config with noise suppression toggled.
func (c ProcessorConfig) WithNoiseSuppression(enabled bool) ProcessorConfig {
	c.NoiseSuppression = enabled
	return c
}

// WithHighPassFilter returns a copy of the config with high pass filter toggled.
func (c ProcessorConfig) WithHighPassFilter(enabled bool) ProcessorConfig {
	c.HighPassFilter = enabled
	return c
}

// WithAutoGainControl returns a copy of the config with auto gain control toggled.
func (c ProcessorConfig) WithAutoGainControl(enabled bool) ProcessorConfig {
	c.AutoGainControl = enabled
	return c
}

// Processor abstracts WebRTC's AudioProcessingModule (AEC3).
type Processor interface {
	// ProcessReverse handles far-end (speaker output) reference – MUST be 10 ms frames.
	ProcessReverse(frame rtc.AudioFrame) error
	
	// ProcessCapture handles near-end (microphone) capture – processed in-place.
	ProcessCapture(frame *rtc.AudioFrame) error

	// SetStreamDelay provides measured delay between reverse/capture paths when EC is on.
	SetStreamDelay(d time.Duration) error
	
	// Close releases resources.
	Close() error
}