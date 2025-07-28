package fake

import (
	"context"
	"math"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// FakeTTS is a fake TTS implementation for testing.
type FakeTTS struct{}

// NewFakeTTS creates a new fake TTS provider.
func NewFakeTTS() *FakeTTS {
	return &FakeTTS{}
}

// Synthesize generates fake audio frames (sine wave) for the given text.
func (f *FakeTTS) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (<-chan rtc.AudioFrame, error) {
	output := make(chan rtc.AudioFrame, 10)
	
	go func() {
		defer close(output)
		
		// Generate roughly 1 second of audio (100 frames of 10ms each)
		duration := time.Duration(len(req.Text)) * 100 * time.Millisecond
		frameCount := int(duration / (10 * time.Millisecond))
		
		sampleRate := 48000
		samplesPerChannel := sampleRate / 100 // 10ms worth
		frequency := 440.0 // A4 note
		
		for i := 0; i < frameCount; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			// Generate sine wave audio data
			data := make([]byte, samplesPerChannel*2) // 16-bit mono
			for j := 0; j < samplesPerChannel; j++ {
				sampleIndex := i*samplesPerChannel + j
				sample := math.Sin(2 * math.Pi * frequency * float64(sampleIndex) / float64(sampleRate))
				sample *= 0.3 // reduce volume
				
				// Convert to 16-bit signed integer
				intSample := int16(sample * 32767)
				
				// Little-endian encoding
				data[j*2] = byte(intSample & 0xFF)
				data[j*2+1] = byte((intSample >> 8) & 0xFF)
			}
			
			frame := rtc.AudioFrame{
				Data:              data,
				SampleRate:        sampleRate,
				SamplesPerChannel: samplesPerChannel,
				NumChannels:       1,
				Timestamp:         time.Duration(i) * 10 * time.Millisecond,
			}
			
			select {
			case output <- frame:
			case <-ctx.Done():
				return
			}
			
			// Simulate real-time playback
			time.Sleep(10 * time.Millisecond)
		}
	}()
	
	return output, nil
}

// Capabilities returns the fake TTS capabilities.
func (f *FakeTTS) Capabilities() tts.TTSCapabilities {
	return tts.TTSCapabilities{
		Streaming:            true,
		SupportedLanguages:   []string{"en-US", "en-GB", "es-ES"},
		SupportedVoices:      []string{"fake-voice-1", "fake-voice-2"},
		SampleRates:          []int{16000, 48000},
		SupportsSSML:         false,
		SupportsSpeedControl: true,
		SupportsPitchControl: true,
	}
}