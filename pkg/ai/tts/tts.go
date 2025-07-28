package tts

import (
	"context"

	"github.com/chriscow/livekit-agents-go/pkg/ai"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// TTS-specific error variables for backward compatibility  
var (
	// ErrRecoverable indicates a temporary TTS failure that may succeed if retried.
	// Examples: service overload, temporary quota exceeded, network issues.
	ErrRecoverable = ai.ErrRecoverable
	
	// ErrFatal indicates a permanent TTS failure that will not succeed if retried.
	// Examples: invalid voice ID, unsupported text format, permanent quota exceeded.
	ErrFatal = ai.ErrFatal
)

// SynthesizeRequest contains parameters for text-to-speech synthesis.
type SynthesizeRequest struct {
	Text     string
	Voice    string
	Language string
	Speed    float32
	Pitch    float32
}

// TTSCapabilities describes the capabilities of a TTS provider.
type TTSCapabilities struct {
	Streaming           bool
	SupportedLanguages  []string
	SupportedVoices     []string
	SampleRates         []int
	SupportsSSML        bool
	SupportsSpeedControl bool
	SupportsPitchControl bool
}

// TTS is the main interface for text-to-speech providers.
type TTS interface {
	// Synthesize converts text to audio frames.
	// Returns a channel that will receive audio frames and close when synthesis is complete.
	Synthesize(ctx context.Context, req SynthesizeRequest) (<-chan rtc.AudioFrame, error)
	
	// Capabilities returns the provider's capabilities.
	Capabilities() TTSCapabilities
}