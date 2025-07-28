package llm

import (
	"context"

	"github.com/chriscow/livekit-agents-go/pkg/ai"
)

// LLM-specific error variables for backward compatibility
var (
	// ErrRecoverable indicates a temporary LLM failure that may succeed if retried.
	// Examples: rate limiting, temporary service error, timeout.
	ErrRecoverable = ai.ErrRecoverable
	
	// ErrFatal indicates a permanent LLM failure that will not succeed if retried.
	// Examples: invalid API key, unsupported model, content policy violation.
	ErrFatal = ai.ErrFatal
)

// MessageRole represents the role of a message in a chat conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleFunction  MessageRole = "function"
)

// Message represents a single message in a chat conversation.
type Message struct {
	Role    MessageRole
	Content string
	Name    string // for function messages
}

// FunctionCall represents a function call request from the LLM.
type FunctionCall struct {
	Name      string
	Arguments string // JSON-encoded arguments
}

// ChatRequest contains parameters for a chat completion request.
type ChatRequest struct {
	Messages    []Message
	MaxTokens   int
	Temperature float32
	TopP        float32
	Functions   []FunctionDefinition
}

// ChatResponse contains the response from a chat completion request.
type ChatResponse struct {
	Message      Message
	FunctionCall *FunctionCall
	TokensUsed   int
	FinishReason string
}

// FunctionDefinition defines a function that the LLM can call.
type FunctionDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON schema
}

// LLMCapabilities describes the capabilities of an LLM provider.
type LLMCapabilities struct {
	SupportsFunctions   bool
	SupportsStreaming   bool
	MaxTokens          int
	SupportedModels    []string
	SupportsSystemRole bool
}

// LLM is the main interface for large language model providers.
type LLM interface {
	// Chat performs a chat completion request.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	
	// Capabilities returns the provider's capabilities.
	Capabilities() LLMCapabilities
}