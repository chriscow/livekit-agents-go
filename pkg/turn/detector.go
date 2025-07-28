package turn

import (
	"context"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

// Detector interface for end-of-utterance (EOU) detection.
// Provides language-aware turn detection that matches the accuracy
// of the Python turn_detector.multilingual plugin.
type Detector interface {
	// UnlikelyThreshold returns the language-specific threshold for EOU detection.
	// Returns the threshold value (0-1) or an error if language is unsupported.
	UnlikelyThreshold(language string) (float64, error)

	// SupportsLanguage returns true if the detector has a tuned threshold for this language.
	SupportsLanguage(language string) bool

	// PredictEndOfTurn returns probability (0–1) that the user has finished speaking
	// given recent chat context. Higher values indicate higher likelihood of turn completion.
	PredictEndOfTurn(ctx context.Context, chatCtx ChatContext) (float64, error)
}

// ChatContext represents the conversation history needed for turn detection.
// This extends the base LLM chat context with turn detection specific data.
type ChatContext struct {
	Messages []llm.Message
	Language string // Language hint for detection optimization
}