package mock

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/services/vad"
)

// MockVAD implements the VAD interface for testing
type MockVAD struct {
	*vad.BaseVAD
	speechProbability float64
	threshold         float64
	delay             time.Duration
	speechPattern     []bool
	patternIndex      int
	noiseLevel        float64
}

// NewMockVAD creates a new mock VAD service
func NewMockVAD() *MockVAD {
	// Create a realistic speech pattern: speech, silence, speech, silence...
	speechPattern := []bool{
		true, true, true, false, false, // 3 speech, 2 silence
		true, true, false, false, false, // 2 speech, 3 silence
		true, true, true, true, false,   // 4 speech, 1 silence
		false, false, true, true, false, // 2 silence, 2 speech, 1 silence
	}

	return &MockVAD{
		BaseVAD:           vad.NewBaseVAD("mock-vad", "1.0.0"),
		speechProbability: 0.8,
		threshold:         0.5,
		delay:             10 * time.Millisecond,
		speechPattern:     speechPattern,
		patternIndex:      0,
		noiseLevel:        0.1,
	}
}

// SetSpeechProbability sets the probability of detecting speech
func (m *MockVAD) SetSpeechProbability(prob float64) {
	m.speechProbability = prob
}

// SetThreshold sets the speech detection threshold
func (m *MockVAD) SetThreshold(threshold float64) {
	m.threshold = threshold
}

// SetDelay sets the mock processing delay
func (m *MockVAD) SetDelay(delay time.Duration) {
	m.delay = delay
}

// SetSpeechPattern sets a custom speech pattern for testing
func (m *MockVAD) SetSpeechPattern(pattern []bool) {
	m.speechPattern = pattern
	m.patternIndex = 0
}

// SetNoiseLevel sets the background noise level (0.0 to 1.0)
func (m *MockVAD) SetNoiseLevel(level float64) {
	m.noiseLevel = level
}

// Detect implements vad.VAD
func (m *MockVAD) Detect(ctx context.Context, audio *media.AudioFrame) (*vad.Detection, error) {
	// Simulate processing delay
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Calculate mock probability based on audio energy and pattern
	energy := m.calculateMockEnergy(audio)
	probability := m.calculateSpeechProbability(energy)
	isSpeech := probability > m.threshold

	return &vad.Detection{
		Probability: probability,
		IsSpeech:    isSpeech,
		Timestamp:   time.Now(),
		Confidence:  probability,
		Energy:      energy,
		Metadata: map[string]interface{}{
			"mock":         true,
			"pattern_index": m.patternIndex,
			"audio_size":   len(audio.Data),
			"duration_ms":  audio.Duration.Milliseconds(),
		},
	}, nil
}

// DetectStream implements vad.VAD
func (m *MockVAD) DetectStream(ctx context.Context, opts *vad.StreamOptions) (vad.DetectionStream, error) {
	return NewMockDetectionStream(m), nil
}

// calculateMockEnergy calculates mock energy from audio data
func (m *MockVAD) calculateMockEnergy(audio *media.AudioFrame) float64 {
	if len(audio.Data) == 0 {
		return 0.0
	}

	// Simple RMS calculation for 16-bit PCM
	var sum float64
	sampleCount := len(audio.Data) / 2 // 16-bit samples

	for i := 0; i < sampleCount; i++ {
		// Convert little-endian 16-bit to int16
		sample := int16(audio.Data[i*2]) | int16(audio.Data[i*2+1])<<8
		sum += float64(sample * sample)
	}

	if sampleCount == 0 {
		return 0.0
	}

	rms := math.Sqrt(sum / float64(sampleCount))
	return rms / 32768.0 // Normalize to 0-1 range
}

// calculateSpeechProbability calculates speech probability based on pattern and energy
func (m *MockVAD) calculateSpeechProbability(energy float64) float64 {
	// Get current pattern value
	isSpeechPattern := m.speechPattern[m.patternIndex%len(m.speechPattern)]
	m.patternIndex++

	// Base probability from pattern
	var baseProbability float64
	if isSpeechPattern {
		baseProbability = m.speechProbability
	} else {
		baseProbability = m.noiseLevel
	}

	// Modulate based on energy
	energyFactor := math.Min(energy*2, 1.0) // Scale energy influence
	probability := baseProbability * (0.5 + 0.5*energyFactor)

	// Add some randomness
	noise := (rand.Float64() - 0.5) * 0.1
	probability += noise

	// Clamp to valid range
	if probability < 0 {
		probability = 0
	} else if probability > 1 {
		probability = 1
	}

	return probability
}

// MockDetectionStream implements vad.DetectionStream
type MockDetectionStream struct {
	vad         *MockVAD
	audioCh     chan *media.AudioFrame
	detectionCh chan *vad.Detection
	closed      bool
	sendClosed  bool
	mu          sync.Mutex
}

// NewMockDetectionStream creates a new mock detection stream
func NewMockDetectionStream(mockVAD *MockVAD) *MockDetectionStream {
	stream := &MockDetectionStream{
		vad:         mockVAD,
		audioCh:     make(chan *media.AudioFrame, 10),
		detectionCh: make(chan *vad.Detection, 10),
		closed:      false,
	}

	// Start processing goroutine
	go stream.processAudio()

	return stream
}

// SendAudio implements vad.DetectionStream
func (s *MockDetectionStream) SendAudio(audio *media.AudioFrame) error {
	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	select {
	case s.audioCh <- audio:
		return nil
	default:
		return fmt.Errorf("audio buffer full")
	}
}

// Recv implements vad.DetectionStream
func (s *MockDetectionStream) Recv() (*vad.Detection, error) {
	if s.closed {
		return nil, fmt.Errorf("stream is closed")
	}

	detection, ok := <-s.detectionCh
	if !ok {
		return nil, fmt.Errorf("stream is closed")
	}

	return detection, nil
}

// Close implements vad.DetectionStream
func (s *MockDetectionStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}

	s.closed = true
	
	// Only close audioCh if it hasn't been closed by CloseSend
	if !s.sendClosed {
		close(s.audioCh)
	}
	
	return nil
}

// CloseSend implements vad.DetectionStream
func (s *MockDetectionStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	if !s.sendClosed {
		s.sendClosed = true
		close(s.audioCh)
	}
	
	return nil
}

// processAudio processes incoming audio and generates detection results
func (s *MockDetectionStream) processAudio() {
	defer close(s.detectionCh)

	for audio := range s.audioCh {
		// Simulate processing delay
		time.Sleep(s.vad.delay)

		// Calculate detection
		energy := s.vad.calculateMockEnergy(audio)
		probability := s.vad.calculateSpeechProbability(energy)
		isSpeech := probability > s.vad.threshold

		detection := &vad.Detection{
			Probability: probability,
			IsSpeech:    isSpeech,
			Timestamp:   time.Now(),
			Confidence:  probability,
			Energy:      energy,
			Metadata: map[string]interface{}{
				"mock":         true,
				"streaming":    true,
				"pattern_index": s.vad.patternIndex - 1,
				"audio_size":   len(audio.Data),
			},
		}

		// Send detection result
		select {
		case s.detectionCh <- detection:
		default:
			// Detection buffer full, skip
			return
		}
	}
}

// MockSileroVAD implements the SileroVAD interface for testing
type MockSileroVAD struct {
	*MockVAD
	capabilities vad.VADCapabilities
	options      vad.VADOptions
}

// NewMockSileroVAD creates a mock Silero VAD implementation
func NewMockSileroVAD() *MockSileroVAD {
	return &MockSileroVAD{
		MockVAD: NewMockVAD(),
		capabilities: vad.VADCapabilities{
			UpdateInterval: 0.032, // 32ms intervals
		},
		options: vad.DefaultVADOptions(),
	}
}

// Capabilities implements vad.SileroVAD
func (m *MockSileroVAD) Capabilities() vad.VADCapabilities {
	return m.capabilities
}

// CreateStream implements vad.SileroVAD
func (m *MockSileroVAD) CreateStream(ctx context.Context) (vad.VADStream, error) {
	return NewMockVADStream(m), nil
}

// UpdateOptions implements vad.SileroVAD
func (m *MockSileroVAD) UpdateOptions(opts vad.VADOptions) error {
	m.options = opts
	m.threshold = opts.ActivationThreshold
	return nil
}

// Close implements vad.SileroVAD
func (m *MockSileroVAD) Close() error {
	return nil
}

// MockVADStream implements vad.VADStream
type MockVADStream struct {
	sileroVAD *MockSileroVAD
	closed    bool
}

// NewMockVADStream creates a new mock VAD stream
func NewMockVADStream(sileroVAD *MockSileroVAD) *MockVADStream {
	return &MockVADStream{
		sileroVAD: sileroVAD,
		closed:    false,
	}
}

// ProcessFrame implements vad.VADStream
func (s *MockVADStream) ProcessFrame(ctx context.Context, frame *media.AudioFrame) ([]vad.VADEvent, error) {
	if s.closed {
		return nil, fmt.Errorf("stream is closed")
	}

	// Simulate processing delay
	time.Sleep(s.sileroVAD.delay)

	// Calculate detection similar to the real energy VAD
	energy := s.sileroVAD.calculateMockEnergy(frame)
	probability := s.sileroVAD.calculateSpeechProbability(energy)
	isSpeech := probability > s.sileroVAD.threshold

	// Create VAD event
	event := vad.VADEvent{
		Type:              vad.VADEventTypeInferenceDone,
		SamplesIndex:      s.sileroVAD.patternIndex * 1000, // Mock sample index
		Timestamp:         float64(time.Now().UnixNano()) / 1e9,
		Probability:       probability,
		InferenceDuration: s.sileroVAD.delay,
		Frames:            []*media.AudioFrame{frame},
		Speaking:          isSpeech,
	}

	// Generate speech start/end events based on state changes
	var events []vad.VADEvent

	// For simplicity, generate start/end events randomly
	if isSpeech && rand.Float64() < 0.1 { // 10% chance of start event
		startEvent := event
		startEvent.Type = vad.VADEventTypeStartOfSpeech
		events = append(events, startEvent)
	} else if !isSpeech && rand.Float64() < 0.05 { // 5% chance of end event
		endEvent := event
		endEvent.Type = vad.VADEventTypeEndOfSpeech
		events = append(events, endEvent)
	}

	// Always include inference done event
	events = append(events, event)

	return events, nil
}

// Close implements vad.VADStream
func (s *MockVADStream) Close() error {
	s.closed = true
	return nil
}