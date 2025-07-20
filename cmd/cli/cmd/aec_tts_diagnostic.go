package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
)

// NewAECTTSTestCmd creates the AEC TTS testing command
func NewAECTTSTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aec-tts-test",
		Short: "Test AEC pipeline with TTS output",
		Long: `Test the AEC pipeline with TTS output to diagnose missing audio output issues.
This command:
1. Creates an AEC pipeline
2. Generates TTS audio 
3. Sends it through the output channel
4. Monitors for successful playback`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAECTTSTest()
		},
	}
	
	return cmd
}

func runAECTTSTest() error {
	ctx := context.Background()
	
	fmt.Println("🧪 Starting AEC TTS Test...")
	
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
	
	// Create AEC pipeline
	fmt.Println("🎵 Creating AEC pipeline...")
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 24000
	
	aecPipeline, err := audio.NewAECPipeline(aecConfig)
	if err != nil {
		return fmt.Errorf("failed to create AEC pipeline: %w", err)
	}
	defer aecPipeline.Close()
	
	// Start pipeline
	fmt.Println("🚀 Starting AEC pipeline...")
	if err := aecPipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start AEC pipeline: %w", err)
	}
	defer aecPipeline.Stop()
	
	// Get output channel
	outputChan := aecPipeline.GetOutputChan()
	
	// Generate TTS audio
	testText := "Hello! This is a test of the TTS output through the AEC pipeline. Can you hear me clearly?"
	fmt.Printf("🗣️  Generating TTS: \"%s\"\n", testText)
	
	ttsStart := time.Now()
	audioFrame, err := services.TTS.Synthesize(ctx, testText, nil)
	if err != nil {
		return fmt.Errorf("TTS synthesis failed: %w", err)
	}
	ttsElapsed := time.Since(ttsStart)
	
	fmt.Printf("✅ TTS completed in %v (%d bytes)\n", ttsElapsed, len(audioFrame.Data))
	fmt.Printf("📊 Audio format: %+v\n", audioFrame.Format)
	
	// Send to AEC pipeline output
	fmt.Println("🔊 Sending TTS audio to AEC pipeline output...")
	
	sendStart := time.Now()
	select {
	case outputChan <- audioFrame:
		sendElapsed := time.Since(sendStart)
		fmt.Printf("✅ Audio sent to output channel in %v\n", sendElapsed)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("❌ Timeout sending audio to output channel")
	}
	
	// Wait for playback
	playbackDuration := estimatePlaybackDuration(audioFrame)
	fmt.Printf("⏳ Waiting for playback completion (%v)...\n", playbackDuration)
	time.Sleep(playbackDuration + 1*time.Second)
	
	// Print statistics
	fmt.Println("\n📊 Pipeline statistics:")
	aecPipeline.PrintStats()
	
	fmt.Println("\n✅ AEC TTS Test completed successfully!")
	return nil
}

func estimatePlaybackDuration(frame *media.AudioFrame) time.Duration {
	// Calculate duration based on sample rate and data size
	sampleRate := frame.Format.SampleRate
	channels := frame.Format.Channels
	bytesPerSample := 2 // Assuming 16-bit audio
	
	if sampleRate > 0 && channels > 0 {
		totalSamples := len(frame.Data) / (channels * bytesPerSample)
		duration := time.Duration(float64(totalSamples)/float64(sampleRate)*1000) * time.Millisecond
		return duration
	}
	
	// Fallback estimation
	return time.Duration(len(frame.Data)/3000) * time.Millisecond
}