package cmd

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewDelayMeasurementCmd creates the delay measurement command
func NewDelayMeasurementCmd() *cobra.Command {
	var (
		duration    time.Duration
		toneFreq    float64
		volume      float64
		sampleRate  int
	)

	cmd := &cobra.Command{
		Use:   "delay-test",
		Short: "Measure DAC-to-ADC delay for AEC calibration",
		Long: `Measure the actual delay between audio output (DAC) and input (ADC) by playing a test tone
and detecting when it appears in the microphone input. This is critical for AEC implementation.

The test works by:
1. Playing a sine wave test tone through speakers
2. Recording microphone input simultaneously  
3. Using cross-correlation to find the delay between output and input
4. Calculating the precise DAC-to-ADC delay

Examples:
  pipeline-test delay-test --duration 5s --tone-freq 1000    # 5 second test with 1kHz tone
  pipeline-test delay-test --volume 0.3                     # Quieter test tone`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelayMeasurement(duration, toneFreq, volume, sampleRate)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 3*time.Second, "measurement duration")
	cmd.Flags().Float64Var(&toneFreq, "tone-freq", 1000.0, "test tone frequency in Hz")
	cmd.Flags().Float64Var(&volume, "volume", 0.2, "test tone volume (0.0-1.0)")
	cmd.Flags().IntVar(&sampleRate, "sample-rate", 48000, "audio sample rate")

	return cmd
}

// runDelayMeasurement performs DAC-to-ADC delay measurement
func runDelayMeasurement(duration time.Duration, toneFreq, volume float64, sampleRate int) error {
	fmt.Println("🔬 DAC-to-ADC Delay Measurement")
	fmt.Println("=====================================")
	fmt.Printf("🔊 Test tone: %.0f Hz at %.1f%% volume\n", toneFreq, volume*100)
	fmt.Printf("⏱️  Duration: %v\n", duration)
	fmt.Printf("🎵 Sample rate: %d Hz\n", sampleRate)
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Make sure speakers and microphone are active!")
	fmt.Println("📢 You should hear a test tone during measurement.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	// Create audio I/O with specified sample rate
	config := audio.DefaultConfig()
	config.SampleRate = sampleRate
	audioIO, err := audio.NewLocalAudioIO(config)
	if err != nil {
		return fmt.Errorf("failed to create audio I/O: %w", err)
	}
	defer audioIO.Close()

	// Start audio I/O
	fmt.Println("🚀 Starting audio I/O...")
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O: %w", err)
	}
	defer audioIO.Stop()

	// Start delay measurement
	fmt.Println("🔬 Starting delay measurement...")
	delay, confidence, err := measureDelay(ctx, audioIO, duration, toneFreq, volume, sampleRate)
	if err != nil {
		return fmt.Errorf("delay measurement failed: %w", err)
	}

	// Report results
	fmt.Println("\n📊 DELAY MEASUREMENT RESULTS")
	fmt.Println("===============================")
	fmt.Printf("⏱️  Measured delay: %.2f ms\n", delay.Seconds()*1000)
	fmt.Printf("🎯 Confidence: %.1f%%\n", confidence*100)
	fmt.Printf("📏 Delay in samples: %.1f samples\n", delay.Seconds()*float64(sampleRate))
	fmt.Println()

	if confidence > 0.7 {
		fmt.Println("✅ HIGH CONFIDENCE - Delay measurement is reliable")
		fmt.Printf("🔧 Recommended AEC delay setting: %.0f ms\n", delay.Seconds()*1000)
	} else if confidence > 0.4 {
		fmt.Println("⚠️  MEDIUM CONFIDENCE - Try again in quieter environment")
		fmt.Printf("🔧 Tentative AEC delay setting: %.0f ms\n", delay.Seconds()*1000)
	} else {
		fmt.Println("❌ LOW CONFIDENCE - Measurement unreliable")
		fmt.Println("🔧 Suggestions:")
		fmt.Println("   - Increase volume with --volume flag")
		fmt.Println("   - Reduce background noise")
		fmt.Println("   - Check speaker/microphone positioning")
		fmt.Println("   - Try different tone frequency with --tone-freq")
	}

	return nil
}

// measureDelay performs the actual delay measurement using cross-correlation
func measureDelay(ctx context.Context, audioIO *audio.LocalAudioIO, duration time.Duration, 
	toneFreq, volume float64, sampleRate int) (time.Duration, float64, error) {
	
	inputChan := audioIO.InputChan()
	outputChan := audioIO.OutputChan()

	// Generate test tone
	frameDuration := time.Duration(1024) * time.Second / time.Duration(sampleRate) // ~21ms at 48kHz
	toneGenerator := NewToneGenerator(toneFreq, volume, sampleRate)

	// Storage for correlation analysis
	var outputSamples []int16
	var inputSamples []int16
	
	timer := time.After(duration)
	frameCount := 0

	fmt.Println("🎵 Playing test tone and recording...")

	for {
		select {
		case <-ctx.Done():
			return 0, 0, fmt.Errorf("measurement cancelled")
		case <-timer:
			fmt.Printf("📊 Collected %d frames for analysis\n", frameCount)
			goto analyze
		case frame := <-inputChan:
			if frame != nil {
				// Store input samples for correlation
				samples := extractSamples(frame)
				inputSamples = append(inputSamples, samples...)
			}
		default:
			// Generate and send test tone
			toneFrame := toneGenerator.GenerateFrame(frameDuration)
			select {
			case outputChan <- toneFrame:
				// Store output samples for correlation
				samples := extractSamples(toneFrame)
				outputSamples = append(outputSamples, samples...)
				frameCount++
			case <-time.After(5 * time.Millisecond):
				// Skip if output buffer full
			}
		}
	}

analyze:
	if len(outputSamples) == 0 || len(inputSamples) == 0 {
		return 0, 0, fmt.Errorf("insufficient audio data collected")
	}

	fmt.Println("🧮 Analyzing cross-correlation...")
	delay, confidence := crossCorrelateDelay(outputSamples, inputSamples, sampleRate)
	
	return delay, confidence, nil
}

// ToneGenerator generates sine wave test tones
type ToneGenerator struct {
	frequency  float64
	volume     float64
	sampleRate int
	phase      float64
}

func NewToneGenerator(frequency, volume float64, sampleRate int) *ToneGenerator {
	return &ToneGenerator{
		frequency:  frequency,
		volume:     volume,
		sampleRate: sampleRate,
		phase:      0,
	}
}

func (tg *ToneGenerator) GenerateFrame(duration time.Duration) *media.AudioFrame {
	sampleCount := int(duration.Seconds() * float64(tg.sampleRate))
	data := make([]byte, sampleCount*2) // 16-bit samples

	phaseIncrement := 2 * math.Pi * tg.frequency / float64(tg.sampleRate)

	for i := 0; i < sampleCount; i++ {
		// Generate sine wave sample
		sample := math.Sin(tg.phase) * tg.volume * 32767
		tg.phase += phaseIncrement
		
		// Keep phase in range [0, 2π]
		if tg.phase > 2*math.Pi {
			tg.phase -= 2 * math.Pi
		}

		// Convert to 16-bit little-endian
		sampleInt := int16(sample)
		data[i*2] = byte(sampleInt & 0xFF)
		data[i*2+1] = byte((sampleInt >> 8) & 0xFF)
	}

	format := media.AudioFormat{
		SampleRate:    tg.sampleRate,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	frame := media.NewAudioFrame(data, format)
	frame.Metadata["tone_frequency"] = tg.frequency
	frame.Metadata["tone_volume"] = tg.volume
	
	return frame
}

// extractSamples extracts 16-bit samples from audio frame
func extractSamples(frame *media.AudioFrame) []int16 {
	if frame.Format.BitsPerSample != 16 || frame.Format.Format != media.AudioFormatPCM {
		return nil
	}

	data := frame.Data
	samples := make([]int16, len(data)/2)

	for i := 0; i < len(samples); i++ {
		samples[i] = int16(data[i*2]) | (int16(data[i*2+1]) << 8)
	}

	return samples
}

// crossCorrelateDelay finds delay using cross-correlation
func crossCorrelateDelay(output, input []int16, sampleRate int) (time.Duration, float64) {
	// Limit search to reasonable delay range (0-500ms)
	maxDelaySamples := sampleRate / 2 // 500ms at given sample rate
	if maxDelaySamples > len(output) {
		maxDelaySamples = len(output)
	}

	// Ensure we have enough input samples
	minLen := len(output)
	if len(input) < minLen+maxDelaySamples {
		minLen = len(input) - maxDelaySamples
		if minLen <= 0 {
			return 0, 0
		}
	}

	// Find peak correlation
	maxCorrelation := 0.0
	bestDelay := 0
	
	for delay := 0; delay < maxDelaySamples; delay++ {
		correlation := 0.0
		
		for i := 0; i < minLen; i++ {
			if delay+i < len(input) {
				correlation += float64(output[i]) * float64(input[delay+i])
			}
		}
		
		if math.Abs(correlation) > math.Abs(maxCorrelation) {
			maxCorrelation = correlation
			bestDelay = delay
		}
	}

	// Calculate confidence based on correlation strength
	// Normalize correlation by signal energy
	outputEnergy := 0.0
	inputEnergy := 0.0
	
	for i := 0; i < minLen; i++ {
		outputEnergy += float64(output[i]) * float64(output[i])
		if bestDelay+i < len(input) {
			inputEnergy += float64(input[bestDelay+i]) * float64(input[bestDelay+i])
		}
	}
	
	normalizedCorrelation := 0.0
	if outputEnergy > 0 && inputEnergy > 0 {
		normalizedCorrelation = math.Abs(maxCorrelation) / math.Sqrt(outputEnergy*inputEnergy)
	}
	
	confidence := normalizedCorrelation
	if confidence > 1.0 {
		confidence = 1.0
	}

	delayTime := time.Duration(bestDelay) * time.Second / time.Duration(sampleRate)
	return delayTime, confidence
}