package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetVersionInfo(t *testing.T) {
	// Test with default values
	info := GetVersionInfo()

	if !strings.Contains(info, "lk-go version") {
		t.Error("version info should contain 'lk-go version'")
	}

	if !strings.Contains(info, "dev") {
		t.Error("version info should contain default version 'dev'")
	}

	if !strings.Contains(info, "unknown") {
		t.Error("version info should contain 'unknown' for commit and build time")
	}

	if !strings.Contains(info, runtime.Version()) {
		t.Errorf("version info should contain Go version %s", runtime.Version())
	}
}

func TestGetVersionInfoWithCustomValues(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildTime := BuildTime

	// Set test values
	Version = "v1.0.0"
	GitCommit = "abc123"
	BuildTime = "2024-01-01T00:00:00Z"

	// Restore original values after test
	defer func() {
		Version = originalVersion
		GitCommit = originalCommit
		BuildTime = originalBuildTime
	}()

	info := GetVersionInfo()

	if !strings.Contains(info, "v1.0.0") {
		t.Error("version info should contain custom version")
	}

	if !strings.Contains(info, "abc123") {
		t.Error("version info should contain custom commit")
	}

	if !strings.Contains(info, "2024-01-01T00:00:00Z") {
		t.Error("version info should contain custom build time")
	}
}