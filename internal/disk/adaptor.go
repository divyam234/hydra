package disk

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/divyam234/hydra/internal/util"
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

	// Apply OS-specific optimizations (e.g., FADV_SEQUENTIAL on Linux)
	ApplyDiskOptimizations(d.file)

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

// BufferedDiskAdaptor writes to file asynchronously
type BufferedDiskAdaptor struct {
	adaptor *DirectDiskAdaptor
	writeCh chan writeRequest
	errorCh chan error
	wg      sync.WaitGroup
	closed  atomic.Bool
}

type writeRequest struct {
	data   []byte
	offset int64
}

// NewBufferedDiskAdaptor creates a new BufferedDiskAdaptor
func NewBufferedDiskAdaptor(allocType string) *BufferedDiskAdaptor {
	return &BufferedDiskAdaptor{
		adaptor: NewDirectDiskAdaptor(allocType),
		writeCh: make(chan writeRequest, 64), // Buffer 64 chunks (approx 16MB with 256KB chunks)
		errorCh: make(chan error, 1),
	}
}

func (b *BufferedDiskAdaptor) Open(path string, totalLength int64) error {
	if err := b.adaptor.Open(path, totalLength); err != nil {
		return err
	}

	b.wg.Add(1)
	go b.writerLoop()
	return nil
}

func (b *BufferedDiskAdaptor) writerLoop() {
	defer b.wg.Done()
	for req := range b.writeCh {
		_, err := b.adaptor.WriteAt(req.data, req.offset)
		if err != nil {
			select {
			case b.errorCh <- err:
			default:
			}
		}
		// Return buffer to pool
		util.PutBuffer(req.data)
	}
}

func (b *BufferedDiskAdaptor) WriteAt(p []byte, off int64) (int, error) {
	if b.closed.Load() {
		return 0, fmt.Errorf("file closed")
	}

	select {
	case err := <-b.errorCh:
		return 0, err
	default:
	}

	// Copy data to ensure safety as p is reused by caller.
	// Use buffer pool to reduce GC pressure.
	var dataCopy []byte
	if len(p) <= util.DefaultBufferSize {
		dataCopy = util.GetBuffer()
	} else {
		// Fallback for unusually large chunks
		dataCopy = make([]byte, len(p))
	}

	copy(dataCopy, p)
	// Slice to actual length
	toQueue := dataCopy[:len(p)]

	b.writeCh <- writeRequest{data: toQueue, offset: off}

	return len(p), nil
}

func (b *BufferedDiskAdaptor) Close() error {
	if b.closed.CompareAndSwap(false, true) {
		close(b.writeCh)
		b.wg.Wait()

		// Check for any final errors
		select {
		case err := <-b.errorCh:
			b.adaptor.Close()
			return err
		default:
		}

		return b.adaptor.Close()
	}
	return nil
}
