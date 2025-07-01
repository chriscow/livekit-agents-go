package agents

import "errors"

var (
	// ErrAgentNotStarted indicates the agent has not been started
	ErrAgentNotStarted = errors.New("agent not started")
	
	// ErrAgentAlreadyStarted indicates the agent is already running
	ErrAgentAlreadyStarted = errors.New("agent already started")
	
	// ErrSessionNotFound indicates the session was not found
	ErrSessionNotFound = errors.New("session not found")
	
	// ErrInvalidConfiguration indicates invalid configuration
	ErrInvalidConfiguration = errors.New("invalid configuration")
	
	// ErrServiceNotAvailable indicates a required service is not available
	ErrServiceNotAvailable = errors.New("service not available")
	
	// ErrPluginNotFound indicates a plugin was not found
	ErrPluginNotFound = errors.New("plugin not found")
	
	// ErrToolNotFound indicates a function tool was not found
	ErrToolNotFound = errors.New("tool not found")
	
	// ErrInvalidArguments indicates invalid function arguments
	ErrInvalidArguments = errors.New("invalid arguments")
	
	// ErrRoomConnectionFailed indicates room connection failed
	ErrRoomConnectionFailed = errors.New("room connection failed")
	
	// ErrAudioProcessingFailed indicates audio processing failed
	ErrAudioProcessingFailed = errors.New("audio processing failed")
)