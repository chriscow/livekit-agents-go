package audio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"livekit-agents-go/media"
	"github.com/gordonklaus/portaudio"
)

var (
	portaudioInitOnce sync.Once
	portaudioInitErr  error
)

const (
	DefaultSampleRate = 48000
	DefaultChannels   = 1
	DefaultBitDepth   = 16
	FramesPerBuffer   = 1024
	
	// AEC ring buffer size for output reference
	OutputRingBufferSize = 48000 * 2 // 2 seconds at 48kHz (in samples)
)

// AudioProcessingCallback is called for each audio frame with input and output reference
type AudioProcessingCallback func(inputFrame, outputReferenceFrame *media.AudioFrame) (*media.AudioFrame, error)

type LocalAudioIO struct {
	sampleRate      int
	channels        int
	bitDepth        int
	framesPerBuffer int

	inputStream  *portaudio.Stream
	outputStream *portaudio.Stream

	inputBuffer  []float32
	outputBuffer []float32

	inputChan  chan *media.AudioFrame
	outputChan chan *media.AudioFrame

	// AEC support - ring buffer for output reference stream
	outputRingBuffer []int16
	ringWriteIndex   int
	ringReadIndex    int
	ringBufferMu     sync.RWMutex
	
	// Audio processing callback for AEC
	processingCallback AudioProcessingCallback
	callbackMu         sync.RWMutex
	
	// Delay tracking for AEC
	estimatedDelay time.Duration
	delayMu        sync.RWMutex

	running bool
	mu      sync.RWMutex
	wg      sync.WaitGroup
}

type Config struct {
	SampleRate      int
	Channels        int
	BitDepth        int
	FramesPerBuffer int
	
	// AEC configuration
	EnableAECProcessing bool
	EstimatedDelay      time.Duration
}

func DefaultConfig() Config {
	return Config{
		SampleRate:          DefaultSampleRate,
		Channels:            DefaultChannels,
		BitDepth:            DefaultBitDepth,
		FramesPerBuffer:     FramesPerBuffer,
		EnableAECProcessing: false,
		EstimatedDelay:      50 * time.Millisecond,
	}
}

// ensurePortAudioInit ensures PortAudio is initialized exactly once
func ensurePortAudioInit() error {
	portaudioInitOnce.Do(func() {
		portaudioInitErr = portaudio.Initialize()
	})
	return portaudioInitErr
}

func NewLocalAudioIO(config Config) (*LocalAudioIO, error) {
	// Ensure PortAudio is initialized
	if err := ensurePortAudioInit(); err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %w", err)
	}
	if config.SampleRate == 0 {
		config.SampleRate = DefaultSampleRate
	}
	if config.Channels == 0 {
		config.Channels = DefaultChannels
	}
	if config.BitDepth == 0 {
		config.BitDepth = DefaultBitDepth
	}
	if config.FramesPerBuffer == 0 {
		config.FramesPerBuffer = FramesPerBuffer
	}

	err := portaudio.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %w", err)
	}

	io := &LocalAudioIO{
		sampleRate:       config.SampleRate,
		channels:         config.Channels,
		bitDepth:         config.BitDepth,
		framesPerBuffer:  config.FramesPerBuffer,
		inputBuffer:      make([]float32, config.FramesPerBuffer*config.Channels),
		outputBuffer:     make([]float32, config.FramesPerBuffer*config.Channels),
		inputChan:        make(chan *media.AudioFrame, 10),
		outputChan:       make(chan *media.AudioFrame, 10),
		outputRingBuffer: make([]int16, OutputRingBufferSize),
		estimatedDelay:   config.EstimatedDelay,
	}

	return io, nil
}

func (io *LocalAudioIO) Start(ctx context.Context) error {
	io.mu.Lock()
	defer io.mu.Unlock()

	if io.running {
		return fmt.Errorf("audio I/O already running")
	}

	// Reinitialize channels if they were closed
	if io.inputChan == nil {
		io.inputChan = make(chan *media.AudioFrame, 10)
	}
	if io.outputChan == nil {
		io.outputChan = make(chan *media.AudioFrame, 10)
	}

	// Get default devices
	inputDevice, err := portaudio.DefaultInputDevice()
	if err != nil {
		return fmt.Errorf("failed to get default input device: %w", err)
	}

	// Initialize input stream for microphone capture
	inputParams := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: io.channels,
			Latency:  inputDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(io.sampleRate),
		FramesPerBuffer: io.framesPerBuffer,
	}

	io.inputStream, err = portaudio.OpenStream(inputParams, io.inputBuffer)
	if err != nil {
		return fmt.Errorf("failed to open input stream: %w", err)
	}

	// Start input stream
	if err := io.inputStream.Start(); err != nil {
		io.inputStream.Close()
		return fmt.Errorf("failed to start input stream: %w", err)
	}

	io.running = true

	// Start input capture goroutine
	io.wg.Add(1)
	go io.captureAudio(ctx)

	// Add a small delay to ensure input stream is fully initialized before starting output goroutine
	time.Sleep(100 * time.Millisecond)

	// Start output playback goroutine  
	io.wg.Add(1)
	go io.playAudio(ctx)

	return nil
}

func (io *LocalAudioIO) Stop() error {
	io.mu.Lock()
	defer io.mu.Unlock()

	if !io.running {
		return nil
	}

	io.running = false

	// Stop PortAudio input stream (output stream is managed by playAudio goroutine)
	if io.inputStream != nil {
		io.inputStream.Stop()
		io.inputStream.Close()
		io.inputStream = nil
	}

	// Wait for goroutines to finish with a timeout
	done := make(chan struct{})
	go func() {
		io.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines finished successfully
	case <-time.After(2 * time.Second):
		// Timeout waiting for goroutines - continue anyway
		fmt.Println("⚠️  Timeout waiting for audio goroutines to finish")
	}

	// Then close channels to prevent panic
	if io.inputChan != nil {
		close(io.inputChan)
		io.inputChan = nil
	}

	if io.outputChan != nil {
		close(io.outputChan)
		io.outputChan = nil
	}

	return nil
}

func (io *LocalAudioIO) Close() error {
	err := io.Stop()
	portaudio.Terminate()
	return err
}

func (io *LocalAudioIO) InputChan() <-chan *media.AudioFrame {
	return io.inputChan
}

func (io *LocalAudioIO) OutputChan() chan<- *media.AudioFrame {
	return io.outputChan
}

// SetAudioProcessingCallback sets the callback for audio processing (AEC)
func (io *LocalAudioIO) SetAudioProcessingCallback(callback AudioProcessingCallback) {
	io.callbackMu.Lock()
	defer io.callbackMu.Unlock()
	io.processingCallback = callback
}

// GetEstimatedDelay returns the current estimated audio delay
func (io *LocalAudioIO) GetEstimatedDelay() time.Duration {
	io.delayMu.RLock()
	defer io.delayMu.RUnlock()
	return io.estimatedDelay
}

// SetEstimatedDelay sets the estimated audio delay for AEC
func (io *LocalAudioIO) SetEstimatedDelay(delay time.Duration) {
	io.delayMu.Lock()
	defer io.delayMu.Unlock()
	io.estimatedDelay = delay
}

// addToRingBuffer adds output audio samples to the ring buffer for AEC reference
func (io *LocalAudioIO) addToRingBuffer(samples []int16) {
	io.ringBufferMu.Lock()
	defer io.ringBufferMu.Unlock()
	
	for _, sample := range samples {
		io.outputRingBuffer[io.ringWriteIndex] = sample
		io.ringWriteIndex = (io.ringWriteIndex + 1) % len(io.outputRingBuffer)
		
		// Move read index if we've wrapped around
		if io.ringWriteIndex == io.ringReadIndex {
			io.ringReadIndex = (io.ringReadIndex + 1) % len(io.outputRingBuffer)
		}
	}
}

// getOutputReference extracts samples from ring buffer for AEC reference
func (io *LocalAudioIO) getOutputReference(delaySamples int, sampleCount int) []int16 {
	io.ringBufferMu.RLock()
	defer io.ringBufferMu.RUnlock()
	
	if sampleCount <= 0 {
		return nil
	}
	
	samples := make([]int16, sampleCount)
	
	// Calculate starting position based on delay
	startIndex := io.ringWriteIndex - delaySamples - sampleCount
	if startIndex < 0 {
		startIndex += len(io.outputRingBuffer)
	}
	
	// Extract samples with wraparound handling
	for i := 0; i < sampleCount; i++ {
		index := (startIndex + i) % len(io.outputRingBuffer)
		samples[i] = io.outputRingBuffer[index]
	}
	
	return samples
}

func (io *LocalAudioIO) captureAudio(ctx context.Context) {
	defer io.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		io.mu.RLock()
		if !io.running {
			io.mu.RUnlock()
			return
		}

		stream := io.inputStream
		buffer := io.inputBuffer
		io.mu.RUnlock()

		if stream == nil {
			return
		}

		// Use a timeout for stream.Read() to avoid blocking indefinitely
		done := make(chan error, 1)
		go func() {
			done <- stream.Read()
		}()

		select {
		case err := <-done:
			if err != nil {
				// Handle error - for now just continue
				time.Sleep(10 * time.Millisecond)
				continue
			}
		case <-time.After(100 * time.Millisecond):
			// Timeout - check if we should stop
			continue
		case <-ctx.Done():
			return
		}

		// Convert float32 to int16 PCM data
		pcmData := make([]byte, len(buffer)*2)
		for i, sample := range buffer {
			// Clamp sample to [-1, 1] and convert to int16
			if sample > 1.0 {
				sample = 1.0
			} else if sample < -1.0 {
				sample = -1.0
			}

			intSample := int16(sample * 32767)
			pcmData[i*2] = byte(intSample)
			pcmData[i*2+1] = byte(intSample >> 8)
		}

		// Create audio format
		format := media.AudioFormat{
			SampleRate:    io.sampleRate,
			Channels:      io.channels,
			BitsPerSample: io.bitDepth,
			Format:        media.AudioFormatPCM,
		}

		// Create audio frame
		frame := media.NewAudioFrame(pcmData, format)
		
		// Apply audio processing (AEC) if callback is set
		processedFrame := frame
		io.callbackMu.RLock()
		callback := io.processingCallback
		io.callbackMu.RUnlock()
		
		if callback != nil {
			// Get output reference frame for AEC
			io.delayMu.RLock()
			delay := io.estimatedDelay
			io.delayMu.RUnlock()
			
			delaySamples := int(delay.Seconds() * float64(io.sampleRate))
			sampleCount := len(pcmData) / 2 // 16-bit samples
			
			outputRefSamples := io.getOutputReference(delaySamples, sampleCount)
			var outputRefFrame *media.AudioFrame
			
			if len(outputRefSamples) > 0 {
				// Convert int16 samples back to PCM bytes
				outputRefData := make([]byte, len(outputRefSamples)*2)
				for i, sample := range outputRefSamples {
					outputRefData[i*2] = byte(sample & 0xFF)
					outputRefData[i*2+1] = byte((sample >> 8) & 0xFF)
				}
				outputRefFrame = media.NewAudioFrame(outputRefData, format)
			}
			
			// Apply processing
			var err error
			processedFrame, err = callback(frame, outputRefFrame)
			if err != nil {
				// Log error but continue with original frame
				fmt.Printf("⚠️  Audio processing error: %v\n", err)
				processedFrame = frame
			}
		}

		// Send processed frame to input channel (non-blocking)
		select {
		case io.inputChan <- processedFrame:
		default:
			// Channel is full, drop frame
		}
	}
}

func (io *LocalAudioIO) playAudio(ctx context.Context) {
	defer io.wg.Done()

	// Buffer to hold audio samples for playback
	var currentSamples []int16
	var sampleIndex int
	var outputStreamCreated bool
	var outputStream *portaudio.Stream

	// Defer cleanup
	defer func() {
		if outputStream != nil {
			outputStream.Stop()
			outputStream.Close()
		}
	}()

	fmt.Println("🔊 Output playback goroutine started")

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-io.outputChan:
			if !ok {
				return
			}

			if frame == nil {
				continue
			}

			// Create output stream lazily on first audio frame to avoid concurrent initialization
			if !outputStreamCreated {
				var err error
				outputStream, err = portaudio.OpenDefaultStream(
					0,                      // input channels
					1,                      // output channels
					float64(io.sampleRate), // sample rate
					io.framesPerBuffer,     // frames per buffer
					func(out []int16) { // callback function
						for i := range out {
							if sampleIndex < len(currentSamples) {
								out[i] = currentSamples[sampleIndex]
								sampleIndex++
							} else {
								out[i] = 0 // silence when no audio available
							}
						}
					},
				)
				if err != nil {
					fmt.Printf("⚠️  Failed to create output stream: %v\n", err)
					continue
				}

				if err := outputStream.Start(); err != nil {
					fmt.Printf("⚠️  Failed to start output stream: %v\n", err)
					outputStream.Close()
					continue
				}
				
				outputStreamCreated = true
				fmt.Println("✅ Output stream created and started")
			}

			// Convert PCM data to int16 samples with sample rate conversion if needed
			pcmData := frame.Data
			frameFormat := frame.Format

			var newSamples []int16

			if frameFormat.SampleRate != io.sampleRate {
				// Simple sample rate conversion for better audio quality
				ratio := float64(frameFormat.SampleRate) / float64(io.sampleRate)
				inputSamples := len(pcmData) / 2
				outputSamples := int(float64(inputSamples) / ratio)

				newSamples = make([]int16, outputSamples)
				for i := 0; i < outputSamples; i++ {
					srcIndex := int(float64(i) * ratio)
					if srcIndex >= inputSamples-1 {
						break
					}

					// Read little-endian int16
					sample := int16(pcmData[srcIndex*2]) | (int16(pcmData[srcIndex*2+1]) << 8)
					newSamples[i] = sample
				}

				fmt.Printf("🔄 Sample rate conversion: %d Hz -> %d Hz (%d -> %d samples)\n",
					frameFormat.SampleRate, io.sampleRate, inputSamples, len(newSamples))
			} else {
				// No sample rate conversion needed - direct copy
				inputSamples := len(pcmData) / 2
				newSamples = make([]int16, inputSamples)

				for i := 0; i < inputSamples; i++ {
					// Read little-endian int16
					sample := int16(pcmData[i*2]) | (int16(pcmData[i*2+1]) << 8)
					newSamples[i] = sample
				}
			}

			// Add new samples to buffer and reset index for new audio
			currentSamples = newSamples
			sampleIndex = 0
			
			// Add samples to ring buffer for AEC reference
			io.addToRingBuffer(newSamples)

			fmt.Printf("🔊 Loaded %d samples for playback\n", len(currentSamples))
		}
	}
}

func (io *LocalAudioIO) GetDeviceInfo() error {
	devices, err := portaudio.Devices()
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	fmt.Printf("Available audio devices (%d):\n", len(devices))

	for i, device := range devices {
		fmt.Printf("  Device %d: %s\n", i, device.Name)
		fmt.Printf("    Max input channels: %d\n", device.MaxInputChannels)
		fmt.Printf("    Max output channels: %d\n", device.MaxOutputChannels)
		fmt.Printf("    Default sample rate: %.0f Hz\n", device.DefaultSampleRate)
	}

	defaultInput, err := portaudio.DefaultInputDevice()
	if err != nil {
		return fmt.Errorf("failed to get default input device: %w", err)
	}

	defaultOutput, err := portaudio.DefaultOutputDevice()
	if err != nil {
		return fmt.Errorf("failed to get default output device: %w", err)
	}

	fmt.Printf("Default input device: %s\n", defaultInput.Name)
	fmt.Printf("Default output device: %s\n", defaultOutput.Name)

	return nil
}
