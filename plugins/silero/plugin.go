package silero

import (
	"fmt"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/vad"
)

// SileroPlugin provides the Silero VAD plugin
type SileroPlugin struct {
	*plugins.BasePlugin
}

// NewSileroPlugin creates a new Silero plugin instance
func NewSileroPlugin() *SileroPlugin {
	return &SileroPlugin{
		BasePlugin: plugins.NewBasePlugin("silero", "1.0.0", "Silero VAD plugin for advanced voice activity detection"),
	}
}

// Register registers the Silero services with the plugin registry
func (p *SileroPlugin) Register(registry *plugins.Registry) error {
	// Register VAD service
	registry.RegisterVAD("silero-vad", func() vad.VAD {
		// Load the default Silero VAD model
		vadInstance, err := LoadDefaultSileroVAD()
		if err != nil {
			// Fall back to a nil implementation or log error
			// For now, return nil - this will be handled by the registry
			return nil
		}
		return vadInstance
	})

	return nil
}

// RegisterPlugin registers the Silero plugin globally
func RegisterPlugin() error {
	return plugins.RegisterPlugin(NewSileroPlugin())
}

// init automatically registers the Silero plugin when imported
func init() {
	if err := RegisterPlugin(); err != nil {
		// Log error but don't crash - plugin may not be available
		// (e.g., ONNX model not found)
		fmt.Printf("⚠️ Silero plugin registration failed: %v\n", err)
	} else {
		fmt.Println("🔊 Silero VAD plugin registered successfully")
	}
}