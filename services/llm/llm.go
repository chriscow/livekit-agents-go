package llm

import (
	"context"
	"io"
	"time"
)

// LLM defines the Large Language Model service interface
type LLM interface {
	// Complete generates text completion
	Complete(ctx context.Context, prompt string, opts *CompletionOptions) (*Completion, error)
	
	// Chat performs chat completion with conversation history
	Chat(ctx context.Context, messages []Message, opts *ChatOptions) (*ChatCompletion, error)
	
	// ChatStream creates a streaming chat session
	ChatStream(ctx context.Context, messages []Message, opts *ChatOptions) (ChatStream, error)
	
	// Service metadata
	Name() string
	Version() string
}

// Message represents a chat message
type Message struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
	Name    string      `json:"name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
}

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleFunction  MessageRole = "function"
	RoleTool      MessageRole = "tool"
)

// ToolCall represents a function tool call
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function call
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// CompletionOptions configures text completion
type CompletionOptions struct {
	Model           string
	MaxTokens       int
	Temperature     float64
	TopP            float64
	FrequencyPenalty float64
	PresencePenalty  float64
	Stop            []string
	Stream          bool
	Metadata        map[string]interface{}
}

// ChatOptions configures chat completion
type ChatOptions struct {
	Model           string
	MaxTokens       int
	Temperature     float64
	TopP            float64
	FrequencyPenalty float64
	PresencePenalty  float64
	Stop            []string
	Stream          bool
	Tools           []Tool
	ToolChoice      interface{}
	Metadata        map[string]interface{}
}

// Tool represents a function tool available to the LLM
type Tool struct {
	Type     string   `json:"type"`
	Function ToolFunc `json:"function"`
}

// ToolFunc defines a function tool
type ToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// Completion represents a text completion result
type Completion struct {
	Text         string
	FinishReason string
	Usage        Usage
	Metadata     map[string]interface{}
}

// ChatCompletion represents a chat completion result
type ChatCompletion struct {
	Message      Message
	FinishReason string
	Usage        Usage
	Metadata     map[string]interface{}
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ChatStream represents a streaming chat session
type ChatStream interface {
	// Recv receives chat completion chunks from the stream
	Recv() (*ChatCompletionChunk, error)
	
	// Close closes the chat stream
	Close() error
}

// ChatCompletionChunk represents a chunk in streaming chat completion
type ChatCompletionChunk struct {
	Delta        MessageDelta
	FinishReason string
	Usage        *Usage
	Metadata     map[string]interface{}
}

// MessageDelta represents incremental message content
type MessageDelta struct {
	Role      MessageRole `json:"role,omitempty"`
	Content   string      `json:"content,omitempty"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
}

// StreamingLLM extends LLM for services that support advanced streaming
type StreamingLLM interface {
	LLM
	
	// Stream chat with advanced options
	Stream(ctx context.Context, messages []Message, opts *StreamChatOptions) (ChatStream, error)
}

// StreamChatOptions configures streaming chat
type StreamChatOptions struct {
	Model             string
	MaxTokens         int
	Temperature       float64
	TopP              float64
	FrequencyPenalty  float64
	PresencePenalty   float64
	Stop              []string
	Tools             []Tool
	ToolChoice        interface{}
	StreamOptions     *StreamOptions
	Metadata          map[string]interface{}
}

// StreamOptions configures streaming behavior
type StreamOptions struct {
	IncludeUsage bool
	ChunkSize    int
	BufferSize   int
}

// FunctionCallingLLM extends LLM for services that support function calling
type FunctionCallingLLM interface {
	LLM
	
	// CallFunction executes a function call
	CallFunction(ctx context.Context, messages []Message, tools []Tool, opts *FunctionCallOptions) (*FunctionCallResult, error)
}

// FunctionCallOptions configures function calling
type FunctionCallOptions struct {
	Model       string
	MaxTokens   int
	Temperature float64
	ToolChoice  interface{}
	Metadata    map[string]interface{}
}

// FunctionCallResult represents the result of a function call
type FunctionCallResult struct {
	ToolCalls    []ToolCall
	Message      Message
	FinishReason string
	Usage        Usage
	Metadata     map[string]interface{}
}

// BaseLLM provides common functionality for LLM implementations
type BaseLLM struct {
	name    string
	version string
}

// NewBaseLLM creates a new base LLM service
func NewBaseLLM(name, version string) *BaseLLM {
	return &BaseLLM{
		name:    name,
		version: version,
	}
}

func (b *BaseLLM) Name() string {
	return b.name
}

func (b *BaseLLM) Version() string {
	return b.version
}

// ChatStreamReader provides a reader interface for streaming chat
type ChatStreamReader struct {
	stream ChatStream
}

// NewChatStreamReader creates a new stream reader
func NewChatStreamReader(stream ChatStream) *ChatStreamReader {
	return &ChatStreamReader{stream: stream}
}

// Read implements io.Reader for chat completion chunks
func (r *ChatStreamReader) Read(p []byte) (n int, err error) {
	chunk, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	
	if chunk == nil {
		return 0, io.EOF
	}
	
	data := []byte(chunk.Delta.Content)
	n = copy(p, data)
	
	if n < len(data) {
		return n, io.ErrShortBuffer
	}
	
	return n, nil
}

// Close closes the stream reader
func (r *ChatStreamReader) Close() error {
	return r.stream.Close()
}

// DefaultCompletionOptions returns default completion options
func DefaultCompletionOptions() *CompletionOptions {
	return &CompletionOptions{
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        1.0,
		Metadata:    make(map[string]interface{}),
	}
}

// DefaultChatOptions returns default chat options
func DefaultChatOptions() *ChatOptions {
	return &ChatOptions{
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        1.0,
		Metadata:    make(map[string]interface{}),
	}
}

// ChatContext represents the conversation context (Python framework equivalent)
type ChatContext struct {
	Messages    []ChatMessage          `json:"messages"`
	SystemPrompt string                `json:"system_prompt,omitempty"`
	Tools       []Tool                 `json:"tools,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ChatMessage represents a message in the chat context (Python framework equivalent)
type ChatMessage struct {
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	Name      string      `json:"name,omitempty"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// NewChatContext creates a new chat context
func NewChatContext() *ChatContext {
	return &ChatContext{
		Messages: make([]ChatMessage, 0),
		Metadata: make(map[string]interface{}),
	}
}

// NewChatContextWithSystem creates a chat context with system prompt
func NewChatContextWithSystem(systemPrompt string) *ChatContext {
	ctx := NewChatContext()
	ctx.SystemPrompt = systemPrompt
	if systemPrompt != "" {
		ctx.Messages = append(ctx.Messages, ChatMessage{
			Role:    RoleSystem,
			Content: systemPrompt,
		})
	}
	return ctx
}

// AddMessage adds a message to the chat context
func (c *ChatContext) AddMessage(role MessageRole, content string) {
	c.Messages = append(c.Messages, ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Unix(),
	})
}

// AddUserMessage adds a user message to the chat context
func (c *ChatContext) AddUserMessage(content string) {
	c.AddMessage(RoleUser, content)
}

// AddAssistantMessage adds an assistant message to the chat context
func (c *ChatContext) AddAssistantMessage(content string) {
	c.AddMessage(RoleAssistant, content)
}

// AddToolCallMessage adds a message with tool calls to the chat context
func (c *ChatContext) AddToolCallMessage(toolCalls []ToolCall) {
	c.Messages = append(c.Messages, ChatMessage{
		Role:      RoleAssistant,
		Content:   "",
		ToolCalls: toolCalls,
		Timestamp: time.Now().Unix(),
	})
}

// AddToolResultMessage adds a tool result message to the chat context
func (c *ChatContext) AddToolResultMessage(content, toolCallID, toolName string) {
	c.Messages = append(c.Messages, ChatMessage{
		Role:       RoleFunction,
		Content:    content,
		Name:       toolName,        // OpenAI requires 'name' field for function messages
		ToolCallID: toolCallID,
		Timestamp:  time.Now().Unix(),
	})
}

// GetMessages returns messages in the format expected by LLM services
func (c *ChatContext) GetMessages() []Message {
	messages := make([]Message, len(c.Messages))
	for i, msg := range c.Messages {
		messages[i] = Message{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		}
	}
	return messages
}

// Clear clears all messages except system prompt
func (c *ChatContext) Clear() {
	systemPrompt := c.SystemPrompt
	c.Messages = make([]ChatMessage, 0)
	if systemPrompt != "" {
		c.AddMessage(RoleSystem, systemPrompt)
	}
}