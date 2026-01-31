//go:build linux

package disk

import (
	"os"

	"golang.org/x/sys/unix"
)

func fallocate(f *os.File, size int64) error {
	// mode=0 (default)
	// offset=0
	return unix.Fallocate(int(f.Fd()), 0, 0, size)
}

// ApplyDiskOptimizations applies OS-specific optimizations to the file
func ApplyDiskOptimizations(f *os.File) {
	// FADV_SEQUENTIAL: Expect sequential access
	unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_SEQUENTIAL)
}
