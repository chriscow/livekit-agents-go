package agents

import "context"

// Agent represents a LiveKit agent implementation
type Agent interface {
	// Start the agent with session context
	Start(ctx context.Context, session *AgentSession) error
	
	// Stop the agent gracefully
	Stop() error
	
	// Handle events from the session
	HandleEvent(event AgentEvent) error
}

// AgentEvent represents events that agents respond to
type AgentEvent interface {
	Type() AgentEventType
	Data() interface{}
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

// BaseAgent provides common agent functionality
type BaseAgent struct {
	name     string
	metadata map[string]string
}

func NewBaseAgent(name string) *BaseAgent {
	return &BaseAgent{
		name:     name,
		metadata: make(map[string]string),
	}
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

func (a *BaseAgent) Name() string {
	return a.name
}

func (a *BaseAgent) SetMetadata(key, value string) {
	a.metadata[key] = value
}

func (a *BaseAgent) GetMetadata(key string) string {
	return a.metadata[key]
}