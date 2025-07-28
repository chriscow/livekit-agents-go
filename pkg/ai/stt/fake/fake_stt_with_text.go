package fake

import (
	"context"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// FakeSTTWithText generates actual speech events with text content for testing.
type FakeSTTWithText struct {
	sampleResponses []string
	responseIndex   int
}

// NewFakeSTTWithText creates a fake STT that generates speech events with actual text.
func NewFakeSTTWithText() *FakeSTTWithText {
	return &FakeSTTWithText{
		sampleResponses: []string{
			"Hello there!",
			"How are you doing today?",
			"This is a test of the echo bot functionality.",
			"Can you repeat what I just said?",
			"The weather is nice today.",
			"I'm testing the voice agent pipeline.",
		},
		responseIndex: 0,
	}
}

// NewStream creates a new fake STT stream.
func (f *FakeSTTWithText) NewStream(ctx context.Context, cfg stt.StreamConfig) (stt.STTStream, error) {
	return &FakeSTTStreamWithText{
		parent:  f,
		events:  make(chan stt.SpeechEvent, 10),
		context: ctx,
	}, nil
}

// Capabilities returns fake STT capabilities.
func (f *FakeSTTWithText) Capabilities() stt.STTCapabilities {
	return stt.STTCapabilities{
		Streaming:          true,
		InterimResults:     true,
		SupportedLanguages: []string{"en-US"},
		SampleRates:        []int{16000, 48000},
	}
}

// FakeSTTStreamWithText implements stt.STTStream with text generation.
type FakeSTTStreamWithText struct {
	parent           *FakeSTTWithText
	events           chan stt.SpeechEvent
	context          context.Context
	frameCount       int
	hasGeneratedText bool
}

// Push processes audio frames and generates speech events with text.
func (s *FakeSTTStreamWithText) Push(frame rtc.AudioFrame) error {
	s.frameCount++
	
	// Generate a speech event after receiving some audio frames (simulate processing delay)
	if s.frameCount == 50 && !s.hasGeneratedText { // After ~1 second of audio
		s.hasGeneratedText = true
		
		// Send interim result first
		select {
		case s.events <- stt.SpeechEvent{
			Type:      stt.SpeechEventInterim,
			Text:      s.getCurrentResponse()[:5] + "...", // Partial text
			IsFinal:   false,
			Language:  "en-US",
			Timestamp: time.Now().UnixMilli(),
		}:
		case <-s.context.Done():
			return s.context.Err()
		}
		
		// Send final result after a short delay
		go func() {
			time.Sleep(200 * time.Millisecond)
			select {
			case s.events <- stt.SpeechEvent{
				Type:      stt.SpeechEventFinal,
				Text:      s.getCurrentResponse(),
				IsFinal:   true,
				Language:  "en-US",
				Timestamp: time.Now().UnixMilli(),
			}:
			case <-s.context.Done():
			}
		}()
	}
	
	return nil
}

// Events returns the speech events channel.
func (s *FakeSTTStreamWithText) Events() <-chan stt.SpeechEvent {
	return s.events
}

// CloseSend closes the STT stream.
func (s *FakeSTTStreamWithText) CloseSend() error {
	close(s.events)
	return nil
}

// getCurrentResponse gets the current response text and cycles to the next.
func (s *FakeSTTStreamWithText) getCurrentResponse() string {
	response := s.parent.sampleResponses[s.parent.responseIndex]
	s.parent.responseIndex = (s.parent.responseIndex + 1) % len(s.parent.sampleResponses)
	return response
}