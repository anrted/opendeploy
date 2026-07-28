//go:build !windows

package main

import "os"

func isRoot() bool { return os.Geteuid() == 0 }
