package voice

import (
	"sync"
	"testing"
)

func TestNewAudioGate(t *testing.T) {
	gate := NewAudioGate()

	// Initially should not discard audio
	if gate.ShouldDiscardAudio() {
		t.Error("NewAudioGate() should initially not discard audio")
	}

	// Set TTS playing
	gate.SetTTSPlaying(true)
	if !gate.ShouldDiscardAudio() {
		t.Error("Should discard audio when TTS is playing")
	}

	// Set TTS not playing
	gate.SetTTSPlaying(false)
	if gate.ShouldDiscardAudio() {
		t.Error("Should not discard audio when TTS is not playing")
	}
}

func TestAudioGateConcurrency(t *testing.T) {
	gate := NewAudioGate()
	
	// Test concurrent access
	var wg sync.WaitGroup
	
	// Start multiple goroutines that set TTS playing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(playing bool) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				gate.SetTTSPlaying(playing)
			}
		}(i%2 == 0)
	}
	
	// Start multiple goroutines that check audio discard
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = gate.ShouldDiscardAudio()
			}
		}()
	}
	
	wg.Wait()
	
	// Should not crash or race
}