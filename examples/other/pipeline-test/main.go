package main

import (
	"context"
	"log"
	"os"

	"livekit-agents-go/agents"
	"livekit-agents-go/plugins"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/openai"
	_ "livekit-agents-go/test/mock"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/media"
)

// PipelineTestAgent implements a service integration testing agent
// This preserves the original voice-assistant simulation logic for testing purposes
type PipelineTestAgent struct {
	*agents.BaseAgent
	instructions string
}

// NewPipelineTestAgent creates a new pipeline testing agent
func NewPipelineTestAgent() *PipelineTestAgent {
	return &PipelineTestAgent{
		BaseAgent: agents.NewBaseAgent("PipelineTestAgent"),
		instructions: "You are a helpful assistant testing the voice pipeline. " +
			"Keep your responses concise and to the point. " +
			"You are curious and friendly, and have a sense of humor.",
	}
}

// Start initializes the pipeline testing agent
func (a *PipelineTestAgent) Start(ctx context.Context, session *agents.AgentSession) error {
	log.Println("Pipeline test agent started")
	log.Printf("Agent instructions: %s", a.instructions)

	// Create smart services using the plugin system
	// This will automatically use real AI services if API keys are available,
	// or fall back to mock services for development
	services, err := plugins.CreateSmartServices()
	if err != nil {
		log.Printf("❌ Failed to create services: %v", err)
		return err
	}

	log.Printf("Services initialized for pipeline testing:")
	if services.STT != nil {
		log.Printf("  STT: %s v%s", services.STT.Name(), services.STT.Version())
	}
	if services.LLM != nil {
		log.Printf("  LLM: %s v%s", services.LLM.Name(), services.LLM.Version())
	}
	if services.TTS != nil {
		log.Printf("  TTS: %s v%s", services.TTS.Name(), services.TTS.Version())
	}
	if services.VAD != nil {
		log.Printf("  VAD: %s v%s", services.VAD.Name(), services.VAD.Version())
	}

	// Run pipeline simulation for testing
	return a.runPipelineTest(ctx, services)
}

// runPipelineTest demonstrates the voice conversation pipeline with simulated data
func (a *PipelineTestAgent) runPipelineTest(ctx context.Context, services *plugins.SmartServices) error {
	log.Println("Starting pipeline test simulation...")

	// Simulate receiving audio and processing it through the pipeline
	for i := 0; i < 3; i++ {
		log.Printf("\n=== Pipeline Test %d ===", i+1)

		// 1. Simulate receiving audio input (silent frames for testing)
		audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
		log.Printf("📥 Received audio frame: %d bytes", len(audioFrame.Data))

		// 2. Voice Activity Detection
		detection, err := services.VAD.Detect(ctx, audioFrame)
		if err != nil {
			log.Printf("❌ VAD error: %v", err)
			continue
		}
		log.Printf("🎤 VAD detection: speech=%.2f%%, is_speech=%t", 
			detection.Probability*100, detection.IsSpeech)

		if !detection.IsSpeech {
			log.Println("⏭️ No speech detected, skipping test (expected with silent audio)")
			continue
		}

		// 3. Speech-to-Text
		recognition, err := services.STT.Recognize(ctx, audioFrame)
		if err != nil {
			log.Printf("❌ STT error: %v", err)
			continue
		}
		log.Printf("📝 STT result: '%s' (confidence: %.2f)", 
			recognition.Text, recognition.Confidence)

		// 4. LLM processing
		messages := []llm.Message{
			{Role: llm.RoleSystem, Content: a.instructions},
			{Role: llm.RoleUser, Content: recognition.Text},
		}
		
		chatResponse, err := services.LLM.Chat(ctx, messages, nil)
		if err != nil {
			log.Printf("❌ LLM error: %v", err)
			continue
		}
		log.Printf("🤖 LLM response: '%s'", chatResponse.Message.Content)
		log.Printf("📊 LLM usage: %d tokens", chatResponse.Usage.TotalTokens)

		// 5. Text-to-Speech
		audioResponse, err := services.TTS.Synthesize(ctx, chatResponse.Message.Content, nil)
		if err != nil {
			log.Printf("❌ TTS error: %v", err)
			continue
		}
		log.Printf("🔊 TTS synthesized: %d bytes audio, duration: %v", 
			len(audioResponse.Data), audioResponse.Duration)

		log.Printf("✅ Pipeline test %d completed successfully", i+1)
	}

	log.Println("\n🎉 Pipeline test simulation completed - all services working!")
	return nil
}

// HandleEvent handles agent events
func (a *PipelineTestAgent) HandleEvent(event agents.AgentEvent) error {
	switch event.Type() {
	case agents.EventParticipantJoined:
		log.Println("Event: Participant joined - starting pipeline test")
	case agents.EventParticipantLeft:
		log.Println("Event: Participant left - ending pipeline test")
	case agents.EventDataReceived:
		log.Printf("Event: Data received - %v", event.Data())
	}
	return nil
}

// entrypoint is the main agent entrypoint function
func entrypoint(ctx *agents.JobContext) error {
	log.Printf("🧪 Pipeline Test Agent starting...")

	// Create and start the pipeline test agent
	agent := NewPipelineTestAgent()
	
	// Create a session (this would normally be handled by the framework)
	session := agents.NewAgentSession(ctx.Context)
	session.Agent = agent

	return session.Start()
}

func main() {
	// Configure worker options
	opts := &agents.WorkerOptions{
		EntrypointFunc: entrypoint,
		AgentName:      "PipelineTestAgent",
		APIKey:         os.Getenv("LIVEKIT_API_KEY"),
		APISecret:      os.Getenv("LIVEKIT_API_SECRET"),
		Host:           os.Getenv("LIVEKIT_HOST"),
		LiveKitURL:     os.Getenv("LIVEKIT_URL"),
		Metadata: map[string]string{
			"description": "Pipeline testing agent for service integration validation",
			"version":     "1.0.0",
			"type":        "pipeline-test",
			"purpose":     "testing",
		},
	}

	// Set defaults for development
	if opts.Host == "" {
		opts.Host = "localhost:7880"
	}
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7880"
	}

	log.Printf("Starting Pipeline Test Agent: %s", opts.AgentName)
	log.Printf("LiveKit Host: %s", opts.Host)
	log.Printf("LiveKit URL: %s", opts.LiveKitURL)

	// Run with CLI
	if err := agents.RunApp(opts); err != nil {
		log.Fatal("Failed to run agent:", err)
	}
}