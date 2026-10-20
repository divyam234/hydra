//go:build !windows

package hydra

import "os"

func syncDir(dir string) error {
	if dir == "" {
		dir = "."
	}
	d, err := os.Open(dir)
	if err != nil {
		return nil
	} // Directory fsync is best effort when opening is unsupported.
	defer d.Close()
	return d.Sync()
}
