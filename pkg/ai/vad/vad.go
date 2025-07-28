package vad

import (
	"context"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// VAD-specific error variables for backward compatibility
var (
	// ErrRecoverable indicates a temporary VAD failure that may succeed if retried.
	// Examples: processing overload, temporary resource shortage.
	ErrRecoverable = ai.ErrRecoverable
	
	// ErrFatal indicates a permanent VAD failure that will not succeed if retried.
	// Examples: unsupported audio format, invalid configuration.
	ErrFatal = ai.ErrFatal
)

// VADEventType represents the type of VAD event.
type VADEventType int

const (
	VADEventSpeechStart VADEventType = iota
	VADEventSpeechEnd
	VADEventError
)

// VADEvent represents a voice activity detection event.
type VADEvent struct {
	Type      VADEventType
	Timestamp time.Time
	Error     error
}

// VADCapabilities describes the capabilities of a VAD provider.
type VADCapabilities struct {
	SampleRates       []int
	MinSpeechDuration time.Duration
	MinSilenceDuration time.Duration
	Sensitivity       float32 // 0.0 to 1.0
}

// VAD is the main interface for voice activity detection providers.
type VAD interface {
	// Detect processes audio frames and returns VAD events.
	// The returned channel will be closed when the input channel is closed or context is cancelled.
	Detect(ctx context.Context, frames <-chan rtc.AudioFrame) (<-chan VADEvent, error)
	
	// Capabilities returns the provider's capabilities.
	Capabilities() VADCapabilities
}