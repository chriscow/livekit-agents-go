package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/openai"
)

// NewLLMCmd creates the LLM testing command
func NewLLMCmd() *cobra.Command {
	var (
		model     string
		prompt    string
		functions bool
		stream    bool
		maxTokens int
	)

	cmd := &cobra.Command{
		Use:   "llm",
		Short: "Test LLM (Large Language Model) using OpenAI GPT",
		Long: `Test Large Language Model responses using OpenAI GPT.

This command tests the LLM service that generates responses to user input.
LLM is the third step in the voice pipeline after VAD and STT.

Examples:
  pipeline-test llm --prompt "Hello, how are you?"          # Basic text completion
  pipeline-test llm --functions --prompt "What's the weather?" # Test function calling
  pipeline-test llm --stream --prompt "Tell me a story"     # Test streaming responses`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate API key
			if os.Getenv("OPENAI_API_KEY") == "" {
				return fmt.Errorf("OPENAI_API_KEY environment variable required for LLM testing")
			}

			return runLLMTest(model, prompt, functions, stream, maxTokens)
		},
	}

	cmd.Flags().StringVarP(&model, "model", "m", "gpt-4o", "GPT model to use")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "Hello! How can I help you today?", "test prompt")
	cmd.Flags().BoolVarP(&functions, "functions", "f", false, "test function calling capabilities")
	cmd.Flags().BoolVarP(&stream, "stream", "s", false, "test streaming responses")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 150, "maximum tokens in response")

	return cmd
}

// runLLMTest tests the LLM service
func runLLMTest(model, prompt string, functions, stream bool, maxTokens int) error {
	fmt.Printf("🧠 Testing LLM: %s\n", model)
	fmt.Printf("💬 Prompt: %s\n", prompt)
	fmt.Printf("🔧 Function calling: %v\n", functions)
	fmt.Printf("🌊 Streaming: %v\n", stream)
	fmt.Printf("📊 Max tokens: %d\n", maxTokens)
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create LLM service
	fmt.Println("🔧 Creating LLM service...")
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	if services.LLM == nil {
		return fmt.Errorf("LLM service not available - check OPENAI_API_KEY")
	}

	fmt.Printf("✅ LLM service: %s v%s\n", services.LLM.Name(), services.LLM.Version())

	// Create chat messages
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant for testing purposes. Keep responses concise."},
		{Role: llm.RoleUser, Content: prompt},
	}

	if stream {
		return runStreamingLLMTest(ctx, services.LLM, messages, maxTokens)
	} else {
		return runBatchLLMTest(ctx, services.LLM, messages, maxTokens)
	}
}

// runBatchLLMTest tests batch LLM completion
func runBatchLLMTest(ctx context.Context, llmService llm.LLM, messages []llm.Message, maxTokens int) error {
	fmt.Println("🎯 Starting batch LLM completion...")
	
	startTime := time.Now()
	
	response, err := llmService.Chat(ctx, messages, &llm.ChatOptions{
		MaxTokens: maxTokens,
	})
	if err != nil {
		return fmt.Errorf("LLM chat failed: %w", err)
	}
	
	completionTime := time.Since(startTime)
	
	fmt.Printf("\n🎯 LLM Response:\n")
	fmt.Printf("📝 Text: \"%s\"\n", response.Message.Content)
	fmt.Printf("⏱️ Completion time: %v\n", completionTime)
	fmt.Printf("🎯 Finish reason: %s\n", response.FinishReason)
	fmt.Printf("📊 Usage: %+v\n", response.Usage)
	
	if len(response.Metadata) > 0 {
		fmt.Printf("📋 Metadata: %+v\n", response.Metadata)
	}
	
	fmt.Println("✅ Batch LLM test completed successfully!")
	return nil
}

// runStreamingLLMTest tests streaming LLM completion
func runStreamingLLMTest(ctx context.Context, llmService llm.LLM, messages []llm.Message, maxTokens int) error {
	fmt.Println("🌊 Starting streaming LLM completion...")
	
	stream, err := llmService.ChatStream(ctx, messages, &llm.ChatOptions{
		MaxTokens: maxTokens,
	})
	if err != nil {
		return fmt.Errorf("failed to create LLM stream: %w", err)
	}
	defer stream.Close()
	
	fmt.Printf("✅ Created LLM streaming session\n")
	fmt.Printf("📤 Streaming response:\n> ")
	
	startTime := time.Now()
	var fullResponse string
	chunkCount := 0
	
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("LLM streaming error: %w", err)
		}
		
		chunkCount++
		fullResponse += chunk.Delta.Content
		
		// Print chunk immediately for streaming effect
		fmt.Print(chunk.Delta.Content)
		
		// Show chunk info occasionally
		if chunkCount%10 == 0 {
			fmt.Printf(" [chunk %d]", chunkCount)
		}
	}
	
	completionTime := time.Since(startTime)
	
	fmt.Printf("\n\n🎯 Streaming Results:\n")
	fmt.Printf("📝 Full response: \"%s\"\n", fullResponse)
	fmt.Printf("⏱️ Total completion time: %v\n", completionTime)
	fmt.Printf("📊 Chunks received: %d\n", chunkCount)
	if chunkCount > 0 {
		fmt.Printf("🚀 Average chunk time: %v\n", completionTime/time.Duration(chunkCount))
	}
	
	fmt.Println("✅ Streaming LLM test completed successfully!")
	return nil
}