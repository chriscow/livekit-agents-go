package tests

import (
	"context"
	"fmt"
	"time"

	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// TestMicrophoneCapture tests microphone audio capture
func TestMicrophoneCapture(audioIO *audio.LocalAudioIO, duration time.Duration) ([]*media.AudioFrame, error) {
	fmt.Printf("🎤 Capturing audio for %v...\n", duration)
	
	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	defer cancel()
	
	// Start audio I/O
	if err := audioIO.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start audio I/O: %w", err)
	}
	
	defer func() {
		fmt.Println("🛑 Stopping audio I/O...")
		if err := audioIO.Stop(); err != nil {
			fmt.Printf("⚠️  Error stopping audio I/O: %v\n", err)
		}
		fmt.Println("✅ Audio I/O stopped")
	}()
	
	// Collect audio frames
	var audioFrames []*media.AudioFrame
	inputChan := audioIO.InputChan()
	
	fmt.Println("🔴 Recording... make some noise!")
	
	// Set up a timer to stop recording
	timer := time.After(duration)
	
	for {
		select {
		case <-ctx.Done():
			return audioFrames, ctx.Err()
		case <-timer:
			fmt.Printf("⏹️  Recording completed. Captured %d frames.\n", len(audioFrames))
			return audioFrames, nil
		case frame, ok := <-inputChan:
			if !ok {
				return audioFrames, fmt.Errorf("audio input channel closed unexpectedly")
			}
			audioFrames = append(audioFrames, frame)
			
			// Show progress every second
			if len(audioFrames)%10 == 0 {
				energy := calculateFrameEnergy(frame)
				fmt.Printf("📊 Energy level: %6.3f\n", energy)
			}
		}
	}
}

// CalculateAudioEnergy calculates the average energy across audio frames
func CalculateAudioEnergy(frames []*media.AudioFrame) float64 {
	if len(frames) == 0 {
		return 0.0
	}
	
	totalEnergy := 0.0
	for _, frame := range frames {
		totalEnergy += calculateFrameEnergy(frame)
	}
	
	return totalEnergy / float64(len(frames))
}

