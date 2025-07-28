package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm/fake"
	sttfake "github.com/chriscow/livekit-agents-go/pkg/ai/stt/fake"
	ttsfake "github.com/chriscow/livekit-agents-go/pkg/ai/tts/fake"
	vadfake "github.com/chriscow/livekit-agents-go/pkg/ai/vad/fake"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

func TestAgent_New(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				STT:    sttfake.NewFakeSTT("test"),
				TTS:    ttsfake.NewFakeTTS(),
				LLM:    fake.NewFakeLLM(),
				VAD:    vadfake.NewFakeVAD(0.3),
				MicIn:  make(<-chan rtc.AudioFrame),
				TTSOut: make(chan<- rtc.AudioFrame),
			},
			expectError: false,
		},
		{
			name: "missing STT",
			config: Config{
				TTS:    ttsfake.NewFakeTTS(),
				LLM:    fake.NewFakeLLM(),
				VAD:    vadfake.NewFakeVAD(0.3),
				MicIn:  make(<-chan rtc.AudioFrame),
				TTSOut: make(chan<- rtc.AudioFrame),
			},
			expectError: true,
		},
		{
			name: "missing TTS",
			config: Config{
				STT:    sttfake.NewFakeSTT("test"),
				LLM:    fake.NewFakeLLM(),
				VAD:    vadfake.NewFakeVAD(0.3),
				MicIn:  make(<-chan rtc.AudioFrame),
				TTSOut: make(chan<- rtc.AudioFrame),
			},
			expectError: true,
		},
		{
			name: "missing LLM",
			config: Config{
				STT:    sttfake.NewFakeSTT("test"),
				TTS:    ttsfake.NewFakeTTS(),
				VAD:    vadfake.NewFakeVAD(0.3),
				MicIn:  make(<-chan rtc.AudioFrame),
				TTSOut: make(chan<- rtc.AudioFrame),
			},
			expectError: true,
		},
		{
			name: "missing VAD",
			config: Config{
				STT:    sttfake.NewFakeSTT("test"),
				TTS:    ttsfake.NewFakeTTS(),
				LLM:    fake.NewFakeLLM(),
				MicIn:  make(<-chan rtc.AudioFrame),
				TTSOut: make(chan<- rtc.AudioFrame),
			},
			expectError: true,
		},
		{
			name: "missing MicIn",
			config: Config{
				STT:    sttfake.NewFakeSTT("test"),
				TTS:    ttsfake.NewFakeTTS(),
				LLM:    fake.NewFakeLLM(),
				VAD:    vadfake.NewFakeVAD(0.3),
				TTSOut: make(chan<- rtc.AudioFrame),
			},
			expectError: true,
		},
		{
			name: "missing TTSOut",
			config: Config{
				STT:   sttfake.NewFakeSTT("test"),
				TTS:   ttsfake.NewFakeTTS(),
				LLM:   fake.NewFakeLLM(),
				VAD:   vadfake.NewFakeVAD(0.3),
				MicIn: make(<-chan rtc.AudioFrame),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := New(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if agent != nil {
					t.Errorf("expected nil agent, got %v", agent)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if agent == nil {
					t.Errorf("expected valid agent, got nil")
				} else {
					// Verify initial state
					if agent.GetState() != StateIdle {
						t.Errorf("expected initial state to be Idle, got %v", agent.GetState())
					}
					agent.Close()
				}
			}
		})
	}
}

func TestAgent_StateTransitions(t *testing.T) {
	micIn := make(chan rtc.AudioFrame, 10)
	ttsOut := make(chan rtc.AudioFrame, 10)

	sttProvider := sttfake.NewFakeSTT("Hello world")
	ttsProvider := ttsfake.NewFakeTTS()
	llmProvider := fake.NewFakeLLM()
	vadProvider := vadfake.NewFakeVAD(0.3)

	llmProvider = fake.NewFakeLLM("Echo: Hello world")

	config := Config{
		STT:    sttProvider,
		TTS:    ttsProvider,
		LLM:    llmProvider,
		VAD:    vadProvider,
		MicIn:  micIn,
		TTSOut: ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	// Initial state should be Idle
	if agent.GetState() != StateIdle {
		t.Errorf("expected initial state to be Idle, got %v", agent.GetState())
	}

	// Create a job for the agent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobInstance, err := job.New(ctx, job.Config{
		RoomName: "test-room",
		Timeout:  time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Start agent in background
	agentDone := make(chan error, 1)
	go func() {
		agentDone <- agent.Start(ctx, jobInstance)
	}()

	// Send some audio frames to trigger VAD and STT
	go func() {
		defer close(micIn)
		for i := 0; i < 10; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960),
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
				Timestamp:         time.Duration(i) * 10 * time.Millisecond,
			}
			// Add some "speech" data
			for j := range frame.Data {
				frame.Data[j] = byte((i + j) % 256)
			}

			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Consume TTS output
	go func() {
		for {
			select {
			case <-ttsOut:
				// Just consume the frames
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion or timeout
	select {
	case err := <-agentDone:
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("agent failed with error: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Error("agent test timed out")
		cancel()
	}
}

func TestAgent_Interrupt(t *testing.T) {
	micIn := make(chan rtc.AudioFrame, 10)
	ttsOut := make(chan rtc.AudioFrame, 10)

	config := Config{
		STT:    sttfake.NewFakeSTT("test speech"),
		TTS:    ttsfake.NewFakeTTS(),
		LLM:    fake.NewFakeLLM(),
		VAD:    vadfake.NewFakeVAD(0.3),
		MicIn:  micIn,
		TTSOut: ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	// Test that interrupt doesn't panic
	agent.Interrupt()

	// Test multiple interrupts don't block
	for i := 0; i < 5; i++ {
		agent.Interrupt()
	}
}

func TestAgent_Close(t *testing.T) {
	micIn := make(chan rtc.AudioFrame)
	ttsOut := make(chan rtc.AudioFrame)

	config := Config{
		STT:    sttfake.NewFakeSTT("test"),
		TTS:    ttsfake.NewFakeTTS(),
		LLM:    fake.NewFakeLLM(),
		VAD:    vadfake.NewFakeVAD(0.3),
		MicIn:  micIn,
		TTSOut: ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Close should not panic
	err = agent.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}

	// Multiple closes should not panic
	err = agent.Close()
	if err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}
}

func TestAgent_StateString(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected string
	}{
		{StateIdle, "Idle"},
		{StateListening, "Listening"},
		{StateThinking, "Thinking"},
		{StateSpeaking, "Speaking"},
		{AgentState(999), "Unknown(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestAgent_RaceConditions runs multiple goroutines performing various operations
// to test for race conditions using go test -race
func TestAgent_RaceConditions(t *testing.T) {
	micIn := make(chan rtc.AudioFrame, 100)
	ttsOut := make(chan rtc.AudioFrame, 100)

	config := Config{
		STT:    sttfake.NewFakeSTT("race test"),
		TTS:    ttsfake.NewFakeTTS(),
		LLM:    fake.NewFakeLLM(),
		VAD:    vadfake.NewFakeVAD(0.3),
		MicIn:  micIn,
		TTSOut: ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobInstance, err := job.New(ctx, job.Config{
		RoomName: "race-test-room",
		Timeout:  time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	var wg sync.WaitGroup

	// Start agent
	wg.Add(1)
	go func() {
		defer wg.Done()
		agent.Start(ctx, jobInstance)
	}()

	// Goroutine 1: Send audio frames
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(micIn)
		for i := 0; i < 50; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960),
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine 2: Consume TTS output
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ttsOut:
				// Just consume
			case <-ctx.Done():
				return
			}
		}
	}()

	// Goroutine 3: Call interrupt repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			agent.Interrupt()
			time.Sleep(5 * time.Millisecond)
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Goroutine 4: Check state repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = agent.GetState()
			time.Sleep(time.Millisecond)
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Wait for all goroutines or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-time.After(5 * time.Second):
		t.Error("race condition test timed out")
		cancel()
		<-done // Wait for cleanup
	}
}

// TestAgent_SimulateConversation tests a full conversation flow
func TestAgent_SimulateConversation(t *testing.T) {
	micIn := make(chan rtc.AudioFrame, 100)
	ttsOut := make(chan rtc.AudioFrame, 100)

	sttProvider := sttfake.NewFakeSTT("Hello, how are you?")
	ttsProvider := ttsfake.NewFakeTTS()
	llmProvider := fake.NewFakeLLM()
	vadProvider := vadfake.NewFakeVAD(0.3)

	// Set up LLM responses
	llmProvider = fake.NewFakeLLM(
		"I'm doing well, thank you for asking!",
		"How can I help you today?",
	)

	config := Config{
		STT:    sttProvider,
		TTS:    ttsProvider,
		LLM:    llmProvider,
		VAD:    vadProvider,
		MicIn:  micIn,
		TTSOut: ttsOut,
	}

	agent, err := New(config)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobInstance, err := job.New(ctx, job.Config{
		RoomName: "conversation-test",
		Timeout:  time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Track state changes
	stateChanges := make([]AgentState, 0)
	stateMu := sync.Mutex{}

	// Monitor state changes
	go func() {
		lastState := agent.GetState()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				currentState := agent.GetState()
				if currentState != lastState {
					stateMu.Lock()
					stateChanges = append(stateChanges, currentState)
					stateMu.Unlock()
					lastState = currentState
				}
			}
		}
	}()

	// Start agent
	agentDone := make(chan error, 1)
	go func() {
		agentDone <- agent.Start(ctx, jobInstance)
	}()

	// Simulate speech input
	go func() {
		defer close(micIn)
		// Send silence first
		for i := 0; i < 10; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960),
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Send speech
		for i := 0; i < 50; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960),
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
			}
			// Fill with non-zero data to simulate speech
			for j := range frame.Data {
				frame.Data[j] = byte((i + j) % 256)
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Send silence again
		for i := 0; i < 10; i++ {
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960),
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
			}
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Consume TTS output
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
	case <-time.After(4 * time.Second):
		t.Error("conversation test timed out")
		cancel()
	}

	// Verify we had some state changes (though exact sequence may vary due to timing)
	stateMu.Lock()
	if len(stateChanges) == 0 {
		t.Error("expected some state changes during conversation, got none")
	}
	stateMu.Unlock()

	t.Logf("Conversation completed with %d state changes and %d TTS frames", len(stateChanges), atomic.LoadInt64(&ttsFrameCount))
}
