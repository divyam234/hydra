package util

import (
	"sync"
)

// DefaultBufferSize is the standard buffer size for download operations (256KB)
const DefaultBufferSize = 256 * 1024

// BufferPool manages a pool of byte slices to reduce GC pressure
type BufferPool struct {
	pool sync.Pool
	size int
}

var (
	// Global 256KB buffer pool
	bufferPool256k = &BufferPool{
		size: DefaultBufferSize,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, DefaultBufferSize)
			},
		},
	}
)

// GetBuffer returns a buffer from the pool
func GetBuffer() []byte {
	return bufferPool256k.pool.Get().([]byte)
}

// PutBuffer returns a buffer to the pool
func PutBuffer(b []byte) {
	if cap(b) < DefaultBufferSize {
		return // Don't pool buffers that are too small
	}
	// Reset length to DefaultBufferSize if needed, or just slice it?
	// The pool expects buffers of DefaultBufferSize.
	// We should probably reslice to full capacity before putting back?
	b = b[:cap(b)]
	if len(b) != DefaultBufferSize {
		return
	}
	bufferPool256k.pool.Put(b)
}
