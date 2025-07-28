//go:build integration
// +build integration

package turn

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/matryer/is"
)

// TestPredictEndOfTurnIntegration verifies the real English model on disk.
func TestPredictEndOfTurnIntegration(t *testing.T) {
	is := is.New(t)

	// Create the local ONNX detector for the English model
	detector, err := NewONNXDetector("english", "")
	is.NoErr(err) // must create detector without error

	// Prepare a simple chat context
	chatCtx := ChatContext{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello, how are you?"}},
		Language: "en-US",
	}

	// Run inference with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prob, err := detector.PredictEndOfTurn(ctx, chatCtx)
	if err != nil {
		t.Skipf("Skipping integration test, ONNX runtime not available: %v", err)
	}
	is.True(prob >= 0 && prob <= 1) // probability in range
}
