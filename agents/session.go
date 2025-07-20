package agents

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tools"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/services/vad"

	lksdk "github.com/livekit/server-sdk-go/v2"
)

// AgentSession manages voice pipeline and agent interaction (Python framework compatible)
type AgentSession struct {
	// Core services - composed from plugins
	VAD vad.VAD
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

	// Python framework compatibility
	ChatContext *llm.ChatContext
	IsEntered   bool

	// Function tools integration
	ToolRegistry *tools.ToolRegistry

	// Turn detection
	TurnDetection TurnDetectionMode

	// Console mode TTS output callback
	ttsOutputCallback func(*media.AudioFrame)

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

// TurnDetectionMode defines how turn detection is performed (Python framework equivalent)
type TurnDetectionMode int

const (
	TurnDetectionVAD         TurnDetectionMode = iota // VAD-based turn detection
	TurnDetectionSTT                                  // STT-based turn detection
	TurnDetectionManual                               // Manual turn detection
	TurnDetectionRealTimeLLM                          // Real-time LLM turn detection
)

// NewAgentSession creates a new agent session
func NewAgentSession(ctx context.Context) *AgentSession {
	return &AgentSession{
		Context:       ctx,
		State:         SessionStateIdle,
		UserData:      make(map[string]interface{}),
		ChatContext:   llm.NewChatContext(),
		ToolRegistry:  tools.NewToolRegistry(),
		IsEntered:     false,
		TurnDetection: TurnDetectionVAD, // Default to VAD-based turn detection
	}
}

// NewAgentSessionWithInstructions creates a session with system instructions
func NewAgentSessionWithInstructions(ctx context.Context, instructions string) *AgentSession {
	session := NewAgentSession(ctx)
	session.ChatContext = llm.NewChatContextWithSystem(instructions)
	return session
}

// Start the agent session with voice pipeline (Python framework compatible)
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
		// Discover and register function tools from the agent
		discoveredTools, err := tools.DiscoverTools(s.Agent)
		if err != nil {
			return fmt.Errorf("failed to discover tools: %w", err)
		}

		// Register discovered tools in the session's tool registry
		registeredCount := 0
		for _, tool := range discoveredTools {
			if s.isValidFunctionTool(tool.Name()) {
				if err := s.ToolRegistry.Register(tool); err != nil {
					// Log warning but continue - some tools might conflict
					fmt.Printf("Warning: Failed to register tool %s: %v\n", tool.Name(), err)
				} else {
					registeredCount++
				}
			}
		}

		fmt.Printf("🔧 Registered %d function tools for LLM use\n", registeredCount)
		if registeredCount > 0 {
			toolNames := []string{}
			for _, tool := range s.ToolRegistry.List() {
				if s.isValidFunctionTool(tool.Name()) {
					toolNames = append(toolNames, tool.Name())
				}
			}
			fmt.Printf("📋 Available tools: %v\n", toolNames)
		}

		if err := s.Agent.Start(s.Context, s); err != nil {
			return err
		}

		// Call OnEnter if agent hasn't entered yet (Python framework pattern)
		if !s.IsEntered {
			if err := s.Agent.OnEnter(s.Context, s); err != nil {
				return err
			}
			s.IsEntered = true
		}

		// Skip automatic conversation loop - let ConsoleAgent handle audio pipeline
		// or dev/start modes handle LiveKit room connection
		fmt.Println("✅ Agent session started - ready for audio pipeline integration")
	}

	return nil
}

// Stop the agent session gracefully (Python framework compatible)
func (s *AgentSession) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Call OnExit if agent has entered (Python framework pattern)
	if s.Agent != nil && s.IsEntered {
		if err := s.Agent.OnExit(s.Context, s); err != nil {
			// Log error but continue with shutdown
			// TODO: Use proper logger when available
		}
		s.IsEntered = false
	}

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

// Python framework compatibility methods

// SetChatContext sets the chat context for the session
func (s *AgentSession) SetChatContext(ctx *llm.ChatContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChatContext = ctx
}

// GetChatContext gets the chat context from the session
func (s *AgentSession) GetChatContext() *llm.ChatContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ChatContext
}

// AddUserMessage adds a user message to the chat context
func (s *AgentSession) AddUserMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChatContext.AddUserMessage(content)
}

// AddAssistantMessage adds an assistant message to the chat context
func (s *AgentSession) AddAssistantMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChatContext.AddAssistantMessage(content)
}

// SetTurnDetection sets the turn detection mode
func (s *AgentSession) SetTurnDetection(mode TurnDetectionMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TurnDetection = mode
}

// GetTurnDetection gets the current turn detection mode
func (s *AgentSession) GetTurnDetection() TurnDetectionMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TurnDetection
}

// SetTTSOutputCallback sets the callback for TTS audio output in console mode
func (s *AgentSession) SetTTSOutputCallback(callback func(*media.AudioFrame)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttsOutputCallback = callback
}

// GenerateReply generates an initial reply when agent enters session (Python framework pattern)
func (s *AgentSession) GenerateReply() error {
	s.mu.RLock()
	agent := s.Agent
	llmService := s.LLM
	ttsService := s.TTS
	chatCtx := s.ChatContext
	s.mu.RUnlock()


	if agent == nil || llmService == nil {
		return fmt.Errorf("agent or LLM service not configured")
	}

	// Generate initial response based on system instructions
	messages := chatCtx.GetMessages()
	
	if len(messages) == 0 || messages[0].Role != llm.RoleSystem {
		// Add default greeting if no system prompt
		defaultGreeting := "Hello! How can I help you today?"
		s.AddAssistantMessage(defaultGreeting)
		
		// Synthesize TTS for default greeting
		if ttsService != nil {
			fmt.Println("🎵 [AUDIO] Synthesizing default greeting for console output...")
			ttsStart := time.Now()
			
			audioResponse, err := ttsService.Synthesize(s.Context, defaultGreeting, nil)
			if err != nil {
				fmt.Printf("❌ [AUDIO] Default greeting TTS synthesis failed: %v\n", err)
			} else {
				ttsElapsed := time.Since(ttsStart)
				fmt.Printf("✅ [AUDIO] Default greeting TTS synthesis completed in %v (%d bytes)\n", ttsElapsed, len(audioResponse.Data))
				
				// Send TTS audio to console if callback is set
				if s.ttsOutputCallback != nil {
					fmt.Println("🔊 [AUDIO] Sending default greeting TTS audio to console speakers...")
					s.ttsOutputCallback(audioResponse)
				} else {
					fmt.Println("⚠️  [AUDIO] No TTS output callback set - default greeting audio will not be played")
				}
			}
		}
		
		return nil
	}

	// Use streaming LLM to generate contextual greeting
	fmt.Printf("🌊 [PERFORMANCE] Starting streaming initial reply...\n")
	replyStart := time.Now()
	
	chatOpts := llm.DefaultChatOptions()
	stream, err := llmService.ChatStream(s.Context, messages, chatOpts)
	if err != nil {
		return fmt.Errorf("failed to start reply streaming: %w", err)
	}
	defer stream.Close()

	// Collect streaming initial reply
	var replyContent strings.Builder
	
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("failed to receive reply stream chunk: %w", err)
		}
		
		if chunk.Delta.Content != "" {
			replyContent.WriteString(chunk.Delta.Content)
			fmt.Printf("🎯 [PERFORMANCE] Initial reply token: %s", chunk.Delta.Content)
		}
	}
	
	replyElapsed := time.Since(replyStart)
	fmt.Printf("\n✅ [PERFORMANCE] Streaming initial reply completed in %v\n", replyElapsed)
	
	completion := &llm.ChatCompletion{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: replyContent.String(),
		},
	}

	s.AddAssistantMessage(completion.Message.Content)

	// Synthesize speech if TTS available and send to console output
	if ttsService != nil {
		fmt.Println("🎵 [AUDIO] Synthesizing initial greeting for console output...")
		ttsStart := time.Now()
		
		audioResponse, err := ttsService.Synthesize(s.Context, completion.Message.Content, nil)
		if err != nil {
			fmt.Printf("❌ [AUDIO] TTS synthesis failed: %v\n", err)
		} else {
			ttsElapsed := time.Since(ttsStart)
			fmt.Printf("✅ [AUDIO] TTS synthesis completed in %v (%d bytes)\n", ttsElapsed, len(audioResponse.Data))
			
			// Send TTS audio to console if callback is set
			if s.ttsOutputCallback != nil {
				fmt.Println("🔊 [AUDIO] Sending TTS audio to console speakers...")
				s.ttsOutputCallback(audioResponse)
			} else {
				fmt.Println("⚠️  [AUDIO] No TTS output callback set - audio will not be played")
			}
		}
	}

	return nil
}

// ProcessUserMessage processes a user message through the LLM with function calling support
func (s *AgentSession) ProcessUserMessage(userMessage string) error {
	s.mu.Lock()
	agent := s.Agent
	llmService := s.LLM
	s.mu.Unlock()

	if agent == nil || llmService == nil {
		return fmt.Errorf("agent or LLM service not configured")
	}

	// Add user message to chat context
	s.AddUserMessage(userMessage)

	// Get current messages for LLM
	messages := s.ChatContext.GetMessages()

	// Convert our function tools to LLM tools format
	llmTools := s.convertToolsToLLMFormat()

	// Create chat options with function tools
	chatOpts := llm.DefaultChatOptions()
	if len(llmTools) > 0 {
		chatOpts.Tools = llmTools
		chatOpts.ToolChoice = "auto" // Let LLM decide when to call functions
	}

	// Call LLM with streaming for better performance
	fmt.Printf("🌊 [PERFORMANCE] Starting streaming LLM completion...\n")
	start := time.Now()
	
	stream, err := llmService.ChatStream(s.Context, messages, chatOpts)
	if err != nil {
		return fmt.Errorf("failed to start LLM streaming: %w", err)
	}
	defer stream.Close()

	// Collect streaming response
	var completion *llm.ChatCompletion
	var fullContent strings.Builder
	var toolCalls []llm.ToolCall
	
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break // End of stream
			}
			return fmt.Errorf("failed to receive LLM stream chunk: %w", err)
		}
		
		// Accumulate content
		if chunk.Delta.Content != "" {
			fullContent.WriteString(chunk.Delta.Content)
			fmt.Printf("🎯 [PERFORMANCE] LLM token: %s", chunk.Delta.Content)
		}
		
		// Handle tool calls
		if len(chunk.Delta.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.Delta.ToolCalls...)
		}
	}
	
	elapsed := time.Since(start)
	fmt.Printf("\n✅ [PERFORMANCE] Streaming LLM completed in %v\n", elapsed)
	
	// Create completion object from streamed content
	completion = &llm.ChatCompletion{
		Message: llm.Message{
			Role:      llm.RoleAssistant,
			Content:   fullContent.String(),
			ToolCalls: toolCalls,
		},
	}

	// Check if LLM wants to call any functions
	if len(completion.Message.ToolCalls) > 0 {
		// First, add the assistant's tool call message to the chat context
		s.ChatContext.AddToolCallMessage(completion.Message.ToolCalls)

		// Execute function calls
		for _, toolCall := range completion.Message.ToolCalls {
			if err := s.executeFunctionCall(toolCall); err != nil {
				// Add error message to conversation
				errorMsg := fmt.Sprintf("Error executing function %s: %v", toolCall.Function.Name, err)
				s.AddAssistantMessage(errorMsg)
				continue
			}
		}

		// Get final response from LLM after function execution with streaming
		fmt.Printf("🌊 [PERFORMANCE] Starting final streaming LLM completion...\n")
		finalStart := time.Now()
		
		finalMessages := s.ChatContext.GetMessages()
		finalStream, err := llmService.ChatStream(s.Context, finalMessages, chatOpts)
		if err != nil {
			return fmt.Errorf("failed to start final LLM streaming: %w", err)
		}
		defer finalStream.Close()

		// Collect final streaming response
		var finalContent strings.Builder
		var finalToolCalls []llm.ToolCall
		
		for {
			chunk, err := finalStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				return fmt.Errorf("failed to receive final LLM stream chunk: %w", err)
			}
			
			if chunk.Delta.Content != "" {
				finalContent.WriteString(chunk.Delta.Content)
				fmt.Printf("🎯 [PERFORMANCE] Final LLM token: %s", chunk.Delta.Content)
			}
			if len(chunk.Delta.ToolCalls) > 0 {
				finalToolCalls = append(finalToolCalls, chunk.Delta.ToolCalls...)
			}
		}
		
		finalElapsed := time.Since(finalStart)
		fmt.Printf("\n✅ [PERFORMANCE] Final streaming LLM completed in %v\n", finalElapsed)
		
		finalCompletion := &llm.ChatCompletion{
			Message: llm.Message{
				Role:      llm.RoleAssistant,
				Content:   finalContent.String(),
				ToolCalls: finalToolCalls,
			},
		}

		// Handle final response - could have content and/or additional tool calls
		if finalCompletion.Message.Content != "" {
			s.AddAssistantMessage(finalCompletion.Message.Content)
		}
		if len(finalCompletion.Message.ToolCalls) > 0 {
			// If the final response also contains tool calls, add them
			s.ChatContext.AddToolCallMessage(finalCompletion.Message.ToolCalls)
		}
	} else {
		// No function calls needed, just add the response
		s.AddAssistantMessage(completion.Message.Content)
	}

	// Trigger OnUserTurnCompleted event
	if agent != nil {
		userMsg := &llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: userMessage,
		}
		if err := agent.OnUserTurnCompleted(s.Context, s.ChatContext, userMsg); err != nil {
			fmt.Printf("Warning: OnUserTurnCompleted failed: %v\n", err)
		}
	}

	return nil
}

// ProcessUserMessageWithResponse processes a user message and returns the assistant response (for ChatCLI)
func (s *AgentSession) ProcessUserMessageWithResponse(userMessage *llm.ChatMessage) (*llm.ChatMessage, error) {
	s.mu.Lock()
	agent := s.Agent
	llmService := s.LLM
	s.mu.Unlock()

	if agent == nil || llmService == nil {
		return nil, fmt.Errorf("agent or LLM service not configured")
	}

	// Add user message to chat context
	s.ChatContext.AddMessage(userMessage.Role, userMessage.Content)

	// Get current messages for LLM
	messages := s.ChatContext.GetMessages()

	// Convert our function tools to LLM tools format
	llmTools := s.convertToolsToLLMFormat()

	// Create chat options with function tools
	chatOpts := llm.DefaultChatOptions()
	if len(llmTools) > 0 {
		chatOpts.Tools = llmTools
		chatOpts.ToolChoice = "auto" // Let LLM decide when to call functions
	}

	// Call LLM with streaming for better performance
	fmt.Printf("🌊 [PERFORMANCE] ChatCLI: Starting streaming LLM completion...\n")
	start := time.Now()
	
	stream, err := llmService.ChatStream(s.Context, messages, chatOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to start LLM streaming: %w", err)
	}
	defer stream.Close()

	// Collect streaming response
	var completion *llm.ChatCompletion
	var fullContent strings.Builder
	var toolCalls []llm.ToolCall
	
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break // End of stream
			}
			return nil, fmt.Errorf("failed to receive LLM stream chunk: %w", err)
		}
		
		// Accumulate content
		if chunk.Delta.Content != "" {
			fullContent.WriteString(chunk.Delta.Content)
			fmt.Printf("🎯 [PERFORMANCE] ChatCLI LLM token: %s", chunk.Delta.Content)
		}
		
		// Handle tool calls
		if len(chunk.Delta.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.Delta.ToolCalls...)
		}
	}
	
	elapsed := time.Since(start)
	fmt.Printf("\n✅ [PERFORMANCE] ChatCLI streaming LLM completed in %v\n", elapsed)
	
	// Create completion object from streamed content
	completion = &llm.ChatCompletion{
		Message: llm.Message{
			Role:      llm.RoleAssistant,
			Content:   fullContent.String(),
			ToolCalls: toolCalls,
		},
	}

	var finalResponse string

	// Check if LLM wants to call any functions
	if len(completion.Message.ToolCalls) > 0 {
		// First, add the assistant's tool call message to the chat context
		s.ChatContext.AddToolCallMessage(completion.Message.ToolCalls)

		// Execute function calls
		for _, toolCall := range completion.Message.ToolCalls {
			if err := s.executeFunctionCall(toolCall); err != nil {
				// Add error message to conversation
				errorMsg := fmt.Sprintf("Error executing function %s: %v", toolCall.Function.Name, err)
				s.AddAssistantMessage(errorMsg)
				continue
			}
		}

		// Get final response from LLM after function execution with streaming
		fmt.Printf("🌊 [PERFORMANCE] ChatCLI: Starting final streaming LLM completion...\n")
		finalStart := time.Now()
		
		finalMessages := s.ChatContext.GetMessages()
		finalStream, err := llmService.ChatStream(s.Context, finalMessages, chatOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to start final LLM streaming: %w", err)
		}
		defer finalStream.Close()

		// Collect final streaming response
		var finalContent strings.Builder
		
		for {
			chunk, err := finalStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				return nil, fmt.Errorf("failed to receive final LLM stream chunk: %w", err)
			}
			
			if chunk.Delta.Content != "" {
				finalContent.WriteString(chunk.Delta.Content)
				fmt.Printf("🎯 [PERFORMANCE] ChatCLI final LLM token: %s", chunk.Delta.Content)
			}
		}
		
		finalElapsed := time.Since(finalStart)
		fmt.Printf("\n✅ [PERFORMANCE] ChatCLI final streaming LLM completed in %v\n", finalElapsed)
		
		finalResponse = finalContent.String()
		
		// Add final response to chat context
		if finalResponse != "" {
			s.AddAssistantMessage(finalResponse)
		}
	} else {
		// No function calls needed, just use the response
		finalResponse = completion.Message.Content
		s.AddAssistantMessage(finalResponse)
	}

	// Trigger OnUserTurnCompleted event
	if agent != nil {
		if err := agent.OnUserTurnCompleted(s.Context, s.ChatContext, userMessage); err != nil {
			fmt.Printf("Warning: OnUserTurnCompleted failed: %v\n", err)
		}
	}

	// Return the assistant response
	return &llm.ChatMessage{
		Role:    llm.RoleAssistant,
		Content: finalResponse,
	}, nil
}

// convertToolsToLLMFormat converts our function tools to LLM Tool format
func (s *AgentSession) convertToolsToLLMFormat() []llm.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ToolRegistry == nil {
		return nil
	}

	availableTools := s.ToolRegistry.List()
	llmTools := make([]llm.Tool, 0, len(availableTools))

	for _, tool := range availableTools {
		// Filter out lifecycle methods and utility functions
		if s.isValidFunctionTool(tool.Name()) {
			llmTool := llm.Tool{
				Type: "function",
				Function: llm.ToolFunc{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  tool.Schema(),
				},
			}
			llmTools = append(llmTools, llmTool)
		}
	}

	return llmTools
}

// isValidFunctionTool checks if a tool should be exposed to the LLM
func (s *AgentSession) isValidFunctionTool(toolName string) bool {
	// Filter out lifecycle and utility methods
	excludedTools := []string{
		"name", "get_chat_context", "get_instructions",
		"update_instructions", "update_chat_context",
	}

	for _, excluded := range excludedTools {
		if toolName == excluded {
			return false
		}
	}
	return true
}

// executeFunctionCall executes a function call and adds the result to chat context
func (s *AgentSession) executeFunctionCall(toolCall llm.ToolCall) error {
	s.mu.RLock()
	toolRegistry := s.ToolRegistry
	s.mu.RUnlock()

	if toolRegistry == nil {
		return fmt.Errorf("tool registry not configured")
	}

	// Look up the function tool
	tool, exists := toolRegistry.Lookup(toolCall.Function.Name)
	if !exists {
		return fmt.Errorf("function %s not found", toolCall.Function.Name)
	}

	// Execute the function call
	result, err := tool.Call(s.Context, []byte(toolCall.Function.Arguments))
	if err != nil {
		return fmt.Errorf("function execution failed: %w", err)
	}

	// Add function result (tool call message already added by ProcessUserMessage)
	s.ChatContext.AddToolResultMessage(string(result), toolCall.ID, toolCall.Function.Name)

	return nil
}

// ProcessAudioFrame processes a real audio frame from console audio I/O
func (s *AgentSession) ProcessAudioFrame(frame *media.AudioFrame) error {
	s.mu.RLock()
	vadService := s.VAD
	s.mu.RUnlock()

	if vadService == nil {
		return fmt.Errorf("VAD service not available")
	}

	// Forward frame to agent if available
	if s.Agent != nil {
		s.Agent.OnAudioFrame(frame)
	}

	// Resample audio to VAD's required sample rate (typically 16kHz)
	// Most VAD services expect 16kHz audio, but input audio is often 48kHz
	vadFrame := frame
	if frame.Format.SampleRate != 16000 {
		resampledFrame, err := media.ResampleAudioFrame(frame, 16000)
		if err != nil {
			return fmt.Errorf("failed to resample audio for VAD: %w", err)
		}
		vadFrame = resampledFrame
	}

	// Use background context for VAD to avoid cancellation issues
	// VAD should continue working even if session context is done
	vadResult, err := vadService.Detect(context.Background(), vadFrame)
	if err != nil {
		// Only return error if it's not a context cancellation
		if err != context.Canceled && err != context.DeadlineExceeded {
			return fmt.Errorf("VAD error: %w", err)
		}
		// Log but don't fail on context cancellation
		return nil
	}

	// Forward speech detection events to agent
	if s.Agent != nil {
		if vadResult.IsSpeech {
			s.Agent.OnSpeechDetected(vadResult.Probability)
		} else {
			s.Agent.OnSpeechEnded()
		}
	}

	return nil
}

// ProcessSpeechFrames processes accumulated speech frames through STT -> LLM -> TTS pipeline
func (s *AgentSession) ProcessSpeechFrames(speechFrames []*media.AudioFrame) (*media.AudioFrame, error) {
	s.mu.RLock()
	sttService := s.STT
	llmService := s.LLM
	ttsService := s.TTS
	s.mu.RUnlock()

	if sttService == nil {
		return nil, fmt.Errorf("STT service not available")
	}
	if llmService == nil {
		return nil, fmt.Errorf("LLM service not available")
	}
	if ttsService == nil {
		return nil, fmt.Errorf("TTS service not available")
	}

	// Combine audio frames for STT processing
	combinedAudio := s.combineAudioFrames(speechFrames)
	if combinedAudio == nil {
		return nil, fmt.Errorf("no audio data to process")
	}

	// Speech-to-Text - use background context to avoid cancellation
	recognition, err := sttService.Recognize(context.Background(), combinedAudio)
	if err != nil {
		return nil, fmt.Errorf("STT error: %w", err)
	}

	if recognition.Text == "" {
		return nil, fmt.Errorf("no text recognized")
	}

	fmt.Printf("📝 Recognized: \"%s\" (confidence: %.2f)\n", recognition.Text, recognition.Confidence)

	// Process message through LLM with function calling
	if err := s.ProcessUserMessage(recognition.Text); err != nil {
		return nil, fmt.Errorf("LLM processing error: %w", err)
	}

	// Get the assistant's response
	messages := s.ChatContext.GetMessages()
	if len(messages) == 0 {
		return nil, fmt.Errorf("no LLM response generated")
	}

	lastMessage := messages[len(messages)-1]
	if lastMessage.Role != llm.RoleAssistant {
		return nil, fmt.Errorf("expected assistant response, got %s", lastMessage.Role)
	}

	responseText := lastMessage.Content
	fmt.Printf("💬 Response: \"%s\"\n", responseText)

	// Text-to-Speech with streaming for better performance
	fmt.Printf("🌊 [PERFORMANCE] Starting streaming TTS synthesis...\n")
	start := time.Now()
	
	// Create streaming TTS session
	stream, err := ttsService.SynthesizeStream(context.Background(), &tts.SynthesizeOptions{
		Voice: "alloy", // Default voice
		Speed: 1.0,     // Normal speed
	})
	if err != nil {
		return nil, fmt.Errorf("TTS streaming error: %w", err)
	}
	defer stream.Close()

	// Send text to stream
	if err := stream.SendText(responseText); err != nil {
		return nil, fmt.Errorf("TTS stream send error: %w", err)
	}

	// Signal end of text
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("TTS stream close send error: %w", err)
	}

	// Collect streaming audio chunks into a single response
	var audioChunks []*media.AudioFrame
	var totalBytes int
	
	for {
		audioFrame, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break // End of stream
			}
			return nil, fmt.Errorf("TTS stream receive error: %w", err)
		}
		
		if audioFrame != nil {
			audioChunks = append(audioChunks, audioFrame)
			totalBytes += len(audioFrame.Data)
			fmt.Printf("🎵 [PERFORMANCE] Received TTS chunk: %d bytes (total: %d bytes)\n", len(audioFrame.Data), totalBytes)
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("✅ [PERFORMANCE] Streaming TTS completed in %v (%d chunks, %d total bytes)\n", elapsed, len(audioChunks), totalBytes)

	// Combine chunks into single audio frame for backward compatibility
	if len(audioChunks) == 0 {
		return nil, fmt.Errorf("no audio chunks received from TTS stream")
	}

	audioResponse := s.combineAudioFrames(audioChunks)
	if audioResponse == nil {
		return nil, fmt.Errorf("failed to combine TTS audio chunks")
	}

	return audioResponse, nil
}

// combineAudioFrames combines multiple audio frames into a single frame
func (s *AgentSession) combineAudioFrames(frames []*media.AudioFrame) *media.AudioFrame {
	if len(frames) == 0 {
		return nil
	}

	// Calculate total data size
	totalSize := 0
	for _, frame := range frames {
		totalSize += len(frame.Data)
	}

	if totalSize == 0 {
		return nil
	}

	// Combine all frame data
	combinedData := make([]byte, 0, totalSize)
	for _, frame := range frames {
		combinedData = append(combinedData, frame.Data...)
	}

	// Create combined frame with same format as first frame
	return media.NewAudioFrame(combinedData, frames[0].Format)
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
