package plugins

import (
	"fmt"
	"log"
	"os"
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

// Service creation methods with fallback support
func (r *Registry) CreateSTT(name string) (stt.STT, error) {
	r.mu.RLock()
	factory, exists := r.sttServices[name]
	r.mu.RUnlock()

	if exists {
		log.Printf("🎙️ Creating STT service: %s", name)
		return factory(), nil
	}

	// Try fallback strategies
	log.Printf("⚠️ STT service '%s' not found, attempting fallbacks...", name)
	
	// 1. Try mock service as fallback
	if mockFactory, mockExists := r.sttServices["mock-stt"]; mockExists {
		log.Printf("🔄 Falling back to mock STT service")
		return mockFactory(), nil
	}
	
	// 2. Try any available STT service
	for fallbackName, fallbackFactory := range r.sttServices {
		log.Printf("🔄 Falling back to available STT service: %s", fallbackName)
		return fallbackFactory(), nil
	}

	return nil, fmt.Errorf("STT service %s not found and no fallback available", name)
}

func (r *Registry) CreateTTS(name string) (tts.TTS, error) {
	r.mu.RLock()
	factory, exists := r.ttsServices[name]
	r.mu.RUnlock()

	if exists {
		log.Printf("🔊 Creating TTS service: %s", name)
		return factory(), nil
	}

	// Try fallback strategies
	log.Printf("⚠️ TTS service '%s' not found, attempting fallbacks...", name)
	
	// 1. Try mock service as fallback
	if mockFactory, mockExists := r.ttsServices["mock-tts"]; mockExists {
		log.Printf("🔄 Falling back to mock TTS service")
		return mockFactory(), nil
	}
	
	// 2. Try any available TTS service
	for fallbackName, fallbackFactory := range r.ttsServices {
		log.Printf("🔄 Falling back to available TTS service: %s", fallbackName)
		return fallbackFactory(), nil
	}

	return nil, fmt.Errorf("TTS service %s not found and no fallback available", name)
}

func (r *Registry) CreateLLM(name string) (llm.LLM, error) {
	r.mu.RLock()
	factory, exists := r.llmServices[name]
	r.mu.RUnlock()

	if exists {
		log.Printf("🤖 Creating LLM service: %s", name)
		return factory(), nil
	}

	// Try fallback strategies
	log.Printf("⚠️ LLM service '%s' not found, attempting fallbacks...", name)
	
	// 1. Try mock service as fallback
	if mockFactory, mockExists := r.llmServices["mock-llm"]; mockExists {
		log.Printf("🔄 Falling back to mock LLM service")
		return mockFactory(), nil
	}
	
	// 2. Try any available LLM service
	for fallbackName, fallbackFactory := range r.llmServices {
		log.Printf("🔄 Falling back to available LLM service: %s", fallbackName)
		return fallbackFactory(), nil
	}

	return nil, fmt.Errorf("LLM service %s not found and no fallback available", name)
}

func (r *Registry) CreateVAD(name string) (vad.VAD, error) {
	r.mu.RLock()
	factory, exists := r.vadServices[name]
	r.mu.RUnlock()

	if exists {
		log.Printf("🎤 Creating VAD service: %s", name)
		return factory(), nil
	}

	// Try fallback strategies
	log.Printf("⚠️ VAD service '%s' not found, attempting fallbacks...", name)
	
	// 1. Try mock service as fallback
	if mockFactory, mockExists := r.vadServices["mock-vad"]; mockExists {
		log.Printf("🔄 Falling back to mock VAD service")
		return mockFactory(), nil
	}
	
	// 2. Try any available VAD service
	for fallbackName, fallbackFactory := range r.vadServices {
		log.Printf("🔄 Falling back to available VAD service: %s", fallbackName)
		return fallbackFactory(), nil
	}

	return nil, fmt.Errorf("VAD service %s not found and no fallback available", name)
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

// Auto-discovery and registration functions

// AutoDiscoverPlugins automatically discovers and registers available plugins
func AutoDiscoverPlugins() error {
	log.Printf("🔍 Auto-discovering plugins based on environment variables...")
	
	pluginsFound := 0
	
	// OpenAI Plugin
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		log.Printf("🔑 Found OpenAI API key, registering OpenAI plugin")
		if err := registerOpenAIPlugin(apiKey); err != nil {
			log.Printf("❌ Failed to register OpenAI plugin: %v", err)
		} else {
			pluginsFound++
		}
	}
	
	// Anthropic Plugin
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		log.Printf("🔑 Found Anthropic API key, registering Anthropic plugin")
		if err := registerAnthropicPlugin(apiKey); err != nil {
			log.Printf("❌ Failed to register Anthropic plugin: %v", err)
		} else {
			pluginsFound++
		}
	}
	
	// Deepgram Plugin
	if apiKey := os.Getenv("DEEPGRAM_API_KEY"); apiKey != "" {
		log.Printf("🔑 Found Deepgram API key, registering Deepgram plugin")
		if err := registerDeepgramPlugin(apiKey); err != nil {
			log.Printf("❌ Failed to register Deepgram plugin: %v", err)
		} else {
			pluginsFound++
		}
	}
	
	// Always register mock plugin as fallback
	if err := registerMockPlugin(); err != nil {
		log.Printf("❌ Failed to register mock plugin: %v", err)
	} else {
		pluginsFound++
		log.Printf("🎭 Mock plugin registered as fallback")
	}
	
	log.Printf("✅ Auto-discovery complete: %d plugins registered", pluginsFound)
	return nil
}

// Helper functions for plugin registration
func registerOpenAIPlugin(apiKey string) error {
	// Use a function to avoid import cycles - will be resolved at runtime
	// This relies on the OpenAI plugin being imported by the calling code
	return callPluginRegister("openai", apiKey)
}

func registerAnthropicPlugin(apiKey string) error {
	return callPluginRegister("anthropic", apiKey)
}

func registerDeepgramPlugin(apiKey string) error {
	return callPluginRegister("deepgram", apiKey)
}

func registerMockPlugin() error {
	return callPluginRegister("mock", "")
}

// Plugin registration delegate - will be set by plugins when they're imported
var pluginRegisters = make(map[string]func(string) error)

// RegisterPluginDelegate allows plugins to register their registration functions
func RegisterPluginDelegate(name string, registerFunc func(string) error) {
	pluginRegisters[name] = registerFunc
}

// callPluginRegister calls the appropriate plugin registration function
func callPluginRegister(pluginName, apiKey string) error {
	if registerFunc, exists := pluginRegisters[pluginName]; exists {
		return registerFunc(apiKey)
	}
	return fmt.Errorf("plugin %s registration delegate not found", pluginName)
}

// GetRecommendedService returns the best available service for the given type
func GetRecommendedService(serviceType string) (string, error) {
	registry := GlobalRegistry()
	
	switch serviceType {
	case "stt":
		services := registry.ListSTTServices()
		return selectBestService(services, []string{"deepgram", "whisper", "mock-stt"}), nil
	case "tts":
		services := registry.ListTTSServices()
		return selectBestService(services, []string{"openai-tts", "mock-tts"}), nil
	case "llm":
		services := registry.ListLLMServices()
		return selectBestService(services, []string{"gpt-4o", "gpt-4", "gpt-4-turbo", "mock-llm"}), nil
	case "vad":
		services := registry.ListVADServices()
		return selectBestService(services, []string{"silero", "mock-vad"}), nil
	default:
		return "", fmt.Errorf("unknown service type: %s", serviceType)
	}
}

// selectBestService selects the best available service from preferences
func selectBestService(available []string, preferences []string) string {
	// Create a map for quick lookup
	availableMap := make(map[string]bool)
	for _, service := range available {
		availableMap[service] = true
	}
	
	// Find the first preferred service that's available
	for _, preferred := range preferences {
		if availableMap[preferred] {
			return preferred
		}
	}
	
	// If no preferred service is available, return the first available
	if len(available) > 0 {
		return available[0]
	}
	
	return ""
}

// PrintRegistryStatus prints the current state of the plugin registry
func PrintRegistryStatus() {
	registry := GlobalRegistry()
	
	log.Printf("🔌 Plugin Registry Status:")
	log.Printf("  Plugins: %d", len(registry.ListPlugins()))
	log.Printf("  STT Services: %v", registry.ListSTTServices())
	log.Printf("  TTS Services: %v", registry.ListTTSServices())
	log.Printf("  LLM Services: %v", registry.ListLLMServices())
	log.Printf("  VAD Services: %v", registry.ListVADServices())
}

// CreateSmartServices creates the best available services automatically
func CreateSmartServices() (*SmartServices, error) {
	// Auto-discover plugins first
	if err := AutoDiscoverPlugins(); err != nil {
		log.Printf("⚠️ Auto-discovery failed: %v", err)
	}
	
	services := &SmartServices{}
	var err error
	
	// Create STT service
	sttName, _ := GetRecommendedService("stt")
	if sttName != "" {
		services.STT, err = CreateSTT(sttName)
		if err != nil {
			log.Printf("❌ Failed to create STT service %s: %v", sttName, err)
		}
	}
	
	// Create TTS service
	ttsName, _ := GetRecommendedService("tts")
	if ttsName != "" {
		services.TTS, err = CreateTTS(ttsName)
		if err != nil {
			log.Printf("❌ Failed to create TTS service %s: %v", ttsName, err)
		}
	}
	
	// Create LLM service
	llmName, _ := GetRecommendedService("llm")
	if llmName != "" {
		services.LLM, err = CreateLLM(llmName)
		if err != nil {
			log.Printf("❌ Failed to create LLM service %s: %v", llmName, err)
		}
	}
	
	// Create VAD service
	vadName, _ := GetRecommendedService("vad")
	if vadName != "" {
		services.VAD, err = CreateVAD(vadName)
		if err != nil {
			log.Printf("❌ Failed to create VAD service %s: %v", vadName, err)
		}
	}
	
	log.Printf("✅ Smart services created:")
	if services.STT != nil {
		log.Printf("  STT: %s v%s", services.STT.Name(), services.STT.Version())
	}
	if services.TTS != nil {
		log.Printf("  TTS: %s v%s", services.TTS.Name(), services.TTS.Version())
	}
	if services.LLM != nil {
		log.Printf("  LLM: %s v%s", services.LLM.Name(), services.LLM.Version())
	}
	if services.VAD != nil {
		log.Printf("  VAD: %s v%s", services.VAD.Name(), services.VAD.Version())
	}
	
	return services, nil
}

// SmartServices holds all the core AI services
type SmartServices struct {
	STT stt.STT
	TTS tts.TTS
	LLM llm.LLM
	VAD vad.VAD
}

// CreateServicesWithPreferences creates services with custom preferences
func CreateServicesWithPreferences(preferences ServicePreferences) (*SmartServices, error) {
	// Auto-discover plugins first
	if err := AutoDiscoverPlugins(); err != nil {
		log.Printf("⚠️ Auto-discovery failed: %v", err)
	}
	
	services := &SmartServices{}
	var err error
	
	// Create services with user preferences
	if preferences.STT != "" {
		services.STT, err = CreateSTT(preferences.STT)
		if err != nil {
			log.Printf("❌ Failed to create preferred STT service %s: %v", preferences.STT, err)
			// Fallback to recommended
			if sttName, _ := GetRecommendedService("stt"); sttName != "" {
				services.STT, _ = CreateSTT(sttName)
			}
		}
	} else {
		if sttName, _ := GetRecommendedService("stt"); sttName != "" {
			services.STT, _ = CreateSTT(sttName)
		}
	}
	
	if preferences.TTS != "" {
		services.TTS, err = CreateTTS(preferences.TTS)
		if err != nil {
			log.Printf("❌ Failed to create preferred TTS service %s: %v", preferences.TTS, err)
			if ttsName, _ := GetRecommendedService("tts"); ttsName != "" {
				services.TTS, _ = CreateTTS(ttsName)
			}
		}
	} else {
		if ttsName, _ := GetRecommendedService("tts"); ttsName != "" {
			services.TTS, _ = CreateTTS(ttsName)
		}
	}
	
	if preferences.LLM != "" {
		services.LLM, err = CreateLLM(preferences.LLM)
		if err != nil {
			log.Printf("❌ Failed to create preferred LLM service %s: %v", preferences.LLM, err)
			if llmName, _ := GetRecommendedService("llm"); llmName != "" {
				services.LLM, _ = CreateLLM(llmName)
			}
		}
	} else {
		if llmName, _ := GetRecommendedService("llm"); llmName != "" {
			services.LLM, _ = CreateLLM(llmName)
		}
	}
	
	if preferences.VAD != "" {
		services.VAD, err = CreateVAD(preferences.VAD)
		if err != nil {
			log.Printf("❌ Failed to create preferred VAD service %s: %v", preferences.VAD, err)
			if vadName, _ := GetRecommendedService("vad"); vadName != "" {
				services.VAD, _ = CreateVAD(vadName)
			}
		}
	} else {
		if vadName, _ := GetRecommendedService("vad"); vadName != "" {
			services.VAD, _ = CreateVAD(vadName)
		}
	}
	
	return services, nil
}

// ServicePreferences allows customizing service selection
type ServicePreferences struct {
	STT string
	TTS string
	LLM string
	VAD string
}

// GetServicePreferencesFromEnv reads service preferences from environment variables
func GetServicePreferencesFromEnv() ServicePreferences {
	return ServicePreferences{
		STT: os.Getenv("AGENTS_STT_SERVICE"),
		TTS: os.Getenv("AGENTS_TTS_SERVICE"),
		LLM: os.Getenv("AGENTS_LLM_SERVICE"),
		VAD: os.Getenv("AGENTS_VAD_SERVICE"),
	}
}
