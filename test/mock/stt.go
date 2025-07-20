package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/services/stt"
)

// MockSTT implements the STT interface for testing
type MockSTT struct {
	*stt.BaseSTT
	responses     []string
	responseIndex int
	delay         time.Duration
	confidence    float64
}

// NewMockSTT creates a new mock STT service
func NewMockSTT(responses ...string) *MockSTT {
	if len(responses) == 0 {
		responses = []string{"Hello world", "This is a test", "Mock recognition result"}
	}

	return &MockSTT{
		BaseSTT:       stt.NewBaseSTT("mock-stt", "1.0.0", []string{"en-US", "en-GB"}),
		responses:     responses,
		responseIndex: 0,
		delay:         100 * time.Millisecond,
		confidence:    0.95,
	}
}

// SetDelay sets the mock delay for recognition
func (m *MockSTT) SetDelay(delay time.Duration) {
	m.delay = delay
}

// SetConfidence sets the mock confidence score
func (m *MockSTT) SetConfidence(confidence float64) {
	m.confidence = confidence
}

// AddResponse adds a response to the mock
func (m *MockSTT) AddResponse(response string) {
	m.responses = append(m.responses, response)
}

// Recognize implements stt.STT
func (m *MockSTT) Recognize(ctx context.Context, audio *media.AudioFrame) (*stt.Recognition, error) {
	// Simulate processing delay
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Get next response (cycling through available responses)
	text := m.responses[m.responseIndex%len(m.responses)]
	m.responseIndex++

	return &stt.Recognition{
		Text:       text,
		Confidence: m.confidence,
		Language:   "en-US",
		IsFinal:    true,
		Metadata: map[string]interface{}{
			"mock":        true,
			"audio_size":  len(audio.Data),
			"duration_ms": audio.Duration.Milliseconds(),
		},
	}, nil
}

// RecognizeStream implements stt.STT
func (m *MockSTT) RecognizeStream(ctx context.Context) (stt.RecognitionStream, error) {
	return NewMockRecognitionStream(m), nil
}

// MockRecognitionStream implements stt.RecognitionStream
type MockRecognitionStream struct {
	stt      *MockSTT
	audioCh  chan *media.AudioFrame
	resultCh chan *stt.Recognition
	closed   bool
	sendClosed bool
	mu       sync.Mutex
}

// NewMockRecognitionStream creates a new mock recognition stream
func NewMockRecognitionStream(mockSTT *MockSTT) *MockRecognitionStream {
	stream := &MockRecognitionStream{
		stt:      mockSTT,
		audioCh:  make(chan *media.AudioFrame, 10),
		resultCh: make(chan *stt.Recognition, 10),
		closed:   false,
	}

	// Start processing goroutine
	go stream.processAudio()

	return stream
}

// SendAudio implements stt.RecognitionStream
func (s *MockRecognitionStream) SendAudio(audio *media.AudioFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed || s.sendClosed {
		return fmt.Errorf("stream is closed")
	}

	select {
	case s.audioCh <- audio:
		return nil
	default:
		return fmt.Errorf("audio buffer full")
	}
}

// Recv implements stt.RecognitionStream
func (s *MockRecognitionStream) Recv() (*stt.Recognition, error) {
	if s.closed {
		return nil, fmt.Errorf("stream is closed")
	}

	result, ok := <-s.resultCh
	if !ok {
		return nil, fmt.Errorf("stream is closed")
	}

	return result, nil
}

// Close implements stt.RecognitionStream
func (s *MockRecognitionStream) Close() error {
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

// CloseSend implements stt.RecognitionStream
func (s *MockRecognitionStream) CloseSend() error {
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

// processAudio processes incoming audio and generates mock recognition results
func (s *MockRecognitionStream) processAudio() {
	defer close(s.resultCh)

	var audioBuffer []*media.AudioFrame
	bufferDuration := time.Duration(0)

	for audio := range s.audioCh {
		audioBuffer = append(audioBuffer, audio)
		bufferDuration += audio.Duration

		// Generate recognition immediately for testing (shorter threshold)
		if bufferDuration >= 100*time.Millisecond || len(audioBuffer) >= 1 {
			// Simulate processing delay
			time.Sleep(s.stt.delay)

			// Generate recognition result
			recognition := &stt.Recognition{
				Text:       s.stt.responses[s.stt.responseIndex%len(s.stt.responses)],
				Confidence: s.stt.confidence,
				Language:   "en-US",
				IsFinal:    true,
				Metadata: map[string]interface{}{
					"mock":          true,
					"frames_count":  len(audioBuffer),
					"total_duration": bufferDuration.Milliseconds(),
				},
			}
			s.stt.responseIndex++

			// Send result
			select {
			case s.resultCh <- recognition:
			default:
				// Result buffer full, skip
			}

			// Reset buffer
			audioBuffer = audioBuffer[:0]
			bufferDuration = 0
		}
	}

	// Process any remaining audio
	if len(audioBuffer) > 0 {
		time.Sleep(s.stt.delay)

		recognition := &stt.Recognition{
			Text:       s.stt.responses[s.stt.responseIndex%len(s.stt.responses)],
			Confidence: s.stt.confidence,
			Language:   "en-US",
			IsFinal:    true,
			Metadata: map[string]interface{}{
				"mock":          true,
				"frames_count":  len(audioBuffer),
				"total_duration": bufferDuration.Milliseconds(),
				"final_chunk":   true,
			},
		}

		select {
		case s.resultCh <- recognition:
		default:
			// Result buffer full, skip
		}
	}
}