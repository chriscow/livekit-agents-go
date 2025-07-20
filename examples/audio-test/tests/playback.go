package tests

import (
	"context"
	"fmt"
	"math"
	"time"

	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// TestSpeakerPlayback tests speaker audio playback with test tones
func TestSpeakerPlayback(audioIO *audio.LocalAudioIO) error {
	fmt.Println("🔊 Testing speaker playback...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Start audio I/O
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O: %w", err)
	}
	
	defer func() {
		fmt.Println("🛑 Stopping speaker audio I/O...")
		if err := audioIO.Stop(); err != nil {
			fmt.Printf("⚠️  Error stopping audio I/O: %v\n", err)
		}
		fmt.Println("✅ Speaker audio I/O stopped")
	}()
	
	outputChan := audioIO.OutputChan()
	
	// Test different frequencies
	frequencies := []float64{440.0, 880.0, 220.0} // A4, A5, A3
	names := []string{"A4 (440 Hz)", "A5 (880 Hz)", "A3 (220 Hz)"}
	
	for i, freq := range frequencies {
		fmt.Printf("🎵 Playing %s for 2 seconds...\n", names[i])
		
		// Generate and play tone
		err := playTone(ctx, outputChan, freq, 2*time.Second, 0.3)
		if err != nil {
			return fmt.Errorf("failed to play tone %s: %w", names[i], err)
		}
		
		// Brief pause between tones
		time.Sleep(500 * time.Millisecond)
	}
	
	fmt.Println("✅ Speaker playback test completed")
	return nil
}

// playTone generates and sends a sine wave tone to the audio output
func playTone(ctx context.Context, outputChan chan<- *media.AudioFrame, frequency float64, duration time.Duration, amplitude float64) error {
	const sampleRate = 48000
	const channels = 1
	const bitsPerSample = 16
	
	samplesPerFrame := sampleRate / 10 // 100ms frames
	totalSamples := int(float64(sampleRate) * duration.Seconds())
	frameCount := totalSamples / samplesPerFrame
	
	format := media.AudioFormat{
		SampleRate:    sampleRate,
		Channels:      channels,
		BitsPerSample: bitsPerSample,
		Format:        media.AudioFormatPCM,
	}
	
	for frame := 0; frame < frameCount; frame++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Generate PCM data for this frame
		pcmData := make([]byte, samplesPerFrame*channels*2) // 2 bytes per 16-bit sample
		
		for i := 0; i < samplesPerFrame; i++ {
			sampleIndex := frame*samplesPerFrame + i
			t := float64(sampleIndex) / float64(sampleRate)
			
			// Generate sine wave
			value := amplitude * math.Sin(2*math.Pi*frequency*t)
			
			// Convert to 16-bit signed integer
			sample := int16(value * 32767.0)
			
			// Store as little-endian
			pcmData[i*2] = byte(sample & 0xFF)
			pcmData[i*2+1] = byte((sample >> 8) & 0xFF)
		}
		
		// Create audio frame
		audioFrame := media.NewAudioFrame(pcmData, format)
		
		// Send to output (non-blocking)
		select {
		case outputChan <- audioFrame:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			fmt.Println("⚠️  Audio output timeout - continuing...")
		}
		
		// Control playback timing
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil
}

// TestLoopback tests microphone to speaker loopback
func TestLoopback(audioIO *audio.LocalAudioIO, duration time.Duration) error {
	fmt.Printf("🔄 Starting loopback test for %v...\n", duration)
	fmt.Println("Speak into the microphone - you should hear your voice through the speakers!")
	
	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	defer cancel()
	
	// Start audio I/O
	if err := audioIO.Start(ctx); err != nil {
		return fmt.Errorf("failed to start audio I/O: %w", err)
	}
	
	defer func() {
		fmt.Println("🛑 Stopping loopback audio I/O...")
		if err := audioIO.Stop(); err != nil {
			fmt.Printf("⚠️  Error stopping audio I/O: %v\n", err)
		}
		fmt.Println("✅ Loopback audio I/O stopped")
	}()
	
	inputChan := audioIO.InputChan()
	outputChan := audioIO.OutputChan()
	
	fmt.Println("🔴 Loopback active - speak now!")
	
	// Set up timer
	timer := time.After(duration)
	frameCount := 0
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer:
			fmt.Printf("⏹️  Loopback completed. Processed %d frames.\n", frameCount)
			return nil
		case frame, ok := <-inputChan:
			if !ok {
				return fmt.Errorf("audio input channel closed unexpectedly")
			}
			
			frameCount++
			
			// Pass audio directly from input to output (loopback)
			select {
			case outputChan <- frame:
				// Successfully sent to output
			case <-time.After(10 * time.Millisecond):
				// Skip this frame if output is not ready
			}
			
			// Show activity indicator every second
			if frameCount%10 == 0 {
				energy := calculateFrameEnergy(frame)
				if energy > 0.001 {
					fmt.Printf("🎤➡️🔊 Activity: %.3f\n", energy)
				}
			}
		}
	}
}