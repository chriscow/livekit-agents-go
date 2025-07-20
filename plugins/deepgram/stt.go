package deepgram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"livekit-agents-go/media"
	"livekit-agents-go/services/stt"
)

// DeepgramSTT implements real-time streaming STT using Deepgram API
type DeepgramSTT struct {
	*stt.BaseSTT
	apiKey    string
	model     string
	language  string
	baseURL   string
}

// NewDeepgramSTT creates a new Deepgram STT service with streaming support
func NewDeepgramSTT(apiKey string, options ...func(*DeepgramSTT)) *DeepgramSTT {
	supportedLangs := []string{
		"en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh", "ar", "hi", "tr",
		"pl", "uk", "sv", "no", "da", "nl", "fi", "el", "cs", "hu", "ro", "bg",
		"hr", "sk", "sl", "et", "lv", "lt", "mt", "ga", "cy", "is", "mk", "sq",
		"sr", "bs", "eu", "ca", "gl", "ast", "oc", "br", "fo", "kw", "gd", "gv",
		"yi", "he", "uz", "ky", "kk", "mn", "hy", "ka", "az", "be", "tg", "tk",
		"tt", "ba", "cv", "ce", "av", "ab", "os", "lez", "kbd", "uby", "ady",
	}

	d := &DeepgramSTT{
		BaseSTT:  stt.NewBaseSTT("deepgram", "1.0.0", supportedLangs),
		apiKey:   apiKey,
		model:    "nova-3", // Default to latest model
		language: "multi",  // Auto-detect language
		baseURL:  "wss://api.deepgram.com/v1/listen",
	}

	// Apply options
	for _, opt := range options {
		opt(d)
	}

	return d
}

// WithModel sets the Deepgram model to use
func WithModel(model string) func(*DeepgramSTT) {
	return func(d *DeepgramSTT) {
		d.model = model
	}
}

// WithLanguage sets the language for transcription
func WithLanguage(language string) func(*DeepgramSTT) {
	return func(d *DeepgramSTT) {
		d.language = language
	}
}

// Recognize transcribes audio using batch processing (fallback)
func (d *DeepgramSTT) Recognize(ctx context.Context, audio *media.AudioFrame) (*stt.Recognition, error) {
	// For batch recognition, we'll use the streaming interface with a single chunk
	stream, err := d.RecognizeStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Send the audio
	if err := stream.SendAudio(audio); err != nil {
		return nil, fmt.Errorf("failed to send audio: %w", err)
	}

	// Signal end of audio
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("failed to close send: %w", err)
	}

	// Wait for final result
	for {
		result, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to receive result: %w", err)
		}
		if result.IsFinal {
			return result, nil
		}
	}

	// Return empty result if no final result received
	return &stt.Recognition{
		Text:       "",
		Confidence: 0.0,
		IsFinal:    true,
		Language:   d.language,
		Metadata:   make(map[string]interface{}),
	}, nil
}

// RecognizeStream creates a real-time streaming recognition session
func (d *DeepgramSTT) RecognizeStream(ctx context.Context) (stt.RecognitionStream, error) {
	// Build WebSocket URL with parameters
	u, err := url.Parse(d.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Add query parameters for Deepgram API
	params := url.Values{}
	params.Add("model", d.model)
	params.Add("language", d.language)
	params.Add("encoding", "linear16")
	params.Add("sample_rate", "48000") // Match our audio format
	params.Add("channels", "1")
	params.Add("interim_results", "true")
	params.Add("endpointing", "300") // 300ms of silence to end utterance
	params.Add("utterance_end_ms", "1000")
	params.Add("vad_events", "true")
	u.RawQuery = params.Encode()

	log.Printf("🌊 Connecting to Deepgram streaming API: %s", u.String())

	// Set up WebSocket headers
	headers := make(map[string][]string)
	headers["Authorization"] = []string{"Token " + d.apiKey}

	// Connect to Deepgram WebSocket
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil {
			log.Printf("❌ Deepgram connection failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to connect to Deepgram: %w", err)
	}

	log.Printf("✅ Connected to Deepgram streaming API")

	// Create stream
	stream := &DeepgramRecognitionStream{
		conn:        conn,
		ctx:         ctx,
		resultChan:  make(chan *stt.Recognition, 50),
		errorChan:   make(chan error, 10),
		closed:      false,
		sendClosed:  false,
	}

	// Start receiving messages
	go stream.receiveMessages()

	return stream, nil
}

// DeepgramRecognitionStream implements streaming recognition for Deepgram
type DeepgramRecognitionStream struct {
	conn       *websocket.Conn
	ctx        context.Context
	resultChan chan *stt.Recognition
	errorChan  chan error
	closed     bool
	sendClosed bool
	mu         sync.Mutex
}

// SendAudio sends audio data to the recognition stream
func (s *DeepgramRecognitionStream) SendAudio(audio *media.AudioFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.sendClosed {
		return fmt.Errorf("stream is closed")
	}

	// Send raw audio data over WebSocket
	if err := s.conn.WriteMessage(websocket.BinaryMessage, audio.Data); err != nil {
		log.Printf("❌ Failed to send audio to Deepgram: %v", err)
		return fmt.Errorf("failed to send audio: %w", err)
	}

	log.Printf("🎵 Sent %d bytes of audio to Deepgram", len(audio.Data))
	return nil
}

// Recv receives recognition results from the stream
func (s *DeepgramRecognitionStream) Recv() (*stt.Recognition, error) {
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
	}
}

// Close closes the recognition stream
func (s *DeepgramRecognitionStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	
	// Close WebSocket connection
	if err := s.conn.Close(); err != nil {
		log.Printf("⚠️ Error closing Deepgram connection: %v", err)
	}

	// Close channels
	close(s.resultChan)
	close(s.errorChan)

	log.Printf("🔌 Closed Deepgram streaming connection")
	return nil
}

// CloseSend signals that no more audio will be sent
func (s *DeepgramRecognitionStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.sendClosed {
		return nil
	}

	s.sendClosed = true

	// Send close message to Deepgram
	closeMsg := map[string]interface{}{
		"type": "CloseStream",
	}
	
	if err := s.conn.WriteJSON(closeMsg); err != nil {
		log.Printf("⚠️ Error sending close message to Deepgram: %v", err)
	}

	log.Printf("🔚 Signaled end of audio stream to Deepgram")
	return nil
}

// receiveMessages runs in background to receive and process Deepgram responses
func (s *DeepgramRecognitionStream) receiveMessages() {
	defer func() {
		log.Printf("📥 Deepgram message receiver stopped")
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set read deadline to prevent hanging
			s.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			
			// Read message from Deepgram
			_, message, err := s.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("🔌 Deepgram connection closed normally")
					return
				}
				
				s.mu.Lock()
				if !s.closed {
					log.Printf("❌ Deepgram connection error: %v", err)
					select {
					case s.errorChan <- fmt.Errorf("connection error: %w", err):
					default:
					}
				}
				s.mu.Unlock()
				return
			}

			// Parse Deepgram response
			if err := s.handleDeepgramMessage(message); err != nil {
				log.Printf("⚠️ Error handling Deepgram message: %v", err)
			}
		}
	}
}

// DeepgramResponse represents the structure of Deepgram's response
type DeepgramResponse struct {
	Type    string          `json:"type"`
	Channel json.RawMessage `json:"channel"` // Can be array for SpeechStarted or object for Results
	IsFinal bool            `json:"is_final"`
	Duration float64        `json:"duration"`
	Metadata struct {
		RequestID string `json:"request_id"`
		ModelInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"model_info"`
	} `json:"metadata"`
}

// DeepgramResultsChannel represents the channel structure for Results messages
type DeepgramResultsChannel struct {
	Alternatives []struct {
		Transcript string  `json:"transcript"`
		Confidence float64 `json:"confidence"`
		Words      []struct {
			Word       string  `json:"word"`
			Start      float64 `json:"start"`
			End        float64 `json:"end"`
			Confidence float64 `json:"confidence"`
		} `json:"words"`
	} `json:"alternatives"`
}

// handleDeepgramMessage processes incoming messages from Deepgram
func (s *DeepgramRecognitionStream) handleDeepgramMessage(message []byte) error {
	var response DeepgramResponse
	if err := json.Unmarshal(message, &response); err != nil {
		return fmt.Errorf("failed to unmarshal Deepgram response: %w", err)
	}

	log.Printf("📨 Deepgram message type: %s", response.Type)

	switch response.Type {
	case "Results":
		// Parse channel as object for Results messages
		var channel DeepgramResultsChannel
		if err := json.Unmarshal(response.Channel, &channel); err != nil {
			log.Printf("⚠️ Failed to parse channel for Results message: %v", err)
			return nil // Skip this message but don't fail
		}
		
		// Process transcription results
		if len(channel.Alternatives) > 0 {
			alternative := channel.Alternatives[0]
			
			recognition := &stt.Recognition{
				Text:       alternative.Transcript,
				Confidence: alternative.Confidence,
				IsFinal:    response.IsFinal,
				Language:   "auto", // Deepgram auto-detects
				Metadata: map[string]interface{}{
					"model":      response.Metadata.ModelInfo.Name,
					"version":    response.Metadata.ModelInfo.Version,
					"request_id": response.Metadata.RequestID,
					"duration":   response.Duration,
					"word_count": len(alternative.Words),
				},
			}

			// Only send non-empty results or final results
			if len(alternative.Transcript) > 0 || response.IsFinal {
				log.Printf("🎯 Deepgram result: '%s' (final: %t, confidence: %.2f)", 
					alternative.Transcript, response.IsFinal, alternative.Confidence)
				
				select {
				case s.resultChan <- recognition:
				case <-s.ctx.Done():
					return s.ctx.Err()
				default:
					log.Printf("⚠️ Result channel full, dropping result")
				}
			}
		}

	case "Metadata":
		log.Printf("📊 Deepgram metadata: %s", string(message))

	case "SpeechStarted":
		log.Printf("🎤 Deepgram detected speech start")

	case "UtteranceEnd":
		log.Printf("🔇 Deepgram detected utterance end")

	case "Error":
		log.Printf("❌ Deepgram error: %s", string(message))
		return fmt.Errorf("Deepgram API error: %s", string(message))

	default:
		log.Printf("❓ Unknown Deepgram message type: %s", response.Type)
	}

	return nil
}