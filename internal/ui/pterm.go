package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"golang.org/x/term"
)

type downloadStatus int

const (
	statusActive downloadStatus = iota
	statusComplete
	statusFailed
)

type downloadState struct {
	gid       string
	filename  string
	total     int64
	completed int64
	speed     int
	numConns  int
	startTime time.Time
	endTime   time.Time
	status    downloadStatus
	err       error
}

// PtermUI implements UserInterface with a rich panel-based progress display
type PtermUI struct {
	mu        sync.Mutex
	quiet     bool
	logWriter io.Writer
	downloads map[string]*downloadState
	order     []string // preserve insertion order for display
	area      *pterm.AreaPrinter
	isTTY     bool
	fallback  *Console
	stopCh    chan struct{}
	stopped   bool
}

// NewPtermUI creates a new rich terminal UI
func NewPtermUI(quiet bool, logWriter io.Writer) *PtermUI {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	p := &PtermUI{
		quiet:     quiet,
		logWriter: logWriter,
		downloads: make(map[string]*downloadState),
		order:     make([]string, 0),
		isTTY:     isTTY,
		stopCh:    make(chan struct{}),
	}

	if !isTTY || quiet {
		p.fallback = NewConsole(quiet, logWriter)
	} else {
		// Start the area printer for live updates
		p.area, _ = pterm.DefaultArea.Start()
	}

	return p
}

// RegisterDownload registers a new download for tracking
func (p *PtermUI) RegisterDownload(gid string, filename string, total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fallback != nil {
		return
	}

	if existing, ok := p.downloads[gid]; ok {
		// Update existing entry
		existing.filename = filename
		if total > 0 {
			existing.total = total
		}
		return
	}

	// New download
	p.downloads[gid] = &downloadState{
		gid:       gid,
		filename:  filename,
		total:     total,
		startTime: time.Now(),
		status:    statusActive,
	}
	p.order = append(p.order, gid)
}

// PrintProgress updates the progress of a download
func (p *PtermUI) PrintProgress(gid string, total, completed int64, speed int, numConns int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.quiet {
		return
	}

	if p.fallback != nil {
		p.fallback.PrintProgress(gid, total, completed, speed, numConns)
		return
	}

	// Update or create download state
	dl, ok := p.downloads[gid]
	if !ok {
		dl = &downloadState{
			gid:       gid,
			filename:  gid, // Use GID as fallback filename
			startTime: time.Now(),
			status:    statusActive,
		}
		p.downloads[gid] = dl
		p.order = append(p.order, gid)
	}

	dl.total = total
	dl.completed = completed
	dl.speed = speed
	dl.numConns = numConns

	// Render the panel
	p.render()
}

// MarkComplete marks a download as successfully completed
func (p *PtermUI) MarkComplete(gid string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fallback != nil {
		return
	}

	if dl, ok := p.downloads[gid]; ok {
		dl.status = statusComplete
		dl.endTime = time.Now()
		dl.completed = dl.total
		p.render()
	}
}

// MarkFailed marks a download as failed
func (p *PtermUI) MarkFailed(gid string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fallback != nil {
		return
	}

	if dl, ok := p.downloads[gid]; ok {
		dl.status = statusFailed
		dl.endTime = time.Now()
		dl.err = err
		p.render()
	}
}

// ClearLine clears the current line
func (p *PtermUI) ClearLine() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fallback != nil {
		p.fallback.ClearLine()
		return
	}
	// Area printer handles its own clearing
}

// Printf prints a formatted message
func (p *PtermUI) Printf(format string, a ...interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	msg := fmt.Sprintf(format, a...)
	if p.logWriter != nil {
		fmt.Fprint(p.logWriter, msg)
	}
	if p.quiet {
		return
	}

	if p.fallback != nil {
		fmt.Print(msg)
		return
	}

	// For pterm, we don't print these messages during active downloads
	// as they would interfere with the area printer
}

// Println prints a line
func (p *PtermUI) Println(a ...interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	msg := fmt.Sprintln(a...)
	if p.logWriter != nil {
		fmt.Fprint(p.logWriter, msg)
	}
	if p.quiet {
		return
	}

	if p.fallback != nil {
		fmt.Print(msg)
		return
	}

	// For pterm, we don't print these messages during active downloads
}

// Stop stops the area printer and cleans up
func (p *PtermUI) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return
	}
	p.stopped = true

	if p.fallback != nil {
		return
	}

	if p.area != nil {
		// Render final state
		p.render()
		p.area.Stop()

		// Print final summary
		fmt.Println()
		for _, gid := range p.order {
			dl := p.downloads[gid]
			switch dl.status {
			case statusComplete:
				duration := dl.endTime.Sub(dl.startTime)
				avgSpeed := int64(0)
				durationSecs := int64(duration.Seconds())
				if durationSecs > 0 {
					avgSpeed = dl.total / durationSecs
				}
				pterm.Success.Printf("%s - %s in %s (avg: %s/s)\n",
					dl.filename,
					formatSize(dl.total),
					formatDuration(duration),
					formatSize(avgSpeed))
			case statusFailed:
				errMsg := "unknown error"
				if dl.err != nil {
					if strings.Contains(dl.err.Error(), "context canceled") {
						continue
					}
					errMsg = dl.err.Error()
				}
				pterm.Error.Printf("%s - Failed: %s\n", dl.filename, errMsg)
			}
		}
	}
}

// render rebuilds and displays the entire panel
func (p *PtermUI) render() {
	if p.area == nil || p.stopped {
		return
	}

	panel := p.buildPanel()
	p.area.Update(panel)
}

// buildPanel constructs the visual panel string
func (p *PtermUI) buildPanel() string {
	if len(p.downloads) == 0 {
		return ""
	}

	var lines []string

	// Header
	header := pterm.NewStyle(pterm.FgCyan, pterm.Bold).Sprint("Hydra Downloads")
	lines = append(lines, header)
	lines = append(lines, pterm.Gray(strings.Repeat("─", 60)))
	lines = append(lines, "")

	// Downloads
	for i, gid := range p.order {
		dl := p.downloads[gid]

		// Filename - color indicates status
		var filenameStyle string
		switch dl.status {
		case statusComplete:
			filenameStyle = pterm.Green(truncateFilename(dl.filename, 60))
		case statusFailed:
			filenameStyle = pterm.Red(truncateFilename(dl.filename, 60))
		default:
			filenameStyle = pterm.White(truncateFilename(dl.filename, 60))
		}
		lines = append(lines, filenameStyle)

		// Progress bar
		percent := 0
		if dl.total > 0 {
			percent = int(float64(dl.completed) / float64(dl.total) * 100)
		}
		if percent > 100 {
			percent = 100
		}

		bar := p.buildProgressBar(percent, 40, dl.status)

		// Percentage with status indicator
		var percentStr string
		switch dl.status {
		case statusComplete:
			percentStr = pterm.Green(fmt.Sprintf("%3d%%  ✓", percent))
		case statusFailed:
			percentStr = pterm.Red(fmt.Sprintf("%3d%%  ✗", percent))
		default:
			percentStr = fmt.Sprintf("%3d%%", percent)
		}
		lines = append(lines, fmt.Sprintf("%s  %s", bar, percentStr))

		// Stats line
		statsLine := p.buildStatsLine(dl)
		lines = append(lines, pterm.Gray(statsLine))

		// Add extra spacing between downloads (2 blank lines)
		if i < len(p.order)-1 {
			lines = append(lines, "")
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

// buildProgressBar creates a colored progress bar
func (p *PtermUI) buildProgressBar(percent, width int, status downloadStatus) string {
	filled := width * percent / 100
	empty := width - filled

	// Use lower half block for a modern, half-height look
	// Thicker than "━" but shorter than "█"
	char := "▄"

	var filledStr, emptyStr string
	switch status {
	case statusComplete:
		filledStr = pterm.Green(strings.Repeat(char, filled))
		emptyStr = pterm.Gray(strings.Repeat(char, empty))
	case statusFailed:
		filledStr = pterm.Red(strings.Repeat(char, filled))
		emptyStr = pterm.Gray(strings.Repeat(char, empty))
	default:
		filledStr = pterm.Cyan(strings.Repeat(char, filled))
		emptyStr = pterm.Gray(strings.Repeat(char, empty))
	}

	return filledStr + emptyStr
}

// buildStatsLine creates the stats line for a download
func (p *PtermUI) buildStatsLine(dl *downloadState) string {
	completedStr := formatSize(dl.completed)
	totalStr := formatSize(dl.total)

	switch dl.status {
	case statusComplete:
		duration := dl.endTime.Sub(dl.startTime)
		avgSpeed := int64(0)
		durationSecs := int64(duration.Seconds())
		if durationSecs > 0 {
			avgSpeed = dl.total / durationSecs
		}
		return fmt.Sprintf("%s / %s  •  Avg: %s/s  •  Took: %s",
			completedStr, totalStr, formatSize(avgSpeed), formatDuration(duration))

	case statusFailed:
		errMsg := "Unknown error"
		if dl.err != nil {
			if strings.Contains(dl.err.Error(), "context canceled") {
				// Don't show "Canceled" text, just show stats
				return fmt.Sprintf("%s / %s", completedStr, totalStr)
			}
			errMsg = dl.err.Error()
			if len(errMsg) > 50 {
				errMsg = errMsg[:47] + "..."
			}
		}
		// Highlight error in red
		return fmt.Sprintf("%s / %s  •  %s", completedStr, totalStr, pterm.Red("Error: "+errMsg))

	default:
		speedStr := formatSize(int64(dl.speed)) + "/s"
		eta := calculateETA(dl.total-dl.completed, dl.speed)
		return fmt.Sprintf("%s / %s  •  %s  •  ETA: %s  •  %d conn",
			completedStr, totalStr, speedStr, eta, dl.numConns)
	}
}

// calculateETA calculates estimated time remaining
func calculateETA(remaining int64, speed int) string {
	if speed <= 0 || remaining <= 0 {
		return "--"
	}

	secs := int(remaining) / speed

	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
	default:
		return fmt.Sprintf("%dh%02dm", secs/3600, (secs%3600)/60)
	}
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	secs := int(d.Seconds())
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
	default:
		return fmt.Sprintf("%dh%02dm%02ds", secs/3600, (secs%3600)/60, secs%60)
	}
}

// truncateFilename truncates a filename for display
func truncateFilename(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return "..." + name[len(name)-maxLen+3:]
}
