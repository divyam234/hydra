package hydra

import "time"

// EventKind describes high-level downloader lifecycle events.
type EventKind string

const (
	EventQueued      EventKind = "queued"
	EventStarted     EventKind = "started"
	EventProbing     EventKind = "probing"
	EventDownloading EventKind = "downloading"
	EventRetrying    EventKind = "retrying"
	EventPieceDone   EventKind = "piece_done"
	EventVerifying   EventKind = "verifying"
	EventCompleted   EventKind = "completed"
	EventSkipped     EventKind = "skipped"
	EventFailed      EventKind = "failed"
	EventCancelled   EventKind = "cancelled"
)

// Event is emitted through Options.OnEvent. It is intentionally stable and
// library-friendly so applications can build logs, UIs, metrics, or RPC APIs on
// top without parsing stdout.
type Event struct {
	Kind       EventKind
	ID         string
	URL        string
	Path       string
	Attempt    int
	PieceIndex int
	Total      int64
	Completed  int64
	Error      string
	Time       time.Time
	Progress   Progress
}

type EventFunc func(Event)
