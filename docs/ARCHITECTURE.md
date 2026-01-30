# Hydra Architecture

Internal design and architecture overview of Hydra.

## Table of Contents

- [Overview](#overview)
- [Package Structure](#package-structure)
- [Core Components](#core-components)
- [Download Flow](#download-flow)
- [Segmented Downloads](#segmented-downloads)
- [Queue Management](#queue-management)
- [Session Persistence](#session-persistence)
- [Control Files](#control-files)

---

## Overview

Hydra is designed as a layered architecture with clear separation between the public API and internal implementation.

```
┌─────────────────────────────────────────────────────────┐
│                    Public API Layer                      │
│              pkg/downloader (Download, Engine)           │
├─────────────────────────────────────────────────────────┤
│                    Internal Engine                       │
│         internal/engine (DownloadEngine, RequestGroup)   │
├─────────────────────────────────────────────────────────┤
│                    Support Modules                       │
│   internal/http    internal/segment    internal/control  │
│   internal/stats   internal/limit      internal/disk     │
└─────────────────────────────────────────────────────────┘
```

## Package Structure

```
hydra/
├── cmd/hydra/              # CLI application
│   └── main.go             # Entry point, Cobra commands
│
├── pkg/                    # Public packages
│   ├── downloader/         # Main public API
│   │   ├── downloader.go   # Download() function
│   │   ├── engine.go       # Engine type and methods
│   │   ├── options.go      # Functional options
│   │   ├── result.go       # Result, Progress, Event types
│   │   └── doc.go          # Package documentation
│   │
│   ├── option/             # Configuration options
│   │   ├── option.go       # Option container
│   │   └── prefs.go        # Option constants and defaults
│   │
│   └── apperror/           # Error codes
│       └── codes.go        # Error definitions
│
├── internal/               # Private packages
│   ├── engine/             # Core download engine
│   │   ├── engine.go       # DownloadEngine
│   │   ├── request_group.go # RequestGroup (single download)
│   │   ├── session.go      # Session persistence
│   │   ├── status.go       # State definitions
│   │   └── gid.go          # GID generator
│   │
│   ├── http/               # HTTP client
│   │   ├── client.go       # HTTP request handling
│   │   └── cookie.go       # Cookie file parsing
│   │
│   ├── segment/            # Segmented download
│   │   ├── segment.go      # Segment definition
│   │   ├── segment_man.go  # Segment manager
│   │   ├── piece.go        # Piece definition
│   │   ├── piece_storage.go # Piece tracking
│   │   └── bitfield.go     # Completion bitfield
│   │
│   ├── control/            # Control files
│   │   └── control_file.go # .hydra file handling
│   │
│   ├── stats/              # Statistics
│   │   ├── speed_calc.go   # Speed calculation
│   │   └── transfer_stat.go # Transfer statistics
│   │
│   ├── limit/              # Rate limiting
│   │   └── limiter.go      # Bandwidth limiter
│   │
│   ├── disk/               # Disk I/O
│   │   └── adaptor.go      # File operations
│   │
│   ├── ui/                 # User interface
│   │   ├── interface.go    # UI interface
│   │   └── console.go      # Console output
│   │
│   └── util/               # Utilities
│       ├── uri.go          # URI parsing
│       └── hash.go         # Hash utilities
│
└── docs/                   # Documentation
    ├── CLI.md
    ├── LIBRARY.md
    ├── EXAMPLES.md
    └── ARCHITECTURE.md
```

## Core Components

### DownloadEngine

The central coordinator that manages all downloads.

```go
type DownloadEngine struct {
    mu            sync.RWMutex
    options       *option.Option
    requestGroups map[GID]*RequestGroup
    gidGen        *GidGenerator
    
    // Queue management
    maxConcurrent int
    activeCount   int
    pendingQueue  []*RequestGroup
    
    // Session management
    sessionManager *SessionManager
    
    // Event hooks
    eventCallback EventCallback
}
```

**Responsibilities:**
- Creating and tracking RequestGroups
- Queue management with priority ordering
- Event dispatching
- Session save/restore
- Graceful shutdown

### RequestGroup

Represents a single download task.

```go
type RequestGroup struct {
    gid      GID
    uris     []string
    options  *option.Option
    state    atomic.Int32
    priority int
    
    // Pause/Resume/Cancel channels
    pauseCh   chan struct{}
    resumeCh  chan struct{}
    cancelCh  chan struct{}
}
```

**Responsibilities:**
- Executing the download
- Managing segment workers
- Handling pause/resume/cancel
- Progress tracking
- Checksum verification

### Segment Manager

Divides files into segments for parallel download.

```go
type SegmentManager struct {
    totalLength  int64
    pieceLength  int64
    pieces       []Piece
    bitfield     *Bitfield
}
```

**Responsibilities:**
- Calculating segment boundaries
- Assigning segments to workers
- Tracking completion status
- Supporting resume via bitfield

## Download Flow

### Simple Download Flow

```
┌──────────────┐
│   User       │
│ Download()   │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Engine     │
│ AddDownload()│
└──────┬───────┘
       │
       ▼
┌──────────────┐
│RequestGroup  │
│  Execute()   │
└──────┬───────┘
       │
       ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Worker 1    │    │  Worker 2    │    │  Worker N    │
│  Segment 0   │    │  Segment 1   │    │  Segment N   │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   Disk       │
                    │   Writer     │
                    └──────────────┘
```

### Detailed Download Sequence

```
1. User calls Download() or Engine.AddDownload()
   
2. Engine creates RequestGroup with unique GID
   
3. Engine checks queue:
   - If activeCount < maxConcurrent: Start immediately
   - Else: Add to pendingQueue (sorted by priority)
   
4. RequestGroup.Execute():
   a. Send HEAD request to get file size
   b. Create output file
   c. Initialize SegmentManager
   d. Check for existing .hydra control file (resume)
   e. Launch worker goroutines
   
5. Each worker:
   a. Get next incomplete segment
   b. Send GET request with Range header
   c. Write data to file at correct offset
   d. Update bitfield
   e. Save control file periodically
   f. Repeat until no more segments
   
6. After all workers complete:
   a. Verify checksum (if configured)
   b. Remove control file
   c. Fire completion event
   d. Update state
   
7. Engine.onDownloadFinished():
   a. Decrement activeCount
   b. Start next download from pendingQueue
   c. Save session (if configured)
```

## Segmented Downloads

### How Segmentation Works

```
File: 100 MB, Split: 4 connections

┌────────────────────────────────────────────────────────┐
│                     100 MB File                         │
├──────────────┬──────────────┬──────────────┬───────────┤
│  Segment 0   │  Segment 1   │  Segment 2   │ Segment 3 │
│  0-25 MB     │  25-50 MB    │  50-75 MB    │ 75-100 MB │
│  Worker 1    │  Worker 2    │  Worker 3    │ Worker 4  │
└──────────────┴──────────────┴──────────────┴───────────┘
```

### Range Requests

Each worker sends an HTTP Range request:

```
GET /file.zip HTTP/1.1
Host: example.com
Range: bytes=26214400-52428799

HTTP/1.1 206 Partial Content
Content-Range: bytes 26214400-52428799/104857600
Content-Length: 26214400
```

### Bitfield Tracking

The bitfield tracks which pieces are complete:

```
Bitfield: 11110000 11111111 00000000

Piece 0-3:   Complete (1111)
Piece 4-7:   Incomplete (0000)
Piece 8-15:  Complete (11111111)
Piece 16-23: Incomplete (00000000)
```

## Queue Management

### Priority Queue

Downloads are sorted by priority (higher = runs first):

```go
// Add to queue
e.pendingQueue = append(e.pendingQueue, rg)

// Sort by priority
sort.Slice(e.pendingQueue, func(i, j int) bool {
    return e.pendingQueue[i].priority > e.pendingQueue[j].priority
})
```

### Concurrent Download Limit

```go
if e.maxConcurrent > 0 && e.activeCount >= e.maxConcurrent {
    // Add to pending queue
    e.pendingQueue = append(e.pendingQueue, rg)
} else {
    // Start immediately
    e.activeCount++
    e.startDownload(ctx, rg)
}
```

### Queue Processing

When a download finishes:

```go
func (e *DownloadEngine) onDownloadFinished(rg *RequestGroup) {
    e.activeCount--
    
    // Start next pending download
    if len(e.pendingQueue) > 0 {
        next := e.pendingQueue[0]
        e.pendingQueue = e.pendingQueue[1:]
        e.activeCount++
        e.startDownload(context.Background(), next)
    }
}
```

## Session Persistence

### Session File Format

```json
{
  "downloads": [
    {
      "gid": "abc123def456",
      "uris": ["https://example.com/file.zip"],
      "options": {
        "dir": "/tmp/downloads",
        "out": "file.zip",
        "split": "8"
      },
      "state": 1,
      "priority": 10
    }
  ]
}
```

### Session Save

```go
func (sm *SessionManager) Save(engine *DownloadEngine) error {
    session := Session{Downloads: []SessionEntry{}}
    
    for gid, rg := range engine.requestGroups {
        // Only save incomplete downloads
        if state != RGStateComplete && state != RGStateCancelled {
            entry := SessionEntry{
                GID:      gid,
                URIs:     rg.uris,
                Options:  rg.options.ToMap(),
                State:    state,
                Priority: rg.priority,
            }
            session.Downloads = append(session.Downloads, entry)
        }
    }
    
    return os.WriteFile(sm.filePath, json.Marshal(session))
}
```

### Session Restore

```go
func (e *DownloadEngine) LoadSession() error {
    session, _ := e.sessionManager.Load()
    
    for _, entry := range session.Downloads {
        opt := option.NewOption()
        opt.FromMap(entry.Options)
        
        rg := NewRequestGroup(entry.GID, entry.URIs, opt)
        rg.priority = entry.Priority
        
        // Queue or start based on limits
        e.requestGroups[entry.GID] = rg
        e.startOrQueue(rg)
    }
    
    return nil
}
```

## Control Files

### Control File Format (.hydra)

```json
{
  "gid": "abc123def456",
  "total_length": 104857600,
  "piece_length": 1048576,
  "num_pieces": 100,
  "bitfield": "ffffffff00000000",
  "uris": ["https://example.com/file.zip"],
  "path": "/tmp/downloads/file.zip"
}
```

### Resume Logic

```go
func (rg *RequestGroup) Execute(ctx context.Context) error {
    ctrl := control.NewController(outputPath)
    
    if ctrl.Exists() {
        // Load existing control file
        cf, _ := ctrl.Load()
        
        // Restore bitfield
        bitfield := segment.ParseBitfield(cf.Bitfield)
        
        // Resume from incomplete pieces
        segMan.SetBitfield(bitfield)
    }
    
    // Continue download...
}
```

### Control File Lifecycle

```
1. Download starts
   └─> Create empty control file
   
2. During download
   └─> Update control file periodically
   
3. Download completes
   └─> Delete control file
   
4. Download interrupted
   └─> Control file remains
   
5. Download resumes
   └─> Load control file
   └─> Skip completed pieces
```

## Thread Safety

### Mutex Usage

```go
type DownloadEngine struct {
    mu       sync.RWMutex  // Protects requestGroups map
    queueMu  sync.Mutex    // Protects queue operations
}

// Reading - multiple concurrent readers allowed
e.mu.RLock()
rg := e.requestGroups[gid]
e.mu.RUnlock()

// Writing - exclusive access
e.mu.Lock()
e.requestGroups[gid] = rg
e.mu.Unlock()
```

### Atomic State

```go
type RequestGroup struct {
    state atomic.Int32  // Thread-safe state
}

// Reading
state := rg.state.Load()

// Writing
rg.state.Store(RGStateComplete)
```

### Channel-Based Pause/Resume

```go
func (rg *RequestGroup) Pause() bool {
    select {
    case rg.pauseCh <- struct{}{}:
        rg.state.Store(RGStatePaused)
        return true
    default:
        return false
    }
}
```

## Performance Considerations

### Connection Reuse

HTTP connections are reused via `http.Transport`:

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
```

### Rate Limiting

Uses `golang.org/x/time/rate` for bandwidth control:

```go
limiter := rate.NewLimiter(rate.Limit(bytesPerSecond), burstSize)

// Before writing
limiter.WaitN(ctx, len(data))
```

### Memory Efficiency

- Streams data directly to disk
- Uses fixed-size buffers (32KB default)
- No full-file memory buffering
