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
- Resume verifies saved bytes with SHA-256 and requires a matching strong ETag; unsafe legacy checkpoints restart.
- HTTP/HTTPS proxy support.
- SOCKS5 and SOCKS5H proxy support through Go's HTTP transport (no external proxy dependency).
- Mirror URL rotation, observed throughput/error scoring, and retry failover.
- Exponential retry backoff with bounded jitter and `Retry-After` support.
- SHA-256, SHA-512, SHA-1, and MD5 verification.
- Existing-file policies: `resume`, `overwrite`, `skip`, `error`.
- Importable package API.
- Queue manager API for multiple downloads.
- Lifecycle events for UIs, logs, metrics, RPC servers, or JSON streams.
- Rich progress snapshots with speed, average speed, ETA, active connections, retries, resumed bytes, and downloaded bytes.
- Clean terminal progress UI without noisy internal block counters.
- File lock guard to prevent multiple writers for the same target.
- Dedicated metadata flushing to keep sidecar serialization off download workers while preserving safe cancellation resume.
- Bounded over-decomposed range queues so idle workers can consume remaining work without increasing connection count.
- Best-effort filesystem preallocation with a portable truncate fallback.
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

One-shot `Download` and `Manager.Close` release idle connections in transports they create. A caller-supplied `Options.Transport` remains caller-owned and is not closed automatically. When using a reusable `Downloader` from `New`, call `CloseIdleConnections()` when finished with its pooled connections.

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
| `MetadataFlushInterval` | Batches `.hydra` sidecar fsyncs; partial/debug state is saved when stopping, completed outputs are synced before cleanup. Default: 2s. |
| `BufferSize` | Copy buffer size per active connection. Default: 256KiB. Buffers are pooled. |
| `Proxy` | `http://`, `https://`, `socks5://`, or `socks5h://`. |
| `ExistingFile` | `resume`, `overwrite`, `skip`, or `error`. |
| `DisableRetries` | Make one attempt per mirror; CLI equivalent: `--retry 0`. `MaxRetries: 0` alone retains zero-value defaults. |
| `DisableResume` | Disable `.part` / `.hydra` continuation. |
| `OnProgress` | Throttled progress snapshots. |
| `OnEvent` | Lifecycle and retry events. |
| `Transport` | Bring your own `http.RoundTripper`. |

## Split size vs resume block size

`Split` / `-s` and `MinSplitSize` / `-k` decide how large the HTTP `Range` requests are and how many concurrent range jobs can exist.

`ResumeBlockSize` only decides how many bytes one bit in the `.hydra` bitfield represents. The scheduler rounds large range jobs to block boundaries so a completed bit is safe, but it does **not** turn a 1MiB resume block into thousands of 1MiB HTTP requests.

## Resume behavior

For segmented downloads, the final output file is preallocated and progress is tracked in `filename.hydra`. Version 3 stores a completion bit and SHA-256 for each saved block. On resume, Hydra checks the actual bytes and redownloads damaged blocks. Older checkpoints restart safely because they lack authenticated block data. Do not delete the `.hydra` sidecar if you want continuation after Ctrl+C.

On cancellation, Hydra fsyncs the output file, saves the bitfield sidecar, and exits with `context canceled`. Running the same command again skips completed blocks and resumes from the next missing block boundary.

`Result.ResumedBytes` reports bytes already present and `Result.Downloaded` reports bytes transferred toward the completed output in this invocation. `Result.AvgSpeed` and completion progress exclude resumed bytes; they do not claim previously downloaded data as new network throughput.

With an explicit output filename, `skip` and `error` policies check existing files locally before contacting the origin. Explicit `skip` keeps the existing file regardless of a differing remote size; requested checksums and expected local size are still enforced. Overwrite and sidecar cleanup never intentionally remove directories.

If a server supports byte ranges but does not send `Accept-Ranges` on `HEAD`, Hydra performs a one-byte ranged `GET` probe for segmentation. Segmentation requires a strong ETag or an explicitly supplied final checksum; otherwise Hydra uses one stream. Cross-run continuation always requires a matching strong ETag. Missing, weak, or changed identity causes a safe restart, not byte mixing. Single-stream `.part` continuation also requires a saved prefix hash; bare or damaged partial files restart. These integrity checks are always enabled.

Hydra validates every `206 Partial Content` response against the requested byte range and expected object size. Missing or inconsistent `Content-Range` headers are rejected before bytes reach the final file. If a server ignores a single-stream resume request and returns `200 OK`, Hydra safely truncates the `.part` file and restarts from byte zero. Empty-body statuses such as `204 No Content` are reported as HTTP errors and never published as completed downloads.

If an origin advertises ranges but ignores segmented requests, Hydra falls back to a single stream. Malformed range responses fail over to other mirrors rather than repeatedly retrying that broken origin. Local file-write errors are not retried as network failures.

## Performance notes

- Use `-s 16 -x 16 -k 20M` for large files on good networks.
- `--timeout` limits the entire HTTP request, including its response body, not just idle time. For very large ranges on slow links, raise it (for example `--timeout 30m`) so normal streaming does not exhaust the retry budget.
- Use `--resume-block-size 512K` or `1M`; smaller blocks reduce re-download after interruption but update metadata more often.
- Keep `BufferSize` around 256KiB to 1MiB. Larger is not always faster and increases memory per active connection.
- `MetadataFlushInterval` defaults to 2s to avoid fsyncing the sidecar on every completed resume block.
- The core copy path uses pooled buffers and `WriteAt`, so memory stays roughly `active_connections * BufferSize` plus small metadata.
- Segmented downloads queue more bounded range jobs than workers, reducing slow-tail completion while keeping the configured connection count fixed.
- Mirror ranking is learned in memory from observed throughput and failures; unobserved mirrors are still explored fairly.

### Reproduce performance measurements

With Python 3.9+ and `aria2c` installed:

```bash
CGO_ENABLED=0 go build -o /tmp/opencode/hydra ./cmd/hydra
python3 scripts/benchmark.py --hydra /tmp/opencode/hydra --directory /tmp/opencode --repeats 3 --output /tmp/opencode/hydra-benchmark.json
go test -run '^$' -bench . -benchmem ./...
```

The comparison uses identical deterministic content, SHA-256 verification, one/eight connections, and both unrestricted loopback and per-connection throttling. It includes process startup and output fsync for both tools. The filesystem baseline measures buffered writes plus fsync and **cached** reads, not raw device bandwidth. Set `--directory` to a directory on the disk you want to measure; run on an otherwise idle machine. `--baseline` optionally compares an older Hydra executable too.

These are controlled local measurements, not a claim about internet bandwidth or the world's fastest downloader. See [BENCHMARKS.md](BENCHMARKS.md) for audit results and limitations.

For a real-network comparison, supply a trusted, stable file with `--url`. For example, OVH's 10 MiB speed-test file (about 130 MiB total traffic with two tools and three repetitions):

```bash
python3 scripts/benchmark.py --hydra /tmp/opencode/hydra --url https://proof.ovh.net/files/10Mb.dat --size-mib 16 --repeats 3 --output /tmp/opencode/hydra-wan.json
```

`--size-mib` caps the reference download in this mode. Real-network results include routing, server load, TLS/protocol differences, and connection startup; they do not isolate downloader CPU efficiency.

Checksums must contain a supported algorithm and a complete hexadecimal digest; malformed checksums are rejected before any download request. With `--json-events`, configuration and transfer failures after flag parsing remain JSON events rather than mixing plain-text errors into the event stream.

## Current scope

Hydra focuses on HTTP/HTTPS downloads. These are future extensions, not core requirements:

- cookie persistence jar API;
- HTTP/2 prioritization knobs;
- persistent mirror scoring across process restarts;
- rate limiting;
- daemon/RPC server.
