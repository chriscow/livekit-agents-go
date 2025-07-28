package fake

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// FuzzSTTStream tests STT stream with random audio frame sequences
func FuzzSTTStream(f *testing.F) {
	// Add seed corpus
	f.Add([]byte{0x00, 0x01, 0x02, 0x03}, uint16(1), uint32(16000))
	f.Add(make([]byte, 320), uint16(1), uint32(16000))  // Standard frame
	f.Add(make([]byte, 960), uint16(2), uint32(48000)) // Stereo 48kHz
	f.Add([]byte{}, uint16(1), uint32(16000))          // Empty frame

	f.Fuzz(func(t *testing.T, data []byte, channels uint16, sampleRate uint32) {
		// Constraint inputs to valid ranges
		if channels < 1 || channels > 2 {
			return
		}
		if sampleRate != 16000 && sampleRate != 48000 {
			return
		}

		provider := NewFakeSTT("Fuzz test transcript")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		stream, err := provider.NewStream(ctx, stt.StreamConfig{
			SampleRate:  int(sampleRate),
			NumChannels: int(channels),
			Lang:        "en-US",
			MaxRetry:    3,
		})
		if err != nil {
			t.Fatalf("NewStream() error = %v", err)
		}

		// Create frame with fuzzed data
		expectedLen := int(sampleRate)/100 * int(channels) * 2 // 10ms worth
		frameData := make([]byte, expectedLen)
		if len(data) > 0 {
			// Copy fuzzed data, repeating if necessary
			for i := 0; i < expectedLen; i++ {
				frameData[i] = data[i%len(data)]
			}
		}

		frame := rtc.AudioFrame{
			Data:              frameData,
			SampleRate:        int(sampleRate),
			SamplesPerChannel: int(sampleRate) / 100,
			NumChannels:       int(channels),
			Timestamp:         0,
		}

		// Test sequence: Push -> Events -> CloseSend
		
		// 1. Push frame (should not panic or deadlock)
		pushErr := stream.Push(frame)
		
		// 2. Try to read events (should not block indefinitely)
		eventChan := stream.Events()
		
		// 3. Close stream
		closeErr := stream.CloseSend()
		
		// 4. Drain events (should complete without hanging)
		eventCount := 0
		timeout := time.After(500 * time.Millisecond)
		
	drainLoop:
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					break drainLoop
				}
				eventCount++
				
				// Validate event structure
				if event.Type != stt.SpeechEventInterim && 
				   event.Type != stt.SpeechEventFinal && 
				   event.Type != stt.SpeechEventError {
					t.Errorf("Invalid event type: %v", event.Type)
				}
				
				// Events should have reasonable content
				if event.Type != stt.SpeechEventError && len(event.Text) > 1000 {
					t.Errorf("Event text too long: %d chars", len(event.Text))
				}
				
			case <-timeout:
				t.Errorf("Timeout draining events after %d events", eventCount)
				break drainLoop
			}
		}

		// Validate behavior
		if pushErr != nil && closeErr == nil {
			// If push failed, close should still work
		}
		
		if pushErr == nil && closeErr != nil {
			t.Errorf("Push succeeded but close failed: %v", closeErr)
		}
		
		// Should get at least one event if push succeeded
		if pushErr == nil && eventCount == 0 {
			t.Error("Expected at least one event when push succeeded")
		}
	})
}

// FuzzSTTStreamOrdering tests various push/close orderings
func FuzzSTTStreamOrdering(f *testing.F) {
	// Seed with different operation sequences
	f.Add([]byte{1, 0})        // push, close
	f.Add([]byte{0})           // close only  
	f.Add([]byte{1, 1, 0})     // push, push, close
	f.Add([]byte{0, 1})        // close, push (invalid)
	f.Add([]byte{1, 0, 1})     // push, close, push (invalid)

	f.Fuzz(func(t *testing.T, operations []byte) {
		if len(operations) == 0 || len(operations) > 20 {
			return // Skip empty or too long sequences
		}

		provider := NewFakeSTT("Fuzz ordering test")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		stream, err := provider.NewStream(ctx, stt.StreamConfig{
			SampleRate:  16000,
			NumChannels: 1,
			Lang:        "en-US",
		})
		if err != nil {
			t.Fatalf("NewStream() error = %v", err)
		}

		frame := rtc.AudioFrame{
			Data:              make([]byte, 320),
			SampleRate:        16000,
			SamplesPerChannel: 160,
			NumChannels:       1,
		}

		closed := false
		
		// Execute operations sequence
		for i, op := range operations {
			switch op % 2 {
			case 0: // CloseSend
				err := stream.CloseSend()
				if closed {
					// Multiple closes should be idempotent
				} else {
					// First close should succeed
					if err != nil {
						t.Errorf("Operation %d: First CloseSend() failed: %v", i, err)
					}
					closed = true
				}
				
			case 1: // Push
				err := stream.Push(frame)
				if closed {
					// Push after close should fail
					if err == nil {
						t.Errorf("Operation %d: Push() after close should fail", i)
					}
				}
				// Push before close may succeed or fail, both are acceptable
			}
		}

		// Drain events to prevent goroutine leaks
		timeout := time.After(100 * time.Millisecond)
	drainLoop:
		for {
			select {
			case _, ok := <-stream.Events():
				if !ok {
					break drainLoop
				}
			case <-timeout:
				break drainLoop
			}
		}
		
		// Ensure stream is closed
		if !closed {
			stream.CloseSend()
		}
	})
}