package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func GetVersionInfo() string {
	return fmt.Sprintf("lk-go version %s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildTime, runtime.Version())
}