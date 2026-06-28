//go:build !linux

package hydra

import "os"

func preallocateFile(f *os.File, size int64) error {
	if size <= 0 {
		return nil
	}
	return f.Truncate(size)
}
