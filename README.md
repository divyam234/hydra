# Hydra

An HTTP/HTTPS downloader with parallel connections, resumable downloads, and checksum verification. Available as a command-line tool and a Go package.

## Features

- Parallel downloads when the server supports byte ranges.
- Resume interrupted downloads with checks for changed or damaged data.
- Mirror failover and automatic retries.
- HTTP, HTTPS, SOCKS5, and SOCKS5H proxies.
- SHA-256, SHA-512, SHA-1, and MD5 checksum verification.
- Progress display with transfer speed and estimated time remaining.
- Go API with a download queue, progress callbacks, and events.

## Install

With Go installed:

```bash
go install github.com/divyam234/hydra/cmd/hydra@latest
```

Or build from source:

```bash
go build -o hydra ./cmd/hydra
```

## Usage

```bash
hydra https://example.com/file.zip
```

Choose a filename and output directory:

```bash
hydra -d downloads -o archive.zip https://example.com/file.zip
```

Use more connections for a large file:

```bash
hydra -s 16 -x 16 https://example.com/big.iso
```

Provide mirrors for the same file:

```bash
hydra -o file.zip https://example.com/file.zip https://mirror.example.com/file.zip
```

Use a proxy or add a request header:

```bash
hydra --proxy socks5h://127.0.0.1:1080 https://example.com/file.zip
hydra -H 'Authorization: Bearer YOUR_TOKEN' https://example.com/file.zip
```

Verify a download using a complete checksum:

```bash
hydra --checksum "sha256:$SHA256" https://example.com/file.zip
```

Set `SHA256` to the expected hexadecimal digest from the file's publisher.

### Resume

Press Ctrl+C to stop a download, then run the same command to resume. Keep the `.hydra` metadata and any `.part` files alongside the download.

Resume requires a matching strong ETag from the server. If the server cannot identify the file reliably, or the saved data fails validation, Hydra restarts the affected data rather than trusting it. Parallel downloads require a strong ETag or a supplied checksum; otherwise Hydra uses a single connection.

### Options

| Flag | Description | Default |
| --- | --- | --- |
| `-d` | Output directory | Current directory |
| `-o`, `--out` | Output filename | From URL |
| `-s` | Requested parallel connections | `5` |
| `-x` | Maximum connections per server | `8` |
| `-k` | Minimum split size | `20M` |
| `--if-exists` | `resume`, `overwrite`, `skip`, or `error` | `resume` |
| `--proxy` | Proxy URL | Environment proxy settings |
| `--checksum` | Expected checksum as `algorithm:hex` | None |
| `--retry` | Maximum retries per piece; `0` disables retries | `5` |
| `--timeout` | Timeout for each entire HTTP request, including the body | `1m` |
| `--quiet` | Hide the progress display | Off |
| `--json-events` | Write JSON events to stderr | Off |

For large files on slow connections, increase the request timeout, for example `--timeout 30m`.

Run `hydra --help` for all options. If you built from source, use `./hydra` instead of `hydra`.

## Benchmarks

Hydra engine at `61358c5` vs aria2c 1.37.0 on Linux/ARM64, built with Go 1.26.9 (`CGO_ENABLED=0`). Median throughput over three runs downloading a 128 MiB file from a local HTTP server; higher is better.

| Local source | Connections | Hydra | aria2c |
| --- | ---: | ---: | ---: |
| Unrestricted | 1 | 408.7 MiB/s | 233.0 MiB/s |
| Unrestricted | 8 | 305.7 MiB/s | 239.7 MiB/s |
| Limited to 8 MiB/s per connection | 1 | 8.0 MiB/s | 7.9 MiB/s |
| Limited to 8 MiB/s per connection | 8 | 50.6 MiB/s | 59.2 MiB/s |

Timing includes process startup and file sync; all 24 downloads passed SHA-256 verification after timing. These loopback results include server and filesystem overhead, not internet performance. Hydra was faster in the unrestricted tests; aria2c was faster with eight throttled connections.

[Methodology and reproduction](BENCHMARKS.md#current-version-comparison) · [Raw results](docs/benchmark-local.json)

## Documentation

- [Go API usage](docs/go-api.md)
- [Benchmarks](BENCHMARKS.md)
