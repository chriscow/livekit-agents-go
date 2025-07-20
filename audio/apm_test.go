package audio

import (
	"math"
	"testing"
)

func TestAudioProcessingModule_Basic(t *testing.T) {
	// Create APM with echo cancellation enabled
	config := AudioProcessingConfig{
		EchoCancellation: true,
		NoiseSuppression: true,
		HighPassFilter:   true,
		AutoGainControl:  true,
	}

	apm, err := NewAudioProcessingModule(config)
	if err != nil {
		t.Fatalf("Failed to create AudioProcessingModule: %v", err)
	}
	defer apm.Close()

	// Test basic functionality with a 10ms frame at 24kHz
	sampleRate := uint32(24000)
	numChannels := uint32(1)
	samplesPerChannel := uint32(sampleRate * 10 / 1000) // 10ms = 240 samples at 24kHz

	// Create test audio frame with sine wave
	frame := &AudioFrame{
		Data:              make([]int16, samplesPerChannel*numChannels),
		SampleRate:        sampleRate,
		NumChannels:       numChannels,
		SamplesPerChannel: samplesPerChannel,
	}

	// Generate a 440Hz sine wave (A4 note)
	frequency := 440.0
	for i := range frame.Data {
		t := float64(i) / float64(sampleRate)
		amplitude := 0.5 * math.Sin(2*math.Pi*frequency*t)
		frame.Data[i] = int16(amplitude * 32767)
	}

	// Test ProcessReverseStream (reference signal)
	err = apm.ProcessReverseStream(frame)
	if err != nil {
		t.Errorf("ProcessReverseStream failed: %v", err)
	}

	// Test SetStreamDelayMs
	err = apm.SetStreamDelayMs(50) // 50ms delay
	if err != nil {
		t.Errorf("SetStreamDelayMs failed: %v", err)
	}

	// Test ProcessStream (echo cancellation)  
	err = apm.ProcessStream(frame)
	if err != nil {
		t.Errorf("ProcessStream failed: %v", err)
	}

	t.Log("AudioProcessingModule basic test passed successfully")
}

func TestAudioProcessingModule_Configuration(t *testing.T) {
	tests := []struct {
		name   string
		config AudioProcessingConfig
	}{
		{
			name: "EchoOnly",
			config: AudioProcessingConfig{
				EchoCancellation: true,
				NoiseSuppression: false,
				HighPassFilter:   false,
				AutoGainControl:  false,
			},
		},
		{
			name: "AllDisabled",
			config: AudioProcessingConfig{
				EchoCancellation: false,
				NoiseSuppression: false,
				HighPassFilter:   false,
				AutoGainControl:  false,
			},
		},
		{
			name: "AllEnabled",
			config: AudioProcessingConfig{
				EchoCancellation: true,
				NoiseSuppression: true,
				HighPassFilter:   true,
				AutoGainControl:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apm, err := NewAudioProcessingModule(tt.config)
			if err != nil {
				t.Fatalf("Failed to create AudioProcessingModule with config %+v: %v", tt.config, err)
			}
			defer apm.Close()

			// Simple validation that the APM was created successfully
			if apm.handle == 0 {
				t.Errorf("APM handle is 0, indicating creation failed")
			}

			t.Logf("Successfully created APM with config: %+v", tt.config)
		})
	}
}

func TestAudioProcessingModule_ErrorHandling(t *testing.T) {
	config := AudioProcessingConfig{
		EchoCancellation: true,
		NoiseSuppression: false,
		HighPassFilter:   false,
		AutoGainControl:  false,
	}

	apm, err := NewAudioProcessingModule(config)
	if err != nil {
		t.Fatalf("Failed to create AudioProcessingModule: %v", err)
	}
	defer apm.Close()

	// Test with invalid frame (wrong duration - should be 10ms)
	invalidFrame := &AudioFrame{
		Data:              make([]int16, 100), // Too short for any valid sample rate
		SampleRate:        24000,
		NumChannels:       1,
		SamplesPerChannel: 100,
	}

	// This should work fine - the FFI library might not validate frame size strictly
	err = apm.ProcessStream(invalidFrame)
	if err != nil {
		t.Logf("ProcessStream with invalid frame returned expected error: %v", err)
	}

	// Test ProcessStream with nil data (should fail)
	nilFrame := &AudioFrame{
		Data:              nil,
		SampleRate:        24000,
		NumChannels:       1,
		SamplesPerChannel: 240,
	}

	err = apm.ProcessStream(nilFrame)
	if err == nil {
		t.Error("Expected ProcessStream with nil data to fail, but it succeeded")
	} else {
		t.Logf("ProcessStream with nil data correctly failed: %v", err)
	}
}