//go:build !linux

package stats

import "fmt"

// statfsResult stubs for non-Linux builds (development on Windows/macOS).
type statfsResult struct {
	Bsize  int64
	Blocks uint64
	Bavail uint64
	Bfree  uint64
}

func statfs(path string, r *statfsResult) error {
	return fmt.Errorf("statfs: not supported on this platform")
}
