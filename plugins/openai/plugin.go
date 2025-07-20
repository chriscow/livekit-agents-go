package openai

import (
	"fmt"
	"log"

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
	if p.apiKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}

	log.Printf("🔌 Registering OpenAI plugin services...")

	// Register STT service (Whisper)
	registry.RegisterSTT("whisper", func() stt.STT {
		log.Printf("🎙️ Creating Whisper STT service")
		return NewWhisperSTT(p.apiKey)
	})

	// Register TTS service
	registry.RegisterTTS("openai-tts", func() tts.TTS {
		log.Printf("🔊 Creating OpenAI TTS service")
		return NewOpenAITTS(p.apiKey)
	})

	// Register LLM services
	llmModels := map[string]string{
		"gpt-4":         "gpt-4",
		"gpt-4-turbo":   "gpt-4-turbo",
		"gpt-4o":        "gpt-4o",
		"gpt-4o-mini":   "gpt-4o-mini",
		"gpt-3.5-turbo": "gpt-3.5-turbo",
	}

	for serviceName, modelName := range llmModels {
		// Capture variables for closure
		svcName := serviceName
		mdlName := modelName
		registry.RegisterLLM(svcName, func() llm.LLM {
			log.Printf("🤖 Creating OpenAI LLM service: %s", svcName)
			return NewGPTLLM(p.apiKey, mdlName)
		})
	}

	log.Printf("✅ OpenAI plugin registered successfully (STT: 1, TTS: 1, LLM: %d)", len(llmModels))
	return nil
}

// RegisterPlugin registers the OpenAI plugin with the global registry
func RegisterPlugin(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("OpenAI API key cannot be empty")
	}
	
	log.Printf("🚀 Registering OpenAI plugin with API key: %s...", apiKey[:8]+"...")
	plugin := NewPlugin(apiKey)
	return plugins.RegisterPlugin(plugin)
}

// Register the delegate function for auto-discovery
func init() {
	plugins.RegisterPluginDelegate("openai", RegisterPlugin)
}
