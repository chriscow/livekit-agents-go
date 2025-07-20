package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"livekit-agents-go/services/llm"
)

// MockLLM implements the LLM interface for testing
type MockLLM struct {
	*llm.BaseLLM
	responses     []string
	responseIndex int
	delay         time.Duration
	tokenCount    int
}

// NewMockLLM creates a new mock LLM service
func NewMockLLM(responses ...string) *MockLLM {
	if len(responses) == 0 {
		responses = []string{
			"Hello! How can I help you today?",
			"That's interesting! Tell me more.",
			"I understand. Is there anything else you'd like to know?",
			"Thank you for asking. Let me think about that...",
			"That's a great question! Here's what I think:",
		}
	}

	return &MockLLM{
		BaseLLM:       llm.NewBaseLLM("mock-llm", "1.0.0"),
		responses:     responses,
		responseIndex: 0,
		delay:         500 * time.Millisecond,
		tokenCount:    50,
	}
}

// SetDelay sets the mock delay for responses
func (m *MockLLM) SetDelay(delay time.Duration) {
	m.delay = delay
}

// SetTokenCount sets the mock token count for usage reporting
func (m *MockLLM) SetTokenCount(count int) {
	m.tokenCount = count
}

// AddResponse adds a response to the mock
func (m *MockLLM) AddResponse(response string) {
	m.responses = append(m.responses, response)
}

// Complete implements llm.LLM
func (m *MockLLM) Complete(ctx context.Context, prompt string, opts *llm.CompletionOptions) (*llm.Completion, error) {
	// Simulate processing delay
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Get next response
	text := m.responses[m.responseIndex%len(m.responses)]
	m.responseIndex++

	// Create smart response based on input
	if strings.Contains(strings.ToLower(prompt), "hello") || strings.Contains(strings.ToLower(prompt), "hi") {
		text = "Hello! How can I help you today?"
	} else if strings.Contains(strings.ToLower(prompt), "weather") {
		text = "I don't have access to real weather data, but I can help you with other questions!"
	} else if strings.Contains(strings.ToLower(prompt), "thank") {
		text = "You're welcome! Is there anything else I can help you with?"
	}

	promptTokens := len(strings.Fields(prompt))
	completionTokens := len(strings.Fields(text))

	return &llm.Completion{
		Text:         text,
		FinishReason: "stop",
		Usage: llm.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
		Metadata: map[string]interface{}{
			"mock":           true,
			"response_index": m.responseIndex - 1,
		},
	}, nil
}

// Chat implements llm.LLM
func (m *MockLLM) Chat(ctx context.Context, messages []llm.Message, opts *llm.ChatOptions) (*llm.ChatCompletion, error) {
	// Simulate processing delay
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Get the last user message for context
	var lastUserMessage string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			lastUserMessage = messages[i].Content
			break
		}
	}

	// Generate contextual response
	var responseText string
	lowerMsg := strings.ToLower(lastUserMessage)

	switch {
	case strings.Contains(lowerMsg, "hello") || strings.Contains(lowerMsg, "hi"):
		responseText = "Hello! How can I help you today?"
	case strings.Contains(lowerMsg, "weather"):
		responseText = "I don't have access to real weather data, but I can help you with other questions!"
	case strings.Contains(lowerMsg, "time"):
		responseText = "I don't have access to the current time, but I can help you with other questions!"
	case strings.Contains(lowerMsg, "name"):
		responseText = "I'm MockLLM, a test assistant. What's your name?"
	case strings.Contains(lowerMsg, "thank"):
		responseText = "You're welcome! Is there anything else I can help you with?"
	case strings.Contains(lowerMsg, "test"):
		responseText = "Yes, this is a test response from the mock LLM service. Everything seems to be working!"
	case lastUserMessage == "":
		responseText = "I didn't receive any message. Could you please try again?"
	default:
		// Use predefined response
		responseText = m.responses[m.responseIndex%len(m.responses)]
		m.responseIndex++
	}

	// Calculate token usage
	promptTokens := 0
	for _, msg := range messages {
		promptTokens += len(strings.Fields(msg.Content))
	}
	completionTokens := len(strings.Fields(responseText))

	return &llm.ChatCompletion{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: responseText,
		},
		FinishReason: "stop",
		Usage: llm.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
		Metadata: map[string]interface{}{
			"mock":              true,
			"conversation_length": len(messages),
			"last_user_message":   lastUserMessage,
		},
	}, nil
}

// ChatStream implements llm.LLM
func (m *MockLLM) ChatStream(ctx context.Context, messages []llm.Message, opts *llm.ChatOptions) (llm.ChatStream, error) {
	return NewMockChatStream(m, messages), nil
}

// MockChatStream implements llm.ChatStream
type MockChatStream struct {
	llm      *MockLLM
	messages []llm.Message
	chunks   chan *llm.ChatCompletionChunk
	closed   bool
	mu       sync.Mutex
}

// NewMockChatStream creates a new mock chat stream
func NewMockChatStream(mockLLM *MockLLM, messages []llm.Message) *MockChatStream {
	stream := &MockChatStream{
		llm:      mockLLM,
		messages: messages,
		chunks:   make(chan *llm.ChatCompletionChunk, 10),
		closed:   false,
	}

	// Start streaming response
	go stream.generateResponse()

	return stream
}

// Recv implements llm.ChatStream
func (s *MockChatStream) Recv() (*llm.ChatCompletionChunk, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	
	if closed {
		return nil, fmt.Errorf("stream is closed")
	}

	chunk, ok := <-s.chunks
	if !ok {
		return nil, fmt.Errorf("stream is closed")
	}

	return chunk, nil
}

// Close implements llm.ChatStream
func (s *MockChatStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}

	s.closed = true
	return nil
}

// generateResponse generates streaming response chunks
func (s *MockChatStream) generateResponse() {
	defer close(s.chunks)

	// Get last user message for context
	var lastUserMessage string
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].Role == llm.RoleUser {
			lastUserMessage = s.messages[i].Content
			break
		}
	}

	// Generate response based on context
	var fullResponse string
	lowerMsg := strings.ToLower(lastUserMessage)

	switch {
	case strings.Contains(lowerMsg, "hello") || strings.Contains(lowerMsg, "hi"):
		fullResponse = "Hello! How can I help you today?"
	case strings.Contains(lowerMsg, "stream"):
		fullResponse = "This is a streaming response from the mock LLM. Each word is sent as a separate chunk!"
	default:
		fullResponse = s.llm.responses[s.llm.responseIndex%len(s.llm.responses)]
		s.llm.responseIndex++
	}

	// Stream response word by word
	words := strings.Fields(fullResponse)
	for i, word := range words {
		// Add space before word (except first)
		content := word
		if i > 0 {
			content = " " + word
		}

		chunk := &llm.ChatCompletionChunk{
			Delta: llm.MessageDelta{
				Role:    llm.RoleAssistant,
				Content: content,
			},
			Metadata: map[string]interface{}{
				"mock":       true,
				"chunk_index": i,
				"total_chunks": len(words),
			},
		}

		// Check if stream is closed before sending
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
		
		select {
		case s.chunks <- chunk:
			// Small delay between chunks to simulate streaming
			time.Sleep(50 * time.Millisecond)
		default:
			// Channel full, skip chunk
			return
		}
	}

	// Send final chunk with finish reason
	finalChunk := &llm.ChatCompletionChunk{
		Delta:        llm.MessageDelta{},
		FinishReason: "stop",
		Usage: &llm.Usage{
			PromptTokens:     len(strings.Fields(lastUserMessage)),
			CompletionTokens: len(words),
			TotalTokens:      len(strings.Fields(lastUserMessage)) + len(words),
		},
		Metadata: map[string]interface{}{
			"mock":  true,
			"final": true,
		},
	}

	// Check if stream is closed before sending final chunk
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	
	select {
	case s.chunks <- finalChunk:
	default:
		// Channel full, skip final chunk
	}
}