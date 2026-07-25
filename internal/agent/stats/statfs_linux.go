//go:build linux

package stats

import "syscall"

// statfsResult mirrors the fields we need from syscall.Statfs_t.
type statfsResult struct {
	Bsize  int64
	Blocks uint64
	Bavail uint64
	Bfree  uint64
}

// statfs wraps syscall.Statfs to get disk usage.
func statfs(path string, r *statfsResult) error {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return err
	}
	r.Bsize = s.Bsize
	r.Blocks = s.Blocks
	r.Bavail = s.Bavail
	r.Bfree = s.Bfree
	return nil
}
