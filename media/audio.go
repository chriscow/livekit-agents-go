package media

import (
	"fmt"
	"time"
)

// AudioFormat represents audio format information
type AudioFormat struct {
	SampleRate   int
	Channels     int
	BitsPerSample int
	Format       AudioFormatType
}

type AudioFormatType int

const (
	AudioFormatPCM AudioFormatType = iota
	AudioFormatFloat32
	AudioFormatFloat64
	AudioFormatOgg
	AudioFormatMP3
	AudioFormatWAV
)

// AudioFrame represents a frame of audio data
type AudioFrame struct {
	Data      []byte
	Format    AudioFormat
	Timestamp time.Time
	Duration  time.Duration
	Metadata  map[string]interface{}
}

// NewAudioFrame creates a new audio frame
func NewAudioFrame(data []byte, format AudioFormat) *AudioFrame {
	return &AudioFrame{
		Data:      data,
		Format:    format,
		Timestamp: time.Now(),
		Duration:  calculateDuration(len(data), format),
		Metadata:  make(map[string]interface{}),
	}
}

// Clone creates a deep copy of the audio frame
func (af *AudioFrame) Clone() *AudioFrame {
	data := make([]byte, len(af.Data))
	copy(data, af.Data)
	
	metadata := make(map[string]interface{})
	for k, v := range af.Metadata {
		metadata[k] = v
	}
	
	return &AudioFrame{
		Data:      data,
		Format:    af.Format,
		Timestamp: af.Timestamp,
		Duration:  af.Duration,
		Metadata:  metadata,
	}
}

// SampleCount returns the number of audio samples in the frame
func (af *AudioFrame) SampleCount() int {
	bytesPerSample := af.Format.BitsPerSample / 8
	return len(af.Data) / (bytesPerSample * af.Format.Channels)
}

// IsEmpty returns true if the frame contains no audio data
func (af *AudioFrame) IsEmpty() bool {
	return len(af.Data) == 0
}

// String returns a string representation of the audio frame
func (af *AudioFrame) String() string {
	return fmt.Sprintf("AudioFrame{samples=%d, format=%+v, duration=%v}",
		af.SampleCount(), af.Format, af.Duration)
}

// calculateDuration calculates the duration of audio data
func calculateDuration(dataLen int, format AudioFormat) time.Duration {
	if format.SampleRate == 0 {
		return 0
	}
	
	bytesPerSample := format.BitsPerSample / 8
	samples := dataLen / (bytesPerSample * format.Channels)
	seconds := float64(samples) / float64(format.SampleRate)
	
	return time.Duration(seconds * float64(time.Second))
}

// Common audio formats
var (
	// Standard 16-bit PCM at 48kHz mono
	AudioFormat48kHz16BitMono = AudioFormat{
		SampleRate:    48000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        AudioFormatPCM,
	}
	
	// Standard 16-bit PCM at 48kHz stereo
	AudioFormat48kHz16BitStereo = AudioFormat{
		SampleRate:    48000,
		Channels:      2,
		BitsPerSample: 16,
		Format:        AudioFormatPCM,
	}
	
	// Standard 16-bit PCM at 16kHz mono (common for speech)
	AudioFormat16kHz16BitMono = AudioFormat{
		SampleRate:    16000,
		Channels:      1,
		BitsPerSample: 16,
		Format:        AudioFormatPCM,
	}
)