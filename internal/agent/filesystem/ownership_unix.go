//go:build !windows

package filesystem

import (
	"io/fs"
	"os/user"
	"strconv"
	"syscall"
)

func ownership(info fs.FileInfo) (string, string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ""
	}
	owner := strconv.FormatUint(uint64(stat.Uid), 10)
	group := strconv.FormatUint(uint64(stat.Gid), 10)
	if value, err := user.LookupId(owner); err == nil {
		owner = value.Username
	}
	if value, err := user.LookupGroupId(group); err == nil {
		group = value.Name
	}
	return owner, group
}
