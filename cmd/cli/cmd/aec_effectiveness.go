package cmd

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
)

// NewAECEffectivenessTestCmd creates the AEC effectiveness test command
func NewAECEffectivenessTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aec-effectiveness-test",
		Short: "Test AEC echo cancellation effectiveness with real TTS playback",
		Long: `Comprehensive test to validate AEC echo cancellation effectiveness.
This test:
1. Plays TTS audio through speakers while recording microphone
2. Compares audio with and without AEC processing
3. Measures actual echo reduction in dB
4. Validates that user voice is preserved while echoes are cancelled
5. Tests the complete TTS→Speaker→Microphone→AEC loop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAECEffectivenessTest()
		},
	}
	
	return cmd
}

func runAECEffectivenessTest() error {
	ctx := context.Background()
	
	fmt.Println("🧪 Starting AEC Effectiveness Test...")
	fmt.Println("📢 This test will play TTS audio while recording to measure echo cancellation")
	
	// Initialize services
	fmt.Println("📋 Initializing services...")
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}
	
	if services.TTS == nil {
		return fmt.Errorf("TTS service not available")
	}
	
	fmt.Printf("✅ TTS service: %s\n", services.TTS.Name())
	
	// Test 1: Record without AEC (baseline with echo)
	fmt.Println("\n=== TEST 1: Recording with Echo (No AEC) ===")
	fmt.Println("🔊 This will play TTS while recording through direct microphone")
	fmt.Println("📏 Measuring baseline echo levels...")
	
	noAECEnergy, err := testEchoWithoutAEC(ctx, services)
	if err != nil {
		fmt.Printf("❌ No-AEC test failed: %v\n", err)
		return err
	}
	
	// Wait between tests
	fmt.Println("\n⏳ Waiting 3 seconds between tests...")
	time.Sleep(3 * time.Second)
	
	// Test 2: Record with AEC (should reduce echo)
	fmt.Println("\n=== TEST 2: Recording with AEC Echo Cancellation ===")
	fmt.Println("🎵 This will play TTS while recording through AEC pipeline")
	fmt.Println("📏 Measuring echo reduction effectiveness...")
	
	aecEnergy, err := testEchoWithAEC(ctx, services)
	if err != nil {
		fmt.Printf("❌ AEC test failed: %v\n", err)
		return err
	}
	
	// Calculate echo reduction
	fmt.Println("\n📊 AEC Effectiveness Analysis:")
	if noAECEnergy > 0 && aecEnergy >= 0 {
		if aecEnergy == 0 {
			fmt.Println("🎯 Perfect echo cancellation - no detectable echo energy")
		} else {
			reductionRatio := noAECEnergy / aecEnergy
			reductionDB := 20 * math.Log10(reductionRatio)
			fmt.Printf("📉 Echo energy reduction: %.6f → %.6f\n", noAECEnergy, aecEnergy)
			fmt.Printf("📊 Echo reduction ratio: %.2fx\n", reductionRatio)
			fmt.Printf("🔊 Echo reduction: %.1f dB\n", reductionDB)
			
			if reductionDB > 20 {
				fmt.Println("✅ EXCELLENT: AEC providing >20dB echo reduction")
			} else if reductionDB > 10 {
				fmt.Println("✅ GOOD: AEC providing >10dB echo reduction")  
			} else if reductionDB > 3 {
				fmt.Println("⚠️  MODERATE: AEC providing some echo reduction")
			} else {
				fmt.Println("❌ POOR: AEC not providing significant echo reduction")
			}
		}
	} else {
		fmt.Println("⚠️  Unable to calculate echo reduction - insufficient data")
	}
	
	// Test 3: Voice preservation test
	fmt.Println("\n=== TEST 3: Voice Preservation Test ===")
	fmt.Println("🎤 Testing that AEC preserves user voice while canceling echoes")
	fmt.Println("📢 Speak into microphone WHILE TTS is playing...")
	
	if err := testVoicePreservation(ctx, services); err != nil {
		fmt.Printf("❌ Voice preservation test failed: %v\n", err)
	}
	
	fmt.Println("\n✅ AEC Effectiveness Test completed!")
	fmt.Println("💡 If AEC is working properly, you should see:")
	fmt.Println("   - Significant echo reduction (>10dB)")
	fmt.Println("   - User voice still clearly recognized during TTS playback")
	fmt.Println("   - Low background noise in processed audio")
	
	return nil
}

func testEchoWithoutAEC(ctx context.Context, services *plugins.SmartServices) (float64, error) {
	fmt.Println("🎤 Setting up direct microphone recording...")
	
	// Create basic audio I/O (no AEC)
	audioConfig := audio.Config{
		SampleRate:      48000,
		Channels:        1,
		BitDepth:        16,
		FramesPerBuffer: 1024,
	}
	
	audioIO, err := audio.NewLocalAudioIO(audioConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()
	
	// Start audio I/O
	if err := audioIO.Start(ctx); err != nil {
		return 0, fmt.Errorf("failed to start audio I/O: %w", err)
	}
	defer audioIO.Stop()
	
	// Generate TTS for echo test
	testText := "This is an echo cancellation test. Can you hear this audio playing through the speakers?"
	fmt.Printf("🗣️  Generating TTS: \"%s\"\n", testText)
	
	ttsAudio, err := services.TTS.Synthesize(ctx, testText, nil)
	if err != nil {
		return 0, fmt.Errorf("TTS synthesis failed: %w", err)
	}
	
	// Start recording before TTS playback
	fmt.Println("🎙️  Starting recording...")
	recordedFrames := make([]*media.AudioFrame, 0)
	inputChan := audioIO.InputChan()
	outputChan := audioIO.OutputChan()
	
	// Start recording goroutine
	recordingDone := make(chan struct{})
	go func() {
		defer close(recordingDone)
		timeout := time.After(8 * time.Second) // Record for 8 seconds
		
		for {
			select {
			case frame := <-inputChan:
				recordedFrames = append(recordedFrames, frame)
			case <-timeout:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Wait 1 second, then play TTS
	time.Sleep(1 * time.Second)
	fmt.Println("🔊 Playing TTS audio (should create echo)...")
	
	select {
	case outputChan <- ttsAudio:
		fmt.Println("✅ TTS audio sent to speakers")
	case <-time.After(5 * time.Second):
		return 0, fmt.Errorf("timeout sending TTS audio")
	}
	
	// Wait for recording to complete
	<-recordingDone
	
	if len(recordedFrames) == 0 {
		return 0, fmt.Errorf("no audio recorded")
	}
	
	// Calculate average energy (should include echo)
	totalEnergy := 0.0
	for _, frame := range recordedFrames {
		energy := calculateFrameEnergy(frame)
		totalEnergy += energy
	}
	avgEnergy := totalEnergy / float64(len(recordedFrames))
	
	fmt.Printf("📊 Recorded %d frames without AEC\n", len(recordedFrames))
	fmt.Printf("📊 Average audio energy (with echo): %.6f\n", avgEnergy)
	
	return avgEnergy, nil
}

func testEchoWithAEC(ctx context.Context, services *plugins.SmartServices) (float64, error) {
	fmt.Println("🎵 Setting up AEC pipeline...")
	
	// Create AEC pipeline
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 24000 // AEC optimized sample rate
	
	aecPipeline, err := audio.NewAECPipeline(aecConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to create AEC pipeline: %w", err)
	}
	defer aecPipeline.Close()
	
	// Start AEC pipeline
	if err := aecPipeline.Start(ctx); err != nil {
		return 0, fmt.Errorf("failed to start AEC pipeline: %w", err)
	}
	defer aecPipeline.Stop()
	
	// Get AEC channels
	audioIO := aecPipeline.GetAudioIO()
	inputChan := audioIO.InputChan()
	outputChan := aecPipeline.GetOutputChan()
	
	// Generate TTS for echo test (same text for consistency)
	testText := "This is an echo cancellation test. Can you hear this audio playing through the speakers?"
	fmt.Printf("🗣️  Generating TTS: \"%s\"\n", testText)
	
	ttsAudio, err := services.TTS.Synthesize(ctx, testText, nil)
	if err != nil {
		return 0, fmt.Errorf("TTS synthesis failed: %w", err)
	}
	
	// Start recording before TTS playback
	fmt.Println("🎙️  Starting AEC recording...")
	recordedFrames := make([]*media.AudioFrame, 0)
	
	// Start recording goroutine
	recordingDone := make(chan struct{})
	go func() {
		defer close(recordingDone)
		timeout := time.After(8 * time.Second) // Record for 8 seconds
		
		for {
			select {
			case frame := <-inputChan:
				recordedFrames = append(recordedFrames, frame)
			case <-timeout:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Wait 1 second, then play TTS through AEC pipeline
	time.Sleep(1 * time.Second)
	fmt.Println("🔊 Playing TTS through AEC pipeline (echo should be cancelled)...")
	
	select {
	case outputChan <- ttsAudio:
		fmt.Println("✅ TTS audio sent to AEC output")
	case <-time.After(5 * time.Second):
		return 0, fmt.Errorf("timeout sending TTS to AEC")
	}
	
	// Wait for recording to complete
	<-recordingDone
	
	if len(recordedFrames) == 0 {
		return 0, fmt.Errorf("no audio recorded from AEC")
	}
	
	// Calculate average energy (should have reduced echo)
	totalEnergy := 0.0
	for _, frame := range recordedFrames {
		energy := calculateFrameEnergy(frame)
		totalEnergy += energy
	}
	avgEnergy := totalEnergy / float64(len(recordedFrames))
	
	fmt.Printf("📊 Recorded %d frames with AEC\n", len(recordedFrames))
	fmt.Printf("📊 Average audio energy (after AEC): %.6f\n", avgEnergy)
	
	// Print AEC statistics
	fmt.Println("\n📊 AEC Pipeline Performance:")
	aecPipeline.PrintStats()
	
	return avgEnergy, nil
}

func testVoicePreservation(ctx context.Context, services *plugins.SmartServices) error {
	fmt.Println("🎵 Setting up AEC pipeline for voice preservation test...")
	
	// Create AEC pipeline
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 24000
	
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
	
	// Get AEC channels
	audioIO := aecPipeline.GetAudioIO()
	inputChan := audioIO.InputChan()
	outputChan := aecPipeline.GetOutputChan()
	
	// Generate background TTS
	backgroundText := "Background audio playing. This should be cancelled by AEC while preserving your voice."
	fmt.Printf("🗣️  Generating background TTS: \"%s\"\n", backgroundText)
	
	backgroundAudio, err := services.TTS.Synthesize(ctx, backgroundText, nil)
	if err != nil {
		return fmt.Errorf("background TTS synthesis failed: %w", err)
	}
	
	// Start background TTS playback
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("🔊 Starting background TTS playback...")
		select {
		case outputChan <- backgroundAudio:
			fmt.Println("✅ Background TTS started")
		case <-time.After(5 * time.Second):
			fmt.Println("⚠️  Timeout starting background TTS")
		}
	}()
	
	// Collect audio with user voice + background TTS
	fmt.Println("🎙️  Please speak into the microphone while background TTS is playing...")
	fmt.Println("💡 Say something like: 'This is my voice during TTS playback'")
	
	recordedFrames := make([]*media.AudioFrame, 0)
	timeout := time.After(10 * time.Second)
	
	for {
		select {
		case frame := <-inputChan:
			recordedFrames = append(recordedFrames, frame)
		case <-timeout:
			goto analyzeVoice
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
analyzeVoice:
	if len(recordedFrames) == 0 {
		return fmt.Errorf("no audio recorded during voice preservation test")
	}
	
	// Combine frames for STT analysis
	combinedAudio := combineAudioFrames(recordedFrames)
	if combinedAudio == nil {
		return fmt.Errorf("failed to combine audio frames")
	}
	
	// Test STT recognition on the mixed audio
	fmt.Println("🧠 Testing voice recognition during TTS playback...")
	
	// Convert sample rate for STT if needed
	if combinedAudio.Format.SampleRate != 48000 {
		upsampled, err := upsampleAudio(combinedAudio, 48000)
		if err != nil {
			fmt.Printf("⚠️  Sample rate conversion failed: %v\n", err)
		} else {
			combinedAudio = upsampled
		}
	}
	
	recognition, err := services.STT.Recognize(ctx, combinedAudio)
	if err != nil {
		return fmt.Errorf("STT recognition failed: %v", err)
	}
	
	fmt.Printf("📝 Voice Recognition Result: \"%s\" (confidence: %.2f)\n", recognition.Text, recognition.Confidence)
	
	if recognition.Text != "" && recognition.Confidence > 0.5 {
		fmt.Println("✅ EXCELLENT: User voice preserved and recognized during TTS playback")
		fmt.Println("🎯 This indicates AEC is working - it cancelled the background TTS but kept your voice")
	} else if recognition.Text != "" {
		fmt.Println("⚠️  PARTIAL: Voice detected but low confidence - AEC may be over-aggressive")
	} else {
		fmt.Println("❌ CONCERNING: No voice recognized - AEC may be cancelling everything")
		fmt.Println("💡 This could indicate AEC is too aggressive or user didn't speak")
	}
	
	return nil
}