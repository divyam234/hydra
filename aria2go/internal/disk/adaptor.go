package disk

import (
	"fmt"
	"os"
	"sync"
)

// DiskAdaptor handles file I/O
type DiskAdaptor interface {
	Open(path string, totalLength int64) error
	WriteAt(p []byte, off int64) (int, error)
	Close() error
}

// DirectDiskAdaptor writes directly to a file
type DirectDiskAdaptor struct {
	file *os.File
	path string
	mu   sync.Mutex
}

// NewDirectDiskAdaptor creates a new DirectDiskAdaptor
func NewDirectDiskAdaptor() *DirectDiskAdaptor {
	return &DirectDiskAdaptor{}
}

// Open opens the file and pre-allocates space if needed
func (d *DirectDiskAdaptor) Open(path string, totalLength int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.path = path

	// Open for reading and writing, create if not exists
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	d.file = f

	// Pre-allocate space (simple truncate for now)
	if totalLength > 0 {
		if err := d.file.Truncate(totalLength); err != nil {
			d.file.Close()
			return fmt.Errorf("failed to allocate space: %w", err)
		}
	}

	return nil
}

// WriteAt writes data to the file at the given offset
func (d *DirectDiskAdaptor) WriteAt(p []byte, off int64) (int, error) {
	// Pwrite is thread-safe on POSIX, but os.File.WriteAt uses pread/pwrite
	// so it should be safe without a mutex lock for concurrent writes to different regions.
	// However, let's keep it safe.
	return d.file.WriteAt(p, off)
}

// Close closes the file
func (d *DirectDiskAdaptor) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}
