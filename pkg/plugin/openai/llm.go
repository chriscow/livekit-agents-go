package openai

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAILLM implements the LLM interface using OpenAI GPT models
type OpenAILLM struct {
	client *openai.Client
	model  string
}

// newOpenAILLM creates a new OpenAI LLM instance
func newOpenAILLM(config map[string]any) (any, error) {
	var apiKey string
	
	// Get API key from config or environment
	if key, ok := config["api_key"].(string); ok {
		apiKey = key
	} else {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required (set OPENAI_API_KEY environment variable or provide api_key in config)")
	}
	
	model, ok := config["model"].(string)
	if !ok || model == "" {
		model = "gpt-3.5-turbo" // default model
	}
	
	return &OpenAILLM{
		client: openai.NewClient(apiKey),
		model:  model,
	}, nil
}

// Chat performs chat completion with conversation history
func (o *OpenAILLM) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	log.Printf("🤖 Starting OpenAI chat completion with %d messages (model: %s)", len(req.Messages), o.model)
	start := time.Now()

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
			Name:    msg.Name,
		}
	}

	// Convert functions to OpenAI tools format
	var tools []openai.Tool
	if len(req.Functions) > 0 {
		tools = make([]openai.Tool, len(req.Functions))
		for i, fn := range req.Functions {
			tools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        fn.Name,
					Description: fn.Description,
					Parameters:  fn.Parameters,
				},
			}
		}
	}

	completionReq := openai.ChatCompletionRequest{
		Model:       o.model,
		Messages:    openaiMessages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       tools,
	}

	resp, err := o.client.CreateChatCompletion(ctx, completionReq)
	if err != nil {
		log.Printf("❌ OpenAI chat completion failed: %v", err)
		return llm.ChatResponse{}, fmt.Errorf("chat completion request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		log.Printf("❌ OpenAI returned no completion choices")
		return llm.ChatResponse{}, fmt.Errorf("no chat completion choices returned")
	}

	choice := resp.Choices[0]
	duration := time.Since(start)

	result := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.MessageRole(choice.Message.Role),
			Content: choice.Message.Content,
		},
		TokensUsed:   resp.Usage.TotalTokens,
		FinishReason: string(choice.FinishReason),
	}

	// Handle function calls - convert first tool call to function call
	if len(choice.Message.ToolCalls) > 0 {
		toolCall := choice.Message.ToolCalls[0]
		result.FunctionCall = &llm.FunctionCall{
			Name:      toolCall.Function.Name,
			Arguments: toolCall.Function.Arguments,
		}
	}

	log.Printf("✅ OpenAI chat completion successful: '%s' (tokens: %d, duration: %v)",
		choice.Message.Content, resp.Usage.TotalTokens, duration)

	return result, nil
}

// Capabilities returns the OpenAI provider's capabilities
func (o *OpenAILLM) Capabilities() llm.LLMCapabilities {
	return llm.LLMCapabilities{
		SupportsFunctions:   true,
		SupportsStreaming:   false, // Not implementing streaming yet
		MaxTokens:          128000, // GPT-4 context length
		SupportedModels:    []string{"gpt-3.5-turbo", "gpt-4", "gpt-4-turbo", "gpt-4o"},
		SupportsSystemRole: true,
	}
}