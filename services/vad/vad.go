package vad

import (
	"context"
	"io"
	"livekit-agents-go/media"
	"math"
	"time"
)

// VADEventType represents the type of VAD event
type VADEventType int

const (
	VADEventTypeInferenceDone VADEventType = iota
	VADEventTypeStartOfSpeech
	VADEventTypeEndOfSpeech
)

// VADEvent represents a voice activity detection event
type VADEvent struct {
	Type              VADEventType
	SamplesIndex      int
	Timestamp         float64
	SilenceDuration   float64
	SpeechDuration    float64
	Probability       float64
	InferenceDuration time.Duration
	Frames            []*media.AudioFrame
	Speaking          bool

	// Raw accumulated durations (for debugging)
	RawAccumulatedSilence float64
	RawAccumulatedSpeech  float64
}

// VADCapabilities defines the capabilities of a VAD implementation
type VADCapabilities struct {
	UpdateInterval float64 // Update interval in seconds
}

// VADOptions contains configuration for VAD processing
type VADOptions struct {
	MinSpeechDuration     float64 // Minimum duration to consider as speech
	MinSilenceDuration    float64 // Minimum silence duration to end speech
	PrefixPaddingDuration float64 // Padding at start of speech chunks
	MaxBufferedSpeech     float64 // Maximum speech buffer duration
	ActivationThreshold   float64 // Probability threshold for speech detection
	SampleRate            int     // Target sample rate for VAD processing
}

// DefaultVADOptions returns sensible default VAD options
func DefaultVADOptions() VADOptions {
	return VADOptions{
		MinSpeechDuration:     0.05,  // 50ms
		MinSilenceDuration:    0.55,  // 550ms
		PrefixPaddingDuration: 0.5,   // 500ms
		MaxBufferedSpeech:     60.0,  // 60 seconds
		ActivationThreshold:   0.5,   // 50% probability
		SampleRate:            16000, // 16kHz
	}
}

// SileroVAD interface defines the contract for Silero-style voice activity detection
type SileroVAD interface {
	// Capabilities returns the VAD capabilities
	Capabilities() VADCapabilities

	// CreateStream creates a new VAD stream for processing
	CreateStream(ctx context.Context) (VADStream, error)

	// UpdateOptions updates the VAD configuration
	UpdateOptions(opts VADOptions) error

	// Close closes the VAD and releases resources
	Close() error
}

// VADStream interface defines a stream for processing audio frames
type VADStream interface {
	// ProcessFrame processes an audio frame and returns VAD events
	ProcessFrame(ctx context.Context, frame *media.AudioFrame) ([]VADEvent, error)

	// Close closes the stream and releases resources
	Close() error
}

// EnergyVAD is a simple energy-based voice activity detector that implements SileroVAD
type EnergyVAD struct {
	opts         VADOptions
	capabilities VADCapabilities
}

// NewEnergyVAD creates a new energy-based VAD
func NewEnergyVAD(opts VADOptions) *EnergyVAD {
	return &EnergyVAD{
		opts: opts,
		capabilities: VADCapabilities{
			UpdateInterval: 0.032, // 32ms intervals
		},
	}
}

// Capabilities returns the VAD capabilities
func (v *EnergyVAD) Capabilities() VADCapabilities {
	return v.capabilities
}

// CreateStream creates a new VAD stream
func (v *EnergyVAD) CreateStream(ctx context.Context) (VADStream, error) {
	return NewEnergyVADStream(v.opts), nil
}

// UpdateOptions updates the VAD options
func (v *EnergyVAD) UpdateOptions(opts VADOptions) error {
	v.opts = opts
	return nil
}

// Close closes the VAD
func (v *EnergyVAD) Close() error {
	return nil
}

// EnergyVADStream processes audio using energy-based voice activity detection
type EnergyVADStream struct {
	opts VADOptions

	// State tracking
	speaking             bool
	speechDuration       float64
	silenceDuration      float64
	speechThresholdTime  float64
	silenceThresholdTime float64
	currentSample        int
	timestamp            float64

	// Audio buffering
	speechBuffer           []int16
	maxSpeechBufferSamples int
	prefixPaddingSamples   int

	// Energy calculation
	energyThreshold   float64
	energyHistory     []float64
	energyHistorySize int
}

// NewEnergyVADStream creates a new energy-based VAD stream
func NewEnergyVADStream(opts VADOptions) *EnergyVADStream {
	maxSpeechBufferSamples := int(opts.MaxBufferedSpeech * float64(opts.SampleRate))
	prefixPaddingSamples := int(opts.PrefixPaddingDuration * float64(opts.SampleRate))

	return &EnergyVADStream{
		opts:                   opts,
		maxSpeechBufferSamples: maxSpeechBufferSamples,
		prefixPaddingSamples:   prefixPaddingSamples,
		speechBuffer:           make([]int16, 0, maxSpeechBufferSamples+prefixPaddingSamples),
		energyThreshold:        1000.0,                 // Will be calibrated dynamically
		energyHistory:          make([]float64, 0, 50), // Track 50 energy values
		energyHistorySize:      50,
	}
}

// ProcessFrame processes an audio frame and detects voice activity
func (s *EnergyVADStream) ProcessFrame(ctx context.Context, frame *media.AudioFrame) ([]VADEvent, error) {
	if frame == nil {
		return nil, nil
	}

	// Convert frame data to int16 samples
	samples := s.convertFrameToSamples(frame)
	if len(samples) == 0 {
		return nil, nil
	}

	// Calculate energy for this frame
	energy := s.calculateEnergy(samples)

	// Update energy history for dynamic threshold adjustment
	s.updateEnergyHistory(energy)

	// Determine if this frame contains speech
	isSpeech := energy > s.energyThreshold

	// Convert to probability (0-1) for compatibility with Silero interface
	probability := s.energyToProbability(energy)

	// Calculate frame duration
	frameDuration := float64(len(samples)) / float64(frame.Format.SampleRate)

	// Update timestamps
	s.currentSample += len(samples)
	s.timestamp += frameDuration

	var events []VADEvent

	// Add samples to speech buffer
	s.addToSpeechBuffer(samples)

	// Track speech/silence durations
	if isSpeech {
		s.speechThresholdTime += frameDuration
		s.silenceThresholdTime = 0.0

		// Check if we should start speech
		if !s.speaking && s.speechThresholdTime >= s.opts.MinSpeechDuration {
			s.speaking = true
			s.silenceDuration = 0.0
			s.speechDuration = s.speechThresholdTime

			// Emit START_OF_SPEECH event
			events = append(events, VADEvent{
				Type:            VADEventTypeStartOfSpeech,
				SamplesIndex:    s.currentSample,
				Timestamp:       s.timestamp,
				SilenceDuration: s.silenceDuration,
				SpeechDuration:  s.speechDuration,
				Frames:          []*media.AudioFrame{s.createSpeechFrame(frame.Format)},
				Speaking:        true,
				Probability:     probability,
			})
		}
	} else {
		s.silenceThresholdTime += frameDuration
		s.speechThresholdTime = 0.0

		// Reset speech buffer if not speaking
		if !s.speaking {
			s.resetSpeechBuffer()
		}

		// Check if we should end speech
		if s.speaking && s.silenceThresholdTime >= s.opts.MinSilenceDuration {
			s.speaking = false
			s.speechDuration = 0.0
			s.silenceDuration = s.silenceThresholdTime

			// Emit END_OF_SPEECH event
			events = append(events, VADEvent{
				Type:            VADEventTypeEndOfSpeech,
				SamplesIndex:    s.currentSample,
				Timestamp:       s.timestamp,
				SilenceDuration: s.silenceDuration,
				SpeechDuration:  s.speechDuration,
				Frames:          []*media.AudioFrame{s.createSpeechFrame(frame.Format)},
				Speaking:        false,
				Probability:     probability,
			})
		}
	}

	// Update public durations
	if s.speaking {
		s.speechDuration += frameDuration
	} else {
		s.silenceDuration += frameDuration
	}

	// Always emit INFERENCE_DONE event
	events = append(events, VADEvent{
		Type:                  VADEventTypeInferenceDone,
		SamplesIndex:          s.currentSample,
		Timestamp:             s.timestamp,
		SilenceDuration:       s.silenceDuration,
		SpeechDuration:        s.speechDuration,
		Probability:           probability,
		InferenceDuration:     time.Microsecond * 100, // Placeholder - energy calc is very fast
		Frames:                []*media.AudioFrame{frame},
		Speaking:              s.speaking,
		RawAccumulatedSilence: s.silenceThresholdTime,
		RawAccumulatedSpeech:  s.speechThresholdTime,
	})

	return events, nil
}

// Close closes the VAD stream
func (s *EnergyVADStream) Close() error {
	return nil
}

// Helper methods

func (s *EnergyVADStream) convertFrameToSamples(frame *media.AudioFrame) []int16 {
	// Convert byte data to int16 samples (assuming 16-bit PCM)
	data := frame.Data
	samples := make([]int16, len(data)/2)

	for i := 0; i < len(samples); i++ {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}

	return samples
}

func (s *EnergyVADStream) calculateEnergy(samples []int16) float64 {
	var sum float64
	for _, sample := range samples {
		sum += float64(sample * sample)
	}
	return sum / float64(len(samples))
}

func (s *EnergyVADStream) updateEnergyHistory(energy float64) {
	s.energyHistory = append(s.energyHistory, energy)
	if len(s.energyHistory) > s.energyHistorySize {
		s.energyHistory = s.energyHistory[1:]
	}

	// Dynamic threshold adjustment - set threshold to 2x average energy
	if len(s.energyHistory) >= 10 {
		var sum float64
		for _, e := range s.energyHistory {
			sum += e
		}
		avgEnergy := sum / float64(len(s.energyHistory))
		s.energyThreshold = avgEnergy * 2.0
	}
}

func (s *EnergyVADStream) energyToProbability(energy float64) float64 {
	// Convert energy to probability (0-1) using sigmoid-like function
	if s.energyThreshold == 0 {
		return 0.0
	}

	ratio := energy / s.energyThreshold
	// Sigmoid: 1 / (1 + exp(-k*(x-1))) where k=3 for smooth transition
	prob := 1.0 / (1.0 + math.Exp(-3.0*(ratio-1.0)))

	return prob
}

func (s *EnergyVADStream) addToSpeechBuffer(samples []int16) {
	// Add samples to speech buffer, maintaining max size
	available := cap(s.speechBuffer) - len(s.speechBuffer)
	toAdd := len(samples)

	if toAdd > available {
		// Shift buffer to make room
		shift := toAdd - available
		if shift >= len(s.speechBuffer) {
			s.speechBuffer = s.speechBuffer[:0]
		} else {
			copy(s.speechBuffer, s.speechBuffer[shift:])
			s.speechBuffer = s.speechBuffer[:len(s.speechBuffer)-shift]
		}
	}

	s.speechBuffer = append(s.speechBuffer, samples...)
}

func (s *EnergyVADStream) resetSpeechBuffer() {
	if len(s.speechBuffer) <= s.prefixPaddingSamples {
		return
	}

	// Keep only prefix padding
	if s.prefixPaddingSamples > 0 {
		padding := s.speechBuffer[len(s.speechBuffer)-s.prefixPaddingSamples:]
		copy(s.speechBuffer[:s.prefixPaddingSamples], padding)
		s.speechBuffer = s.speechBuffer[:s.prefixPaddingSamples]
	} else {
		s.speechBuffer = s.speechBuffer[:0]
	}
}

func (s *EnergyVADStream) createSpeechFrame(format media.AudioFormat) *media.AudioFrame {
	if len(s.speechBuffer) == 0 {
		return nil
	}

	// Convert int16 samples back to byte data
	data := make([]byte, len(s.speechBuffer)*2)
	for i, sample := range s.speechBuffer {
		data[i*2] = byte(sample & 0xff)
		data[i*2+1] = byte((sample >> 8) & 0xff)
	}

	return media.NewAudioFrame(data, format)
}

// VAD defines the Voice Activity Detection service interface
type VAD interface {
	// Detect voice activity in audio frame
	Detect(ctx context.Context, audio *media.AudioFrame) (*Detection, error)

	// DetectStream creates a streaming detection session
	DetectStream(ctx context.Context, opts *StreamOptions) (DetectionStream, error)

	// Service metadata
	Name() string
	Version() string
}

// Detection represents voice activity detection result
type Detection struct {
	Probability float64
	IsSpeech    bool
	Timestamp   time.Time
	Confidence  float64
	Energy      float64
	Metadata    map[string]interface{}
}

// DetectionStream represents a streaming detection session
type DetectionStream interface {
	// SendAudio sends audio data to the detection stream
	SendAudio(audio *media.AudioFrame) error

	// Recv receives detection results from the stream
	Recv() (*Detection, error)

	// Close closes the detection stream
	Close() error

	// CloseSend signals that no more audio will be sent
	CloseSend() error
}

// StreamOptions configures streaming detection
type StreamOptions struct {
	Threshold          float64
	MinSpeechDuration  time.Duration
	MinSilenceDuration time.Duration
	WindowSize         time.Duration
	HopSize            time.Duration
	Model              string
	Metadata           map[string]interface{}
}

// StreamingVAD extends VAD for services that support advanced streaming
type StreamingVAD interface {
	VAD

	// Stream detection with advanced options
	Stream(ctx context.Context, opts *AdvancedStreamOptions) (DetectionStream, error)
}

// AdvancedStreamOptions configures advanced streaming detection
type AdvancedStreamOptions struct {
	Threshold          float64
	MinSpeechDuration  time.Duration
	MinSilenceDuration time.Duration
	WindowSize         time.Duration
	HopSize            time.Duration
	Model              string
	SpeechPadding      time.Duration
	SilencePadding     time.Duration
	ReturnProbs        bool
	ReturnEnergy       bool
	Metadata           map[string]interface{}
}

// DetectionResult represents detection results with additional information
type DetectionResult struct {
	Detection   *Detection
	AudioChunk  *media.AudioFrame
	SegmentType SegmentType
	StartTime   time.Time
	EndTime     time.Time
	Error       error
	StreamID    string
}

type SegmentType int

const (
	SegmentSpeech SegmentType = iota
	SegmentSilence
	SegmentUnknown
)

// SpeechSegment represents a segment of speech
type SpeechSegment struct {
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Probability float64
	AudioData   *media.AudioFrame
	Confidence  float64
	Energy      float64
	Metadata    map[string]interface{}
}

// SilenceSegment represents a segment of silence
type SilenceSegment struct {
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Probability float64
	AudioData   *media.AudioFrame
	Energy      float64
	Metadata    map[string]interface{}
}

// SegmentingVAD extends VAD for services that support audio segmentation
type SegmentingVAD interface {
	VAD

	// Segment audio into speech and silence segments
	Segment(ctx context.Context, audio *media.AudioFrame, opts *SegmentOptions) ([]SpeechSegment, []SilenceSegment, error)

	// SegmentStream creates a streaming segmentation session
	SegmentStream(ctx context.Context, opts *SegmentOptions) (SegmentationStream, error)
}

// SegmentOptions configures audio segmentation
type SegmentOptions struct {
	Threshold          float64
	MinSpeechDuration  time.Duration
	MinSilenceDuration time.Duration
	SpeechPadding      time.Duration
	SilencePadding     time.Duration
	ReturnSilence      bool
	ReturnAudio        bool
	Metadata           map[string]interface{}
}

// SegmentationStream represents a streaming segmentation session
type SegmentationStream interface {
	// SendAudio sends audio data to the segmentation stream
	SendAudio(audio *media.AudioFrame) error

	// Recv receives segmentation results from the stream
	Recv() (*SegmentationResult, error)

	// Close closes the segmentation stream
	Close() error

	// CloseSend signals that no more audio will be sent
	CloseSend() error
}

// SegmentationResult represents segmentation results
type SegmentationResult struct {
	SpeechSegments  []SpeechSegment
	SilenceSegments []SilenceSegment
	Error           error
	StreamID        string
	IsFinal         bool
}

// BaseVAD provides common functionality for VAD implementations
type BaseVAD struct {
	name    string
	version string
}

// NewBaseVAD creates a new base VAD service
func NewBaseVAD(name, version string) *BaseVAD {
	return &BaseVAD{
		name:    name,
		version: version,
	}
}

func (b *BaseVAD) Name() string {
	return b.name
}

func (b *BaseVAD) Version() string {
	return b.version
}

// DetectionStreamReader provides a reader interface for streaming detection
type DetectionStreamReader struct {
	stream DetectionStream
}

// NewDetectionStreamReader creates a new stream reader
func NewDetectionStreamReader(stream DetectionStream) *DetectionStreamReader {
	return &DetectionStreamReader{stream: stream}
}

// Read implements io.Reader for detection results
func (r *DetectionStreamReader) Read(p []byte) (n int, err error) {
	detection, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}

	if detection == nil {
		return 0, io.EOF
	}

	// Convert detection to byte representation
	var data []byte
	if detection.IsSpeech {
		data = []byte("speech")
	} else {
		data = []byte("silence")
	}

	n = copy(p, data)

	if n < len(data) {
		return n, io.ErrShortBuffer
	}

	return n, nil
}

// Close closes the stream reader
func (r *DetectionStreamReader) Close() error {
	return r.stream.Close()
}

// DefaultStreamOptions returns default stream options
func DefaultStreamOptions() *StreamOptions {
	return &StreamOptions{
		Threshold:          0.5,
		MinSpeechDuration:  100 * time.Millisecond,
		MinSilenceDuration: 100 * time.Millisecond,
		WindowSize:         30 * time.Millisecond,
		HopSize:            10 * time.Millisecond,
		Metadata:           make(map[string]interface{}),
	}
}

// DefaultSegmentOptions returns default segment options
func DefaultSegmentOptions() *SegmentOptions {
	return &SegmentOptions{
		Threshold:          0.5,
		MinSpeechDuration:  100 * time.Millisecond,
		MinSilenceDuration: 100 * time.Millisecond,
		SpeechPadding:      50 * time.Millisecond,
		SilencePadding:     50 * time.Millisecond,
		ReturnSilence:      false,
		ReturnAudio:        true,
		Metadata:           make(map[string]interface{}),
	}
}
