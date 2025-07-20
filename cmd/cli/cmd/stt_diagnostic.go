package cmd

import (
	"context"
	"fmt"
	"time"

	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"

	"github.com/spf13/cobra"
)

// NewSTTDiagnosticCmd creates the STT diagnostic command
func NewSTTDiagnosticCmd() *cobra.Command {
	var duration time.Duration
	
	cmd := &cobra.Command{
		Use:   "stt-diagnostic",
		Short: "Test STT service independently with different audio sources",
		Long: `Diagnostic tool to test STT (Speech-to-Text) service independently.
This command:
1. Tests STT with direct microphone input (no AEC)
2. Tests STT with AEC-processed audio
3. Compares recognition quality and identifies audio processing issues
4. Provides detailed audio quality metrics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSTTDiagnostic(duration)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 10*time.Second, "test duration")
	
	return cmd
}

func runSTTDiagnostic(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	fmt.Println("🧪 Starting STT Diagnostic Test...")

	// Initialize services
	fmt.Println("📋 Initializing services...")
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	if services.STT == nil {
		return fmt.Errorf("STT service not available")
	}

	fmt.Printf("✅ STT service: %s\n", services.STT.Name())

	// Test 1: Direct microphone input (no AEC)
	fmt.Println("\n=== TEST 1: Direct Microphone Input (No AEC) ===")
	if err := testDirectMicrophoneSTT(ctx, services); err != nil {
		fmt.Printf("❌ Direct microphone test failed: %v\n", err)
	}

	// Test 2: AEC-processed audio
	fmt.Println("\n=== TEST 2: AEC-Processed Audio ===")
	if err := testAECProcessedSTT(ctx, services); err != nil {
		fmt.Printf("❌ AEC-processed test failed: %v\n", err)
	}

	fmt.Println("\n✅ STT Diagnostic completed!")
	return nil
}

func testDirectMicrophoneSTT(ctx context.Context, services *plugins.SmartServices) error {
	fmt.Println("🎤 Testing STT with direct microphone input...")

	// Create basic audio I/O (no AEC)
	audioConfig := audio.Config{
		SampleRate:      48000,
		Channels:        1,
		BitDepth:        16,
		FramesPerBuffer: 1024,
	}
	audioIO, err := audio.NewLocalAudioIO(audioConfig)
	if err != nil {
		return fmt.Errorf("failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()

	// Start audio I/O
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O: %w", err)
	}
	defer audioIO.Stop()

	fmt.Println("🎙️  Listening for speech (5 seconds)... Please speak now!")

	// Collect audio for 5 seconds
	audioFrames := make([]*media.AudioFrame, 0)
	inputChan := audioIO.InputChan()
	timeout := time.After(5 * time.Second)

	for {
		select {
		case frame := <-inputChan:
			audioFrames = append(audioFrames, frame)
		case <-timeout:
			goto processAudio
		case <-ctx.Done():
			return ctx.Err()
		}
	}

processAudio:
	if len(audioFrames) == 0 {
		return fmt.Errorf("no audio frames captured")
	}

	// Combine frames
	combinedAudio := combineAudioFrames(audioFrames)
	if combinedAudio == nil {
		return fmt.Errorf("failed to combine audio frames")
	}

	fmt.Printf("📊 Captured %d frames (%d bytes total)\n", len(audioFrames), len(combinedAudio.Data))
	fmt.Printf("📊 Audio format: %+v\n", combinedAudio.Format)

	// Calculate audio quality metrics
	energy := calculateFrameEnergy(combinedAudio)
	fmt.Printf("📊 Audio energy: %.6f\n", energy)

	// Test STT recognition
	fmt.Println("🧠 Processing with STT service...")
	recognition, err := services.STT.Recognize(ctx, combinedAudio)
	if err != nil {
		return fmt.Errorf("STT recognition failed: %w", err)
	}

	fmt.Printf("📝 Direct STT Result: \"%s\" (confidence: %.2f)\n", recognition.Text, recognition.Confidence)

	if recognition.Text == "" {
		fmt.Println("⚠️  WARNING: Empty transcription from direct microphone input")
	}

	return nil
}

func testAECProcessedSTT(ctx context.Context, services *plugins.SmartServices) error {
	fmt.Println("🎵 Testing STT with AEC-processed audio...")

	// Create AEC pipeline
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 24000 // AEC sample rate

	aecPipeline, err := audio.NewAECPipeline(aecConfig)
	if err != nil {
		return fmt.Errorf("failed to create AEC pipeline: %w", err)
	}
	defer aecPipeline.Close()

	// Start AEC pipeline
	if err := aecPipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start AEC pipeline: %w", err)
	}
	defer aecPipeline.Stop()

	// Get AEC-processed input
	audioIO := aecPipeline.GetAudioIO()
	inputChan := audioIO.InputChan()

	fmt.Printf("🎙️  Listening for speech through AEC (5 seconds, %d Hz)... Please speak now!\n", aecConfig.SampleRate)

	// Collect AEC-processed audio for 5 seconds
	audioFrames := make([]*media.AudioFrame, 0)
	timeout := time.After(5 * time.Second)

	for {
		select {
		case frame := <-inputChan:
			audioFrames = append(audioFrames, frame)
		case <-timeout:
			goto processAECAudio
		case <-ctx.Done():
			return ctx.Err()
		}
	}

processAECAudio:
	if len(audioFrames) == 0 {
		return fmt.Errorf("no AEC-processed audio frames captured")
	}

	// Combine frames
	combinedAudio := combineAudioFrames(audioFrames)
	if combinedAudio == nil {
		return fmt.Errorf("failed to combine AEC audio frames")
	}

	fmt.Printf("📊 AEC captured %d frames (%d bytes total)\n", len(audioFrames), len(combinedAudio.Data))
	fmt.Printf("📊 AEC audio format: %+v\n", combinedAudio.Format)

	// Calculate audio quality metrics
	energy := calculateFrameEnergy(combinedAudio)
	fmt.Printf("📊 AEC audio energy: %.6f\n", energy)

	// Check if AEC is over-canceling (very low energy might indicate this)
	if energy < 0.00001 {
		fmt.Println("⚠️  WARNING: Very low audio energy detected - AEC may be over-canceling")
	}

	// Test sample rate conversion for Deepgram (24kHz -> 48kHz)
	fmt.Println("🔧 Converting sample rate for Deepgram (24kHz -> 48kHz)...")
	upsampled, err := upsampleAudio(combinedAudio, 48000)
	if err != nil {
		fmt.Printf("⚠️  Sample rate conversion failed: %v, using original\n", err)
		upsampled = combinedAudio
	} else {
		fmt.Printf("✅ Upsampled to %d Hz (%d bytes)\n", upsampled.Format.SampleRate, len(upsampled.Data))
	}

	// Test STT recognition with upsampled audio
	fmt.Println("🧠 Processing AEC audio with STT service...")
	recognition, err := services.STT.Recognize(ctx, upsampled)
	if err != nil {
		return fmt.Errorf("AEC STT recognition failed: %w", err)
	}

	fmt.Printf("📝 AEC STT Result: \"%s\" (confidence: %.2f)\n", recognition.Text, recognition.Confidence)

	if recognition.Text == "" {
		fmt.Println("⚠️  WARNING: Empty transcription from AEC-processed audio")
		fmt.Println("💡 Possible causes:")
		fmt.Println("   - AEC over-cancellation removing user's voice")
		fmt.Println("   - Sample rate mismatch (24kHz AEC vs 48kHz Deepgram)")
		fmt.Println("   - Audio quality degradation through AEC processing")
	}

	// Print AEC stats
	fmt.Println("\n📊 AEC Pipeline Statistics:")
	aecPipeline.PrintStats()

	return nil
}

func upsampleAudio(frame *media.AudioFrame, targetSampleRate int) (*media.AudioFrame, error) {
	if frame.Format.SampleRate == targetSampleRate {
		return frame, nil // No conversion needed
	}

	if frame.Format.SampleRate <= 0 || targetSampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rates: %d -> %d", frame.Format.SampleRate, targetSampleRate)
	}

	// Simple linear interpolation upsampling
	ratio := float64(targetSampleRate) / float64(frame.Format.SampleRate)
	inputSamples := len(frame.Data) / 2 // 16-bit samples
	outputSamples := int(float64(inputSamples) * ratio)

	outputData := make([]byte, outputSamples*2)

	for i := 0; i < outputSamples; i++ {
		// Calculate corresponding input sample position
		inputPos := float64(i) / ratio
		inputIndex := int(inputPos) * 2

		if inputIndex >= len(frame.Data)-2 {
			inputIndex = len(frame.Data) - 2
		}

		// Read input sample
		sample := int16(frame.Data[inputIndex]) | (int16(frame.Data[inputIndex+1]) << 8)

		// Write output sample
		outputData[i*2] = byte(sample)
		outputData[i*2+1] = byte(sample >> 8)
	}

	// Create new frame with upsampled audio
	newFormat := frame.Format
	newFormat.SampleRate = targetSampleRate

	return media.NewAudioFrame(outputData, newFormat), nil
}
