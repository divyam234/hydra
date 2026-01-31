package ui

// UserInterface defines how the download engine communicates with the user
type UserInterface interface {
	PrintProgress(gid string, total, completed int64, speed int, numConns int)
	ClearLine()
	Printf(format string, a ...interface{})
	Println(a ...interface{})
}

// DownloadTracker is an optional interface for UIs that track download metadata
// for richer progress displays (e.g., filenames, completion status, errors)
type DownloadTracker interface {
	// RegisterDownload registers a new download with its filename and total size
	RegisterDownload(gid string, filename string, total int64)

	// MarkComplete marks a download as successfully completed
	MarkComplete(gid string)

	// MarkFailed marks a download as failed with an error
	MarkFailed(gid string, err error)

	// Stop stops the UI and cleans up resources
	Stop()
}
