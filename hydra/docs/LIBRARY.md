# Hydra Library Reference

Complete Go library documentation for Hydra.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Package Overview](#package-overview)
- [Types](#types)
- [Functions](#functions)
- [Engine Methods](#engine-methods)
- [Options](#options)
- [Events](#events)
- [Error Handling](#error-handling)

## Installation

```bash
go get github.com/bhunter/hydra
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    result, err := downloader.Download(context.Background(),
        "https://example.com/file.zip",
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("Downloaded: %s\n", result.Filename)
}
```

## Package Overview

The main package is `github.com/bhunter/hydra/pkg/downloader`.

```go
import "github.com/bhunter/hydra/pkg/downloader"
```

## Types

### DownloadID

Unique identifier for a download task.

```go
type DownloadID string
```

### Result

Represents the outcome of a completed download.

```go
type Result struct {
    Filename         string        // Path to the downloaded file
    TotalBytes       int64         // Total size in bytes
    Duration         time.Duration // Time taken to download
    AverageSpeed     int64         // Average speed in bytes per second
    ChecksumOK       bool          // Whether checksum verified successfully
    ChecksumVerified bool          // Whether checksum verification was attempted
}
```

### Progress

Represents the current progress of a download.

```go
type Progress struct {
    ID          DownloadID // Unique identifier for this download
    Downloaded  int64      // Bytes downloaded so far
    Total       int64      // Total size in bytes
    Percent     float64    // Completion percentage (0-100)
    Speed       int64      // Current speed in bytes per second
    Connections int        // Number of active connections
}
```

### State

Represents the status of a download.

```go
type State int

const (
    StatePending   State = iota // Waiting to start
    StateActive                 // Currently downloading
    StatePaused                 // Paused by user
    StateComplete               // Successfully completed
    StateError                  // Failed with error
    StateCancelled              // Cancelled by user
)
```

### Status

Full status of a download task.

```go
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
```

### EventType

Types of download events.

```go
type EventType int

const (
    EventComplete EventType = iota // Download completed successfully
    EventError                     // Download failed with error
    EventPause                     // Download paused
    EventResume                    // Download resumed
    EventCancel                    // Download cancelled
    EventStart                     // Download started
)
```

### Event

Represents a download event with progress information.

```go
type Event struct {
    Type       EventType
    ID         DownloadID
    Error      error  // Set for EventError
    Downloaded int64  // Bytes downloaded so far
    Total      int64  // Total bytes
    Speed      int64  // Current speed in bytes/sec
}
```

### Engine

Manages concurrent downloads.

```go
type Engine struct {
    // contains filtered or unexported fields
}
```

## Functions

### Download

Simple one-shot download function.

```go
func Download(ctx context.Context, url string, opts ...Option) (*Result, error)
```

**Parameters:**
- `ctx` — Context for cancellation
- `url` — URL to download
- `opts` — Optional configuration options

**Returns:**
- `*Result` — Download result with file info
- `error` — Error if download failed

**Example:**
```go
result, err := downloader.Download(ctx, "https://example.com/file.zip",
    downloader.WithDir("/tmp"),
    downloader.WithSplit(8),
)
```

### NewEngine

Creates a new download engine.

```go
func NewEngine(opts ...Option) *Engine
```

**Parameters:**
- `opts` — Engine configuration options

**Returns:**
- `*Engine` — New engine instance

**Example:**
```go
eng := downloader.NewEngine(
    downloader.WithMaxConcurrentDownloads(3),
    downloader.WithSessionFile("/tmp/session.json"),
    downloader.OnEvent(func(e downloader.Event) {
        fmt.Printf("Event: %s for %s\n", e.Type, e.ID)
    }),
)
defer eng.Shutdown()
```

## Engine Methods

### AddDownload

Adds a new download to the engine.

```go
func (e *Engine) AddDownload(ctx context.Context, urls []string, opts ...Option) (DownloadID, error)
```

**Parameters:**
- `ctx` — Context for this specific download
- `urls` — List of URLs (mirrors) for the file
- `opts` — Per-download options

**Returns:**
- `DownloadID` — Unique identifier for tracking
- `error` — Error if failed to add

**Example:**
```go
id, err := eng.AddDownload(ctx, []string{"https://example.com/file.zip"},
    downloader.WithFilename("myfile.zip"),
    downloader.WithPriority(10),
)
```

### Wait

Waits for all downloads to complete.

```go
func (e *Engine) Wait() error
```

**Returns:**
- `error` — Aggregated errors from failed downloads

### Shutdown

Gracefully shuts down the engine, saving state.

```go
func (e *Engine) Shutdown()
```

### Status

Gets the current status of a download.

```go
func (e *Engine) Status(id DownloadID) (*Status, error)
```

**Parameters:**
- `id` — Download ID

**Returns:**
- `*Status` — Current status
- `error` — Error if download not found

**Example:**
```go
status, err := eng.Status(id)
if err == nil {
    fmt.Printf("Progress: %.1f%%\n", status.Progress.Percent)
}
```

### Pause

Pauses an active download.

```go
func (e *Engine) Pause(id DownloadID) bool
```

**Returns:**
- `bool` — `true` if successfully paused

### Resume

Resumes a paused download.

```go
func (e *Engine) Resume(id DownloadID) bool
```

**Returns:**
- `bool` — `true` if successfully resumed

### Cancel

Cancels a download.

```go
func (e *Engine) Cancel(id DownloadID) bool
```

**Returns:**
- `bool` — `true` if successfully cancelled

### SaveSession

Saves current session to disk.

```go
func (e *Engine) SaveSession() error
```

**Note:** Requires `WithSessionFile()` option when creating engine.

### LoadSession

Loads and restores session from disk.

```go
func (e *Engine) LoadSession() error
```

### GetActiveCount

Returns the number of currently active downloads.

```go
func (e *Engine) GetActiveCount() int
```

### GetPendingCount

Returns the number of queued/pending downloads.

```go
func (e *Engine) GetPendingCount() int
```

### SetMaxConcurrentDownloads

Changes the concurrent download limit at runtime.

```go
func (e *Engine) SetMaxConcurrentDownloads(n int)
```

### GetQueuePosition

Gets the position of a download in the queue.

```go
func (e *Engine) GetQueuePosition(id DownloadID) int
```

**Returns:**
- Position in queue (0 = next to start)
- `-1` if not in queue (active, completed, or not found)

### GetQueuedDownloads

Returns all queued download IDs in order.

```go
func (e *Engine) GetQueuedDownloads() []DownloadID
```

### SetProgressCallback

Sets a global progress callback.

```go
func (e *Engine) SetProgressCallback(cb func(Progress))
```

### SetMessageCallback

Sets a global message/log callback.

```go
func (e *Engine) SetMessageCallback(cb func(string))
```

## Options

Options use the functional options pattern. They can be used with both `Download()` and `NewEngine()`.

### Download Options

#### WithDir

Sets the download directory.

```go
downloader.WithDir("/path/to/directory")
```

#### WithFilename

Sets the output filename.

```go
downloader.WithFilename("custom-name.zip")
```

#### WithSplit

Sets the number of connections.

```go
downloader.WithSplit(8) // Use 8 connections
```

#### WithMaxSpeed

Limits download speed.

```go
downloader.WithMaxSpeed("5M")  // 5 MB/s
downloader.WithMaxSpeed("500K") // 500 KB/s
```

#### WithLowestSpeed

Sets minimum speed before reconnect.

```go
downloader.WithLowestSpeed("10K") // Reconnect if < 10 KB/s
```

#### WithRetries

Sets retry count.

```go
downloader.WithRetries(10)
```

#### WithRetryWait

Sets wait time between retries.

```go
downloader.WithRetryWait(5) // 5 seconds
```

#### WithTimeout

Sets overall timeout.

```go
downloader.WithTimeout(120) // 120 seconds
```

#### WithConnectTimeout

Sets connection timeout.

```go
downloader.WithConnectTimeout(30) // 30 seconds
```

#### WithProxy

Sets proxy for all protocols.

```go
downloader.WithProxy("http://proxy:8080")
downloader.WithProxy("http://user:pass@proxy:8080")
```

#### WithAuth

Sets HTTP Basic Auth.

```go
downloader.WithAuth("username", "password")
```

#### WithCookieFile

Loads cookies from file.

```go
downloader.WithCookieFile("/path/to/cookies.txt")
```

#### WithChecksum

Enables checksum verification.

```go
downloader.WithChecksum("sha-256=abc123...")
```

#### WithUserAgent

Sets User-Agent header.

```go
downloader.WithUserAgent("Mozilla/5.0")
```

#### WithReferer

Sets Referer header.

```go
downloader.WithReferer("https://example.com/")
```

#### WithHeader

Adds custom header.

```go
downloader.WithHeader("Authorization", "Bearer token")
downloader.WithHeader("X-Custom", "value")
```

#### WithPriority

Sets download priority (higher = runs first).

```go
downloader.WithPriority(10) // High priority
downloader.WithPriority(1)  // Low priority
```

### Callback Options

#### WithProgress

Sets progress callback.

```go
downloader.WithProgress(func(p downloader.Progress) {
    fmt.Printf("\r%.1f%% @ %d KB/s", p.Percent, p.Speed/1024)
})
```

#### WithMessageCallback

Sets message/log callback.

```go
downloader.WithMessageCallback(func(msg string) {
    log.Println(msg)
})
```

### Engine Options

#### WithMaxConcurrentDownloads

Limits concurrent downloads.

```go
downloader.WithMaxConcurrentDownloads(3)
```

#### WithSessionFile

Enables session persistence.

```go
downloader.WithSessionFile("/path/to/session.json")
```

#### OnEvent

Subscribes to download events.

```go
downloader.OnEvent(func(e downloader.Event) {
    switch e.Type {
    case downloader.EventStart:
        fmt.Printf("Started: %s\n", e.ID)
    case downloader.EventComplete:
        fmt.Printf("Completed: %s (%d bytes)\n", e.ID, e.Total)
    case downloader.EventError:
        fmt.Printf("Failed: %s - %v\n", e.ID, e.Error)
    }
})
```

## Events

The event system provides real-time notifications about download state changes.

### Event Types

| Type | Description | Fields Set |
|------|-------------|------------|
| `EventStart` | Download started | ID, Total (if known) |
| `EventComplete` | Download completed | ID, Downloaded, Total, Speed |
| `EventError` | Download failed | ID, Error, Downloaded, Total |
| `EventPause` | Download paused | ID, Downloaded, Total |
| `EventResume` | Download resumed | ID, Downloaded, Total |
| `EventCancel` | Download cancelled | ID, Downloaded, Total |

### Example Event Handler

```go
eng := downloader.NewEngine(
    downloader.OnEvent(func(e downloader.Event) {
        switch e.Type {
        case downloader.EventStart:
            log.Printf("[START] %s", e.ID)
        case downloader.EventComplete:
            log.Printf("[DONE] %s - %d bytes", e.ID, e.Total)
        case downloader.EventError:
            log.Printf("[ERROR] %s - %v (downloaded %d/%d)", 
                e.ID, e.Error, e.Downloaded, e.Total)
        case downloader.EventPause:
            log.Printf("[PAUSE] %s at %.1f%%", 
                e.ID, float64(e.Downloaded)/float64(e.Total)*100)
        case downloader.EventResume:
            log.Printf("[RESUME] %s", e.ID)
        case downloader.EventCancel:
            log.Printf("[CANCEL] %s", e.ID)
        }
    }),
)
```

## Error Handling

### Context Cancellation

Downloads respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := downloader.Download(ctx, url)
if err == context.DeadlineExceeded {
    fmt.Println("Download timed out")
}
```

### Aggregated Errors

When using the engine, `Wait()` returns aggregated errors:

```go
err := eng.Wait()
if err != nil {
    // err contains all failed downloads
    fmt.Printf("Some downloads failed: %v\n", err)
}
```

### Per-Download Errors

Check individual download status:

```go
status, _ := eng.Status(id)
if status.State == downloader.StateError {
    fmt.Printf("Download failed: %v\n", status.Error)
}
```

### Common Errors

| Error | Cause |
|-------|-------|
| `context.Canceled` | Download cancelled via context |
| `context.DeadlineExceeded` | Timeout exceeded |
| `checksum failed` | Checksum verification failed |
| `download cancelled` | Cancelled via `Cancel()` |
| `download not found` | Invalid DownloadID |

## Thread Safety

The `Engine` is fully thread-safe. All methods can be called concurrently from multiple goroutines.

```go
// Safe to call from multiple goroutines
go func() { eng.AddDownload(ctx, urls1) }()
go func() { eng.AddDownload(ctx, urls2) }()
go func() { eng.Pause(id) }()
go func() { status, _ := eng.Status(id) }()
```
