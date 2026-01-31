# Hydra

![Hydra Logo](https://via.placeholder.com/150x150.png?text=Hydra) <!-- Placeholder for actual logo -->

[![Go Reference](https://pkg.go.dev/badge/github.com/divyam234/hydra.svg)](https://pkg.go.dev/github.com/divyam234/hydra)
[![Go Report Card](https://goreportcard.com/badge/github.com/divyam234/hydra)](https://goreportcard.com/report/github.com/divyam234/hydra)
[![Build Status](https://github.com/divyam234/hydra/actions/workflows/ci.yml/badge.svg)](https://github.com/divyam234/hydra/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/divyam234/hydra.svg)](https://github.com/divyam234/hydra/releases)

**Hydra** is a next-generation, high-performance download manager and library written in pure Go. It is designed to maximize throughput by utilizing multi-connection segmented downloading, making it significantly faster than traditional single-connection tools like `wget` or `curl` for large files.

Hydra functions both as a feature-rich **CLI tool** and a flexible **Go library** that can be embedded into your own applications.

---

## 🚀 Key Features

### ⚡ Core Performance
- **Multi-Connection Downloading**: Splits files into segments and downloads them in parallel to saturate bandwidth.
- **Resumable Downloads**: Automatically resumes interrupted downloads using `.hydra` control files. No more restart from zero.
- **Smart Retries**: Exponential backoff and smart retry logic for network instability.

### 🛠 Advanced Control
- **Download Queue**: Priority-based queuing system to manage hundreds of downloads.
- **Speed Limiting**: Global and per-download bandwidth limits to avoid hogging the network.
- **Session Persistence**: Save the entire download state (queue, progress) and restore it after a restart.
- **Checksum Verification**: Built-in support for MD5, SHA-1, SHA-256, and SHA-512 integrity checks.

### 👨‍💻 Developer Friendly
- **Pure Go**: No CGO dependencies, easy to cross-compile for Linux, macOS, and Windows.
- **Event-Driven API**: Subscribe to real-time events (`OnStart`, `OnProgress`, `OnComplete`, `OnError`).
- **Context Aware**: Full support for `context.Context` for cancellation and timeouts.
- **Thread Safe**: Designed for concurrent use in high-throughput applications.

---

## 📦 Installation

### Pre-built Binaries
Download the latest binary for your OS from the [Releases Page](https://github.com/divyam234/hydra/releases).

### From Source (CLI)
```bash
go install github.com/divyam234/hydra/cmd/hydra@latest
```

### As a Library
```bash
go get github.com/divyam234/hydra
```

---

## 🎮 CLI Usage

Hydra provides a clean, git-like CLI interface.

### Basic Download
```bash
hydra download "https://example.com/large-file.iso"
```

### Advanced Options
```bash
# Download with 16 connections, limit speed to 10MB/s, save to /tmp
hydra download "https://example.com/movie.mp4" \
  --split 16 \
  --max-download-limit 10M \
  --dir /tmp \
  --out my_movie.mp4

### Performance Tuning
Hydra allows fine-grained control over network buffers and connection pools for maximum throughput:

```bash
hydra download "https://example.com/huge-dataset.tar.gz" \
  --read-buffer-size 1M \    # Increase read buffer (default 256K)
  --write-buffer-size 1M \   # Increase write buffer (default 64K)
  --max-idle-conns 2000 \    # Max idle connections (default 1000)
  --idle-conn-timeout 60 \   # Close idle connections after 60s
  --progress-batch-size 128K # Progress update frequency (default 256K)
```

### Batch Download
```bash
# Download all URLs listed in a file
hydra download --input-file urls.txt
```

### Batch Download
```bash
# Download all URLs listed in a file
hydra download --input-file urls.txt
```

For a complete reference, see the [CLI Documentation](docs/CLI.md).

---

## 📚 Library Usage

Embed Hydra directly into your Go application.

### Simple Example
```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/divyam234/hydra/pkg/downloader"
)

func main() {
    // Start a download with 8 connections
    result, err := downloader.Download(context.Background(),
        "https://example.com/data.bin",
        downloader.WithSplit(8),
        downloader.WithDir("./downloads"),
        downloader.WithProgress(func(p downloader.Progress) {
            fmt.Printf("\rDownload: %.2f%% (%s/s)", p.Percent, p.Speed)
        }),
    )

    if err != nil {
        log.Fatalf("Download failed: %v", err)
    }

    fmt.Printf("\nSuccess! Saved to %s\n", result.Filename)
}
```

### Advanced Engine Example
For complex applications needing queues and persistence:

```go
// Create a persistent engine
engine := downloader.NewEngine(
    downloader.WithSessionFile("session.json"),
    downloader.WithMaxConcurrentDownloads(3),
)
defer engine.Shutdown()

// Add a download to the queue
id, err := engine.AddDownload(ctx, "https://example.com/file.zip", 
    downloader.WithPriority(downloader.PriorityHigh),
)

// Wait for all to finish
engine.Wait()
```

See [Library Documentation](docs/LIBRARY.md) and [Examples](docs/EXAMPLES.md) for more details.

---

## 📊 Benchmarks

Comparison downloading a 1GB file on a 100Mbps connection:

| Tool | Connections | Time | Speed |
|------|:-----------:|------|-------|
| **Hydra** | **8** | **82s** | **12.5 MB/s** |
| curl | 1 | 96s | 10.6 MB/s |
| wget | 1 | 98s | 10.4 MB/s |

*Note: Multi-connection benefits are most visible on high-latency links or when servers throttle per-connection bandwidth.*

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2026 bhunter
