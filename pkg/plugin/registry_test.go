package plugin

import (
	"reflect"
	"testing"
)

// mockSTT is a mock STT implementation for testing
type mockSTT struct {
	name string
}

func newMockSTT(cfg map[string]any) (any, error) {
	name := "default"
	if n, ok := cfg["name"].(string); ok {
		name = n
	}
	return &mockSTT{name: name}, nil
}

func TestRegistry_Register(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	// Test successful registration
	r.Register("stt", "mock", newMockSTT)

	if factory, ok := r.Get("stt", "mock"); !ok {
		t.Error("Expected plugin to be registered")
	} else if factory == nil {
		t.Error("Expected factory to not be nil")
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	// Register first plugin
	r.Register("stt", "mock", newMockSTT)

	// Attempt to register duplicate should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for duplicate registration")
		}
	}()

	r.Register("stt", "mock", newMockSTT)
}

func TestRegistry_Register_EmptyKind(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for empty kind")
		}
	}()

	r.Register("", "mock", newMockSTT)
}

func TestRegistry_Register_EmptyName(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for empty name")
		}
	}()

	r.Register("stt", "", newMockSTT)
}

func TestRegistry_Register_NilFactory(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil factory")
		}
	}()

	r.Register("stt", "mock", nil)
}

func TestRegistry_Get(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	// Register a plugin
	r.Register("stt", "mock", newMockSTT)

	// Test successful get
	factory, ok := r.Get("stt", "mock")
	if !ok {
		t.Error("Expected to find registered plugin")
	}
	if factory == nil {
		t.Error("Expected factory to not be nil")
	}

	// Test factory functionality
	instance, err := factory(map[string]any{"name": "test"})
	if err != nil {
		t.Errorf("Factory failed: %v", err)
	}
	if mock, ok := instance.(*mockSTT); !ok {
		t.Error("Expected mockSTT instance")
	} else if mock.name != "test" {
		t.Errorf("Expected name 'test', got %s", mock.name)
	}

	// Test non-existent plugin
	_, ok = r.Get("stt", "nonexistent")
	if ok {
		t.Error("Expected to not find non-existent plugin")
	}

	// Test non-existent kind
	_, ok = r.Get("nonexistent", "mock")
	if ok {
		t.Error("Expected to not find plugin with non-existent kind")
	}
}

func TestRegistry_List(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	// Register multiple plugins
	r.RegisterWithMetadata(&Plugin{
		Kind:        "stt",
		Name:        "openai",
		Factory:     newMockSTT,
		Description: "OpenAI Whisper STT",
		Version:     "1.0.0",
	})
	r.RegisterWithMetadata(&Plugin{
		Kind:        "stt",
		Name:        "fake",
		Factory:     newMockSTT,
		Description: "Fake STT for testing",
		Version:     "1.0.0",
	})
	r.RegisterWithMetadata(&Plugin{
		Kind:        "tts",
		Name:        "openai",
		Factory:     newMockSTT,
		Description: "OpenAI TTS",
		Version:     "1.0.0",
	})

	// Test listing all plugins
	allPlugins := r.List("")
	if len(allPlugins) != 3 {
		t.Errorf("Expected 3 plugins, got %d", len(allPlugins))
	}

	// Verify sorting (should be sorted by kind, then name)
	expectedOrder := []struct{ kind, name string }{
		{"stt", "fake"},
		{"stt", "openai"},
		{"tts", "openai"},
	}
	for i, expected := range expectedOrder {
		if i >= len(allPlugins) {
			t.Errorf("Missing plugin at index %d", i)
			continue
		}
		if allPlugins[i].Kind != expected.kind || allPlugins[i].Name != expected.name {
			t.Errorf("Expected plugin %d to be %s/%s, got %s/%s",
				i, expected.kind, expected.name, allPlugins[i].Kind, allPlugins[i].Name)
		}
	}

	// Test listing specific kind
	sttPlugins := r.List("stt")
	if len(sttPlugins) != 2 {
		t.Errorf("Expected 2 STT plugins, got %d", len(sttPlugins))
	}

	// Test listing non-existent kind
	nonExistentPlugins := r.List("nonexistent")
	if len(nonExistentPlugins) != 0 {
		t.Errorf("Expected 0 plugins for non-existent kind, got %d", len(nonExistentPlugins))
	}
}

func TestRegistry_ListKinds(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	// Initially should be empty
	kinds := r.ListKinds()
	if len(kinds) != 0 {
		t.Errorf("Expected 0 kinds initially, got %d", len(kinds))
	}

	// Register plugins of different kinds
	r.Register("stt", "fake", newMockSTT)
	r.Register("tts", "fake", newMockSTT)
	r.Register("vad", "fake", newMockSTT)

	kinds = r.ListKinds()
	expected := []string{"stt", "tts", "vad"}
	if !reflect.DeepEqual(kinds, expected) {
		t.Errorf("Expected kinds %v, got %v", expected, kinds)
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := &Registry{
		plugins: make(map[string]map[string]*Plugin),
	}

	// Register some plugins
	r.Register("stt", "fake", newMockSTT)
	r.Register("tts", "fake", newMockSTT)

	// Verify they exist
	if len(r.List("")) != 2 {
		t.Error("Expected 2 plugins before clear")
	}

	// Clear and verify
	r.Clear()
	if len(r.List("")) != 0 {
		t.Error("Expected 0 plugins after clear")
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Save current state to restore later
	originalPlugins := make(map[string]map[string]*Plugin)
	for kind, kindMap := range globalRegistry.plugins {
		originalPlugins[kind] = make(map[string]*Plugin)
		for name, plugin := range kindMap {
			originalPlugins[kind][name] = plugin
		}
	}

	// Clear global registry for clean test
	globalRegistry.Clear()

	// Test global functions
	Register("stt", "global-test", newMockSTT)

	factory, ok := Get("stt", "global-test")
	if !ok {
		t.Error("Expected to find globally registered plugin")
	}
	if factory == nil {
		t.Error("Expected factory to not be nil")
	}

	plugins := List("stt")
	if len(plugins) != 1 {
		t.Errorf("Expected 1 global plugin, got %d", len(plugins))
	}

	kinds := ListKinds()
	if len(kinds) != 1 || kinds[0] != "stt" {
		t.Errorf("Expected kinds [stt], got %v", kinds)
	}

	// Restore original state
	globalRegistry.Clear()
	globalRegistry.plugins = originalPlugins
}