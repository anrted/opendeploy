// Package version provides build-time version information for OpenDeploy.
package version

import "fmt"

// These variables are populated at build time via -ldflags.
var (
	// Version is the semantic version of the binary.
	Version = "dev"
	// BuildTime is the UTC time when the binary was built.
	BuildTime = "unknown"
)

// Info returns a human-readable version string.
func Info() string {
	return fmt.Sprintf("OpenDeploy %s (built %s)", Version, BuildTime)
}
