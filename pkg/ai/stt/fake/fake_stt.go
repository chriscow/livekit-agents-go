package fake

import (
	"context"
	"fmt"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

const (
	// InterimResultFrameInterval controls how often interim results are sent
	InterimResultFrameInterval = 10
	// DefaultTranscript is used when no transcript is provided
	DefaultTranscript = "This is a fake transcript from the fake STT provider."
)

// FakeSTT is a fake STT implementation for testing.
type FakeSTT struct {
	transcript string
}

// NewFakeSTT creates a new fake STT provider with a fixed transcript.
func NewFakeSTT(transcript string) *FakeSTT {
	if transcript == "" {
		transcript = DefaultTranscript
	}
	return &FakeSTT{transcript: transcript}
}

// NewStream creates a new fake STT stream.
func (f *FakeSTT) NewStream(ctx context.Context, cfg stt.StreamConfig) (stt.STTStream, error) {
	return &FakeSTTStream{
		transcript: f.transcript,
		events:     make(chan stt.SpeechEvent, 10),
		ctx:        ctx,
	}, nil
}

// Capabilities returns the fake STT capabilities.
func (f *FakeSTT) Capabilities() stt.STTCapabilities {
	return stt.STTCapabilities{
		Streaming:          true,
		InterimResults:     true,
		SupportedLanguages: []string{"en-US", "en-GB", "es-ES"},
		SampleRates:        []int{16000, 48000},
	}
}

// FakeSTTStream is a fake STT stream implementation.
type FakeSTTStream struct {
	transcript  string
	events      chan stt.SpeechEvent
	ctx         context.Context
	frameCount  int
	closed      bool
}

// Push processes an audio frame (fake implementation just counts frames).
func (s *FakeSTTStream) Push(frame rtc.AudioFrame) error {
	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	s.frameCount++
	
	// Send interim result every InterimResultFrameInterval frames
	if s.frameCount%InterimResultFrameInterval == 0 {
		select {
		case s.events <- stt.SpeechEvent{
			Type:      stt.SpeechEventInterim,
			Text:      s.transcript[:min(len(s.transcript), s.frameCount/2)],
			IsFinal:   false,
			Language:  "en-US",
			Timestamp: time.Now().UnixMilli(),
		}:
		case <-s.ctx.Done():
			return s.ctx.Err()
		}
	}

	return nil
}

// Events returns the events channel.
func (s *FakeSTTStream) Events() <-chan stt.SpeechEvent {
	return s.events
}

// CloseSend closes the stream and sends final result.
func (s *FakeSTTStream) CloseSend() error {
	if s.closed {
		return nil
	}
	
	s.closed = true
	
	// Send final result
	select {
	case s.events <- stt.SpeechEvent{
		Type:      stt.SpeechEventFinal,
		Text:      s.transcript,
		IsFinal:   true,
		Language:  "en-US",
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-s.ctx.Done():
		close(s.events)
		return s.ctx.Err()
	}
	
	close(s.events)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}