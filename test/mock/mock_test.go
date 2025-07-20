package mock

import (
	"context"
	"testing"
	"time"

	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/vad"
)

// TestMockSTT tests the mock STT implementation
func TestMockSTT(t *testing.T) {
	mockSTT := NewMockSTT("Test response 1", "Test response 2")
	
	// Test basic recognition
	audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	recognition, err := mockSTT.Recognize(ctx, audioFrame)
	if err != nil {
		t.Fatalf("Failed to recognize: %v", err)
	}
	
	if recognition.Text != "Test response 1" {
		t.Errorf("Expected 'Test response 1', got '%s'", recognition.Text)
	}
	
	if recognition.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", recognition.Confidence)
	}
	
	// Test streaming recognition
	stream, err := mockSTT.RecognizeStream(ctx)
	if err != nil {
		t.Fatalf("Failed to create recognition stream: %v", err)
	}
	defer stream.Close()
	
	// Send audio
	err = stream.SendAudio(audioFrame)
	if err != nil {
		t.Errorf("Failed to send audio: %v", err)
	}
	
	// Close sending to trigger processing
	err = stream.CloseSend()
	if err != nil {
		t.Errorf("Failed to close send: %v", err)
	}
	
	// Should get response eventually (within timeout)
	done := make(chan bool)
	go func() {
		_, err := stream.Recv()
		if err != nil {
			t.Errorf("Failed to receive recognition: %v", err)
		}
		done <- true
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for recognition result")
	}
}

// TestMockLLM tests the mock LLM implementation
func TestMockLLM(t *testing.T) {
	mockLLM := NewMockLLM("Hello response", "Follow-up response")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test completion
	completion, err := mockLLM.Complete(ctx, "Hello", nil)
	if err != nil {
		t.Fatalf("Failed to complete: %v", err)
	}
	
	if completion.Text == "" {
		t.Error("Expected non-empty completion text")
	}
	
	if completion.Usage.TotalTokens == 0 {
		t.Error("Expected non-zero token usage")
	}
	
	// Test chat completion
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello there"},
	}
	
	chatCompletion, err := mockLLM.Chat(ctx, messages, nil)
	if err != nil {
		t.Fatalf("Failed to chat: %v", err)
	}
	
	if chatCompletion.Message.Content == "" {
		t.Error("Expected non-empty chat response")
	}
	
	if chatCompletion.Message.Role != llm.RoleAssistant {
		t.Error("Expected assistant role in response")
	}
	
	// Test streaming chat
	stream, err := mockLLM.ChatStream(ctx, messages, nil)
	if err != nil {
		t.Fatalf("Failed to create chat stream: %v", err)
	}
	defer stream.Close()
	
	// Receive at least one chunk
	chunk, err := stream.Recv()
	if err != nil {
		t.Errorf("Failed to receive chat chunk: %v", err)
	}
	
	if chunk.Delta.Content == "" && chunk.FinishReason == "" {
		t.Error("Expected either content or finish reason in chunk")
	}
}

// TestMockTTS tests the mock TTS implementation
func TestMockTTS(t *testing.T) {
	mockTTS := NewMockTTS()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test synthesis
	audioFrame, err := mockTTS.Synthesize(ctx, "Hello world", nil)
	if err != nil {
		t.Fatalf("Failed to synthesize: %v", err)
	}
	
	if len(audioFrame.Data) == 0 {
		t.Error("Expected non-empty audio data")
	}
	
	if audioFrame.Duration == 0 {
		t.Error("Expected non-zero audio duration")
	}
	
	// Test streaming synthesis
	stream, err := mockTTS.SynthesizeStream(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create synthesis stream: %v", err)
	}
	defer stream.Close()
	
	// Send text
	err = stream.SendText("Test synthesis")
	if err != nil {
		t.Errorf("Failed to send text: %v", err)
	}
	
	// Receive audio
	audio, err := stream.Recv()
	if err != nil {
		t.Errorf("Failed to receive audio: %v", err)
	}
	
	if len(audio.Data) == 0 {
		t.Error("Expected non-empty synthesized audio")
	}
}

// TestMockVAD tests the mock VAD implementation
func TestMockVAD(t *testing.T) {
	mockVAD := NewMockVAD()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test detection
	audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
	
	detection, err := mockVAD.Detect(ctx, audioFrame)
	if err != nil {
		t.Fatalf("Failed to detect: %v", err)
	}
	
	if detection.Probability < 0 || detection.Probability > 1 {
		t.Errorf("Expected probability between 0 and 1, got %f", detection.Probability)
	}
	
	if detection.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
	
	// Test streaming detection
	stream, err := mockVAD.DetectStream(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create detection stream: %v", err)
	}
	defer stream.Close()
	
	// Send audio
	err = stream.SendAudio(audioFrame)
	if err != nil {
		t.Errorf("Failed to send audio: %v", err)
	}
	
	// Receive detection
	streamDetection, err := stream.Recv()
	if err != nil {
		t.Errorf("Failed to receive detection: %v", err)
	}
	
	if streamDetection.Probability < 0 || streamDetection.Probability > 1 {
		t.Errorf("Expected probability between 0 and 1, got %f", streamDetection.Probability)
	}
}

// TestMockSileroVAD tests the mock Silero VAD implementation
func TestMockSileroVAD(t *testing.T) {
	mockSileroVAD := NewMockSileroVAD()
	defer mockSileroVAD.Close()
	
	// Test capabilities
	capabilities := mockSileroVAD.Capabilities()
	if capabilities.UpdateInterval <= 0 {
		t.Error("Expected positive update interval")
	}
	
	// Test stream creation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	stream, err := mockSileroVAD.CreateStream(ctx)
	if err != nil {
		t.Fatalf("Failed to create VAD stream: %v", err)
	}
	defer stream.Close()
	
	// Test frame processing
	audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
	
	events, err := stream.ProcessFrame(ctx, audioFrame)
	if err != nil {
		t.Errorf("Failed to process frame: %v", err)
	}
	
	if len(events) == 0 {
		t.Error("Expected at least one VAD event")
	}
	
	// Check that we got inference done event
	foundInferenceDone := false
	for _, event := range events {
		if event.Type == vad.VADEventTypeInferenceDone {
			foundInferenceDone = true
			break
		}
	}
	
	if !foundInferenceDone {
		t.Error("Expected inference done event")
	}
}

// TestMockPlugin tests the mock plugin registration
func TestMockPlugin(t *testing.T) {
	// Create a new registry for testing
	registry := plugins.NewRegistry()
	
	// Register mock plugin
	mockPlugin := NewMockPlugin()
	err := registry.RegisterPlugin(mockPlugin)
	if err != nil {
		t.Fatalf("Failed to register mock plugin: %v", err)
	}
	
	// Test service creation
	sttService, err := registry.CreateSTT("mock-stt")
	if err != nil {
		t.Fatalf("Failed to create mock STT: %v", err)
	}
	
	if sttService.Name() != "mock-stt" {
		t.Errorf("Expected service name 'mock-stt', got '%s'", sttService.Name())
	}
	
	llmService, err := registry.CreateLLM("mock-llm")
	if err != nil {
		t.Fatalf("Failed to create mock LLM: %v", err)
	}
	
	if llmService.Name() != "mock-llm" {
		t.Errorf("Expected service name 'mock-llm', got '%s'", llmService.Name())
	}
	
	ttsService, err := registry.CreateTTS("mock-tts")
	if err != nil {
		t.Fatalf("Failed to create mock TTS: %v", err)
	}
	
	if ttsService.Name() != "mock-tts" {
		t.Errorf("Expected service name 'mock-tts', got '%s'", ttsService.Name())
	}
	
	vadService, err := registry.CreateVAD("mock-vad")
	if err != nil {
		t.Fatalf("Failed to create mock VAD: %v", err)
	}
	
	if vadService.Name() != "mock-vad" {
		t.Errorf("Expected service name 'mock-vad', got '%s'", vadService.Name())
	}
}

// TestTestScenarios tests the predefined test scenarios
func TestTestScenarios(t *testing.T) {
	scenarios := GetTestScenarios()
	
	if len(scenarios) == 0 {
		t.Error("Expected at least one test scenario")
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			if scenario.STT == nil {
				t.Error("Expected STT service in scenario")
			}
			
			if scenario.LLM == nil {
				t.Error("Expected LLM service in scenario")
			}
			
			if scenario.TTS == nil {
				t.Error("Expected TTS service in scenario")
			}
			
			if scenario.VAD == nil {
				t.Error("Expected VAD service in scenario")
			}
			
			if scenario.Description == "" {
				t.Error("Expected non-empty description")
			}
		})
	}
}

// TestGlobalMockRegistration tests registering mock plugin globally
func TestGlobalMockRegistration(t *testing.T) {
	// This test ensures we can register with the global registry
	err := RegisterMockPlugin()
	if err != nil {
		t.Fatalf("Failed to register mock plugin globally: %v", err)
	}
	
	// Test that services are available
	sttService, err := plugins.CreateSTT("mock-stt")
	if err != nil {
		t.Fatalf("Failed to create mock STT from global registry: %v", err)
	}
	
	if sttService == nil {
		t.Error("Expected non-nil STT service")
	}
}

// BenchmarkMockServices benchmarks the mock services
func BenchmarkMockSTT(b *testing.B) {
	mockSTT := NewMockSTT()
	audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockSTT.Recognize(ctx, audioFrame)
		if err != nil {
			b.Fatalf("Recognition failed: %v", err)
		}
	}
}

func BenchmarkMockLLM(b *testing.B) {
	mockLLM := NewMockLLM()
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Hello"}}
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockLLM.Chat(ctx, messages, nil)
		if err != nil {
			b.Fatalf("Chat failed: %v", err)
		}
	}
}

func BenchmarkMockTTS(b *testing.B) {
	mockTTS := NewMockTTS()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockTTS.Synthesize(ctx, "Hello world", nil)
		if err != nil {
			b.Fatalf("Synthesis failed: %v", err)
		}
	}
}

func BenchmarkMockVAD(b *testing.B) {
	mockVAD := NewMockVAD()
	audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockVAD.Detect(ctx, audioFrame)
		if err != nil {
			b.Fatalf("Detection failed: %v", err)
		}
	}
}