package disk

import (
	"fmt"
	"os"
	"sync"
)

// AllocationType defines the file allocation strategy
type AllocationType string

const (
	AllocTrunc  AllocationType = "trunc"  // ftruncate (fast, sparse)
	AllocFalloc AllocationType = "falloc" // fallocate (pre-allocate blocks)
	AllocNone   AllocationType = "none"   // No pre-allocation
)

// DiskAdaptor handles file I/O
type DiskAdaptor interface {
	Open(path string, totalLength int64) error
	WriteAt(p []byte, off int64) (int, error)
	Close() error
}

// DirectDiskAdaptor writes directly to a file
type DirectDiskAdaptor struct {
	file      *os.File
	path      string
	allocType AllocationType
	mu        sync.Mutex
}

// NewDirectDiskAdaptor creates a new DirectDiskAdaptor
func NewDirectDiskAdaptor(allocType string) *DirectDiskAdaptor {
	if allocType == "" {
		allocType = string(AllocTrunc)
	}
	return &DirectDiskAdaptor{
		allocType: AllocationType(allocType),
	}
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

	// Pre-allocate space
	if totalLength > 0 {
		switch d.allocType {
		case AllocNone:
			// Do nothing
		case AllocFalloc:
			if err := fallocate(d.file, totalLength); err != nil {
				// Fallback to truncate if fallocate fails
				// fmt.Printf("fallocate failed: %v, falling back to truncate\n", err)
				if err := d.file.Truncate(totalLength); err != nil {
					d.file.Close()
					return fmt.Errorf("failed to allocate space (fallback): %w", err)
				}
			}
		case AllocTrunc:
			fallthrough
		default:
			if err := d.file.Truncate(totalLength); err != nil {
				d.file.Close()
				return fmt.Errorf("failed to allocate space: %w", err)
			}
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
