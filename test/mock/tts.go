package mock

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/services/tts"
)

// MockTTS implements the TTS interface for testing
type MockTTS struct {
	*tts.BaseTTS
	delay       time.Duration
	sampleRate  int
	audioFormat media.AudioFormat
}

// NewMockTTS creates a new mock TTS service
func NewMockTTS() *MockTTS {
	voices := []tts.Voice{
		{ID: "mock-voice-1", Name: "MockVoice Male", Gender: "male", Language: "en-US"},
		{ID: "mock-voice-2", Name: "MockVoice Female", Gender: "female", Language: "en-US"},
		{ID: "mock-voice-3", Name: "MockVoice Child", Gender: "neutral", Language: "en-US"},
	}

	audioFormat := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	return &MockTTS{
		BaseTTS:     tts.NewBaseTTS("mock-tts", "1.0.0", voices),
		delay:       200 * time.Millisecond,
		sampleRate:  24000,
		audioFormat: audioFormat,
	}
}

// SetDelay sets the mock delay for synthesis
func (m *MockTTS) SetDelay(delay time.Duration) {
	m.delay = delay
}

// SetAudioFormat sets the audio format for generated audio
func (m *MockTTS) SetAudioFormat(format media.AudioFormat) {
	m.audioFormat = format
	m.sampleRate = format.SampleRate
}

// Synthesize implements tts.TTS
func (m *MockTTS) Synthesize(ctx context.Context, text string, opts *tts.SynthesizeOptions) (*media.AudioFrame, error) {
	// Simulate processing delay
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Use options if provided, otherwise use defaults
	format := m.audioFormat
	if opts != nil && opts.SampleRate > 0 {
		format.SampleRate = opts.SampleRate
	}

	// Generate synthetic audio data based on text length
	// Estimate ~150 words per minute, ~5 characters per word = ~12.5 chars per second
	estimatedDuration := time.Duration(float64(len(text)) * float64(time.Second) / 12.5)
	if estimatedDuration < 500*time.Millisecond {
		estimatedDuration = 500 * time.Millisecond // Minimum duration
	}

	audioData := m.generateSyntheticAudio(estimatedDuration, format)

	audioFrame := media.NewAudioFrame(audioData, format)
	audioFrame.Metadata["text"] = text
	audioFrame.Metadata["mock"] = true
	audioFrame.Metadata["voice"] = "mock-voice-1"
	if opts != nil && opts.Voice != "" {
		audioFrame.Metadata["voice"] = opts.Voice
	}

	return audioFrame, nil
}

// SynthesizeStream implements tts.TTS
func (m *MockTTS) SynthesizeStream(ctx context.Context, opts *tts.SynthesizeOptions) (tts.SynthesisStream, error) {
	return NewMockSynthesisStream(m, opts), nil
}

// generateSyntheticAudio generates synthetic PCM audio data
func (m *MockTTS) generateSyntheticAudio(duration time.Duration, format media.AudioFormat) []byte {
	sampleCount := int(float64(format.SampleRate) * duration.Seconds())
	bytesPerSample := format.BitsPerSample / 8
	audioData := make([]byte, sampleCount*bytesPerSample)

	// Generate a simple sine wave with some variation to simulate speech
	baseFreq := 220.0 // A3 note
	sampleRate := float64(format.SampleRate)

	for i := 0; i < sampleCount; i++ {
		t := float64(i) / sampleRate

		// Create a complex waveform that sounds more like speech
		// Multiple sine waves with varying frequencies and amplitudes
		signal := 0.0
		signal += 0.3 * math.Sin(2*math.Pi*baseFreq*t)                    // Fundamental frequency
		signal += 0.2 * math.Sin(2*math.Pi*baseFreq*2*t)                 // First harmonic
		signal += 0.1 * math.Sin(2*math.Pi*baseFreq*3*t)                 // Second harmonic
		signal += 0.05 * math.Sin(2*math.Pi*baseFreq*0.5*t)              // Sub-harmonic
		signal += 0.02 * math.Sin(2*math.Pi*baseFreq*7*t)                // Higher harmonic
		signal *= 0.5 * (1.0 + 0.3*math.Sin(2*math.Pi*3*t))             // Amplitude modulation
		signal *= math.Exp(-t * 0.5)                                     // Decay envelope
		signal *= (1.0 - math.Exp(-t*10))                                // Attack envelope

		// Add some noise for realism
		noise := (float64(i%7) - 3.0) / 100.0
		signal += noise

		// Convert to 16-bit PCM
		sample := int16(signal * 16000) // Scale to reasonable amplitude

		// Store as little-endian 16-bit
		audioData[i*2] = byte(sample & 0xFF)
		audioData[i*2+1] = byte((sample >> 8) & 0xFF)
	}

	return audioData
}

// MockSynthesisStream implements tts.SynthesisStream
type MockSynthesisStream struct {
	tts        *MockTTS
	opts       *tts.SynthesizeOptions
	textCh     chan string
	audioCh    chan *media.AudioFrame
	closed     bool
	sendClosed bool
	mu         sync.Mutex
}

// NewMockSynthesisStream creates a new mock synthesis stream
func NewMockSynthesisStream(mockTTS *MockTTS, opts *tts.SynthesizeOptions) *MockSynthesisStream {
	stream := &MockSynthesisStream{
		tts:     mockTTS,
		opts:    opts,
		textCh:  make(chan string, 10),
		audioCh: make(chan *media.AudioFrame, 10),
		closed:  false,
	}

	// Start processing goroutine
	go stream.processText()

	return stream
}

// SendText implements tts.SynthesisStream
func (s *MockSynthesisStream) SendText(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed || s.sendClosed {
		return fmt.Errorf("stream is closed")
	}

	select {
	case s.textCh <- text:
		return nil
	default:
		return fmt.Errorf("text buffer full")
	}
}

// Recv implements tts.SynthesisStream
func (s *MockSynthesisStream) Recv() (*media.AudioFrame, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	
	if closed {
		return nil, fmt.Errorf("stream is closed")
	}

	audio, ok := <-s.audioCh
	if !ok {
		return nil, fmt.Errorf("stream is closed")
	}

	return audio, nil
}

// Close implements tts.SynthesisStream
func (s *MockSynthesisStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}

	s.closed = true
	
	// Only close textCh if it hasn't been closed by CloseSend
	if !s.sendClosed {
		close(s.textCh)
	}
	
	return nil
}

// CloseSend implements tts.SynthesisStream
func (s *MockSynthesisStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	if !s.sendClosed {
		s.sendClosed = true
		close(s.textCh)
	}
	
	return nil
}

// processText processes incoming text and generates synthetic audio
func (s *MockSynthesisStream) processText() {
	defer close(s.audioCh)

	for text := range s.textCh {
		// Simulate processing delay
		time.Sleep(s.tts.delay)

		// Generate audio for the text
		format := s.tts.audioFormat
		if s.opts != nil && s.opts.SampleRate > 0 {
			format.SampleRate = s.opts.SampleRate
		}

		// Estimate duration based on text length
		estimatedDuration := time.Duration(float64(len(text)) * float64(time.Second) / 12.5)
		if estimatedDuration < 200*time.Millisecond {
			estimatedDuration = 200 * time.Millisecond
		}

		audioData := s.tts.generateSyntheticAudio(estimatedDuration, format)
		audioFrame := media.NewAudioFrame(audioData, format)
		audioFrame.Metadata["text"] = text
		audioFrame.Metadata["mock"] = true
		audioFrame.Metadata["streaming"] = true

		// Send audio frame
		select {
		case s.audioCh <- audioFrame:
		default:
			// Audio buffer full, skip frame
			return
		}
	}
}