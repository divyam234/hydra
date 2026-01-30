package engine

import "time"

// State constants for download status
const (
	RGStatePending int32 = iota
	RGStateActive
	RGStatePaused
	RGStateComplete
	RGStateError
	RGStateCancelled
)

// DownloadStatus contains the full status of a download
type DownloadStatus struct {
	GID              GID
	Total            int64
	Completed        int64
	Speed            int
	State            int32
	OutputPath       string
	StartTime        time.Time
	EndTime          time.Time
	ChecksumOK       bool
	ChecksumVerified bool
	Error            error
}
