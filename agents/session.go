package agents

import (
	"context"
	"sync"
	"time"

	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"

	lksdk "github.com/livekit/server-sdk-go/v2"
)

// AgentSession manages voice pipeline and agent interaction (equivalent to Python AgentSession)
type AgentSession struct {
	// Core services - composed from plugins
	VAD stt.STT
	STT stt.STT
	LLM llm.LLM
	TTS tts.TTS

	// Pipeline configuration
	Pipeline *VoicePipeline

	// Agent implementation
	Agent Agent

	// Room and context
	Room    *lksdk.Room
	Context context.Context

	// State management
	State    SessionState
	UserData map[string]interface{}

	// Synchronization
	mu sync.RWMutex
}

type SessionState int

const (
	SessionStateIdle SessionState = iota
	SessionStateListening
	SessionStateProcessing
	SessionStateResponding
)

// NewAgentSession creates a new agent session
func NewAgentSession(ctx context.Context) *AgentSession {
	return &AgentSession{
		Context:  ctx,
		State:    SessionStateIdle,
		UserData: make(map[string]interface{}),
	}
}

// Start the agent session with voice pipeline
func (s *AgentSession) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Initialize voice pipeline if configured
	if s.Pipeline != nil {
		if err := s.Pipeline.Start(s.Context); err != nil {
			return err
		}
	}

	// Start agent if configured
	if s.Agent != nil {
		return s.Agent.Start(s.Context, s)
	}

	return nil
}

// Stop the agent session gracefully
func (s *AgentSession) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop agent
	if s.Agent != nil {
		if err := s.Agent.Stop(); err != nil {
			return err
		}
	}

	// Stop pipeline
	if s.Pipeline != nil {
		return s.Pipeline.Stop()
	}

	return nil
}

// SetState updates the session state
func (s *AgentSession) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

// GetState returns the current session state
func (s *AgentSession) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// SetUserData sets user data on the session
func (s *AgentSession) SetUserData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UserData[key] = value
}

// GetUserData gets user data from the session
func (s *AgentSession) GetUserData(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.UserData[key]
}

// VoicePipeline orchestrates voice processing components
type VoicePipeline struct {
	// Pipeline components
	components []PipelineComponent

	// Configuration
	VADSettings VADSettings
	STTSettings STTSettings
	LLMSettings LLMSettings
	TTSSettings TTSSettings

	// Turn detection
	TurnDetection TurnDetectionSettings

	// State
	running bool
	mu      sync.RWMutex
}

// PipelineComponent represents a component in the voice pipeline
type PipelineComponent interface {
	Process(ctx context.Context, input interface{}) (interface{}, error)
	Start(ctx context.Context) error
	Stop() error
}

// VADSettings configures voice activity detection
type VADSettings struct {
	Threshold          float64
	MinSpeechDuration  time.Duration
	MinSilenceDuration time.Duration
	Model              string
}

// STTSettings configures speech-to-text
type STTSettings struct {
	Language       string
	Model          string
	InterimResults bool
	WordTimestamps bool
}

// LLMSettings configures large language model
type LLMSettings struct {
	Model        string
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
}

// TTSSettings configures text-to-speech
type TTSSettings struct {
	Voice    string
	Language string
	Speed    float64
	Pitch    float64
	Volume   float64
}

// TurnDetectionSettings configures turn detection
type TurnDetectionSettings struct {
	Enabled   bool
	Threshold float64
	Timeout   time.Duration
	Model     string
}

// NewVoicePipeline creates a new voice pipeline
func NewVoicePipeline() *VoicePipeline {
	return &VoicePipeline{
		components: make([]PipelineComponent, 0),
		VADSettings: VADSettings{
			Threshold:          0.5,
			MinSpeechDuration:  100 * time.Millisecond,
			MinSilenceDuration: 100 * time.Millisecond,
		},
		STTSettings: STTSettings{
			Language:       "en-US",
			InterimResults: true,
		},
		LLMSettings: LLMSettings{
			Temperature: 0.7,
			MaxTokens:   150,
		},
		TTSSettings: TTSSettings{
			Speed:  1.0,
			Pitch:  1.0,
			Volume: 1.0,
		},
		TurnDetection: TurnDetectionSettings{
			Enabled:   true,
			Threshold: 0.5,
			Timeout:   3 * time.Second,
		},
	}
}

// AddComponent adds a component to the pipeline
func (vp *VoicePipeline) AddComponent(component PipelineComponent) {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	vp.components = append(vp.components, component)
}

// Start starts the voice pipeline
func (vp *VoicePipeline) Start(ctx context.Context) error {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	if vp.running {
		return ErrAgentAlreadyStarted
	}

	// Start all components
	for _, component := range vp.components {
		if err := component.Start(ctx); err != nil {
			// Stop already started components
			for i := len(vp.components) - 1; i >= 0; i-- {
				vp.components[i].Stop()
			}
			return err
		}
	}

	vp.running = true
	return nil
}

// Stop stops the voice pipeline
func (vp *VoicePipeline) Stop() error {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	if !vp.running {
		return nil
	}

	// Stop all components in reverse order
	for i := len(vp.components) - 1; i >= 0; i-- {
		if err := vp.components[i].Stop(); err != nil {
			// Continue stopping other components even if one fails
			continue
		}
	}

	vp.running = false
	return nil
}

// Process processes input through the pipeline
func (vp *VoicePipeline) Process(ctx context.Context, input interface{}) (interface{}, error) {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	if !vp.running {
		return nil, ErrAgentNotStarted
	}

	current := input
	for _, component := range vp.components {
		var err error
		current, err = component.Process(ctx, current)
		if err != nil {
			return nil, err
		}
	}

	return current, nil
}

// IsRunning returns true if the pipeline is running
func (vp *VoicePipeline) IsRunning() bool {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	return vp.running
}

// GetComponentCount returns the number of components in the pipeline
func (vp *VoicePipeline) GetComponentCount() int {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	return len(vp.components)
}
