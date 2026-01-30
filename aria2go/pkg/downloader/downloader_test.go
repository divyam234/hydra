package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// setupTestServer creates a simple HTTP server for testing
func setupTestServer(t *testing.T, content []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")

		// Handle HEAD request
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle GET request
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
			w.Write(content)
			return
		}

		// Simple range parsing
		var start, end int64
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
		if err != nil {
			_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if err != nil {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			end = int64(len(content)) - 1
		}

		if start >= int64(len(content)) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= int64(len(content)) {
			end = int64(len(content)) - 1
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[start : end+1])
	}))
}

func TestDownload(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "aria2go_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dummy content
	content := []byte("Hello, aria2go!")
	server := setupTestServer(t, content)
	defer server.Close()

	// Test basic download
	outFile := "test_download.txt"
	ctx := context.Background()

	result, err := Download(ctx, server.URL,
		WithDir(tmpDir),
		WithFilename(outFile),
		WithSplit(2),
	)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify result
	if result.Filename != filepath.Join(tmpDir, outFile) {
		t.Errorf("Expected filename %s, got %s", filepath.Join(tmpDir, outFile), result.Filename)
	}
	if result.TotalBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.TotalBytes)
	}
	if result.ChecksumVerified {
		t.Error("Expected ChecksumVerified to be false")
	}

	// Verify file content
	downloaded, err := os.ReadFile(result.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != string(content) {
		t.Errorf("Content mismatch. Expected %s, got %s", string(content), string(downloaded))
	}
}

func TestDownload_ContextCancellation(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_cancel_test")
	defer os.RemoveAll(tmpDir)

	// Large content to ensure we can catch it in progress
	content := make([]byte, 1024*1024) // 1MB
	server := setupTestServer(t, content)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Limit speed to ensure it takes long enough to be canceled
	_, err := Download(ctx, server.URL,
		WithDir(tmpDir),
		WithMaxSpeed("100K"), // 100KB/s -> ~10s for 1MB
	)
	if err == nil {
		t.Error("Expected error due to cancellation, got nil")
	} else if err != context.DeadlineExceeded {
		// Just ensure it's an error.
	}
}

func TestEngine_Lifecycle(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_engine_test")
	defer os.RemoveAll(tmpDir)

	content := []byte("Engine test content")
	server := setupTestServer(t, content)
	defer server.Close()

	eng := NewEngine(WithDir(tmpDir))
	defer eng.Shutdown()

	// Add download
	id, err := eng.AddDownload(context.Background(), []string{server.URL}, WithFilename("engine_test.txt"))
	if err != nil {
		t.Fatalf("AddDownload failed: %v", err)
	}

	// Check status immediately
	status, err := eng.Status(id)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.ID != id {
		t.Errorf("ID mismatch")
	}

	// Wait for completion
	if err := eng.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Check final status
	status, err = eng.Status(id)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateComplete {
		t.Errorf("Expected StateComplete, got %v", status.State)
	}
	if status.Progress.Downloaded != int64(len(content)) {
		t.Errorf("Expected downloaded %d, got %d", len(content), status.Progress.Downloaded)
	}
}

func TestEngine_Callbacks(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_callback_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*100) // 100KB
	server := setupTestServer(t, content)
	defer server.Close()

	progressCalls := atomic.Int32{}
	progressChan := make(chan struct{}, 10)

	eng := NewEngine(
		WithDir(tmpDir),
		WithProgress(func(p Progress) {
			progressCalls.Add(1)
			select {
			case progressChan <- struct{}{}:
			default:
			}
		}),
	)
	defer eng.Shutdown()

	// Use low speed to ensure we get updates
	// 50KB/s -> 2s total
	id, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("callback.bin"),
		WithMaxSpeed("50K"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for at least one progress callback
	select {
	case <-progressChan:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for progress callback")
	}

	// Wait for completion
	if err := eng.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if progressCalls.Load() == 0 {
		t.Error("Progress callback was never called")
	}

	// Check status
	st, _ := eng.Status(id)
	if st.State == StateError {
		t.Errorf("Download failed: %v", st.Error)
	}
}

func TestWithHeader(t *testing.T) {
	// Tests that options are created without panic
	_ = WithHeader("X-Test", "123")
}

func TestEngine_Status_NotFound(t *testing.T) {
	eng := NewEngine()
	defer eng.Shutdown()

	_, err := eng.Status("invalid-id")
	if err == nil {
		t.Error("Expected error for invalid download ID, got nil")
	}
}

func TestDownload_WithChecksum(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_checksum_test")
	defer os.RemoveAll(tmpDir)

	content := []byte("Hello, checksum test!")
	server := setupTestServer(t, content)
	defer server.Close()

	// Calculate SHA-256 of content
	// SHA-256 of "Hello, checksum test!" is a]...
	// For testing, we'll use a wrong checksum first

	// Test with WRONG checksum - should fail
	_, err := Download(context.Background(), server.URL,
		WithDir(tmpDir),
		WithFilename("checksum_fail.txt"),
		WithChecksum("sha-256=0000000000000000000000000000000000000000000000000000000000000000"),
	)
	if err == nil {
		t.Error("Expected error for wrong checksum, got nil")
	}
}

func TestEngine_PerDownloadCallbacks(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_perdownload_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*100) // 100KB
	server := setupTestServer(t, content)
	defer server.Close()

	download1Calls := atomic.Int32{}
	download2Calls := atomic.Int32{}
	download1Chan := make(chan struct{}, 10)
	download2Chan := make(chan struct{}, 10)

	eng := NewEngine(WithDir(tmpDir))
	defer eng.Shutdown()

	// Download 1 with its own callback
	id1, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("download1.bin"),
		WithMaxSpeed("50K"), // Slow enough to get progress updates
		WithProgress(func(p Progress) {
			download1Calls.Add(1)
			select {
			case download1Chan <- struct{}{}:
			default:
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Download 2 with its own callback
	id2, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("download2.bin"),
		WithMaxSpeed("50K"),
		WithProgress(func(p Progress) {
			download2Calls.Add(1)
			select {
			case download2Chan <- struct{}{}:
			default:
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for at least one callback from each download
	select {
	case <-download1Chan:
	case <-time.After(5 * time.Second):
		t.Log("Timeout waiting for download 1 callback")
	}

	select {
	case <-download2Chan:
	case <-time.After(5 * time.Second):
		t.Log("Timeout waiting for download 2 callback")
	}

	// Wait for completion
	eng.Wait()

	// Verify per-download callbacks were called
	if download1Calls.Load() == 0 {
		t.Error("Download 1 callback was never called")
	}
	if download2Calls.Load() == 0 {
		t.Error("Download 2 callback was never called")
	}

	// Verify both downloads completed
	st1, _ := eng.Status(id1)
	st2, _ := eng.Status(id2)
	if st1.State != StateComplete {
		t.Errorf("Download 1 state: %v, error: %v", st1.State, st1.Error)
	}
	if st2.State != StateComplete {
		t.Errorf("Download 2 state: %v, error: %v", st2.State, st2.Error)
	}
}

func TestEngine_SetMessageCallback(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_msgcb_test")
	defer os.RemoveAll(tmpDir)

	content := []byte("Message callback test")
	server := setupTestServer(t, content)
	defer server.Close()

	messages := make([]string, 0)
	msgMu := sync.Mutex{}

	eng := NewEngine(WithDir(tmpDir))
	defer eng.Shutdown()

	eng.SetMessageCallback(func(msg string) {
		msgMu.Lock()
		messages = append(messages, msg)
		msgMu.Unlock()
	})

	_, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("msg_test.txt"),
	)
	if err != nil {
		t.Fatal(err)
	}

	eng.Wait()

	// Messages may or may not be generated depending on internal logging
	// Just verify no panic occurred and the callback was set
}

func TestEngine_ConcurrentDownloads(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_concurrent_test")
	defer os.RemoveAll(tmpDir)

	content := []byte("Concurrent download content")
	server := setupTestServer(t, content)
	defer server.Close()

	eng := NewEngine(WithDir(tmpDir))
	defer eng.Shutdown()

	numDownloads := 5
	ids := make([]DownloadID, numDownloads)

	for i := 0; i < numDownloads; i++ {
		id, err := eng.AddDownload(context.Background(), []string{server.URL},
			WithFilename(fmt.Sprintf("concurrent_%d.txt", i)),
		)
		if err != nil {
			t.Fatalf("Failed to add download %d: %v", i, err)
		}
		ids[i] = id
	}

	// Wait for all
	if err := eng.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify all completed
	for i, id := range ids {
		st, err := eng.Status(id)
		if err != nil {
			t.Errorf("Status %d failed: %v", i, err)
			continue
		}
		if st.State != StateComplete {
			t.Errorf("Download %d state: %v, error: %v", i, st.State, st.Error)
		}

		// Verify file exists and has correct content
		filePath := filepath.Join(tmpDir, fmt.Sprintf("concurrent_%d.txt", i))
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read file %d: %v", i, err)
			continue
		}
		if string(data) != string(content) {
			t.Errorf("Content mismatch for download %d", i)
		}
	}
}

func TestWithHeader_VerifyHeaders(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_header_test")
	defer os.RemoveAll(tmpDir)

	receivedHeaders := make(map[string]string)
	headerMu := sync.Mutex{}

	// Custom server that captures headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerMu.Lock()
		receivedHeaders["X-Custom-Header"] = r.Header.Get("X-Custom-Header")
		receivedHeaders["X-Another-Header"] = r.Header.Get("X-Another-Header")
		receivedHeaders["User-Agent"] = r.Header.Get("User-Agent")
		headerMu.Unlock()

		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "5")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	_, err := Download(context.Background(), server.URL,
		WithDir(tmpDir),
		WithFilename("header_test.txt"),
		WithHeader("X-Custom-Header", "custom-value"),
		WithHeader("X-Another-Header", "another-value"),
		WithUserAgent("TestAgent/1.0"),
	)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	headerMu.Lock()
	defer headerMu.Unlock()

	if receivedHeaders["X-Custom-Header"] != "custom-value" {
		t.Errorf("Expected X-Custom-Header 'custom-value', got '%s'", receivedHeaders["X-Custom-Header"])
	}
	if receivedHeaders["X-Another-Header"] != "another-value" {
		t.Errorf("Expected X-Another-Header 'another-value', got '%s'", receivedHeaders["X-Another-Header"])
	}
	if receivedHeaders["User-Agent"] != "TestAgent/1.0" {
		t.Errorf("Expected User-Agent 'TestAgent/1.0', got '%s'", receivedHeaders["User-Agent"])
	}
}

func TestProgress_HasDownloadID(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_progress_id_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*100) // 100KB
	server := setupTestServer(t, content)
	defer server.Close()

	var capturedID DownloadID
	idChan := make(chan DownloadID, 1)

	eng := NewEngine(
		WithDir(tmpDir),
		WithProgress(func(p Progress) {
			select {
			case idChan <- p.ID:
			default:
			}
		}),
	)
	defer eng.Shutdown()

	expectedID, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("progress_id.bin"),
		WithMaxSpeed("50K"),
	)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case capturedID = <-idChan:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for progress callback")
	}

	eng.Wait()

	if capturedID != expectedID {
		t.Errorf("Expected progress ID %s, got %s", expectedID, capturedID)
	}
}

func TestEngine_Cancel(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_cancel_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*1024) // 1MB
	server := setupTestServer(t, content)
	defer server.Close()

	eng := NewEngine(WithDir(tmpDir))
	defer eng.Shutdown()

	// Start a slow download
	id, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("cancel_test.bin"),
		WithMaxSpeed("50K"), // 50KB/s -> ~20s for 1MB
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait a bit for download to start
	time.Sleep(200 * time.Millisecond)

	// Cancel the download
	cancelled := eng.Cancel(id)
	if !cancelled {
		t.Error("Cancel returned false")
	}

	// Wait for completion
	eng.Wait()

	// Check status
	st, err := eng.Status(id)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateCancelled {
		t.Errorf("Expected StateCancelled, got %v", st.State)
	}
}

func TestEngine_PauseResume(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_pause_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*200) // 200KB
	server := setupTestServer(t, content)
	defer server.Close()

	eng := NewEngine(WithDir(tmpDir))
	defer eng.Shutdown()

	// Start a slow download
	id, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("pause_test.bin"),
		WithMaxSpeed("50K"), // 50KB/s -> ~4s for 200KB
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait a bit for download to start
	time.Sleep(500 * time.Millisecond)

	// Pause the download
	paused := eng.Pause(id)
	if !paused {
		t.Error("Pause returned false")
	}

	// Check status is paused
	st, _ := eng.Status(id)
	if st.State != StatePaused {
		t.Errorf("Expected StatePaused, got %v", st.State)
	}

	// Wait a bit while paused
	time.Sleep(200 * time.Millisecond)

	// Resume the download
	resumed := eng.Resume(id)
	if !resumed {
		t.Error("Resume returned false")
	}

	// Check status is active
	st, _ = eng.Status(id)
	if st.State != StateActive {
		t.Errorf("Expected StateActive after resume, got %v", st.State)
	}

	// Wait for completion
	eng.Wait()

	// Check final status
	st, _ = eng.Status(id)
	if st.State != StateComplete {
		t.Errorf("Expected StateComplete, got %v, error: %v", st.State, st.Error)
	}
}

func TestEngine_CancelNotFound(t *testing.T) {
	eng := NewEngine()
	defer eng.Shutdown()

	// Try to cancel non-existent download
	cancelled := eng.Cancel("non-existent-id")
	if cancelled {
		t.Error("Cancel should return false for non-existent download")
	}
}

func TestEngine_PauseNotActive(t *testing.T) {
	eng := NewEngine()
	defer eng.Shutdown()

	// Try to pause non-existent download
	paused := eng.Pause("non-existent-id")
	if paused {
		t.Error("Pause should return false for non-existent download")
	}
}

func TestEngine_MaxConcurrentDownloads(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_maxconcurrent_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*200) // 200KB - larger to ensure queue behavior
	server := setupTestServer(t, content)
	defer server.Close()

	// Create engine with max 2 concurrent downloads
	eng := NewEngine(
		WithDir(tmpDir),
		WithMaxConcurrentDownloads(2),
	)
	defer eng.Shutdown()

	// Verify initial counts
	if eng.GetActiveCount() != 0 {
		t.Errorf("Expected 0 active downloads, got %d", eng.GetActiveCount())
	}
	if eng.GetPendingCount() != 0 {
		t.Errorf("Expected 0 pending downloads, got %d", eng.GetPendingCount())
	}

	// Add 4 downloads with very slow speed
	ids := make([]DownloadID, 4)
	for i := 0; i < 4; i++ {
		id, err := eng.AddDownload(context.Background(), []string{server.URL},
			WithFilename(fmt.Sprintf("maxconcurrent_%d.bin", i)),
			WithMaxSpeed("10K"), // 10KB/s -> 20s for 200KB
		)
		if err != nil {
			t.Fatalf("Failed to add download %d: %v", i, err)
		}
		ids[i] = id
	}

	// Give downloads time to start
	time.Sleep(500 * time.Millisecond)

	// Check that only 2 are active, 2 are pending
	active := eng.GetActiveCount()
	pending := eng.GetPendingCount()

	// At least verify the queue limiting is working
	if active > 2 {
		t.Errorf("Expected max 2 active downloads, got %d", active)
	}
	t.Logf("Active: %d, Pending: %d", active, pending)

	// Cancel all to speed up test
	for _, id := range ids {
		eng.Cancel(id)
	}

	// Wait for all to complete
	eng.Wait()
}

func TestEngine_EventCallbacks(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_events_test")
	defer os.RemoveAll(tmpDir)

	content := []byte("Event callback test content")
	server := setupTestServer(t, content)
	defer server.Close()

	events := make([]Event, 0)
	eventMu := sync.Mutex{}
	eventChan := make(chan Event, 10)

	eng := NewEngine(
		WithDir(tmpDir),
		OnEvent(func(e Event) {
			eventMu.Lock()
			events = append(events, e)
			eventMu.Unlock()
			select {
			case eventChan <- e:
			default:
			}
		}),
	)
	defer eng.Shutdown()

	// Add a download
	id, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("event_test.txt"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for start event
	select {
	case e := <-eventChan:
		if e.Type != EventStart {
			t.Errorf("Expected EventStart, got %v", e.Type)
		}
		if e.ID != id {
			t.Errorf("Expected event ID %s, got %s", id, e.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for start event")
	}

	// Wait for completion
	eng.Wait()

	// Check we got a complete event
	eventMu.Lock()
	defer eventMu.Unlock()

	hasStart := false
	hasComplete := false
	for _, e := range events {
		if e.Type == EventStart && e.ID == id {
			hasStart = true
		}
		if e.Type == EventComplete && e.ID == id {
			hasComplete = true
		}
	}

	if !hasStart {
		t.Error("Did not receive EventStart")
	}
	if !hasComplete {
		t.Error("Did not receive EventComplete")
	}
}

func TestEngine_EventOnCancel(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_cancel_event_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*500) // 500KB
	server := setupTestServer(t, content)
	defer server.Close()

	events := make([]Event, 0)
	eventMu := sync.Mutex{}

	eng := NewEngine(
		WithDir(tmpDir),
		OnEvent(func(e Event) {
			eventMu.Lock()
			events = append(events, e)
			eventMu.Unlock()
		}),
	)
	defer eng.Shutdown()

	// Start a slow download
	id, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("cancel_event.bin"),
		WithMaxSpeed("10K"), // 10KB/s -> 50s for 500KB
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for download to start
	time.Sleep(500 * time.Millisecond)

	// Cancel it
	eng.Cancel(id)

	// Wait for completion
	eng.Wait()

	// Check we got a cancel event (or error event if cancel didn't work in time)
	eventMu.Lock()
	defer eventMu.Unlock()

	hasCancel := false
	hasError := false
	for _, e := range events {
		if e.Type == EventCancel && e.ID == id {
			hasCancel = true
		}
		if e.Type == EventError && e.ID == id {
			hasError = true
		}
	}

	// Either cancel or error is acceptable (timing dependent)
	if !hasCancel && !hasError {
		t.Error("Did not receive EventCancel or EventError")
	}
}

func TestEngine_EventOnPauseResume(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_pauseresume_event_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*100) // 100KB
	server := setupTestServer(t, content)
	defer server.Close()

	events := make([]Event, 0)
	eventMu := sync.Mutex{}

	eng := NewEngine(
		WithDir(tmpDir),
		OnEvent(func(e Event) {
			eventMu.Lock()
			events = append(events, e)
			eventMu.Unlock()
		}),
	)
	defer eng.Shutdown()

	// Start a slow download
	id, err := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("pauseresume_event.bin"),
		WithMaxSpeed("25K"), // 25KB/s -> 4s for 100KB
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for download to start
	time.Sleep(300 * time.Millisecond)

	// Pause it
	eng.Pause(id)
	time.Sleep(100 * time.Millisecond)

	// Resume it
	eng.Resume(id)

	// Wait for completion
	eng.Wait()

	// Check we got pause and resume events
	eventMu.Lock()
	defer eventMu.Unlock()

	hasPause := false
	hasResume := false
	for _, e := range events {
		if e.Type == EventPause && e.ID == id {
			hasPause = true
		}
		if e.Type == EventResume && e.ID == id {
			hasResume = true
		}
	}

	if !hasPause {
		t.Error("Did not receive EventPause")
	}
	if !hasResume {
		t.Error("Did not receive EventResume")
	}
}

func TestEngine_Priority(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_priority_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*50) // 50KB
	server := setupTestServer(t, content)
	defer server.Close()

	startOrder := make([]DownloadID, 0)
	orderMu := sync.Mutex{}

	eng := NewEngine(
		WithDir(tmpDir),
		WithMaxConcurrentDownloads(1), // Only one at a time to test ordering
		OnEvent(func(e Event) {
			if e.Type == EventStart {
				orderMu.Lock()
				startOrder = append(startOrder, e.ID)
				orderMu.Unlock()
			}
		}),
	)
	defer eng.Shutdown()

	// Add low priority download first
	lowID, _ := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("low_priority.bin"),
		WithPriority(1),
		WithMaxSpeed("100K"),
	)

	// Add high priority download second - should run before low priority
	highID, _ := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("high_priority.bin"),
		WithPriority(10),
		WithMaxSpeed("100K"),
	)

	// Add medium priority
	medID, _ := eng.AddDownload(context.Background(), []string{server.URL},
		WithFilename("med_priority.bin"),
		WithPriority(5),
		WithMaxSpeed("100K"),
	)

	// Wait for all to complete
	eng.Wait()

	orderMu.Lock()
	defer orderMu.Unlock()

	// First one starts immediately (low priority since it was first)
	// Then high priority, then medium priority
	if len(startOrder) != 3 {
		t.Fatalf("Expected 3 starts, got %d", len(startOrder))
	}

	// First download (lowID) starts immediately because queue is empty
	// After it completes, highID should start before medID
	if startOrder[0] != lowID {
		t.Errorf("First download should be low (started immediately), got %s", startOrder[0])
	}
	if startOrder[1] != highID {
		t.Errorf("Second download should be high priority, got %s", startOrder[1])
	}
	if startOrder[2] != medID {
		t.Errorf("Third download should be medium priority, got %s", startOrder[2])
	}
}

func TestEngine_SetMaxConcurrentDownloads(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_setmax_test")
	defer os.RemoveAll(tmpDir)

	content := make([]byte, 1024*200) // 200KB - larger for longer downloads
	server := setupTestServer(t, content)
	defer server.Close()

	// Start with max 1
	eng := NewEngine(
		WithDir(tmpDir),
		WithMaxConcurrentDownloads(1),
	)
	defer eng.Shutdown()

	// Add 3 slow downloads
	ids := make([]DownloadID, 3)
	for i := 0; i < 3; i++ {
		id, err := eng.AddDownload(context.Background(), []string{server.URL},
			WithFilename(fmt.Sprintf("setmax_%d.bin", i)),
			WithMaxSpeed("10K"), // 10KB/s -> 20s for 200KB
		)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}

	time.Sleep(500 * time.Millisecond)

	active := eng.GetActiveCount()
	pending := eng.GetPendingCount()
	t.Logf("Initial state - Active: %d, Pending: %d", active, pending)

	// Increase max to 3
	eng.SetMaxConcurrentDownloads(3)
	time.Sleep(500 * time.Millisecond)

	active2 := eng.GetActiveCount()
	pending2 := eng.GetPendingCount()
	t.Logf("After increase - Active: %d, Pending: %d", active2, pending2)

	// Cancel all to speed up test
	for _, id := range ids {
		eng.Cancel(id)
	}

	eng.Wait()
}

func TestEngine_SessionSaveLoad(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "aria2go_session_test")
	defer os.RemoveAll(tmpDir)

	sessionFile := filepath.Join(tmpDir, "test_session.json")

	// Create engine with session file
	eng := NewEngine(
		WithDir(tmpDir),
		WithSessionFile(sessionFile),
	)

	// Just verify the methods don't panic
	// Full session testing requires more complex setup
	err := eng.SaveSession()
	if err != nil {
		t.Logf("SaveSession error (expected for empty session): %v", err)
	}

	eng.Shutdown()

	// Create new engine and try to load
	eng2 := NewEngine(
		WithDir(tmpDir),
		WithSessionFile(sessionFile),
	)
	defer eng2.Shutdown()

	err = eng2.LoadSession()
	if err != nil {
		t.Logf("LoadSession error (expected for non-existent session): %v", err)
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		e    EventType
		want string
	}{
		{EventComplete, "Complete"},
		{EventError, "Error"},
		{EventPause, "Pause"},
		{EventResume, "Resume"},
		{EventCancel, "Cancel"},
		{EventStart, "Start"},
		{EventType(999), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.e.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.e, got, tt.want)
		}
	}
}
