package benchmark

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	_ "net/http/pprof" // Enable pprof
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/divyam234/hydra/pkg/downloader"
)

// TestStress_SustainedLoad runs a sustained load test with high concurrency.
// Run with: go test -v -timeout 15m -run TestStress_SustainedLoad ./test/benchmark
func TestStress_SustainedLoad(t *testing.T) {
	// Configuration
	// Adjust these for local testing vs CI
	var (
		concurrency = 50
		fileSize    = int64(5 * 1024 * 1024) // 5MB
		duration    = 30 * time.Second       // Default short duration for CI
	)

	// Check for environment variable to run longer
	if os.Getenv("HYDRA_STRESS_DURATION") != "" {
		if d, err := time.ParseDuration(os.Getenv("HYDRA_STRESS_DURATION")); err == nil {
			duration = d
		}
	}

	// Start pprof server
	go func() {
		fmt.Println("Starting pprof on :6060")
		http.ListenAndServe("localhost:6060", nil)
	}()

	fmt.Printf("Starting stress test: %d concurrent downloads, %v duration, %d bytes per file\n",
		concurrency, duration, fileSize)

	// Setup Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
		w.Header().Set("Content-Type", "application/octet-stream")

		// Write dummy data (zeros) efficiently
		// We don't allocate a 5MB buffer, we reuse a small one
		chunk := make([]byte, 32*1024)
		for i := int64(0); i < fileSize; i += int64(len(chunk)) {
			remaining := fileSize - i
			if remaining < int64(len(chunk)) {
				w.Write(chunk[:remaining])
			} else {
				w.Write(chunk)
			}
		}
	}))
	defer ts.Close()

	// Temp Dir
	tmpDir, err := os.MkdirTemp("", "hydra-stress")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Stats tracking
	var (
		added     int
		completed int
		failed    int
		mu        sync.Mutex
		fileMap   sync.Map // Map[DownloadID]string (filename)
	)

	// Create Engine
	// We use one engine to test the queuing and resource management
	eng := downloader.NewEngine(
		downloader.WithDir(tmpDir),
		downloader.WithMaxConcurrentDownloads(concurrency),
		downloader.WithQuiet(true),
		// Tuned performance settings
		downloader.WithReadBufferSize("256K"),
		downloader.WithWriteBufferSize("64K"),
		downloader.WithMaxIdleConns(concurrency*5), // Ensure enough idle conns
		downloader.WithIdleConnTimeout(60),
		downloader.OnEvent(func(e downloader.Event) {
			if e.Type == downloader.EventComplete {
				mu.Lock()
				completed++
				mu.Unlock()
				// Cleanup
				if filename, ok := fileMap.Load(e.ID); ok {
					_ = os.Remove(filepath.Join(tmpDir, filename.(string)))
					fileMap.Delete(e.ID)
				}
			} else if e.Type == downloader.EventError {
				mu.Lock()
				failed++
				mu.Unlock()
				// Also cleanup failed files
				if _, ok := fileMap.Load(e.ID); ok {
					fileMap.Delete(e.ID)
				}
			}
		}),
	)
	defer eng.Shutdown()

	// Monitor Goroutine
	stopMonitor := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopMonitor:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				mu.Lock()
				a, c, f := added, completed, failed
				mu.Unlock()
				fmt.Printf("[%s] Added: %d, Completed: %d, Failed: %d, Goroutines: %d, Heap: %d MB\n",
					time.Now().Format("15:04:05"), a, c, f, runtime.NumGoroutine(), m.HeapAlloc/1024/1024)
			}
		}
	}()

	// Load Generator
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// We need to keep adding downloads to keep the queue full.
	// Since AddDownload is non-blocking, we can just flood it, but we don't want to OOM
	// by adding 1 million items to the queue immediately.
	// We'll use a semaphore or ticker to pace additions, or check Active status?
	// The Engine doesn't expose queue size easily via public API for polling.
	// We'll just add a batch, sleep, add another batch.

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Add a batch
				for i := 0; i < 10; i++ {
					select {
					case <-ctx.Done():
						return
					default:
						filename := fmt.Sprintf("file_%d_%d.bin", time.Now().UnixNano(), i)
						id, err := eng.AddDownload(context.Background(), []string{ts.URL},
							downloader.WithFilename(filename),
							downloader.WithSplit(4), // 4 connections per file
						)

						mu.Lock()
						if err != nil {
							// If queue is full (though AddDownload doesn't usually block/fail on queue full unless limited),
							// we might see errors.
							fmt.Printf("Failed to add download: %v\n", err)
						} else {
							added++
							fileMap.Store(id, filename)
						}
						mu.Unlock()
					}
				}
				// Sleep a bit to roughly match download speed
				// 5MB file at 100MB/s (local) ~ 0.05s. 50 concurrent ~ 400MB/s?
				// We don't want to overflow memory with pending download objects.
				// Let's sleep 100ms.
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// We need to wait for the duration to complete
	<-ctx.Done()
	close(stopMonitor)

	fmt.Println("Stress test duration reached. Waiting for active downloads to finish (with 10s timeout)...")

	// Give a little time for cleanup, but we won't wait for ALL pending to finish
	// because we might have added thousands.
	// Actually, eng.Shutdown() will be called on defer.

	mu.Lock()
	fmt.Printf("Final Stats - Added: %d, Completed (Approx): %d\n", added, completed)
	mu.Unlock()
}
