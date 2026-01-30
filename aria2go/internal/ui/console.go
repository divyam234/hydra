package ui

import (
	"fmt"
	"strings"
	"sync"
)

// Console manages output
type Console struct {
	mu sync.Mutex
}

// NewConsole creates a new Console
func NewConsole() *Console {
	return &Console{}
}

// PrintProgress prints the progress of a download
func (c *Console) PrintProgress(gid string, total, completed int64, speed int, numConns int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	percent := 0
	if total > 0 {
		percent = int(float64(completed) / float64(total) * 100)
	}

	// Simple status line
	// [DL: 2.5MiB 25% 1.2MiB/s CN:5] [file.zip]

	line := fmt.Sprintf("\r[%s %d%% %s/s CN:%d] ",
		formatSize(completed),
		percent,
		formatSize(int64(speed)),
		numConns)

	// Pad with spaces to clear line
	fmt.Print(line)
}

// ClearLine clears the current line
func (c *Console) ClearLine() {
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
