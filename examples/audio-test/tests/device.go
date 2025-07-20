package tests

import (
	"fmt"
	"livekit-agents-go/audio"
)

// TestDeviceDiscovery tests audio device discovery and enumeration
func TestDeviceDiscovery(audioIO *audio.LocalAudioIO) error {
	fmt.Println("🔍 Discovering audio devices...")
	
	// Get device information
	err := audioIO.GetDeviceInfo()
	if err != nil {
		return fmt.Errorf("failed to get device info: %w", err)
	}
	
	fmt.Println("✅ Device discovery completed successfully")
	return nil
}