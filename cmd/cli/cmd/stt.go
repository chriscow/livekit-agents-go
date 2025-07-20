package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/stt"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/deepgram"
	_ "livekit-agents-go/plugins/openai"
)

// NewSTTCmd creates the STT testing command
func NewSTTCmd() *cobra.Command {
	var (
		duration   time.Duration
		model      string
		language   string
		audioFile  string
		continuous bool
	)

	cmd := &cobra.Command{
		Use:   "stt",
		Short: "Test STT (Speech-to-Text) using available services (Deepgram/Whisper)",
		Long: `Test Speech-to-Text transcription using available services.

This command tests the STT service that converts speech audio into text.
It will automatically use the best available STT service (Deepgram streaming preferred).
STT is the second step in the voice pipeline after VAD detection.

Examples:
  pipeline-test stt --duration 30s                      # Record and transcribe for 30s
  pipeline-test stt --continuous --duration 60s         # Continuous transcription  
  pipeline-test stt --model deepgram --language en      # Use specific service
  pipeline-test stt --audio-file speech.wav             # Transcribe audio file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for any STT API keys
			hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""
			hasDeepgram := os.Getenv("DEEPGRAM_API_KEY") != ""
			
			if !hasOpenAI && !hasDeepgram {
				return fmt.Errorf("either OPENAI_API_KEY or DEEPGRAM_API_KEY environment variable required for STT testing")
			}

			if audioFile != "" {
				return runSTTFileTest(audioFile, model, language)
			}
			return runSTTMicrophoneTest(duration, model, language, continuous)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 30*time.Second, "recording duration")
	cmd.Flags().StringVarP(&model, "model", "m", "", "STT service to use (auto-detects if not specified)")
	cmd.Flags().StringVarP(&language, "language", "l", "en", "language hint for transcription")
	cmd.Flags().StringVarP(&audioFile, "audio-file", "f", "", "transcribe audio file instead of microphone")
	cmd.Flags().BoolVarP(&continuous, "continuous", "c", false, "continuous transcription (multiple chunks)")

	return cmd
}

// runSTTMicrophoneTest tests STT with live microphone input
func runSTTMicrophoneTest(duration time.Duration, model, language string, continuous bool) error {
	fmt.Printf("🎤 Starting STT test for %v...\n", duration)
	fmt.Printf("🤖 Model: %s\n", model)
	fmt.Printf("🌍 Language: %s\n", language)
	if continuous {
		fmt.Println("🔄 Mode: Continuous transcription")
	} else {
		fmt.Println("🎯 Mode: Single transcription at end")
	}
	fmt.Println("🗣️  Speak clearly into the microphone!")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), duration+30*time.Second)
	defer cancel()

	// Create STT service
	fmt.Println("🔧 Creating STT service...")
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	if services.STT == nil {
		return fmt.Errorf("STT service not available - check API keys")
	}

	fmt.Printf("✅ STT service: %s v%s\n", services.STT.Name(), services.STT.Version())

	// Create audio I/O
	audioIO, err := audio.NewLocalAudioIO(audio.DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()

	// Start audio I/O
	fmt.Println("🚀 Starting audio I/O...")
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O: %w", err)
	}

	defer func() {
		fmt.Println("\n🛑 Stopping audio I/O...")
		if err := audioIO.Stop(); err != nil {
			fmt.Printf("⚠️  Error stopping audio I/O: %v\n", err)
		}
		fmt.Println("✅ Audio I/O stopped")
	}()

	inputChan := audioIO.InputChan()

	if continuous {
		return runContinuousSTT(ctx, inputChan, services.STT, duration)
	} else {
		return runSingleSTT(ctx, inputChan, services.STT, duration)
	}
}

// runSingleSTT collects audio for the full duration then transcribes once
func runSingleSTT(ctx context.Context, inputChan <-chan *media.AudioFrame, sttService stt.STT, duration time.Duration) error {
	fmt.Println("🔴 Recording - speak now! (will transcribe at the end)")
	fmt.Println("Press Ctrl+C to stop early")

	timer := time.After(duration)
	var audioFrames []*media.AudioFrame
	frameCount := 0

	// Collect audio frames
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n⏹️  Recording stopped: %v\n", ctx.Err())
			if len(audioFrames) == 0 {
				return nil
			}
		case <-timer:
			fmt.Printf("\n⏹️  Recording completed (%d frames). Transcribing...\n", frameCount)
		case frame, ok := <-inputChan:
			if !ok {
				fmt.Println("\n⚠️  Audio input channel closed")
				break
			}

			frameCount++
			audioFrames = append(audioFrames, frame)

			// Show progress
			if frameCount%50 == 0 {
				energy := calculateFrameEnergy(frame)
				if energy > 0.001 {
					fmt.Printf("🎤 Recording... (frame %d, energy: %.3f)\n", frameCount, energy)
				} else if frameCount%200 == 0 {
					fmt.Printf("🔇 Recording... (frame %d)\n", frameCount)
				}
			}
			continue
		}
		break
	}

	if len(audioFrames) == 0 {
		fmt.Println("❌ No audio recorded")
		return nil
	}

	// Combine audio frames
	fmt.Printf("🔧 Combining %d audio frames...\n", len(audioFrames))
	combinedFrame := combineAudioFrames(audioFrames)
	if combinedFrame == nil {
		return fmt.Errorf("failed to combine audio frames")
	}

	fmt.Printf("📊 Audio duration: %.2f seconds\n", float64(len(combinedFrame.Data))/float64(combinedFrame.Format.SampleRate*2))

	// Transcribe the audio
	fmt.Println("🎯 Transcribing audio...")
	startTime := time.Now()
	
	result, err := sttService.Recognize(ctx, combinedFrame)
	if err != nil {
		return fmt.Errorf("STT transcription failed: %w", err)
	}
	
	transcriptionTime := time.Since(startTime)
	
	// Display results
	fmt.Printf("\n🎯 Transcription Results:\n")
	fmt.Printf("📝 Text: \"%s\"\n", result.Text)
	fmt.Printf("📊 Confidence: %.2f\n", result.Confidence)
	fmt.Printf("🌍 Language: %s\n", result.Language)
	fmt.Printf("⏱️ Transcription time: %v\n", transcriptionTime)
	fmt.Printf("✅ Final: %t\n", result.IsFinal)
	
	if len(result.Metadata) > 0 {
		fmt.Printf("📋 Metadata: %+v\n", result.Metadata)
	}

	return nil
}

// runContinuousSTT transcribes audio using streaming STT
func runContinuousSTT(ctx context.Context, inputChan <-chan *media.AudioFrame, sttService stt.STT, duration time.Duration) error {
	fmt.Println("🔴 Streaming transcription active - speak now!")
	fmt.Println("Press Ctrl+C to stop early")

	// Create streaming recognition session
	stream, err := sttService.RecognizeStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to create STT stream: %w", err)
	}
	defer stream.Close()

	fmt.Printf("✅ Created streaming STT session with %s\n", sttService.Name())

	// Start goroutine to handle transcription results
	resultsChan := make(chan *stt.Recognition, 10)
	errorsChan := make(chan error, 5)
	
	go func() {
		defer close(resultsChan)
		defer close(errorsChan)
		
		for {
			result, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("📭 STT stream ended")
				return
			}
			if err != nil {
				select {
				case errorsChan <- err:
				case <-ctx.Done():
					return
				}
				return
			}
			
			select {
			case resultsChan <- result:
			case <-ctx.Done():
				return
			}
		}
	}()

	timer := time.After(duration)
	frameCount := 0
	transcriptionCount := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n⏹️  Test stopped: %v\n", ctx.Err())
			stream.CloseSend()
			return nil
			
		case <-timer:
			fmt.Printf("\n⏹️  Streaming STT test completed. Received %d transcriptions.\n", transcriptionCount)
			stream.CloseSend()
			// Wait a bit for final results
			time.Sleep(2 * time.Second)
			return nil
			
		case err := <-errorsChan:
			return fmt.Errorf("STT streaming error: %w", err)
			
		case result := <-resultsChan:
			transcriptionCount++
			if result.Text != "" || result.IsFinal {
				fmt.Printf("🎯 #%d: \"%s\" (final: %t, confidence: %.2f)\n", 
					transcriptionCount, result.Text, result.IsFinal, result.Confidence)
			}
			
		case frame, ok := <-inputChan:
			if !ok {
				fmt.Println("⚠️  Audio input closed")
				stream.CloseSend()
				return nil
			}

			frameCount++
			
			// Send audio to STT stream
			if err := stream.SendAudio(frame); err != nil {
				return fmt.Errorf("failed to send audio to STT: %w", err)
			}

			// Show progress occasionally
			if frameCount%100 == 0 {
				energy := calculateFrameEnergy(frame)
				fmt.Printf("🎤 Streaming... (frame %d, energy: %.3f)\n", frameCount, energy)
			}
		}
	}
}

// runSTTFileTest tests STT with an audio file
func runSTTFileTest(filename, model, language string) error {
	fmt.Printf("📂 Testing STT with audio file: %s\n", filename)
	fmt.Printf("🤖 Model: %s\n", model)
	fmt.Printf("🌍 Language: %s\n", language)
	fmt.Println()

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("audio file not found: %s", filename)
	}

	// TODO: Implement audio file loading and transcription
	fmt.Println("📝 Audio file transcription not yet implemented.")
	fmt.Println("Use microphone testing for now: pipeline-test stt --duration 30s")
	
	return nil
}

// combineAudioFrames combines multiple audio frames into a single frame
func combineAudioFrames(frames []*media.AudioFrame) *media.AudioFrame {
	if len(frames) == 0 {
		return nil
	}

	// Calculate total size
	totalSize := 0
	format := frames[0].Format
	
	for _, frame := range frames {
		totalSize += len(frame.Data)
	}

	// Combine data
	combinedData := make([]byte, totalSize)
	offset := 0
	
	for _, frame := range frames {
		copy(combinedData[offset:], frame.Data)
		offset += len(frame.Data)
	}

	return media.NewAudioFrame(combinedData, format)
}