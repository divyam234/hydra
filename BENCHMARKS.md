# Downloader benchmarks: Hydra, aria2c, Surge

November 19, 2026 · ARM64 Linux · HTTP/1.1 loopback

[Raw measurements (105 runs)](docs/benchmark-three-way.csv) · [Runner](scripts/benchmark_three_way.py) · [Test server](scripts/benchmark_server.go)

## Setup

| Parameter | Value |
| --- | --- |
| Hydra | Commit [`4c61022`](https://github.com/divyam234/hydra/commit/4c6102262b6cf638feb52b4284e86b6a661f4ce6), Go 1.27.3, `CGO_ENABLED=0` |
| aria2c | 1.37.0, `--no-conf` |
| SurgeDM | Commit `474a989e3569` |
| Host | ARM64 NixOS, Neoverse-N1; 2 available CPUs; `GOMAXPROCS=2` |
| File | 256 MiB of zero bytes |
| SHA-256 | `a6d72ac7690f53be6ae46ba88506bd97302a093f7108472bd9efc3cefda06484` |
| Connections | 4, 8, 16 for uniform tests; 16 for slow-range test |
| Split size | 1 MiB minimum configured for each client |
| Repetitions | 5 per downloader, scenario, and connection count |

The local server supports HTTP ranges and supplies a strong ETag. Each run uses a fresh output directory. Client order is randomized per round. Tables give arithmetic means.

Wall time includes process startup and download. CPU seconds are process user + system time from GNU Time; memory is peak process RSS. The server's CPU and filesystem cache are not included in these measurements. The independent SHA-256 check runs *after* timing, without a checksum supplied to the downloader.

All 105 downloaded files passed that verification.

## 1. Throttled HTTP

8 MiB/s cap per GET; 40 ms delay per request. 45 runs.

| Connections | Downloader | MiB/s | Wall (s) | CPU (s) | RSS (MiB) |
| ---: | --- | ---: | ---: | ---: | ---: |
| 4 | Hydra | 31.42 | 8.147 | 0.336 | 7.44 |
| 4 | aria2c | 30.84 | 8.300 | 0.498 | 20.84 |
| 4 | Surge | 30.30 | 8.449 | 0.582 | 33.77 |
| 8 | Hydra | 61.06 | 4.193 | 0.302 | 8.54 |
| 8 | aria2c | 59.16 | 4.327 | 0.484 | 24.79 |
| 8 | Surge | 56.47 | 4.536 | 0.772 | 42.27 |
| 16 | Hydra | 113.07 | 2.265 | 0.308 | 11.39 |
| 16 | aria2c | 109.62 | 2.336 | 0.468 | 31.52 |
| 16 | Surge | 101.05 | 2.538 | 1.110 | 46.94 |

## 2. Unthrottled HTTP

No server-side rate cap or artificial delay. 45 runs.

| Connections | Downloader | MiB/s | Wall (s) | CPU (s) | RSS (MiB) |
| ---: | --- | ---: | ---: | ---: | ---: |
| 4 | Hydra | 567.99 | 0.463 | 0.260 | 7.64 |
| 4 | aria2c | 304.96 | 0.840 | 0.480 | 20.81 |
| 4 | Surge | 386.76 | 0.687 | 0.692 | 37.09 |
| 8 | Hydra | 572.39 | 0.458 | 0.262 | 8.81 |
| 8 | aria2c | 294.15 | 0.873 | 0.522 | 24.44 |
| 8 | Surge | 288.89 | 0.906 | 1.118 | 42.27 |
| 16 | Hydra | 609.28 | 0.424 | 0.256 | 11.71 |
| 16 | aria2c | 296.32 | 0.868 | 0.524 | 30.04 |
| 16 | Surge | 349.63 | 0.786 | 0.988 | 46.77 |

Hydra made more HTTP requests when splitting active ranges. At 16 connections, the server sent an average of 8.9% extra bytes for Hydra, versus 3.2% for aria2c. Short run times are sensitive to system load.

## 3. One slow range

The range starting at byte 251658240 (240 MiB) is capped at 2 MiB/s. Other GETs are capped at 8 MiB/s. Request delay is 40 ms. 16 requested connections; 15 runs.

| Downloader | MiB/s | Wall (s) | CPU (s) | RSS (MiB) | GETs | Peak concurrent GETs |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Hydra | 84.52 | 3.029 | 0.314 | 11.39 | 26 | 16 |
| aria2c | 93.52 | 2.737 | 0.474 | 30.07 | 27 | 17 |
| Surge | 81.76 | 3.135 | 0.888 | 46.98 | 24.8 | 16 |

aria2c finished about 0.29 seconds before Hydra. It briefly reached 17 concurrent GETs against a requested 16.

## Reproduce

Requires Linux, Python 3.9+, Go, GNU Time, aria2c, and SurgeDM. Run from the Hydra repository root:

```sh
python3 scripts/benchmark_three_way.py --surge /path/to/surge
```

The script builds Hydra and the Go test server, runs all three scenarios, and writes `benchmark-results/raw.csv`, `summary.json`, `metadata.json`, and per-run `*.time` files. Temporary downloads are deleted after verification.

For a short test:

```sh
python3 scripts/benchmark_three_way.py \
  --surge /path/to/surge \
  --size-mib 16 --repeats 1 --connections 4,8 --skew-connections 4
```

The published measurements used the original local server with equivalent rate and range settings. The bundled server is a replacement, not the same binary. Results vary by host, runtime, kernel, filesystem, and load.

These tests use localhost and a cached file, not WAN HTTPS or an uncached disk. They measure fresh downloads rather than interruption and resume.
