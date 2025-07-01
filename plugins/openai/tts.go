package openai

import (
	"context"
	"fmt"
	"io"

	"livekit-agents-go/media"
	"livekit-agents-go/services/tts"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAITTS implements the TTS interface using OpenAI TTS
type OpenAITTS struct {
	*tts.BaseTTS
	client *openai.Client
	model  openai.SpeechModel
}

// NewOpenAITTS creates a new OpenAI TTS service
func NewOpenAITTS(apiKey string) *OpenAITTS {
	voices := []tts.Voice{
		{ID: "alloy", Name: "Alloy", Gender: "neutral", Language: "en"},
		{ID: "echo", Name: "Echo", Gender: "male", Language: "en"},
		{ID: "fable", Name: "Fable", Gender: "neutral", Language: "en"},
		{ID: "onyx", Name: "Onyx", Gender: "male", Language: "en"},
		{ID: "nova", Name: "Nova", Gender: "female", Language: "en"},
		{ID: "shimmer", Name: "Shimmer", Gender: "female", Language: "en"},
	}

	return &OpenAITTS{
		BaseTTS: tts.NewBaseTTS("openai-tts", "1.0", voices),
		client:  openai.NewClient(apiKey),
		model:   openai.TTSModel1HD, // Use high-definition model
	}
}

// Synthesize converts text to speech audio
func (o *OpenAITTS) Synthesize(ctx context.Context, text string, opts *tts.SynthesizeOptions) (*media.AudioFrame, error) {
	// Create speech request
	request := openai.CreateSpeechRequest{
		Model:          o.model,
		Input:          text,
		Voice:          openai.SpeechVoice(opts.Voice),
		ResponseFormat: "pcm", // Request PCM format directly
		Speed:          float64(opts.Speed),
	}

	// Call OpenAI TTS API
	response, err := o.client.CreateSpeech(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("OpenAI TTS synthesis failed: %w", err)
	}
	defer response.Close()

	// Read the PCM audio data
	audioData, err := io.ReadAll(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS response: %w", err)
	}

	// Create audio frame with PCM format (24kHz, 16-bit, mono as per OpenAI TTS)
	audioFormat := media.AudioFormat{
		SampleRate:    24000, // OpenAI TTS PCM is 24kHz
		Channels:      1,     // Mono
		BitsPerSample: 16,    // 16-bit
		Format:        media.AudioFormatPCM,
	}

	audioFrame := media.NewAudioFrame(audioData, audioFormat)
	return audioFrame, nil
}

// SynthesizeStream creates a streaming synthesis session
func (o *OpenAITTS) SynthesizeStream(ctx context.Context, opts *tts.SynthesizeOptions) (tts.SynthesisStream, error) {
	return &OpenAISynthesisStream{
		ctx:    ctx,
		client: o.client,
		model:  o.model,
		opts:   opts,
		closed: false,
	}, nil
}

// OpenAISynthesisStream implements the SynthesisStream interface
type OpenAISynthesisStream struct {
	ctx    context.Context
	client *openai.Client
	model  openai.SpeechModel
	opts   *tts.SynthesizeOptions
	closed bool
	text   string
}

// SendText sends text to be synthesized
func (s *OpenAISynthesisStream) SendText(text string) error {
	if s.closed {
		return fmt.Errorf("synthesis stream is closed")
	}

	// For OpenAI TTS, we synthesize immediately when text is sent
	// Store the text for processing in Recv()
	s.text = text
	return nil
}

// Recv receives synthesized audio from the stream
func (s *OpenAISynthesisStream) Recv() (*media.AudioFrame, error) {
	if s.closed {
		return nil, io.EOF
	}

	if s.text == "" {
		return nil, fmt.Errorf("no text to synthesize")
	}

	// Create speech request
	request := openai.CreateSpeechRequest{
		Model:          s.model,
		Input:          s.text,
		Voice:          openai.SpeechVoice(s.opts.Voice),
		ResponseFormat: "pcm",
		Speed:          float64(s.opts.Speed),
	}

	// Call OpenAI TTS API
	response, err := s.client.CreateSpeech(s.ctx, request)
	if err != nil {
		return nil, fmt.Errorf("OpenAI TTS synthesis failed: %w", err)
	}
	defer response.Close()

	// Read the PCM audio data
	audioData, err := io.ReadAll(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS response: %w", err)
	}

	// Create audio frame with PCM format
	audioFormat := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	audioFrame := media.NewAudioFrame(audioData, audioFormat)

	// Clear text after processing
	s.text = ""
	return audioFrame, nil
}

// Close closes the synthesis stream
func (s *OpenAISynthesisStream) Close() error {
	s.closed = true
	return nil
}

// CloseSend signals that no more text will be sent
func (s *OpenAISynthesisStream) CloseSend() error {
	// For OpenAI TTS, we don't need to do anything special
	return nil
}
