package openai

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/services/stt"

	openai "github.com/sashabaranov/go-openai"
)

// WhisperSTT implements the STT interface using OpenAI Whisper
type WhisperSTT struct {
	*stt.BaseSTT
	client *openai.Client
	model  string
}

// NewWhisperSTT creates a new Whisper STT service
func NewWhisperSTT(apiKey string) *WhisperSTT {
	supportedLangs := []string{
		"en", "zh", "de", "es", "ru", "ko", "fr", "ja", "pt", "tr", "pl", "ca", "nl",
		"ar", "sv", "it", "id", "hi", "fi", "vi", "he", "uk", "el", "ms", "cs", "ro",
		"da", "hu", "ta", "no", "th", "ur", "hr", "bg", "lt", "la", "mi", "ml", "cy",
		"sk", "te", "fa", "lv", "bn", "sr", "az", "sl", "kn", "et", "mk", "br", "eu",
		"is", "hy", "ne", "mn", "bs", "kk", "sq", "sw", "gl", "mr", "pa", "si", "km",
		"sn", "yo", "so", "af", "oc", "ka", "be", "tg", "sd", "gu", "am", "yi", "lo",
		"uz", "fo", "ht", "ps", "tk", "nn", "mt", "sa", "lb", "my", "bo", "tl", "mg",
		"as", "tt", "haw", "ln", "ha", "ba", "jw", "su",
	}

	return &WhisperSTT{
		BaseSTT: stt.NewBaseSTT("whisper", "1.0.0", supportedLangs),
		client:  openai.NewClient(apiKey),
		model:   openai.Whisper1,
	}
}

// Recognize transcribes audio using Whisper
func (w *WhisperSTT) Recognize(ctx context.Context, audio *media.AudioFrame) (*stt.Recognition, error) {
	if audio.IsEmpty() {
		return &stt.Recognition{
			Text:       "",
			Confidence: 0.0,
			IsFinal:    true,
			Metadata:   make(map[string]interface{}),
		}, nil
	}

	// Check if audio meets OpenAI's minimum duration requirement (0.1 seconds)
	minDuration := 100 * time.Millisecond
	if audio.Duration < minDuration {
		log.Printf("⚠️ Audio too short for Whisper (%v < %v) - returning empty result", audio.Duration, minDuration)
		return &stt.Recognition{
			Text:       "",
			Confidence: 0.0,
			IsFinal:    true,
			Language:   "en",
			Metadata:   make(map[string]interface{}),
		}, nil
	}

	log.Printf("🎙️ Starting Whisper transcription for %d bytes of audio (duration: %v)", len(audio.Data), audio.Duration)
	
	// Convert AudioFrame to WAV format for OpenAI API
	wavData, err := w.convertToWAV(audio)
	if err != nil {
		return nil, fmt.Errorf("failed to convert audio to WAV: %w", err)
	}

	// Create a reader from the WAV data
	audioReader := bytes.NewReader(wavData)

	// Create transcription request
	req := openai.AudioRequest{
		Model:    w.model,
		Language: "en", // TODO: Make this configurable
		Format:   openai.AudioResponseFormatJSON,
		Reader:   audioReader,
		FilePath: "audio.wav", // Required by OpenAI API
	}

	// Call OpenAI Whisper API
	response, err := w.client.CreateTranscription(ctx, req)
	if err != nil {
		log.Printf("❌ Whisper transcription failed: %v", err)
		return nil, fmt.Errorf("failed to transcribe audio: %w", err)
	}

	log.Printf("🎯 Whisper transcription result: '%s'", response.Text)

	// Calculate confidence based on segments if available
	confidence := 0.95 // Default confidence
	if len(response.Segments) > 0 {
		totalConfidence := 0.0
		for _, segment := range response.Segments {
			// Use inverse of no_speech_prob as a confidence indicator
			segmentConfidence := 1.0 - segment.NoSpeechProb
			totalConfidence += segmentConfidence
		}
		confidence = totalConfidence / float64(len(response.Segments))
	}

	return &stt.Recognition{
		Text:       response.Text,
		Confidence: confidence,
		Language:   response.Language,
		IsFinal:    true,
		Metadata: map[string]interface{}{
			"model":    w.model,
			"duration": response.Duration,
			"segments": len(response.Segments),
		},
	}, nil
}

// RecognizeStream creates a streaming recognition session
func (w *WhisperSTT) RecognizeStream(ctx context.Context) (stt.RecognitionStream, error) {
	stream := &WhisperRecognitionStream{
		stt:            w,
		ctx:            ctx,
		closed:         false,
		audioBuffer:    make([]*media.AudioFrame, 0),
		resultChan:     make(chan *stt.Recognition, 10),
		errorChan:      make(chan error, 5),
		processingDone: make(chan struct{}),
	}
	
	// Start background processing goroutine
	go stream.processAudioBuffer()
	
	return stream, nil
}

// WhisperRecognitionStream implements streaming recognition for Whisper
type WhisperRecognitionStream struct {
	stt            *WhisperSTT
	ctx            context.Context
	closed         bool
	resultChanClosed bool
	errorChanClosed  bool
	processingClosed bool
	audioBuffer    []*media.AudioFrame
	resultChan     chan *stt.Recognition
	errorChan      chan error
	processingDone chan struct{}
	mu             sync.Mutex
}

// SendAudio sends audio data to the recognition stream
func (s *WhisperRecognitionStream) SendAudio(audio *media.AudioFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	// Buffer audio frames for batch processing
	// Whisper doesn't support true streaming, so we accumulate audio
	// and process it in chunks of ~3 seconds for optimal results
	s.audioBuffer = append(s.audioBuffer, audio)
	
	log.Printf("🎙️ Buffered audio frame: %d bytes (total frames: %d)", 
		len(audio.Data), len(s.audioBuffer))

	return nil
}

// Recv receives recognition results from the stream
func (s *WhisperRecognitionStream) Recv() (*stt.Recognition, error) {
	if s.closed {
		return nil, io.EOF
	}

	select {
	case result := <-s.resultChan:
		return result, nil
	case err := <-s.errorChan:
		return nil, err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case <-s.processingDone:
		// Check if there are any remaining results
		select {
		case result := <-s.resultChan:
			return result, nil
		default:
			return nil, io.EOF
		}
	}
}

// Close closes the recognition stream
func (s *WhisperRecognitionStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.closed {
		s.closed = true
		
		// Close channels safely
		if !s.processingClosed {
			close(s.processingDone)
			s.processingClosed = true
		}
		if !s.resultChanClosed {
			close(s.resultChan)
			s.resultChanClosed = true
		}
		if !s.errorChanClosed {
			close(s.errorChan)
			s.errorChanClosed = true
		}
	}
	return nil
}

// CloseSend signals that no more audio will be sent
func (s *WhisperRecognitionStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("stream is already closed")
	}
	
	log.Printf("🔚 CloseSend called - processing final audio buffer (%d frames)", len(s.audioBuffer))
	
	// Process any remaining audio in the buffer
	if len(s.audioBuffer) > 0 {
		go s.processFinalBuffer()
	} else {
		// No buffered audio, signal completion
		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if !s.processingClosed {
				close(s.processingDone)
				s.processingClosed = true
			}
		}()
	}
	
	return nil
}

// processAudioBuffer runs in the background to process audio chunks
func (s *WhisperRecognitionStream) processAudioBuffer() {
	ticker := time.NewTicker(3 * time.Second) // Process every 3 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.processBufferedAudio(false)
		case <-s.ctx.Done():
			return
		case <-s.processingDone:
			return
		}
	}
}

// processFinalBuffer processes all remaining audio when stream is closing
func (s *WhisperRecognitionStream) processFinalBuffer() {
	s.processBufferedAudio(true)
	
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.processingClosed {
		close(s.processingDone)
		s.processingClosed = true
	}
}

// processBufferedAudio processes currently buffered audio frames
func (s *WhisperRecognitionStream) processBufferedAudio(isFinal bool) {
	s.mu.Lock()
	frames := make([]*media.AudioFrame, len(s.audioBuffer))
	copy(frames, s.audioBuffer)
	if !isFinal {
		// Keep last few frames for continuity unless this is final processing
		keepFrames := 2
		if len(s.audioBuffer) > keepFrames {
			s.audioBuffer = s.audioBuffer[len(s.audioBuffer)-keepFrames:]
		}
	} else {
		s.audioBuffer = nil
	}
	s.mu.Unlock()
	
	if len(frames) == 0 {
		return
	}
	
	log.Printf("🔄 Processing %d buffered audio frames (final: %t)", len(frames), isFinal)
	
	// Combine all frames into a single audio frame
	combined, err := s.stt.AccumulateAudioForSTT(frames)
	if err != nil {
		log.Printf("❌ Failed to combine audio frames: %v", err)
		select {
		case s.errorChan <- err:
		case <-s.ctx.Done():
		case <-s.processingDone:
		}
		return
	}
	
	// Skip processing if combined audio is too short (OpenAI requires minimum 0.1 seconds)
	minDuration := 100 * time.Millisecond
	if combined.Duration < minDuration {
		if isFinal {
			log.Printf("⚠️ Final audio too short (%v < %v) - sending empty result", combined.Duration, minDuration)
			// Send empty result for final processing
			recognition := &stt.Recognition{
				Text:       "",
				Confidence: 0.0,
				IsFinal:    true,
				Language:   "en",
				Metadata:   make(map[string]interface{}),
			}
			select {
			case s.resultChan <- recognition:
			case <-s.ctx.Done():
			case <-s.processingDone:
			}
		} else {
			log.Printf("⏭️ Skipping recognition - audio too short (%v < %v)", combined.Duration, minDuration)
		}
		return
	}
	
	// Perform recognition
	recognition, err := s.stt.Recognize(s.ctx, combined)
	if err != nil {
		log.Printf("❌ Streaming recognition failed: %v", err)
		select {
		case s.errorChan <- err:
		case <-s.ctx.Done():
		case <-s.processingDone:
		}
		return
	}
	
	// Mark result as final only if this is the final processing or we got substantial text
	recognition.IsFinal = isFinal || len(recognition.Text) > 0
	
	// Send result
	select {
	case s.resultChan <- recognition:
		log.Printf("📤 Sent streaming recognition result: '%s' (final: %t)", recognition.Text, recognition.IsFinal)
	case <-s.ctx.Done():
	case <-s.processingDone:
	}
}

// convertToWAV converts an AudioFrame to WAV format for OpenAI API
func (w *WhisperSTT) convertToWAV(audio *media.AudioFrame) ([]byte, error) {
	// WAV file structure:
	// - RIFF header (12 bytes)
	// - fmt chunk (24 bytes for PCM)
	// - data chunk header (8 bytes)
	// - audio data

	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	fileSize := uint32(36 + len(audio.Data)) // Total file size - 8 bytes
	binary.Write(&buf, binary.LittleEndian, fileSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	chunkSize := uint32(16) // PCM format chunk size
	binary.Write(&buf, binary.LittleEndian, chunkSize)
	
	audioFormat := uint16(1) // PCM
	binary.Write(&buf, binary.LittleEndian, audioFormat)
	
	numChannels := uint16(audio.Format.Channels)
	binary.Write(&buf, binary.LittleEndian, numChannels)
	
	sampleRate := uint32(audio.Format.SampleRate)
	binary.Write(&buf, binary.LittleEndian, sampleRate)
	
	bitsPerSample := uint16(audio.Format.BitsPerSample)
	byteRate := uint32(sampleRate * uint32(numChannels) * uint32(bitsPerSample) / 8)
	binary.Write(&buf, binary.LittleEndian, byteRate)
	
	blockAlign := uint16(numChannels * bitsPerSample / 8)
	binary.Write(&buf, binary.LittleEndian, blockAlign)
	
	binary.Write(&buf, binary.LittleEndian, bitsPerSample)

	// data chunk
	buf.WriteString("data")
	dataSize := uint32(len(audio.Data))
	binary.Write(&buf, binary.LittleEndian, dataSize)
	
	// Write audio data
	buf.Write(audio.Data)

	return buf.Bytes(), nil
}

// AccumulateAudioForSTT accumulates audio frames for batch processing
func (w *WhisperSTT) AccumulateAudioForSTT(frames []*media.AudioFrame) (*media.AudioFrame, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no audio frames to accumulate")
	}

	// Use format from first frame
	format := frames[0].Format
	
	// Calculate total data size
	totalSize := 0
	for _, frame := range frames {
		totalSize += len(frame.Data)
	}

	// Concatenate all audio data
	combinedData := make([]byte, 0, totalSize)
	var totalDuration time.Duration
	
	for _, frame := range frames {
		combinedData = append(combinedData, frame.Data...)
		totalDuration += frame.Duration
	}

	// Create combined audio frame
	combined := media.NewAudioFrame(combinedData, format)
	combined.Duration = totalDuration
	
	return combined, nil
}
