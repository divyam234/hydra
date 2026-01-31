package util

import (
	"testing"
)

func TestBufferPool(t *testing.T) {
	// Test GetBuffer
	b1 := GetBuffer()
	if len(b1) != DefaultBufferSize {
		t.Errorf("Expected buffer length %d, got %d", DefaultBufferSize, len(b1))
	}
	if cap(b1) != DefaultBufferSize {
		t.Errorf("Expected buffer capacity %d, got %d", DefaultBufferSize, cap(b1))
	}

	// Modify buffer
	b1[0] = 0xFF

	// Test PutBuffer
	PutBuffer(b1)

	// Get buffer again (might be the same one, but sync.Pool doesn't guarantee it)
	// We can't deterministically test reuse with sync.Pool, but we can ensure
	// PutBuffer doesn't panic and accepts valid buffers.

	b2 := GetBuffer()
	if len(b2) != DefaultBufferSize {
		t.Errorf("Expected buffer length %d, got %d", DefaultBufferSize, len(b2))
	}

	// Test putting invalid buffer
	smallBuf := make([]byte, 1024)
	PutBuffer(smallBuf) // Should safe-guard against small buffers
}

func TestBufferPool_Concurrency(t *testing.T) {
	const workers = 10
	done := make(chan bool)

	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				b := GetBuffer()
				if len(b) != DefaultBufferSize {
					t.Errorf("Worker got bad buffer size: %d", len(b))
				}
				PutBuffer(b)
			}
			done <- true
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
	}
}
