package stt

import (
	"context"
	"io"
	"livekit-agents-go/media"
)

// STT defines the Speech-to-Text service interface
type STT interface {
	// Recognize speech from audio sample
	Recognize(ctx context.Context, audio *media.AudioFrame) (*Recognition, error)

	// RecognizeStream creates a streaming recognition session
	RecognizeStream(ctx context.Context) (RecognitionStream, error)

	// SupportedLanguages returns the languages supported by this STT service
	SupportedLanguages() []string

	// Service metadata
	Name() string
	Version() string
}

// Recognition represents the result of speech recognition
type Recognition struct {
	Text       string
	Confidence float64
	Language   string
	IsFinal    bool
	Metadata   map[string]interface{}
}

// RecognitionStream represents a streaming recognition session
type RecognitionStream interface {
	// SendAudio sends audio data to the recognition stream
	SendAudio(audio *media.AudioFrame) error

	// Recv receives recognition results from the stream
	Recv() (*Recognition, error)

	// Close closes the recognition stream
	Close() error

	// CloseSend signals that no more audio will be sent
	CloseSend() error
}

// StreamingSTT extends STT for services that support streaming
type StreamingSTT interface {
	STT

	// Stream recognition with more advanced options
	Stream(ctx context.Context, opts *StreamOptions) (RecognitionStream, error)
}

// StreamOptions configures streaming recognition
type StreamOptions struct {
	Language           string
	Model              string
	InterimResults     bool
	MaxAlternatives    int
	ProfanityFilter    bool
	WordTimeOffsets    bool
	SpeakerDiarization bool
	Metadata           map[string]interface{}
}

// RecognitionResult represents a complete recognition result
type RecognitionResult struct {
	Recognition *Recognition
	Error       error
	StreamID    string
}

// Alternative represents alternative recognition results
type Alternative struct {
	Text       string
	Confidence float64
	Words      []WordInfo
}

// WordInfo represents timing information for individual words
type WordInfo struct {
	Word       string
	StartTime  float64
	EndTime    float64
	Confidence float64
	SpeakerTag int
}

// StreamingRecognitionResult represents streaming recognition results
type StreamingRecognitionResult struct {
	Alternatives  []Alternative
	IsFinal       bool
	Stability     float64
	ResultEndTime float64
	ChannelTag    int
	LanguageCode  string
}

// BaseSTT provides common functionality for STT implementations
type BaseSTT struct {
	name           string
	version        string
	supportedLangs []string
}

// NewBaseSTT creates a new base STT service
func NewBaseSTT(name, version string, supportedLangs []string) *BaseSTT {
	return &BaseSTT{
		name:           name,
		version:        version,
		supportedLangs: supportedLangs,
	}
}

func (b *BaseSTT) Name() string {
	return b.name
}

func (b *BaseSTT) Version() string {
	return b.version
}

func (b *BaseSTT) SupportedLanguages() []string {
	return b.supportedLangs
}

// StreamRecognitionReader provides a reader interface for streaming recognition
type StreamRecognitionReader struct {
	stream RecognitionStream
}

// NewStreamRecognitionReader creates a new stream reader
func NewStreamRecognitionReader(stream RecognitionStream) *StreamRecognitionReader {
	return &StreamRecognitionReader{stream: stream}
}

// Read implements io.Reader for recognition results
func (r *StreamRecognitionReader) Read(p []byte) (n int, err error) {
	result, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}

	if result == nil {
		return 0, io.EOF
	}

	data := []byte(result.Text)
	n = copy(p, data)

	if n < len(data) {
		return n, io.ErrShortBuffer
	}

	return n, nil
}

// Close closes the stream reader
func (r *StreamRecognitionReader) Close() error {
	return r.stream.Close()
}
