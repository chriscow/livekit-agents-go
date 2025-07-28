package audio

import "testing"

func TestNewProcessorConfig(t *testing.T) {
	config := NewProcessorConfig()

	// Should have all features enabled by default
	if !config.EchoCancellation {
		t.Error("Expected EchoCancellation to be enabled by default")
	}
	if !config.NoiseSuppression {
		t.Error("Expected NoiseSuppression to be enabled by default")
	}
	if !config.HighPassFilter {
		t.Error("Expected HighPassFilter to be enabled by default")
	}
	if !config.AutoGainControl {
		t.Error("Expected AutoGainControl to be enabled by default")
	}
}

func TestNewProcessorConfigDisabled(t *testing.T) {
	config := NewProcessorConfigDisabled()

	// Should have all features disabled
	if config.EchoCancellation {
		t.Error("Expected EchoCancellation to be disabled")
	}
	if config.NoiseSuppression {
		t.Error("Expected NoiseSuppression to be disabled")
	}
	if config.HighPassFilter {
		t.Error("Expected HighPassFilter to be disabled")
	}
	if config.AutoGainControl {
		t.Error("Expected AutoGainControl to be disabled")
	}
}

func TestProcessorConfigChaining(t *testing.T) {
	// Test fluent API chaining
	config := NewProcessorConfig().
		WithEchoCancellation(false).
		WithNoiseSuppression(true).
		WithHighPassFilter(false).
		WithAutoGainControl(true)

	if config.EchoCancellation {
		t.Error("Expected EchoCancellation to be disabled")
	}
	if !config.NoiseSuppression {
		t.Error("Expected NoiseSuppression to be enabled")
	}
	if config.HighPassFilter {
		t.Error("Expected HighPassFilter to be disabled")
	}
	if !config.AutoGainControl {
		t.Error("Expected AutoGainControl to be enabled")
	}
}

func TestProcessorConfigImmutable(t *testing.T) {
	// Test that With* methods return copies
	original := NewProcessorConfig()
	modified := original.WithEchoCancellation(false)

	// Original should be unchanged
	if !original.EchoCancellation {
		t.Error("Original config should not be modified")
	}

	// Modified should have the change
	if modified.EchoCancellation {
		t.Error("Modified config should have EchoCancellation disabled")
	}
}