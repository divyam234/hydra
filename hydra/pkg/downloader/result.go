package downloader

import (
	"time"
)

// Result represents the outcome of a finished download
type Result struct {
	Filename         string        // Path to the downloaded file
	TotalBytes       int64         // Total size in bytes
	Duration         time.Duration // Time taken to download
	AverageSpeed     int64         // Average speed in bytes per second
	ChecksumOK       bool          // Whether checksum was verified successfully
	ChecksumVerified bool          // Whether checksum verification was attempted
}

// Progress represents the current state of a download
type Progress struct {
	ID          DownloadID // The unique identifier for this download
	Downloaded  int64      // Bytes downloaded so far
	Total       int64      // Total size in bytes
	Percent     float64    // Completion percentage (0-100)
	Speed       int64      // Current speed in bytes per second
	Connections int        // Number of active connections
}

// State represents the status of a download
type State int

const (
	StatePending State = iota
	StateActive
	StatePaused
	StateComplete
	StateError
	StateCancelled
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "Pending"
	case StateActive:
		return "Active"
	case StatePaused:
		return "Paused"
	case StateComplete:
		return "Complete"
	case StateError:
		return "Error"
	case StateCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// Status represents the full status of a download task
type Status struct {
	ID               DownloadID
	State            State
	Progress         Progress
	Filename         string
	Error            error
	Duration         time.Duration
	ChecksumOK       bool
	ChecksumVerified bool
}

// DownloadID is a unique identifier for a download task
type DownloadID string

// EventType represents the type of download event
type EventType int

const (
	EventComplete EventType = iota
	EventError
	EventPause
	EventResume
	EventCancel
	EventStart
)

func (e EventType) String() string {
	switch e {
	case EventComplete:
		return "Complete"
	case EventError:
		return "Error"
	case EventPause:
		return "Pause"
	case EventResume:
		return "Resume"
	case EventCancel:
		return "Cancel"
	case EventStart:
		return "Start"
	default:
		return "Unknown"
	}
}

// Event represents a download event
type Event struct {
	Type       EventType
	ID         DownloadID
	Error      error
	Downloaded int64 // Bytes downloaded so far
	Total      int64 // Total bytes
	Speed      int64 // Current speed in bytes/sec
}
