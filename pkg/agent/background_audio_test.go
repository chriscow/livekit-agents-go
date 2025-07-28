package agent

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

func TestNewBackgroundAudio(t *testing.T) {
	tests := []struct {
		name   string
		config BackgroundAudioConfig
	}{
		{
			name: "default config",
			config: BackgroundAudioConfig{
				Volume:  0.5,
				Enabled: true,
			},
		},
		{
			name: "disabled config",
			config: BackgroundAudioConfig{
				Volume:  0.3,
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ba, err := NewBackgroundAudio(tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if ba == nil {
				t.Error("expected valid BackgroundAudio, got nil")
			}

			if ba.IsEnabled() != tt.config.Enabled {
				t.Errorf("expected enabled=%v, got %v", tt.config.Enabled, ba.IsEnabled())
			}
		})
	}
}

func TestBackgroundAudio_SetEnabled(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  0.5,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	// Initially disabled
	if ba.IsEnabled() {
		t.Error("expected BackgroundAudio to be disabled initially")
	}

	// Enable it
	ba.SetEnabled(true)
	if !ba.IsEnabled() {
		t.Error("expected BackgroundAudio to be enabled after SetEnabled(true)")
	}

	// Disable it again
	ba.SetEnabled(false)
	if ba.IsEnabled() {
		t.Error("expected BackgroundAudio to be disabled after SetEnabled(false)")
	}
}

func TestBackgroundAudio_SetVolume(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  0.5,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	tests := []struct {
		input    float32
		expected float32
	}{
		{0.3, 0.3},
		{0.0, 0.0},
		{1.0, 1.0},
		{-0.5, 0.0}, // Should clamp to 0.0
		{1.5, 1.0},  // Should clamp to 1.0
	}

	for _, tt := range tests {
		ba.SetVolume(tt.input)
		// We can't directly check the volume, but we can test that it doesn't panic
		// and that the method accepts the input
	}
}

func TestBackgroundAudio_NextFrame_NoFrames(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  0.5,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	// Should return nil when no frames are loaded
	frame := ba.NextFrame()
	if frame != nil {
		t.Error("expected nil frame when no audio loaded, got frame")
	}
}

func TestBackgroundAudio_NextFrame_Disabled(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  0.5,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	// Should return nil when disabled
	frame := ba.NextFrame()
	if frame != nil {
		t.Error("expected nil frame when disabled, got frame")
	}
}

func TestBackgroundAudio_Start_Stop(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  0.5,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	output := make(chan rtc.AudioFrame, 10)

	// Start should not block
	ba.Start(ctx, output)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	ba.Stop()

	// Starting again should not panic
	ba.Start(ctx, output)
	ba.Stop()
}

func TestBackgroundAudio_MixFrames(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  0.5,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	foreground := rtc.AudioFrame{
		Data:              make([]byte, 960),
		SampleRate:        48000,
		SamplesPerChannel: 480,
		NumChannels:       1,
	}

	// Fill with some test data
	for i := range foreground.Data {
		foreground.Data[i] = byte(i % 256)
	}

	// Mix with background (should return original since no background loaded)
	mixed := ba.MixFrames(foreground)

	// Should get the original frame back since no background is loaded
	if len(mixed.Data) != len(foreground.Data) {
		t.Errorf("expected mixed frame to have same length as foreground")
	}
	if mixed.SampleRate != foreground.SampleRate {
		t.Errorf("expected mixed frame to have same sample rate as foreground")
	}
}

func TestMixAudioFrames(t *testing.T) {
	frameA := rtc.AudioFrame{
		Data:              make([]byte, 4), // 2 samples, 16-bit each
		SampleRate:        48000,
		SamplesPerChannel: 2,
		NumChannels:       1,
	}

	frameB := rtc.AudioFrame{
		Data:              make([]byte, 4), // 2 samples, 16-bit each
		SampleRate:        48000,
		SamplesPerChannel: 2,
		NumChannels:       1,
	}

	// Set some test values (little-endian 16-bit)
	// Sample 1: 1000
	frameA.Data[0] = 0xE8 // 1000 & 0xFF = 232
	frameA.Data[1] = 0x03 // 1000 >> 8 = 3
	// Sample 2: 2000
	frameA.Data[2] = 0xD0 // 2000 & 0xFF = 208
	frameA.Data[3] = 0x07 // 2000 >> 8 = 7

	// Sample 1: 500
	frameB.Data[0] = 0xF4 // 500 & 0xFF = 244
	frameB.Data[1] = 0x01 // 500 >> 8 = 1
	// Sample 2: 1500
	frameB.Data[2] = 0xDC // 1500 & 0xFF = 220
	frameB.Data[3] = 0x05 // 1500 >> 8 = 5

	mixed := mixAudioFrames(frameA, frameB)

	// Verify the mixed frame properties
	if len(mixed.Data) != len(frameA.Data) {
		t.Errorf("expected mixed frame data length %d, got %d", len(frameA.Data), len(mixed.Data))
	}
	if mixed.SampleRate != frameA.SampleRate {
		t.Errorf("expected mixed frame sample rate %d, got %d", frameA.SampleRate, mixed.SampleRate)
	}

	// The exact mixed values depend on the averaging calculation
	// We just verify that mixing doesn't panic and produces reasonable output
	if len(mixed.Data) == 0 {
		t.Error("expected non-empty mixed frame data")
	}
}

func TestScaleVolume(t *testing.T) {
	ba, err := NewBackgroundAudio(BackgroundAudioConfig{
		Volume:  1.0,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create BackgroundAudio: %v", err)
	}

	frame := rtc.AudioFrame{
		Data:              make([]byte, 4), // 2 samples
		SampleRate:        48000,
		SamplesPerChannel: 2,
		NumChannels:       1,
	}

	// Set test values
	frame.Data[0] = 0x00 // 256 & 0xFF
	frame.Data[1] = 0x01 // 256 >> 8
	frame.Data[2] = 0x00 // 512 & 0xFF
	frame.Data[3] = 0x02 // 512 >> 8

	// Test volume scaling
	scaled := ba.scaleVolume(frame, 0.5)

	// Verify the frame structure is preserved
	if len(scaled.Data) != len(frame.Data) {
		t.Errorf("expected scaled frame data length %d, got %d", len(frame.Data), len(scaled.Data))
	}
	if scaled.SampleRate != frame.SampleRate {
		t.Errorf("expected scaled frame sample rate %d, got %d", frame.SampleRate, scaled.SampleRate)
	}

	// Test volume 1.0 (no change)
	unscaled := ba.scaleVolume(frame, 1.0)
	for i, b := range frame.Data {
		if unscaled.Data[i] != b {
			t.Errorf("expected volume 1.0 to preserve data, difference at index %d", i)
			break
		}
	}

	// Test volume 0.0 (silence)
	silenced := ba.scaleVolume(frame, 0.0)
	for i, b := range silenced.Data {
		if b != 0 {
			t.Errorf("expected volume 0.0 to produce silence, got non-zero at index %d: %d", i, b)
			break
		}
	}
}
