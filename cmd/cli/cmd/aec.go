package cmd

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewAECTestCmd creates the AEC testing command
func NewAECTestCmd() *cobra.Command {
	var (
		duration        time.Duration
		sampleRate      int
		enableEcho      bool
		enableNoise     bool
		enableAGC       bool
		delayMs         int
		toneFreq        float64
		volume          float64
		suppressionLevel int
	)

	cmd := &cobra.Command{
		Use:   "aec-test",
		Short: "Test Acoustic Echo Cancellation functionality",
		Long: `Test the LiveKit WebRTC AudioProcessingModule with synthetic audio.

This command creates synthetic audio streams to test echo cancellation:
1. Generates a reference tone for the "speaker output"
2. Creates a "microphone input" that includes the reference tone as echo
3. Processes the input through the AEC to remove the echo
4. Reports the echo reduction achieved

Examples:
  pipeline-test aec-test --duration 5s --tone-freq 1000    # Test with 1kHz tone
  pipeline-test aec-test --delay-ms 50 --suppression 2     # Configure delay and suppression
  pipeline-test aec-test --sample-rate 48000               # Test with 48kHz audio`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAECTest(AECTestConfig{
				Duration:         duration,
				SampleRate:       sampleRate,
				EnableEcho:       enableEcho,
				EnableNoise:      enableNoise,
				EnableAGC:        enableAGC,
				DelayMs:          delayMs,
				ToneFreq:         toneFreq,
				Volume:           volume,
				SuppressionLevel: suppressionLevel,
			})
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 3*time.Second, "test duration")
	cmd.Flags().IntVar(&sampleRate, "sample-rate", 24000, "audio sample rate (Hz)")
	cmd.Flags().BoolVar(&enableEcho, "enable-echo", true, "enable echo cancellation")
	cmd.Flags().BoolVar(&enableNoise, "enable-noise", true, "enable noise suppression")
	cmd.Flags().BoolVar(&enableAGC, "enable-agc", true, "enable automatic gain control")
	cmd.Flags().IntVar(&delayMs, "delay-ms", 50, "simulated echo delay in milliseconds")
	cmd.Flags().Float64Var(&toneFreq, "tone-freq", 1000.0, "test tone frequency in Hz")
	cmd.Flags().Float64Var(&volume, "volume", 0.3, "test tone volume (0.0-1.0)")
	cmd.Flags().IntVar(&suppressionLevel, "suppression", 1, "echo suppression level (0-2)")

	return cmd
}

// AECTestConfig holds configuration for AEC testing
type AECTestConfig struct {
	Duration         time.Duration
	SampleRate       int
	EnableEcho       bool
	EnableNoise      bool
	EnableAGC        bool
	DelayMs          int
	ToneFreq         float64
	Volume           float64
	SuppressionLevel int
}

// runAECTest performs the AEC test with synthetic audio
func runAECTest(config AECTestConfig) error {
	fmt.Println("🔬 LiveKit WebRTC AudioProcessingModule Test")
	fmt.Println("===========================================")
	fmt.Printf("🎵 Sample rate: %d Hz\n", config.SampleRate)
	fmt.Printf("🔊 Test tone: %.0f Hz at %.1f%% volume\n", config.ToneFreq, config.Volume*100)
	fmt.Printf("⏱️  Duration: %v\n", config.Duration)
	fmt.Printf("⏳ Simulated delay: %d ms\n", config.DelayMs)
	fmt.Printf("🎛️  Echo cancellation: %v\n", config.EnableEcho)
	fmt.Printf("🔇 Noise suppression: %v\n", config.EnableNoise)
	fmt.Printf("📈 Auto gain control: %v\n", config.EnableAGC)
	fmt.Println()

	// Create AEC processor
	aecConfig := audio.AECConfig{
		EnableEchoCancellation: config.EnableEcho,
		EnableNoiseSuppression: config.EnableNoise,
		EnableAutoGainControl:  config.EnableAGC,
		EchoSuppressionLevel:   config.SuppressionLevel,
		DelayMs:               config.DelayMs,
		SampleRate:            config.SampleRate,
		Channels:              1,
	}

	processor, err := audio.NewLiveKitAECProcessor(aecConfig)
	if err != nil {
		return fmt.Errorf("failed to create AEC processor: %w", err)
	}
	defer processor.Close()

	// Get frame configuration
	frameSize := processor.GetFrameSize()
	frameDuration := time.Duration(frameSize) * time.Second / time.Duration(config.SampleRate)
	
	fmt.Printf("🎯 Frame size: %d samples (%.1f ms)\n", frameSize, frameDuration.Seconds()*1000)
	fmt.Printf("🔧 Using LiveKit WebRTC AudioProcessingModule (Chrome-grade processing)\n")
	fmt.Println()

	// Create audio format
	audioFormat := media.AudioFormat{
		SampleRate:    config.SampleRate,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	// Create tone generator
	toneGen := NewSyntheticToneGenerator(config.ToneFreq, config.Volume, config.SampleRate)
	echoGen := NewEchoSimulator(config.DelayMs, 0.3, config.SampleRate) // 30% echo level

	// Test execution
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration+5*time.Second)
	defer cancel()

	fmt.Println("🚀 Starting AEC test...")
	fmt.Println("📊 Processing synthetic audio frames...")

	frameCount := 0
	totalFrames := int(config.Duration / frameDuration)
	
	// Statistics tracking
	var beforeEchoEnergy, afterEchoEnergy float64
	var totalProcessingTime time.Duration

	startTime := time.Now()
	
	for frameCount < totalFrames {
		select {
		case <-ctx.Done():
			return fmt.Errorf("test cancelled: %v", ctx.Err())
		default:
		}

		// Generate reference tone (speaker output)
		referenceFrame := toneGen.GenerateFrame(frameSize, audioFormat)
		
		// Generate microphone input with simulated echo
		microphoneFrame := toneGen.GenerateFrame(frameSize, audioFormat)
		echoGen.AddEcho(microphoneFrame, referenceFrame)
		
		// Add some noise to make it more realistic
		addNoise(microphoneFrame, 0.05) // 5% noise level

		// Measure energy before AEC
		beforeEnergy := calculateFrameEnergy(microphoneFrame)
		beforeEchoEnergy += beforeEnergy

		// Process through AEC
		processingStart := time.Now()
		processedFrame, err := processor.ProcessStreams(ctx, microphoneFrame, referenceFrame)
		processingTime := time.Since(processingStart)
		totalProcessingTime += processingTime
		
		if err != nil {
			return fmt.Errorf("AEC processing failed on frame %d: %w", frameCount, err)
		}

		// Measure energy after AEC
		afterEnergy := calculateFrameEnergy(processedFrame)
		afterEchoEnergy += afterEnergy

		frameCount++

		// Progress indicator
		if frameCount%50 == 0 || frameCount == totalFrames {
			progress := float64(frameCount) / float64(totalFrames) * 100
			fmt.Printf("\r🎯 Progress: %.1f%% (%d/%d frames)", progress, frameCount, totalFrames)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✅ Test completed in %v\n\n", elapsed)

	// Calculate results
	avgProcessingTime := totalProcessingTime / time.Duration(frameCount)
	avgBeforeEnergy := beforeEchoEnergy / float64(frameCount)
	avgAfterEnergy := afterEchoEnergy / float64(frameCount)
	
	// Echo reduction calculation
	var echoReductionDB float64
	if avgAfterEnergy > 0 && avgBeforeEnergy > 0 {
		ratio := avgBeforeEnergy / avgAfterEnergy
		if ratio > 1.0 {
			echoReductionDB = 20.0 * math.Log10(ratio)
		}
	}

	// Get AEC statistics
	stats := processor.GetStats()

	// Report results
	fmt.Println("📊 AEC TEST RESULTS")
	fmt.Println("==================")
	fmt.Printf("🎯 Frames processed: %d\n", frameCount)
	fmt.Printf("⚡ Avg processing time: %.2f ms per frame\n", avgProcessingTime.Seconds()*1000)
	fmt.Printf("🎵 Real-time factor: %.2fx (%.2f ms processing for %.2f ms audio)\n", 
		avgProcessingTime.Seconds()*1000/frameDuration.Seconds()/1000,
		avgProcessingTime.Seconds()*1000, 
		frameDuration.Seconds()*1000)
	fmt.Println()

	fmt.Printf("🔊 Energy before AEC: %.6f\n", avgBeforeEnergy)
	fmt.Printf("🔇 Energy after AEC: %.6f\n", avgAfterEnergy)
	fmt.Printf("📉 Echo reduction: %.1f dB\n", echoReductionDB)
	fmt.Println()

	fmt.Printf("📈 AEC Stats:\n")
	fmt.Printf("   - Frames processed: %d\n", stats.FramesProcessed)
	fmt.Printf("   - Frames dropped: %d\n", stats.FramesDropped)
	fmt.Printf("   - Configured delay: %d ms\n", stats.DelayMs)
	fmt.Printf("   - Processing latency: %.2f ms\n", stats.ProcessingLatencyMs)
	if stats.EchoReturnLoss > 0 {
		fmt.Printf("   - Echo return loss: %.1f dB\n", stats.EchoReturnLoss)
		fmt.Printf("   - ERL enhancement: %.1f dB\n", stats.EchoReturnLossEnhancement)
	}
	fmt.Println()

	// Performance assessment
	if avgProcessingTime.Seconds()*1000 < frameDuration.Seconds()*1000 {
		fmt.Println("✅ PERFORMANCE: Excellent - Processing faster than real-time")
	} else {
		fmt.Println("⚠️  PERFORMANCE: Warning - Processing slower than real-time")
	}

	if echoReductionDB > 20.0 {
		fmt.Println("✅ ECHO REDUCTION: Excellent - >20 dB reduction achieved")
	} else if echoReductionDB > 10.0 {
		fmt.Println("✅ ECHO REDUCTION: Good - >10 dB reduction achieved")
	} else if echoReductionDB > 5.0 {
		fmt.Println("⚠️  ECHO REDUCTION: Moderate - >5 dB reduction achieved")
	} else {
		fmt.Println("❌ ECHO REDUCTION: Poor - <5 dB reduction achieved")
	}

	fmt.Println()
	fmt.Println("🎯 Test Summary:")
	if config.EnableEcho && echoReductionDB > 20.0 && avgProcessingTime.Seconds()*1000 < frameDuration.Seconds()*1000 {
		fmt.Println("✅ LiveKit WebRTC APM is working effectively!")
	} else {
		fmt.Println("⚠️  LiveKit WebRTC APM may need tuning or have configuration issues")
	}

	return nil
}

// SyntheticToneGenerator generates test tones
type SyntheticToneGenerator struct {
	frequency  float64
	volume     float64
	sampleRate int
	phase      float64
}

func NewSyntheticToneGenerator(frequency, volume float64, sampleRate int) *SyntheticToneGenerator {
	return &SyntheticToneGenerator{
		frequency:  frequency,
		volume:     volume,
		sampleRate: sampleRate,
		phase:      0,
	}
}

func (g *SyntheticToneGenerator) GenerateFrame(frameSize int, format media.AudioFormat) *media.AudioFrame {
	data := make([]byte, frameSize*2) // 16-bit samples
	
	phaseIncrement := 2 * math.Pi * g.frequency / float64(g.sampleRate)
	
	for i := 0; i < frameSize; i++ {
		// Generate sine wave sample
		sample := math.Sin(g.phase) * g.volume * 32767
		g.phase += phaseIncrement
		
		// Keep phase in range [0, 2π]
		if g.phase > 2*math.Pi {
			g.phase -= 2 * math.Pi
		}
		
		// Convert to 16-bit little-endian
		sampleInt := int16(sample)
		data[i*2] = byte(sampleInt & 0xFF)
		data[i*2+1] = byte((sampleInt >> 8) & 0xFF)
	}
	
	return media.NewAudioFrame(data, format)
}

// EchoSimulator simulates acoustic echo
type EchoSimulator struct {
	delayMs    int
	echoLevel  float64
	sampleRate int
	delayBuffer []int16
	bufferIndex int
}

func NewEchoSimulator(delayMs int, echoLevel float64, sampleRate int) *EchoSimulator {
	bufferSize := (sampleRate * delayMs) / 1000
	return &EchoSimulator{
		delayMs:     delayMs,
		echoLevel:   echoLevel,
		sampleRate:  sampleRate,
		delayBuffer: make([]int16, bufferSize),
		bufferIndex: 0,
	}
}

func (e *EchoSimulator) AddEcho(inputFrame, referenceFrame *media.AudioFrame) {
	if len(inputFrame.Data) != len(referenceFrame.Data) {
		return
	}
	
	frameSize := len(inputFrame.Data) / 2
	
	for i := 0; i < frameSize; i++ {
		// Extract reference sample
		refSample := int16(referenceFrame.Data[i*2]) | (int16(referenceFrame.Data[i*2+1]) << 8)
		
		// Store in delay buffer
		e.delayBuffer[e.bufferIndex] = refSample
		
		// Get delayed echo
		echoSample := e.delayBuffer[e.bufferIndex]
		echoSample = int16(float64(echoSample) * e.echoLevel)
		
		// Add echo to input
		inputSample := int16(inputFrame.Data[i*2]) | (int16(inputFrame.Data[i*2+1]) << 8)
		combinedSample := inputSample + echoSample
		
		// Prevent clipping
		if combinedSample > 32767 {
			combinedSample = 32767
		} else if combinedSample < -32768 {
			combinedSample = -32768
		}
		
		// Write back
		inputFrame.Data[i*2] = byte(combinedSample & 0xFF)
		inputFrame.Data[i*2+1] = byte((combinedSample >> 8) & 0xFF)
		
		// Advance buffer index
		e.bufferIndex = (e.bufferIndex + 1) % len(e.delayBuffer)
	}
}

// addNoise adds white noise to audio frame
func addNoise(frame *media.AudioFrame, level float64) {
	frameSize := len(frame.Data) / 2
	
	for i := 0; i < frameSize; i++ {
		sample := int16(frame.Data[i*2]) | (int16(frame.Data[i*2+1]) << 8)
		
		// Add random noise
		noise := int16((rand.Float64() - 0.5) * level * 32767)
		sample += noise
		
		// Prevent clipping
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}
		
		frame.Data[i*2] = byte(sample & 0xFF)
		frame.Data[i*2+1] = byte((sample >> 8) & 0xFF)
	}
}