// Package plugin provides a registry system for dynamically loading and managing
// AI providers (STT, TTS, LLM, VAD) without requiring changes to the core framework.
// It supports both compile-time and runtime plugin registration.
package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// Factory creates a new provider instance from configuration.
// The returned interface{} should be cast to the appropriate provider type
// (stt.STT, tts.TTS, llm.LLM, or vad.VAD).
type Factory func(cfg map[string]any) (any, error)

// Downloader interface for plugins that need to download model files.
type Downloader interface {
	Download() error
}

// Plugin represents a registered plugin with its metadata.
type Plugin struct {
	Kind        string                 // "stt", "tts", "llm", "vad"
	Name        string                 // Plugin name (e.g., "openai", "silero")
	Factory     Factory                // Factory function to create instances
	Description string                 // Human-readable description
	Version     string                 // Plugin version
	Config      map[string]any         // Configuration schema or defaults
	Downloader  Downloader             // Optional downloader for model files
}

// Registry manages plugin registration and lookup.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]map[string]*Plugin // [kind][name] -> Plugin
}

// Global registry instance
var globalRegistry = &Registry{
	plugins: make(map[string]map[string]*Plugin),
}

// Register adds a plugin to the global registry.
// This function is typically called from init() functions in plugin packages.
// Panics if a plugin with the same kind and name is already registered.
func Register(kind, name string, factory Factory) {
	globalRegistry.Register(kind, name, factory)
}

// RegisterWithMetadata adds a plugin with additional metadata to the global registry.
// Panics if a plugin with the same kind and name is already registered.
func RegisterWithMetadata(plugin *Plugin) {
	globalRegistry.RegisterWithMetadata(plugin)
}

// Get retrieves a plugin factory from the global registry.
// Returns the factory and true if found, nil and false otherwise.
func Get(kind, name string) (Factory, bool) {
	return globalRegistry.Get(kind, name)
}

// List returns all registered plugins of a specific kind.
// If kind is empty, returns all plugins.
func List(kind string) []*Plugin {
	return globalRegistry.List(kind)
}

// ListKinds returns all registered plugin kinds.
func ListKinds() []string {
	return globalRegistry.ListKinds()
}

// Register adds a plugin to this registry instance.
// Panics if a plugin with the same kind and name is already registered.
func (r *Registry) Register(kind, name string, factory Factory) {
	plugin := &Plugin{
		Kind:    kind,
		Name:    name,
		Factory: factory,
	}
	r.RegisterWithMetadata(plugin)
}

// RegisterWithMetadata adds a plugin with metadata to this registry instance.
// Panics if a plugin with the same kind and name is already registered.
func (r *Registry) RegisterWithMetadata(plugin *Plugin) {
	if plugin.Kind == "" {
		panic("plugin kind cannot be empty")
	}
	if plugin.Name == "" {
		panic("plugin name cannot be empty")
	}
	if plugin.Factory == nil {
		panic("plugin factory cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize kind map if needed
	if r.plugins[plugin.Kind] == nil {
		r.plugins[plugin.Kind] = make(map[string]*Plugin)
	}

	// Check for duplicate registration
	if existing, exists := r.plugins[plugin.Kind][plugin.Name]; exists {
		panic(fmt.Sprintf("plugin %s/%s already registered (existing version: %s, new version: %s)",
			plugin.Kind, plugin.Name, existing.Version, plugin.Version))
	}

	// Store the plugin
	r.plugins[plugin.Kind][plugin.Name] = plugin
}

// Get retrieves a plugin factory from this registry instance.
// Returns the factory and true if found, nil and false otherwise.
func (r *Registry) Get(kind, name string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	kindMap, exists := r.plugins[kind]
	if !exists {
		return nil, false
	}

	plugin, exists := kindMap[name]
	if !exists {
		return nil, false
	}

	return plugin.Factory, true
}

// List returns all registered plugins of a specific kind.
// If kind is empty, returns all plugins sorted by kind then name.
func (r *Registry) List(kind string) []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []*Plugin

	if kind == "" {
		// Return all plugins
		for _, kindMap := range r.plugins {
			for _, plugin := range kindMap {
				plugins = append(plugins, plugin)
			}
		}
	} else {
		// Return plugins of specific kind
		if kindMap, exists := r.plugins[kind]; exists {
			for _, plugin := range kindMap {
				plugins = append(plugins, plugin)
			}
		}
	}

	// Sort by kind, then by name
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Kind != plugins[j].Kind {
			return plugins[i].Kind < plugins[j].Kind
		}
		return plugins[i].Name < plugins[j].Name
	})

	return plugins
}

// ListKinds returns all registered plugin kinds in sorted order.
func (r *Registry) ListKinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	kinds := make([]string, 0, len(r.plugins))
	for kind := range r.plugins {
		kinds = append(kinds, kind)
	}

	sort.Strings(kinds)
	return kinds
}

// Clear removes all plugins from this registry instance.
// This is primarily useful for testing.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]map[string]*Plugin)
}