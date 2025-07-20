package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
)

// NewChatCLIAudioDebugCmd creates the ChatCLI audio debug command
func NewChatCLIAudioDebugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chatcli-audio-debug",
		Short: "Debug ChatCLI audio processing to identify transcription issues",
		Long: `Debug tool to compare ChatCLI audio processing with working direct microphone.
This command:
1. Captures audio using the same ChatCLI AEC pipeline as basic-agent
2. Saves the processed audio for analysis
3. Compares energy levels and quality metrics
4. Tests the exact same audio path that basic-agent uses`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChatCLIAudioDebug()
		},
	}
	
	return cmd
}

func runChatCLIAudioDebug() error {
	ctx := context.Background()
	
	fmt.Println("🔍 Starting ChatCLI Audio Debug...")
	fmt.Println("🎯 This will capture audio using the same pipeline as basic-agent")
	
	// Create output directory
	outputDir := "./test-results/chatcli-debug"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Initialize services (same as basic-agent)
	fmt.Println("📋 Initializing services (same as basic-agent)...")
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}
	
	if services.STT == nil {
		return fmt.Errorf("STT service not available")
	}
	
	fmt.Printf("✅ STT service: %s\n", services.STT.Name())
	
	// Test the ChatCLI audio processing pipeline
	fmt.Println("\n=== ChatCLI Audio Pipeline Debug ===")
	fmt.Println("🎵 Using same AEC pipeline configuration as basic-agent")
	
	// Create AEC pipeline (same as ChatCLI)
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 24000 // Same as ChatCLI
	
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
	
	// Get AEC audio I/O (same as ChatCLI)
	audioIO := aecPipeline.GetAudioIO()
	inputChan := audioIO.InputChan()
	
	fmt.Printf("🎙️  Listening through ChatCLI AEC pipeline (24kHz)... Please speak for 5 seconds!\n")
	
	// Collect audio frames (same logic as ChatCLI)
	recordedFrames := make([]*media.AudioFrame, 0)
	timeout := time.After(5 * time.Second)
	
	frameCount := 0
	totalBytes := 0
	var minEnergy, maxEnergy, totalEnergy float64 = 1.0, 0.0, 0.0
	
	for {
		select {
		case frame := <-inputChan:
			recordedFrames = append(recordedFrames, frame)
			frameCount++
			totalBytes += len(frame.Data)
			
			// Calculate energy (same as ChatCLI)
			energy := calculateFrameEnergy(frame)
			totalEnergy += energy
			if energy < minEnergy {
				minEnergy = energy
			}
			if energy > maxEnergy {
				maxEnergy = energy
			}
			
			if frameCount%50 == 0 {
				fmt.Printf("📊 ChatCLI: %d frames, %d bytes, energy: %.6f\n", frameCount, totalBytes, energy)
			}
		case <-timeout:
			goto analyzeAudio
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
analyzeAudio:
	if len(recordedFrames) == 0 {
		return fmt.Errorf("no audio captured")
	}
	
	avgEnergy := totalEnergy / float64(frameCount)
	
	fmt.Printf("\n📊 ChatCLI Audio Analysis:\n")
	fmt.Printf("   - Frames captured: %d\n", frameCount)
	fmt.Printf("   - Total bytes: %d\n", totalBytes)
	fmt.Printf("   - Sample rate: %d Hz\n", recordedFrames[0].Format.SampleRate)
	fmt.Printf("   - Energy stats: min=%.6f, max=%.6f, avg=%.6f\n", minEnergy, maxEnergy, avgEnergy)
	
	// Combine frames (same as ChatCLI)
	combinedAudio := combineAudioFrames(recordedFrames)
	if combinedAudio == nil {
		return fmt.Errorf("failed to combine audio frames")
	}
	
	fmt.Printf("📊 Combined audio: %d bytes at %d Hz\n", len(combinedAudio.Data), combinedAudio.Format.SampleRate)
	
	// Save raw audio for analysis
	rawAudioPath := filepath.Join(outputDir, "chatcli_raw_audio.bin")
	if err := saveRawAudio(combinedAudio, rawAudioPath); err != nil {
		fmt.Printf("⚠️  Failed to save raw audio: %v\n", err)
	} else {
		fmt.Printf("💾 Raw audio saved: %s\n", rawAudioPath)
	}
	
	// Test sample rate conversion (same as ChatCLI uses for STT)
	fmt.Println("\n🔧 Testing sample rate conversion (24kHz → 48kHz)...")
	upsampled, err := upsampleAudio(combinedAudio, 48000)
	if err != nil {
		fmt.Printf("❌ Sample rate conversion failed: %v\n", err)
		upsampled = combinedAudio
	} else {
		fmt.Printf("✅ Upsampled: %d bytes at %d Hz\n", len(upsampled.Data), upsampled.Format.SampleRate)
		
		// Save upsampled audio
		upsampledPath := filepath.Join(outputDir, "chatcli_upsampled_audio.bin")
		if err := saveRawAudio(upsampled, upsampledPath); err != nil {
			fmt.Printf("⚠️  Failed to save upsampled audio: %v\n", err)
		} else {
			fmt.Printf("💾 Upsampled audio saved: %s\n", upsampledPath)
		}
	}
	
	// Test STT recognition (same as ChatCLI)
	fmt.Println("\n🧠 Testing STT recognition with ChatCLI-processed audio...")
	recognition, err := services.STT.Recognize(ctx, upsampled)
	if err != nil {
		fmt.Printf("❌ STT recognition failed: %v\n", err)
	} else {
		fmt.Printf("📝 ChatCLI STT Result: \"%s\" (confidence: %.2f)\n", recognition.Text, recognition.Confidence)
		
		if recognition.Text == "" {
			fmt.Println("🚨 ISSUE REPRODUCED: Empty transcription like basic-agent")
			fmt.Println("💡 This confirms the problem is in the ChatCLI AEC pipeline")
		} else {
			fmt.Println("✅ Transcription worked - issue may be elsewhere")
		}
	}
	
	// Compare with direct microphone
	fmt.Println("\n=== Comparison with Direct Microphone ===")
	fmt.Println("🎤 Testing same speech with direct microphone...")
	
	directEnergy, directTranscript, err := testDirectMicrophone(ctx, services)
	if err != nil {
		fmt.Printf("⚠️  Direct microphone test failed: %v\n", err)
	} else {
		fmt.Printf("📊 Direct mic energy: %.6f\n", directEnergy)
		fmt.Printf("📝 Direct mic result: \"%s\"\n", directTranscript)
		
		fmt.Println("\n📊 Comparison Summary:")
		fmt.Printf("   ChatCLI AEC Energy:   %.6f\n", avgEnergy)
		fmt.Printf("   Direct Mic Energy:    %.6f\n", directEnergy)
		
		if avgEnergy > 0 && directEnergy > 0 {
			ratio := directEnergy / avgEnergy
			fmt.Printf("   Energy Ratio:         %.2fx\n", ratio)
			
			if ratio > 5 {
				fmt.Println("🚨 ChatCLI AEC is severely reducing audio energy")
			} else if ratio > 2 {
				fmt.Println("⚠️  ChatCLI AEC is reducing audio energy")
			} else {
				fmt.Println("✅ Audio energy levels are comparable")
			}
		}
		
		if recognition.Text == "" && directTranscript != "" {
			fmt.Println("🎯 CONFIRMED: ChatCLI AEC pipeline is the issue")
			fmt.Println("💡 Recommendations:")
			fmt.Println("   1. Adjust AEC configuration parameters")
			fmt.Println("   2. Fix sample rate conversion quality")
			fmt.Println("   3. Verify AEC reference signal setup")
			fmt.Println("   4. Consider disabling AEC temporarily")
		}
	}
	
	// Print AEC stats
	fmt.Println("\n📊 AEC Pipeline Statistics:")
	aecPipeline.PrintStats()
	
	fmt.Printf("\n✅ ChatCLI Audio Debug completed!\n")
	fmt.Printf("📁 Debug files saved to: %s\n", outputDir)
	
	return nil
}

func testDirectMicrophone(ctx context.Context, services *plugins.SmartServices) (float64, string, error) {
	// Create direct microphone input (no AEC)
	audioConfig := audio.Config{
		SampleRate:      48000,
		Channels:        1,
		BitDepth:        16,
		FramesPerBuffer: 1024,
	}
	
	audioIO, err := audio.NewLocalAudioIO(audioConfig)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()
	
	// Start audio I/O
	if err := audioIO.Start(ctx); err != nil {
		return 0, "", fmt.Errorf("failed to start audio I/O: %w", err)
	}
	defer audioIO.Stop()
	
	fmt.Println("🎙️  Please repeat the same speech with direct microphone (3 seconds)...")
	
	// Collect audio
	recordedFrames := make([]*media.AudioFrame, 0)
	inputChan := audioIO.InputChan()
	timeout := time.After(3 * time.Second)
	
	totalEnergy := 0.0
	frameCount := 0
	
	for {
		select {
		case frame := <-inputChan:
			recordedFrames = append(recordedFrames, frame)
			energy := calculateFrameEnergy(frame)
			totalEnergy += energy
			frameCount++
		case <-timeout:
			goto processDirect
		case <-ctx.Done():
			return 0, "", ctx.Err()
		}
	}
	
processDirect:
	if len(recordedFrames) == 0 {
		return 0, "", fmt.Errorf("no audio captured from direct microphone")
	}
	
	avgEnergy := totalEnergy / float64(frameCount)
	
	// Combine and test STT
	combinedAudio := combineAudioFrames(recordedFrames)
	if combinedAudio == nil {
		return avgEnergy, "", fmt.Errorf("failed to combine direct audio")
	}
	
	recognition, err := services.STT.Recognize(ctx, combinedAudio)
	if err != nil {
		return avgEnergy, "", fmt.Errorf("direct STT failed: %w", err)
	}
	
	return avgEnergy, recognition.Text, nil
}

func saveRawAudio(frame *media.AudioFrame, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Write header info
	fmt.Fprintf(file, "# Raw Audio Data\n")
	fmt.Fprintf(file, "# Sample Rate: %d Hz\n", frame.Format.SampleRate)
	fmt.Fprintf(file, "# Channels: %d\n", frame.Format.Channels)
	fmt.Fprintf(file, "# Data Size: %d bytes\n", len(frame.Data))
	fmt.Fprintf(file, "# Format: 16-bit PCM little-endian\n")
	
	// Write raw audio data
	_, err = file.Write(frame.Data)
	if err != nil {
		return fmt.Errorf("failed to write audio data: %w", err)
	}
	
	return nil
}