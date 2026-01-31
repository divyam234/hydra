# Hydra

A fast, multi-connection download manager written in Go. Works as a CLI tool or embeddable library.

## Features

- Multi-connection segmented downloads
- Resumable interrupted downloads
- Priority-based download queue
- Bandwidth limiting
- Session persistence
- Checksum verification (MD5, SHA-1, SHA-256, SHA-512)
- HTTP Basic Auth and cookie support
- Proxy support

## Installation

```bash
# CLI
go install github.com/divyam234/hydra/cmd/hydra@latest

# Library
go get github.com/divyam234/hydra
```

## CLI Usage

```bash
# Basic download
hydra download "https://example.com/file.zip"

# Multiple connections with speed limit
hydra download "https://example.com/large.iso" \
  --split 8 \
  --max-download-limit 5M \
  --dir /downloads

# With authentication
hydra download "https://example.com/private.zip" \
  --http-user admin \
  --http-passwd secret

# With checksum verification
hydra download "https://example.com/file.tar.gz" \
  --checksum "sha-256=e3b0c44298fc1c149afbf4c8996fb924..."
```

### Common Options

| Option | Description |
|--------|-------------|
| `--split, -s` | Number of connections (default: 5) |
| `--dir, -d` | Download directory |
| `--out, -o` | Output filename |
| `--max-download-limit` | Speed limit (e.g. `5M`, `500K`) |
| `--max-tries` | Retry attempts (default: 5) |
| `--checksum` | Verify hash after download |

See [CLI Reference](docs/CLI.md) for all options.

## Library Usage

### Simple Download

```go
package main

import (
    "context"
    "fmt"
    "github.com/divyam234/hydra/pkg/downloader"
)

func main() {
    result, err := downloader.Download(context.Background(),
        "https://example.com/file.zip",
        downloader.WithSplit(8),
        downloader.WithDir("/downloads"),
        downloader.WithProgress(func(p downloader.Progress) {
            fmt.Printf("\r%.1f%% @ %d KB/s", p.Percent, p.Speed/1024)
        }),
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("\nSaved to: %s\n", result.Filename)
}
```

### Download Manager

```go
package main

import (
    "context"
    "fmt"
    "github.com/divyam234/hydra/pkg/downloader"
)

func main() {
    eng := downloader.NewEngine(
        downloader.WithMaxConcurrentDownloads(3),
        downloader.WithSessionFile("session.json"),
        downloader.OnEvent(func(e downloader.Event) {
            switch e.Type {
            case downloader.EventComplete:
                fmt.Printf("Done: %s\n", e.ID)
            case downloader.EventError:
                fmt.Printf("Failed: %s - %v\n", e.ID, e.Error)
            }
        }),
    )
    defer eng.Shutdown()

    // Add downloads
    eng.AddDownload(context.Background(),
        []string{"https://example.com/file1.zip"},
        downloader.WithPriority(10),
    )
    eng.AddDownload(context.Background(),
        []string{"https://example.com/file2.zip"},
    )

    // Wait for all to complete
    eng.Wait()
}
```

### Core Functions

| Function | Description |
|----------|-------------|
| `Download(ctx, url, opts...)` | One-shot download |
| `NewEngine(opts...)` | Create download manager |
| `Engine.AddDownload(ctx, urls, opts...)` | Queue a download |
| `Engine.Wait()` | Wait for all downloads |
| `Engine.Pause(id)` / `Resume(id)` | Pause/resume download |
| `Engine.Cancel(id)` | Cancel download |
| `Engine.Status(id)` | Get download status |
| `Engine.Shutdown()` | Graceful shutdown |

See [Library Reference](docs/LIBRARY.md) for complete API.

## How It Works

Hydra splits files into segments and downloads them in parallel using HTTP Range requests. Progress is saved to `.hydra` control files, enabling resume after interruption.

```
100 MB file with 4 connections:

[===Worker 1===][===Worker 2===][===Worker 3===][===Worker 4===]
    0-25 MB        25-50 MB        50-75 MB        75-100 MB
```

## Documentation

- [CLI Reference](docs/CLI.md) - All command-line options
- [Library Reference](docs/LIBRARY.md) - Go API documentation
- [Examples](docs/EXAMPLES.md) - Code examples
- [Architecture](docs/ARCHITECTURE.md) - Internal design

## License

MIT
