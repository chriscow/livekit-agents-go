package openai

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

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
	if opts == nil {
		opts = tts.DefaultSynthesizeOptions()
	}
	
	// Validate inputs
	if text == "" {
		return &media.AudioFrame{}, nil // Return empty frame for empty text
	}
	
	if opts.Voice == "" {
		opts.Voice = "alloy" // Default voice
	}
	
	log.Printf("🔊 Starting OpenAI TTS synthesis: '%s' (voice: %s, speed: %.1f)", 
		text, opts.Voice, opts.Speed)
	
	start := time.Now()

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
		log.Printf("❌ OpenAI TTS synthesis failed: %v", err)
		return nil, fmt.Errorf("OpenAI TTS synthesis failed: %w", err)
	}
	defer response.Close()

	// Read the PCM audio data
	audioData, err := io.ReadAll(response)
	if err != nil {
		log.Printf("❌ Failed to read TTS response: %v", err)
		return nil, fmt.Errorf("failed to read TTS response: %w", err)
	}

	// Create audio frame with proper sample rate conversion  
	// OpenAI returns 24kHz but we need 48kHz for the audio pipeline
	audioFormat := media.AudioFormat{
		SampleRate:    48000, // Convert to 48kHz to match audio pipeline
		Channels:      1,     // Mono
		BitsPerSample: 16,    // 16-bit
		Format:        media.AudioFormatPCM,
	}

	// Convert from 24kHz to 48kHz
	convertedData := convertSampleRate(audioData, 24000, 48000)
	audioFrame := media.NewAudioFrame(convertedData, audioFormat)
	
	// Calculate and log performance metrics
	duration := time.Since(start)
	audioSeconds := float64(len(convertedData)) / float64(audioFormat.SampleRate*audioFormat.Channels*audioFormat.BitsPerSample/8)
	log.Printf("✅ OpenAI TTS completed: %d bytes (converted to 48kHz), %.2fs audio, %v processing time", 
		len(convertedData), audioSeconds, duration)
	
	return audioFrame, nil
}

// SynthesizeStream creates a streaming synthesis session
func (o *OpenAITTS) SynthesizeStream(ctx context.Context, opts *tts.SynthesizeOptions) (tts.SynthesisStream, error) {
	if opts == nil {
		opts = tts.DefaultSynthesizeOptions()
	}
	
	if opts.Voice == "" {
		opts.Voice = "alloy" // Default voice
	}
	
	log.Printf("🎤 Creating OpenAI TTS stream (voice: %s, speed: %.1f)", opts.Voice, opts.Speed)
	
	return &OpenAISynthesisStream{
		ctx:        ctx,
		client:     o.client,
		model:      o.model,
		opts:       opts,
		closed:     false,
		textBuffer: make([]string, 0),
		resultChan: make(chan *media.AudioFrame, 10),
		errorChan:  make(chan error, 5),
	}, nil
}

// OpenAISynthesisStream implements the SynthesisStream interface
type OpenAISynthesisStream struct {
	ctx        context.Context
	client     *openai.Client
	model      openai.SpeechModel
	opts       *tts.SynthesizeOptions
	closed     bool
	textBuffer []string
	resultChan chan *media.AudioFrame
	errorChan  chan error
	mu         sync.Mutex
}

// SendText sends text to be synthesized
func (s *OpenAISynthesisStream) SendText(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("synthesis stream is closed")
	}

	if text == "" {
		return nil // Skip empty text
	}

	log.Printf("📝 Received text for TTS streaming: '%s'", text)
	
	// For better streaming latency, process text immediately in chunks
	// Split by sentences for more responsive synthesis
	sentences := s.splitIntoSentences(text)
	
	for _, sentence := range sentences {
		if sentence != "" {
			// Process each sentence immediately
			go s.processSingleText(sentence)
		}
	}
	
	return nil
}

// Recv receives synthesized audio from the stream
func (s *OpenAISynthesisStream) Recv() (*media.AudioFrame, error) {
	if s.closed {
		return nil, io.EOF
	}

	select {
	case audioFrame, ok := <-s.resultChan:
		if !ok || audioFrame == nil {
			// Channel is closed or received nil frame
			return nil, io.EOF
		}
		log.Printf("📤 Returning TTS audio frame: %d bytes", len(audioFrame.Data))
		return audioFrame, nil
	case err := <-s.errorChan:
		log.Printf("❌ TTS streaming error: %v", err)
		return nil, err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

// Close closes the synthesis stream
func (s *OpenAISynthesisStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.closed {
		s.closed = true
		close(s.resultChan)
		close(s.errorChan)
		log.Printf("🔒 OpenAI TTS stream closed")
	}
	return nil
}

// CloseSend signals that no more text will be sent
func (s *OpenAISynthesisStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	log.Printf("🔚 CloseSend called - processing final text buffer (%d chunks)", len(s.textBuffer))
	
	// Process any remaining text in the buffer
	if len(s.textBuffer) > 0 {
		go s.processFinalBuffer()
	}
	
	return nil
}

// processTextBuffer processes the current text buffer for synthesis
func (s *OpenAISynthesisStream) processTextBuffer() {
	s.mu.Lock()
	if len(s.textBuffer) == 0 || s.closed {
		s.mu.Unlock()
		return
	}
	
	// Take all text from buffer for processing
	textToProcess := strings.Join(s.textBuffer, " ")
	s.textBuffer = s.textBuffer[:0] // Clear buffer
	s.mu.Unlock()
	
	if textToProcess == "" {
		return
	}
	
	log.Printf("🔄 Processing TTS text buffer: '%s'", textToProcess)
	
	// Create speech request
	request := openai.CreateSpeechRequest{
		Model:          s.model,
		Input:          textToProcess,
		Voice:          openai.SpeechVoice(s.opts.Voice),
		ResponseFormat: "pcm",
		Speed:          float64(s.opts.Speed),
	}

	// Call OpenAI TTS API
	response, err := s.client.CreateSpeech(s.ctx, request)
	if err != nil {
		select {
		case s.errorChan <- fmt.Errorf("OpenAI TTS synthesis failed: %w", err):
		case <-s.ctx.Done():
		default:
		}
		return
	}
	defer response.Close()

	// Read the PCM audio data
	audioData, err := io.ReadAll(response)
	if err != nil {
		select {
		case s.errorChan <- fmt.Errorf("failed to read TTS response: %w", err):
		case <-s.ctx.Done():
		default:
		}
		return
	}

	// Create audio frame with PCM format
	audioFormat := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	audioFrame := media.NewAudioFrame(audioData, audioFormat)
	
	// Send result
	select {
	case s.resultChan <- audioFrame:
		log.Printf("✅ TTS synthesis completed: %d bytes audio", len(audioData))
	case <-s.ctx.Done():
	default:
		log.Printf("⚠️ TTS result channel full, dropping audio frame")
	}
}

// processFinalBuffer processes all remaining text when stream is closing
func (s *OpenAISynthesisStream) processFinalBuffer() {
	s.processTextBuffer()
}

// splitIntoSentences splits text into sentences for more responsive streaming
func (s *OpenAISynthesisStream) splitIntoSentences(text string) []string {
	// Simple sentence splitting - split on common sentence endings
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	
	// Split on sentence endings but keep the punctuation
	sentences := make([]string, 0)
	current := strings.Builder{}
	
	for i, char := range text {
		current.WriteRune(char)
		
		// Check for sentence endings
		if char == '.' || char == '!' || char == '?' {
			// Look ahead to see if this is end of sentence (not abbreviation)
			if i == len(text)-1 || (i < len(text)-1 && text[i+1] == ' ') {
				sentence := strings.TrimSpace(current.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}
	
	// Add any remaining text
	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}
	
	// If no sentences found, return the whole text
	if len(sentences) == 0 {
		return []string{text}
	}
	
	return sentences
}

// processSingleText processes a single text chunk immediately
func (s *OpenAISynthesisStream) processSingleText(text string) {
	if text == "" {
		return
	}
	
	log.Printf("🎯 Processing single TTS text: '%s'", text)
	
	// Create speech request
	request := openai.CreateSpeechRequest{
		Model:          s.model,
		Input:          text,
		Voice:          openai.SpeechVoice(s.opts.Voice),
		ResponseFormat: "pcm",
		Speed:          float64(s.opts.Speed),
	}

	// Call OpenAI TTS API
	response, err := s.client.CreateSpeech(s.ctx, request)
	if err != nil {
		select {
		case s.errorChan <- fmt.Errorf("OpenAI TTS synthesis failed: %w", err):
		case <-s.ctx.Done():
		default:
		}
		return
	}
	defer response.Close()

	// Read the PCM audio data
	audioData, err := io.ReadAll(response)
	if err != nil {
		select {
		case s.errorChan <- fmt.Errorf("failed to read TTS response: %w", err):
		case <-s.ctx.Done():
		default:
		}
		return
	}

	// Create audio frame with proper sample rate conversion
	// OpenAI returns 24kHz but we need 48kHz for the audio pipeline
	audioFormat := media.AudioFormat{
		SampleRate:    48000, // Convert to 48kHz to match audio pipeline
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	// Convert from 24kHz to 48kHz by simple duplication
	convertedData := convertSampleRate(audioData, 24000, 48000)
	audioFrame := media.NewAudioFrame(convertedData, audioFormat)
	
	// Send result
	select {
	case s.resultChan <- audioFrame:
		log.Printf("✅ TTS synthesis completed: %d bytes audio (converted to 48kHz)", len(convertedData))
	case <-s.ctx.Done():
	default:
		log.Printf("⚠️ TTS result channel full, dropping audio frame")
	}
}

// convertSampleRate performs simple sample rate conversion by duplication (24kHz -> 48kHz)
func convertSampleRate(data []byte, fromRate, toRate int) []byte {
	if fromRate == toRate {
		return data
	}
	
	// Simple 2x upsampling for 24kHz -> 48kHz conversion
	if fromRate == 24000 && toRate == 48000 {
		converted := make([]byte, len(data)*2)
		for i := 0; i < len(data); i += 2 {
			// Copy each 16-bit sample twice
			converted[i*2] = data[i]
			converted[i*2+1] = data[i+1]
			converted[i*2+2] = data[i]
			converted[i*2+3] = data[i+1]
		}
		return converted
	}
	
	// For other conversions, just return original data
	log.Printf("⚠️ Sample rate conversion from %d to %d not implemented", fromRate, toRate)
	return data
}
