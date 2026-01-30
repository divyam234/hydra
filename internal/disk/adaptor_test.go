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
