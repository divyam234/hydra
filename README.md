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

## Quick Start

### CLI

```bash
hydra download "https://example.com/file.zip" --split 8 --dir /tmp
```

### Library

```go
package main

import (
    "context"
    "github.com/divyam234/hydra/pkg/downloader"
)

func main() {
    downloader.Download(context.Background(),
        "https://example.com/file.zip",
        downloader.WithSplit(8),
        downloader.WithDir("/tmp"),
    )
}
```

## Documentation

- [CLI Reference](docs/CLI.md) - Command-line options and examples
- [Library Reference](docs/LIBRARY.md) - Go API documentation
- [Examples](docs/EXAMPLES.md) - Code examples for common use cases
- [Architecture](docs/ARCHITECTURE.md) - Internal design overview

## License

MIT
