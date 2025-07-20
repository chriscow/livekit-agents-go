package main

import (
	"context"
	"log"
	"os"

	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
	"livekit-agents-go/plugins/openai"
	"livekit-agents-go/test/mock"
)

func main() {
	log.Println("🧪 Testing OpenAI Plugin System")
	
	// Always register mock plugin as fallback
	err := mock.RegisterMockPlugin()
	if err != nil {
		log.Printf("Warning: Failed to register mock plugin: %v", err)
	} else {
		log.Println("✅ Mock plugin registered successfully")
	}
	
	// Try to register OpenAI plugin if API key is available
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		log.Printf("🔑 Found OpenAI API key, registering OpenAI plugin")
		err = openai.RegisterPlugin(apiKey)
		if err != nil {
			log.Printf("❌ Failed to register OpenAI plugin: %v", err)
		} else {
			log.Println("✅ OpenAI plugin registered successfully")
		}
	} else {
		log.Println("⚠️ No OpenAI API key found, using mock services only")
	}
	
	// Print registry status
	plugins.PrintRegistryStatus()
	
	// Test service creation with fallbacks
	ctx := context.Background()
	
	// Test STT service
	log.Println("\n🎙️ Testing STT Service Creation")
	sttService, err := plugins.CreateSTT("whisper")
	if err != nil {
		log.Printf("❌ Failed to create STT service: %v", err)
		return
	}
	log.Printf("✅ STT service created: %s v%s", sttService.Name(), sttService.Version())
	
	// Test with dummy audio
	audioFrame := media.NewAudioFrame(make([]byte, 1024), media.AudioFormat48kHz16BitMono)
	recognition, err := sttService.Recognize(ctx, audioFrame)
	if err != nil {
		log.Printf("❌ STT recognition failed: %v", err)
	} else {
		log.Printf("✅ STT recognition result: '%s' (confidence: %.2f)", 
			recognition.Text, recognition.Confidence)
	}
	
	// Test TTS service
	log.Println("\n🔊 Testing TTS Service Creation")
	ttsService, err := plugins.CreateTTS("openai-tts")
	if err != nil {
		log.Printf("❌ Failed to create TTS service: %v", err)
		return
	}
	log.Printf("✅ TTS service created: %s v%s", ttsService.Name(), ttsService.Version())
	
	// Test text synthesis
	audioResult, err := ttsService.Synthesize(ctx, "Hello from the OpenAI plugin test!", nil)
	if err != nil {
		log.Printf("❌ TTS synthesis failed: %v", err)
	} else {
		log.Printf("✅ TTS synthesis result: %d bytes audio", len(audioResult.Data))
	}
	
	// Test LLM service
	log.Println("\n🤖 Testing LLM Service Creation")
	llmService, err := plugins.CreateLLM("gpt-4o")
	if err != nil {
		log.Printf("❌ Failed to create LLM service: %v", err)
		return
	}
	log.Printf("✅ LLM service created: %s v%s", llmService.Name(), llmService.Version())
	
	// Test chat completion - simplified for now
	log.Printf("💬 Chat completion test skipped (requires proper message types)")
	
	// Test service recommendations
	log.Println("\n🎯 Testing Service Recommendations")
	for _, serviceType := range []string{"stt", "tts", "llm", "vad"} {
		recommended, err := plugins.GetRecommendedService(serviceType)
		if err != nil {
			log.Printf("❌ Failed to get recommendation for %s: %v", serviceType, err)
		} else {
			log.Printf("✅ Recommended %s service: %s", serviceType, recommended)
		}
	}
	
	log.Println("\n🎉 OpenAI Plugin System Test Complete!")
}