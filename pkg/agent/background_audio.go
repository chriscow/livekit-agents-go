package agent

import (
	"context"
	"sync"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/audio/wav"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// BackgroundAudio manages optional looping background audio tracks
// that can be mixed with TTS output.
type BackgroundAudio struct {
	mu       sync.RWMutex
	enabled  bool
	volume   float32
	reader   *wav.Reader
	frames   []rtc.AudioFrame
	position int
	playing  bool
}

// BackgroundAudioConfig holds configuration for background audio.
type BackgroundAudioConfig struct {
	// AudioFile is the path to the WAV file to loop
	AudioFile string
	// Volume is the mixing volume (0.0 to 1.0)
	Volume float32
	// Enabled determines if background audio should start immediately
	Enabled bool
}

// NewBackgroundAudio creates a new BackgroundAudio instance.
func NewBackgroundAudio(cfg BackgroundAudioConfig) (*BackgroundAudio, error) {
	ba := &BackgroundAudio{
		enabled: cfg.Enabled,
		volume:  cfg.Volume,
	}

	if cfg.AudioFile != "" {
		if err := ba.LoadAudioFile(cfg.AudioFile); err != nil {
			return nil, err
		}
	}

	return ba, nil
}

// LoadAudioFile loads a WAV file for background audio playback.
func (ba *BackgroundAudio) LoadAudioFile(filename string) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	reader, err := wav.NewReader(filename)
	if err != nil {
		return err
	}
	ba.reader = reader

	// Pre-load all frames for efficient looping
	frames, err := reader.ReadFrames()
	if err != nil {
		return err
	}

	ba.frames = frames
	ba.position = 0
	return nil
}

// SetEnabled controls whether background audio is active.
func (ba *BackgroundAudio) SetEnabled(enabled bool) {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.enabled = enabled
}

// SetVolume adjusts the background audio volume (0.0 to 1.0).
func (ba *BackgroundAudio) SetVolume(volume float32) {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	if volume < 0.0 {
		volume = 0.0
	} else if volume > 1.0 {
		volume = 1.0
	}
	ba.volume = volume
}

// IsEnabled returns whether background audio is currently enabled.
func (ba *BackgroundAudio) IsEnabled() bool {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.enabled
}

// NextFrame returns the next background audio frame, looping if necessary.
func (ba *BackgroundAudio) NextFrame() *rtc.AudioFrame {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if !ba.enabled || len(ba.frames) == 0 {
		return nil
	}

	frame := ba.frames[ba.position]
	ba.position = (ba.position + 1) % len(ba.frames)

	// Apply volume scaling
	if ba.volume != 1.0 {
		frame = ba.scaleVolume(frame, ba.volume)
	}

	return &frame
}

// Start begins background audio playback in a separate goroutine.
func (ba *BackgroundAudio) Start(ctx context.Context, output chan<- rtc.AudioFrame) {
	ba.mu.Lock()
	if ba.playing {
		ba.mu.Unlock()
		return
	}
	ba.playing = true
	ba.mu.Unlock()

	go ba.playLoop(ctx, output)
}

// Stop stops background audio playback.
func (ba *BackgroundAudio) Stop() {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.playing = false
}

// playLoop runs the background audio playback loop.
func (ba *BackgroundAudio) playLoop(ctx context.Context, output chan<- rtc.AudioFrame) {
	ticker := time.NewTicker(10 * time.Millisecond) // 10ms frame rate
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ba.mu.RLock()
			if !ba.playing {
				ba.mu.RUnlock()
				return
			}
			ba.mu.RUnlock()

			if frame := ba.NextFrame(); frame != nil {
				select {
				case output <- *frame:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// MixFrames combines a foreground frame (like TTS) with background audio.
func (ba *BackgroundAudio) MixFrames(foreground rtc.AudioFrame) rtc.AudioFrame {
	background := ba.NextFrame()
	if background == nil {
		return foreground
	}

	return mixAudioFrames(foreground, *background)
}

// scaleVolume applies volume scaling to an audio frame.
func (ba *BackgroundAudio) scaleVolume(frame rtc.AudioFrame, volume float32) rtc.AudioFrame {
	if volume == 1.0 {
		return frame
	}

	// Create a copy to avoid modifying the original
	scaled := rtc.AudioFrame{
		Data:              make([]byte, len(frame.Data)),
		SampleRate:        frame.SampleRate,
		SamplesPerChannel: frame.SamplesPerChannel,
		NumChannels:       frame.NumChannels,
		Timestamp:         frame.Timestamp,
	}

	// Scale 16-bit PCM samples with overflow protection
	for i := 0; i < len(frame.Data); i += 2 {
		// Read 16-bit sample (little-endian)
		sample := int16(frame.Data[i]) | int16(frame.Data[i+1])<<8

		// Apply volume scaling with overflow-safe arithmetic
		// Use int32 to prevent intermediate overflow, then clamp to int16 range
		scaledInt32 := int32(sample) * int32(volume*32768) / 32768
		if scaledInt32 > 32767 {
			scaledInt32 = 32767
		} else if scaledInt32 < -32768 {
			scaledInt32 = -32768
		}
		scaled_sample := int16(scaledInt32)

		// Write back (little-endian)
		scaled.Data[i] = byte(scaled_sample)
		scaled.Data[i+1] = byte(scaled_sample >> 8)
	}

	return scaled
}

// mixAudioFrames combines two audio frames by averaging their samples.
func mixAudioFrames(a, b rtc.AudioFrame) rtc.AudioFrame {
	// Use the properties of the first frame
	mixed := rtc.AudioFrame{
		Data:              make([]byte, len(a.Data)),
		SampleRate:        a.SampleRate,
		SamplesPerChannel: a.SamplesPerChannel,
		NumChannels:       a.NumChannels,
		Timestamp:         a.Timestamp,
	}

	// Ensure both frames have the same length
	minLen := len(a.Data)
	if len(b.Data) < minLen {
		minLen = len(b.Data)
	}

	// Mix 16-bit PCM samples with overflow protection
	for i := 0; i < minLen; i += 2 {
		// Read samples from both frames
		sampleA := int16(a.Data[i]) | int16(a.Data[i+1])<<8
		sampleB := int16(b.Data[i]) | int16(b.Data[i+1])<<8

		// Mix by averaging with overflow-safe arithmetic
		// Use int32 to prevent intermediate overflow, then clamp to int16 range
		mixedInt32 := (int32(sampleA) + int32(sampleB)) / 2
		if mixedInt32 > 32767 {
			mixedInt32 = 32767
		} else if mixedInt32 < -32768 {
			mixedInt32 = -32768
		}
		mixedSample := int16(mixedInt32)

		// Write mixed sample
		mixed.Data[i] = byte(mixedSample)
		mixed.Data[i+1] = byte(mixedSample >> 8)
	}

	// If frame A is longer, copy remaining samples
	if len(a.Data) > minLen {
		copy(mixed.Data[minLen:], a.Data[minLen:])
	}

	return mixed
}
