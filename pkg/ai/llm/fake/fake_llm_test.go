package fake

import (
	"context"
	"strings"
	"testing"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

func TestFakeLLMCapabilities(t *testing.T) {
	provider := NewFakeLLM()
	caps := provider.Capabilities()

	if !caps.SupportsFunctions {
		t.Error("Expected SupportsFunctions to be true")
	}
	
	if caps.MaxTokens <= 0 {
		t.Error("Expected MaxTokens to be positive")
	}
	
	if len(caps.SupportedModels) == 0 {
		t.Error("Expected SupportedModels to be non-empty")
	}
	
	if !caps.SupportsSystemRole {
		t.Error("Expected SupportsSystemRole to be true")
	}
}

func TestFakeLLMChat(t *testing.T) {
	provider := NewFakeLLM("Test response 1", "Test response 2")
	ctx := context.Background()

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Should return one of the predefined responses
	if resp.Message.Role != llm.RoleAssistant {
		t.Errorf("Expected assistant role, got %v", resp.Message.Role)
	}
	
	if !strings.Contains(resp.Message.Content, "Test response") {
		t.Errorf("Expected predefined response, got %q", resp.Message.Content)
	}
	
	if resp.TokensUsed <= 0 {
		t.Error("Expected TokensUsed to be positive")
	}
	
	if resp.FinishReason == "" {
		t.Error("Expected FinishReason to be set")
	}
}

func TestFakeLLMFunctionCall(t *testing.T) {
	provider := NewFakeLLM()
	ctx := context.Background()

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Please call a function"},
		},
		Functions: []llm.FunctionDefinition{
			{
				Name:        "test_function",
				Description: "A test function",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Should return a function call when user mentions "function"
	if resp.FunctionCall == nil {
		t.Fatal("Expected function call response")
	}
	
	if resp.FunctionCall.Name != "test_function" {
		t.Errorf("Expected function name 'test_function', got %q", resp.FunctionCall.Name)
	}
	
	if resp.FunctionCall.Arguments == "" {
		t.Error("Expected function arguments to be set")
	}
	
	if resp.FinishReason != "function_call" {
		t.Errorf("Expected finish reason 'function_call', got %q", resp.FinishReason)
	}
}

func TestFakeLLMResponseCycling(t *testing.T) {
	responses := []string{"Response A", "Response B", "Response C"}
	provider := NewFakeLLM(responses...)
	ctx := context.Background()

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Test"},
		},
	}

	// Make multiple requests and verify cycling
	for i := 0; i < len(responses)*2; i++ {
		resp, err := provider.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Chat() iteration %d error = %v", i, err)
		}
		
		expectedIndex := i % len(responses)
		expectedResponse := responses[expectedIndex]
		
		if !strings.Contains(resp.Message.Content, expectedResponse) {
			t.Errorf("Iteration %d: expected response containing %q, got %q", 
				i, expectedResponse, resp.Message.Content)
		}
	}
}

func TestFakeLLMContextInResponse(t *testing.T) {
	provider := NewFakeLLM("Base response")
	ctx := context.Background()

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "My name is Alice"},
		},
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Should include user's message in the response
	if !strings.Contains(resp.Message.Content, "Alice") {
		t.Errorf("Expected response to include user context, got %q", resp.Message.Content)
	}
}

func TestFakeLLMMultipleMessages(t *testing.T) {
	provider := NewFakeLLM("Response")
	ctx := context.Background()

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a helpful assistant"},
			{Role: llm.RoleUser, Content: "Hello"},
			{Role: llm.RoleAssistant, Content: "Hi there!"},
			{Role: llm.RoleUser, Content: "How are you?"},
		},
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Should handle multiple messages without error
	if resp.Message.Role != llm.RoleAssistant {
		t.Errorf("Expected assistant role, got %v", resp.Message.Role)
	}
	
	// Should include context from last user message
	if !strings.Contains(resp.Message.Content, "How are you?") {
		t.Errorf("Expected response to include last user message, got %q", resp.Message.Content)
	}
}