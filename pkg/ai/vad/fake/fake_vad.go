package fake

import (
	"context"
	"math/rand"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/vad"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

const (
	// DefaultSpeechProbability is the default probability of speech detection per frame
	DefaultSpeechProbability = 0.3
	// HysteresisFrames is the number of frames to wait before switching speech state
	HysteresisFrames = 5
	// MinSpeechDurationMs is the minimum duration in milliseconds for a speech segment
	MinSpeechDurationMs = 200
	// DefaultSeed is the deterministic seed for reproducible testing
	DefaultSeed = 42
)

// FakeVAD is a fake VAD implementation for testing.
type FakeVAD struct {
	speechProbability float32
	rng               *rand.Rand
}

// NewFakeVAD creates a new fake VAD provider.
// speechProbability controls how often speech is detected (0.0 to 1.0).
// Uses a deterministic seed for reproducible testing.
func NewFakeVAD(speechProbability float32) *FakeVAD {
	if speechProbability <= 0 {
		speechProbability = DefaultSpeechProbability
	}
	return &FakeVAD{
		speechProbability: speechProbability,
		rng:               rand.New(rand.NewSource(DefaultSeed)),
	}
}

// NewFakeVADWithSeed creates a new fake VAD provider with a custom seed.
// Use this for tests that need different random sequences.
func NewFakeVADWithSeed(speechProbability float32, seed int64) *FakeVAD {
	if speechProbability <= 0 {
		speechProbability = DefaultSpeechProbability
	}
	return &FakeVAD{
		speechProbability: speechProbability,
		rng:               rand.New(rand.NewSource(seed)),
	}
}

// Detect processes audio frames and generates fake VAD events.
func (f *FakeVAD) Detect(ctx context.Context, frames <-chan rtc.AudioFrame) (<-chan vad.VADEvent, error) {
	output := make(chan vad.VADEvent, 10)
	
	go func() {
		defer close(output)
		
		var isSpeaking bool
		var speechStartTime time.Time
		frameCount := 0
		
		for {
			select {
			case _, ok := <-frames:
				if !ok {
					// Send speech end if we were speaking
					if isSpeaking {
						select {
						case output <- vad.VADEvent{
							Type:      vad.VADEventSpeechEnd,
							Timestamp: time.Now(),
						}:
						case <-ctx.Done():
							return
						}
					}
					return
				}
				
				frameCount++
				
				// Simple fake logic: randomly determine speech/silence using seeded RNG
				hasActivity := f.rng.Float32() < f.speechProbability
				
				// Add some hysteresis to avoid rapid switching
				if !isSpeaking && hasActivity && frameCount%HysteresisFrames == 0 {
					// Start speaking
					isSpeaking = true
					speechStartTime = time.Now()
					select {
					case output <- vad.VADEvent{
						Type:      vad.VADEventSpeechStart,
						Timestamp: speechStartTime,
					}:
					case <-ctx.Done():
						return
					}
				} else if isSpeaking && !hasActivity && time.Since(speechStartTime) > MinSpeechDurationMs*time.Millisecond {
					// Stop speaking (with minimum duration)
					isSpeaking = false
					select {
					case output <- vad.VADEvent{
						Type:      vad.VADEventSpeechEnd,
						Timestamp: time.Now(),
					}:
					case <-ctx.Done():
						return
					}
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return output, nil
}

// Capabilities returns the fake VAD capabilities.
func (f *FakeVAD) Capabilities() vad.VADCapabilities {
	return vad.VADCapabilities{
		SampleRates:        []int{16000, 48000},
		MinSpeechDuration:  100 * time.Millisecond,
		MinSilenceDuration: 200 * time.Millisecond,
		Sensitivity:        f.speechProbability,
	}
}