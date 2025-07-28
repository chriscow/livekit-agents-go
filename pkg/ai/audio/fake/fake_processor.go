package fake

import (
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/audio"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// FakeProcessor is a fake audio processor implementation for testing.
// It passes through audio frames unchanged.
type FakeProcessor struct {
	config audio.ProcessorConfig
	closed bool
}

// NewFakeProcessor creates a new fake audio processor with default configuration.
func NewFakeProcessor() *FakeProcessor {
	return &FakeProcessor{config: audio.NewProcessorConfig()}
}

// NewFakeProcessorWithConfig creates a new fake audio processor with the specified configuration.
func NewFakeProcessorWithConfig(config audio.ProcessorConfig) *FakeProcessor {
	return &FakeProcessor{config: config}
}

// ProcessReverse handles far-end (speaker output) reference.
// This fake implementation does nothing.
func (p *FakeProcessor) ProcessReverse(frame rtc.AudioFrame) error {
	if p.closed {
		return audio.ErrFatal
	}
	// No-op: fake processor doesn't modify audio
	return nil
}

// ProcessCapture handles near-end (microphone) capture.
// This fake implementation passes frames through unchanged.
func (p *FakeProcessor) ProcessCapture(frame *rtc.AudioFrame) error {
	if p.closed {
		return audio.ErrFatal
	}
	// No-op: fake processor doesn't modify audio
	return nil
}

// SetStreamDelay provides measured delay between reverse/capture paths.
// This fake implementation ignores the delay.
func (p *FakeProcessor) SetStreamDelay(d time.Duration) error {
	if p.closed {
		return audio.ErrFatal
	}
	// No-op: fake processor doesn't use delay information
	return nil
}

// Close releases resources.
func (p *FakeProcessor) Close() error {
	p.closed = true
	return nil
}