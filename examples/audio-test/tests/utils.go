package tests

import (
	"livekit-agents-go/media"
)

// calculateFrameEnergy calculates the RMS energy of a single audio frame
// This is shared utility function used across multiple test files
func calculateFrameEnergy(frame *media.AudioFrame) float64 {
	if frame == nil || len(frame.Data) < 2 {
		return 0.0
	}
	
	// Convert byte data to int16 samples and calculate RMS
	var sum int64
	sampleCount := len(frame.Data) / 2
	
	for i := 0; i < sampleCount; i++ {
		// Read 16-bit little-endian sample
		sample := int16(frame.Data[i*2]) | (int16(frame.Data[i*2+1]) << 8)
		sum += int64(sample) * int64(sample)
	}
	
	if sampleCount == 0 {
		return 0.0
	}
	
	rms := float64(sum) / float64(sampleCount)
	return rms / (32767.0 * 32767.0) // Normalize to [0, 1]
}