package disk

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiskAdaptorAllocation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hydra-disk-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		allocType AllocationType
		size      int64
		expectLen int64 // For 'none', length might be 0 until written
	}{
		{"Truncate", AllocTrunc, 1024 * 1024, 1024 * 1024},
		{"None", AllocNone, 1024 * 1024, 0},
		// Falloc test depends on OS, might fallback or succeed.
		// We expect at least the file size to be set if it works, or fallback to trunc.
		{"Falloc", AllocFalloc, 1024 * 1024, 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tempDir, "test-"+tt.name)
			d := NewDirectDiskAdaptor(string(tt.allocType))
			if err := d.Open(path, tt.size); err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			defer d.Close()

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("Stat failed: %v", err)
			}

			if tt.allocType == AllocNone {
				if info.Size() != 0 {
					t.Errorf("Expected size 0 for None, got %d", info.Size())
				}
			} else {
				if info.Size() != tt.size {
					t.Errorf("Expected size %d, got %d", tt.size, info.Size())
				}
			}
		})
	}
}

func TestBufferedDiskAdaptor(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hydra-buffered-disk-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, "test-buffered.bin")
	size := int64(1024 * 1024)

	d := NewBufferedDiskAdaptor("trunc")
	if err := d.Open(path, size); err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Write pattern
	data := []byte("hello world")
	offset := int64(100)

	n, err := d.WriteAt(data, offset)
	if err != nil {
		t.Fatalf("WriteAt failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Write more data
	data2 := []byte("buffered data")
	offset2 := int64(200000)
	d.WriteAt(data2, offset2)

	// Close to flush
	if err := d.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify file content
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Check size
	info, _ := f.Stat()
	if info.Size() != size {
		t.Errorf("Expected size %d, got %d", size, info.Size())
	}

	// Read back first chunk
	buf := make([]byte, len(data))
	if _, err := f.ReadAt(buf, offset); err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if string(buf) != string(data) {
		t.Errorf("Expected %q, got %q", string(data), string(buf))
	}

	// Read back second chunk
	buf2 := make([]byte, len(data2))
	if _, err := f.ReadAt(buf2, offset2); err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if string(buf2) != string(data2) {
		t.Errorf("Expected %q, got %q", string(data2), string(buf2))
	}
}
