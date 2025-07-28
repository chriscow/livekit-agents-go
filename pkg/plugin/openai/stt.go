// Package openai provides OpenAI-based AI providers (STT, TTS, LLM).
// This plugin integrates with OpenAI's APIs including Whisper for speech-to-text.
package openai

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
	openai "github.com/sashabaranov/go-openai"
)

// WhisperSTT implements STT using OpenAI's Whisper API.
type WhisperSTT struct {
	client   *openai.Client
	model    string
	language string
}

// Config holds configuration for OpenAI STT.
type Config struct {
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`    // Default: whisper-1
	Language string `json:"language"` // Default: auto-detect (empty)
}

// NewWhisperSTT creates a new OpenAI Whisper STT provider.
func NewWhisperSTT(cfg Config) (*WhisperSTT, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	
	model := cfg.Model
	if model == "" {
		model = openai.Whisper1
	}

	return &WhisperSTT{
		client:   openai.NewClient(cfg.APIKey),
		model:    model,
		language: cfg.Language,
	}, nil
}

// NewStream creates a new STT streaming session.
func (w *WhisperSTT) NewStream(ctx context.Context, cfg stt.StreamConfig) (stt.STTStream, error) {
	stream := &whisperStream{
		stt:         w,
		ctx:         ctx,
		config:      cfg,
		eventChan:   make(chan stt.SpeechEvent, 10),
		audioBuffer: make([]rtc.AudioFrame, 0),
	}

	go stream.processLoop()
	return stream, nil
}

// Capabilities returns the STT capabilities.
func (w *WhisperSTT) Capabilities() stt.STTCapabilities {
	return stt.STTCapabilities{
		Streaming:      true, // Pseudo-streaming via batching
		InterimResults: false, // Whisper doesn't support interim results
		SupportedLanguages: []string{
			"en", "zh", "de", "es", "ru", "ko", "fr", "ja", "pt", "tr", "pl", "ca", "nl",
			"ar", "sv", "it", "id", "hi", "fi", "vi", "he", "uk", "el", "ms", "cs", "ro",
			"da", "hu", "ta", "no", "th", "ur", "hr", "bg", "lt", "la", "mi", "ml", "cy",
			"sk", "te", "fa", "lv", "bn", "sr", "az", "sl", "kn", "et", "mk", "br", "eu",
			"is", "hy", "ne", "mn", "bs", "kk", "sq", "sw", "gl", "mr", "pa", "si", "km",
			"sn", "yo", "so", "af", "oc", "ka", "be", "tg", "sd", "gu", "am", "yi", "lo",
			"uz", "fo", "ht", "ps", "tk", "nn", "mt", "sa", "lb", "my", "bo", "tl", "mg",
			"as", "tt", "haw", "ln", "ha", "ba", "jw", "su",
		},
		SampleRates: []int{16000, 22050, 44100, 48000},
	}
}

// whisperStream implements STT streaming using batched processing.
type whisperStream struct {
	stt         *WhisperSTT
	ctx         context.Context
	config      stt.StreamConfig
	eventChan   chan stt.SpeechEvent
	audioBuffer []rtc.AudioFrame
	closed      bool
	mu          sync.Mutex
}

// Push sends an audio frame for processing.
func (s *whisperStream) Push(frame rtc.AudioFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	s.audioBuffer = append(s.audioBuffer, frame)
	return nil
}

// Events returns the channel for receiving speech events.
func (s *whisperStream) Events() <-chan stt.SpeechEvent {
	return s.eventChan
}

// CloseSend signals that no more audio will be sent.
func (s *whisperStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stream already closed")
	}

	s.closed = true
	return nil
}

// processLoop runs the background processing loop.
func (s *whisperStream) processLoop() {
	defer close(s.eventChan)

	ticker := time.NewTicker(3 * time.Second) // Process every 3 seconds
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processBufferedAudio(false)
		}

		// Check if stream is closed and process final audio
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			s.processBufferedAudio(true)
			return
		}
		s.mu.Unlock()
	}
}

// processBufferedAudio processes currently buffered audio frames.
func (s *whisperStream) processBufferedAudio(isFinal bool) {
	s.mu.Lock()
	if len(s.audioBuffer) == 0 {
		s.mu.Unlock()
		return
	}

	frames := make([]rtc.AudioFrame, len(s.audioBuffer))
	copy(frames, s.audioBuffer)
	
	if !isFinal {
		// Keep last few frames for continuity
		keepFrames := 2
		if len(s.audioBuffer) > keepFrames {
			s.audioBuffer = s.audioBuffer[len(s.audioBuffer)-keepFrames:]
		}
	} else {
		s.audioBuffer = nil
	}
	s.mu.Unlock()

	// Combine frames into single audio data
	combined, err := s.combineFrames(frames)
	if err != nil {
		slog.Error("Failed to combine audio frames", slog.String("error", err.Error()))
		s.sendErrorEvent(err)
		return
	}

	// Check minimum duration requirement (OpenAI requires ≥ 0.1 seconds)
	minDuration := 100 * time.Millisecond
	if combined.duration < minDuration {
		if isFinal {
			// Send empty result for final processing
			s.sendFinalEvent("", "")
		}
		return
	}

	// Convert to WAV and transcribe
	wavData, err := s.convertToWAV(combined)
	if err != nil {
		slog.Error("Failed to convert audio to WAV", slog.String("error", err.Error()))
		s.sendErrorEvent(err)
		return
	}

	// Call OpenAI Whisper API  
	text, language, err := s.transcribe(wavData)
	if err != nil {
		slog.Error("Whisper transcription failed", slog.String("error", err.Error()))
		s.sendErrorEvent(err)
		return
	}

	// Send result
	if isFinal {
		s.sendFinalEvent(text, language)
	} else if text != "" {
		s.sendFinalEvent(text, language) // Whisper doesn't support interim results
	}
}

// combinedAudio represents combined audio data.
type combinedAudio struct {
	data        []byte
	sampleRate  int
	channels    int
	duration    time.Duration
}

// combineFrames combines multiple audio frames into a single buffer.
func (s *whisperStream) combineFrames(frames []rtc.AudioFrame) (*combinedAudio, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames to combine")
	}

	// Calculate total size
	totalSize := 0
	var totalDuration time.Duration
	sampleRate := frames[0].SampleRate
	channels := frames[0].NumChannels

	for _, frame := range frames {
		totalSize += len(frame.Data)
		totalDuration += frame.Timestamp
	}

	// Combine data
	combined := make([]byte, 0, totalSize)
	for _, frame := range frames {
		combined = append(combined, frame.Data...)
	}

	return &combinedAudio{
		data:       combined,
		sampleRate: sampleRate,
		channels:   channels,
		duration:   totalDuration,
	}, nil
}

// convertToWAV converts audio data to WAV format for OpenAI API.
func (s *whisperStream) convertToWAV(audio *combinedAudio) ([]byte, error) {
	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	fileSize := uint32(36 + len(audio.data))
	binary.Write(&buf, binary.LittleEndian, fileSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	chunkSize := uint32(16)
	binary.Write(&buf, binary.LittleEndian, chunkSize)

	audioFormat := uint16(1) // PCM
	binary.Write(&buf, binary.LittleEndian, audioFormat)

	numChannels := uint16(audio.channels)
	binary.Write(&buf, binary.LittleEndian, numChannels)

	sampleRate := uint32(audio.sampleRate)
	binary.Write(&buf, binary.LittleEndian, sampleRate)

	bitsPerSample := uint16(16) // Assume 16-bit
	byteRate := uint32(sampleRate * uint32(numChannels) * uint32(bitsPerSample) / 8)
	binary.Write(&buf, binary.LittleEndian, byteRate)

	blockAlign := uint16(numChannels * bitsPerSample / 8)
	binary.Write(&buf, binary.LittleEndian, blockAlign)

	binary.Write(&buf, binary.LittleEndian, bitsPerSample)

	// data chunk
	buf.WriteString("data")
	dataSize := uint32(len(audio.data))
	binary.Write(&buf, binary.LittleEndian, dataSize)

	buf.Write(audio.data)

	return buf.Bytes(), nil
}

// transcribe calls the OpenAI Whisper API.
func (s *whisperStream) transcribe(wavData []byte) (string, string, error) {
	reader := bytes.NewReader(wavData)

	req := openai.AudioRequest{
		Model:    s.stt.model,
		Language: s.stt.language,
		Format:   openai.AudioResponseFormatJSON,
		Reader:   reader,
		FilePath: "audio.wav",
	}

	response, err := s.stt.client.CreateTranscription(s.ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("transcription failed: %w", err)
	}

	slog.Debug("Whisper transcription result", slog.String("text", response.Text))

	return response.Text, response.Language, nil
}

// sendFinalEvent sends a final speech event.
func (s *whisperStream) sendFinalEvent(text, language string) {
	event := stt.SpeechEvent{
		Type:      stt.SpeechEventFinal,
		Text:      text,
		IsFinal:   true,
		Language:  language,
		Timestamp: time.Now().UnixMilli(),
	}

	select {
	case s.eventChan <- event:
	case <-s.ctx.Done():
	}
}

// sendErrorEvent sends an error event.
func (s *whisperStream) sendErrorEvent(err error) {
	event := stt.SpeechEvent{
		Type:      stt.SpeechEventError,
		Error:     err,
		Timestamp: time.Now().UnixMilli(),
	}

	select {
	case s.eventChan <- event:
	case <-s.ctx.Done():
	}
}