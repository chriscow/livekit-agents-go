package openai

import (
	"fmt"
	"os"

	"github.com/chriscow/livekit-agents-go/pkg/plugin"
)

// newOpenAISTT is the factory function for OpenAI STT.
func newOpenAISTT(cfg map[string]any) (any, error) {
	config := Config{}

	// Get API key from config or environment
	if apiKey, ok := cfg["api_key"].(string); ok {
		config.APIKey = apiKey
	} else {
		config.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required (set OPENAI_API_KEY environment variable or provide api_key in config)")
	}

	if model, ok := cfg["model"].(string); ok {
		config.Model = model
	}

	if language, ok := cfg["language"].(string); ok {
		config.Language = language
	}

	return NewWhisperSTT(config)
}

func init() {
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "stt",
		Name:        "openai",
		Factory:     newOpenAISTT,
		Description: "OpenAI Whisper speech-to-text service",
		Version:     "1.0.0",
		Config: map[string]any{
			"api_key":  "OpenAI API key (or set OPENAI_API_KEY env var)",
			"model":    "whisper-1",
			"language": "auto-detect (leave empty) or specify language code",
		},
	})
	
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "llm",
		Name:        "openai",
		Factory:     newOpenAILLM,
		Description: "OpenAI GPT chat completion service",
		Version:     "1.0.0",
		Config: map[string]any{
			"api_key": "OpenAI API key (or set OPENAI_API_KEY env var)",
			"model":   "gpt-3.5-turbo",
		},
	})
	
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "tts",
		Name:        "openai",
		Factory:     newOpenAITTS,
		Description: "OpenAI text-to-speech service",
		Version:     "1.0.0",
		Config: map[string]any{
			"api_key": "OpenAI API key (or set OPENAI_API_KEY env var)",
			"model":   "tts-1",
			"voice":   "alloy",
		},
	})
}