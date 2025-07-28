// Dynamic plugin loading support for Go's plugin system.
// This is only available on Linux and requires the plugindyn build tag.
//go:build plugindyn && linux

package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"plugin"
	"strings"
)

// LoadDynamicPlugins loads .so plugins from the specified directory.
// If no directory is specified, it uses the LK_PLUGIN_PATH environment variable
// or defaults to /usr/local/lib/livekit-agents/plugins.
func LoadDynamicPlugins(pluginDir string) error {
	if pluginDir == "" {
		pluginDir = os.Getenv("LK_PLUGIN_PATH")
		if pluginDir == "" {
			pluginDir = "/usr/local/lib/livekit-agents/plugins"
		}
	}

	// Check if plugin directory exists
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		// Not an error - just no plugins to load
		return nil
	}

	// Find all .so files in the plugin directory
	soFiles, err := filepath.Glob(filepath.Join(pluginDir, "*.so"))
	if err != nil {
		return fmt.Errorf("failed to search for plugin files in %s: %w", pluginDir, err)
	}

	loadedCount := 0
	for _, soFile := range soFiles {
		if err := loadPlugin(soFile); err != nil {
			return fmt.Errorf("failed to load plugin %s: %w", soFile, err)
		}
		loadedCount++
	}

	if loadedCount > 0 {
		slog.Info("Loaded dynamic plugins", 
			slog.Int("count", loadedCount), 
			slog.String("directory", pluginDir))
	}

	return nil
}

// loadPlugin loads a single .so plugin file.
func loadPlugin(soFile string) error {
	// Load the plugin
	p, err := plugin.Open(soFile)
	if err != nil {
		return fmt.Errorf("failed to open plugin file: %w", err)
	}

	// Look for the standard registration function
	initFunc, err := p.Lookup("RegisterPlugins")
	if err != nil {
		return fmt.Errorf("plugin does not export RegisterPlugins function: %w", err)
	}

	// Call the registration function
	registerFunc, ok := initFunc.(func() error)
	if !ok {
		return fmt.Errorf("RegisterPlugins function has invalid signature")
	}

	if err := registerFunc(); err != nil {
		return fmt.Errorf("plugin registration failed: %w", err)
	}

	pluginName := strings.TrimSuffix(filepath.Base(soFile), ".so")
	slog.Info("Successfully loaded plugin", slog.String("name", pluginName), slog.String("file", soFile))

	return nil
}