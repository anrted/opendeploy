//go:build windows

package filesystem

// Windows does not support fsync on directory handles opened by os.Open.
// The file itself is still flushed before the atomic rename.
func syncDirectory(string) error {
	return nil
}
