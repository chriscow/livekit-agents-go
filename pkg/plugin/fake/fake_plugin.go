// Package fake provides fake implementations of all AI provider types for testing and demonstration.
// This plugin serves as a reference implementation and helps validate the plugin system.
package fake

import (
	llmfake "github.com/chriscow/livekit-agents-go/pkg/ai/llm/fake"
	sttfake "github.com/chriscow/livekit-agents-go/pkg/ai/stt/fake"
	ttsfake "github.com/chriscow/livekit-agents-go/pkg/ai/tts/fake"
	vadfake "github.com/chriscow/livekit-agents-go/pkg/ai/vad/fake"
	"github.com/chriscow/livekit-agents-go/pkg/plugin"
)

// newFakeSTT creates a new fake STT provider from configuration.
func newFakeSTT(cfg map[string]any) (any, error) {
	transcript := "Hello, this is a fake STT transcript"
	if t, ok := cfg["transcript"].(string); ok {
		transcript = t
	}
	return sttfake.NewFakeSTT(transcript), nil
}

// newFakeTTS creates a new fake TTS provider from configuration.
func newFakeTTS(cfg map[string]any) (any, error) {
	return ttsfake.NewFakeTTS(), nil
}

// newFakeLLM creates a new fake LLM provider from configuration.
func newFakeLLM(cfg map[string]any) (any, error) {
	responses := []string{
		"This is a fake LLM response",
		"I'm a test AI assistant",
		"How can I help you today?",
	}
	
	if r, ok := cfg["responses"].([]string); ok {
		responses = r
	}
	
	return llmfake.NewFakeLLM(responses...), nil
}

// newFakeVAD creates a new fake VAD provider from configuration.
func newFakeVAD(cfg map[string]any) (any, error) {
	threshold := float32(0.5)
	if t, ok := cfg["threshold"].(float32); ok {
		threshold = t
	} else if t, ok := cfg["threshold"].(float64); ok {
		threshold = float32(t)
	}
	
	return vadfake.NewFakeVAD(threshold), nil
}

func init() {
	// Register all fake providers
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "stt",
		Name:        "fake",
		Factory:     newFakeSTT,
		Description: "Fake STT provider for testing and development",
		Version:     "1.0.0",
		Config: map[string]interface{}{
			"transcript": "Customizable transcript text",
		},
	})
	
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "tts",
		Name:        "fake",
		Factory:     newFakeTTS,
		Description: "Fake TTS provider for testing and development",
		Version:     "1.0.0",
		Config:      map[string]interface{}{},
	})
	
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "llm",
		Name:        "fake",
		Factory:     newFakeLLM,
		Description: "Fake LLM provider for testing and development",
		Version:     "1.0.0",
		Config: map[string]interface{}{
			"responses": []string{"List of predefined responses"},
		},
	})
	
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "vad",
		Name:        "fake",
		Factory:     newFakeVAD,
		Description: "Fake VAD provider for testing and development",
		Version:     "1.0.0",
		Config: map[string]interface{}{
			"threshold": 0.5,
		},
	})
}