package ui

import (
	"testing"
	"time"
)

func TestPtermUI_BasicFlow_Fallback(t *testing.T) {
	// In test environment (not a TTY), PtermUI uses fallback mode
	// This test verifies fallback mode works correctly
	progressUI := NewPtermUI(false, nil)

	// In non-TTY environment, fallback should be set
	if progressUI.fallback == nil && progressUI.area == nil {
		t.Error("Expected either fallback or area to be set")
	}

	// Register should not panic in either mode
	progressUI.RegisterDownload("gid1", "test-file.zip", 10*1024*1024)

	// Progress should not panic
	progressUI.PrintProgress("gid1", 10*1024*1024, 5*1024*1024, 1024*1024, 4)

	// Mark complete should not panic
	progressUI.MarkComplete("gid1")

	// Stop should not panic
	progressUI.Stop()
}

func TestPtermUI_QuietMode(t *testing.T) {
	// When quiet=true, it should use fallback
	progressUI := NewPtermUI(true, nil)

	if progressUI.fallback == nil {
		t.Error("Expected fallback to be set in quiet mode")
	}

	// Operations should not panic in quiet mode
	progressUI.RegisterDownload("gid1", "file.txt", 1024)
	progressUI.PrintProgress("gid1", 1024, 512, 100, 1)
	progressUI.MarkComplete("gid1")
	progressUI.MarkFailed("gid2", nil)
	progressUI.Stop()
}

func TestPtermUI_RichMode_DirectState(t *testing.T) {
	// Test the internal state management directly
	// This simulates TTY mode behavior
	progressUI := &PtermUI{
		quiet:     false,
		downloads: make(map[string]*downloadState),
		order:     make([]string, 0),
		isTTY:     true,
		stopCh:    make(chan struct{}),
		// Note: area is nil so render() won't actually render
	}

	// Register a download
	progressUI.RegisterDownload("gid1", "test-file.zip", 10*1024*1024)

	progressUI.mu.Lock()
	if len(progressUI.downloads) != 1 {
		t.Errorf("Expected 1 download, got %d", len(progressUI.downloads))
	}
	dl := progressUI.downloads["gid1"]
	if dl == nil {
		t.Fatal("Download gid1 not found")
	}
	if dl.filename != "test-file.zip" {
		t.Errorf("Expected filename 'test-file.zip', got '%s'", dl.filename)
	}
	if dl.total != 10*1024*1024 {
		t.Errorf("Expected total 10MB, got %d", dl.total)
	}
	progressUI.mu.Unlock()

	// Simulate progress update
	progressUI.PrintProgress("gid1", 10*1024*1024, 5*1024*1024, 1024*1024, 4)

	progressUI.mu.Lock()
	dl = progressUI.downloads["gid1"]
	if dl.completed != 5*1024*1024 {
		t.Errorf("Expected completed 5MB, got %d", dl.completed)
	}
	if dl.speed != 1024*1024 {
		t.Errorf("Expected speed 1MB/s, got %d", dl.speed)
	}
	progressUI.mu.Unlock()

	// Mark complete
	progressUI.MarkComplete("gid1")

	progressUI.mu.Lock()
	dl = progressUI.downloads["gid1"]
	if dl.status != statusComplete {
		t.Errorf("Expected status complete, got %d", dl.status)
	}
	progressUI.mu.Unlock()

	// Stop (area is nil so this should be safe)
	progressUI.Stop()
}

func TestPtermUI_MarkFailed_DirectState(t *testing.T) {
	progressUI := &PtermUI{
		quiet:     false,
		downloads: make(map[string]*downloadState),
		order:     make([]string, 0),
		isTTY:     true,
		stopCh:    make(chan struct{}),
	}

	progressUI.RegisterDownload("gid1", "failed-file.zip", 1024)
	progressUI.MarkFailed("gid1", nil)

	progressUI.mu.Lock()
	dl := progressUI.downloads["gid1"]
	if dl.status != statusFailed {
		t.Errorf("Expected status failed, got %d", dl.status)
	}
	progressUI.mu.Unlock()

	progressUI.Stop()
}

func TestCalculateETA(t *testing.T) {
	tests := []struct {
		remaining int64
		speed     int
		expected  string
	}{
		{0, 1000, "--"},
		{1000, 0, "--"},
		{30, 1, "30s"},
		{90, 1, "1m30s"},
		{3700, 1, "1h01m"},
	}

	for _, tt := range tests {
		result := calculateETA(tt.remaining, tt.speed)
		if result != tt.expected {
			t.Errorf("calculateETA(%d, %d) = %s, expected %s", tt.remaining, tt.speed, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{3700 * time.Second, "1h01m40s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}

func TestTruncateFilename(t *testing.T) {
	tests := []struct {
		name     string
		maxLen   int
		expected string
	}{
		{"short.txt", 20, "short.txt"},
		{"this-is-a-very-long-filename.txt", 20, "...long-filename.txt"},
	}

	for _, tt := range tests {
		result := truncateFilename(tt.name, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateFilename(%s, %d) = %s, expected %s", tt.name, tt.maxLen, result, tt.expected)
		}
	}
}

func TestConsole_DownloadTrackerMethods(t *testing.T) {
	// Ensure Console implements DownloadTracker (no-op)
	c := NewConsole(false, nil)

	// These should not panic
	c.RegisterDownload("gid1", "file.txt", 1024)
	c.MarkComplete("gid1")
	c.MarkFailed("gid1", nil)
	c.Stop()
}
