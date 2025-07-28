package fake

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
)

func TestFakeTTSCapabilities(t *testing.T) {
	provider := NewFakeTTS()
	caps := provider.Capabilities()

	if !caps.Streaming {
		t.Error("Expected Streaming to be true")
	}
	
	if len(caps.SupportedLanguages) == 0 {
		t.Error("Expected SupportedLanguages to be non-empty")
	}
	
	if len(caps.SupportedVoices) == 0 {
		t.Error("Expected SupportedVoices to be non-empty")
	}
	
	if len(caps.SampleRates) == 0 {
		t.Error("Expected SampleRates to be non-empty")
	}
	
	if !caps.SupportsSpeedControl {
		t.Error("Expected SupportsSpeedControl to be true")
	}
	
	if !caps.SupportsPitchControl {
		t.Error("Expected SupportsPitchControl to be true")
	}
}

func TestFakeTTSSynthesize(t *testing.T) {
	provider := NewFakeTTS()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := tts.SynthesizeRequest{
		Text:     "Hello world",
		Voice:    "fake-voice-1",
		Language: "en-US",
		Speed:    1.0,
		Pitch:    1.0,
	}

	frames, err := provider.Synthesize(ctx, req)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	// Count frames and verify they have proper structure
	frameCount := 0
	for frame := range frames {
		frameCount++
		
		// Verify frame structure
		if frame.SampleRate != 48000 {
			t.Errorf("Expected sample rate 48000, got %d", frame.SampleRate)
		}
		
		if frame.NumChannels != 1 {
			t.Errorf("Expected 1 channel, got %d", frame.NumChannels)
		}
		
		if frame.SamplesPerChannel != 480 {
			t.Errorf("Expected 480 samples per channel, got %d", frame.SamplesPerChannel)
		}
		
		if len(frame.Data) != 960 { // 480 samples * 1 channel * 2 bytes
			t.Errorf("Expected 960 bytes of data, got %d", len(frame.Data))
		}
		
		// Verify timestamp progression
		expectedTimestamp := time.Duration(frameCount-1) * 10 * time.Millisecond
		if frame.Timestamp != expectedTimestamp {
			t.Errorf("Expected timestamp %v, got %v", expectedTimestamp, frame.Timestamp)
		}
	}

	// Should generate roughly the right number of frames for text length
	expectedFrames := len(req.Text) * 10 // roughly 100ms per character
	if frameCount < expectedFrames/2 || frameCount > expectedFrames*2 {
		t.Errorf("Unexpected frame count %d for text length %d", frameCount, len(req.Text))
	}
}

func TestFakeTTSContextCancellation(t *testing.T) {
	provider := NewFakeTTS()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is always called

	req := tts.SynthesizeRequest{
		Text:     "This is a longer text that should generate many frames",
		Voice:    "fake-voice-1",
		Language: "en-US",
	}

	frames, err := provider.Synthesize(ctx, req)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	// Read a few frames then cancel
	frameCount := 0
	for range frames {
		frameCount++
		if frameCount == 3 {
			cancel() // Cancel context after 3 frames
		}
	}

	// Should have stopped early due to cancellation
	if frameCount > 10 {
		t.Errorf("Expected early termination due to context cancellation, got %d frames", frameCount)
	}
}

func TestFakeTTSEmptyText(t *testing.T) {
	provider := NewFakeTTS()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := tts.SynthesizeRequest{
		Text:     "",
		Voice:    "fake-voice-1",
		Language: "en-US",
	}

	frames, err := provider.Synthesize(ctx, req)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	// Should handle empty text gracefully (minimal or no frames)
	frameCount := 0
	for range frames {
		frameCount++
	}

	// Empty text should generate very few frames
	if frameCount > 5 {
		t.Errorf("Expected few frames for empty text, got %d", frameCount)
	}
}