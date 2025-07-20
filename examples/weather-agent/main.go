package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"livekit-agents-go/agents"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/openai"
	_ "livekit-agents-go/plugins/silero"
)

// WeatherAgent implements a weather agent with function calling
type WeatherAgent struct {
	*agents.BaseAgent
}

// WeatherData represents weather API response structure
type WeatherData struct {
	Current struct {
		Temperature2m float64 `json:"temperature_2m"`
	} `json:"current"`
}

// NewWeatherAgent creates a new weather agent
func NewWeatherAgent() *WeatherAgent {
	instructions := "You are a weather agent. When users ask about the weather in a location, " +
		"use the get_weather function. You should estimate latitude and longitude for any location " +
		"the user mentions - don't ask them for coordinates."
	
	return &WeatherAgent{
		BaseAgent: agents.NewBaseAgentWithInstructions("WeatherAgent", instructions),
	}
}

// OnEnter is called when the agent enters the session
func (a *WeatherAgent) OnEnter(ctx context.Context, session *agents.AgentSession) error {
	log.Println("🌦️ Weather agent OnEnter called - generating initial reply...")
	
	// Log session details
	log.Printf("🔍 Session details:")
	log.Printf("  - STT: %v", session.STT != nil)
	log.Printf("  - LLM: %v", session.LLM != nil) 
	log.Printf("  - TTS: %v", session.TTS != nil)
	log.Printf("  - VAD: %v", session.VAD != nil)
	log.Printf("  - Tool Registry: %d tools", session.ToolRegistry.Count())
	
	err := session.GenerateReply()
	if err != nil {
		log.Printf("❌ Failed to generate initial reply: %v", err)
		return err
	}
	
	log.Println("✅ Weather agent OnEnter completed successfully")
	return nil
}

// OnUserTurnCompleted is called after each user turn in conversation
func (a *WeatherAgent) OnUserTurnCompleted(ctx context.Context, chatCtx *llm.ChatContext, newMessage *llm.ChatMessage) error {
	log.Printf("🎤 OnUserTurnCompleted: User said '%s'", newMessage.Content)
	log.Printf("🔍 Chat context has %d messages", len(chatCtx.Messages))
	return nil
}

// LookupWeather implements weather function tool (matching Python basic_agent.py)
// Called when the user asks for weather related information.
// Ensure the user's location (city or region) is provided.
// When given a location, please estimate the latitude and longitude of the location and
// do not ask the user for them.
func (a *WeatherAgent) LookupWeather(ctx context.Context, location, latitude, longitude string) (string, error) {
	log.Printf("🌤️ LookupWeather CALLED! location=%s, lat=%s, lon=%s", location, latitude, longitude)

	// Try real weather API if available, otherwise return mock data
	if os.Getenv("USE_REAL_WEATHER") == "true" {
		return a.getRealWeather(ctx, latitude, longitude)
	}

	// Return mock weather data (matching Python basic_agent.py)
	return "sunny with a temperature of 72 degrees.", nil
}

// getRealWeather calls the actual weather API
func (a *WeatherAgent) getRealWeather(ctx context.Context, latitude, longitude string) (string, error) {
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&current=temperature_2m", latitude, longitude)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var weatherData WeatherData
	if err := json.Unmarshal(body, &weatherData); err != nil {
		return "", fmt.Errorf("failed to parse weather data: %w", err)
	}

	return fmt.Sprintf("The temperature is %.1f degrees Celsius", weatherData.Current.Temperature2m), nil
}

// HandleEvent handles agent events
func (a *WeatherAgent) HandleEvent(event agents.AgentEvent) error {
	switch event.Type() {
	case agents.EventParticipantJoined:
		log.Println("Event: Participant joined - weather agent ready")
	case agents.EventParticipantLeft:
		log.Println("Event: Participant left")
	case agents.EventDataReceived:
		log.Printf("Event: Data received - %v", event.Data())
	}
	return nil
}

// entrypoint is the main agent entrypoint function
func entrypoint(ctx *agents.JobContext) error {
	if ctx.Room != nil {
		log.Printf("Starting weather agent in room: %s", ctx.Room.Name())
	} else {
		log.Printf("Starting weather agent (room not connected yet)")
	}

	var session *agents.AgentSession

	// Use pre-configured session from console mode if available, otherwise create new one
	if ctx.Session != nil {
		log.Println("Using pre-configured session from console mode")
		session = ctx.Session
		session.Agent = NewWeatherAgent()
	} else {
		// Create services and session for other modes (dev, start)
		services, err := plugins.CreateSmartServices()
		if err != nil {
			return fmt.Errorf("failed to create services: %w", err)
		}

		log.Printf("Services initialized:")
		if services.STT != nil {
			log.Printf("  STT: %s", services.STT.Name())
		}
		if services.LLM != nil {
			log.Printf("  LLM: %s", services.LLM.Name())
		}
		if services.TTS != nil {
			log.Printf("  TTS: %s", services.TTS.Name())
		}
		if services.VAD != nil {
			log.Printf("  VAD: %s", services.VAD.Name())
		}

		// Create agent session with services
		agent := NewWeatherAgent()
		session = agents.NewAgentSessionWithInstructions(ctx.Context, agent.GetInstructions())
		session.Agent = agent
		
		// Wire services into session
		session.VAD = services.VAD
		session.STT = services.STT  
		session.LLM = services.LLM
		session.TTS = services.TTS
	}

	// Start the session with the agent and room
	log.Println("🚀 Starting session...")
	err := session.Start()
	if err != nil {
		log.Printf("❌ Session failed to start: %v", err)
		return err
	}
	
	log.Println("✅ Session started successfully")
	return nil
}

func main() {
	// Load environment variables from .env file, overriding any existing env vars
	if err := godotenv.Overload(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Configure worker options
	opts := &agents.WorkerOptions{
		EntrypointFunc: entrypoint,
		AgentName:      "WeatherAgent",
		APIKey:         os.Getenv("LIVEKIT_API_KEY"),
		APISecret:      os.Getenv("LIVEKIT_API_SECRET"),
		Host:           os.Getenv("LIVEKIT_HOST"),
		LiveKitURL:     os.Getenv("LIVEKIT_URL"),
		Metadata: map[string]string{
			"description": "Weather agent with lookup_weather function calling",
			"version":     "1.0.0",
			"type":        "weather-agent",
			"functions":   "lookup_weather",
		},
	}

	// Set defaults for development
	if opts.Host == "" {
		opts.Host = "localhost:7880"
	}
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7880"
	}

	log.Printf("Starting Weather Agent: %s", opts.AgentName)
	log.Printf("LiveKit Host: %s", opts.Host)
	log.Printf("LiveKit URL: %s", opts.LiveKitURL)
	log.Printf("Using real weather API: %t", os.Getenv("USE_REAL_WEATHER") == "true")

	// Run with CLI
	if err := agents.RunApp(opts); err != nil {
		log.Fatal("Failed to run agent:", err)
	}
}