package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"livekit-agents-go/agents"
	"livekit-agents-go/plugins/openai"
	"livekit-agents-go/services/llm"

	"github.com/livekit/protocol/auth"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// GreetingAgent implements a simple agent that greets participants
type GreetingAgent struct {
	*agents.BaseAgent
}

// NewGreetingAgent creates a new greeting agent
func NewGreetingAgent() *GreetingAgent {
	return &GreetingAgent{
		BaseAgent: agents.NewBaseAgent("GreetingAgent"),
	}
}

// Start initializes the greeting agent
func (a *GreetingAgent) Start(ctx context.Context, session *agents.AgentSession) error {
	log.Println("Greeting agent started")

	// TODO: Set up room event handlers using proper LiveKit SDK API
	// The callback structure may differ in the actual LiveKit SDK
	// This would be implemented once we have proper room connection

	return nil
}

// onParticipantConnected handles new participant connections
func (a *GreetingAgent) onParticipantConnected(participant *lksdk.RemoteParticipant) {
	log.Printf("Participant connected: %s", participant.Identity())

	// Send a greeting message
	greeting := "Hello! Welcome to the LiveKit room. I'm your AI assistant."
	if participant.Identity() != "" {
		greeting = "Hello " + participant.Identity() + "! Welcome to the LiveKit room."
	}

	// TODO: Use TTS to synthesize the greeting
	// For now, just log the greeting
	log.Printf("Greeting sent: %s", greeting)
}

// onParticipantDisconnected handles participant disconnections
func (a *GreetingAgent) onParticipantDisconnected(participant *lksdk.RemoteParticipant) {
	log.Printf("Participant disconnected: %s", participant.Identity())
}

// HandleEvent handles agent events
func (a *GreetingAgent) HandleEvent(event agents.AgentEvent) error {
	switch event.Type() {
	case agents.EventParticipantJoined:
		log.Println("Event: Participant joined")
	case agents.EventParticipantLeft:
		log.Println("Event: Participant left")
	case agents.EventDataReceived:
		log.Printf("Event: Data received - %v", event.Data())
	}
	return nil
}

// entrypoint is the main agent entrypoint function
func entrypoint(ctx *agents.JobContext) error {
	log.Printf("🎉 ENTRYPOINT CALLED! JobContext received")

	if ctx.Room != nil {
		log.Printf("Starting greeting agent in room: %s", ctx.Room.Name())

		// Test Step 5: Create OpenAI LLM service and generate greeting
		return testLLMService(ctx)
	} else {
		log.Printf("No room connection yet, but entrypoint function is working!")
		return nil
	}
}

// testLLMService tests the OpenAI LLM service with a simple greeting
func testLLMService(ctx *agents.JobContext) error {
	log.Printf("Testing OpenAI LLM service...")

	// Get OpenAI API key
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Printf("No OpenAI API key provided, skipping LLM test")
		return nil
	}

	// Create OpenAI LLM service
	llmService := openai.NewGPTLLM(openaiAPIKey, "gpt-4.1-nano")
	log.Printf("OpenAI LLM service created: %s v%s", llmService.Name(), llmService.Version())

	// Create a simple chat message to generate a greeting
	messages := []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: "You are a friendly AI assistant in a video call. Generate a very short greeting message (1 sentence) to welcome someone to the room.",
		},
		{
			Role:    llm.RoleUser,
			Content: "Someone just joined the room. Say hello!",
		},
	}

	// Get greeting from LLM
	response, err := llmService.Chat(ctx.Context, messages, nil)
	if err != nil {
		log.Printf("LLM chat failed: %v", err)
		return err
	}

	greeting := response.Message.Content
	log.Printf("🤖 LLM generated greeting: %s", greeting)
	log.Printf("📊 Token usage: %d prompt + %d completion = %d total",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)

	// Send greeting to the room as data
	if ctx.Room != nil {
		data := []byte(greeting)
		err := ctx.Room.LocalParticipant.PublishData(data, lksdk.WithDataPublishReliable(true))
		if err != nil {
			log.Printf("Failed to publish greeting: %v", err)
		} else {
			log.Printf("📡 Published greeting to room: %s", greeting)
		}
	}

	return nil
}

// generateToken generates a JWT token for testing
func generateToken(apiKey, apiSecret, room, identity string, validFor time.Duration) (string, error) {
	at := auth.NewAccessToken(apiKey, apiSecret)

	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}

	at.AddGrant(grant).
		SetIdentity(identity).
		SetValidFor(validFor)

	return at.ToJWT()
}

func main() {
	// Parse command line flags
	generateTokenFlag := flag.Bool("generate-token", false, "Generate a JWT token and exit")
	roomFlag := flag.String("room", "test-room", "Room name for token generation")
	identityFlag := flag.String("identity", "test-user", "User identity for token generation")
	validForFlag := flag.Duration("valid-for", time.Hour, "Token validity duration")
	flag.Parse()

	// Check for required environment variables
	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		log.Fatal("LIVEKIT_API_KEY and LIVEKIT_API_SECRET environment variables are required")
	}

	// Handle token generation flag
	if *generateTokenFlag {
		token, err := generateToken(apiKey, apiSecret, *roomFlag, *identityFlag, *validForFlag)
		if err != nil {
			log.Fatalf("Failed to generate token: %v", err)
		}
		fmt.Printf("Generated token for room '%s', identity '%s', valid for %v:\n", *roomFlag, *identityFlag, *validForFlag)
		fmt.Println(token)
		return
	}

	// Register OpenAI plugin if API key is available
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey != "" {
		err := openai.RegisterPlugin(openaiAPIKey)
		if err != nil {
			log.Printf("Warning: Failed to register OpenAI plugin: %v", err)
		} else {
			log.Println("OpenAI plugin registered successfully")
		}
	}

	// Configure worker options
	opts := &agents.WorkerOptions{
		EntrypointFunc: entrypoint,
		AgentName:      "GreetingAgent",
		APIKey:         apiKey,
		APISecret:      apiSecret,
		Host:           os.Getenv("LIVEKIT_HOST"),
		LiveKitURL:     os.Getenv("LIVEKIT_URL"), // Add LiveKit URL for Step 2
		Metadata: map[string]string{
			"description": "A simple greeting agent",
			"version":     "1.0.0",
		},
	}

	// Set default host if not provided
	if opts.Host == "" {
		opts.Host = "localhost:7880"
	}

	// Set default LiveKit URL if not provided
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7880"
	}

	log.Printf("Starting LiveKit Agent: %s", opts.AgentName)
	log.Printf("LiveKit Host: %s", opts.Host)
	log.Printf("LiveKit URL: %s", opts.LiveKitURL)
	log.Printf("API Key: %s", opts.APIKey)
	log.Printf("API Secret: %s", opts.APISecret)

	// Run with CLI - equivalent to Python's cli.run_app()
	if err := agents.RunApp(opts); err != nil {
		log.Fatal("Failed to run agent:", err)
	}
}
