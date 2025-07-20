package audio

import (
	"context"
	"testing"
	"time"

	"livekit-agents-go/media"
)

func TestNewAECPipeline(t *testing.T) {
	config := DefaultAECConfig()
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	if pipeline.config.SampleRate != config.SampleRate {
		t.Errorf("Expected sample rate %d, got %d", config.SampleRate, pipeline.config.SampleRate)
	}

	if pipeline.frameSize != config.SampleRate/100 {
		t.Errorf("Expected frame size %d, got %d", config.SampleRate/100, pipeline.frameSize)
	}
}

func TestAECPipelineStartStop(t *testing.T) {
	config := DefaultAECConfig()
	config.SampleRate = 24000 // Use 24kHz for faster testing
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test start
	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}

	// Verify running state
	pipeline.mu.RLock()
	running := pipeline.running
	pipeline.mu.RUnlock()
	
	if !running {
		t.Error("Pipeline should be running after Start()")
	}

	// Wait a bit to let it process
	time.Sleep(100 * time.Millisecond)

	// Test stop
	if err := pipeline.Stop(); err != nil {
		t.Fatalf("Failed to stop pipeline: %v", err)
	}

	// Verify stopped state
	pipeline.mu.RLock()
	running = pipeline.running
	pipeline.mu.RUnlock()
	
	if running {
		t.Error("Pipeline should not be running after Stop()")
	}
}

func TestAECPipelineProcessAudioFrame(t *testing.T) {
	config := DefaultAECConfig()
	config.SampleRate = 24000
	config.EnableEchoCancellation = true
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Create test audio frame (10ms at 24kHz = 240 samples)
	frameSize := 240
	pcmData := make([]byte, frameSize*2) // 16-bit samples
	
	// Generate test tone
	for i := 0; i < frameSize; i++ {
		// 1kHz sine wave
		sample := int16(8000.0 * 0.5) // Moderate amplitude
		pcmData[i*2] = byte(sample & 0xFF)
		pcmData[i*2+1] = byte((sample >> 8) & 0xFF)
	}

	format := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	inputFrame := media.NewAudioFrame(pcmData, format)
	
	// Test processing without output reference
	processedFrame, err := pipeline.processAudioFrame(inputFrame, nil)
	if err != nil {
		t.Fatalf("Failed to process audio frame: %v", err)
	}

	if processedFrame == nil {
		t.Fatal("Processed frame should not be nil")
	}

	// Verify metadata was added
	if processedFrame.Metadata["aec_pipeline_processed"] != true {
		t.Error("Processed frame should have aec_pipeline_processed metadata")
	}

	if processedFrame.Metadata["has_output_reference"] != false {
		t.Error("Processed frame should indicate no output reference")
	}

	// Test processing with output reference
	outputRefFrame := media.NewAudioFrame(pcmData, format)
	processedFrame2, err := pipeline.processAudioFrame(inputFrame, outputRefFrame)
	if err != nil {
		t.Fatalf("Failed to process audio frame with output reference: %v", err)
	}

	if processedFrame2.Metadata["has_output_reference"] != true {
		t.Error("Processed frame should indicate output reference present")
	}
}

func TestAECPipelineStats(t *testing.T) {
	config := DefaultAECConfig()
	config.SampleRate = 24000
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Initial stats should be zeroed
	stats := pipeline.GetStats()
	if stats.FramesProcessed != 0 {
		t.Error("Initial frames processed should be 0")
	}

	// Process a frame to update stats
	frameSize := 240
	pcmData := make([]byte, frameSize*2)
	format := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}
	frame := media.NewAudioFrame(pcmData, format)

	_, err = pipeline.processAudioFrame(frame, nil)
	if err != nil {
		t.Fatalf("Failed to process frame: %v", err)
	}

	// Check updated stats
	stats = pipeline.GetStats()
	if stats.FramesProcessed != 1 {
		t.Errorf("Expected 1 frame processed, got %d", stats.FramesProcessed)
	}

	if stats.PipelineLatency <= 0 {
		t.Error("Pipeline latency should be measured")
	}
}

func TestAECPipelineSetDelay(t *testing.T) {
	config := DefaultAECConfig()
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	newDelay := 75 * time.Millisecond
	err = pipeline.SetDelay(newDelay)
	if err != nil {
		t.Fatalf("Failed to set delay: %v", err)
	}

	// Verify delay was set in audio I/O
	actualDelay := pipeline.audioIO.GetEstimatedDelay()
	if actualDelay != newDelay {
		t.Errorf("Expected delay %v, got %v", newDelay, actualDelay)
	}
}

func TestAECPipelineChannels(t *testing.T) {
	config := DefaultAECConfig()
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Test channel access
	inputChan := pipeline.GetInputChan()
	if inputChan == nil {
		t.Error("Input channel should not be nil")
	}

	outputChan := pipeline.GetOutputChan()
	if outputChan == nil {
		t.Error("Output channel should not be nil")
	}
}

func TestAECPipelineInvalidFrameRate(t *testing.T) {
	config := DefaultAECConfig()
	config.SampleRate = 24000
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Create frame with wrong sample rate
	pcmData := make([]byte, 480) // 240 samples
	format := media.AudioFormat{
		SampleRate:    48000, // Wrong sample rate
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}
	frame := media.NewAudioFrame(pcmData, format)

	_, err = pipeline.processAudioFrame(frame, nil)
	if err == nil {
		t.Error("Expected error for mismatched sample rate")
	}

	// Check that frame was dropped
	stats := pipeline.GetStats()
	if stats.FramesDropped == 0 {
		t.Error("Frame should have been dropped")
	}
}

func TestAECPipelineDisabledEchoCancellation(t *testing.T) {
	config := DefaultAECConfig()
	config.EnableEchoCancellation = false // Disable AEC
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		t.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Should use MockAECProcessor when AEC is disabled
	_, isMock := pipeline.processor.(*MockAECProcessor)
	if !isMock {
		t.Error("Should use MockAECProcessor when echo cancellation is disabled")
	}
}

func BenchmarkAECPipelineProcessing(b *testing.B) {
	config := DefaultAECConfig()
	config.SampleRate = 24000
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		b.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Create test frame
	frameSize := 240 // 10ms at 24kHz
	pcmData := make([]byte, frameSize*2)
	format := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}
	frame := media.NewAudioFrame(pcmData, format)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := pipeline.processAudioFrame(frame, nil)
		if err != nil {
			b.Fatalf("Processing failed: %v", err)
		}
	}
}

func BenchmarkAECPipelineWithOutputReference(b *testing.B) {
	config := DefaultAECConfig()
	config.SampleRate = 24000
	
	pipeline, err := NewAECPipeline(config)
	if err != nil {
		b.Fatalf("Failed to create AEC pipeline: %v", err)
	}
	defer pipeline.Close()

	// Create test frames
	frameSize := 240
	pcmData := make([]byte, frameSize*2)
	format := media.AudioFormat{
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}
	inputFrame := media.NewAudioFrame(pcmData, format)
	outputFrame := media.NewAudioFrame(pcmData, format)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := pipeline.processAudioFrame(inputFrame, outputFrame)
		if err != nil {
			b.Fatalf("Processing failed: %v", err)
		}
	}
}