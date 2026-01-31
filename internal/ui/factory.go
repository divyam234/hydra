package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

// UIStyle represents the progress display style
type UIStyle string

const (
	// UIStyleAuto automatically selects the best UI based on terminal capabilities
	UIStyleAuto UIStyle = "auto"
	// UIStyleRich uses pterm for rich panel-based progress display
	UIStyleRich UIStyle = "rich"
	// UIStyleSimple uses basic console output
	UIStyleSimple UIStyle = "simple"
)

// NewUI creates a UserInterface based on the specified style
func NewUI(style UIStyle, quiet bool, logWriter io.Writer) UserInterface {
	if quiet {
		return NewConsole(true, logWriter)
	}

	switch style {
	case UIStyleRich:
		return NewPtermUI(false, logWriter)
	case UIStyleSimple:
		return NewConsole(false, logWriter)
	default: // auto
		if term.IsTerminal(int(os.Stdout.Fd())) {
			return NewPtermUI(false, logWriter)
		}
		return NewConsole(false, logWriter)
	}
}
