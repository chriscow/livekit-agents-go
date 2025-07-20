package mock

import (
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/services/vad"
)

// MockPlugin implements the Plugin interface for testing
type MockPlugin struct {
	*plugins.BasePlugin
}

// NewMockPlugin creates a new mock plugin
func NewMockPlugin() *MockPlugin {
	return &MockPlugin{
		BasePlugin: plugins.NewBasePlugin("mock", "1.0.0", "Mock services for testing"),
	}
}

// Register registers mock services with the plugin registry
func (p *MockPlugin) Register(registry *plugins.Registry) error {
	// Register mock STT service
	registry.RegisterSTT("mock-stt", func() stt.STT {
		return NewMockSTT()
	})

	// Register mock STT with custom responses
	registry.RegisterSTT("mock-stt-custom", func() stt.STT {
		return NewMockSTT("Custom response 1", "Custom response 2", "Custom response 3")
	})

	// Register mock TTS service
	registry.RegisterTTS("mock-tts", func() tts.TTS {
		return NewMockTTS()
	})

	// Register mock LLM service
	registry.RegisterLLM("mock-llm", func() llm.LLM {
		return NewMockLLM()
	})

	// Register mock LLM with custom responses
	registry.RegisterLLM("mock-llm-friendly", func() llm.LLM {
		return NewMockLLM(
			"Hello there! I'm your friendly test assistant!",
			"That sounds wonderful! Tell me more about it.",
			"I'm here to help with anything you need!",
			"What a great question! Let me think about that...",
		)
	})

	// Register mock VAD service
	registry.RegisterVAD("mock-vad", func() vad.VAD {
		return NewMockVAD()
	})

	// Register mock Silero VAD
	registry.RegisterVAD("mock-silero", func() vad.VAD {
		return NewMockSileroVAD()
	})

	return nil
}

// RegisterMockPlugin registers the mock plugin with the global registry
func RegisterMockPlugin() error {
	plugin := NewMockPlugin()
	return plugins.RegisterPlugin(plugin)
}

// Register the delegate function for auto-discovery
func init() {
	plugins.RegisterPluginDelegate("mock", func(apiKey string) error {
		return RegisterMockPlugin() // Mock doesn't need API key
	})
}

// Helper functions for creating pre-configured mock services

// CreateMockSTTWithResponses creates a mock STT with custom responses
func CreateMockSTTWithResponses(responses ...string) *MockSTT {
	return NewMockSTT(responses...)
}

// CreateMockLLMWithResponses creates a mock LLM with custom responses  
func CreateMockLLMWithResponses(responses ...string) *MockLLM {
	return NewMockLLM(responses...)
}

// CreateMockTTSWithFormat creates a mock TTS with custom audio format
func CreateMockTTSWithFormat(sampleRate, channels, bitsPerSample int) *MockTTS {
	mockTTS := NewMockTTS()
	mockTTS.SetAudioFormat(media.AudioFormat{
		SampleRate:    sampleRate,
		Channels:      channels,
		BitsPerSample: bitsPerSample,
		Format:        media.AudioFormatPCM,
	})
	return mockTTS
}

// CreateMockVADWithPattern creates a mock VAD with custom speech pattern
func CreateMockVADWithPattern(pattern []bool) *MockVAD {
	mockVAD := NewMockVAD()
	mockVAD.SetSpeechPattern(pattern)
	return mockVAD
}

// TestScenarios provides pre-configured test scenarios

// TestScenario represents a complete test scenario
type TestScenario struct {
	Name        string
	Description string
	STT         *MockSTT
	LLM         *MockLLM
	TTS         *MockTTS
	VAD         *MockVAD
}

// GetTestScenarios returns predefined test scenarios
func GetTestScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:        "basic-conversation",
			Description: "Basic conversation flow with standard responses",
			STT:         NewMockSTT("Hello", "How are you?", "Thank you"),
			LLM:         NewMockLLM("Hello! How can I help you today?", "I'm doing well, thank you for asking!", "You're welcome!"),
			TTS:         NewMockTTS(),
			VAD:         NewMockVAD(),
		},
		{
			Name:        "noisy-environment",
			Description: "Conversation in noisy environment with low confidence",
			STT: func() *MockSTT {
				stt := NewMockSTT("Hello... static...", "Can you... hear me?", "...better now")
				stt.SetConfidence(0.6) // Lower confidence
				return stt
			}(),
			LLM: NewMockLLM("I'm having trouble hearing you clearly. Could you speak louder?", "That's much better, thank you!"),
			TTS: NewMockTTS(),
			VAD: func() *MockVAD {
				vad := NewMockVAD()
				vad.SetNoiseLevel(0.3) // Higher noise level
				return vad
			}(),
		},
		{
			Name:        "quick-responses",
			Description: "Fast-paced conversation with quick responses",
			STT: func() *MockSTT {
				stt := NewMockSTT("Yes", "No", "Maybe", "Okay", "Sure")
				stt.SetDelay(50 * time.Millisecond) // Fast recognition
				return stt
			}(),
			LLM: func() *MockLLM {
				llm := NewMockLLM("Got it!", "Understood!", "Makes sense!", "Alright!", "Perfect!")
				llm.SetDelay(100 * time.Millisecond) // Fast responses
				return llm
			}(),
			TTS: func() *MockTTS {
				tts := NewMockTTS()
				tts.SetDelay(50 * time.Millisecond) // Fast synthesis
				return tts
			}(),
			VAD: NewMockVAD(),
		},
		{
			Name:        "long-form-content",
			Description: "Longer responses and detailed conversations",
			STT: NewMockSTT(
				"Can you explain how artificial intelligence works?",
				"That's very interesting, tell me more about machine learning",
				"What are the practical applications of this technology?",
			),
			LLM: NewMockLLM(
				"Artificial intelligence is a fascinating field that involves creating systems that can perform tasks typically requiring human intelligence. It encompasses machine learning, neural networks, and many other technologies.",
				"Machine learning is a subset of AI where systems learn from data rather than being explicitly programmed. They identify patterns and make predictions based on training data.",
				"AI has many practical applications including healthcare diagnostics, autonomous vehicles, recommendation systems, natural language processing, and robotics.",
			),
			TTS: NewMockTTS(),
			VAD: NewMockVAD(),
		},
	}
}

// ApplyTestScenario applies a test scenario to the global plugin registry
func ApplyTestScenario(scenario TestScenario) error {
	registry := plugins.GlobalRegistry()

	// Register scenario-specific services
	registry.RegisterSTT("scenario-stt", func() stt.STT {
		return scenario.STT
	})

	registry.RegisterLLM("scenario-llm", func() llm.LLM {
		return scenario.LLM
	})

	registry.RegisterTTS("scenario-tts", func() tts.TTS {
		return scenario.TTS
	})

	registry.RegisterVAD("scenario-vad", func() vad.VAD {
		return scenario.VAD
	})

	return nil
}