// Package stt provides interfaces and types for speech-to-text providers.
// It defines streaming STT interfaces that convert audio frames to text transcripts
// with support for interim results, multiple languages, and error handling.
package stt

import (
	"context"

	"github.com/chriscow/livekit-agents-go/pkg/ai"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// STT-specific error variables for backward compatibility
var (
	// ErrRecoverable indicates a temporary STT failure that may succeed if retried.
	// Examples: network timeout, service unavailable, rate limiting.
	ErrRecoverable = ai.ErrRecoverable
	
	// ErrFatal indicates a permanent STT failure that will not succeed if retried.
	// Examples: invalid audio format, unsupported language, authentication failure.
	ErrFatal = ai.ErrFatal
)

// StreamConfig contains configuration for STT streams.
type StreamConfig struct {
	SampleRate  int
	NumChannels int
	Lang        string
	MaxRetry    int
}

// SpeechEvent represents a speech recognition event containing transcription results or errors.
type SpeechEvent struct {
	Type      SpeechEventType // Type of event (interim, final, or error)
	Text      string          // Transcribed text (empty for error events)
	IsFinal   bool            // True if this is a final result that won't change
	Language  string          // Detected or configured language code
	Timestamp int64           // Event timestamp in milliseconds since epoch
	Error     error           // Error details (only set for error events)
}

// SpeechEventType represents the type of speech recognition event.
type SpeechEventType int

const (
	// SpeechEventInterim represents partial transcription results that may change
	SpeechEventInterim SpeechEventType = iota
	// SpeechEventFinal represents final transcription results that won't change  
	SpeechEventFinal
	// SpeechEventError represents transcription errors
	SpeechEventError
)

// STTCapabilities describes the capabilities of an STT provider.
type STTCapabilities struct {
	Streaming          bool
	InterimResults     bool
	SupportedLanguages []string
	SampleRates        []int
}

// STT is the main interface for speech-to-text providers.
type STT interface {
	// NewStream creates a new streaming STT session.
	NewStream(ctx context.Context, cfg StreamConfig) (STTStream, error)
	
	// Capabilities returns the provider's capabilities.
	Capabilities() STTCapabilities
}

// STTStream represents an active STT streaming session.
type STTStream interface {
	// Push sends an audio frame for processing.
	Push(frame rtc.AudioFrame) error
	
	// Events returns a channel of speech recognition events.
	Events() <-chan SpeechEvent
	
	// CloseSend signals that no more audio will be sent and flushes any pending data.
	CloseSend() error
}