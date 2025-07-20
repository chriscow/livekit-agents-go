package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"livekit-agents-go/agents"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/deepgram"
	_ "livekit-agents-go/plugins/openai"
	_ "livekit-agents-go/plugins/silero"
)

// BasicAgent implements a basic conversational voice agent matching Python basic_agent.py
type BasicAgent struct {
	*agents.BaseAgent
	audioFrameCount int64
	speechDetected  bool
	lastSTTResult   string
}

// WeatherData represents weather API response structure
type WeatherData struct {
	Current struct {
		Temperature2m float64 `json:"temperature_2m"`
	} `json:"current"`
}

// NewBasicAgent creates a new basic agent matching Python basic_agent.py
func NewBasicAgent() *BasicAgent {
	instructions := "Your name is Kelly. You would interact with users via voice. " +
		"With that in mind keep your responses concise and to the point. " +
		"You are curious and friendly, and have a sense of humor. " +
		"You use streaming AI services for real-time conversation."

	return &BasicAgent{
		BaseAgent:       agents.NewBaseAgentWithInstructions("BasicAgent", instructions),
		audioFrameCount: 0,
		speechDetected:  false,
		lastSTTResult:   "",
	}
}

// OnEnter is called when the agent enters the session (Python framework pattern)
func (a *BasicAgent) OnEnter(ctx context.Context, session *agents.AgentSession) error {
	logEssential("🚀 Basic agent entered session - generating initial reply")
	logVerbose("🔧 [DIAGNOSTIC] Session details: VAD=%v, STT=%v, LLM=%v, TTS=%v",
		session.VAD != nil, session.STT != nil, session.LLM != nil, session.TTS != nil)

	// Log service names if available
	if session.VAD != nil {
		logVerbose("🎤 [DIAGNOSTIC] VAD service: %s", session.VAD.Name())
	}
	if session.STT != nil {
		logVerbose("👂 [DIAGNOSTIC] STT service: %s", session.STT.Name())
	}
	if session.LLM != nil {
		logVerbose("🧠 [DIAGNOSTIC] LLM service: %s", session.LLM.Name())
	}
	if session.TTS != nil {
		logVerbose("🔊 [DIAGNOSTIC] TTS service: %s", session.TTS.Name())
	}

	logVerbose("📝 [DIAGNOSTIC] About to generate initial reply...")
	// when the agent is added to the session, it'll generate a reply
	// according to its instructions (matching Python basic_agent.py)

	// Skip automatic greeting in console mode to avoid duplicate greetings
	// ChatCLI will handle the initial greeting instead
	if session.Room != nil {
		// Generate initial reply asynchronously to avoid blocking session start (room mode only)
		go func() {
			logVerbose("🎬 [DIAGNOSTIC] Generating initial reply in background...")
			if err := session.GenerateReply(); err != nil {
				logEssential("⚠️ Initial reply generation failed: %v", err)
			} else {
				logEssential("✅ Initial reply generated successfully")
				logAudio("🔍 [AUDIO] TTS AUDIO PATH: Initial reply audio was generated")
				logAudio("🔍 [AUDIO] TTS AUDIO PATH: Audio will be sent to LiveKit room track (if room connected)")
				logAudio("🔍 [AUDIO] TTS AUDIO PATH: Audio will be routed via session callback (console mode)")
			}
		}()
	} else {
		logVerbose("🔇 [CONSOLE] Skipping automatic greeting - ChatCLI will handle initial greeting")
	}

	logVerbose("✅ [DIAGNOSTIC] OnEnter completed - session.Start() can return now")
	return nil
}

// OnUserTurnCompleted is called after each user turn in conversation (Python framework pattern)
func (a *BasicAgent) OnUserTurnCompleted(ctx context.Context, chatCtx *llm.ChatContext, newMessage *llm.ChatMessage) error {
	logVerbose("✅ [DIAGNOSTIC] User turn completed: %s", newMessage.Content)
	logVerbose("💬 [DIAGNOSTIC] Chat context has %d messages", len(chatCtx.Messages))
	a.lastSTTResult = newMessage.Content
	logVerbose("🔊 [DIAGNOSTIC] Audio frames processed so far: %d", a.audioFrameCount)
	logVerbose("🎯 [DIAGNOSTIC] Speech detection status: %v", a.speechDetected)
	// This is where agents can perform any post-turn processing
	// such as updating context, triggering actions, etc.
	return nil
}

// OnAudioFrame is called when audio frames are received (diagnostic purposes)
func (a *BasicAgent) OnAudioFrame(frame interface{}) {
	a.audioFrameCount++
	if a.audioFrameCount%100 == 0 { // Log every 100 frames to avoid spam
		logVerbose("🎤 [DIAGNOSTIC] Audio frame #%d received (logging every 100 frames)", a.audioFrameCount)
	}
}

// OnSpeechDetected is called when VAD detects speech
func (a *BasicAgent) OnSpeechDetected(probability float64) {
	if !a.speechDetected {
		logVerbose("🗣️ [DIAGNOSTIC] SPEECH DETECTED! Probability: %.2f", probability)
		a.speechDetected = true
	}
}

// OnSpeechEnded is called when VAD detects speech has ended
func (a *BasicAgent) OnSpeechEnded() {
	if a.speechDetected {
		logVerbose("🤫 [DIAGNOSTIC] Speech ended")
		a.speechDetected = false
	}
}

// LookupWeather implements weather function tool (matching Python basic_agent.py)
// Called when the user asks for weather related information.
// Ensure the user's location (city or region) is provided.
// When given a location, please estimate the latitude and longitude of the location and
// do not ask the user for them.
func (a *BasicAgent) LookupWeather(ctx context.Context, location, latitude, longitude string) (string, error) {
	logVerbose("🌤️ [DIAGNOSTIC] Function called: LookupWeather for %s (lat: %s, lon: %s)", location, latitude, longitude)

	// Try real weather API if available, otherwise return mock data
	if os.Getenv("USE_REAL_WEATHER") == "true" {
		return a.getRealWeather(ctx, latitude, longitude)
	}

	// Return mock weather data (matching Python basic_agent.py)
	return "sunny with a temperature of 70 degrees.", nil
}

// getRealWeather calls the actual weather API (like Python basic_agent.py would)
func (a *BasicAgent) getRealWeather(ctx context.Context, latitude, longitude string) (string, error) {
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
func (a *BasicAgent) HandleEvent(event agents.AgentEvent) error {
	switch event.Type() {
	case agents.EventParticipantJoined:
		logVerbose("👥 [DIAGNOSTIC] Event: Participant joined - basic agent ready")
	case agents.EventParticipantLeft:
		logVerbose("👋 [DIAGNOSTIC] Event: Participant left")
	case agents.EventDataReceived:
		logVerbose("📦 [DIAGNOSTIC] Event: Data received - %v", event.Data())
	default:
		logVerbose("❓ [DIAGNOSTIC] Event: Unknown event type: %v", event.Type())
	}
	return nil
}

// Global context and cancel for proper shutdown
var (
	globalCtx    context.Context
	globalCancel context.CancelFunc
	shutdownWg   sync.WaitGroup
	verboseMode  bool
)

// Helper function for conditional logging
func logVerbose(format string, args ...interface{}) {
	if verboseMode {
		log.Printf(format, args...)
	}
}

func logAudio(format string, args ...interface{}) {
	// Always log audio-related messages
	log.Printf(format, args...)
}

func logEssential(format string, args ...interface{}) {
	// Always log essential messages
	log.Printf(format, args...)
}

// entrypoint is the main agent entrypoint function (matching Python basic_agent.py structure)
func entrypoint(ctx *agents.JobContext) error {
	startTime := time.Now()
	logVerbose("⏰ [DIAGNOSTIC] Entrypoint started at: %s", startTime.Format("15:04:05.000"))

	// Create a cancellable context for proper shutdown handling
	globalCtx, globalCancel = context.WithCancel(ctx.Context)
	defer globalCancel()

	// Set up signal handling for graceful shutdown
	setupSignalHandling()

	// each log entry will include these fields (matching Python pattern)
	// TODO: Implement log context fields when logging framework is ready
	if ctx.Room != nil {
		logEssential("🏠 Starting basic agent in room: %s", ctx.Room.Name())
	} else {
		logEssential("🔗 Starting basic agent (room not connected yet)")
	}

	var session *agents.AgentSession

	// Use pre-configured session from console mode if available, otherwise create new one
	if ctx.Session != nil {
		logVerbose("📋 [DIAGNOSTIC] Using pre-configured session from console mode")
		logVerbose("🔍 [DIAGNOSTIC] Pre-configured session VAD: %v, STT: %v, LLM: %v, TTS: %v",
			ctx.Session.VAD != nil, ctx.Session.STT != nil, ctx.Session.LLM != nil, ctx.Session.TTS != nil)
		session = ctx.Session
		session.Agent = NewBasicAgent()

		// Replace the session's context with our cancellable one
		session.Context = globalCtx
	} else {
		logVerbose("🛠️ [DIAGNOSTIC] Creating new session and services...")
		logVerbose("🌊 [DIAGNOSTIC] Basic agent configured for streaming mode (default)")

		// Create services and session for other modes (dev, start)
		services, err := plugins.CreateSmartServices()
		if err != nil {
			logEssential("❌ Failed to create services: %v", err)
			return fmt.Errorf("failed to create services: %w", err)
		}

		logVerbose("✅ [DIAGNOSTIC] Services initialized with streaming support:")
		if services.STT != nil {
			logVerbose("  👂 STT: %s (🌊 streaming)", services.STT.Name())
		} else {
			logVerbose("  ❌ STT: nil")
		}
		if services.LLM != nil {
			logVerbose("  🧠 LLM: %s (🌊 streaming)", services.LLM.Name())
		} else {
			logVerbose("  ❌ LLM: nil")
		}
		if services.TTS != nil {
			logVerbose("  🔊 TTS: %s (🌊 streaming)", services.TTS.Name())
		} else {
			logVerbose("  ❌ TTS: nil")
		}
		if services.VAD != nil {
			logVerbose("  🎤 VAD: %s (real-time)", services.VAD.Name())
		} else {
			logVerbose("  ❌ VAD: nil")
		}

		// Create agent session with services (Python framework pattern)
		logVerbose("🎭 [DIAGNOSTIC] Creating agent instance...")
		agent := NewBasicAgent()
		logVerbose("💡 [DIAGNOSTIC] Agent instructions: %s", agent.GetInstructions())

		logVerbose("📝 [DIAGNOSTIC] Creating agent session...")
		session = agents.NewAgentSessionWithInstructions(globalCtx, agent.GetInstructions())
		session.Agent = agent

		// Wire services into session
		logVerbose("🔌 [DIAGNOSTIC] Wiring services into session...")
		session.VAD = services.VAD
		session.STT = services.STT
		session.LLM = services.LLM
		session.TTS = services.TTS

		logVerbose("🔧 [DIAGNOSTIC] Session wired: VAD=%v, STT=%v, LLM=%v, TTS=%v",
			session.VAD != nil, session.STT != nil, session.LLM != nil, session.TTS != nil)
	}

	logVerbose("🚀 [DIAGNOSTIC] About to start session...")

	// Start periodic status logger to help diagnose hanging issues
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		for {
			select {
			case <-statusTicker.C:
				if session.Agent != nil {
					agent, ok := session.Agent.(*BasicAgent)
					if ok {
						logVerbose("📊 [DIAGNOSTIC] Status: Audio frames: %d, Speech detected: %v, Last STT: '%s'",
							agent.audioFrameCount, agent.speechDetected, agent.lastSTTResult)
					}
				}
				logVerbose("⚡ [DIAGNOSTIC] Session alive check: VAD=%v, STT=%v, LLM=%v, TTS=%v",
					session.VAD != nil, session.STT != nil, session.LLM != nil, session.TTS != nil)
			case <-globalCtx.Done():
				logVerbose("📊 [DIAGNOSTIC] Status ticker stopped due to context cancellation")
				return
			}
		}
	}()

	// Start the session with proper error handling and timeout
	sessionDone := make(chan error, 1)
	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		logVerbose("🎬 [DIAGNOSTIC] Starting session goroutine...")
		startTime := time.Now()
		err := session.Start()
		elapsed := time.Since(startTime)
		logVerbose("📴 [DIAGNOSTIC] Session.Start() returned after %v with error: %v", elapsed, err)
		sessionDone <- err
	}()

	// For console mode, keep running until shutdown signal - don't wait for session completion
	if ctx.Room == nil {
		logVerbose("🎯 [DIAGNOSTIC] Console mode - keeping context alive for audio pipeline...")
		// Wait for shutdown signal only
		<-globalCtx.Done()
		logVerbose("🛑 [DIAGNOSTIC] Received shutdown signal, stopping session...")
		// Give session time to stop gracefully
		select {
		case err := <-sessionDone:
			logVerbose("✅ [DIAGNOSTIC] Session stopped gracefully with: %v", err)
		case <-time.After(10 * time.Second):
			logVerbose("⚠️ [DIAGNOSTIC] Session shutdown timeout after 10s - forcing exit")
		}
	} else {
		// For dev/start modes, wait for session completion or shutdown signal
		select {
		case err := <-sessionDone:
			if err != nil {
				logEssential("❌ Session start failed: %v", err)
				return err
			}
		case <-globalCtx.Done():
			logVerbose("🛑 [DIAGNOSTIC] Received shutdown signal, stopping session...")
			// Give session time to stop gracefully
			select {
			case err := <-sessionDone:
				logVerbose("✅ [DIAGNOSTIC] Session stopped gracefully with: %v", err)
			case <-time.After(10 * time.Second):
				logVerbose("⚠️ [DIAGNOSTIC] Session shutdown timeout after 10s - forcing exit")
			}
		}
	}

	elapsed := time.Since(startTime)
	logVerbose("✅ [DIAGNOSTIC] Session completed after %v", elapsed)
	return nil
}

// setupSignalHandling sets up graceful shutdown on SIGINT and SIGTERM
func setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		sig := <-sigChan
		logVerbose("🔔 [DIAGNOSTIC] Received signal: %v", sig)
		logVerbose("🛑 [DIAGNOSTIC] Initiating graceful shutdown...")

		if globalCancel != nil {
			globalCancel()
		}

		// Force exit if graceful shutdown takes too long
		go func() {
			time.Sleep(15 * time.Second)
			logVerbose("💀 [DIAGNOSTIC] Graceful shutdown timeout after 15s - forcing exit")
			os.Exit(1)
		}()
	}()
}

func main() {
	// Parse command line flags
	var (
		duration = flag.Duration("duration", 0, "test duration for console mode (0 = run forever)")
		mode     = flag.String("mode", "", "agent mode: console, dev, start")
		verbose  = flag.Bool("verbose", false, "enable verbose diagnostic logging")
	)
	flag.Parse()

	// Set global verbose mode
	verboseMode = *verbose

	logVerbose("🏁 [DIAGNOSTIC] Main function started")

	// If duration is specified, set up auto-shutdown timer
	if *duration > 0 {
		logVerbose("⏱️ [DIAGNOSTIC] Auto-shutdown timer set for %v", *duration)
		go func() {
			time.Sleep(*duration)
			logVerbose("⏰ [DIAGNOSTIC] Duration %v expired - initiating shutdown", *duration)
			if globalCancel != nil {
				globalCancel()
			}
		}()
	}

	// Parse mode from args if not provided via flag
	if *mode == "" && len(flag.Args()) > 0 {
		*mode = flag.Args()[0]
	}

	// Load environment variables from .env file, overriding any existing env vars
	if err := godotenv.Overload(); err != nil {
		logVerbose("⚠️ [DIAGNOSTIC] Warning: .env file not found, using system environment variables")
	} else {
		logVerbose("📁 [DIAGNOSTIC] Loaded .env file successfully")
	}

	// Log environment variables (safely, without revealing secrets)
	logVerbose("🔑 [DIAGNOSTIC] Environment check - LIVEKIT_API_KEY set: %v", os.Getenv("LIVEKIT_API_KEY") != "")
	logVerbose("🔑 [DIAGNOSTIC] Environment check - OPENAI_API_KEY set: %v", os.Getenv("OPENAI_API_KEY") != "")
	logVerbose("🎯 [DIAGNOSTIC] Agent mode: %s, Duration: %v", *mode, *duration)

	// Configure worker options (matching Python cli.run_app(WorkerOptions(...)))
	opts := &agents.WorkerOptions{
		EntrypointFunc: entrypoint,
		AgentName:      "BasicAgent", // Match Python basic_agent.py
		APIKey:         os.Getenv("LIVEKIT_API_KEY"),
		APISecret:      os.Getenv("LIVEKIT_API_SECRET"),
		Host:           os.Getenv("LIVEKIT_HOST"),
		LiveKitURL:     os.Getenv("LIVEKIT_URL"),
		Metadata: map[string]string{
			"description": "Basic agent with weather function calling",
			"version":     "1.0.0",
			"type":        "basic-agent",
			"functions":   "lookup_weather",
		},
	}

	// Override mode if specified via command line
	if *mode != "" {
		os.Args = append([]string{os.Args[0]}, *mode)
	}

	// Set defaults for development
	if opts.Host == "" {
		opts.Host = "localhost:7880"
	}
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7880"
	}

	logVerbose("🤖 [DIAGNOSTIC] Starting Basic Agent: %s", opts.AgentName)
	logVerbose("🌐 [DIAGNOSTIC] LiveKit Host: %s", opts.Host)
	logVerbose("🔗 [DIAGNOSTIC] LiveKit URL: %s", opts.LiveKitURL)

	logVerbose("🚀 [DIAGNOSTIC] About to call RunApp...")

	// Run with CLI
	if err := agents.RunApp(opts); err != nil {
		logEssential("💥 RunApp failed: %v", err)
		log.Fatal("Failed to run agent:", err)
	}

	// Wait for all shutdown operations to complete
	logVerbose("🔄 [DIAGNOSTIC] Waiting for graceful shutdown...")
	shutdownWg.Wait()
	logVerbose("🏁 [DIAGNOSTIC] RunApp completed normally")
}
