package audio

import (
	"context"
	"testing"
	"time"

	"livekit-agents-go/media"
)

func TestMockAECProcessor(t *testing.T) {
	config := DefaultAECConfig()
	processor := NewMockAECProcessor(config)
	defer processor.Close()

	// Create test audio frames
	inputData := make([]byte, 1024) // 512 samples at 16-bit
	for i := 0; i < len(inputData); i += 2 {
		// Create a test tone
		sample := int16(1000) // Small amplitude
		inputData[i] = byte(sample & 0xFF)
		inputData[i+1] = byte((sample >> 8) & 0xFF)
	}

	outputData := make([]byte, 1024)
	for i := 0; i < len(outputData); i += 2 {
		// Create a different test tone for output
		sample := int16(500)
		outputData[i] = byte(sample & 0xFF)
		outputData[i+1] = byte((sample >> 8) & 0xFF)
	}

	format := media.AudioFormat{
		SampleRate:    48000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	inputFrame := media.NewAudioFrame(inputData, format)
	outputFrame := media.NewAudioFrame(outputData, format)

	ctx := context.Background()

	// Test dual-stream processing
	processed, err := processor.ProcessStreams(ctx, inputFrame, outputFrame)
	if err != nil {
		t.Fatalf("ProcessStreams failed: %v", err)
	}

	if processed == nil {
		t.Fatal("ProcessStreams returned nil frame")
	}

	// Check metadata
	if processed.Metadata["aec_processed"] != true {
		t.Error("Frame not marked as AEC processed")
	}

	// Test input-only processing
	processed2, err := processor.ProcessInput(ctx, inputFrame)
	if err != nil {
		t.Fatalf("ProcessInput failed: %v", err)
	}

	if processed2 == nil {
		t.Fatal("ProcessInput returned nil frame")
	}

	// Test delay setting
	newDelay := 100 * time.Millisecond
	err = processor.SetDelay(newDelay)
	if err != nil {
		t.Fatalf("SetDelay failed: %v", err)
	}

	// Test stats
	stats := processor.GetStats()
	if stats.FramesProcessed != 2 {
		t.Errorf("Expected 2 frames processed, got %d", stats.FramesProcessed)
	}
}

func TestLocalAudioIOWithAEC(t *testing.T) {
	config := DefaultConfig()
	config.EnableAECProcessing = true
	config.EstimatedDelay = 50 * time.Millisecond

	// Note: This test will fail in CI without audio devices
	// We'll create the instance but not start it
	audioIO, err := NewLocalAudioIO(config)
	if err != nil {
		t.Skipf("Failed to create LocalAudioIO (likely no audio devices): %v", err)
	}
	defer audioIO.Close()

	// Test callback setting
	callback := func(input, output *media.AudioFrame) (*media.AudioFrame, error) {
		return input, nil
	}

	audioIO.SetAudioProcessingCallback(callback)

	// Test delay methods
	delay := audioIO.GetEstimatedDelay()
	if delay != config.EstimatedDelay {
		t.Errorf("Expected delay %v, got %v", config.EstimatedDelay, delay)
	}

	newDelay := 75 * time.Millisecond
	audioIO.SetEstimatedDelay(newDelay)
	delay = audioIO.GetEstimatedDelay()
	if delay != newDelay {
		t.Errorf("Expected delay %v, got %v", newDelay, delay)
	}

	// Test ring buffer operations
	testSamples := []int16{1000, 2000, 3000, 4000, 5000}
	audioIO.addToRingBuffer(testSamples)

	// Get reference samples
	refSamples := audioIO.getOutputReference(0, 3)
	if len(refSamples) != 3 {
		t.Errorf("Expected 3 reference samples, got %d", len(refSamples))
	}
}

func TestPionAECProcessor(t *testing.T) {
	config := DefaultAECConfig()
	processor, err := NewPionAECProcessor(config)
	if err != nil {
		t.Fatalf("Failed to create Pion AEC processor: %v", err)
	}
	defer processor.Close()

	// Create test audio frame
	inputData := make([]byte, 1024)
	for i := 0; i < len(inputData); i += 2 {
		sample := int16(1000)
		inputData[i] = byte(sample & 0xFF)
		inputData[i+1] = byte((sample >> 8) & 0xFF)
	}

	format := media.AudioFormat{
		SampleRate:    48000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	inputFrame := media.NewAudioFrame(inputData, format)
	ctx := context.Background()

	// Test processing
	processed, err := processor.ProcessInput(ctx, inputFrame)
	if err != nil {
		t.Fatalf("ProcessInput failed: %v", err)
	}

	if processed == nil {
		t.Fatal("ProcessInput returned nil frame")
	}

	// Check metadata
	if processed.Metadata["aec_engine"] != "pion_fallback" {
		t.Error("Frame not marked with pion_fallback engine")
	}

	// Test stats
	stats := processor.GetStats()
	if stats.FramesProcessed != 1 {
		t.Errorf("Expected 1 frame processed, got %d", stats.FramesProcessed)
	}
}

func TestAECIntegration(t *testing.T) {
	// Test integration between LocalAudioIO and AEC processor
	config := DefaultConfig()
	config.EnableAECProcessing = true

	audioIO, err := NewLocalAudioIO(config)
	if err != nil {
		t.Skipf("Failed to create LocalAudioIO (likely no audio devices): %v", err)
	}
	defer audioIO.Close()

	// Create AEC processor
	aecConfig := DefaultAECConfig()
	aecProcessor := NewMockAECProcessor(aecConfig)
	defer aecProcessor.Close()

	// Set up callback to use AEC processor
	audioIO.SetAudioProcessingCallback(func(input, output *media.AudioFrame) (*media.AudioFrame, error) {
		ctx := context.Background()
		return aecProcessor.ProcessStreams(ctx, input, output)
	})

	// Verify the callback was set (we can't easily test execution without audio devices)
	// This test validates the integration setup
	t.Log("AEC integration setup completed successfully")
}