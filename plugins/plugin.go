package plugins

import (
	"fmt"
	"sync"

	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/services/vad"
)

// Plugin interface (equivalent to Python Plugin base class)
type Plugin interface {
	// Plugin metadata
	Name() string
	Version() string
	Description() string

	// Register plugin services
	Register(registry *Registry) error

	// Initialize plugin with configuration
	Initialize(config map[string]interface{}) error

	// Cleanup plugin resources
	Cleanup() error
}

// Registry manages plugin registration and service creation
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin

	// Service registries
	sttServices map[string]func() stt.STT
	ttsServices map[string]func() tts.TTS
	llmServices map[string]func() llm.LLM
	vadServices map[string]func() vad.VAD
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{
		plugins:     make(map[string]Plugin),
		sttServices: make(map[string]func() stt.STT),
		ttsServices: make(map[string]func() tts.TTS),
		llmServices: make(map[string]func() llm.LLM),
		vadServices: make(map[string]func() vad.VAD),
	}
}

var globalRegistry = NewRegistry()

// GlobalRegistry returns the global plugin registry
func GlobalRegistry() *Registry {
	return globalRegistry
}

// RegisterPlugin registers a plugin globally (equivalent to Python Plugin.register_plugin())
func RegisterPlugin(plugin Plugin) error {
	return globalRegistry.RegisterPlugin(plugin)
}

// RegisterPlugin registers a plugin with this registry
func (r *Registry) RegisterPlugin(plugin Plugin) error {
	r.mu.Lock()
	name := plugin.Name()
	if _, exists := r.plugins[name]; exists {
		r.mu.Unlock()
		return fmt.Errorf("plugin %s already registered", name)
	}

	r.plugins[name] = plugin
	r.mu.Unlock() // Release lock before calling plugin.Register to avoid deadlock
	
	return plugin.Register(r)
}

// GetPlugin gets a plugin by name
func (r *Registry) GetPlugin(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	return plugin, exists
}

// ListPlugins returns all registered plugins
func (r *Registry) ListPlugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// Service registration methods
func (r *Registry) RegisterSTT(name string, factory func() stt.STT) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sttServices[name] = factory
}

func (r *Registry) RegisterTTS(name string, factory func() tts.TTS) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ttsServices[name] = factory
}

func (r *Registry) RegisterLLM(name string, factory func() llm.LLM) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llmServices[name] = factory
}

func (r *Registry) RegisterVAD(name string, factory func() vad.VAD) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vadServices[name] = factory
}

// Service creation methods
func (r *Registry) CreateSTT(name string) (stt.STT, error) {
	r.mu.RLock()
	factory, exists := r.sttServices[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("STT service %s not found", name)
	}

	return factory(), nil
}

func (r *Registry) CreateTTS(name string) (tts.TTS, error) {
	r.mu.RLock()
	factory, exists := r.ttsServices[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("TTS service %s not found", name)
	}

	return factory(), nil
}

func (r *Registry) CreateLLM(name string) (llm.LLM, error) {
	r.mu.RLock()
	factory, exists := r.llmServices[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("LLM service %s not found", name)
	}

	return factory(), nil
}

func (r *Registry) CreateVAD(name string) (vad.VAD, error) {
	r.mu.RLock()
	factory, exists := r.vadServices[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("VAD service %s not found", name)
	}

	return factory(), nil
}

// List available services
func (r *Registry) ListSTTServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.sttServices))
	for name := range r.sttServices {
		services = append(services, name)
	}
	return services
}

func (r *Registry) ListTTSServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.ttsServices))
	for name := range r.ttsServices {
		services = append(services, name)
	}
	return services
}

func (r *Registry) ListLLMServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.llmServices))
	for name := range r.llmServices {
		services = append(services, name)
	}
	return services
}

func (r *Registry) ListVADServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.vadServices))
	for name := range r.vadServices {
		services = append(services, name)
	}
	return services
}

// Global service creation functions
func CreateSTT(name string) (stt.STT, error) {
	return globalRegistry.CreateSTT(name)
}

func CreateTTS(name string) (tts.TTS, error) {
	return globalRegistry.CreateTTS(name)
}

func CreateLLM(name string) (llm.LLM, error) {
	return globalRegistry.CreateLLM(name)
}

func CreateVAD(name string) (vad.VAD, error) {
	return globalRegistry.CreateVAD(name)
}

// BasePlugin provides common functionality for plugin implementations
type BasePlugin struct {
	name        string
	version     string
	description string
	config      map[string]interface{}
}

// NewBasePlugin creates a new base plugin
func NewBasePlugin(name, version, description string) *BasePlugin {
	return &BasePlugin{
		name:        name,
		version:     version,
		description: description,
		config:      make(map[string]interface{}),
	}
}

func (bp *BasePlugin) Name() string {
	return bp.name
}

func (bp *BasePlugin) Version() string {
	return bp.version
}

func (bp *BasePlugin) Description() string {
	return bp.description
}

func (bp *BasePlugin) Initialize(config map[string]interface{}) error {
	bp.config = config
	return nil
}

func (bp *BasePlugin) Cleanup() error {
	return nil
}

func (bp *BasePlugin) GetConfig() map[string]interface{} {
	return bp.config
}

func (bp *BasePlugin) GetConfigValue(key string) interface{} {
	return bp.config[key]
}

func (bp *BasePlugin) SetConfigValue(key string, value interface{}) {
	bp.config[key] = value
}
