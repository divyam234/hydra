package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bhunter/aria2go/pkg/option"
)

// Helper to create a standard range server
func setupResumeTestServer(t *testing.T, data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")

		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Write(data)
			return
		}

		var start, end int64
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
		if err != nil {
			fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			end = int64(len(data)) - 1
		}
		if end >= int64(len(data)) {
			end = int64(len(data)) - 1
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
}

// Helper for throttled server to ensure we can interrupt
func setupThrottledServer(t *testing.T, data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")

		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Write(data)
			return
		}

		var start, end int64
		fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
		if end == 0 {
			end = int64(len(data)) - 1
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Send byte by byte with delay
		for i := start; i <= end; i++ {
			time.Sleep(1 * time.Millisecond) // Slow enough to catch
			w.Write([]byte{data[i]})
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
}

func TestResume_CorruptedControlFile(t *testing.T) {
	data := []byte("hello world")
	server := setupResumeTestServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_corrupt")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "file.dat")

	// Create a corrupted control file
	controlPath := filepath.Join(tmpDir, "file.dat.aria2")
	os.WriteFile(controlPath, []byte("NOT JSON DATA"), 0644)

	rg := NewRequestGroup("corrupt-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Should ignore corrupted control file and restart: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "file.dat"))
	if string(content) != string(data) {
		t.Error("Download failed with corrupted control file")
	}
}

func TestResume_MismatchedSize(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_size_change")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "change.dat")
	opt.Put(option.Split, "1") // Force single connection

	var serverLen int64 = 100
	var serverData = make([]byte, 200) // Max size

	// Use atomic to safely share serverLen between test and server handler
	var atomicLen atomic.Int64
	atomicLen.Store(serverLen)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentLen := atomicLen.Load()
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", currentLen))
			w.WriteHeader(http.StatusOK)
			return
		}

		if currentLen == 100 {
			w.Header().Set("Content-Length", "100")
			w.Header().Set("Content-Range", "bytes 0-99/100")
			w.WriteHeader(http.StatusPartialContent)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(100 * time.Millisecond) // Wait to be cancelled
			w.Write(serverData[:50])           // Send partial
			return
		}

		rangeHeader := r.Header.Get("Range")
		var start, end int64
		fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)

		if rangeHeader == "" || start == 0 {
			w.Header().Set("Content-Length", "200")
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-199/200"))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(serverData)
		} else {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		}
	}))
	defer server.Close()

	// 1. Start and Cancel
	ctx, cancel := context.WithCancel(context.Background())
	rg1 := NewRequestGroup("gid-size", []string{server.URL}, opt)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	rg1.Execute(ctx)

	// Verify control file matches 100 bytes
	if _, err := os.Stat(filepath.Join(tmpDir, "change.dat.aria2")); os.IsNotExist(err) {
		t.Fatal("Control file missing")
	}

	// 2. Change server size safely
	atomicLen.Store(200)

	// 3. Resume
	rg2 := NewRequestGroup("gid-size", []string{server.URL}, opt)
	err := rg2.Execute(context.Background())
	if err != nil {
		t.Fatalf("Failed to handle size mismatch: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "change.dat"))
	if len(content) != 200 {
		t.Errorf("Expected 200 bytes after size change, got %d", len(content))
	}
}

func TestResume_FileDeleted(t *testing.T) {
	data := make([]byte, 1024)
	server := setupThrottledServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_deleted")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "del.dat")
	opt.Put(option.Split, "2")

	// 1. Create initial state
	ctx, cancel := context.WithCancel(context.Background())
	rg1 := NewRequestGroup("gid-del", []string{server.URL}, opt)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	rg1.Execute(ctx)

	// Verify control file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "del.dat.aria2")); os.IsNotExist(err) {
		t.Fatal("Setup failed: no control file")
	}

	// 2. Delete the data file
	os.Remove(filepath.Join(tmpDir, "del.dat"))

	// 3. Resume
	rg2 := NewRequestGroup("gid-del", []string{server.URL}, opt)
	err := rg2.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "del.dat"))
	if len(content) != 1024 {
		t.Errorf("Expected full download 1024, got %d", len(content))
	}
}

func TestResume_CompleteFile(t *testing.T) {
	data := []byte("complete")
	server := setupResumeTestServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_complete")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "done.dat")

	// Create existing partial file with NO control file
	os.WriteFile(filepath.Join(tmpDir, "done.dat"), []byte("partial"), 0644)

	rg := NewRequestGroup("gid-done", []string{server.URL}, opt)
	rg.Execute(context.Background())

	content, _ := os.ReadFile(filepath.Join(tmpDir, "done.dat"))
	if string(content) != "complete" {
		t.Error("Should have overwritten partial file without control file")
	}
}
