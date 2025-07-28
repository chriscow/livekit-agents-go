package fake

import (
	"context"

	"github.com/chriscow/livekit-agents-go/pkg/turn"
)

// FakeTurnDetector is a simple fake implementation for testing.
type FakeTurnDetector struct {
	probability float64
	threshold   float64
}

// NewFakeTurnDetector creates a new fake turn detector.
func NewFakeTurnDetector() *FakeTurnDetector {
	return &FakeTurnDetector{
		probability: 0.85,
		threshold:   0.85,
	}
}

// NewFakeTurnDetectorWithValues creates a fake detector with specific values.
func NewFakeTurnDetectorWithValues(probability, threshold float64) *FakeTurnDetector {
	return &FakeTurnDetector{
		probability: probability,
		threshold:   threshold,
	}
}

// UnlikelyThreshold returns the configured threshold.
func (f *FakeTurnDetector) UnlikelyThreshold(language string) (float64, error) {
	return f.threshold, nil
}

// SupportsLanguage always returns true for testing.
func (f *FakeTurnDetector) SupportsLanguage(language string) bool {
	return true
}

// PredictEndOfTurn returns the configured probability.
func (f *FakeTurnDetector) PredictEndOfTurn(ctx context.Context, chatCtx turn.ChatContext) (float64, error) {
	return f.probability, nil
}