package rtc

import (
	"fmt"
	"time"
)

// AudioFrame represents exactly 10 ms of PCM audio.
// Len(Data) == SamplesPerChannel * NumChannels * 2.
// All fields are immutable after creation except Data when processed in-place.
//
// A zero Timestamp means "live"; otherwise it points to absolute wall-clock.
type AudioFrame struct {
	Data              []byte        // 16-bit PCM, little-endian
	SampleRate        int           // 48 000 or 16 000
	SamplesPerChannel int           // SampleRate / 100
	NumChannels       int           // 1 or 2
	Timestamp         time.Duration // optional
}

// NewAudioFrame creates a new AudioFrame with the specified parameters.
// Data length is validated to match SamplesPerChannel * NumChannels * 2.
// Returns an error if the data length doesn't match the expected size for 10ms of audio.
func NewAudioFrame(data []byte, sampleRate, numChannels int, timestamp time.Duration) (*AudioFrame, error) {
	samplesPerChannel := sampleRate / 100
	expectedLen := samplesPerChannel * numChannels * 2
	
	if len(data) != expectedLen {
		return nil, fmt.Errorf("AudioFrame data length mismatch: got %d bytes, expected %d bytes for %dHz %d-channel 10ms audio", 
			len(data), expectedLen, sampleRate, numChannels)
	}
	
	return &AudioFrame{
		Data:              data,
		SampleRate:        sampleRate,
		SamplesPerChannel: samplesPerChannel,
		NumChannels:       numChannels,
		Timestamp:         timestamp,
	}, nil
}

// Clone creates a deep copy of the AudioFrame.
func (f *AudioFrame) Clone() *AudioFrame {
	data := make([]byte, len(f.Data))
	copy(data, f.Data)
	
	return &AudioFrame{
		Data:              data,
		SampleRate:        f.SampleRate,
		SamplesPerChannel: f.SamplesPerChannel,
		NumChannels:       f.NumChannels,
		Timestamp:         f.Timestamp,
	}
}

// Duration returns the duration represented by this frame (always 10ms).
func (f *AudioFrame) Duration() time.Duration {
	return 10 * time.Millisecond
}