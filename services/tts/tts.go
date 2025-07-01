package tts

import (
	"context"
	"io"
	"livekit-agents-go/media"
)

// TTS defines the Text-to-Speech service interface
type TTS interface {
	// Synthesize speech from text
	Synthesize(ctx context.Context, text string, opts *SynthesizeOptions) (*media.AudioFrame, error)

	// SynthesizeStream creates a streaming synthesis session
	SynthesizeStream(ctx context.Context, opts *SynthesizeOptions) (SynthesisStream, error)

	// Voices returns available voices for this TTS service
	Voices() []Voice

	// Service metadata
	Name() string
	Version() string
}

// SynthesizeOptions configures speech synthesis
type SynthesizeOptions struct {
	Voice      string
	Language   string
	Speed      float64
	Pitch      float64
	Volume     float64
	Format     media.AudioFormat
	SampleRate int
	Metadata   map[string]interface{}
}

// Voice represents a TTS voice
type Voice struct {
	ID       string
	Name     string
	Gender   string
	Language string
	Preview  string
	Metadata map[string]interface{}
}

// SynthesisStream represents a streaming synthesis session
type SynthesisStream interface {
	// SendText sends text to be synthesized
	SendText(text string) error

	// Recv receives synthesized audio from the stream
	Recv() (*media.AudioFrame, error)

	// Close closes the synthesis stream
	Close() error

	// CloseSend signals that no more text will be sent
	CloseSend() error
}

// StreamingTTS extends TTS for services that support streaming
type StreamingTTS interface {
	TTS

	// Stream synthesis with advanced options
	Stream(ctx context.Context, opts *StreamSynthesizeOptions) (SynthesisStream, error)
}

// StreamSynthesizeOptions configures streaming synthesis
type StreamSynthesizeOptions struct {
	Voice      string
	Language   string
	Speed      float64
	Pitch      float64
	Volume     float64
	Format     media.AudioFormat
	EnableSSML bool
	ChunkSize  int
	BufferSize int
	Metadata   map[string]interface{}
}

// SynthesisResult represents synthesis results
type SynthesisResult struct {
	Audio    *media.AudioFrame
	Error    error
	StreamID string
	IsFinal  bool
}

// SSML represents Speech Synthesis Markup Language
type SSML struct {
	Text     string
	Elements []SSMLElement
}

// SSMLElement represents an SSML element
type SSMLElement struct {
	Type       SSMLElementType
	Text       string
	Attributes map[string]string
}

type SSMLElementType int

const (
	SSMLBreak SSMLElementType = iota
	SSMLEmphasis
	SSMLProsody
	SSMLSayAs
	SSMLSubstitute
	SSMLPhoneme
)

// BaseTTS provides common functionality for TTS implementations
type BaseTTS struct {
	name    string
	version string
	voices  []Voice
}

// NewBaseTTS creates a new base TTS service
func NewBaseTTS(name, version string, voices []Voice) *BaseTTS {
	return &BaseTTS{
		name:    name,
		version: version,
		voices:  voices,
	}
}

func (b *BaseTTS) Name() string {
	return b.name
}

func (b *BaseTTS) Version() string {
	return b.version
}

func (b *BaseTTS) Voices() []Voice {
	return b.voices
}

// StreamSynthesisReader provides a reader interface for streaming synthesis
type StreamSynthesisReader struct {
	stream SynthesisStream
}

// NewStreamSynthesisReader creates a new stream reader
func NewStreamSynthesisReader(stream SynthesisStream) *StreamSynthesisReader {
	return &StreamSynthesisReader{stream: stream}
}

// Read implements io.Reader for audio frames
func (r *StreamSynthesisReader) Read(p []byte) (n int, err error) {
	frame, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}

	if frame == nil || frame.IsEmpty() {
		return 0, io.EOF
	}

	n = copy(p, frame.Data)

	if n < len(frame.Data) {
		return n, io.ErrShortBuffer
	}

	return n, nil
}

// Close closes the stream reader
func (r *StreamSynthesisReader) Close() error {
	return r.stream.Close()
}

// DefaultSynthesizeOptions returns default synthesis options
func DefaultSynthesizeOptions() *SynthesizeOptions {
	return &SynthesizeOptions{
		Speed:      1.0,
		Pitch:      1.0,
		Volume:     1.0,
		Format:     media.AudioFormat48kHz16BitMono,
		SampleRate: 48000,
		Metadata:   make(map[string]interface{}),
	}
}

// DefaultStreamSynthesizeOptions returns default stream synthesis options
func DefaultStreamSynthesizeOptions() *StreamSynthesizeOptions {
	return &StreamSynthesizeOptions{
		Speed:      1.0,
		Pitch:      1.0,
		Volume:     1.0,
		Format:     media.AudioFormat48kHz16BitMono,
		ChunkSize:  1024,
		BufferSize: 4096,
		Metadata:   make(map[string]interface{}),
	}
}
