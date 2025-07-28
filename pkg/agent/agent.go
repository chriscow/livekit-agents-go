// Package agent implements the voice agent framework with a finite state machine
// that manages conversation flow through Idle → Listening → Thinking → Speaking states.
package agent

import (
	"context"
	"expvar"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
	"github.com/chriscow/livekit-agents-go/pkg/ai/vad"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// AgentState represents the current state of the voice agent.
type AgentState int32

const (
	StateIdle AgentState = iota
	StateListening
	StateThinking
	StateSpeaking
)

func (s AgentState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateListening:
		return "Listening"
	case StateThinking:
		return "Thinking"
	case StateSpeaking:
		return "Speaking"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// Agent represents a voice agent that manages conversation flow through
// a finite state machine. It coordinates STT, TTS, LLM, and VAD components
// to provide a natural conversation experience.
type Agent struct {
	// AI components
	stt stt.STT
	tts tts.TTS
	llm llm.LLM
	vad vad.VAD

	// State management
	state atomic.Int32

	// Channels for communication
	micIn  <-chan rtc.AudioFrame
	ttsOut chan<- rtc.AudioFrame

	// Control channels
	vadEvents    <-chan vad.VADEvent
	sttEvents    <-chan stt.SpeechEvent
	interrupts   chan struct{}
	shutdown     chan struct{}
	shutdownOnce sync.Once

	// Current streams
	sttStream      stt.STTStream
	streamMu       sync.Mutex
	feederCancel   context.CancelFunc
	feederCancelMu sync.Mutex
	feederDone     chan struct{}

	// Metrics
	sessionStart      time.Time
	firstWordTimeOnce sync.Once
	firstWordTime     time.Time
	metrics           *AgentMetrics

	// Background audio
	backgroundAudio *BackgroundAudio
}

// AgentMetrics holds performance metrics for the agent.
type AgentMetrics struct {
	FirstWordLatency *expvar.Float
	SessionDuration  *expvar.Float
	StateTransitions *expvar.Map
}

// Config holds configuration for creating an Agent.
type Config struct {
	STT stt.STT
	TTS tts.TTS
	LLM llm.LLM
	VAD vad.VAD

	MicIn  <-chan rtc.AudioFrame
	TTSOut chan<- rtc.AudioFrame

	// BackgroundAudio is optional background audio support
	BackgroundAudio *BackgroundAudio
}

// New creates a new Agent with the given configuration.
func New(cfg Config) (*Agent, error) {
	if cfg.STT == nil {
		return nil, fmt.Errorf("STT is required")
	}
	if cfg.TTS == nil {
		return nil, fmt.Errorf("TTS is required")
	}
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM is required")
	}
	if cfg.VAD == nil {
		return nil, fmt.Errorf("VAD is required")
	}
	if cfg.MicIn == nil {
		return nil, fmt.Errorf("MicIn channel is required")
	}
	if cfg.TTSOut == nil {
		return nil, fmt.Errorf("TTSOut channel is required")
	}

	a := &Agent{
		stt:             cfg.STT,
		tts:             cfg.TTS,
		llm:             cfg.LLM,
		vad:             cfg.VAD,
		micIn:           cfg.MicIn,
		ttsOut:          cfg.TTSOut,
		interrupts:      make(chan struct{}, 1),
		shutdown:        make(chan struct{}),
		metrics:         newAgentMetrics(),
		backgroundAudio: cfg.BackgroundAudio,
	}

	a.setState(StateIdle)
	return a, nil
}

// Start begins the agent's main loop with the given context and job.
// It returns when either context is cancelled or an unrecoverable error occurs.
// The agent will respond to cancellation from both the provided context and the job's context.
func (a *Agent) Start(ctx context.Context, j *job.Job) error {
	if j == nil {
		return fmt.Errorf("job is required")
	}

	// Combine both cancellation signals for flexible control
	combinedCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-j.Context.Done():
			cancel()
		case <-ctx.Done():
			cancel()
		case <-combinedCtx.Done():
			// Already cancelled
		}
	}()

	a.sessionStart = time.Now()
	defer a.updateSessionDuration()

	// Start VAD processing
	vadEvents, err := a.vad.Detect(combinedCtx, a.micIn)
	if err != nil {
		return fmt.Errorf("failed to start VAD: %w", err)
	}
	a.vadEvents = vadEvents

	// Main agent loop
	return a.run(combinedCtx)
}

// Interrupt signals the agent to stop current processing and transition to listening state.
func (a *Agent) Interrupt() {
	select {
	case a.interrupts <- struct{}{}:
	default:
		// Channel full, interrupt already pending
	}
}

// Close shuts down the agent and cleans up resources.
func (a *Agent) Close() error {
	a.shutdownOnce.Do(func() {
		close(a.shutdown)
	})

	// Clean up current streams
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	if a.sttStream != nil {
		a.sttStream.CloseSend()
		a.sttStream = nil
	}

	return nil
}

// GetState returns the current state of the agent.
func (a *Agent) GetState() AgentState {
	return AgentState(a.state.Load())
}

// setState atomically updates the agent's state and records metrics.
func (a *Agent) setState(newState AgentState) {
	oldState := AgentState(a.state.Swap(int32(newState)))

	// Record state transition metric
	transitionKey := fmt.Sprintf("%s_to_%s", oldState.String(), newState.String())
	if counter := a.metrics.StateTransitions.Get(transitionKey); counter != nil {
		counter.(*expvar.Int).Add(1)
	} else {
		newCounter := &expvar.Int{}
		newCounter.Set(1)
		a.metrics.StateTransitions.Set(transitionKey, newCounter)
	}
}

// run is the main agent loop that processes events and manages state transitions.
func (a *Agent) run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.shutdown:
			return nil
		case <-a.interrupts:
			if err := a.handleInterrupt(ctx); err != nil {
				return fmt.Errorf("interrupt handling failed: %w", err)
			}
		case vadEvent := <-a.vadEvents:
			if err := a.handleVADEvent(ctx, vadEvent); err != nil {
				return fmt.Errorf("VAD event handling failed: %w", err)
			}
		case sttEvent := <-a.sttEvents:
			if err := a.handleSTTEvent(ctx, sttEvent); err != nil {
				return fmt.Errorf("STT event handling failed: %w", err)
			}
		}
	}
}

// handleInterrupt processes interruption requests based on current state.
func (a *Agent) handleInterrupt(ctx context.Context) error {
	currentState := a.GetState()

	switch currentState {
	case StateSpeaking:
		// Cancel TTS playback and transition to listening
		a.setState(StateListening)
		return a.startListening(ctx)
	case StateThinking:
		// Cancel LLM processing and transition to listening
		a.setState(StateListening)
		return a.startListening(ctx)
	default:
		// No action needed for Idle or Listening states
		return nil
	}
}

// handleVADEvent processes voice activity detection events.
func (a *Agent) handleVADEvent(ctx context.Context, event vad.VADEvent) error {
	currentState := a.GetState()

	switch event.Type {
	case vad.VADEventSpeechStart:
		switch currentState {
		case StateIdle:
			a.setState(StateListening)
			return a.startListening(ctx)
		case StateSpeaking:
			// Interrupt current speech
			return a.handleInterrupt(ctx)
		}
	case vad.VADEventSpeechEnd:
		if currentState == StateListening {
			a.setState(StateThinking)
			return a.startThinking(ctx)
		}
	}

	return nil
}

// handleSTTEvent processes speech-to-text events.
func (a *Agent) handleSTTEvent(ctx context.Context, event stt.SpeechEvent) error {
	if event.Type == stt.SpeechEventFinal && a.GetState() == StateThinking {
		// Process the final transcript with LLM
		return a.processLLMResponse(ctx, event.Text)
	}
	return nil
}

// startListening begins STT processing for the current audio stream.
func (a *Agent) startListening(ctx context.Context) error {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	// Cancel existing feeder if any
	a.feederCancelMu.Lock()
	if a.feederCancel != nil {
		a.feederCancel()
		a.feederCancel = nil
	}
	a.feederCancelMu.Unlock()

	// Close existing stream if any
	if a.sttStream != nil {
		a.sttStream.CloseSend()
	}

	// Create new STT stream
	stream, err := a.stt.NewStream(ctx, stt.StreamConfig{
		SampleRate:  48000,
		NumChannels: 1,
		Lang:        "en-US",
		MaxRetry:    3,
	})
	if err != nil {
		return fmt.Errorf("failed to create STT stream: %w", err)
	}

	a.sttStream = stream
	a.sttEvents = stream.Events()

	// Start feeder goroutine to push microphone audio to STT
	feederCtx, feederCancel := context.WithCancel(ctx)
	feederDone := make(chan struct{})
	
	a.feederCancelMu.Lock()
	a.feederCancel = feederCancel
	a.feederDone = feederDone
	a.feederCancelMu.Unlock()

	go func() {
		defer close(feederDone) // Signal completion
		defer feederCancel()    // Ensures cleanup if feeder exits early
		for {
			select {
			case <-feederCtx.Done():
				return
			case frame, ok := <-a.micIn:
				if !ok {
					return
				}
				if err := stream.Push(frame); err != nil {
					// STT push failed, likely due to stream closure or error
					// Exit the feeder goroutine gracefully
					return
				}
			}
		}
	}()

	return nil
}

// startThinking transitions to thinking state and closes STT stream.
func (a *Agent) startThinking(ctx context.Context) error {
	// Stop the feeder first to avoid race with CloseSend
	var feederDone chan struct{}
	a.feederCancelMu.Lock()
	if a.feederCancel != nil {
		a.feederCancel()
		feederDone = a.feederDone
		a.feederCancel = nil
		a.feederDone = nil
	}
	a.feederCancelMu.Unlock()

	// Wait for feeder goroutine to complete before closing stream
	if feederDone != nil {
		select {
		case <-feederDone:
			// Feeder completed
		case <-ctx.Done():
			// Context cancelled, proceed anyway
		case <-time.After(100 * time.Millisecond):
			// Timeout to prevent indefinite blocking
		}
	}

	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	if a.sttStream != nil {
		if err := a.sttStream.CloseSend(); err != nil {
			return fmt.Errorf("failed to close STT stream: %w", err)
		}
	}

	return nil
}

// processLLMResponse handles LLM processing and TTS synthesis.
func (a *Agent) processLLMResponse(ctx context.Context, transcript string) error {
	// Send transcript to LLM
	response, err := a.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: transcript},
		},
	})
	if err != nil {
		return fmt.Errorf("LLM chat failed: %w", err)
	}

	// Start speaking
	a.setState(StateSpeaking)
	return a.startSpeaking(ctx, response.Message.Content)
}

// startSpeaking begins TTS synthesis and audio playback.
func (a *Agent) startSpeaking(ctx context.Context, text string) error {
	// Record first word latency if this is the first response (thread-safe)
	a.firstWordTimeOnce.Do(func() {
		a.firstWordTime = time.Now()
		latency := a.firstWordTime.Sub(a.sessionStart)
		a.metrics.FirstWordLatency.Set(float64(latency.Milliseconds()))
	})

	// Synthesize speech
	audioFrames, err := a.tts.Synthesize(ctx, tts.SynthesizeRequest{
		Text:     text,
		Voice:    "default",
		Language: "en-US",
	})
	if err != nil {
		return fmt.Errorf("TTS synthesis failed: %w", err)
	}

	// Stream audio frames to output
	go func() {
		defer func() {
			// Return to idle state when speaking is done
			a.setState(StateIdle)
		}()

		for frame := range audioFrames {
			// Mix with background audio if enabled
			if a.backgroundAudio != nil && a.backgroundAudio.IsEnabled() {
				frame = a.backgroundAudio.MixFrames(frame)
			}

			select {
			case a.ttsOut <- frame:
			case <-ctx.Done():
				return
			case <-a.shutdown:
				return
			}
		}
	}()

	return nil
}

// updateSessionDuration updates the session duration metric.
func (a *Agent) updateSessionDuration() {
	duration := time.Since(a.sessionStart)
	a.metrics.SessionDuration.Set(float64(duration.Milliseconds()))
}

// newAgentMetrics creates a new set of agent metrics with unique names.
func newAgentMetrics() *AgentMetrics {
	// Create individual metrics without global registration for testing
	firstWordLatency := &expvar.Float{}
	sessionDuration := &expvar.Float{}
	stateTransitions := &expvar.Map{}
	stateTransitions.Init()

	return &AgentMetrics{
		FirstWordLatency: firstWordLatency,
		SessionDuration:  sessionDuration,
		StateTransitions: stateTransitions,
	}
}
