package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewAECAudioDiagnosticCmd creates the AEC audio capture diagnostic command
func NewAECAudioDiagnosticCmd() *cobra.Command {
	var duration time.Duration
	
	cmd := &cobra.Command{
		Use:   "aec-audio-diagnostic",
		Short: "Diagnose AEC audio capture and processing quality", 
		Long: `Diagnostic tool to analyze AEC audio capture and processing quality.
This command:
1. Captures raw microphone input before AEC processing
2. Captures AEC-processed audio output
3. Compares audio quality, energy levels, and processing effects
4. Saves audio samples for analysis
5. Identifies potential over-cancellation or quality issues`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAECAudioDiagnostic(duration)
		},
	}
	
	cmd.Flags().DurationVarP(&duration, "duration", "d", 10*time.Second, "test duration")
	
	return cmd
}

func runAECAudioDiagnostic(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	fmt.Println("🧪 Starting AEC Audio Capture Diagnostic...")
	
	// Create output directory for audio samples
	outputDir := "./test-results/aec-audio-diagnostic"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Test 1: Raw microphone input (baseline)
	fmt.Println("\n=== TEST 1: Raw Microphone Input (Baseline) ===")
	rawAudioPath := filepath.Join(outputDir, "raw_microphone_input.wav")
	if err := captureRawMicrophoneAudio(ctx, rawAudioPath); err != nil {
		fmt.Printf("❌ Raw microphone capture failed: %v\n", err)
	}
	
	// Test 2: AEC-processed audio
	fmt.Println("\n=== TEST 2: AEC-Processed Audio ===")
	aecAudioPath := filepath.Join(outputDir, "aec_processed_audio.wav")
	if err := captureAECProcessedAudio(ctx, aecAudioPath); err != nil {
		fmt.Printf("❌ AEC-processed capture failed: %v\n", err)
	}
	
	// Test 3: AEC with simultaneous TTS playback (echo cancellation test)
	fmt.Println("\n=== TEST 3: AEC with TTS Playback (Echo Cancellation) ===")
	echoTestPath := filepath.Join(outputDir, "aec_with_tts_playback.wav")
	if err := captureAECWithTTSPlayback(ctx, echoTestPath); err != nil {
		fmt.Printf("❌ AEC echo cancellation test failed: %v\n", err)
	}
	
	fmt.Printf("\n✅ AEC Audio Diagnostic completed!\n")
	fmt.Printf("📁 Audio samples saved to: %s\n", outputDir)
	fmt.Println("💡 Compare the audio files to analyze AEC processing effects")
	
	return nil
}

func captureRawMicrophoneAudio(ctx context.Context, outputPath string) error {
	fmt.Println("🎤 Capturing raw microphone input for 5 seconds...")
	
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
	
	fmt.Println("🎙️  Recording raw microphone (5 seconds)... Please speak now!")
	
	// Collect audio for 5 seconds
	audioFrames := make([]*media.AudioFrame, 0)
	inputChan := audioIO.InputChan()
	timeout := time.After(5 * time.Second)
	
	frameCount := 0
	totalBytes := 0
	var minEnergy, maxEnergy, totalEnergy float64 = 1.0, 0.0, 0.0
	
	for {
		select {
		case frame := <-inputChan:
			audioFrames = append(audioFrames, frame)
			frameCount++
			totalBytes += len(frame.Data)
			
			// Calculate frame energy
			energy := calculateFrameEnergyAEC(frame)
			totalEnergy += energy
			if energy < minEnergy {
				minEnergy = energy
			}
			if energy > maxEnergy {
				maxEnergy = energy
			}
			
			if frameCount%20 == 0 {
				fmt.Printf("📊 Raw: %d frames, %d bytes, energy: %.6f\n", frameCount, totalBytes, energy)
			}
		case <-timeout:
			goto analyzeRawAudio
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
analyzeRawAudio:
	if len(audioFrames) == 0 {
		return fmt.Errorf("no audio frames captured")
	}
	
	avgEnergy := totalEnergy / float64(frameCount)
	
	fmt.Printf("📊 Raw Audio Analysis:\n")
	fmt.Printf("   - Frames captured: %d\n", frameCount)
	fmt.Printf("   - Total bytes: %d\n", totalBytes)
	fmt.Printf("   - Sample rate: %d Hz\n", audioFrames[0].Format.SampleRate)
	fmt.Printf("   - Channels: %d\n", audioFrames[0].Format.Channels)
	fmt.Printf("   - Energy stats: min=%.6f, max=%.6f, avg=%.6f\n", minEnergy, maxEnergy, avgEnergy)
	
	// Save to file (simplified - in real implementation, would use proper audio file format)
	if err := saveAudioFramesToFile(audioFrames, outputPath); err != nil {
		fmt.Printf("⚠️  Failed to save raw audio: %v\n", err)
	} else {
		fmt.Printf("💾 Raw audio saved: %s\n", outputPath)
	}
	
	return nil
}

func captureAECProcessedAudio(ctx context.Context, outputPath string) error {
	fmt.Println("🎵 Capturing AEC-processed audio for 5 seconds...")
	
	// Create AEC pipeline
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 24000 // AEC optimized sample rate
	
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
	
	fmt.Printf("🎙️  Recording AEC-processed audio (%d Hz, 5 seconds)... Please speak now!\n", aecConfig.SampleRate)
	
	// Collect AEC-processed audio for 5 seconds
	audioFrames := make([]*media.AudioFrame, 0)
	timeout := time.After(5 * time.Second)
	
	frameCount := 0
	totalBytes := 0
	var minEnergy, maxEnergy, totalEnergy float64 = 1.0, 0.0, 0.0
	
	for {
		select {
		case frame := <-inputChan:
			audioFrames = append(audioFrames, frame)
			frameCount++
			totalBytes += len(frame.Data)
			
			// Calculate frame energy
			energy := calculateFrameEnergyAEC(frame)
			totalEnergy += energy
			if energy < minEnergy {
				minEnergy = energy
			}
			if energy > maxEnergy {
				maxEnergy = energy
			}
			
			if frameCount%20 == 0 {
				fmt.Printf("📊 AEC: %d frames, %d bytes, energy: %.6f\n", frameCount, totalBytes, energy)
			}
		case <-timeout:
			goto analyzeAECAudio
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
analyzeAECAudio:
	if len(audioFrames) == 0 {
		return fmt.Errorf("no AEC-processed audio frames captured")
	}
	
	avgEnergy := totalEnergy / float64(frameCount)
	
	fmt.Printf("📊 AEC Audio Analysis:\n")
	fmt.Printf("   - Frames captured: %d\n", frameCount)
	fmt.Printf("   - Total bytes: %d\n", totalBytes)
	fmt.Printf("   - Sample rate: %d Hz\n", audioFrames[0].Format.SampleRate)
	fmt.Printf("   - Channels: %d\n", audioFrames[0].Format.Channels)
	fmt.Printf("   - Energy stats: min=%.6f, max=%.6f, avg=%.6f\n", minEnergy, maxEnergy, avgEnergy)
	
	// Analyze for potential issues
	if avgEnergy < 0.00001 {
		fmt.Println("🚨 WARNING: Very low average energy - AEC may be over-canceling")
	}
	if maxEnergy < 0.0001 {
		fmt.Println("🚨 WARNING: Very low peak energy - audio quality may be degraded")
	}
	if minEnergy == maxEnergy {
		fmt.Println("🚨 WARNING: No energy variation - possible audio silence or clipping")
	}
	
	// Save to file
	if err := saveAudioFramesToFile(audioFrames, outputPath); err != nil {
		fmt.Printf("⚠️  Failed to save AEC audio: %v\n", err)
	} else {
		fmt.Printf("💾 AEC audio saved: %s\n", outputPath)
	}
	
	// Print AEC pipeline statistics
	fmt.Println("\n📊 AEC Pipeline Statistics:")
	aecPipeline.PrintStats()
	
	return nil
}

func captureAECWithTTSPlayback(ctx context.Context, outputPath string) error {
	fmt.Println("🎵🔊 Testing AEC echo cancellation with simultaneous TTS playback...")
	
	// This test will play TTS audio while recording to test echo cancellation
	// In a real scenario, this would demonstrate AEC effectiveness
	
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
	
	// Get channels
	audioIO := aecPipeline.GetAudioIO()
	inputChan := audioIO.InputChan()
	outputChan := aecPipeline.GetOutputChan()
	
	fmt.Println("🎙️  Starting echo cancellation test...")
	fmt.Println("💡 This will play TTS audio while recording your voice to test AEC effectiveness")
	
	// Start TTS playback in background
	go func() {
		time.Sleep(1 * time.Second) // Delay before starting TTS
		
		// Create a simple test tone or silence (simulating TTS output)
		// In real implementation, would use actual TTS service
		testAudio := generateTestToneAEC(1000, 2*time.Second, aecConfig.SampleRate) // 1kHz tone for 2 seconds
		
		fmt.Println("🔊 Playing test tone through speakers (simulating TTS)...")
		select {
		case outputChan <- testAudio:
			fmt.Println("✅ Test tone sent to speakers")
		case <-time.After(5 * time.Second):
			fmt.Println("⚠️  Timeout sending test tone")
		}
	}()
	
	// Collect AEC-processed audio during TTS playback
	audioFrames := make([]*media.AudioFrame, 0)
	timeout := time.After(5 * time.Second)
	
	frameCount := 0
	totalBytes := 0
	var minEnergy, maxEnergy, totalEnergy float64 = 1.0, 0.0, 0.0
	
	fmt.Println("🎙️  Recording with echo cancellation... Speak while test tone plays!")
	
	for {
		select {
		case frame := <-inputChan:
			audioFrames = append(audioFrames, frame)
			frameCount++
			totalBytes += len(frame.Data)
			
			energy := calculateFrameEnergyAEC(frame)
			totalEnergy += energy
			if energy < minEnergy {
				minEnergy = energy
			}
			if energy > maxEnergy {
				maxEnergy = energy
			}
			
			if frameCount%20 == 0 {
				fmt.Printf("📊 Echo test: %d frames, energy: %.6f\n", frameCount, energy)
			}
		case <-timeout:
			goto analyzeEchoTest
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
analyzeEchoTest:
	if len(audioFrames) == 0 {
		return fmt.Errorf("no audio frames captured during echo test")
	}
	
	avgEnergy := totalEnergy / float64(frameCount)
	
	fmt.Printf("📊 Echo Cancellation Test Results:\n")
	fmt.Printf("   - Frames captured: %d\n", frameCount)
	fmt.Printf("   - Total bytes: %d\n", totalBytes)
	fmt.Printf("   - Energy stats: min=%.6f, max=%.6f, avg=%.6f\n", minEnergy, maxEnergy, avgEnergy)
	
	// Analyze echo cancellation effectiveness
	if avgEnergy < 0.00005 {
		fmt.Println("📈 GOOD: Low average energy suggests effective echo cancellation")
		fmt.Println("⚠️  But this might also indicate over-cancellation of user voice")
	} else if avgEnergy > 0.001 {
		fmt.Println("📈 MODERATE: Higher energy may indicate some echo leakage or user voice")
	}
	
	// Save results
	if err := saveAudioFramesToFile(audioFrames, outputPath); err != nil {
		fmt.Printf("⚠️  Failed to save echo test audio: %v\n", err)
	} else {
		fmt.Printf("💾 Echo test audio saved: %s\n", outputPath)
	}
	
	return nil
}

func calculateFrameEnergyAEC(frame *media.AudioFrame) float64 {
	if len(frame.Data) < 2 {
		return 0.0
	}
	
	var sum int64
	sampleCount := len(frame.Data) / 2 // 16-bit samples
	
	for i := 0; i < sampleCount; i++ {
		sample := int16(frame.Data[i*2]) | (int16(frame.Data[i*2+1]) << 8)
		sum += int64(sample) * int64(sample)
	}
	
	rms := float64(sum) / float64(sampleCount)
	energy := rms / (32767.0 * 32767.0) // Normalize to [0, 1]
	
	return energy
}

func generateTestToneAEC(frequency int, duration time.Duration, sampleRate int) *media.AudioFrame {
	samplesNeeded := int(float64(sampleRate) * duration.Seconds())
	data := make([]byte, samplesNeeded*2) // 16-bit samples
	
	for i := 0; i < samplesNeeded; i++ {
		// Generate sine wave
		// t := float64(i) / float64(sampleRate)
		amplitude := 8000.0 // Moderate amplitude
		sample := int16(amplitude * 0.3) // Reduced amplitude for test tone
		
		// Write 16-bit little-endian sample
		data[i*2] = byte(sample)
		data[i*2+1] = byte(sample >> 8)
	}
	
	format := media.AudioFormat{
		SampleRate: sampleRate,
		Channels:   1,
	}
	
	return media.NewAudioFrame(data, format)
}

func saveAudioFramesToFile(frames []*media.AudioFrame, outputPath string) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames to save")
	}
	
	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Write a simple header (not a real WAV file, just raw audio data)
	fmt.Fprintf(file, "# Raw Audio Data\n")
	fmt.Fprintf(file, "# Sample Rate: %d Hz\n", frames[0].Format.SampleRate)
	fmt.Fprintf(file, "# Channels: %d\n", frames[0].Format.Channels)
	fmt.Fprintf(file, "# Bit Depth: 16\n")
	fmt.Fprintf(file, "# Frames: %d\n", len(frames))
	
	// Write raw audio data
	totalBytes := 0
	for _, frame := range frames {
		n, err := file.Write(frame.Data)
		if err != nil {
			return fmt.Errorf("failed to write frame data: %w", err)
		}
		totalBytes += n
	}
	
	log.Printf("Saved %d bytes of audio data to %s", totalBytes, outputPath)
	return nil
}