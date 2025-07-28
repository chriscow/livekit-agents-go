// Package gate provides internal implementations for voice gating.
// This package is deprecated - use voice.NewAudioGate() instead.
package gate

import "github.com/chriscow/livekit-agents-go/pkg/voice"

// NewDefaultGate creates a new AudioGate.
// Deprecated: Use voice.NewAudioGate() instead.
func NewDefaultGate() voice.AudioGate {
	return voice.NewAudioGate()
}