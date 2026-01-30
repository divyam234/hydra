//go:build !linux

package disk

import (
	"fmt"
	"os"
)

func fallocate(f *os.File, size int64) error {
	return fmt.Errorf("fallocate not supported on this OS")
}
