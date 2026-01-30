# Hydra Examples

Comprehensive code examples for common use cases.

## Table of Contents

- [Basic Downloads](#basic-downloads)
- [Progress Tracking](#progress-tracking)
- [Download Manager](#download-manager)
- [Queue Management](#queue-management)
- [Event Handling](#event-handling)
- [Session Persistence](#session-persistence)
- [Error Handling](#error-handling)
- [Advanced Patterns](#advanced-patterns)

---

## Basic Downloads

### Simple Download

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    result, err := downloader.Download(context.Background(),
        "https://example.com/file.zip",
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Downloaded: %s\n", result.Filename)
    fmt.Printf("Size: %d bytes\n", result.TotalBytes)
    fmt.Printf("Duration: %v\n", result.Duration)
    fmt.Printf("Average Speed: %.2f MB/s\n", float64(result.AverageSpeed)/1024/1024)
}
```

### Download with Options

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    result, err := downloader.Download(context.Background(),
        "https://example.com/large-file.iso",
        downloader.WithDir("/home/user/downloads"),
        downloader.WithFilename("ubuntu.iso"),
        downloader.WithSplit(8),              // 8 connections
        downloader.WithMaxSpeed("10M"),       // Limit to 10 MB/s
        downloader.WithRetries(5),            // Retry 5 times
        downloader.WithTimeout(3600),         // 1 hour timeout
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Downloaded: %s\n", result.Filename)
}
```

### Download with Checksum Verification

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    result, err := downloader.Download(context.Background(),
        "https://example.com/important-file.tar.gz",
        downloader.WithDir("/tmp"),
        downloader.WithChecksum("sha-256=e3b0c44298fc1c149afbf4c8996fb924..."),
    )
    if err != nil {
        log.Fatalf("Download failed: %v", err)
    }

    if result.ChecksumVerified {
        if result.ChecksumOK {
            fmt.Println("✓ Checksum verified successfully")
        } else {
            fmt.Println("✗ Checksum verification failed!")
        }
    }
}
```

### Download with Authentication

```go
package main

import (
    "context"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    // HTTP Basic Auth
    _, err := downloader.Download(context.Background(),
        "https://example.com/protected/file.zip",
        downloader.WithAuth("username", "password"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Cookie-based auth
    _, err = downloader.Download(context.Background(),
        "https://example.com/protected/file.zip",
        downloader.WithCookieFile("/path/to/cookies.txt"),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

### Download with Custom Headers

```go
package main

import (
    "context"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    _, err := downloader.Download(context.Background(),
        "https://api.example.com/download/file.zip",
        downloader.WithHeader("Authorization", "Bearer eyJhbGciOiJIUzI1NiIs..."),
        downloader.WithHeader("X-API-Key", "your-api-key"),
        downloader.WithUserAgent("MyApp/1.0"),
        downloader.WithReferer("https://example.com/"),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

### Download through Proxy

```go
package main

import (
    "context"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    // Simple proxy
    _, err := downloader.Download(context.Background(),
        "https://example.com/file.zip",
        downloader.WithProxy("http://proxy.company.com:8080"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Proxy with authentication
    _, err = downloader.Download(context.Background(),
        "https://example.com/file.zip",
        downloader.WithProxy("http://user:pass@proxy.company.com:8080"),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## Progress Tracking

### Simple Progress Bar

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    _, err := downloader.Download(context.Background(),
        "https://example.com/large-file.iso",
        downloader.WithProgress(func(p downloader.Progress) {
            fmt.Printf("\r[%-50s] %.1f%% @ %.2f MB/s",
                progressBar(p.Percent, 50),
                p.Percent,
                float64(p.Speed)/1024/1024,
            )
        }),
    )
    fmt.Println() // New line after progress

    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Download complete!")
}

func progressBar(percent float64, width int) string {
    filled := int(percent / 100 * float64(width))
    bar := ""
    for i := 0; i < width; i++ {
        if i < filled {
            bar += "█"
        } else {
            bar += "░"
        }
    }
    return bar
}
```

### Detailed Progress Information

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    startTime := time.Now()

    _, err := downloader.Download(context.Background(),
        "https://example.com/large-file.iso",
        downloader.WithProgress(func(p downloader.Progress) {
            elapsed := time.Since(startTime)
            
            // Calculate ETA
            var eta time.Duration
            if p.Speed > 0 && p.Total > 0 {
                remaining := p.Total - p.Downloaded
                eta = time.Duration(float64(remaining) / float64(p.Speed) * float64(time.Second))
            }

            fmt.Printf("\r%s / %s (%.1f%%) | %s/s | %d conns | ETA: %s    ",
                formatBytes(p.Downloaded),
                formatBytes(p.Total),
                p.Percent,
                formatBytes(p.Speed),
                p.Connections,
                formatDuration(eta),
            )
        }),
    )
    fmt.Println()

    if err != nil {
        log.Fatal(err)
    }
}

func formatBytes(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
    if d < 0 {
        return "--:--"
    }
    h := d / time.Hour
    m := (d % time.Hour) / time.Minute
    s := (d % time.Minute) / time.Second
    if h > 0 {
        return fmt.Sprintf("%d:%02d:%02d", h, m, s)
    }
    return fmt.Sprintf("%d:%02d", m, s)
}
```

---

## Download Manager

### Multiple Concurrent Downloads

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithDir("/home/user/downloads"),
        downloader.WithSplit(4),
    )
    defer eng.Shutdown()

    urls := []string{
        "https://example.com/file1.zip",
        "https://example.com/file2.zip",
        "https://example.com/file3.zip",
        "https://example.com/file4.zip",
    }

    // Add all downloads
    ids := make([]downloader.DownloadID, 0, len(urls))
    for _, url := range urls {
        id, err := eng.AddDownload(context.Background(), []string{url})
        if err != nil {
            log.Printf("Failed to add %s: %v", url, err)
            continue
        }
        ids = append(ids, id)
        fmt.Printf("Added download: %s\n", id)
    }

    // Wait for all to complete
    if err := eng.Wait(); err != nil {
        log.Printf("Some downloads failed: %v", err)
    }

    // Print results
    for _, id := range ids {
        status, _ := eng.Status(id)
        fmt.Printf("%s: %s\n", id, status.State)
    }
}
```

### Download Manager with Progress for Each File

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithDir("/tmp/downloads"),
    )
    defer eng.Shutdown()

    urls := []string{
        "https://example.com/file1.zip",
        "https://example.com/file2.zip",
        "https://example.com/file3.zip",
    }

    // Track progress for each download
    progress := make(map[downloader.DownloadID]float64)
    var mu sync.Mutex

    for i, url := range urls {
        id, _ := eng.AddDownload(context.Background(), []string{url},
            downloader.WithFilename(fmt.Sprintf("file%d.zip", i+1)),
            downloader.WithProgress(func(p downloader.Progress) {
                mu.Lock()
                progress[p.ID] = p.Percent
                mu.Unlock()
                printProgress(progress, &mu)
            }),
        )
        progress[id] = 0
    }

    eng.Wait()
    fmt.Println("\nAll downloads complete!")
}

func printProgress(progress map[downloader.DownloadID]float64, mu *sync.Mutex) {
    mu.Lock()
    defer mu.Unlock()
    
    fmt.Print("\r")
    for id, pct := range progress {
        fmt.Printf("[%s: %.0f%%] ", string(id)[:8], pct)
    }
}
```

---

## Queue Management

### Priority Queue

```go
package main

import (
    "context"
    "fmt"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithMaxConcurrentDownloads(2), // Only 2 at a time
        downloader.OnEvent(func(e downloader.Event) {
            if e.Type == downloader.EventStart {
                fmt.Printf("Started: %s\n", e.ID)
            }
        }),
    )
    defer eng.Shutdown()

    // Add downloads with different priorities
    // Higher priority = starts first
    
    eng.AddDownload(context.Background(),
        []string{"https://example.com/low-priority.zip"},
        downloader.WithPriority(1),
    )

    eng.AddDownload(context.Background(),
        []string{"https://example.com/high-priority.zip"},
        downloader.WithPriority(100), // Will start before low-priority
    )

    eng.AddDownload(context.Background(),
        []string{"https://example.com/medium-priority.zip"},
        downloader.WithPriority(50),
    )

    eng.Wait()
}
```

### Queue Inspection

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithMaxConcurrentDownloads(1),
        downloader.WithMaxSpeed("100K"), // Slow to see queue
    )
    defer eng.Shutdown()

    // Add several downloads
    for i := 1; i <= 5; i++ {
        eng.AddDownload(context.Background(),
            []string{fmt.Sprintf("https://example.com/file%d.zip", i)},
        )
    }

    // Monitor queue
    go func() {
        for {
            time.Sleep(time.Second)
            
            active := eng.GetActiveCount()
            pending := eng.GetPendingCount()
            queued := eng.GetQueuedDownloads()
            
            fmt.Printf("\rActive: %d | Pending: %d | Queue: %v    ",
                active, pending, queued)
            
            if active == 0 && pending == 0 {
                break
            }
        }
    }()

    eng.Wait()
}
```

### Dynamic Queue Control

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithMaxConcurrentDownloads(1),
    )
    defer eng.Shutdown()

    // Add downloads
    for i := 1; i <= 10; i++ {
        eng.AddDownload(context.Background(),
            []string{fmt.Sprintf("https://example.com/file%d.zip", i)},
            downloader.WithMaxSpeed("500K"),
        )
    }

    // Simulate dynamic control
    go func() {
        time.Sleep(5 * time.Second)
        fmt.Println("\nIncreasing concurrent downloads to 3...")
        eng.SetMaxConcurrentDownloads(3)
        
        time.Sleep(10 * time.Second)
        fmt.Println("\nIncreasing concurrent downloads to 5...")
        eng.SetMaxConcurrentDownloads(5)
    }()

    eng.Wait()
}
```

---

## Event Handling

### Complete Event Handler

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.OnEvent(handleEvent),
    )
    defer eng.Shutdown()

    eng.AddDownload(context.Background(),
        []string{"https://example.com/file.zip"},
    )

    eng.Wait()
}

func handleEvent(e downloader.Event) {
    switch e.Type {
    case downloader.EventStart:
        log.Printf("📥 Download started: %s", e.ID)
        
    case downloader.EventComplete:
        log.Printf("✅ Download complete: %s (%s)",
            e.ID, formatBytes(e.Total))
        
    case downloader.EventError:
        log.Printf("❌ Download failed: %s - %v (downloaded %s/%s)",
            e.ID, e.Error,
            formatBytes(e.Downloaded),
            formatBytes(e.Total))
        
    case downloader.EventPause:
        log.Printf("⏸️  Download paused: %s at %.1f%%",
            e.ID, float64(e.Downloaded)/float64(e.Total)*100)
        
    case downloader.EventResume:
        log.Printf("▶️  Download resumed: %s", e.ID)
        
    case downloader.EventCancel:
        log.Printf("🚫 Download cancelled: %s", e.ID)
    }
}

func formatBytes(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
```

### Event-Driven Download Monitor

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

type DownloadMonitor struct {
    downloads map[downloader.DownloadID]*DownloadInfo
    mu        sync.RWMutex
}

type DownloadInfo struct {
    StartTime  time.Time
    EndTime    time.Time
    Total      int64
    Downloaded int64
    Speed      int64
    State      string
    Error      error
}

func NewMonitor() *DownloadMonitor {
    return &DownloadMonitor{
        downloads: make(map[downloader.DownloadID]*DownloadInfo),
    }
}

func (m *DownloadMonitor) HandleEvent(e downloader.Event) {
    m.mu.Lock()
    defer m.mu.Unlock()

    info, exists := m.downloads[e.ID]
    if !exists {
        info = &DownloadInfo{}
        m.downloads[e.ID] = info
    }

    info.Downloaded = e.Downloaded
    info.Total = e.Total
    info.Speed = e.Speed

    switch e.Type {
    case downloader.EventStart:
        info.StartTime = time.Now()
        info.State = "downloading"
    case downloader.EventComplete:
        info.EndTime = time.Now()
        info.State = "complete"
    case downloader.EventError:
        info.EndTime = time.Now()
        info.State = "error"
        info.Error = e.Error
    case downloader.EventPause:
        info.State = "paused"
    case downloader.EventResume:
        info.State = "downloading"
    case downloader.EventCancel:
        info.EndTime = time.Now()
        info.State = "cancelled"
    }
}

func (m *DownloadMonitor) PrintStatus() {
    m.mu.RLock()
    defer m.mu.RUnlock()

    fmt.Println("\n--- Download Status ---")
    for id, info := range m.downloads {
        duration := time.Since(info.StartTime)
        if !info.EndTime.IsZero() {
            duration = info.EndTime.Sub(info.StartTime)
        }
        
        fmt.Printf("%s: %s | %d/%d bytes | %v\n",
            string(id)[:8], info.State,
            info.Downloaded, info.Total, duration.Round(time.Second))
    }
}

func main() {
    monitor := NewMonitor()

    eng := downloader.NewEngine(
        downloader.OnEvent(monitor.HandleEvent),
    )
    defer eng.Shutdown()

    eng.AddDownload(context.Background(),
        []string{"https://example.com/file1.zip"})
    eng.AddDownload(context.Background(),
        []string{"https://example.com/file2.zip"})

    eng.Wait()
    monitor.PrintStatus()
}
```

---

## Session Persistence

### Save and Restore Session

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    sessionFile := "/tmp/hydra-session.json"

    eng := downloader.NewEngine(
        downloader.WithSessionFile(sessionFile),
        downloader.WithMaxConcurrentDownloads(2),
    )

    // Handle graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        fmt.Println("\nSaving session and shutting down...")
        eng.SaveSession()
        eng.Shutdown()
        os.Exit(0)
    }()

    // Try to restore previous session
    if err := eng.LoadSession(); err == nil {
        fmt.Println("Restored previous session")
    }

    // Add new downloads
    eng.AddDownload(context.Background(),
        []string{"https://example.com/large-file.iso"},
        downloader.WithMaxSpeed("1M"),
    )

    if err := eng.Wait(); err != nil {
        fmt.Printf("Some downloads failed: %v\n", err)
    }

    fmt.Println("All downloads complete!")
}
```

### Auto-Save Session Periodically

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithSessionFile("/tmp/hydra-session.json"),
    )
    defer eng.Shutdown()

    // Auto-save every 30 seconds
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            if err := eng.SaveSession(); err != nil {
                fmt.Printf("Failed to save session: %v\n", err)
            } else {
                fmt.Println("Session saved")
            }
        }
    }()

    // Add downloads
    for i := 1; i <= 5; i++ {
        eng.AddDownload(context.Background(),
            []string{fmt.Sprintf("https://example.com/file%d.zip", i)},
            downloader.WithMaxSpeed("500K"),
        )
    }

    eng.Wait()
}
```

---

## Error Handling

### Handling Download Errors

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine()
    defer eng.Shutdown()

    id, _ := eng.AddDownload(context.Background(),
        []string{"https://example.com/nonexistent.zip"},
        downloader.WithRetries(3),
    )

    err := eng.Wait()
    if err != nil {
        // Check individual download status
        status, _ := eng.Status(id)
        
        switch status.State {
        case downloader.StateError:
            fmt.Printf("Download failed: %v\n", status.Error)
            
            // Handle specific errors
            if errors.Is(status.Error, context.DeadlineExceeded) {
                fmt.Println("Timeout - try increasing timeout")
            }
            
        case downloader.StateCancelled:
            fmt.Println("Download was cancelled")
        }
    }
}
```

### Retry with Exponential Backoff

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

func downloadWithRetry(url string, maxRetries int) error {
    var lastErr error
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(1<<attempt) * time.Second
            fmt.Printf("Retry %d/%d in %v...\n", attempt+1, maxRetries, backoff)
            time.Sleep(backoff)
        }

        _, err := downloader.Download(context.Background(), url,
            downloader.WithTimeout(30),
        )
        
        if err == nil {
            return nil
        }
        
        lastErr = err
        fmt.Printf("Attempt %d failed: %v\n", attempt+1, err)
    }

    return fmt.Errorf("all %d attempts failed, last error: %w", maxRetries, lastErr)
}

func main() {
    err := downloadWithRetry("https://example.com/file.zip", 5)
    if err != nil {
        fmt.Printf("Download failed: %v\n", err)
    }
}
```

---

## Advanced Patterns

### Download Pool with Worker Limit

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    urls := []string{
        "https://example.com/file1.zip",
        "https://example.com/file2.zip",
        "https://example.com/file3.zip",
        "https://example.com/file4.zip",
        "https://example.com/file5.zip",
    }

    // Process 2 downloads at a time
    results := downloadPool(urls, 2)
    
    for url, err := range results {
        if err != nil {
            fmt.Printf("❌ %s: %v\n", url, err)
        } else {
            fmt.Printf("✅ %s: success\n", url)
        }
    }
}

func downloadPool(urls []string, workers int) map[string]error {
    results := make(map[string]error)
    var mu sync.Mutex
    
    sem := make(chan struct{}, workers)
    var wg sync.WaitGroup

    for _, url := range urls {
        wg.Add(1)
        go func(u string) {
            defer wg.Done()
            
            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release

            _, err := downloader.Download(context.Background(), u)
            
            mu.Lock()
            results[u] = err
            mu.Unlock()
        }(url)
    }

    wg.Wait()
    return results
}
```

### Pause/Resume with User Input

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.OnEvent(func(e downloader.Event) {
            fmt.Printf("\n[Event] %s: %s\n", e.ID, e.Type)
        }),
    )
    defer eng.Shutdown()

    id, _ := eng.AddDownload(context.Background(),
        []string{"https://example.com/large-file.iso"},
        downloader.WithMaxSpeed("500K"),
    )

    fmt.Println("Commands: pause, resume, cancel, status, quit")

    // Handle user input
    go func() {
        reader := bufio.NewReader(os.Stdin)
        for {
            fmt.Print("> ")
            input, _ := reader.ReadString('\n')
            cmd := strings.TrimSpace(input)

            switch cmd {
            case "pause":
                if eng.Pause(id) {
                    fmt.Println("Paused")
                }
            case "resume":
                if eng.Resume(id) {
                    fmt.Println("Resumed")
                }
            case "cancel":
                if eng.Cancel(id) {
                    fmt.Println("Cancelled")
                }
            case "status":
                status, _ := eng.Status(id)
                fmt.Printf("State: %s, Progress: %.1f%%\n",
                    status.State, status.Progress.Percent)
            case "quit":
                eng.Shutdown()
                return
            }
        }
    }()

    eng.Wait()
}
```

### Batch Download with Progress Summary

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/bhunter/hydra/pkg/downloader"
)

type BatchDownloader struct {
    eng       *downloader.Engine
    stats     map[downloader.DownloadID]*DownloadStats
    mu        sync.RWMutex
    startTime time.Time
}

type DownloadStats struct {
    URL        string
    Downloaded int64
    Total      int64
    State      downloader.State
}

func NewBatchDownloader() *BatchDownloader {
    bd := &BatchDownloader{
        stats:     make(map[downloader.DownloadID]*DownloadStats),
        startTime: time.Now(),
    }

    bd.eng = downloader.NewEngine(
        downloader.WithMaxConcurrentDownloads(3),
        downloader.OnEvent(bd.handleEvent),
    )

    return bd
}

func (bd *BatchDownloader) handleEvent(e downloader.Event) {
    bd.mu.Lock()
    defer bd.mu.Unlock()

    if stats, ok := bd.stats[e.ID]; ok {
        stats.Downloaded = e.Downloaded
        stats.Total = e.Total
    }
}

func (bd *BatchDownloader) Add(url string) downloader.DownloadID {
    id, _ := bd.eng.AddDownload(context.Background(), []string{url})
    
    bd.mu.Lock()
    bd.stats[id] = &DownloadStats{URL: url, State: downloader.StatePending}
    bd.mu.Unlock()
    
    return id
}

func (bd *BatchDownloader) Wait() {
    // Print progress periodically
    done := make(chan struct{})
    go func() {
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                bd.printSummary()
            case <-done:
                return
            }
        }
    }()

    bd.eng.Wait()
    close(done)
    bd.printFinalSummary()
}

func (bd *BatchDownloader) printSummary() {
    bd.mu.RLock()
    defer bd.mu.RUnlock()

    var totalBytes, downloadedBytes int64
    for _, s := range bd.stats {
        totalBytes += s.Total
        downloadedBytes += s.Downloaded
    }

    elapsed := time.Since(bd.startTime)
    speed := float64(downloadedBytes) / elapsed.Seconds()

    fmt.Printf("\r[%d files] %.1f MB / %.1f MB | %.2f MB/s | %v elapsed    ",
        len(bd.stats),
        float64(downloadedBytes)/1024/1024,
        float64(totalBytes)/1024/1024,
        speed/1024/1024,
        elapsed.Round(time.Second),
    )
}

func (bd *BatchDownloader) printFinalSummary() {
    elapsed := time.Since(bd.startTime)
    
    var totalBytes int64
    for _, s := range bd.stats {
        totalBytes += s.Total
    }

    fmt.Printf("\n\n=== Download Complete ===\n")
    fmt.Printf("Files: %d\n", len(bd.stats))
    fmt.Printf("Total: %.2f MB\n", float64(totalBytes)/1024/1024)
    fmt.Printf("Time: %v\n", elapsed.Round(time.Second))
    fmt.Printf("Avg Speed: %.2f MB/s\n", float64(totalBytes)/elapsed.Seconds()/1024/1024)
}

func (bd *BatchDownloader) Shutdown() {
    bd.eng.Shutdown()
}

func main() {
    bd := NewBatchDownloader()
    defer bd.Shutdown()

    urls := []string{
        "https://example.com/file1.zip",
        "https://example.com/file2.zip",
        "https://example.com/file3.zip",
    }

    for _, url := range urls {
        bd.Add(url)
    }

    bd.Wait()
}
```
