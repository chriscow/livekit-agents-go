package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins/silero"
)

// NewVADCmd creates the VAD testing command
func NewVADCmd() *cobra.Command {
	var (
		duration      time.Duration
		threshold     float64
		showActivity  bool
		audioFile     string
		sampleRate    int
	)

	cmd := &cobra.Command{
		Use:   "vad",
		Short: "Test VAD (Voice Activity Detection) using Silero",
		Long: `Test Voice Activity Detection using the Silero VAD plugin.

This command tests the VAD service that detects when speech is present in audio.
VAD is the first step in the voice pipeline and critical for proper turn detection.

Examples:
  pipeline-test vad --duration 30s                    # Test VAD for 30 seconds
  pipeline-test vad --threshold 0.7 --show-activity  # Test with higher threshold
  pipeline-test vad --audio-file speech.wav          # Test with audio file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if audioFile != "" {
				return runVADFileTest(audioFile, threshold)
			}
			return runVADMicrophoneTest(duration, threshold, showActivity, sampleRate)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 30*time.Second, "test duration")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 0.5, "VAD activation threshold (0.0-1.0)")
	cmd.Flags().BoolVarP(&showActivity, "show-activity", "a", true, "show real-time VAD activity")
	cmd.Flags().StringVarP(&audioFile, "audio-file", "f", "", "test with audio file instead of microphone")
	cmd.Flags().IntVar(&sampleRate, "sample-rate", 16000, "sample rate for VAD (8000 or 16000)")

	return cmd
}

// runVADMicrophoneTest tests VAD with live microphone input
func runVADMicrophoneTest(duration time.Duration, threshold float64, showActivity bool, sampleRate int) error {
	fmt.Printf("🎤 Starting VAD test for %v...\n", duration)
	fmt.Printf("🎯 Threshold: %.2f\n", threshold)
	fmt.Printf("📊 Sample Rate: %d Hz\n", sampleRate)
	fmt.Println("🗣️  Speak into the microphone to test speech detection!")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	// Load Silero VAD
	fmt.Println("🔧 Loading Silero VAD...")
	vadService, err := silero.LoadDefaultSileroVAD()
	if err != nil {
		return fmt.Errorf("failed to load Silero VAD: %w", err)
	}
	defer vadService.Close()

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

	fmt.Println("🔴 VAD active - speak now!")
	fmt.Println("Press Ctrl+C to stop early")

	// Set up timer and stats
	timer := time.After(duration)
	frameCount := 0
	speechFrames := 0
	lastDetectionTime := time.Now()
	currentSpeaking := false

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n⏹️  Test stopped: %v\n", ctx.Err())
			return nil
		case <-timer:
			speechPercent := float64(speechFrames) / float64(frameCount) * 100
			fmt.Printf("\n⏹️  VAD test completed.\n")
			fmt.Printf("📊 Stats: %d frames processed, %.1f%% contained speech\n", frameCount, speechPercent)
			return nil
		case frame, ok := <-inputChan:
			if !ok {
				return fmt.Errorf("audio input channel closed unexpectedly")
			}

			frameCount++

			// Resample audio if needed for VAD (VAD expects 16kHz)
			vadFrame := frame
			if frame.Format.SampleRate != sampleRate {
				resampledFrame, err := media.ResampleAudioFrame(frame, sampleRate)
				if err != nil {
					if showActivity && frameCount%50 == 0 {
						fmt.Printf("⚠️  Resampling error (frame %d): %v\n", frameCount, err)
					}
					continue
				}
				vadFrame = resampledFrame
			}

			// Run VAD detection
			detection, err := vadService.Detect(ctx, vadFrame)
			if err != nil {
				if showActivity && frameCount%50 == 0 {
					fmt.Printf("⚠️  VAD error (frame %d): %v\n", frameCount, err)
				}
				continue
			}

			// Track speech statistics
			if detection.IsSpeech {
				speechFrames++
			}

			// Show activity if requested
			if showActivity {
				// Show state changes
				if detection.IsSpeech && !currentSpeaking {
					fmt.Printf("🗣️  SPEECH STARTED (prob: %.3f, frame: %d)\n", detection.Probability, frameCount)
					currentSpeaking = true
					lastDetectionTime = time.Now()
				} else if !detection.IsSpeech && currentSpeaking {
					duration := time.Since(lastDetectionTime)
					fmt.Printf("🤫 SPEECH ENDED (duration: %v, frame: %d)\n", duration.Round(time.Millisecond*100), frameCount)
					currentSpeaking = false
				}

				// Show periodic status
				if frameCount%100 == 0 {
					speechPercent := float64(speechFrames) / float64(frameCount) * 100
					status := "🔇 Silent"
					if currentSpeaking {
						status = "🗣️  Speaking"
					}
					fmt.Printf("📊 Frame %d: %s (%.1f%% speech so far, prob: %.3f)\n", 
						frameCount, status, speechPercent, detection.Probability)
				}
			}
		}
	}
}

// runVADFileTest tests VAD with an audio file
func runVADFileTest(filename string, threshold float64) error {
	fmt.Printf("📂 Testing VAD with audio file: %s\n", filename)
	fmt.Printf("🎯 Threshold: %.2f\n", threshold)
	fmt.Println()

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("audio file not found: %s", filename)
	}

	// TODO: Implement audio file loading and processing
	// This would require audio file decoding (WAV, MP3, etc.)
	fmt.Println("📝 Audio file testing not yet implemented.")
	fmt.Println("Use microphone testing for now: pipeline-test vad --duration 30s")
	
	return nil
}