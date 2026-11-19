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

Resume uses a completion bitmap (sidecar format v4). Each completed range block is reused only when the server returns the same strong ETag and file size. Hydra does **not** compute SHA-256 or CRC32C for resume blocks and does **not** rehash previously saved bytes. A missing or changed strong ETag causes a restart, but silent local corruption cannot be detected by the bitmap alone. Supply a **publisher-provided full-file checksum** with `--checksum` to verify the result. Parallel downloads require a strong ETag or a supplied checksum; otherwise Hydra uses a single connection.

On the first resume after upgrading from an older v3 (or earlier) checkpoint, Hydra discards the old progress bits and downloads those bytes again. It does not silently reinterpret the old SHA-256 sidecar as an unchecked bitmap.

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

Latest ARM64 HTTP benchmark, **Hydra `4c61022` vs aria2c 1.37.0 vs SurgeDM `474a989`**. All results are means over five independent 256 MiB downloads per configuration, with CPU time and peak process RAM measured separately. All 105 downloads passed independent SHA-256 verification after timing.

| Scenario (16 requested connections) | Downloader | Speed | CPU time | Peak RAM |
| --- | --- | ---: | ---: | ---: |
| Uniform 8 MiB/s per GET | **Hydra** | **113.07 MiB/s** | **0.308 s** | **11.39 MiB** |
| Uniform 8 MiB/s per GET | aria2c | 109.62 MiB/s | 0.468 s | 31.52 MiB |
| Uniform 8 MiB/s per GET | Surge | 101.05 MiB/s | 1.110 s | 46.94 MiB |
| Unthrottled loopback | **Hydra** | **609.28 MiB/s** | **0.256 s** | **11.71 MiB** |
| Unthrottled loopback | aria2c | 296.32 MiB/s | 0.524 s | 30.04 MiB |
| Unthrottled loopback | Surge | 349.63 MiB/s | 0.988 s | 46.77 MiB |
| One slow range | Hydra | 84.52 MiB/s | **0.314 s** | **11.39 MiB** |
| One slow range | **aria2c** | **93.52 MiB/s** | 0.474 s | 30.07 MiB |
| One slow range | Surge | 81.76 MiB/s | 0.888 s | 46.98 MiB |

These are synthetic, locally served, page-cache-assisted HTTP results—not promises of real internet speeds. In the slow-range test, aria2c is faster, although Hydra uses less CPU and RAM.

[Full three-scenario report](BENCHMARKS.md) · [105-run raw dataset](docs/benchmark-three-way.csv) · [Reproducible three-way script](scripts/benchmark_three_way.py)

## Documentation

- [Go API usage](docs/go-api.md)
- [Benchmarks](BENCHMARKS.md)
