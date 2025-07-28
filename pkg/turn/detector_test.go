package turn

import (
	"context"
	"fmt"
	"testing"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

// StubDetector is a simple test implementation.
type StubDetector struct {
	probability float64
	threshold   float64
	supported   bool
}

func (s *StubDetector) UnlikelyThreshold(language string) (float64, error) {
	if !s.supported {
		return 0, ErrUnsupportedLanguage
	}
	return s.threshold, nil
}

func (s *StubDetector) SupportsLanguage(language string) bool {
	return s.supported
}

func (s *StubDetector) PredictEndOfTurn(ctx context.Context, chatCtx ChatContext) (float64, error) {
	return s.probability, nil
}

var ErrUnsupportedLanguage = fmt.Errorf("unsupported language")

func TestDetectorInterface(t *testing.T) {
	stub := &StubDetector{
		probability: 0.95,
		threshold:   0.85,
		supported:   true,
	}

	// Test supports language
	if !stub.SupportsLanguage("en-US") {
		t.Error("Expected language to be supported")
	}

	// Test threshold
	threshold, err := stub.UnlikelyThreshold("en-US")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if threshold != 0.85 {
		t.Errorf("Expected threshold 0.85, got %f", threshold)
	}

	// Test prediction
	ctx := context.Background()
	chatCtx := ChatContext{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
		Language: "en-US",
	}

	probability, err := stub.PredictEndOfTurn(ctx, chatCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if probability != 0.95 {
		t.Errorf("Expected probability 0.95, got %f", probability)
	}
}

func TestUnsupportedLanguage(t *testing.T) {
	stub := &StubDetector{
		probability: 0.95,
		threshold:   0.85,
		supported:   false,
	}

	// Test unsupported language
	if stub.SupportsLanguage("unsupported") {
		t.Error("Expected language to be unsupported")
	}

	_, err := stub.UnlikelyThreshold("unsupported")
	if err == nil {
		t.Error("Expected error for unsupported language")
	}
}