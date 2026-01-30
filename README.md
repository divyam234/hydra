# Hydra

[![Go Reference](https://pkg.go.dev/badge/github.com/bhunter/hydra.svg)](https://pkg.go.dev/github.com/bhunter/hydra)
[![Go Report Card](https://goreportcard.com/badge/github.com/bhunter/hydra)](https://goreportcard.com/report/github.com/bhunter/hydra)

**Hydra** is a high-performance, multi-connection download manager written in Go. It accelerates downloads by splitting files into segments and downloading them in parallel across multiple connections.

## Features

- **Multi-Connection Downloads** — Split files into segments and download in parallel
- **Resume Support** — Automatically resume interrupted downloads using `.hydra` control files
- **Download Queue** — Priority-based queue with configurable concurrent download limits
- **Pause/Resume/Cancel** — Full control over active downloads
- **Event System** — Subscribe to download events (start, complete, error, pause, resume, cancel)
- **Session Persistence** — Save and restore download state across restarts
- **Bandwidth Control** — Limit download speed per-download or globally
- **Checksum Verification** — Verify file integrity with MD5, SHA-1, SHA-256, SHA-512
- **Proxy Support** — HTTP/HTTPS/SOCKS proxy support
- **Authentication** — HTTP Basic Auth and cookie-based authentication
- **Zero Dependencies** — Pure Go, no CGO required

## Installation

### CLI Tool

```bash
go install github.com/bhunter/hydra/cmd/hydra@latest
```

### Library

```bash
go get github.com/bhunter/hydra
```

### Build from Source

```bash
git clone https://github.com/bhunter/hydra.git
cd hydra
go build -o hydra ./cmd/hydra
```

## Quick Start

### CLI

```bash
# Basic download
hydra download "https://example.com/file.zip"

# Download with 8 connections
hydra download "https://example.com/large.iso" --split 8

# Download to specific location
hydra download "https://example.com/file.zip" -d /tmp -o myfile.zip

# Limit speed to 5MB/s
hydra download "https://example.com/file.zip" --max-download-limit 5M
```

### Library

```go
package main

import (
    "context"
    "fmt"
    "github.com/bhunter/hydra/pkg/downloader"
)

func main() {
    // Simple one-liner download
    result, err := downloader.Download(context.Background(),
        "https://example.com/file.zip",
        downloader.WithDir("/tmp"),
        downloader.WithSplit(8),
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("Downloaded %s (%d bytes)\n", result.Filename, result.TotalBytes)
}
```

## Documentation

| Document | Description |
|----------|-------------|
| [CLI Reference](docs/CLI.md) | Complete CLI documentation with all flags and options |
| [Library Guide](docs/LIBRARY.md) | Library API documentation with types and methods |
| [Examples](docs/EXAMPLES.md) | Comprehensive code examples for common use cases |
| [Architecture](docs/ARCHITECTURE.md) | Internal design and architecture overview |

## Performance

Hydra uses multiple connections to maximize download speed, especially on high-latency or bandwidth-limited connections:

```
Single connection:    ████████████████████  100 MB in 60s (1.67 MB/s)
8 connections:        ████████████████████  100 MB in 12s (8.33 MB/s)
```

## Comparison

| Feature | Hydra | wget | curl |
|---------|-------|------|------|
| Multi-connection | ✅ | ❌ | ❌ |
| Resume downloads | ✅ | ✅ | ✅ |
| Library API | ✅ | ❌ | ✅ |
| Download queue | ✅ | ❌ | ❌ |
| Event system | ✅ | ❌ | ❌ |
| Session persistence | ✅ | ❌ | ❌ |
| Pure Go | ✅ | ❌ | ❌ |

## License

MIT License - see [LICENSE](LICENSE) for details.
