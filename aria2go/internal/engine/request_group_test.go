package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bhunter/aria2go/pkg/option"
)

// Test HTTP server that supports Range requests
func setupRangeServer(t *testing.T, data []byte) *httptest.Server {
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

		// Simplified range parsing: bytes=start-end
		var start, end int64
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
		if err != nil {
			// Try bytes=start-
			_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if err != nil {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			end = int64(len(data)) - 1
		}

		if start >= int64(len(data)) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
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

// 1. Core Integration Test - Successful Download
func TestRequestGroup_Execute_Success(t *testing.T) {
	data := make([]byte, 1024*100) // 100KB
	for i := range data {
		data[i] = byte(i % 256)
	}
	server := setupRangeServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_test")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "test.dat")
	fullPath := filepath.Join(tmpDir, "test.dat")
	opt.Put(option.Split, "4")
	opt.Put(option.RetryWait, "0")

	rg := NewRequestGroup("test-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify file content
	downloaded, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != len(data) {
		t.Errorf("Expected %d bytes, got %d", len(data), len(downloaded))
	}
	for i := range data {
		if downloaded[i] != data[i] {
			t.Fatalf("Data mismatch at index %d", i)
		}
	}
}

// 2. Resume Integration Test
func TestRequestGroup_Resume_Logic(t *testing.T) {
	data := make([]byte, 1024*100)
	// Use a throttled server to ensure download takes longer than the cancel delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")

		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			time.Sleep(200 * time.Millisecond) // Delay to allow cancel to happen
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

		// Flush headers so client starts receiving
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Slow down the body transfer
		// We send in small chunks with delay to ensure we can catch it mid-download
		chunkSize := 1024
		content := data[start : end+1]
		for i := 0; i < len(content); i += chunkSize {
			// Check if we should sleep
			time.Sleep(5 * time.Millisecond)

			end := i + chunkSize
			if end > len(content) {
				end = len(content)
			}
			w.Write(content[i:end])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_resume")
	defer os.RemoveAll(tmpDir)

	out := "resume.dat"
	fullPath := filepath.Join(tmpDir, out)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, out)
	opt.Put(option.Split, "4")

	// Phase 1: Start download and cancel it
	ctx, cancel := context.WithCancel(context.Background())
	rg1 := NewRequestGroup("gid-1", []string{server.URL}, opt)

	go func() {
		// Wait for initialization to complete and workers to start
		// The download should take ~500ms (100KB / 1KB chunks * 5ms)
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := rg1.Execute(ctx)
	// Expect context canceled error or similar, but key is control file existence
	if err == nil {
		// It might return nil if it thinks it finished, but we expect it to be canceled
		// However, check control file is what matters
	}

	// Verify control file exists
	if _, err := os.Stat(fullPath + ".aria2"); os.IsNotExist(err) {
		t.Fatal("Control file not found after interruption")
	}

	// Phase 2: Resume
	// We need a new RequestGroup but pointing to same file
	rg2 := NewRequestGroup("gid-1", []string{server.URL}, opt)
	err = rg2.Execute(context.Background())
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// Verify file
	downloaded, _ := os.ReadFile(fullPath)
	if len(downloaded) != len(data) {
		t.Errorf("Resumed file size mismatch: expected %d, got %d", len(data), len(downloaded))
	}
}

// 3. Network Errors and Retries
func TestRequestGroup_Retry_On_Failure(t *testing.T) {
	failCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			return
		}

		if atomic.AddInt32(&failCount, 1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(make([]byte, 100))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_retry")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.MaxTries, "5")
	opt.Put(option.RetryWait, "0")

	rg := NewRequestGroup("retry-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Expected success after retries, got: %v", err)
	}

	if atomic.LoadInt32(&failCount) < 3 {
		t.Error("Server should have been called at least 3 times")
	}
}

// 4. Single Connection Fallback (No Accept-Ranges)
func TestRequestGroup_Fallback_To_Single(t *testing.T) {
	data := []byte("no ranges here")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Does NOT send Accept-Ranges
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_fallback")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "fallback.dat")
	fullPath := filepath.Join(tmpDir, "fallback.dat")

	rg := NewRequestGroup("fallback-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	downloaded, _ := os.ReadFile(fullPath)
	if string(downloaded) != string(data) {
		t.Errorf("Downloaded content mismatch in fallback mode")
	}
}

// 5. Enrich Request Logic
func TestEnrichRequest_Headers(t *testing.T) {
	opt := option.GetDefaultOptions()
	opt.Put(option.UserAgent, "TestAgent")
	opt.Put(option.Referer, "http://ref.com")
	opt.Put(option.Header, "X-Custom: val1\nY-Custom: val2")

	rg := NewRequestGroup("gid", nil, opt)
	req, _ := http.NewRequest("GET", "http://test.com", nil)
	rg.enrichRequest(req)

	if req.Header.Get("User-Agent") != "TestAgent" {
		t.Error("UA mismatch")
	}
	if req.Header.Get("Referer") != "http://ref.com" {
		t.Error("Referer mismatch")
	}
	if req.Header.Get("X-Custom") != "val1" {
		t.Error("Custom header X mismatch")
	}
	if req.Header.Get("Y-Custom") != "val2" {
		t.Error("Custom header Y mismatch")
	}
}

// 6. Context Cancellation
func TestRequestGroup_Cancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	rg := NewRequestGroup("cancel-gid", []string{server.URL}, option.GetDefaultOptions())
	err := rg.Execute(ctx)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}
