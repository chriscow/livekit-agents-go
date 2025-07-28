package fake

import (
	"context"
	"fmt"
	"strings"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

// FakeLLM is a fake LLM implementation for testing.
type FakeLLM struct {
	responses []string
	callCount int
}

// NewFakeLLM creates a new fake LLM provider with predefined responses.
func NewFakeLLM(responses ...string) *FakeLLM {
	if len(responses) == 0 {
		responses = []string{
			"This is a fake response from the fake LLM provider.",
			"I'm a fake AI assistant. How can I help you?",
			"This is another fake response for testing purposes.",
		}
	}
	return &FakeLLM{responses: responses}
}

// Chat processes a chat request and returns a fake response.
func (f *FakeLLM) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	// Simple response selection based on call count
	responseIndex := f.callCount % len(f.responses)
	response := f.responses[responseIndex]
	f.callCount++
	
	// If the user mentions a function, return a fake function call
	if len(req.Functions) > 0 {
		for _, msg := range req.Messages {
			if msg.Role == llm.RoleUser && strings.Contains(strings.ToLower(msg.Content), "function") {
				return llm.ChatResponse{
					Message: llm.Message{
						Role:    llm.RoleAssistant,
						Content: "",
					},
					FunctionCall: &llm.FunctionCall{
						Name:      req.Functions[0].Name,
						Arguments: `{"param": "fake_value"}`,
					},
					TokensUsed:   50,
					FinishReason: "function_call",
				}, nil
			}
		}
	}
	
	// Add some context from the user's message if available
	if len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1]
		if lastMsg.Role == llm.RoleUser {
			response = fmt.Sprintf("%s (You said: %s)", response, lastMsg.Content)
		}
	}
	
	return llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: response,
		},
		TokensUsed:   len(strings.Fields(response)) + 10,
		FinishReason: "stop",
	}, nil
}

// Capabilities returns the fake LLM capabilities.
func (f *FakeLLM) Capabilities() llm.LLMCapabilities {
	return llm.LLMCapabilities{
		SupportsFunctions:   true,
		SupportsStreaming:   false,
		MaxTokens:          4096,
		SupportedModels:    []string{"fake-model-1", "fake-model-2"},
		SupportsSystemRole: true,
	}
}