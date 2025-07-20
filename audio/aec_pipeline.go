package audio

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"livekit-agents-go/media"
)

// AECPipeline manages the complete acoustic echo cancellation pipeline
// integrating audio I/O with AEC processing in a production-ready system
type AECPipeline struct {
	config AECConfig
	audioIO *LocalAudioIO
	processor AECProcessor
	
	// Pipeline state
	running bool
	mu sync.RWMutex
	wg sync.WaitGroup
	
	// Statistics and monitoring
	stats PipelineStats
	statsMu sync.RWMutex
	
	// Frame processing
	frameSize int
	
	// Context for pipeline shutdown
	ctx context.Context
	cancel context.CancelFunc
}

// PipelineStats provides comprehensive statistics for the AEC pipeline
type PipelineStats struct {
	// Audio I/O stats
	FramesProcessed uint64
	FramesDropped uint64
	InputLatency time.Duration
	OutputLatency time.Duration
	
	// AEC processing stats
	AECStats AECStats
	
	// Pipeline performance
	PipelineLatency time.Duration
	CPUUsage float64
	
	// Timing information
	StartTime time.Time
	LastFrameTime time.Time
}

// NewAECPipeline creates a new AEC pipeline with the specified configuration
func NewAECPipeline(config AECConfig) (*AECPipeline, error) {
	// Validate configuration
	if config.SampleRate <= 0 {
		config.SampleRate = 24000 // Default to 24kHz for optimal AEC performance
	}
	if config.Channels <= 0 {
		config.Channels = 1 // Mono audio for AEC
	}
	
	// Calculate frame size for 10ms processing (optimal for real-time AEC)
	frameSize := config.SampleRate / 100 // 10ms at sample rate
	
	// Create audio I/O configuration
	audioConfig := Config{
		SampleRate: config.SampleRate,
		Channels: config.Channels,
		BitDepth: 16,
		FramesPerBuffer: frameSize,
		EnableAECProcessing: true,
		EstimatedDelay: time.Duration(config.DelayMs) * time.Millisecond,
	}
	
	// Create audio I/O
	audioIO, err := NewLocalAudioIO(audioConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio I/O: %w", err)
	}
	
	// Create AEC processor
	var processor AECProcessor
	if config.EnableEchoCancellation {
		processor, err = NewLiveKitAECProcessor(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create LiveKit AEC processor: %w", err)
		}
	} else {
		// Use pass-through processor when AEC is disabled
		processor = NewMockAECProcessor(config)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	pipeline := &AECPipeline{
		config: config,
		audioIO: audioIO,
		processor: processor,
		frameSize: frameSize,
		ctx: ctx,
		cancel: cancel,
		stats: PipelineStats{
			StartTime: time.Now(),
		},
	}
	
	return pipeline, nil
}

// Start starts the AEC pipeline
func (p *AECPipeline) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.running {
		return fmt.Errorf("AEC pipeline already running")
	}
	
	// Set up AEC processing callback
	p.audioIO.SetAudioProcessingCallback(p.processAudioFrame)
	
	// Start audio I/O
	if err := p.audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O: %w", err)
	}
	
	p.running = true
	p.stats.StartTime = time.Now()
	
	// Start statistics monitoring
	p.wg.Add(1)
	go p.monitorStats()
	
	fmt.Printf("🎵 AEC Pipeline started (Sample Rate: %d Hz, Frame Size: %d samples)\n", 
		p.config.SampleRate, p.frameSize)
	
	return nil
}

// Stop stops the AEC pipeline
func (p *AECPipeline) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.running {
		return nil
	}
	
	p.running = false
	
	// Cancel context to signal shutdown
	p.cancel()
	
	// Stop audio I/O
	if err := p.audioIO.Stop(); err != nil {
		fmt.Printf("⚠️  Error stopping audio I/O: %v\n", err)
	}
	
	// Wait for monitoring goroutines to finish
	p.wg.Wait()
	
	fmt.Println("🛑 AEC Pipeline stopped")
	return nil
}

// Close closes the AEC pipeline and releases all resources
func (p *AECPipeline) Close() error {
	if err := p.Stop(); err != nil {
		return err
	}
	
	// Close processor
	if p.processor != nil {
		if err := p.processor.Close(); err != nil {
			fmt.Printf("⚠️  Error closing AEC processor: %v\n", err)
		}
	}
	
	// Close audio I/O
	if p.audioIO != nil {
		if err := p.audioIO.Close(); err != nil {
			fmt.Printf("⚠️  Error closing audio I/O: %v\n", err)
		}
	}
	
	return nil
}

// GetInputChan returns the input audio channel for processed audio
func (p *AECPipeline) GetInputChan() <-chan *media.AudioFrame {
	return p.audioIO.InputChan()
}

// GetOutputChan returns the output audio channel for playback
func (p *AECPipeline) GetOutputChan() chan<- *media.AudioFrame {
	return p.audioIO.OutputChan()
}

// GetStats returns current pipeline statistics
func (p *AECPipeline) GetStats() PipelineStats {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()
	
	stats := p.stats
	stats.AECStats = p.processor.GetStats()
	return stats
}

// SetDelay updates the estimated delay for AEC processing
func (p *AECPipeline) SetDelay(delay time.Duration) error {
	// Update audio I/O delay
	p.audioIO.SetEstimatedDelay(delay)
	
	// Update AEC processor delay
	return p.processor.SetDelay(delay)
}

// processAudioFrame is the callback function for audio processing with AEC
func (p *AECPipeline) processAudioFrame(inputFrame, outputReferenceFrame *media.AudioFrame) (*media.AudioFrame, error) {
	startTime := time.Now()
	
	// Update statistics
	p.statsMu.Lock()
	p.stats.FramesProcessed++
	p.stats.LastFrameTime = startTime
	p.statsMu.Unlock()
	
	// Validate frame format matches our configuration
	if inputFrame.Format.SampleRate != p.config.SampleRate {
		p.statsMu.Lock()
		p.stats.FramesDropped++
		p.statsMu.Unlock()
		return nil, fmt.Errorf("input frame sample rate mismatch: expected %d, got %d", 
			p.config.SampleRate, inputFrame.Format.SampleRate)
	}
	
	// Process frame with AEC
	var processedFrame *media.AudioFrame
	var err error
	
	if outputReferenceFrame != nil {
		// Full AEC processing with output reference
		processedFrame, err = p.processor.ProcessStreams(p.ctx, inputFrame, outputReferenceFrame)
	} else {
		// Input-only processing (noise suppression, AGC)
		processedFrame, err = p.processor.ProcessInput(p.ctx, inputFrame)
	}
	
	if err != nil {
		p.statsMu.Lock()
		p.stats.FramesDropped++
		p.statsMu.Unlock()
		return inputFrame, fmt.Errorf("AEC processing failed: %w", err)
	}
	
	// Update processing latency statistics
	processingLatency := time.Since(startTime)
	p.statsMu.Lock()
	p.stats.PipelineLatency = processingLatency
	p.statsMu.Unlock()
	
	// Add processing metadata to frame
	if processedFrame.Metadata == nil {
		processedFrame.Metadata = make(map[string]interface{})
	}
	processedFrame.Metadata["aec_pipeline_processed"] = true
	processedFrame.Metadata["processing_latency_us"] = processingLatency.Microseconds()
	processedFrame.Metadata["has_output_reference"] = outputReferenceFrame != nil
	
	return processedFrame, nil
}

// monitorStats periodically updates pipeline performance statistics
func (p *AECPipeline) monitorStats() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.updatePerformanceStats()
		}
	}
}

// updatePerformanceStats calculates and updates performance metrics
func (p *AECPipeline) updatePerformanceStats() {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	
	// Calculate approximate CPU usage based on processing latency
	// For 10ms frames, processing should be much less than 10ms for real-time
	frameInterval := time.Duration(p.frameSize * int(time.Second) / p.config.SampleRate)
	if p.stats.PipelineLatency > 0 && frameInterval > 0 {
		p.stats.CPUUsage = float64(p.stats.PipelineLatency) / float64(frameInterval) * 100.0
	}
}

// GetAudioIO returns the underlying audio I/O for advanced configuration
func (p *AECPipeline) GetAudioIO() *LocalAudioIO {
	return p.audioIO
}

// GetProcessor returns the underlying AEC processor for advanced configuration
func (p *AECPipeline) GetProcessor() AECProcessor {
	return p.processor
}

// PrintStats prints current pipeline statistics to stdout
func (p *AECPipeline) PrintStats() {
	stats := p.GetStats()
	
	fmt.Printf("📊 AEC Pipeline Statistics:\n")
	fmt.Printf("  Runtime: %v\n", time.Since(stats.StartTime).Round(time.Second))
	fmt.Printf("  Frames Processed: %d\n", stats.FramesProcessed)
	fmt.Printf("  Frames Dropped: %d\n", stats.FramesDropped)
	fmt.Printf("  Processing Latency: %v\n", stats.PipelineLatency.Round(time.Microsecond))
	fmt.Printf("  CPU Usage: %.2f%%\n", stats.CPUUsage)
	
	if stats.AECStats.FramesProcessed > 0 {
		fmt.Printf("  Echo Return Loss: %.1f dB\n", stats.AECStats.EchoReturnLoss)
		fmt.Printf("  Echo Suppression: %.1f dB\n", stats.AECStats.EchoReturnLossEnhancement)
		fmt.Printf("  AEC Delay: %d ms\n", stats.AECStats.DelayMs)
	}
}

// CalibrateDelay automatically measures and sets the optimal delay for AEC
func (p *AECPipeline) CalibrateDelay(ctx context.Context, calibrationDuration time.Duration) error {
	fmt.Printf("🔧 Starting automatic delay calibration (%v)...\n", calibrationDuration)
	
	// Generate calibration tone parameters
	toneFreq := 1000.0 // 1 kHz test tone
	amplitude := 0.2   // Moderate amplitude
	
	// Record start time
	startTime := time.Now()
	calibrationCtx, cancel := context.WithTimeout(ctx, calibrationDuration)
	defer cancel()
	
	// Get audio channels
	inputChan := p.GetInputChan()
	outputChan := p.GetOutputChan()
	
	// Storage for analysis
	inputSamples := make([]int16, 0)
	
	// Generate and send calibration tone
	go p.generateCalibrationTone(calibrationCtx, outputChan, toneFreq, amplitude)
	
	fmt.Println("🎵 Playing calibration tone and recording...")
	
	// Collect audio samples for analysis
	frameCount := 0
	for {
		select {
		case <-calibrationCtx.Done():
			goto analysis
		case frame := <-inputChan:
			if frame != nil {
				frameCount++
				samples := p.extractSamples(frame)
				inputSamples = append(inputSamples, samples...)
			}
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
	
analysis:
	elapsed := time.Since(startTime)
	fmt.Printf("📊 Collected %d frames in %v\n", frameCount, elapsed.Round(time.Millisecond))
	
	if len(inputSamples) < p.config.SampleRate/10 { // Need at least 100ms of audio
		return fmt.Errorf("insufficient audio data for calibration (%d samples)", len(inputSamples))
	}
	
	// Calculate delay using simple energy-based detection
	delay := p.estimateDelayFromEnergy(inputSamples, toneFreq)
	
	if delay > 0 {
		fmt.Printf("✅ Calibration complete - detected delay: %v\n", delay)
		return p.SetDelay(delay)
	}
	
	fmt.Println("⚠️  Could not detect delay - using default")
	return nil
}

// generateCalibrationTone generates a test tone for delay measurement
func (p *AECPipeline) generateCalibrationTone(ctx context.Context, outputChan chan<- *media.AudioFrame, freq, amplitude float64) {
	frameSize := p.frameSize
	frameDuration := time.Duration(frameSize * int(time.Second) / p.config.SampleRate)
	
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()
	
	phase := 0.0
	phaseIncrement := 2.0 * 3.14159 * freq / float64(p.config.SampleRate)
	
	format := media.AudioFormat{
		SampleRate:    p.config.SampleRate,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pcmData := make([]byte, frameSize*2)
			
			for i := 0; i < frameSize; i++ {
				sample := int16(amplitude * 32767.0 * math.Sin(phase))
				pcmData[i*2] = byte(sample & 0xFF)
				pcmData[i*2+1] = byte((sample >> 8) & 0xFF)
				
				phase += phaseIncrement
				if phase > 2.0*3.14159 {
					phase -= 2.0 * 3.14159
				}
			}
			
			frame := media.NewAudioFrame(pcmData, format)
			
			select {
			case outputChan <- frame:
			default:
				// Channel full, drop frame
			}
		}
	}
}

// extractSamples extracts int16 samples from an audio frame
func (p *AECPipeline) extractSamples(frame *media.AudioFrame) []int16 {
	sampleCount := len(frame.Data) / 2
	samples := make([]int16, sampleCount)
	
	for i := 0; i < sampleCount; i++ {
		sample := int16(frame.Data[i*2]) | (int16(frame.Data[i*2+1]) << 8)
		samples[i] = sample
	}
	
	return samples
}

// estimateDelayFromEnergy estimates delay using energy-based detection
func (p *AECPipeline) estimateDelayFromEnergy(inputSamples []int16, toneFreq float64) time.Duration {
	// Simple energy-based delay estimation
	// Find the point where energy significantly increases
	windowSize := p.config.SampleRate / 100 // 10ms windows
	
	if len(inputSamples) < windowSize*3 {
		return 0
	}
	
	// Calculate energy in sliding windows
	maxEnergy := 0.0
	maxEnergyIndex := 0
	
	for i := 0; i < len(inputSamples)-windowSize; i += windowSize/4 {
		energy := 0.0
		for j := 0; j < windowSize && i+j < len(inputSamples); j++ {
			sample := float64(inputSamples[i+j])
			energy += sample * sample
		}
		
		if energy > maxEnergy {
			maxEnergy = energy
			maxEnergyIndex = i
		}
	}
	
	// Convert sample index to time delay
	if maxEnergyIndex > 0 {
		delaySamples := maxEnergyIndex
		delayMs := (delaySamples * 1000) / p.config.SampleRate
		return time.Duration(delayMs) * time.Millisecond
	}
	
	return 0
}