package openai

import (
	"context"
	"testing"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

func TestWhisperSTT_Configuration(t *testing.T) {
	// Test with missing API key
	_, err := NewWhisperSTT(Config{})
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	// Test with API key
	cfg := Config{
		APIKey:   "test-key",
		Model:    "whisper-1",
		Language: "en",
	}
	
	whisper, err := NewWhisperSTT(cfg)
	if err != nil {
		t.Fatalf("Failed to create WhisperSTT: %v", err)
	}

	if whisper.model != "whisper-1" {
		t.Errorf("Expected model whisper-1, got %s", whisper.model)
	}

	if whisper.language != "en" {
		t.Errorf("Expected language en, got %s", whisper.language)
	}
}

func TestWhisperSTT_Capabilities(t *testing.T) {
	whisper, err := NewWhisperSTT(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("Failed to create WhisperSTT: %v", err)
	}

	caps := whisper.Capabilities()
	
	if !caps.Streaming {
		t.Error("Expected streaming to be supported")
	}

	if caps.InterimResults {
		t.Error("Expected interim results to be false for Whisper")
	}

	if len(caps.SupportedLanguages) == 0 {
		t.Error("Expected supported languages to be populated")
	}

	// Check for some common languages
	langMap := make(map[string]bool)
	for _, lang := range caps.SupportedLanguages {
		langMap[lang] = true
	}

	expectedLangs := []string{"en", "es", "fr", "de", "ja", "zh"}
	for _, lang := range expectedLangs {
		if !langMap[lang] {
			t.Errorf("Expected language %s to be supported", lang)
		}
	}
}

func TestWhisperSTT_Stream(t *testing.T) {
	whisper, err := NewWhisperSTT(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("Failed to create WhisperSTT: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := whisper.NewStream(ctx, stt.StreamConfig{
		SampleRate:  16000,
		NumChannels: 1,
		Lang:        "en",
		MaxRetry:    3,
	})
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	// Test that we can push audio frames (won't actually transcribe without real API key)
	frame := rtc.AudioFrame{
		Data:              make([]byte, 1024),
		SampleRate:        16000,
		SamplesPerChannel: 512,
		NumChannels:       1,
		Timestamp:         100 * time.Millisecond,
	}

	err = stream.Push(frame)
	if err != nil {
		t.Errorf("Failed to push audio frame: %v", err)
	}

	// Test closing
	err = stream.CloseSend()
	if err != nil {
		t.Errorf("Failed to close stream: %v", err)
	}

	// Test that pushing after close fails
	err = stream.Push(frame)
	if err == nil {
		t.Error("Expected error when pushing to closed stream")
	}
}

func TestCombineFrames(t *testing.T) {
	stream := &whisperStream{}
	
	// Test empty frames
	_, err := stream.combineFrames([]rtc.AudioFrame{})
	if err == nil {
		t.Error("Expected error for empty frames")
	}

	// Test combining frames
	frames := []rtc.AudioFrame{
		{
			Data:              []byte{1, 2, 3, 4},
			SampleRate:        16000,
			NumChannels:       1,
			Timestamp:         100 * time.Millisecond,
		},
		{
			Data:              []byte{5, 6, 7, 8},
			SampleRate:        16000,
			NumChannels:       1,
			Timestamp:         200 * time.Millisecond,
		},
	}

	combined, err := stream.combineFrames(frames)
	if err != nil {
		t.Fatalf("Failed to combine frames: %v", err)
	}

	expectedData := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	if len(combined.data) != len(expectedData) {
		t.Errorf("Expected combined data length %d, got %d", len(expectedData), len(combined.data))
	}

	for i, b := range expectedData {
		if combined.data[i] != b {
			t.Errorf("Expected byte %d at position %d, got %d", b, i, combined.data[i])
		}
	}

	if combined.sampleRate != 16000 {
		t.Errorf("Expected sample rate 16000, got %d", combined.sampleRate)
	}

	if combined.channels != 1 {
		t.Errorf("Expected 1 channel, got %d", combined.channels)
	}
}

func TestConvertToWAV(t *testing.T) {
	stream := &whisperStream{}
	
	audio := &combinedAudio{
		data:       []byte{0, 1, 2, 3, 4, 5, 6, 7},
		sampleRate: 16000,
		channels:   1,
		duration:   time.Second,
	}

	wavData, err := stream.convertToWAV(audio)
	if err != nil {
		t.Fatalf("Failed to convert to WAV: %v", err)
	}

	// Check WAV header
	if len(wavData) < 44 {
		t.Errorf("WAV data too short: %d bytes", len(wavData))
	}

	// Check RIFF header
	if string(wavData[0:4]) != "RIFF" {
		t.Error("Expected RIFF header")
	}

	if string(wavData[8:12]) != "WAVE" {
		t.Error("Expected WAVE format")
	}

	// Check fmt chunk
	if string(wavData[12:16]) != "fmt " {
		t.Error("Expected fmt chunk")
	}

	// Check data chunk
	if string(wavData[36:40]) != "data" {
		t.Error("Expected data chunk")
	}

	// Check that audio data is included
	expectedTotalSize := 44 + len(audio.data)
	if len(wavData) != expectedTotalSize {
		t.Errorf("Expected WAV size %d, got %d", expectedTotalSize, len(wavData))
	}
}