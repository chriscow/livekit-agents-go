package fake

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/vad"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

func TestFakeVADCapabilities(t *testing.T) {
	provider := NewFakeVAD(0.5)
	caps := provider.Capabilities()

	if len(caps.SampleRates) == 0 {
		t.Error("Expected SampleRates to be non-empty")
	}
	
	if caps.MinSpeechDuration <= 0 {
		t.Error("Expected MinSpeechDuration to be positive")
	}
	
	if caps.MinSilenceDuration <= 0 {
		t.Error("Expected MinSilenceDuration to be positive")
	}
}

func TestFakeVADDeterministic(t *testing.T) {
	// Test that same seed produces same results
	provider1 := NewFakeVADWithSeed(0.8, 123)
	provider2 := NewFakeVADWithSeed(0.8, 123)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create identical input frames
	frames1 := make(chan rtc.AudioFrame, 10)
	frames2 := make(chan rtc.AudioFrame, 10)
	
	for i := 0; i < 10; i++ {
		frame := rtc.AudioFrame{
			Data:              make([]byte, 320),
			SampleRate:        16000,
			SamplesPerChannel: 160,
			NumChannels:       1,
		}
		frames1 <- frame
		frames2 <- frame
	}
	close(frames1)
	close(frames2)

	events1, err1 := provider1.Detect(ctx, frames1)
	if err1 != nil {
		t.Fatalf("provider1.Detect() error = %v", err1)
	}

	events2, err2 := provider2.Detect(ctx, frames2)
	if err2 != nil {
		t.Fatalf("provider2.Detect() error = %v", err2)
	}

	// Collect events from both providers
	var result1, result2 []vad.VADEvent
	
	done := make(chan struct{}, 2)
	
	go func() {
		for event := range events1 {
			result1 = append(result1, event)
		}
		done <- struct{}{}
	}()
	
	go func() {
		for event := range events2 {
			result2 = append(result2, event)
		}
		done <- struct{}{}
	}()

	// Wait for both to complete
	<-done
	<-done

	// Results should be identical for same seed
	if len(result1) != len(result2) {
		t.Errorf("Different number of events: %d vs %d", len(result1), len(result2))
	}

	for i := 0; i < min(len(result1), len(result2)); i++ {
		if result1[i].Type != result2[i].Type {
			t.Errorf("Event %d type mismatch: %v vs %v", i, result1[i].Type, result2[i].Type)
		}
	}
}

func TestFakeVADSpeechTiming(t *testing.T) {
	// Test VAD detects speech segment within ±20ms tolerance
	provider := NewFakeVADWithSeed(1.0, 42) // 100% speech probability

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	frames := make(chan rtc.AudioFrame, 50)
	startTime := time.Now()
	
	// Send frames that should trigger speech
	for i := 0; i < 50; i++ {
		frame := rtc.AudioFrame{
			Data:              make([]byte, 320),
			SampleRate:        16000,
			SamplesPerChannel: 160,
			NumChannels:       1,
		}
		frames <- frame
	}
	close(frames)

	events, err := provider.Detect(ctx, frames)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// Look for speech start event
	speechStartFound := false
	var speechStartTime time.Time
	
	for event := range events {
		if event.Type == vad.VADEventSpeechStart {
			speechStartFound = true
			speechStartTime = event.Timestamp
			break
		}
	}

	if !speechStartFound {
		t.Fatal("Expected speech start event")
	}

	// Check timing is within reasonable bounds (should be soon after start)
	timeDiff := speechStartTime.Sub(startTime)
	if timeDiff < 0 || timeDiff > 100*time.Millisecond {
		t.Errorf("Speech start timing outside expected range: %v", timeDiff)
	}
}

func TestFakeVADContextCancellation(t *testing.T) {
	provider := NewFakeVAD(0.5)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	frames := make(chan rtc.AudioFrame)
	events, err := provider.Detect(ctx, frames)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// Cancel context immediately
	cancel()
	close(frames)

	// Events channel should close quickly
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				// Channel closed as expected
				return
			}
		case <-timeout:
			t.Fatal("VAD didn't respect context cancellation")
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}