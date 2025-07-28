package plugin_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/chriscow/livekit-agents-go/pkg/ai/vad"
	"github.com/chriscow/livekit-agents-go/pkg/plugin"
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/fake"   // Register fake plugins
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/openai" // Register OpenAI plugin
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/silero" // Register silero plugin
)

func TestPluginIntegration_FakeSTT(t *testing.T) {
	// Get the fake STT plugin
	factory, exists := plugin.Get("stt", "fake")
	if !exists {
		t.Fatal("Fake STT plugin not found")
	}

	// Create an instance
	cfg := map[string]any{
		"transcript": "Integration test transcript",
	}
	
	instance, err := factory(cfg)
	if err != nil {
		t.Fatalf("Failed to create STT instance: %v", err)
	}

	// Verify it implements STT interface
	sttInstance, ok := instance.(stt.STT)
	if !ok {
		t.Fatal("Plugin instance does not implement STT interface")
	}

	// Test capabilities
	caps := sttInstance.Capabilities()
	if !caps.Streaming {
		t.Error("Expected fake STT to support streaming")
	}

	// Test stream creation
	ctx := context.Background()
	stream, err := sttInstance.NewStream(ctx, stt.StreamConfig{
		SampleRate:  16000,
		NumChannels: 1,
		Lang:        "en-US",
		MaxRetry:    3,
	})
	if err != nil {
		t.Fatalf("Failed to create STT stream: %v", err)
	}

	if stream == nil {
		t.Error("STT stream should not be nil")
	}
}

func TestPluginIntegration_FakeTTS(t *testing.T) {
	// Get the fake TTS plugin
	factory, exists := plugin.Get("tts", "fake")
	if !exists {
		t.Fatal("Fake TTS plugin not found")
	}

	// Create an instance
	instance, err := factory(map[string]any{})
	if err != nil {
		t.Fatalf("Failed to create TTS instance: %v", err)
	}

	// Verify it implements TTS interface
	ttsInstance, ok := instance.(tts.TTS)
	if !ok {
		t.Fatal("Plugin instance does not implement TTS interface")
	}

	// Test capabilities
	caps := ttsInstance.Capabilities()
	if len(caps.SupportedLanguages) == 0 {
		t.Error("Expected fake TTS to have some supported languages")
	}
}

func TestPluginIntegration_FakeLLM(t *testing.T) {
	// Get the fake LLM plugin
	factory, exists := plugin.Get("llm", "fake")
	if !exists {
		t.Fatal("Fake LLM plugin not found")
	}

	// Create an instance with custom responses
	cfg := map[string]any{
		"responses": []string{"Test response 1", "Test response 2"},
	}
	
	instance, err := factory(cfg)
	if err != nil {
		t.Fatalf("Failed to create LLM instance: %v", err)
	}

	// Verify it implements LLM interface
	llmInstance, ok := instance.(llm.LLM)
	if !ok {
		t.Fatal("Plugin instance does not implement LLM interface")
	}

	// Test capabilities
	caps := llmInstance.Capabilities()
	if caps.SupportsStreaming {
		t.Error("Expected fake LLM to NOT support streaming (it's a fake)")
	}
	if !caps.SupportsFunctions {
		t.Error("Expected fake LLM to support functions")
	}
}

func TestPluginIntegration_FakeVAD(t *testing.T) {
	// Get the fake VAD plugin
	factory, exists := plugin.Get("vad", "fake")
	if !exists {
		t.Fatal("Fake VAD plugin not found")
	}

	// Create an instance with custom threshold
	cfg := map[string]any{
		"threshold": 0.7,
	}
	
	instance, err := factory(cfg)
	if err != nil {
		t.Fatalf("Failed to create VAD instance: %v", err)
	}

	// Verify it implements VAD interface
	vadInstance, ok := instance.(vad.VAD)
	if !ok {
		t.Fatal("Plugin instance does not implement VAD interface")
	}

	// Test capabilities
	caps := vadInstance.Capabilities()
	if caps.Sensitivity != 0.7 {
		t.Errorf("Expected sensitivity 0.7, got %f", caps.Sensitivity)
	}
}

func TestPluginIntegration_SileroVADStub(t *testing.T) {
	// Get the silero VAD plugin (stub version)
	factory, exists := plugin.Get("vad", "silero")
	if !exists {
		t.Fatal("Silero VAD plugin not found")
	}

	// Try to create an instance - should fail because we're not using the silero build tag
	_, err := factory(map[string]any{})
	if err == nil {
		t.Error("Expected error when creating silero VAD without build tag, but got none")
	}

	expectedErrMsg := "silero VAD plugin not available"
	if err != nil && !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedErrMsg, err)
	}
}

func TestPluginIntegration_OpenAISTT(t *testing.T) {
	// Get the OpenAI STT plugin
	factory, exists := plugin.Get("stt", "openai")
	if !exists {
		t.Fatal("OpenAI STT plugin not found")
	}

	// Try to create an instance without API key - should fail
	_, err := factory(map[string]any{})
	if err == nil {
		t.Error("Expected error when creating OpenAI STT without API key")
	}

	expectedErrMsg := "OpenAI API key is required"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedErrMsg, err)
	}

	// Create an instance with API key
	cfg := map[string]any{
		"api_key": "test-key",
		"model":   "whisper-1",
	}
	
	instance, err := factory(cfg)
	if err != nil {
		t.Fatalf("Failed to create OpenAI STT instance: %v", err)
	}

	// Verify it implements STT interface
	sttInstance, ok := instance.(stt.STT)
	if !ok {
		t.Fatal("Plugin instance does not implement STT interface")
	}

	// Test capabilities
	caps := sttInstance.Capabilities()
	if !caps.Streaming {
		t.Error("Expected OpenAI STT to support streaming")
	}

	if caps.InterimResults {
		t.Error("Expected OpenAI STT to NOT support interim results")
	}

	if len(caps.SupportedLanguages) == 0 {
		t.Error("Expected OpenAI STT to have supported languages")
	}
}

func TestPluginIntegration_PluginListing(t *testing.T) {
	// Test listing all plugins
	allPlugins := plugin.List("")
	if len(allPlugins) < 6 {
		t.Errorf("Expected at least 6 plugins (4 fake + 1 openai + 1 silero stub), got %d", len(allPlugins))
	}

	// Test listing by kind
	vadPlugins := plugin.List("vad")
	if len(vadPlugins) != 2 {
		t.Errorf("Expected 2 VAD plugins (fake + silero), got %d", len(vadPlugins))
	}

	// Verify plugin names
	vadNames := make(map[string]bool)
	for _, p := range vadPlugins {
		vadNames[p.Name] = true
	}
	
	if !vadNames["fake"] {
		t.Error("Expected to find fake VAD plugin")
	}
	if !vadNames["silero"] {
		t.Error("Expected to find silero VAD plugin")
	}

	// Test listing non-existent kind
	nonExistent := plugin.List("nonexistent")
	if len(nonExistent) != 0 {
		t.Errorf("Expected 0 plugins for non-existent kind, got %d", len(nonExistent))
	}
}