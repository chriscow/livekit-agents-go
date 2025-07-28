package voice

import "sync/atomic"

// AudioGate controls whether audio should be discarded during TTS playback.
// During TTS playback the microphone stream may need to be logically muted
// (frames discarded) when interruptions are disabled.
type AudioGate interface {
	// SetTTSPlaying sets whether TTS is currently playing.
	SetTTSPlaying(playing bool)
	
	// ShouldDiscardAudio returns true if microphone frames should be dropped.
	ShouldDiscardAudio() bool
}

// NewAudioGate creates a new AudioGate with default behavior.
// The gate starts in a state where audio is not discarded.
func NewAudioGate() AudioGate {
	return &defaultGate{}
}

// defaultGate is the default implementation of AudioGate using atomic operations.
type defaultGate struct {
	ttsPlaying int32
}

// SetTTSPlaying sets whether TTS is currently playing.
func (g *defaultGate) SetTTSPlaying(playing bool) {
	var val int32
	if playing {
		val = 1
	}
	atomic.StoreInt32(&g.ttsPlaying, val)
}

// ShouldDiscardAudio returns true if microphone frames should be dropped.
func (g *defaultGate) ShouldDiscardAudio() bool {
	return atomic.LoadInt32(&g.ttsPlaying) == 1
}