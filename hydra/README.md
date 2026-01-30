# aria2go

A native Go port of the popular `aria2c` download utility. `aria2go` focuses on providing a lightweight, dependency-free, high-performance HTTP/HTTPS downloader with multi-connection support.

## Features

- **Multi-Connection Download**: Accelerate downloads by splitting files into segments and downloading them in parallel (`--split`).
- **Resume Capability**: Automatically resumes interrupted downloads using a `.aria2` control file.
- **Bandwidth Limiting**: Control download speed with `--max-download-limit`.
- **Lowest Speed Limit**: Automatically drop and retry connections that are too slow (`--lowest-speed-limit`).
- **Retry Logic**: Configurable retry attempts and wait times for robust downloading.
- **Proxy Support**: Supports HTTP and HTTPS proxies via environment variables or options.
- **Authentication**: HTTP Basic Authentication support.
- **Cookies**: Load cookies from Netscape/Mozilla formatted files (`--load-cookies`).
- **Checksum Verification**: Verify file integrity after download (MD5, SHA-1, SHA-256).

## Installation

```bash
go install github.com/bhunter/aria2go/cmd/aria2go@latest
```

Or build from source:

```bash
git clone https://github.com/bhunter/aria2go.git
cd aria2go/aria2go
go build -o aria2go cmd/aria2go/main.go
```

## Usage

### CLI Usage

Basic download:
```bash
./aria2go download "https://example.com/file.zip"
```

Download with 8 connections and 5MB/s limit:
```bash
./aria2go download "https://example.com/large-file.iso" --split 8 --max-download-limit 5M
```

Save to specific directory and filename:
```bash
./aria2go download "https://example.com/file.zip" -d /tmp -o my_download.zip
```

Verify checksum:
```bash
./aria2go download "https://example.com/file.zip" --checksum sha-256=YOUR_HASH_HERE
```

Load cookies for authenticated downloads:
```bash
./aria2go download "https://example.com/protected.zip" --load-cookies cookies.txt
```

### Library Usage

You can use aria2go as a library in your Go programs.

```go
package main

import (
    "context"
    "fmt"
    "github.com/bhunter/aria2go/pkg/downloader"
)

func main() {
    // Simple one-liner
    result, err := downloader.Download(context.Background(), 
        "https://example.com/file.zip",
        downloader.WithDir("/tmp"),
        downloader.WithSplit(8),
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("Downloaded %s (%d bytes) in %v\n", 
        result.Filename, result.TotalBytes, result.Duration)
    
    // Advanced usage with Engine
    eng := downloader.NewEngine(
        downloader.WithMaxSpeed("10M"),
        downloader.WithMessageCallback(func(msg string) {
            fmt.Printf("[Log] %s\n", msg)
        }),
    )
    defer eng.Shutdown()
    
    id, _ := eng.AddDownload(context.Background(), []string{"https://example.com/file.zip"})
    eng.Wait()
}
```

See `pkg/downloader` documentation for more details.

## Options

| Option | Description | Default |
|--------|-------------|---------|
| `-s, --split` | Number of connections to use | 5 |
| `-d, --dir` | Directory to store the downloaded file | Current Dir |
| `-o, --out` | The filename of the downloaded file | URL basename |
| `--max-download-limit` | Max download speed per download (e.g. 1M, 500K) | Unlimited |
| `--lowest-speed-limit` | Close connection if speed is lower than this (e.g. 10K) | 0 (Disabled) |
| `--max-tries` | Number of retries on error | 5 |
| `--retry-wait` | Wait time between retries in seconds | 0 |
| `--load-cookies` | Load cookies from Netscape/Mozilla format file | None |
| `--timeout` | Timeout in seconds | 60 |
| `--connect-timeout` | Connect timeout in seconds | 15 |
| `--max-pieces-per-segment` | Max pieces per segment (chunk size control) | 20 |
| `--http-proxy` | HTTP proxy URL | Env |
| `--https-proxy` | HTTPS proxy URL | Env |
| `--all-proxy` | Proxy for all protocols | Env |
| `--user-agent` | Set User-Agent header | aria2go/0.1.0 |
| `--referer` | Set Referer header | None |
| `--header` | Append header to HTTP request | None |
| `--http-user` | Set HTTP Basic Auth user | None |
| `--http-passwd` | Set HTTP Basic Auth password | None |
| `--checksum` | Verify checksum (algo=hash) | None |

## License

MIT
