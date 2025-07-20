package cmd

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewDelayMeasurementV2Cmd creates an improved delay measurement command
func NewDelayMeasurementV2Cmd() *cobra.Command {
	var (
		duration    time.Duration
		toneFreq    float64
		volume      float64
		sampleRate  int
	)

	cmd := &cobra.Command{
		Use:   "delay-test-v2",
		Short: "Measure DAC-to-ADC delay (improved version)",
		Long: `Improved delay measurement tool with better error handling and algorithm robustness.

This version uses:
- Safe buffer management with bounds checking
- Synchronized audio collection
- Improved cross-correlation algorithm
- Better noise handling

Examples:
  pipeline-test delay-test-v2 --duration 3s                 # Standard test
  pipeline-test delay-test-v2 --tone-freq 440 --volume 0.4  # Custom settings`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelayMeasurementV2(duration, toneFreq, volume, sampleRate)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 3*time.Second, "measurement duration")
	cmd.Flags().Float64Var(&toneFreq, "tone-freq", 1000.0, "test tone frequency in Hz")
	cmd.Flags().Float64Var(&volume, "volume", 0.3, "test tone volume (0.0-1.0)")
	cmd.Flags().IntVar(&sampleRate, "sample-rate", 48000, "audio sample rate")

	return cmd
}

// DelayMeasurement holds synchronized audio data for analysis
type DelayMeasurement struct {
	outputSamples []int16
	inputSamples  []int16
	sampleRate    int
	mu            sync.RWMutex
	collecting    bool
}

func NewDelayMeasurement(sampleRate int) *DelayMeasurement {
	return &DelayMeasurement{
		sampleRate: sampleRate,
		collecting: true,
	}
}

func (dm *DelayMeasurement) AddOutputSamples(samples []int16) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if dm.collecting && len(samples) > 0 {
		dm.outputSamples = append(dm.outputSamples, samples...)
	}
}

func (dm *DelayMeasurement) AddInputSamples(samples []int16) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if dm.collecting && len(samples) > 0 {
		dm.inputSamples = append(dm.inputSamples, samples...)
	}
}

func (dm *DelayMeasurement) StopCollecting() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.collecting = false
}

func (dm *DelayMeasurement) GetSampleCounts() (int, int) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.outputSamples), len(dm.inputSamples)
}

func (dm *DelayMeasurement) AnalyzeDelay() (time.Duration, float64, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	if len(dm.outputSamples) < 1000 || len(dm.inputSamples) < 1000 {
		return 0, 0, fmt.Errorf("insufficient samples: output=%d, input=%d", len(dm.outputSamples), len(dm.inputSamples))
	}
	
	return dm.crossCorrelateDelay()
}

func runDelayMeasurementV2(duration time.Duration, toneFreq, volume float64, sampleRate int) error {
	fmt.Println("🔬 DAC-to-ADC Delay Measurement V2")
	fmt.Println("==================================")
	fmt.Printf("🔊 Test tone: %.0f Hz at %.1f%% volume\n", toneFreq, volume*100)
	fmt.Printf("⏱️  Duration: %v\n", duration)
	fmt.Printf("🎵 Sample rate: %d Hz\n", sampleRate)
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Ensure speakers and microphone are active!")
	fmt.Println("📢 You will hear a test tone during measurement.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	// Create audio I/O
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

	// Create delay measurement
	measurement := NewDelayMeasurement(sampleRate)
	
	// Start measurement
	fmt.Println("🔬 Starting delay measurement...")
	delay, confidence, err := measureDelayV2(ctx, audioIO, measurement, duration, toneFreq, volume, sampleRate)
	if err != nil {
		return fmt.Errorf("delay measurement failed: %w", err)
	}

	// Report results
	fmt.Println("\n📊 DELAY MEASUREMENT RESULTS")
	fmt.Println("===============================")
	fmt.Printf("⏱️  Measured delay: %.2f ms\n", delay.Seconds()*1000)
	fmt.Printf("🎯 Confidence: %.1f%%\n", confidence*100)
	fmt.Printf("📏 Delay in samples: %.1f samples\n", delay.Seconds()*float64(sampleRate))
	
	outputCount, inputCount := measurement.GetSampleCounts()
	fmt.Printf("📊 Collected samples: output=%d, input=%d\n", outputCount, inputCount)
	fmt.Println()

	// Provide recommendations
	if confidence > 0.7 {
		fmt.Println("✅ HIGH CONFIDENCE - Delay measurement is reliable")
		fmt.Printf("🔧 Recommended AEC delay setting: %.0f ms\n", delay.Seconds()*1000)
	} else if confidence > 0.4 {
		fmt.Println("⚠️  MEDIUM CONFIDENCE - Measurement may be accurate")
		fmt.Printf("🔧 Suggested AEC delay setting: %.0f ms\n", delay.Seconds()*1000)
		fmt.Println("💡 Try running the test again for consistency")
	} else {
		fmt.Println("❌ LOW CONFIDENCE - Measurement unreliable")
		fmt.Println("🔧 Troubleshooting suggestions:")
		fmt.Println("   - Ensure speakers are audible to microphone")
		fmt.Println("   - Increase --volume (currently %.1f)", volume)
		fmt.Println("   - Reduce background noise") 
		fmt.Println("   - Check audio device configuration")
		fmt.Printf("   - Try different --tone-freq (currently %.0f Hz)\n", toneFreq)
	}

	return nil
}

func measureDelayV2(parentCtx context.Context, audioIO *audio.LocalAudioIO, measurement *DelayMeasurement,
	duration time.Duration, toneFreq, volume float64, sampleRate int) (time.Duration, float64, error) {
	
	// Create measurement context that we can cancel
	ctx, cancel := context.WithCancel(parentCtx)
	
	inputChan := audioIO.InputChan()
	outputChan := audioIO.OutputChan()

	// Create tone generator
	toneGen := NewSafeToneGenerator(toneFreq, volume, sampleRate)
	
	// Start measurement goroutines
	var wg sync.WaitGroup
	
	// Input collection goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		frameCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-inputChan:
				if !ok {
					return
				}
				if frame != nil {
					samples := safeExtractSamples(frame)
					if len(samples) > 0 {
						measurement.AddInputSamples(samples)
						frameCount++
					}
				}
			}
		}
	}()
	
	// Output generation and collection goroutine  
	wg.Add(1)
	go func() {
		defer wg.Done()
		frameCount := 0
		ticker := time.NewTicker(20 * time.Millisecond) // ~50 FPS
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Generate tone frame
				toneFrame := toneGen.GenerateFrame(20 * time.Millisecond)
				if toneFrame != nil {
					samples := safeExtractSamples(toneFrame)
					if len(samples) > 0 {
						measurement.AddOutputSamples(samples)
						
						// Send to output (non-blocking)
						select {
						case outputChan <- toneFrame:
							frameCount++
						case <-time.After(5 * time.Millisecond):
							// Skip if output buffer full
						}
					}
				}
			}
		}
	}()

	fmt.Println("🎵 Playing test tone and recording...")
	
	// Wait for measurement duration
	measurementTimer := time.After(duration)
	select {
	case <-measurementTimer:
		fmt.Println("⏱️  Measurement duration completed")
	case <-ctx.Done():
		return 0, 0, fmt.Errorf("measurement cancelled")
	}
	
	// Stop collecting and wait for goroutines
	measurement.StopCollecting()
	
	// Cancel context to stop goroutines
	cancel()
	
	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Goroutines finished successfully
	case <-time.After(2 * time.Second):
		fmt.Println("⚠️  Warning: Some goroutines may not have finished cleanly")
	}
	
	outputCount, inputCount := measurement.GetSampleCounts()
	fmt.Printf("📊 Analysis: collected %d output samples, %d input samples\n", outputCount, inputCount)
	
	// Analyze delay
	return measurement.AnalyzeDelay()
}

// SafeToneGenerator generates test tones with better error handling
type SafeToneGenerator struct {
	frequency  float64
	volume     float64
	sampleRate int
	phase      float64
	mu         sync.Mutex
}

func NewSafeToneGenerator(frequency, volume float64, sampleRate int) *SafeToneGenerator {
	return &SafeToneGenerator{
		frequency:  frequency,
		volume:     math.Min(volume, 1.0), // Clamp volume
		sampleRate: sampleRate,
		phase:      0,
	}
}

func (tg *SafeToneGenerator) GenerateFrame(duration time.Duration) *media.AudioFrame {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	
	if duration <= 0 {
		return nil
	}
	
	sampleCount := int(duration.Seconds() * float64(tg.sampleRate))
	if sampleCount <= 0 || sampleCount > 100000 { // Sanity check
		return nil
	}
	
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

		// Convert to 16-bit little-endian with bounds checking
		sampleInt := int16(math.Max(-32768, math.Min(32767, sample)))
		data[i*2] = byte(sampleInt & 0xFF)
		data[i*2+1] = byte((sampleInt >> 8) & 0xFF)
	}

	format := media.AudioFormat{
		SampleRate:    tg.sampleRate,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	return media.NewAudioFrame(data, format)
}

// safeExtractSamples safely extracts samples with bounds checking
func safeExtractSamples(frame *media.AudioFrame) []int16 {
	if frame == nil || frame.Format.BitsPerSample != 16 || frame.Format.Format != media.AudioFormatPCM {
		return nil
	}

	data := frame.Data
	if len(data)%2 != 0 {
		return nil // Invalid data length for 16-bit samples
	}

	samples := make([]int16, len(data)/2)
	for i := 0; i < len(samples); i++ {
		if i*2+1 < len(data) {
			samples[i] = int16(data[i*2]) | (int16(data[i*2+1]) << 8)
		}
	}

	return samples
}

// crossCorrelateDelay performs safe cross-correlation analysis
func (dm *DelayMeasurement) crossCorrelateDelay() (time.Duration, float64, error) {
	// Validate input data
	if len(dm.outputSamples) == 0 || len(dm.inputSamples) == 0 {
		return 0, 0, fmt.Errorf("no samples available for analysis")
	}
	
	// Limit analysis to reasonable ranges to prevent crashes
	maxOutputSamples := 48000 * 2  // 2 seconds max
	maxInputSamples := 48000 * 3   // 3 seconds max
	maxDelaySamples := 48000 / 2   // 500ms max delay
	
	outputSamples := dm.outputSamples
	if len(outputSamples) > maxOutputSamples {
		outputSamples = outputSamples[:maxOutputSamples]
	}
	
	inputSamples := dm.inputSamples
	if len(inputSamples) > maxInputSamples {
		inputSamples = inputSamples[:maxInputSamples]
	}
	
	if maxDelaySamples > len(outputSamples) {
		maxDelaySamples = len(outputSamples)
	}
	
	if maxDelaySamples > len(inputSamples) {
		maxDelaySamples = len(inputSamples)
	}
	
	// Ensure we have minimum samples for reliable correlation
	minSamples := 1000
	if len(outputSamples) < minSamples || len(inputSamples) < minSamples {
		return 0, 0, fmt.Errorf("insufficient samples for reliable analysis: output=%d, input=%d", len(outputSamples), len(inputSamples))
	}
	
	// Find correlation peak with safe bounds checking
	maxCorrelation := 0.0
	bestDelay := 0
	correlationLength := len(outputSamples)
	
	if correlationLength > len(inputSamples)-maxDelaySamples {
		correlationLength = len(inputSamples) - maxDelaySamples
	}
	
	if correlationLength <= 0 {
		return 0, 0, fmt.Errorf("insufficient data for correlation analysis")
	}
	
	fmt.Printf("🧮 Analyzing correlation: output=%d, input=%d, maxDelay=%d, corrLen=%d\n", 
		len(outputSamples), len(inputSamples), maxDelaySamples, correlationLength)
	
	for delay := 0; delay < maxDelaySamples; delay++ {
		correlation := 0.0
		
		for i := 0; i < correlationLength; i++ {
			if delay+i < len(inputSamples) && i < len(outputSamples) {
				correlation += float64(outputSamples[i]) * float64(inputSamples[delay+i])
			}
		}
		
		if math.Abs(correlation) > math.Abs(maxCorrelation) {
			maxCorrelation = correlation
			bestDelay = delay
		}
	}
	
	// Calculate confidence with proper normalization
	outputEnergy := 0.0
	inputEnergy := 0.0
	
	for i := 0; i < correlationLength; i++ {
		if i < len(outputSamples) {
			outputEnergy += float64(outputSamples[i]) * float64(outputSamples[i])
		}
		if bestDelay+i < len(inputSamples) {
			inputEnergy += float64(inputSamples[bestDelay+i]) * float64(inputSamples[bestDelay+i])
		}
	}
	
	confidence := 0.0
	if outputEnergy > 0 && inputEnergy > 0 {
		normalizedCorrelation := math.Abs(maxCorrelation) / math.Sqrt(outputEnergy*inputEnergy)
		confidence = math.Min(normalizedCorrelation, 1.0)
	}
	
	delayTime := time.Duration(bestDelay) * time.Second / time.Duration(dm.sampleRate)
	
	fmt.Printf("🎯 Correlation analysis: bestDelay=%d samples, correlation=%.2e, confidence=%.3f\n", 
		bestDelay, maxCorrelation, confidence)
	
	return delayTime, confidence, nil
}