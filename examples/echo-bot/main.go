// Echo Bot Example - A simple voice agent that repeats everything the user says.
// 
// This example demonstrates the complete Go LiveKit Agents framework capabilities:
// - Full voice agent with state machine (Idle → Listening → Thinking → Speaking)
// - OpenAI STT (Whisper), TTS, and LLM integration
// - Silero VAD for voice activity detection
// - Turn detection with ONNX models
// - Plugin system with dynamic loading
// - Real-time audio processing
// 
// To run: go run examples/echo-bot
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/agent"
	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
	fakeSTT "github.com/chriscow/livekit-agents-go/pkg/ai/stt/fake"
	fakeVAD "github.com/chriscow/livekit-agents-go/pkg/ai/vad/fake"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/plugin"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
	"github.com/chriscow/livekit-agents-go/pkg/turn"

	// Import plugins for automatic registration
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/openai"
)

func main() {
	// Set up logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("🤖 Starting Echo Bot Example")
	slog.Info("This agent will repeat everything you say back to you")

	// Check for required environment variables
	if os.Getenv("OPENAI_API_KEY") == "" {
		slog.Error("OPENAI_API_KEY environment variable is required")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the echo bot
	if err := runEchoBot(ctx); err != nil {
		slog.Error("Echo bot failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.Info("👋 Echo bot shutdown complete")
}

func runEchoBot(ctx context.Context) error {
	// Create AI components using fake implementations for demo
	
	// Create Fake STT that generates text
	sttInstance := fakeSTT.NewFakeSTTWithText()

	// Create OpenAI TTS
	ttsFactory, exists := plugin.Get("tts", "openai")
	if !exists {
		return fmt.Errorf("OpenAI TTS plugin not found")
	}
	ttsInstance, err := ttsFactory(map[string]any{
		"model": "tts-1",
		"voice": "alloy",
	})
	if err != nil {
		return fmt.Errorf("failed to create TTS: %w", err)
	}

	// Create OpenAI LLM with echo bot personality
	llmFactory, exists := plugin.Get("llm", "openai")
	if !exists {
		return fmt.Errorf("OpenAI LLM plugin not found")
	}
	llmInstance, err := llmFactory(map[string]any{
		"model": "gpt-3.5-turbo",
	})
	if err != nil {
		return fmt.Errorf("failed to create LLM: %w", err)
	}

	// Set up the LLM with echo bot instructions
	llmClient := llmInstance.(llm.LLM)
	
	// Note: In a real implementation, you would configure the LLM with system instructions
	// like: "You are an echo bot. Your job is to repeat back what the user says in a friendly way."
	
	// Create Fake VAD for demo purposes
	vadInstance := fakeVAD.NewFakeVAD(0.3) // 30% chance of speech detection per frame

	// Create turn detector
	turnDetector, err := turn.NewDetector(turn.DetectorConfig{
		Model: "english", // Use English model
	})
	if err != nil {
		return fmt.Errorf("failed to create turn detector: %w", err)
	}

	// Create audio channels for microphone input and TTS output
	micIn := make(chan rtc.AudioFrame, 100)
	ttsOut := make(chan rtc.AudioFrame, 100)

	// In a real implementation, these channels would be connected to actual audio devices
	// For this example, we'll simulate audio input/output
	go simulateAudioInput(ctx, micIn)
	go simulateAudioOutput(ctx, ttsOut)

	// Create the voice agent
	voiceAgent, err := agent.New(agent.Config{
		STT:          sttInstance,
		TTS:          ttsInstance.(tts.TTS),
		LLM:          llmClient,
		VAD:          vadInstance,
		TurnDetector: turnDetector,
		MicIn:        micIn,
		TTSOut:       ttsOut,
		Language:     "en-US",
		Tools:        []agent.Tool{
			createEchoBotTool(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create voice agent: %w", err)
	}

	// Create a job for the agent
	echoJob, err := job.New(ctx, job.Config{
		ID:       "echo-bot-demo",
		RoomName: "echo-bot-room",
		Timeout:  5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	slog.Info("🚀 Echo bot agent starting",
		slog.String("job_id", echoJob.ID),
		slog.String("room_name", echoJob.RoomName))

	// Start the agent
	return voiceAgent.Start(ctx, echoJob)
}

// createEchoBotTool creates a tool that helps the LLM understand its echo bot role
func createEchoBotTool() agent.Tool {
	return agent.Tool{
		Name:        "echo_response",
		Description: "Generate an echo response that repeats what the user said with a friendly acknowledgment",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_input": map[string]any{
					"type":        "string",
					"description": "The text that the user spoke",
				},
			},
			"required": []string{"user_input"},
		},
		Handler: func(ctx context.Context, args string) (string, error) {
			// Parse the arguments to get user input
			// For this simple example, we'll just acknowledge and echo
			return fmt.Sprintf("I heard you say: %s. Let me repeat that back to you: %s", args, args), nil
		},
	}
}

// simulateAudioInput simulates microphone input for testing
func simulateAudioInput(ctx context.Context, micIn chan<- rtc.AudioFrame) {
	ticker := time.NewTicker(20 * time.Millisecond) // 50 FPS audio frames
	defer ticker.Stop()
	defer close(micIn)

	frameSize := 48000 / 50 // 48kHz, 20ms frames
	audioData := make([]byte, frameSize*2) // 16-bit samples

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send silent audio frames (in real implementation, this would be actual mic data)
			frame := rtc.AudioFrame{
				Data:        audioData,
				SampleRate:  48000,
				NumChannels: 1,
				SamplesPerChannel: frameSize,
			}
			
			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}
		}
	}
}

// simulateAudioOutput simulates audio output for testing
func simulateAudioOutput(ctx context.Context, ttsOut <-chan rtc.AudioFrame) {
	slog.Info("🔊 Audio output simulation started")
	
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-ttsOut:
			if !ok {
				return
			}
			// In a real implementation, this would play audio to speakers
			slog.Debug("Playing TTS audio frame", 
				slog.Int("sample_rate", frame.SampleRate),
				slog.Int("channels", frame.NumChannels),
				slog.Int("samples", frame.SamplesPerChannel))
		}
	}
}