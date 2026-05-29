# Hydra

Hydra is a Go-native, library-first HTTP/HTTPS downloader focused on fast, resumable, low-overhead file transfers.

The core package is designed for applications to import directly. The CLI is a small wrapper over the same API.

## Features

- HTTP and HTTPS downloads.
- Parallel segmented downloads with large `Range` requests.
- Compact bitfield resume metadata in `filename.hydra` for segmented downloads.
- `.part` continuation for single-stream downloads.
- Graceful Ctrl+C handling that saves metadata and fsyncs the output before exit.
- Range support detection even when `HEAD` does not advertise `Accept-Ranges`.
- Resume tolerant of unstable CDN/signed-URL validators by default; strict validation is available.
- HTTP/HTTPS proxy support.
- SOCKS5 and SOCKS5H proxy support through `golang.org/x/net/proxy`.
- Mirror URL rotation and retry failover.
- Exponential retry backoff.
- SHA-256, SHA-512, SHA-1, and MD5 verification.
- Existing-file policies: `resume`, `overwrite`, `skip`, `error`.
- Importable package API.
- Queue manager API for multiple downloads.
- Lifecycle events for UIs, logs, metrics, RPC servers, or JSON streams.
- Rich progress snapshots with speed, average speed, ETA, active connections, retries, resumed bytes, and downloaded bytes.
- Clean terminal progress UI without noisy internal block counters.
- File lock guard to prevent multiple writers for the same target.
- Batched metadata flushing to reduce filesystem overhead while preserving safe cancellation resume.
- Tests for segmented download, bitfield resume, Ctrl+C/context cancellation resume, checksum, retry, manager, HTTP proxy, and SOCKS proxy.

## Install / build

```bash
go test ./...
go build ./cmd/hydra
```

## CLI examples

```bash
# Fast segmented download
./hydra -s 16 -x 16 -k 20M -d downloads https://example.com/big.iso

# Save with a custom filename
./hydra -o ubuntu.iso https://example.com/releases/current
./hydra --out ubuntu.iso https://example.com/releases/current

# SOCKS proxy with remote DNS
./hydra --proxy socks5h://127.0.0.1:1080 https://example.com/file.bin

# HTTP proxy with auth
./hydra --proxy http://user:pass@127.0.0.1:8080 https://example.com/file.bin

# Verify checksum
./hydra --checksum sha256:abc123... https://example.com/file.bin

# Force re-download
./hydra --if-exists overwrite https://example.com/file.bin

# JSON lifecycle events for your own UI
./hydra --json-events https://example.com/file.bin

# Strict validator mode: reject resume metadata if ETag/Last-Modified changed
./hydra --strict-resume-validation https://example.com/file.bin

# More precise resume after interruption
./hydra -s 16 -x 16 -k 20M --resume-block-size 512K https://example.com/big.iso
```

## Library one-shot API

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/divyam234/hydra"
)

func main() {
    opts := hydra.DefaultOptions()
    opts.Split = 16
    opts.MaxConnectionsPerServer = 16
    opts.MinSplitSize = 20 << 20
    opts.Proxy = "socks5h://127.0.0.1:1080"
    opts.OnProgress = func(p hydra.Progress) {
        fmt.Printf("%.1f%% %.1f MiB/s\n", float64(p.Completed)*100/float64(p.Total), p.Speed/1024/1024)
    }

    checksum, _ := hydra.ParseChecksum("sha256:...")
    res, err := hydra.Download(context.Background(), hydra.Request{
        URLs: []string{"https://example.com/big.iso"},
        Dir: "downloads",
        Out: "big.iso",
        Checksum: checksum,
    }, opts)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(res.Path, res.Verified)
}
```

## Queue manager API

```go
ctx := context.Background()
opts := hydra.DefaultOptions()
opts.MaxConcurrentDownloads = 4
opts.Split = 8

mgr, err := hydra.NewManager(ctx, opts)
if err != nil { panic(err) }
defer mgr.Close()

id1, _ := mgr.Enqueue(hydra.Request{URLs: []string{"https://example.com/a.bin"}, Dir: "downloads"})
id2, _ := mgr.Enqueue(hydra.Request{URLs: []string{"https://example.com/b.bin"}, Dir: "downloads"})

for _, r := range mgr.WaitAll(ctx, id1, id2) {
    if r.Err != nil {
        fmt.Println("failed", r.ID, r.Err)
        continue
    }
    fmt.Println("done", r.ID, r.Result.Path)
}
```

## Important options

| Option | Meaning |
| --- | --- |
| `Split` | Maximum number of large HTTP range jobs per file. |
| `MaxConnectionsPerServer` | Per-host connection limit. |
| `MaxConcurrentDownloads` | Queue concurrency used by `Manager`. |
| `MinSplitSize` | Avoids wasteful splitting for small files. Default: 20MiB. |
| `ResumeBlockSize` | One bit in `.hydra` represents this many bytes. Smaller means more precise resume; default: 1MiB. |
| `MetadataFlushInterval` | Batches `.hydra` sidecar fsyncs during segmented downloads; final save is still forced. Default: 2s. |
| `BufferSize` | Copy buffer size per active connection. Default: 256KiB. Buffers are pooled. |
| `Proxy` | `http://`, `https://`, `socks5://`, or `socks5h://`. |
| `ExistingFile` | `resume`, `overwrite`, `skip`, or `error`. |
| `DisableResume` | Disable `.part` / `.hydra` continuation. |
| `StrictResumeValidation` | Reject `.hydra` metadata when ETag/Last-Modified changed. Default is false for CDN/signed URL compatibility. |
| `OnProgress` | Throttled progress snapshots. |
| `OnEvent` | Lifecycle and retry events. |
| `Transport` | Bring your own `http.RoundTripper`. |

## Split size vs resume block size

`Split` / `-s` and `MinSplitSize` / `-k` decide how large the HTTP `Range` requests are and how many concurrent range jobs can exist.

`ResumeBlockSize` only decides how many bytes one bit in the `.hydra` bitfield represents. The scheduler rounds large range jobs to block boundaries so a completed bit is safe, but it does **not** turn a 1MiB resume block into thousands of 1MiB HTTP requests.

## Resume behavior

For segmented downloads, the final output file is preallocated and progress is tracked in `filename.hydra`. A set bit means that block is complete and durable on disk. Do not delete the `.hydra` sidecar if you want continuation after Ctrl+C.

On cancellation, Hydra fsyncs the output file, saves the bitfield sidecar, and exits with `context canceled`. Running the same command again skips completed blocks and resumes from the next missing block boundary.

If a server supports byte ranges but does not send `Accept-Ranges` on `HEAD`, Hydra performs a one-byte ranged `GET` probe so resume and segmentation still work. If a CDN or signed URL changes ETag/Last-Modified on every request, resume still works by default as long as path and size match. Use `--strict-resume-validation` or `Options.StrictResumeValidation` when you prefer to fail instead of resuming across changed validators.

## Performance notes

- Use `-s 16 -x 16 -k 20M` for large files on good networks.
- Use `--resume-block-size 512K` or `1M`; smaller blocks reduce re-download after interruption but update metadata more often.
- Keep `BufferSize` around 256KiB to 1MiB. Larger is not always faster and increases memory per active connection.
- `MetadataFlushInterval` defaults to 2s to avoid fsyncing the sidecar on every completed resume block.
- The core copy path uses pooled buffers and `WriteAt`, so memory stays roughly `active_connections * BufferSize` plus small metadata.

## Current scope

Hydra focuses on HTTP/HTTPS downloads. These are future extensions, not core requirements:

- cookie persistence jar API;
- HTTP/2 prioritization knobs;
- adaptive mirror scoring;
- rate limiting;
- daemon/RPC server.
