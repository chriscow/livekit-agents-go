// Stub implementation for dynamic plugin loading when not supported.
//go:build !plugindyn || !linux

package plugin

import (
	"fmt"
)

// LoadDynamicPlugins returns an error indicating dynamic loading is not supported.
func LoadDynamicPlugins(pluginDir string) error {
	return fmt.Errorf("dynamic plugin loading not supported on this platform or build configuration (use -tags=plugindyn on Linux)")
}