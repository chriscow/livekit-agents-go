package rtc

import (
	"testing"
	"time"
)

func TestNewAudioFrame(t *testing.T) {
	tests := []struct {
		name        string
		sampleRate  int
		numChannels int
		dataLen     int
		shouldPanic bool
	}{
		{
			name:        "valid 48kHz mono",
			sampleRate:  48000,
			numChannels: 1,
			dataLen:     960, // 48000/100 * 1 * 2
			shouldPanic: false,
		},
		{
			name:        "valid 16kHz mono",
			sampleRate:  16000,
			numChannels: 1,
			dataLen:     320, // 16000/100 * 1 * 2
			shouldPanic: false,
		},
		{
			name:        "valid 48kHz stereo",
			sampleRate:  48000,
			numChannels: 2,
			dataLen:     1920, // 48000/100 * 2 * 2
			shouldPanic: false,
		},
		{
			name:        "invalid data length",
			sampleRate:  48000,
			numChannels: 1,
			dataLen:     500,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			timestamp := 100 * time.Millisecond

			frame, err := NewAudioFrame(data, tt.sampleRate, tt.numChannels, timestamp)

			if tt.shouldPanic {
				if err == nil {
					t.Errorf("NewAudioFrame() should have returned an error but didn't")
				}
				return // Skip validation for error cases
			}

			if err != nil {
				t.Errorf("NewAudioFrame() unexpected error: %v", err)
				return
			}

			if frame != nil {
				if frame.SampleRate != tt.sampleRate {
					t.Errorf("SampleRate = %d, want %d", frame.SampleRate, tt.sampleRate)
				}
				if frame.NumChannels != tt.numChannels {
					t.Errorf("NumChannels = %d, want %d", frame.NumChannels, tt.numChannels)
				}
				if frame.SamplesPerChannel != tt.sampleRate/100 {
					t.Errorf("SamplesPerChannel = %d, want %d", frame.SamplesPerChannel, tt.sampleRate/100)
				}
				if frame.Timestamp != timestamp {
					t.Errorf("Timestamp = %v, want %v", frame.Timestamp, timestamp)
				}
				if len(frame.Data) != tt.dataLen {
					t.Errorf("Data length = %d, want %d", len(frame.Data), tt.dataLen)
				}
			}
		})
	}
}

func TestAudioFrameClone(t *testing.T) {
	data := make([]byte, 320)
	for i := range data {
		data[i] = byte(i % 256)
	}

	original, err := NewAudioFrame(data, 16000, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewAudioFrame() error = %v", err)
	}
	clone := original.Clone()

	// Verify clone has same values
	if clone.SampleRate != original.SampleRate {
		t.Errorf("Clone SampleRate = %d, want %d", clone.SampleRate, original.SampleRate)
	}
	if clone.NumChannels != original.NumChannels {
		t.Errorf("Clone NumChannels = %d, want %d", clone.NumChannels, original.NumChannels)
	}
	if clone.SamplesPerChannel != original.SamplesPerChannel {
		t.Errorf("Clone SamplesPerChannel = %d, want %d", clone.SamplesPerChannel, original.SamplesPerChannel)
	}
	if clone.Timestamp != original.Timestamp {
		t.Errorf("Clone Timestamp = %v, want %v", clone.Timestamp, original.Timestamp)
	}

	// Verify data is copied (not same slice)
	if &clone.Data[0] == &original.Data[0] {
		t.Error("Clone data points to same memory as original")
	}

	// Verify data content is identical
	for i, b := range clone.Data {
		if b != original.Data[i] {
			t.Errorf("Clone data[%d] = %d, want %d", i, b, original.Data[i])
		}
	}

	// Verify modifying clone doesn't affect original
	clone.Data[0] = 255
	if original.Data[0] == 255 {
		t.Error("Modifying clone data affected original")
	}
}

func TestAudioFrameDuration(t *testing.T) {
	frame := &AudioFrame{}
	duration := frame.Duration()
	expected := 10 * time.Millisecond

	if duration != expected {
		t.Errorf("Duration() = %v, want %v", duration, expected)
	}
}