//go:build linux

package hydra

import (
	"errors"
	"os"
	"syscall"
)

func preallocateFile(f *os.File, size int64) error {
	if size <= 0 {
		return nil
	}
	if err := syscall.Fallocate(int(f.Fd()), 0, 0, size); err != nil {
		if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EINVAL) {
			return f.Truncate(size)
		}
		return err
	}
	return nil
}
