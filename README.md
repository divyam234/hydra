# Hydra

An HTTP/HTTPS downloader with parallel connections, resumable downloads, and checksum verification. Available as a command-line tool and a Go package.

## Features

- Parallel downloads when the server supports byte ranges.
- Resume interrupted downloads using completion bitmaps and strong HTTP ETag identity checks.
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

Resume metadata v4 stores completed blocks in a bitmap, without per-block hashes. Hydra checks the server's ETag and file size before resuming but cannot detect corruption in saved blocks. Use `--checksum` with a publisher-supplied hash for full-file verification. Parallel downloads require a strong ETag or a supplied checksum.

Checkpoints from v3 or earlier are restarted instead of reused.

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

256 MiB HTTP downloads on ARM64, November 19, 2026. Hydra `4c61022`, aria2c 1.37.0, SurgeDM `474a989`. Five runs per setting; 16 requested connections. CPU is process user + system time; RAM is peak RSS.

| Scenario | Downloader | MiB/s | CPU (s) | RSS (MiB) |
| --- | --- | ---: | ---: | ---: |
| 8 MiB/s per connection | Hydra | 113.07 | 0.308 | 11.39 |
| 8 MiB/s per connection | aria2c | 109.62 | 0.468 | 31.52 |
| 8 MiB/s per connection | Surge | 101.05 | 1.110 | 46.94 |
| Unthrottled | Hydra | 609.28 | 0.256 | 11.71 |
| Unthrottled | aria2c | 296.32 | 0.524 | 30.04 |
| Unthrottled | Surge | 349.63 | 0.988 | 46.77 |
| One slow connection | Hydra | 84.52 | 0.314 | 11.39 |
| One slow connection | aria2c | 93.52 | 0.474 | 30.07 |
| One slow connection | Surge | 81.76 | 0.888 | 46.98 |

Local HTTP tests with filesystem caching, not WAN measurements. All 105 outputs passed SHA-256 verification after timing.

[Methodology and all results](BENCHMARKS.md) · [Raw CSV](docs/benchmark-three-way.csv) · [Benchmark script](scripts/benchmark_three_way.py)

## Documentation

- [Go API usage](docs/go-api.md)
- [Benchmarks](BENCHMARKS.md)
