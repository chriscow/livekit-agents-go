package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"livekit-agents-go/agents"
	"livekit-agents-go/plugins"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/openai"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/media"
)

// STTTTSDemo demonstrates standalone Speech-to-Text and Text-to-Speech usage
type STTTTSDemo struct {
	*agents.BaseAgent
}

// NewSTTTTSDemo creates a new STT/TTS demo
func NewSTTTTSDemo() *STTTTSDemo {
	return &STTTTSDemo{
		BaseAgent: agents.NewBaseAgent("STTTTSDemo"),
	}
}

// Start demonstrates STT and TTS service usage
func (d *STTTTSDemo) Start(ctx context.Context, session *agents.AgentSession) error {
	log.Println("🎙️ STT/TTS Demo starting...")

	// Create smart services using the plugin system
	services, err := plugins.CreateSmartServices()
	if err != nil {
		log.Printf("❌ Failed to create services: %v", err)
		return err
	}

	log.Printf("Services initialized for STT/TTS demo:")
	if services.STT != nil {
		log.Printf("  STT: %s v%s", services.STT.Name(), services.STT.Version())
	}
	if services.TTS != nil {
		log.Printf("  TTS: %s v%s", services.TTS.Name(), services.TTS.Version())
	}

	// Run demonstrations
	log.Println("\n=== STT/TTS Service Demonstrations ===")
	
	if services.STT != nil {
		if err := d.demonstrateSTT(ctx, services.STT); err != nil {
			return err
		}
		if err := d.demonstrateSTTStreaming(ctx, services.STT); err != nil {
			return err
		}
	} else {
		log.Println("⚠️ No STT service available, skipping STT demos")
	}

	if services.TTS != nil {
		if err := d.demonstrateTTS(ctx, services.TTS); err != nil {
			return err
		}
		if err := d.demonstrateTTSStreaming(ctx, services.TTS); err != nil {
			return err
		}
	} else {
		log.Println("⚠️ No TTS service available, skipping TTS demos")
	}

	log.Println("\n🎉 STT/TTS Demo completed successfully!")
	return nil
}


// demonstrateSTT shows basic speech-to-text usage
func (d *STTTTSDemo) demonstrateSTT(ctx context.Context, sttService stt.STT) error {
	log.Println("\n📝 === STT (Speech-to-Text) Demo ===")

	// Test different audio formats and sizes
	// Note: OpenAI Whisper requires minimum 0.1 seconds of audio
	testCases := []struct {
		name   string
		size   int
		format media.AudioFormat
	}{
		{"Short audio (0.15s)", 4800, media.AudioFormat16kHz16BitMono},   // 0.15s at 16kHz mono
		{"Medium audio (0.2s)", 19200, media.AudioFormat48kHz16BitMono},  // 0.2s at 48kHz mono
		{"Long audio (0.25s)", 48000, media.AudioFormat48kHz16BitStereo}, // 0.25s at 48kHz stereo
	}

	for i, tc := range testCases {
		log.Printf("\n📥 STT Test %d: %s (%d bytes, %s)", 
			i+1, tc.name, tc.size, formatDescription(tc.format))

		// Create test audio frame
		audioFrame := media.NewAudioFrame(make([]byte, tc.size), tc.format)
		log.Printf("   Audio frame: %v", audioFrame)

		// Perform speech recognition
		start := time.Now()
		recognition, err := sttService.Recognize(ctx, audioFrame)
		duration := time.Since(start)

		if err != nil {
			log.Printf("❌ STT error: %v", err)
			continue
		}

		log.Printf("✅ STT result: '%s'", recognition.Text)
		log.Printf("   Confidence: %.2f%%, Language: %s, Final: %t", 
			recognition.Confidence*100, recognition.Language, recognition.IsFinal)
		log.Printf("   Processing time: %v", duration)
	}

	return nil
}

// demonstrateTTS shows basic text-to-speech usage
func (d *STTTTSDemo) demonstrateTTS(ctx context.Context, ttsService tts.TTS) error {
	log.Println("\n🔊 === TTS (Text-to-Speech) Demo ===")

	// Test different text inputs
	testTexts := []string{
		"Hello world!",
		"This is a longer sentence that should generate more audio data.",
		"LiveKit agents provide powerful voice capabilities for developers.",
		"Speech synthesis technology converts text into natural-sounding speech.",
	}

	for i, text := range testTexts {
		log.Printf("\n🗣️  TTS Test %d: '%s'", i+1, text)

		// Perform text-to-speech synthesis
		start := time.Now()
		audioFrame, err := ttsService.Synthesize(ctx, text, nil)
		duration := time.Since(start)

		if err != nil {
			log.Printf("❌ TTS error: %v", err)
			continue
		}

		log.Printf("✅ TTS result: %d bytes audio", len(audioFrame.Data))
		log.Printf("   Duration: %v, Format: %s", 
			audioFrame.Duration, formatDescription(audioFrame.Format))
		log.Printf("   Processing time: %v", duration)
		log.Printf("   Metadata: %v", audioFrame.Metadata)
	}

	return nil
}

// demonstrateSTTStreaming shows streaming speech-to-text
func (d *STTTTSDemo) demonstrateSTTStreaming(ctx context.Context, sttService stt.STT) error {
	log.Println("\n🎤 === STT Streaming Demo ===")

	// Create streaming recognition session
	stream, err := sttService.RecognizeStream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	log.Println("📡 Created STT stream")

	// Send multiple audio frames (each ~50ms, combined will be ~200ms which exceeds OpenAI's 100ms minimum)
	audioFrames := []int{4800, 4800, 4800, 4800} // Each frame is ~50ms at 48kHz mono
	
	for i, size := range audioFrames {
		audioFrame := media.NewAudioFrame(make([]byte, size), media.AudioFormat48kHz16BitMono)
		
		log.Printf("📤 Sending audio frame %d: %d bytes", i+1, size)
		if err := stream.SendAudio(audioFrame); err != nil {
			log.Printf("❌ Send error: %v", err)
			continue
		}
		
		// Small delay between frames
		time.Sleep(50 * time.Millisecond)
	}

	// Close sending to trigger final processing
	if err := stream.CloseSend(); err != nil {
		log.Printf("❌ CloseSend error: %v", err)
		return err
	}

	// Receive streaming results
	log.Println("📥 Receiving streaming results...")
	resultCount := 0
	for {
		recognition, err := stream.Recv()
		if err != nil {
			log.Printf("🔚 Stream ended: %v", err)
			break
		}
		
		resultCount++
		log.Printf("✅ Stream result %d: '%s' (confidence: %.2f%%)", 
			resultCount, recognition.Text, recognition.Confidence*100)
		
		if resultCount >= 3 { // Limit output
			break
		}
	}

	log.Printf("📊 Received %d streaming results", resultCount)
	return nil
}

// demonstrateTTSStreaming shows streaming text-to-speech
func (d *STTTTSDemo) demonstrateTTSStreaming(ctx context.Context, ttsService tts.TTS) error {
	log.Println("\n🔊 === TTS Streaming Demo ===")

	// Create streaming synthesis session
	stream, err := ttsService.SynthesizeStream(ctx, nil)
	if err != nil {
		return err
	}
	defer stream.Close()

	log.Println("📡 Created TTS stream")

	// Send text chunks for streaming synthesis
	textChunks := []string{
		"Hello there!",
		"This is streaming text-to-speech.",
		"Each chunk generates audio.",
		"Final chunk complete.",
	}

	// Send text chunks
	for i, text := range textChunks {
		log.Printf("📤 Sending text chunk %d: '%s'", i+1, text)
		if err := stream.SendText(text); err != nil {
			log.Printf("❌ Send error: %v", err)
			continue
		}
		
		// Small delay between chunks
		time.Sleep(30 * time.Millisecond)
	}

	// Close sending
	if err := stream.CloseSend(); err != nil {
		log.Printf("❌ CloseSend error: %v", err)
		return err
	}

	// Receive streaming audio
	log.Println("📥 Receiving streaming audio...")
	audioCount := 0
	totalAudioBytes := 0
	
	for {
		audioFrame, err := stream.Recv()
		if err != nil {
			log.Printf("🔚 Stream ended: %v", err)
			break
		}
		
		audioCount++
		totalAudioBytes += len(audioFrame.Data)
		log.Printf("✅ Audio chunk %d: %d bytes, duration: %v", 
			audioCount, len(audioFrame.Data), audioFrame.Duration)
		
		if audioCount >= 4 { // Limit output
			break
		}
	}

	log.Printf("📊 Received %d audio chunks, %d total bytes", audioCount, totalAudioBytes)
	return nil
}

// formatDescription returns a human-readable description of audio format
func formatDescription(format media.AudioFormat) string {
	channels := "mono"
	if format.Channels > 1 {
		channels = "stereo"
	}
	
	return fmt.Sprintf("%dHz %dbit %s", 
		format.SampleRate, format.BitsPerSample, channels)
}

// HandleEvent handles agent events
func (d *STTTTSDemo) HandleEvent(event agents.AgentEvent) error {
	// STT/TTS demo doesn't need event handling
	return nil
}

// entrypoint is the main demo entrypoint function
func entrypoint(ctx *agents.JobContext) error {
	log.Printf("🚀 STT/TTS Demo starting...")

	// Create and start the demo
	demo := NewSTTTTSDemo()
	
	// Create a session
	session := agents.NewAgentSession(ctx.Context)
	session.Agent = demo

	return session.Start()
}

func main() {
	// Configure worker options
	opts := &agents.WorkerOptions{
		EntrypointFunc: entrypoint,
		AgentName:      "STTTTSDemo",
		APIKey:         os.Getenv("LIVEKIT_API_KEY"),
		APISecret:      os.Getenv("LIVEKIT_API_SECRET"),
		Host:           os.Getenv("LIVEKIT_HOST"),
		LiveKitURL:     os.Getenv("LIVEKIT_URL"),
		Metadata: map[string]string{
			"description": "Demonstration of STT and TTS services",
			"version":     "1.0.0",
			"type":        "stt-tts-demo",
			"services":    "speech-to-text,text-to-speech",
		},
	}

	// Set defaults for development
	if opts.Host == "" {
		opts.Host = "localhost:7880"
	}
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7880"
	}

	log.Printf("🎙️ Starting STT/TTS Demo: %s", opts.AgentName)
	log.Printf("🌐 LiveKit Host: %s", opts.Host)
	log.Printf("🔗 LiveKit URL: %s", opts.LiveKitURL)

	// Run with CLI
	if err := agents.RunApp(opts); err != nil {
		log.Fatal("❌ Failed to run demo:", err)
	}
}