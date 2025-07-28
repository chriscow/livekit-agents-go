// Package silero provides a stub implementation when the silero build tag is not used.
//go:build !silero

package silero

import (
	"fmt"
	
	"github.com/chriscow/livekit-agents-go/pkg/plugin"
)

// Stub factory that returns an error when silero tag is not used.
func newSileroVAD(cfg map[string]any) (any, error) {
	return nil, fmt.Errorf("silero VAD plugin not available (build with -tags=silero)")
}

func init() {
	plugin.RegisterWithMetadata(&plugin.Plugin{
		Kind:        "vad",
		Name:        "silero",
		Factory:     newSileroVAD,
		Description: "Silero VAD (disabled - build with -tags=silero to enable)",
		Version:     "1.0.0",
		Config: map[string]interface{}{
			"note": "This plugin requires -tags=silero build flag",
		},
		Downloader: nil, // No downloader for stub
	})
}