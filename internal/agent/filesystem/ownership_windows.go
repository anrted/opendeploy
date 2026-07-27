//go:build windows

package filesystem

import "io/fs"

func ownership(fs.FileInfo) (string, string) {
	return "", ""
}
