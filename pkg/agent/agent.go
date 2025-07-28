// Package agent implements the voice agent framework with a finite state machine
// that manages conversation flow through Idle → Listening → Thinking → Speaking states.
package agent

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
	"github.com/chriscow/livekit-agents-go/pkg/ai/vad"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
	"github.com/chriscow/livekit-agents-go/pkg/turn"
)

// Tool represents a function that the agent can call in response to LLM function calls.
type Tool struct {
	Name        string                                        // Function name
	Description string                                        // Description for the LLM
	Schema      map[string]any                                // JSON schema for parameters
	Handler     func(context.Context, string) (string, error) // Function handler (receives JSON args, returns result)
}

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
// a finite state machine. It coordinates STT, TTS, LLM, VAD, and turn detection
// components to provide a natural conversation experience.
type Agent struct {
	// AI components
	stt          stt.STT
	tts          tts.TTS
	llm          llm.LLM
	vad          vad.VAD
	turnDetector turn.Detector

	// Tools for function calling
	tools map[string]Tool

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

	// Conversation history for turn detection
	conversation   []llm.Message
	conversationMu sync.RWMutex
	language       string // Language for turn detection

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
	FirstWordLatency     *expvar.Float
	SessionDuration      *expvar.Float
	StateTransitions     *expvar.Map
	EndOfUtteranceDelay  *expvar.Float
	TurnInferenceLatency *expvar.Float
	EOUProbability       *expvar.Float
}

// Config holds configuration for creating an Agent.
type Config struct {
	STT          stt.STT
	TTS          tts.TTS
	LLM          llm.LLM
	VAD          vad.VAD
	TurnDetector turn.Detector

	MicIn  <-chan rtc.AudioFrame
	TTSOut chan<- rtc.AudioFrame

	// Tools for function calling (optional)
	Tools []Tool

	// BackgroundAudio is optional background audio support
	BackgroundAudio *BackgroundAudio

	// Language for turn detection (optional, defaults to "en-US")
	Language string
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
	if cfg.TurnDetector == nil {
		return nil, fmt.Errorf("TurnDetector is required")
	}
	if cfg.MicIn == nil {
		return nil, fmt.Errorf("MicIn channel is required")
	}
	if cfg.TTSOut == nil {
		return nil, fmt.Errorf("TTSOut channel is required")
	}

	// Initialize tools map
	toolsMap := make(map[string]Tool)
	for _, tool := range cfg.Tools {
		toolsMap[tool.Name] = tool
	}

	// Set default language if not provided
	language := cfg.Language
	if language == "" {
		language = "en-US"
	}

	a := &Agent{
		stt:             cfg.STT,
		tts:             cfg.TTS,
		llm:             cfg.LLM,
		vad:             cfg.VAD,
		turnDetector:    cfg.TurnDetector,
		tools:           toolsMap,
		micIn:           cfg.MicIn,
		ttsOut:          cfg.TTSOut,
		interrupts:      make(chan struct{}, 1),
		shutdown:        make(chan struct{}),
		metrics:         newAgentMetrics(),
		backgroundAudio: cfg.BackgroundAudio,
		conversation:    make([]llm.Message, 0),
		language:        language,
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
			return a.handleSpeechEnd(ctx)
		}
	}

	return nil
}

// handleSpeechEnd processes speech end events using turn detection.
func (a *Agent) handleSpeechEnd(ctx context.Context) error {
	// Start VAD-silence timer and turn detection
	go a.runTurnDetection(ctx)
	return nil
}

// runTurnDetection runs the turn detection logic after VAD speech end.
func (a *Agent) runTurnDetection(ctx context.Context) {
	silenceStart := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond) // Check every 50ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if we've been silent for at least 50ms
			if time.Since(silenceStart) < 50*time.Millisecond {
				continue
			}

			// Get conversation history for turn detection
			a.conversationMu.RLock()
			chatCtx := turn.ChatContext{
				Messages: a.conversation,
				Language: a.language,
			}
			a.conversationMu.RUnlock()

			// Run turn detection with timing
			inferenceStart := time.Now()
			probability, err := a.turnDetector.PredictEndOfTurn(ctx, chatCtx)
			inferenceLatency := time.Since(inferenceStart)
			// Structured logging for inference
			slog.Debug("turn detection inference",
				slog.Float64("eou_probability", probability),
				slog.Float64("inference_latency_ms", float64(inferenceLatency.Milliseconds())),
				slog.String("language", a.language),
			)

			// Record metrics
			a.metrics.TurnInferenceLatency.Set(float64(inferenceLatency.Milliseconds()))
			if err == nil {
				a.metrics.EOUProbability.Set(probability)
			}

			if err != nil {
				log.Printf("Turn detection failed (latency: %v): %v", inferenceLatency, err)
				// Fall back to thinking state after 2s timeout
				if time.Since(silenceStart) >= 2*time.Second {
					endOfUtteranceDelay := time.Since(silenceStart)
					a.metrics.EndOfUtteranceDelay.Set(float64(endOfUtteranceDelay.Milliseconds()))
					log.Printf("Turn ended by timeout after %v (probability: N/A, threshold: N/A)", endOfUtteranceDelay)
					// Structured logging for timeout end
					slog.Info("turn ended",
						slog.String("reason", "timeout"),
						slog.Float64("end_of_utterance_delay_ms", float64(endOfUtteranceDelay.Milliseconds())),
					)

					a.setState(StateThinking)
					a.startThinking(ctx)
					return
				}
				continue
			}

			// Get threshold for current language, with fallback to base code
			threshold, err := a.turnDetector.UnlikelyThreshold(a.language)
			if err != nil && strings.Contains(a.language, "-") {
				parts := strings.SplitN(a.language, "-", 2)
				threshold, _ = a.turnDetector.UnlikelyThreshold(parts[0]) // fallback
			}
			if err != nil {
				threshold = 0.85 // Default threshold
			}

			// Check if we should end the turn
			shouldEndTurn := probability >= threshold || time.Since(silenceStart) >= 2*time.Second

			if shouldEndTurn {
				endOfUtteranceDelay := time.Since(silenceStart)
				a.metrics.EndOfUtteranceDelay.Set(float64(endOfUtteranceDelay.Milliseconds()))
				// Structured logging for probability end
				reason := "probability"
				if time.Since(silenceStart) >= 2*time.Second {
					reason = "timeout"
				}
				slog.Info("turn ended",
					slog.String("reason", reason),
					slog.Float64("eou_probability", probability),
					slog.Float64("threshold", threshold),
					slog.Float64("end_of_utterance_delay_ms", float64(endOfUtteranceDelay.Milliseconds())),
				)

				endReason := "probability"
				if time.Since(silenceStart) >= 2*time.Second {
					endReason = "timeout"
				}

				log.Printf("Turn ended by %s after %v (probability: %.3f, threshold: %.3f, inference: %v)",
					endReason, endOfUtteranceDelay, probability, threshold, inferenceLatency)

				a.setState(StateThinking)
				a.startThinking(ctx)
				return
			}
		}
	}
}

// handleSTTEvent processes speech-to-text events.
func (a *Agent) handleSTTEvent(ctx context.Context, event stt.SpeechEvent) error {
	if event.Type == stt.SpeechEventFinal && a.GetState() == StateThinking {
		// Add user message to conversation history
		a.conversationMu.Lock()
		a.conversation = append(a.conversation, llm.Message{
			Role:    llm.RoleUser,
			Content: event.Text,
		})
		a.conversationMu.Unlock()

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

// processLLMResponse handles LLM processing with tool calling and TTS synthesis.
func (a *Agent) processLLMResponse(ctx context.Context, transcript string) error {
	// Initialize conversation with user message
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: transcript},
	}

	// Convert tools to function definitions for LLM
	var functions []llm.FunctionDefinition
	for _, tool := range a.tools {
		functions = append(functions, llm.FunctionDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Schema,
		})
	}

	// Tool calling loop with max depth to prevent infinite loops
	const maxToolCalls = 10
	for i := 0; i < maxToolCalls; i++ {
		response, err := a.llm.Chat(ctx, llm.ChatRequest{
			Messages:  messages,
			Functions: functions,
		})
		if err != nil {
			return fmt.Errorf("LLM chat failed: %w", err)
		}

		// If no function call, we're done - start speaking
		if response.FunctionCall == nil {
			// Add assistant message to conversation history
			a.conversationMu.Lock()
			a.conversation = append(a.conversation, llm.Message{
				Role:    llm.RoleAssistant,
				Content: response.Message.Content,
			})
			a.conversationMu.Unlock()

			a.setState(StateSpeaking)
			return a.startSpeaking(ctx, response.Message.Content)
		}

		// Handle function call
		log.Printf("🔧 LLM requested function call: %s", response.FunctionCall.Name)

		// Add assistant message with function call to history
		messages = append(messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: response.Message.Content,
		})

		// Execute the function
		toolResult, err := a.executeTool(ctx, response.FunctionCall)
		if err != nil {
			log.Printf("❌ Tool execution failed: %v", err)
			toolResult = fmt.Sprintf("Error: %s", err.Error())
		}

		// Add function result to conversation history
		messages = append(messages, llm.Message{
			Role:    llm.RoleFunction,
			Content: toolResult,
			Name:    response.FunctionCall.Name,
		})

		log.Printf("✅ Tool %s executed, result: %s", response.FunctionCall.Name, toolResult)
	}

	// If we hit max tool calls, continue with final response
	log.Printf("⚠️ Maximum tool calls (%d) reached, proceeding with final response", maxToolCalls)
	response, err := a.llm.Chat(ctx, llm.ChatRequest{
		Messages: messages,
	})
	if err != nil {
		return fmt.Errorf("final LLM chat failed: %w", err)
	}

	// Add final assistant message to conversation history
	a.conversationMu.Lock()
	a.conversation = append(a.conversation, llm.Message{
		Role:    llm.RoleAssistant,
		Content: response.Message.Content,
	})
	a.conversationMu.Unlock()

	a.setState(StateSpeaking)
	return a.startSpeaking(ctx, response.Message.Content)
}

// executeTool executes a function call and returns the result
func (a *Agent) executeTool(ctx context.Context, functionCall *llm.FunctionCall) (string, error) {
	tool, exists := a.tools[functionCall.Name]
	if !exists {
		return "", fmt.Errorf("unknown function: %s", functionCall.Name)
	}

	// Validate that arguments is valid JSON
	var args map[string]any
	if err := json.Unmarshal([]byte(functionCall.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid function arguments JSON: %w", err)
	}

	// Execute the tool handler
	return tool.Handler(ctx, functionCall.Arguments)
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
	endOfUtteranceDelay := &expvar.Float{}
	turnInferenceLatency := &expvar.Float{}
	eouProbability := &expvar.Float{}

	return &AgentMetrics{
		FirstWordLatency:     firstWordLatency,
		SessionDuration:      sessionDuration,
		StateTransitions:     stateTransitions,
		EndOfUtteranceDelay:  endOfUtteranceDelay,
		TurnInferenceLatency: turnInferenceLatency,
		EOUProbability:       eouProbability,
	}
}
