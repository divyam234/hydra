package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/divyam234/hydra/pkg/option"
)

// Category 3: Concurrency & Race Conditions

func TestConcurrency_MultipleWorkers(t *testing.T) {
	// Test 10 workers downloading concurrently
	fileSize := 1024 * 1024 // 1MB
	data := make([]byte, fileSize)
	// Fill with pattern
	for i := range data {
		data[i] = byte(i % 256)
	}

	server := setupRangeServer(t, data) // Reusing helper from request_group_test.go (same package)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_conc_workers")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Split, "10")        // 10 connections
	opt.Put(option.MinSplitSize, "1K") // Allow small pieces to force split

	rg := NewRequestGroup("conc-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "index.html"))
	if len(content) != fileSize {
		t.Errorf("Expected %d bytes, got %d", fileSize, len(content))
	}
}

func TestConcurrency_RapidCancelResume(t *testing.T) {
	// Rapidly start and cancel downloads to stress test setup/teardown and control file locking
	data := make([]byte, 1024*100)
	server := setupRangeServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_rapid")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "rapid.dat")
	opt.Put(option.Split, "4")

	var wg sync.WaitGroup
	// Run 10 rapid cycles
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iter int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			rg := NewRequestGroup(GID(fmt.Sprintf("gid-%d", iter)), []string{server.URL}, opt)

			// Cancel quickly after start
			go func() {
				time.Sleep(time.Duration(10+iter*5) * time.Millisecond)
				cancel()
			}()

			_ = rg.Execute(ctx)
		}(i)
	}
	wg.Wait()

	// Finally, do a clean download to ensure system is still stable
	rgFinal := NewRequestGroup("gid-final", []string{server.URL}, opt)
	if err := rgFinal.Execute(context.Background()); err != nil {
		t.Fatalf("Final download failed: %v", err)
	}
}

func TestConcurrency_ProgressCounter(t *testing.T) {
	// Verify completedBytes atomic counter accuracy with concurrent writes
	fileSize := 1024 * 100
	data := make([]byte, fileSize)
	server := setupRangeServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_counter")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Split, "20") // High concurrency
	opt.Put(option.MinSplitSize, "1K")

	rg := NewRequestGroup("counter-gid", []string{server.URL}, opt)
	if err := rg.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}

	if rg.completedBytes.Load() != int64(fileSize) {
		t.Errorf("Atomic counter mismatch: expected %d, got %d", fileSize, rg.completedBytes.Load())
	}
}
