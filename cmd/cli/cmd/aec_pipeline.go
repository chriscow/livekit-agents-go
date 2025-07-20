package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewAECPipelineCmd creates the AEC pipeline testing command
func NewAECPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aec-pipeline",
		Short: "Test complete AEC pipeline with real-time audio I/O",
		Long: `Tests the complete Acoustic Echo Cancellation pipeline including:
- Real-time audio capture and playback via PortAudio
- LiveKit WebRTC AudioProcessingModule  
- Dual-stream processing with output reference
- Performance monitoring and statistics
- Production-ready latency and quality validation`,
		RunE: runAECPipeline,
	}

	cmd.Flags().IntVar(&pipelineSampleRate, "sample-rate", 24000, "Audio sample rate in Hz")
	cmd.Flags().IntVar(&pipelineFrameSize, "frame-size", 240, "Frame size in samples (0 = auto-calculate)")
	cmd.Flags().IntVar(&pipelineDelayMs, "delay", 50, "Initial delay estimate in milliseconds")
	cmd.Flags().IntVar(&pipelineRunTime, "runtime", 30, "Test runtime in seconds (0 = run until Ctrl+C)")
	cmd.Flags().IntVar(&pipelineStatsInterval, "stats-interval", 5, "Statistics reporting interval in seconds")
	cmd.Flags().IntVar(&pipelineEchoLevel, "echo-level", 1, "Echo suppression level (0-2)")
	cmd.Flags().IntVar(&pipelineNoiseLevel, "noise-level", 2, "Noise suppression level (0-3)")
	cmd.Flags().BoolVar(&pipelineEnableAGC, "agc", true, "Enable automatic gain control")
	cmd.Flags().BoolVar(&pipelineEnableNS, "noise-suppress", true, "Enable noise suppression")
	cmd.Flags().BoolVar(&pipelinePlayTone, "play-tone", false, "Play test tone through output (for loopback testing)")
	cmd.Flags().IntVar(&pipelineToneFreq, "tone-freq", 1000, "Test tone frequency in Hz")

	return cmd
}

var (
	pipelineSampleRate int
	pipelineFrameSize  int
	pipelineDelayMs    int
	pipelineRunTime    int
	pipelineStatsInterval int
	pipelineEchoLevel  int
	pipelineNoiseLevel int
	pipelineEnableAGC  bool
	pipelineEnableNS   bool
	pipelinePlayTone   bool
	pipelineToneFreq   int
)

func runAECPipeline(cmd *cobra.Command, args []string) error {
	fmt.Printf("🎵 Starting AEC Pipeline Test\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Sample Rate: %d Hz\n", pipelineSampleRate)
	fmt.Printf("  Frame Size: %d samples (%.1f ms)\n", pipelineFrameSize, 
		float64(pipelineFrameSize*1000)/float64(pipelineSampleRate))
	fmt.Printf("  Delay Estimate: %d ms\n", pipelineDelayMs)
	fmt.Printf("  Echo Suppression: Level %d\n", pipelineEchoLevel)
	fmt.Printf("  Noise Suppression: %v (Level %d)\n", pipelineEnableNS, pipelineNoiseLevel)
	fmt.Printf("  Auto Gain Control: %v\n", pipelineEnableAGC)
	if pipelinePlayTone {
		fmt.Printf("  Test Tone: %d Hz\n", pipelineToneFreq)
	}
	fmt.Printf("\n")

	// Create AEC configuration
	config := audio.AECConfig{
		EnableEchoCancellation: true,
		EnableNoiseSuppression: pipelineEnableNS,
		EnableAutoGainControl:  pipelineEnableAGC,
		EchoSuppressionLevel:   pipelineEchoLevel,
		NoiseSuppressionLevel:  pipelineNoiseLevel,
		DelayMs:               pipelineDelayMs,
		SampleRate:            pipelineSampleRate,
		Channels:              1,
	}

	// Create and start AEC pipeline
	pipeline, err := audio.NewAECPipeline(config)
	if err != nil {
		return fmt.Errorf("failed to create AEC pipeline: %w", err)
	}
	defer pipeline.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start pipeline
	fmt.Println("🚀 Starting audio pipeline...")
	if err := pipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}
	defer pipeline.Stop()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start test tone generator if requested
	if pipelinePlayTone {
		go generateTestTone(ctx, pipeline.GetOutputChan(), pipelineSampleRate, pipelineToneFreq)
	}

	// Set up runtime limit if specified
	var runtimeTimer *time.Timer
	if pipelineRunTime > 0 {
		runtimeTimer = time.NewTimer(time.Duration(pipelineRunTime) * time.Second)
		defer runtimeTimer.Stop()
	}

	// Set up statistics reporting
	statsTicker := time.NewTicker(time.Duration(pipelineStatsInterval) * time.Second)
	defer statsTicker.Stop()

	fmt.Println("🎧 Pipeline running... Press Ctrl+C to stop")
	fmt.Println("📊 Real-time statistics:")
	
	startTime := time.Now()

	// Main loop
	for {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Received interrupt signal, stopping...")
			return nil

		case <-statsTicker.C:
			printPipelineStats(pipeline, time.Since(startTime))

		case <-func() <-chan time.Time {
			if runtimeTimer != nil {
				return runtimeTimer.C
			}
			// Return a channel that never sends if no timer
			return make(<-chan time.Time)
		}():
			fmt.Printf("\n⏰ Runtime limit reached (%d seconds)\n", pipelineRunTime)
			return nil

		default:
			// Process any input audio frames (just consume them for this test)
			select {
			case frame := <-pipeline.GetInputChan():
				if frame != nil {
					// In a real application, this would be sent to STT/LLM
					_ = frame
				}
			default:
				// No frame available, continue
			}
			
			time.Sleep(1 * time.Millisecond) // Small sleep to prevent busy waiting
		}
	}
}

func printPipelineStats(pipeline *audio.AECPipeline, runtime time.Duration) {
	stats := pipeline.GetStats()
	
	fmt.Printf("\n📊 [%v] Pipeline Statistics:\n", runtime.Round(time.Second))
	fmt.Printf("  ├─ Frames: %d processed, %d dropped (%.2f%% success)\n", 
		stats.FramesProcessed, stats.FramesDropped, 
		float64(stats.FramesProcessed)/float64(stats.FramesProcessed+stats.FramesDropped)*100)
	
	if stats.PipelineLatency > 0 {
		fmt.Printf("  ├─ Latency: %v (%.1f%% of frame time)\n", 
			stats.PipelineLatency.Round(time.Microsecond),
			stats.CPUUsage)
	}
	
	if stats.AECStats.FramesProcessed > 0 {
		fmt.Printf("  ├─ Echo Return Loss: %.1f dB\n", stats.AECStats.EchoReturnLoss)
		fmt.Printf("  ├─ Echo Suppression: %.1f dB\n", stats.AECStats.EchoReturnLossEnhancement)
		fmt.Printf("  ├─ AEC Delay: %d ms\n", stats.AECStats.DelayMs)
		fmt.Printf("  └─ AEC Latency: %.1f ms\n", stats.AECStats.ProcessingLatencyMs)
	} else {
		fmt.Printf("  └─ AEC: No frames processed yet\n")
	}
}

func generateTestTone(ctx context.Context, outputChan chan<- *media.AudioFrame, sampleRate, frequency int) {
	fmt.Printf("🎵 Generating %d Hz test tone for loopback testing...\n", frequency)
	
	frameSize := sampleRate / 100 // 10ms frames
	frameDuration := time.Duration(frameSize * int(time.Second) / sampleRate)
	
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()
	
	phase := 0.0
	phaseIncrement := 2.0 * 3.14159 * float64(frequency) / float64(sampleRate)
	
	format := media.AudioFormat{
		SampleRate:    sampleRate,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Generate sine wave samples
			pcmData := make([]byte, frameSize*2) // 16-bit samples
			
			for i := 0; i < frameSize; i++ {
				// Generate sine wave sample
				amplitude := 0.3 * math.Sin(phase)
				sample := int16(amplitude * 8000.0)
				
				// Apply fade in/out envelope
				envelope := 1.0
				if i < frameSize/10 {
					// Fade in
					envelope = float64(i) / float64(frameSize/10)
				} else if i > frameSize*9/10 {
					// Fade out
					envelope = float64(frameSize-i) / float64(frameSize/10)
				}
				
				sample = int16(float64(sample) * envelope)
				
				pcmData[i*2] = byte(sample & 0xFF)
				pcmData[i*2+1] = byte((sample >> 8) & 0xFF)
				
				phase += phaseIncrement
				if phase > 2.0*3.14159 {
					phase -= 2.0 * 3.14159
				}
			}
			
			frame := media.NewAudioFrame(pcmData, format)
			
			// Send frame to output (non-blocking)
			select {
			case outputChan <- frame:
			default:
				// Channel full, drop frame
			}
		}
	}
}