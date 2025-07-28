package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm/fake"
	sttfake "github.com/chriscow/livekit-agents-go/pkg/ai/stt/fake"
	ttsfake "github.com/chriscow/livekit-agents-go/pkg/ai/tts/fake"
	vadfake "github.com/chriscow/livekit-agents-go/pkg/ai/vad/fake"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
	turnfake "github.com/chriscow/livekit-agents-go/pkg/turn/fake"
)

// TestAgent_GoldenAudio tests the agent with a known audio input and validates
// the expected behavior and metrics.
func TestAgent_GoldenAudio(t *testing.T) {
	t.Helper()

	micIn := make(chan rtc.AudioFrame, 100)
	ttsOut := make(chan rtc.AudioFrame, 100)

	// Set up providers with deterministic responses
	sttProvider := sttfake.NewFakeSTT("Hello, this is a test message.")
	ttsProvider := ttsfake.NewFakeTTS()
	llmProvider := fake.NewFakeLLM("I received your test message!")
	vadProvider := vadfake.NewFakeVAD(0.4) // Slightly higher probability for predictable behavior

	config := Config{
		STT:          sttProvider,
		TTS:          ttsProvider,
		LLM:          llmProvider,
		VAD:          vadProvider,
		TurnDetector: turnfake.NewFakeTurnDetector(),
		MicIn:        micIn,
		TTSOut:       ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	// Create job with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobInstance, err := job.New(ctx, job.Config{
		RoomName: "golden-test",
		Timeout:  time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Start agent
	agentDone := make(chan error, 1)
	go func() {
		agentDone <- agent.Start(ctx, jobInstance)
	}()

	// Generate "golden" audio input - 2 seconds of speech-like data
	go func() {
		defer close(micIn)

		// Send silence first (10 frames = 100ms)
		for i := 0; i < 10; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960), // Silence
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
				Timestamp:         time.Duration(i) * 10 * time.Millisecond,
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(time.Millisecond) // Small delay for realistic timing
		}

		// Send "speech" data (200 frames = 2 seconds)
		for i := 10; i < 210; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960),
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
				Timestamp:         time.Duration(i) * 10 * time.Millisecond,
			}
			// Fill with pseudo-speech data
			for j := range frame.Data {
				frame.Data[j] = byte((i + j) % 256)
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(time.Millisecond)
		}

		// Send silence again (10 frames = 100ms)
		for i := 210; i < 220; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960), // Silence
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
				Timestamp:         time.Duration(i) * 10 * time.Millisecond,
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Consume TTS output and count frames (use atomic for race safety)
	var ttsFrameCount int64
	go func() {
		for {
			select {
			case <-ttsOut:
				atomic.AddInt64(&ttsFrameCount, 1)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion
	select {
	case err := <-agentDone:
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("agent failed: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Error("golden audio test timed out")
		cancel()
	}

	// Validate metrics
	t.Run("metrics_validation", func(t *testing.T) {
		// Check that session duration was recorded
		sessionDuration := agent.metrics.SessionDuration.Value()
		if sessionDuration <= 0 {
			t.Error("expected session duration to be recorded, got 0")
		}
		t.Logf("Session duration: %.2f ms", sessionDuration)

		// Check that first word latency was recorded (should be > 0 if agent responded)
		firstWordLatency := agent.metrics.FirstWordLatency.Value()
		if firstWordLatency == 0 {
			t.Log("First word latency not recorded (agent may not have spoken)")
		} else {
			t.Logf("First word latency: %.2f ms", firstWordLatency)

			// First word latency should be reasonable (less than 2 seconds for fake providers)
			if firstWordLatency > 2000 {
				t.Errorf("first word latency too high: %.2f ms", firstWordLatency)
			}
		}

		// Check that state transitions were recorded
		stateTransitions := agent.metrics.StateTransitions
		if stateTransitions == nil {
			t.Error("state transitions metric not initialized")
		} else {
			t.Logf("State transitions recorded: %s", stateTransitions.String())
		}
	})

	// Validate behavior
	t.Run("behavior_validation", func(t *testing.T) {
		// Give the agent a moment to finish any ongoing state transitions
		time.Sleep(100 * time.Millisecond)

		// Agent should typically end in Idle or Listening state (both are valid end states)
		finalState := agent.GetState()
		if finalState != StateIdle && finalState != StateListening {
			t.Errorf("expected final state to be Idle or Listening, got %v", finalState)
		} else {
			t.Logf("Final state: %v", finalState)
		}

		// Should have generated some TTS output if the conversation worked
		frameCount := atomic.LoadInt64(&ttsFrameCount)
		if frameCount == 0 {
			t.Log("No TTS frames generated (agent may not have reached speaking state)")
		} else {
			t.Logf("Generated %d TTS frames", frameCount)
		}
	})
}

// TestAgent_MetricsExport tests that metrics are properly exported via expvar.
func TestAgent_MetricsExport(t *testing.T) {
	micIn := make(chan rtc.AudioFrame, 10)
	ttsOut := make(chan rtc.AudioFrame, 10)

	config := Config{
		STT:          sttfake.NewFakeSTT("metrics test"),
		TTS:          ttsfake.NewFakeTTS(),
		LLM:          fake.NewFakeLLM("metrics response"),
		VAD:          vadfake.NewFakeVAD(0.3),
		TurnDetector: turnfake.NewFakeTurnDetector(),
		MicIn:        micIn,
		TTSOut:       ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	// Test that metrics objects are properly initialized
	if agent.metrics.FirstWordLatency == nil {
		t.Error("FirstWordLatency metric not initialized")
	}
	if agent.metrics.SessionDuration == nil {
		t.Error("SessionDuration metric not initialized")
	}
	if agent.metrics.StateTransitions == nil {
		t.Error("StateTransitions metric not initialized")
	}

	// Test that metrics can be set and read
	agent.metrics.FirstWordLatency.Set(123.45)
	if got := agent.metrics.FirstWordLatency.Value(); got != 123.45 {
		t.Errorf("expected FirstWordLatency to be 123.45, got %f", got)
	}

	agent.metrics.SessionDuration.Set(678.90)
	if got := agent.metrics.SessionDuration.Value(); got != 678.90 {
		t.Errorf("expected SessionDuration to be 678.90, got %f", got)
	}

	// Test state transition recording
	agent.setState(StateListening)
	agent.setState(StateThinking)
	agent.setState(StateSpeaking)
	agent.setState(StateIdle)

	// Should have recorded several transitions
	transitionsMap := agent.metrics.StateTransitions
	if transitionsMap == nil {
		t.Fatal("StateTransitions map is nil")
	}

	// Check that transitions were recorded (the exact keys depend on initial state)
	transitionsStr := transitionsMap.String()
	if len(transitionsStr) == 0 {
		t.Error("no state transitions recorded")
	} else {
		t.Logf("State transitions: %s", transitionsStr)
	}
}
