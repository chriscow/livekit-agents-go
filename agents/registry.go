package agents

import (
	"livekit-agents-go/plugins"
)

// Registry is an alias for the plugin registry in the agents package
type Registry = plugins.Registry

// GlobalRegistry returns the global plugin registry
func GlobalRegistry() *Registry {
	return plugins.GlobalRegistry()
}
