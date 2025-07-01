package openai

import (
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"
)

// Plugin implements the OpenAI plugin
type Plugin struct {
	*plugins.BasePlugin
	apiKey string
}

// NewPlugin creates a new OpenAI plugin
func NewPlugin(apiKey string) *Plugin {
	return &Plugin{
		BasePlugin: plugins.NewBasePlugin("openai", "1.0.0", "OpenAI services (STT, TTS, LLM)"),
		apiKey:     apiKey,
	}
}

// Register registers OpenAI services with the plugin registry
func (p *Plugin) Register(registry *plugins.Registry) error {
	// Register STT service (Whisper)
	registry.RegisterSTT("whisper", func() stt.STT {
		return NewWhisperSTT(p.apiKey)
	})

	// Register TTS service
	registry.RegisterTTS("openai-tts", func() tts.TTS {
		return NewOpenAITTS(p.apiKey)
	})

	// Register LLM services
	registry.RegisterLLM("gpt-4", func() llm.LLM {
		return NewGPTLLM(p.apiKey, "gpt-4")
	})

	registry.RegisterLLM("gpt-3.5-turbo", func() llm.LLM {
		return NewGPTLLM(p.apiKey, "gpt-3.5-turbo")
	})

	registry.RegisterLLM("gpt-4-turbo", func() llm.LLM {
		return NewGPTLLM(p.apiKey, "gpt-4-turbo")
	})

	return nil
}

// RegisterPlugin registers the OpenAI plugin with the global registry
func RegisterPlugin(apiKey string) error {
	plugin := NewPlugin(apiKey)
	return plugins.RegisterPlugin(plugin)
}
