package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// Console manages output
type Console struct {
	mu        sync.Mutex
	quiet     bool
	logWriter io.Writer
}

// NewConsole creates a new Console
func NewConsole(quiet bool, logWriter io.Writer) *Console {
	return &Console{quiet: quiet, logWriter: logWriter}
}

// PrintProgress prints the progress of a download
func (c *Console) PrintProgress(gid string, total, completed int64, speed int, numConns int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.quiet {
		return
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.quiet {
		return
	}
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")
}

// Printf prints a formatted string
func (c *Console) Printf(format string, a ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := fmt.Sprintf(format, a...)
	if c.logWriter != nil {
		fmt.Fprint(c.logWriter, msg)
	}

	if c.quiet {
		return
	}
	fmt.Print(msg)
}

// Println prints a line
func (c *Console) Println(a ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := fmt.Sprintln(a...)
	if c.logWriter != nil {
		fmt.Fprint(c.logWriter, msg)
	}

	if c.quiet {
		return
	}
	fmt.Print(msg)
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
