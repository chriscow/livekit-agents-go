package agents

import (
	"context"
	"livekit-agents-go/services/llm"
)

// Agent represents a LiveKit agent implementation with Python framework compatibility
type Agent interface {
	// Start the agent with session context (existing method)
	Start(ctx context.Context, session *AgentSession) error

	// Stop the agent gracefully (existing method)
	Stop() error

	// Handle events from the session (existing method)
	HandleEvent(event AgentEvent) error

	// Python framework compatibility methods

	// OnEnter is called when the agent enters the session (Python equivalent)
	OnEnter(ctx context.Context, session *AgentSession) error

	// OnExit is called when the agent exits the session (Python equivalent)
	OnExit(ctx context.Context, session *AgentSession) error

	// OnUserTurnCompleted is called after each user turn in conversation (Python equivalent)
	OnUserTurnCompleted(ctx context.Context, chatCtx *llm.ChatContext, newMessage *llm.ChatMessage) error

	// Runtime update methods (Python equivalent)

	// UpdateInstructions updates the agent's system instructions
	UpdateInstructions(instructions string) error

	// UpdateChatContext updates the conversation context
	UpdateChatContext(chatCtx *llm.ChatContext) error

	// Audio processing methods (for console mode integration)

	// OnAudioFrame is called when audio frames are received
	OnAudioFrame(frame any)

	// OnSpeechDetected is called when VAD detects speech
	OnSpeechDetected(probability float64)

	// OnSpeechEnded is called when VAD detects speech has ended
	OnSpeechEnded()
}

// AgentEvent represents events that agents respond to
type AgentEvent interface {
	Type() AgentEventType
	Data() any
}

type AgentEventType int

const (
	EventUserSpeechStart AgentEventType = iota
	EventUserSpeechEnd
	EventUserTranscript
	EventAgentResponseStart
	EventAgentResponseEnd
	EventParticipantJoined
	EventParticipantLeft
	EventTrackSubscribed
	EventDataReceived
)

// BaseAgent provides common agent functionality with Python framework compatibility
type BaseAgent struct {
	name         string
	metadata     map[string]string
	instructions string
	chatContext  *llm.ChatContext
}

func NewBaseAgent(name string) *BaseAgent {
	return &BaseAgent{
		name:         name,
		metadata:     make(map[string]string),
		instructions: "",
		chatContext: &llm.ChatContext{
			Messages: make([]llm.ChatMessage, 0),
		},
	}
}

// NewBaseAgentWithInstructions creates a BaseAgent with system instructions
func NewBaseAgentWithInstructions(name, instructions string) *BaseAgent {
	agent := NewBaseAgent(name)
	agent.instructions = instructions
	return agent
}

func (a *BaseAgent) Start(ctx context.Context, session *AgentSession) error {
	// Default implementation - can be overridden
	return nil
}

func (a *BaseAgent) Stop() error {
	return nil
}

func (a *BaseAgent) HandleEvent(event AgentEvent) error {
	// Default event handling
	return nil
}

// Python framework compatibility implementations

// OnEnter default implementation (can be overridden)
func (a *BaseAgent) OnEnter(ctx context.Context, session *AgentSession) error {
	// Default implementation - agents can override this
	return nil
}

// OnExit default implementation (can be overridden)
func (a *BaseAgent) OnExit(ctx context.Context, session *AgentSession) error {
	// Default implementation - agents can override this
	return nil
}

// OnUserTurnCompleted default implementation (can be overridden)
func (a *BaseAgent) OnUserTurnCompleted(ctx context.Context, chatCtx *llm.ChatContext, newMessage *llm.ChatMessage) error {
	// Default implementation - agents can override this
	return nil
}

// UpdateInstructions updates the agent's system instructions
func (a *BaseAgent) UpdateInstructions(instructions string) error {
	a.instructions = instructions
	return nil
}

// UpdateChatContext updates the conversation context
func (a *BaseAgent) UpdateChatContext(chatCtx *llm.ChatContext) error {
	a.chatContext = chatCtx
	return nil
}

// Audio processing default implementations (can be overridden)

// OnAudioFrame default implementation
func (a *BaseAgent) OnAudioFrame(frame any) {
	// Default implementation - agents can override this for diagnostics
}

// OnSpeechDetected default implementation
func (a *BaseAgent) OnSpeechDetected(probability float64) {
	// Default implementation - agents can override this
}

// OnSpeechEnded default implementation
func (a *BaseAgent) OnSpeechEnded() {
	// Default implementation - agents can override this
}

func (a *BaseAgent) Name() string {
	return a.name
}

func (a *BaseAgent) SetMetadata(key, value string) {
	a.metadata[key] = value
}

func (a *BaseAgent) GetMetadata(key string) string {
	return a.metadata[key]
}

// GetInstructions returns the current system instructions
func (a *BaseAgent) GetInstructions() string {
	return a.instructions
}

// GetChatContext returns the current chat context
func (a *BaseAgent) GetChatContext() *llm.ChatContext {
	return a.chatContext
}
