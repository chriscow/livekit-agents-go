package openai

import (
	"context"
	"fmt"
	"io"

	"livekit-agents-go/services/llm"

	openai "github.com/sashabaranov/go-openai"
)

// GPTLLM implements the LLM interface using OpenAI GPT models
type GPTLLM struct {
	*llm.BaseLLM
	client *openai.Client
	model  string
}

// NewGPTLLM creates a new GPT LLM service
func NewGPTLLM(apiKey, model string) *GPTLLM {
	return &GPTLLM{
		BaseLLM: llm.NewBaseLLM("gpt", "1.0.0"),
		client:  openai.NewClient(apiKey),
		model:   model,
	}
}

// Complete generates text completion
func (g *GPTLLM) Complete(ctx context.Context, prompt string, opts *llm.CompletionOptions) (*llm.Completion, error) {
	if opts == nil {
		opts = llm.DefaultCompletionOptions()
	}

	req := openai.CompletionRequest{
		Model:            g.model,
		Prompt:           prompt,
		MaxTokens:        opts.MaxTokens,
		Temperature:      float32(opts.Temperature),
		TopP:             float32(opts.TopP),
		FrequencyPenalty: float32(opts.FrequencyPenalty),
		PresencePenalty:  float32(opts.PresencePenalty),
		Stop:             opts.Stop,
	}

	resp, err := g.client.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	choice := resp.Choices[0]
	return &llm.Completion{
		Text:         choice.Text,
		FinishReason: string(choice.FinishReason),
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Metadata: map[string]interface{}{
			"model":    g.model,
			"index":    choice.Index,
			"logprobs": choice.LogProbs,
		},
	}, nil
}

// Chat performs chat completion with conversation history
func (g *GPTLLM) Chat(ctx context.Context, messages []llm.Message, opts *llm.ChatOptions) (*llm.ChatCompletion, error) {
	if opts == nil {
		opts = llm.DefaultChatOptions()
	}

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
			Name:    msg.Name,
		}
	}

	// Convert tools to OpenAI format
	var tools []openai.Tool
	if len(opts.Tools) > 0 {
		tools = make([]openai.Tool, len(opts.Tools))
		for i, tool := range opts.Tools {
			tools[i] = openai.Tool{
				Type: openai.ToolType(tool.Type),
				Function: &openai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
	}

	req := openai.ChatCompletionRequest{
		Model:            g.model,
		Messages:         openaiMessages,
		MaxTokens:        opts.MaxTokens,
		Temperature:      float32(opts.Temperature),
		TopP:             float32(opts.TopP),
		FrequencyPenalty: float32(opts.FrequencyPenalty),
		PresencePenalty:  float32(opts.PresencePenalty),
		Stop:             opts.Stop,
		Tools:            tools,
		ToolChoice:       opts.ToolChoice,
	}

	resp, err := g.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("chat completion request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no chat completion choices returned")
	}

	choice := resp.Choices[0]

	// Convert tool calls back to our format
	var toolCalls []llm.ToolCall
	if len(choice.Message.ToolCalls) > 0 {
		toolCalls = make([]llm.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			toolCalls[i] = llm.ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: llm.Function{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return &llm.ChatCompletion{
		Message: llm.Message{
			Role:      llm.MessageRole(choice.Message.Role),
			Content:   choice.Message.Content,
			ToolCalls: toolCalls,
		},
		FinishReason: string(choice.FinishReason),
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Metadata: map[string]interface{}{
			"model": g.model,
			"index": choice.Index,
		},
	}, nil
}

// ChatStream creates a streaming chat session
func (g *GPTLLM) ChatStream(ctx context.Context, messages []llm.Message, opts *llm.ChatOptions) (llm.ChatStream, error) {
	if opts == nil {
		opts = llm.DefaultChatOptions()
	}

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
			Name:    msg.Name,
		}
	}

	// Convert tools to OpenAI format
	var tools []openai.Tool
	if len(opts.Tools) > 0 {
		tools = make([]openai.Tool, len(opts.Tools))
		for i, tool := range opts.Tools {
			tools[i] = openai.Tool{
				Type: openai.ToolType(tool.Type),
				Function: &openai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
	}

	req := openai.ChatCompletionRequest{
		Model:            g.model,
		Messages:         openaiMessages,
		MaxTokens:        opts.MaxTokens,
		Temperature:      float32(opts.Temperature),
		TopP:             float32(opts.TopP),
		FrequencyPenalty: float32(opts.FrequencyPenalty),
		PresencePenalty:  float32(opts.PresencePenalty),
		Stop:             opts.Stop,
		Tools:            tools,
		ToolChoice:       opts.ToolChoice,
		Stream:           true,
	}

	stream, err := g.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("chat completion stream request failed: %w", err)
	}

	return &GPTChatStream{
		stream: stream,
		closed: false,
	}, nil
}

// GPTChatStream implements streaming chat for GPT models
type GPTChatStream struct {
	stream *openai.ChatCompletionStream
	closed bool
}

// Recv receives chat completion chunks from the stream
func (s *GPTChatStream) Recv() (*llm.ChatCompletionChunk, error) {
	if s.closed {
		return nil, fmt.Errorf("stream is closed")
	}

	resp, err := s.stream.Recv()
	if err != nil {
		if err == io.EOF {
			s.closed = true
			return nil, io.EOF
		}
		return nil, fmt.Errorf("stream receive failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in stream response")
	}

	choice := resp.Choices[0]

	// Convert tool calls back to our format
	var toolCalls []llm.ToolCall
	if len(choice.Delta.ToolCalls) > 0 {
		toolCalls = make([]llm.ToolCall, len(choice.Delta.ToolCalls))
		for i, tc := range choice.Delta.ToolCalls {
			toolCalls[i] = llm.ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
			}
			if tc.Function.Name != "" || tc.Function.Arguments != "" {
				toolCalls[i].Function = llm.Function{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
			}
		}
	}

	chunk := &llm.ChatCompletionChunk{
		Delta: llm.MessageDelta{
			Role:      llm.MessageRole(choice.Delta.Role),
			Content:   choice.Delta.Content,
			ToolCalls: toolCalls,
		},
		FinishReason: string(choice.FinishReason),
		Metadata: map[string]interface{}{
			"model": resp.Model,
			"index": choice.Index,
		},
	}

	// Note: Usage might not be available in streaming responses
	// This would be implementation specific based on OpenAI API version

	return chunk, nil
}

// Close closes the chat stream
func (s *GPTChatStream) Close() error {
	if !s.closed {
		s.stream.Close()
		s.closed = true
	}
	return nil
}
