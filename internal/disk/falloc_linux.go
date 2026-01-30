//go:build linux

package disk

import (
	"os"
	"syscall"
)

func fallocate(f *os.File, size int64) error {
	// mode=0 (default)
	// offset=0
	return syscall.Fallocate(int(f.Fd()), 0, 0, size)
}
