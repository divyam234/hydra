# Hydra vs aria2c vs Surge — three-way benchmarks

**Measured:** November 19, 2026 · **Hydra:** [`4c61022`](https://github.com/divyam234/hydra/commit/4c6102262b6cf638feb52b4284e86b6a661f4ce6) (bitmap-only resume + adaptive work stealing).

These are the latest measured CPU, memory and download-throughput comparisons for **Hydra (Go)**, **aria2c** and **SurgeDM (Go)**. This report replaces older, incompatible results from Hydra versions that hashed every resume block.

[Raw results — 105 verified trials](docs/benchmark-three-way.csv) · [Three-way benchmark script](scripts/benchmark_three_way.py) · [Local Go HTTP test server](scripts/benchmark_server.go)

## Test environment and reproducibility

- **Host:** ARM64 NixOS (Neoverse-N1; `nproc=2` available CPUs), Go 1.27.3, `GOMAXPROCS=2`; Hydra compiled with `CGO_ENABLED=0`.
- **Hydra:** `github.com/divyam234/hydra`, Git commit `4c6102262b6cf638feb52b4284e86b6a661f4ce6`.
- **aria2c:** version 1.37.0, invoked with `--no-conf`.
- **SurgeDM:** `github.com/SurgeDM/Surge`, Git commit `474a989e3569`.
- **Source:** immutable 256 MiB zero-filled file served locally over HTTP/1.1 with byte-range support and a strong ETag. Independent SHA-256: `a6d72ac7690f53be6ae46ba88506bd97302a093f7108472bd9efc3cefda06484`.
- **Connections:** requested limits 4, 8 and 16 for uniform-speed tests, 16 for the slow-connection test. The *observed* maximum concurrent HTTP GETs are recorded separately; in the slow-connection test aria2c briefly reached **17**, despite a requested limit of 16.
- **Minimum chunk/split:** configured to 1 MiB for each client; the clients choose their own actual request partitioning.
- **Trial design:** five independent runs per client and connection setting, deterministic randomized client order, fresh output directories. All numbers below are arithmetic means of the runs.
- **Timing:** includes process startup, download, file writes and sync. `GNU time` measures **user + system CPU time** (total CPU seconds, not CPU utilization) and **per-process peak RSS** (MiB). Wall time and derived transfer speed are measured by the Python harness.
- **Integrity:** all **105/105** full outputs passed a separate SHA-256 verification **after** timing. This validation does not count toward any downloader's CPU seconds or wall time. No full-file checksum was supplied to any downloader during its timed run.
- **Limitations:** the HTTP source and clients run on the **same machine** with a warm filesystem page cache. These results are not proof of equivalent performance over real WAN HTTPS, slow disks or concurrent multi-file downloads. Server CPU and system-wide page-cache memory are excluded.

## Scenario 1 — Uniformly throttled HTTP

Each HTTP GET is capped at **8 MiB/s**, with **40 ms** of simulated per-request latency. Five repeats per configuration (**45 trials**).

| Requested connections | Downloader | Throughput (MiB/s) | Wall (s) | CPU (s) | Peak RSS (MiB) |
|---:|---|---:|---:|---:|---:|
| 4 | **Hydra** | **31.42** | **8.147** | **0.336** | **7.44** |
| 4 | aria2c | 30.84 | 8.300 | 0.498 | 20.84 |
| 4 | Surge | 30.30 | 8.449 | 0.582 | 33.77 |
| 8 | **Hydra** | **61.06** | **4.193** | **0.302** | **8.54** |
| 8 | aria2c | 59.16 | 4.327 | 0.484 | 24.79 |
| 8 | Surge | 56.47 | 4.536 | 0.772 | 42.27 |
| 16 | **Hydra** | **113.07** | **2.265** | **0.308** | **11.39** |
| 16 | aria2c | 109.62 | 2.336 | 0.468 | 31.52 |
| 16 | Surge | 101.05 | 2.538 | 1.110 | 46.94 |

At 16 connections Hydra consumed **34% less CPU** and **64% less RSS** than aria2c, with approximately **3% more throughput**.

## Scenario 2 — Unthrottled local HTTP

No artificial bandwidth limit or request delay. Five repeats per configuration (**45 trials**).

| Requested connections | Downloader | Throughput (MiB/s) | Wall (s) | CPU (s) | Peak RSS (MiB) |
|---:|---|---:|---:|---:|---:|
| 4 | **Hydra** | **567.99** | **0.463** | **0.260** | **7.64** |
| 4 | aria2c | 304.96 | 0.840 | 0.480 | 20.81 |
| 4 | Surge | 386.76 | 0.687 | 0.692 | 37.09 |
| 8 | **Hydra** | **572.39** | **0.458** | **0.262** | **8.81** |
| 8 | aria2c | 294.15 | 0.873 | 0.522 | 24.44 |
| 8 | Surge | 288.89 | 0.906 | 1.118 | 42.27 |
| 16 | **Hydra** | **609.28** | **0.424** | **0.256** | **11.71** |
| 16 | aria2c | 296.32 | 0.868 | 0.524 | 30.04 |
| 16 | Surge | 349.63 | 0.786 | 0.988 | 46.77 |

At 16 connections Hydra transferred about **2.06× as fast** as aria2c while consuming **51% less CPU** and **61% less RSS** on this local workload. Note that adaptive splitting can trigger extra requests and discarded server bytes: Hydra consumed a mean **8.9%** more bytes on the server side than the file size in this scenario, compared with **3.2%** for aria2c. Subsecond runs exhibit noticeable relative noise.

## Scenario 3 — One slow HTTP range

One GET beginning at byte **251658240** (240 MiB of a 256 MiB file) is capped at **2 MiB/s**; all other GETs remain capped at **8 MiB/s**, with 40 ms request latency. **16 requested connections**, five repeats (**15 trials**).

| Downloader | Throughput (MiB/s) | Wall (s) | CPU (s) | Peak RSS (MiB) | Mean GETs | Peak GETs |
|---|---:|---:|---:|---:|---:|---:|
| Hydra | 84.52 | 3.029 | **0.314** | **11.39** | 26 | 16 |
| **aria2c** | **93.52** | **2.737** | 0.474 | 30.07 | 27 | 17 |
| Surge | 81.76 | 3.135 | 0.888 | 46.98 | 24.8 | 16 |

**aria2c is approximately 11% faster than Hydra in this deliberately uneven test.** Hydra still uses 34% less CPU and 62% less RAM; improving slow-worker range redistribution is a reasonable next optimization.

## Running all three benchmarks

**Requirements:** Linux, Python 3.9+, Go, [aria2c](https://aria2.github.io/), [SurgeDM](https://github.com/SurgeDM/Surge) CLI, and **GNU Time**. Python uses only its standard library. Run from the Hydra repository root. The runner builds Hydra from the checked-out source with `CGO_ENABLED=0` and builds the bundled Go HTTP test server automatically; no pre-existing server or payload is required.

```bash
# Surge must be installed and available as a CLI binary.
# Set the path explicitly if it is not on PATH:
python3 scripts/benchmark_three_way.py --surge /path/to/surge
```

Defaults reproduce **256 MiB**, **five repetitions**, and **4/8/16 connections** in the two uniform scenarios, plus **16 connections** with one slow range. Output appears under `benchmark-results/` as a combined `raw.csv`, `summary.json`, `metadata.json` (including binary SHA-256 hashes), and per-run GNU Time records. All temporary downloaded files and the HTTP test server are removed on completion.

For a quick smoke test before a complete run:

```bash
python3 scripts/benchmark_three_way.py \
  --surge /path/to/surge \
  --size-mib 16 --repeats 1 --connections 4,8 --skew-connections 4
```

Other useful options include `--hydra /path/to/hydra`, `--gnu-time /path/to/time`, `--scenarios throttled unthrottled skew`, `--output-dir /some/path`, and `--timeout-seconds 120`. Do not compare a smoke test using a 16 MiB file with the published 256 MiB results.

**Reproduction caveat:** the new bundled HTTP fixture preserves the earlier Go test server's range handling, artificial rate limits and delays. The published figures were measured using the original local benchmark server and binaries on the ARM64 host; new runs may differ due to compiler/runtime, kernel, scheduler and cache state. Always use newly generated results for claims about another machine.

The historical performance optimization experiments are deliberately not included here. This page, the single three-way runner, and the raw dataset are the maintained benchmark artifacts.
