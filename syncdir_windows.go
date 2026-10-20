//go:build windows

package hydra

// Windows FlushFileBuffers requires a writable handle. Go's os.Open directory
// handle is read-only, so syncing it fails and would reject every saved sidecar.
// File contents are still synced; directory durability is best effort here.
func syncDir(_ string) error { return nil }
