# Hydra

A high-performance, multi-connection download manager and library written in Go.

## Key Features

*   **Multi-Connection**: Splits files into segments for parallel downloading.
*   **Resumable**: Automatically resumes interrupted downloads.
*   **Queue Management**: Priority-based queuing and concurrency control.
*   **Rate Limiting**: Global and per-download bandwidth limits.
*   **Persistence**: Save and restore download sessions.
*   **Pure Go**: Cross-platform single binary, no dependencies.

## Installation

### CLI

```bash
go install github.com/divyam234/hydra/cmd/hydra@latest
```

### Library

```bash
go get github.com/divyam234/hydra
```

## CLI Usage

```bash
# Basic download
hydra download "https://example.com/file.zip"

# With options: 8 connections, 5MB/s limit, save to specific path
hydra download "https://example.com/file.zip" \
  --split 8 \
  --max-download-limit 5M \
  --out /tmp/file.zip

# Performance tuning
hydra download "https://example.com/large.iso" \
  --read-buffer-size 1M \
  --write-buffer-size 1M \
  --max-idle-conns 2000
```

## Library Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/divyam234/hydra/pkg/downloader"
)

func main() {
    // Start download with 8 connections
    _, err := downloader.Download(context.Background(),
        "https://example.com/data.bin",
        downloader.WithSplit(8),
        downloader.WithProgress(func(p downloader.Progress) {
            fmt.Printf("\r%.2f%%", p.Percent)
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

## License

MIT
