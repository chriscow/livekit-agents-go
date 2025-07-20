package deepgram

import (
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/stt"
)

// DeepgramPlugin implements the Plugin interface for Deepgram services
type DeepgramPlugin struct {
	*plugins.BasePlugin
	apiKey string
}

// NewDeepgramPlugin creates a new Deepgram plugin
func NewDeepgramPlugin(apiKey string) *DeepgramPlugin {
	return &DeepgramPlugin{
		BasePlugin: plugins.NewBasePlugin("deepgram", "1.0.0", "Deepgram Speech-to-Text streaming service"),
		apiKey:     apiKey,
	}
}

// Register registers the Deepgram services with the registry
func (p *DeepgramPlugin) Register(registry *plugins.Registry) error {
	// Register Deepgram STT service
	registry.RegisterSTT("deepgram", func() stt.STT {
		return NewDeepgramSTT(p.apiKey)
	})

	return nil
}

// registerDeepgramPlugin is the registration function called by the plugin system
func registerDeepgramPlugin(apiKey string) error {
	plugin := NewDeepgramPlugin(apiKey)
	return plugins.RegisterPlugin(plugin)
}

// init registers the plugin delegate for auto-discovery
func init() {
	plugins.RegisterPluginDelegate("deepgram", registerDeepgramPlugin)
}