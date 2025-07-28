package fake

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

func TestFakeSTTCapabilities(t *testing.T) {
	provider := NewFakeSTT("test")
	caps := provider.Capabilities()

	if !caps.Streaming {
		t.Error("Expected Streaming to be true")
	}
	if !caps.InterimResults {
		t.Error("Expected InterimResults to be true")
	}
	if len(caps.SupportedLanguages) == 0 {
		t.Error("Expected SupportedLanguages to be non-empty")
	}
	if len(caps.SampleRates) == 0 {
		t.Error("Expected SampleRates to be non-empty")
	}
}

func TestFakeSTTStream(t *testing.T) {
	transcript := "Hello world"
	provider := NewFakeSTT(transcript)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := provider.NewStream(ctx, stt.StreamConfig{
		SampleRate:  16000,
		NumChannels: 1,
		Lang:        "en-US",
		MaxRetry:    3,
	})
	if err != nil {
		t.Fatalf("NewStream() error = %v", err)
	}

	// Test pushing frames
	frame := rtc.AudioFrame{
		Data:              make([]byte, 320),
		SampleRate:        16000,
		SamplesPerChannel: 160,
		NumChannels:       1,
		Timestamp:         0,
	}

	// Push several frames to trigger interim results
	for i := 0; i < 15; i++ {
		if err := stream.Push(frame); err != nil {
			t.Fatalf("Push() error = %v", err)
		}
	}

	// Close the stream
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend() error = %v", err)
	}

	// Collect events
	var events []stt.SpeechEvent
	eventTimeout := time.After(1 * time.Second)

	for {
		select {
		case event, ok := <-stream.Events():
			if !ok {
				// Channel closed, we're done
				goto checkEvents
			}
			events = append(events, event)
		case <-eventTimeout:
			t.Fatal("Timeout waiting for events")
		}
	}

checkEvents:
	if len(events) == 0 {
		t.Fatal("Expected at least one event")
	}

	// Check that we got a final event with the expected transcript
	foundFinal := false
	for _, event := range events {
		if event.Type == stt.SpeechEventFinal {
			foundFinal = true
			if event.Text != transcript {
				t.Errorf("Final event text = %q, want %q", event.Text, transcript)
			}
			if !event.IsFinal {
				t.Error("Final event IsFinal should be true")
			}
		}
	}

	if !foundFinal {
		t.Error("Expected at least one final event")
	}
}

func TestFakeSTTStreamClosedPush(t *testing.T) {
	provider := NewFakeSTT("test")
	ctx := context.Background()

	stream, err := provider.NewStream(ctx, stt.StreamConfig{
		SampleRate:  16000,
		NumChannels: 1,
	})
	if err != nil {
		t.Fatalf("NewStream() error = %v", err)
	}

	// Close the stream first
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend() error = %v", err)
	}

	// Now try to push a frame - should return error
	frame := rtc.AudioFrame{
		Data:              make([]byte, 320),
		SampleRate:        16000,
		SamplesPerChannel: 160,
		NumChannels:       1,
	}

	err = stream.Push(frame)
	if err == nil {
		t.Error("Expected error when pushing to closed stream")
	}
}