package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewAudioCmd creates the audio testing command
func NewAudioCmd() *cobra.Command {
	var (
		duration   time.Duration
		loopback   bool
		deviceInfo bool
		volume     float64
	)

	cmd := &cobra.Command{
		Use:   "audio",
		Short: "Test basic audio I/O functionality",
		Long: `Test basic audio input/output functionality to validate the audio pipeline foundation.

This command tests the core audio I/O that all other services depend on. If this doesn't work,
nothing else will work properly.

Examples:
  pipeline-test audio --loopback --duration 10s    # Test mic → speaker for 10 seconds
  pipeline-test audio --device-info                # Show available audio devices
  pipeline-test audio --duration 5s                # Basic audio capture test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if deviceInfo {
				return showDeviceInfo()
			}

			if loopback {
				return runLoopbackTest(duration, volume)
			}

			return runBasicAudioTest(duration)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 10*time.Second, "test duration")
	cmd.Flags().BoolVarP(&loopback, "loopback", "l", false, "run microphone to speaker loopback test")
	cmd.Flags().BoolVar(&deviceInfo, "device-info", false, "show available audio devices")
	cmd.Flags().Float64Var(&volume, "volume", 0.5, "volume level for loopback test (0.0-1.0)")

	return cmd
}

// showDeviceInfo displays available audio devices
func showDeviceInfo() error {
	fmt.Println("🎤 Audio Device Information")
	fmt.Println("=========================================")
	
	// Create a temporary audio I/O to get device info
	audioIO, err := audio.NewLocalAudioIO(audio.DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()

	// Start briefly to initialize and get device info
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O for device detection: %w", err)
	}
	
	audioIO.Stop()

	fmt.Println("Device enumeration completed.")
	fmt.Println("\nNote: Detailed device information is logged during audio I/O startup.")
	fmt.Println("Use --verbose flag to see full device details.")
	
	return nil
}

// runLoopbackTest runs a microphone to speaker loopback test
func runLoopbackTest(duration time.Duration, volume float64) error {
	fmt.Printf("🔄 Starting loopback test for %v...\n", duration)
	fmt.Printf("🔊 Volume level: %.1f\n", volume)
	fmt.Println("🎤 Speak into the microphone - you should hear your voice through the speakers!")
	fmt.Println("This tests the complete audio I/O pipeline that agents depend on.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	defer cancel()

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
	outputChan := audioIO.OutputChan()

	fmt.Println("🔴 Loopback active - speak now!")
	fmt.Println("Press Ctrl+C to stop early")

	// Set up timer
	timer := time.After(duration)
	frameCount := 0
	lastActivityTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n⏹️  Test stopped: %v\n", ctx.Err())
			return nil
		case <-timer:
			fmt.Printf("\n⏹️  Loopback test completed. Processed %d frames.\n", frameCount)
			return nil
		case frame, ok := <-inputChan:
			if !ok {
				return fmt.Errorf("audio input channel closed unexpectedly")
			}

			frameCount++

			// Apply volume adjustment if needed
			if volume != 1.0 {
				adjustFrameVolume(frame, volume)
			}

			// Pass audio directly from input to output (loopback)
			select {
			case outputChan <- frame:
				// Successfully sent to output
			case <-time.After(10 * time.Millisecond):
				// Skip this frame if output is not ready
			}

			// Show activity indicator
			if frameCount%20 == 0 { // Every ~2 seconds at 10fps
				energy := calculateFrameEnergy(frame)
				if energy > 0.001 {
					fmt.Printf("🎤➡️🔊 Activity: %.3f (frame %d)\n", energy, frameCount)
					lastActivityTime = time.Now()
				} else if time.Since(lastActivityTime) > 5*time.Second {
					fmt.Printf("🔇 Listening... (frame %d)\n", frameCount)
					lastActivityTime = time.Now()
				}
			}
		}
	}
}

// runBasicAudioTest runs a basic audio capture test
func runBasicAudioTest(duration time.Duration) error {
	fmt.Printf("🎤 Starting basic audio capture test for %v...\n", duration)
	fmt.Println("This test captures audio input and reports activity levels.")
	fmt.Println("Speak into the microphone to see audio activity.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	defer cancel()

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

	fmt.Println("🔴 Recording active - speak now!")
	fmt.Println("Press Ctrl+C to stop early")

	// Set up timer
	timer := time.After(duration)
	frameCount := 0
	totalEnergy := 0.0
	lastActivityTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n⏹️  Test stopped: %v\n", ctx.Err())
			return nil
		case <-timer:
			avgEnergy := totalEnergy / float64(frameCount)
			fmt.Printf("\n⏹️  Capture test completed.\n")
			fmt.Printf("📊 Stats: %d frames, average energy: %.6f\n", frameCount, avgEnergy)
			return nil
		case frame, ok := <-inputChan:
			if !ok {
				return fmt.Errorf("audio input channel closed unexpectedly")
			}

			frameCount++
			energy := calculateFrameEnergy(frame)
			totalEnergy += energy

			// Show activity indicator
			if frameCount%20 == 0 { // Every ~2 seconds at 10fps
				if energy > 0.001 {
					fmt.Printf("🎤 Activity: %.6f (frame %d)\n", energy, frameCount)
					lastActivityTime = time.Now()
				} else if time.Since(lastActivityTime) > 5*time.Second {
					fmt.Printf("🔇 Listening... (frame %d, avg energy: %.6f)\n", frameCount, totalEnergy/float64(frameCount))
					lastActivityTime = time.Now()
				}
			}
		}
	}
}

// adjustFrameVolume adjusts the volume of an audio frame
func adjustFrameVolume(frame *media.AudioFrame, volume float64) {
	// Simple volume adjustment for 16-bit PCM
	if frame.Format.BitsPerSample == 16 && frame.Format.Format == media.AudioFormatPCM {
		data := frame.Data
		for i := 0; i < len(data)-1; i += 2 {
			// Read 16-bit sample
			sample := int16(data[i]) | (int16(data[i+1]) << 8)
			
			// Apply volume
			adjusted := int16(float64(sample) * volume)
			
			// Write back
			data[i] = byte(adjusted & 0xFF)
			data[i+1] = byte((adjusted >> 8) & 0xFF)
		}
	}
}

// calculateFrameEnergy calculates the RMS energy of an audio frame
func calculateFrameEnergy(frame *media.AudioFrame) float64 {
	if frame.Format.BitsPerSample != 16 || frame.Format.Format != media.AudioFormatPCM {
		return 0.0
	}

	data := frame.Data
	var sum float64
	sampleCount := len(data) / 2

	for i := 0; i < len(data)-1; i += 2 {
		// Read 16-bit sample
		sample := int16(data[i]) | (int16(data[i+1]) << 8)
		normalized := float64(sample) / 32767.0
		sum += normalized * normalized
	}

	if sampleCount == 0 {
		return 0.0
	}

	return sum / float64(sampleCount)
}