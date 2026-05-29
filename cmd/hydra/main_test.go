package main

import (
	"strings"
	"testing"
	"time"

	"github.com/divyam234/hydra"
)

func TestRenderProgressDoesNotShowPieces(t *testing.T) {
	line := renderProgress(hydra.Progress{
		State:       hydra.StateDownloading,
		Total:       100 << 20,
		Completed:   25 << 20,
		Active:      4,
		Connections: 8,
		PiecesDone:  25,
		PiecesTotal: 100,
		Speed:       2 << 20,
		AvgSpeed:    1500 << 10,
		ETA:         30 * time.Second,
	}, 2<<20)
	if strings.Contains(line, "pieces=") || strings.Contains(line, "blocks=") {
		t.Fatalf("normal terminal UI must not show piece/block counters: %q", line)
	}
	for _, want := range []string{"25.0%", "2.0MiB/s", "avg=", "eta=30s", "active=4/8", "retries=0"} {
		if !strings.Contains(line, want) {
			t.Fatalf("rendered line missing %q: %q", want, line)
		}
	}
}

func TestTerminalUISmoothsDisplaySpeed(t *testing.T) {
	ui := newTerminalUI(0)
	ui.speed = 10 << 20
	ui.lastState = hydra.StateDownloading
	p := hydra.Progress{State: hydra.StateDownloading, Total: 100, Completed: 50, Speed: 100 << 20}
	ui.Render(p)
	if ui.speed <= 10<<20 || ui.speed >= 100<<20 {
		t.Fatalf("expected EMA-smoothed speed between old and raw, got %f", ui.speed)
	}
}
