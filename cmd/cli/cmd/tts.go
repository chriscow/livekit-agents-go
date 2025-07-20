package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/tts"
	// Import plugins to register their delegates
	_ "livekit-agents-go/plugins/deepgram"
	_ "livekit-agents-go/plugins/openai"
)

// NewTTSCmd creates the TTS testing command
func NewTTSCmd() *cobra.Command {
	var (
		text           string
		voice          string
		speed          float64
		output         string
		play           bool
		streaming      bool
		testLargeFrame bool
	)

	cmd := &cobra.Command{
		Use:   "tts",
		Short: "Test TTS (Text-to-Speech) using available services",
		Long: `Test Text-to-Speech synthesis using available TTS services.

This command tests the TTS service that converts text into spoken audio.
It supports both batch and streaming synthesis modes.
TTS is the final step in the voice pipeline after LLM response generation.

Examples:
  pipeline-test tts --text "Hello world"                    # Basic synthesis and playback
  pipeline-test tts --voice alloy --speed 1.2              # Custom voice and speed
  pipeline-test tts --output audio.wav --text "Test"       # Save to file
  pipeline-test tts --streaming --text "Hello. How are you?" # Test streaming synthesis`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate API key
			if os.Getenv("OPENAI_API_KEY") == "" {
				return fmt.Errorf("OPENAI_API_KEY environment variable required for TTS testing")
			}

			if streaming {
				return runTTSStreamingTest(text, voice, speed, play)
			}
			return runTTSTest(text, voice, speed, output, play, testLargeFrame)
		},
	}

	cmd.Flags().StringVarP(&text, "text", "t", "Hello! This is a test of the text-to-speech system.", "text to synthesize")
	cmd.Flags().StringVar(&voice, "voice", "alloy", "voice to use (alloy, echo, fable, onyx, nova, shimmer)")
	cmd.Flags().Float64Var(&speed, "speed", 1.0, "speech speed (0.25-4.0)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file (optional)")
	cmd.Flags().BoolVarP(&play, "play", "p", true, "play audio after synthesis")
	cmd.Flags().BoolVar(&streaming, "streaming", false, "test streaming TTS synthesis")
	cmd.Flags().BoolVar(&testLargeFrame, "test-large-frame", false, "test sending entire audio frame at once (like basic-agent)")

	return cmd
}

// runTTSTest tests the TTS service incrementally
// NOTE: Batch TTS produces audio that sounds "insanely fast" according to user feedback.
// User confirmed to ignore batch implementation and focus on streaming TTS which sounds correct.
func runTTSTest(text, voice string, speed float64, outputFile string, play bool, testLargeFrame bool) error {
	fmt.Printf("🔊 TTS Step 1A: Testing basic TTS service integration\n")
	fmt.Printf("📝 Text: %s\n", text)
	fmt.Printf("🎭 Voice: %s\n", voice)
	fmt.Printf("⚡ Speed: %.1fx\n", speed)
	if outputFile != "" {
		fmt.Printf("💾 Output file: %s\n", outputFile)
	}
	fmt.Printf("🔊 Play audio: %v\n", play)
	fmt.Println()

	// Step 1A: Can we create TTS service at all?
	return runTTSStep1A(text, voice, speed, outputFile, play, testLargeFrame)
}

// runTTSStep1A - Basic TTS service integration
func runTTSStep1A(text, voice string, speed float64, outputFile string, play bool, testLargeFrame bool) error {
	fmt.Println("🔧 Step 1A: Creating TTS service...")
	
	// Try to create services using the same pattern as agents
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("❌ Failed to create services: %w", err)
	}

	if services.TTS == nil {
		return fmt.Errorf("❌ TTS service not available - check OPENAI_API_KEY")
	}

	fmt.Printf("✅ TTS service created successfully: %s v%s\n", services.TTS.Name(), services.TTS.Version())
	
	// List available voices
	voices := services.TTS.Voices()
	fmt.Printf("🎭 Available voices (%d):\n", len(voices))
	for _, v := range voices {
		marker := "  "
		if v.ID == voice {
			marker = "→ "
		}
		fmt.Printf("%s%s (%s, %s)\n", marker, v.Name, v.ID, v.Gender)
	}
	
	fmt.Println("\n✅ Step 1A PASSED: TTS service integration successful")
	fmt.Println("🎯 Next: Proceeding to Step 1B (audio synthesis)")
	fmt.Println()
	
	// Automatically proceed to Step 1B
	return runTTSStep1B(services.TTS, text, voice, speed, outputFile, play, testLargeFrame)
}

// runTTSStep1B - TTS text→audio conversion
func runTTSStep1B(ttsService tts.TTS, text, voice string, speed float64, outputFile string, play bool, testLargeFrame bool) error {
	fmt.Println("🔧 Step 1B: Testing TTS audio synthesis...")
	fmt.Printf("📝 Synthesizing: \"%s\"\n", text)
	
	// Create synthesis options
	opts := tts.DefaultSynthesizeOptions()
	opts.Voice = voice
	opts.Speed = speed
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Println("⏳ Calling OpenAI TTS API...")
	start := time.Now()
	
	// Call TTS synthesis
	audioFrame, err := ttsService.Synthesize(ctx, text, opts)
	if err != nil {
		return fmt.Errorf("❌ TTS synthesis failed: %w", err)
	}
	
	duration := time.Since(start)
	
	if audioFrame == nil {
		return fmt.Errorf("❌ TTS returned nil audio frame")
	}
	
	if len(audioFrame.Data) == 0 {
		return fmt.Errorf("❌ TTS returned empty audio data")
	}
	
	// Calculate audio metrics
	audioSeconds := float64(len(audioFrame.Data)) / 
		float64(audioFrame.Format.SampleRate * audioFrame.Format.Channels * audioFrame.Format.BitsPerSample / 8)
	
	fmt.Printf("✅ TTS synthesis successful!\n")
	fmt.Printf("🎵 Audio format: %d Hz, %d-bit, %d channels\n", 
		audioFrame.Format.SampleRate, audioFrame.Format.BitsPerSample, audioFrame.Format.Channels)
	fmt.Printf("📊 Audio data: %d bytes (%.2f seconds)\n", len(audioFrame.Data), audioSeconds)
	fmt.Printf("⏱️  API response time: %v\n", duration)
	fmt.Printf("🏃‍♂️ Realtime factor: %.2fx (%.2fs processing / %.2fs audio)\n", 
		duration.Seconds()/audioSeconds, duration.Seconds(), audioSeconds)
	
	// Save to file if requested
	if outputFile != "" {
		if err := saveAudioToFile(audioFrame, outputFile); err != nil {
			fmt.Printf("⚠️  Failed to save audio file: %v\n", err)
		} else {
			fmt.Printf("💾 Audio saved to: %s\n", outputFile)
		}
	}
	
	fmt.Println("\n✅ Step 1B PASSED: TTS audio synthesis successful")
	if play {
		fmt.Println("🎯 Next: Proceeding to Step 1C (audio playback)")
		fmt.Println()
		return runTTSStep1C(audioFrame, play, testLargeFrame)
	} else {
		fmt.Println("🎯 Use --play flag to test audio output (Step 1C)")
	}
	
	return nil
}

// saveAudioToFile saves audio frame to a file (simple WAV format)
func saveAudioToFile(audioFrame *media.AudioFrame, filename string) error {
	// TODO: Implement proper WAV file writing
	// For now, just save raw PCM data
	return os.WriteFile(filename, audioFrame.Data, 0644)
}

// runTTSStep1C - Audio output using working audio-test pattern
func runTTSStep1C(audioFrame *media.AudioFrame, play bool, testLargeFrame bool) error {
	if !play {
		fmt.Println("🔇 Skipping audio playback (--play=false)")
		return nil
	}
	
	fmt.Println("🔧 Step 1C: Testing audio output...")
	fmt.Printf("🎵 Playing %d bytes of %d Hz audio\n", len(audioFrame.Data), audioFrame.Format.SampleRate)
	
	// Create audio I/O using the same pattern as audio-test
	audioIO, err := audio.NewLocalAudioIO(audio.DefaultConfig())
	if err != nil {
		return fmt.Errorf("❌ Failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()
	
	// Create context for audio playback
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Start audio I/O
	fmt.Println("🚀 Starting audio I/O...")
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("❌ Failed to start audio I/O: %w", err)
	}
	
	defer func() {
		fmt.Println("🛑 Stopping audio I/O...")
		if err := audioIO.Stop(); err != nil {
			fmt.Printf("⚠️  Error stopping audio I/O: %v\n", err)
		}
	}()
	
	outputChan := audioIO.OutputChan()
	
	fmt.Println("🔊 Playing TTS audio...")
	
	if testLargeFrame {
		// TEST MODE: Send entire frame at once (like basic-agent) to validate hypothesis
		fmt.Printf("🧪 TEST MODE: Sending entire frame (%d bytes) at once to validate chunking hypothesis\n", len(audioFrame.Data))
		
		select {
		case outputChan <- audioFrame:
			fmt.Printf("🎵 Sent entire frame: %d bytes (testing basic-agent behavior)\n", len(audioFrame.Data))
		case <-time.After(10 * time.Second):
			return fmt.Errorf("❌ Audio output timeout - output channel not ready")
		case <-ctx.Done():
			return fmt.Errorf("❌ Audio playback cancelled: %w", ctx.Err())
		}
	} else {
		// NORMAL MODE: Send entire frame at once (matches production behavior in agents/console.go:305)
		fmt.Printf("✅ NORMAL MODE: Sending complete frame (%d bytes) - matches production agent behavior\n", len(audioFrame.Data))
		
		select {
		case outputChan <- audioFrame:
			fmt.Printf("🎵 Sent complete frame: %d bytes (production behavior)\n", len(audioFrame.Data))
		case <-time.After(10 * time.Second):
			return fmt.Errorf("❌ Audio output timeout - output channel not ready")
		case <-ctx.Done():
			return fmt.Errorf("❌ Audio playback cancelled: %w", ctx.Err())
		}
	}
	
	// Wait a bit for the audio to finish playing
	audioSeconds := float64(len(audioFrame.Data)) / 
		float64(audioFrame.Format.SampleRate * audioFrame.Format.Channels * audioFrame.Format.BitsPerSample / 8)
	waitTime := time.Duration(audioSeconds*1000+500) * time.Millisecond // Audio duration + 500ms buffer
	
	fmt.Printf("⏳ Waiting %.1fs for audio to finish playing...\n", waitTime.Seconds())
	time.Sleep(waitTime)
	
	fmt.Println("\n✅ Step 1C PASSED: TTS audio output successful")
	fmt.Println("🎉 All TTS tests completed successfully!")
	
	return nil
}

// runTTSStreamingTest tests streaming TTS synthesis
func runTTSStreamingTest(text, voice string, speed float64, play bool) error {
	fmt.Printf("🌊 Testing TTS streaming synthesis\n")
	fmt.Printf("📝 Text: %s\n", text)
	fmt.Printf("🎭 Voice: %s\n", voice)
	fmt.Printf("⚡ Speed: %.1fx\n", speed)
	fmt.Printf("🔊 Play audio: %v\n", play)
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create TTS service
	fmt.Println("🔧 Creating TTS service...")
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	if services.TTS == nil {
		return fmt.Errorf("TTS service not available - check OPENAI_API_KEY")
	}

	fmt.Printf("✅ TTS service: %s v%s\n", services.TTS.Name(), services.TTS.Version())

	// Create streaming synthesis session
	opts := &tts.SynthesizeOptions{
		Voice: voice,
		Speed: float64(speed),
	}

	stream, err := services.TTS.SynthesizeStream(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to create TTS stream: %w", err)
	}
	defer stream.Close()

	fmt.Printf("✅ Created TTS streaming session\n")

	// Set up audio I/O if playing
	var audioIO *audio.LocalAudioIO
	var outputChan chan<- *media.AudioFrame
	
	if play {
		audioIO, err = audio.NewLocalAudioIO(audio.DefaultConfig())
		if err != nil {
			return fmt.Errorf("failed to create audio I/O: %w", err)
		}
		defer audioIO.Close()

		if err := audioIO.Start(ctx); err != nil {
			return fmt.Errorf("failed to start audio I/O: %w", err)
		}

		defer func() {
			fmt.Println("🛑 Stopping audio I/O...")
			if err := audioIO.Stop(); err != nil {
				fmt.Printf("⚠️ Error stopping audio I/O: %v\n", err)
			}
		}()

		outputChan = audioIO.OutputChan()
		fmt.Println("🚀 Audio I/O started")
	}

	// Start goroutine to handle audio results
	resultCount := 0
	go func() {
		for {
			audioFrame, err := stream.Recv()
			if err != nil {
				if err.Error() != "EOF" {
					fmt.Printf("❌ TTS stream error: %v\n", err)
				}
				return
			}

			resultCount++
			fmt.Printf("🎯 Received audio chunk #%d: %d bytes\n", resultCount, len(audioFrame.Data))

			if play && outputChan != nil {
				// Send audio to output for playback
				select {
				case outputChan <- audioFrame:
					fmt.Printf("🔊 Playing audio chunk #%d\n", resultCount)
				case <-ctx.Done():
					return
				default:
					fmt.Printf("⚠️ Audio output busy, skipping chunk #%d\n", resultCount)
				}
			}
		}
	}()

	// Split text into sentences and send them progressively
	sentences := splitTextIntoSentences(text)
	fmt.Printf("📝 Split text into %d sentences\n", len(sentences))

	for i, sentence := range sentences {
		fmt.Printf("📤 Sending sentence %d/%d: \"%s\"\n", i+1, len(sentences), sentence)
		
		if err := stream.SendText(sentence); err != nil {
			return fmt.Errorf("failed to send text to TTS stream: %w", err)
		}

		// Small delay to see streaming effect
		time.Sleep(500 * time.Millisecond)
	}

	// Signal end of text
	fmt.Println("🔚 Signaling end of text...")
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("failed to close TTS send: %w", err)
	}

	// Wait for all audio to be processed
	fmt.Println("⏳ Waiting for streaming synthesis to complete...")
	time.Sleep(5 * time.Second)

	fmt.Printf("✅ Streaming TTS test completed - processed %d audio chunks\n", resultCount)
	return nil
}

// splitTextIntoSentences is a simple sentence splitter for TTS streaming demo
func splitTextIntoSentences(text string) []string {
	if text == "" {
		return nil
	}
	
	sentences := make([]string, 0)
	current := ""
	
	for i, char := range text {
		current += string(char)
		
		if char == '.' || char == '!' || char == '?' {
			if i == len(text)-1 || (i < len(text)-1 && text[i+1] == ' ') {
				sentence := strings.TrimSpace(current)
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current = ""
			}
		}
	}
	
	if current != "" {
		sentence := strings.TrimSpace(current)
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}
	
	if len(sentences) == 0 {
		return []string{text}
	}
	
	return sentences
}