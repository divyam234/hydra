# Correctness and performance audit

## Current-version comparison

Measured on 2026-10-20 using the Hydra engine at `61358c5`, Go 1.26.9 with `CGO_ENABLED=0`, and aria2c 1.37.0 on Linux 7.2.10 / ARM64. The only working-tree change at build time was the benchmark server's strong ETag header; downloader source was unchanged. The ETag identifies the deterministic payload and allows current Hydra to use segmented downloads rather than fall back to a single stream.

128 MiB deterministic payload served over HTTP/1.1 loopback. Medians of three runs, in MiB/s:

| Source | Configured connections | Hydra | aria2c |
| --- | ---: | ---: | ---: |
| Unrestricted | 1 | 408.7 | 233.0 |
| Unrestricted | 8 | 305.7 | 239.7 |
| 8 MiB/s per connection | 1 | 8.0 | 7.9 |
| 8 MiB/s per connection | 8 | 50.6 | 59.2 |

All 24 timed downloads passed SHA-256 verification. Timing includes startup and output file sync; checksum verification is outside the timed interval. Tool ordering rotates between repetitions. The filesystem baseline was 786.9 MiB/s for buffered writes plus fsync and 6,768.3 MiB/s for cached reads, not physical disk bandwidth.

Hydra won the unrestricted medians, while aria2c won the eight-connection throttled median. Hydra's throttled eight-connection samples ranged from 50.5 to 60.3 MiB/s; its unrestricted eight-connection result was slower than its single-connection result. More connections are not always faster. These three-sample local tests do not establish internet throughput or a universal performance advantage.

[Raw timings, commands, executable hashes, and build information](docs/benchmark-local.json) are checked into the repository. The binary reports a dirty working tree because of the benchmark-server change described above.

Reproduce with Python 3.9+, Go, and aria2c installed:

```sh
CGO_ENABLED=0 go build -o /tmp/opencode/hydra-readme-bench ./cmd/hydra
python3 scripts/benchmark.py --hydra /tmp/opencode/hydra-readme-bench --directory /tmp/opencode --size-mib 128 --repeats 3 --output /tmp/opencode/hydra-readme-benchmark.json
```

Create `/tmp/opencode` first if needed, or substitute an existing directory on the filesystem you want to measure. The audit sections below retain earlier results and their limitations; they are not measurements of this revision.

## Scope and evidence

This audit covers the existing HTTP/HTTPS engine, CLI, resume metadata, queue manager, proxies, progress reporting, and local filesystem/HTTP performance. It does not certify that all bugs are gone or that Hydra is the fastest downloader on every workload.

New regression coverage includes:

- Missing/truncated segmented outputs with apparently complete sidecars: restart instead of publishing holes.
- Unknown-length responses with `ExpectedSize`: validate the downloaded length before publishing.
- Known-empty remote files versus nonempty existing outputs.
- Repeatable queue `Wait`, simultaneous `Close` calls, enqueue/shutdown races, and canceled managers.
- File-data sync before publishing completed metadata bits, including simulated sync failure.
- Dirty metadata flushing even if workers stop producing new checkpoints.
- Single-connection resume without an advertised `Accept-Ranges` header.
- Malformed range probes, unsafe server filenames, invalid request policies, and CLI size overflow.
- Linux preallocation shrinking an existing oversized output.
- Removing stale segmented metadata after a successful single-stream download.
- Failing over to healthy mirrors after permanent HTTP errors from another mirror, in single and segmented modes.
- HTTPS/HTTP2 segmented transfers through a trusted test transport.
- Environment proxy wiring and seeded/fuzzed `Content-Range` parsing.
- Near-int64-limit block/range arithmetic and resource limits on untrusted sizes and sidecars.
- Final result/completion progress speeds exclude pre-existing resumed bytes, with explicit `ResumedBytes` and `Downloaded` result counters.
- Fresh single-stream downloads interrupted by non-range servers restart safely when a retry's Range request is ignored.
- CLI subprocess tests reproduce and guard `--retry 0`, JSON-only failure events, and checksum rejection before network activity. Retry jitter stays below its configured cap and extreme delay/header values do not overflow.
- Windows directory checkpoint syncing no longer calls `FlushFileBuffers` on Go's read-only directory handle. Regular output and sidecar file syncs remain enabled; directory durability is best effort on Windows.
- Concurrent generated job IDs are unique: a 32-worker test reproduced timestamp collisions in all five pre-fix runs; an atomic sequence now disambiguates the readable timestamp.
- Manager lifecycle ordering is deterministic: `queued` precedes `started`/`probing`; three pre-fix runs reproduced the reversed order.
- A falsely advertised range-capable origin falls back to a single stream instead of failing with HTTP 200. Invalid `Content-Range` and local file-write errors fail fast rather than wasting retry backoff; valid alternate mirrors can still be used.
- Known-path `skip`/`error` policies resolve existing files locally without requiring a reachable origin. Explicit skip accepts the existing file even if a probed remote size differs, but still enforces requested checksums/sizes and file locks.
- Overwrite and sidecar cleanup refuse directories; directory-shaped `.hydra`/`.part` entries are not treated as valid resume files.
- One-shot downloads and closed managers release internally owned idle transport connections. Server connection-state tests reproduced retained connections before the fix. Caller-supplied transports remain caller-owned; reusable Downloaders expose explicit idle-connection cleanup.
- Fresh files too small to split no longer incur a redundant one-byte range probe; a request-count test reproduced three requests where only HEAD + GET were needed. HEAD 501 responses now use the existing GET fallback instead of rejecting a downloadable object.

Existing coverage also exercises checksum verification, HTTP/SOCKS proxies, cancellation/resume, mirror scoring, retries, existing-file policies, and progress accounting. Single-stream downloads attempt an actual resume when a partial file exists, without adding an extra range probe to fresh downloads.

## Performance changes

- Serialize the base64 metadata bitfield when saving, not on every copy buffer/read of metadata counters.
- Advance checkpoint scan boundaries during range transfers instead of rescanning the whole cumulative range on each buffer.
- Avoid progress sampling on the quiet copy path and avoid mutating rolling-speed samples when an update is throttled.
- Preserve existing block-completion progress callbacks.
- Completed segmented transfers sync the output and directory without serializing/fsyncing a completed checkpoint immediately before deleting it. Cancellation/failure and `KeepPartFile` still persist the checkpoint after data sync.
- Remove range-job metadata-count replacement that could race with other workers' block-counter increments; block completion is counted only once per new bit.

Periodic checkpoints now sync file data before sidecars. This is a correctness cost, not an optimization to remove for benchmark scores.

A targeted `strace` comparison of `TestSegmentedDownload` confirmed **5 → 4 fsync calls** after removing the redundant successful-completion checkpoint rewrite. Output-file sync and final directory sync remain; the cancellation and kept-checkpoint regression tests also pass. This syscall count establishes reduced work, not a promise of a specific throughput gain on every disk.

## Verification results

- Original repository: `go test ./...` passed before changes. Newly added tests then reproduced missing/truncated-output corruption, unchecked unknown response lengths, and non-repeatable manager results.
- Latest source: `go test -race -coverprofile=... ./...`, `go vet ./...`, and `git diff --check` passed.
- Statement coverage after extended hardening: core package **79.8%**, CLI tests **80.9%**. CLI benchmark invocations exercise the compiled CLI separately and are not included in this coverage figure.
- `Content-Range` fuzzing completed 231,542 executions in a recorded 10-second run with no failing input (an earlier run completed 246,101).
- Missing-range scheduling fuzzing completed **411,222 executions** in 10 seconds with no failing input. It checks exact coverage of missing blocks, exclusion of completed blocks, valid bounds, and block alignment.
- Extreme-size arithmetic and metadata-allocation rejection tests pass without attempting physically enormous downloads.
- A Linux sparse-file test writes and reads back a range **above the 4 GiB boundary**, validating real 64-bit file offsets without transferring or allocating a full multi-gigabyte payload.
- `.github/workflows/test.yml` runs race tests, vet, and benchmark-harness unit tests on Linux (Go 1.25.x and stable), plus stable Go on Windows and macOS. These native-platform CI jobs are configured but have not been run on GitHub by this local audit; network benchmarks are deliberately manual.
- CGO-disabled CLI builds succeeded for all six release targets: Linux, Windows and macOS, each on AMD64 and ARM64. This is build verification, not native runtime verification on those other platforms.

Passing tests and these coverage figures are evidence for the tested cases, not proof that every bug has been eliminated. Real power-loss testing, arbitrary hostile origins, and every operating system remain separate verification tasks.

### Microbenchmarks

Linux/ARM64, Go 1.26.8, three runs per revision; medians below. Baseline is commit `bc5dbd4`. These helper benchmarks do not measure end-to-end network or physical disk throughput.

| Benchmark | Baseline | Audited | Allocations/op (before → after) |
| --- | ---: | ---: | ---: |
| Metadata checkpoint, 4 GiB bitfield | 12,676 ns/op | 9,664 ns/op | 2 → 0 |
| Copy 8 MiB to discard, quiet progress | 870,761 ns/op | 836,342 ns/op | 7 → 2 |

The checkpoint helper improved about 24%; quiet-copy improvement was about 4%, small enough to treat cautiously without a larger statistical study. Metadata allocations fell from roughly 1.46 KiB/op to zero; quiet-copy allocations fell from 2,192 to 80 bytes/op. The metadata benchmark intentionally reuses cumulative ranges, while actual transfers now also advance scan boundaries.

## End-to-end results

**Historical measurements:** these figures predate the subsequent resume-integrity fixes (v3 block hashes, authenticated single-stream prefixes, and safe single-stream fallback without a strong ETag/checksum). They do not establish throughput of the current source.

Recorded on Linux 7.2.10 / ARM64, Go 1.26.8, aria2c 1.37.0, ext4 on `/dev/vda2`. Each entry is the median of three runs with the corrected blocking-wait harness. Baseline is `bc5dbd4`; Hydra is the audited working tree including the completion-checkpoint optimization. Both Go executables use **CGO_ENABLED=0**, matching release builds; executable identities/build settings are recorded in JSON. All **54 timed downloads passed SHA-256 verification**.

### Local HTTP and filesystem

128 MiB deterministic, non-repeating payload; HTTP/1.1 loopback source. Units: MiB/s.

| Source / configured connections | Baseline Hydra | Audited Hydra | aria2c |
| --- | ---: | ---: | ---: |
| Unrestricted, 1 | 536.0 | 567.5 | 252.5 |
| Unrestricted, 8 | 495.8 | 587.0 | 296.5 |
| 8 MiB/s per connection, 1 | 8.0 | 8.0 | 7.9 |
| 8 MiB/s per connection, 8 | 61.1 | 61.0 | 59.9 |

Filesystem baseline: **821.1 MiB/s buffered write + fsync**, **6,931.5 MiB/s cached read**, medians over three 128 MiB runs. The cached read figure is not physical disk bandwidth.

Audited Hydra's unrestricted local medians were faster than aria2c and the baseline in this final run. The eight-worker Hydra range was 586.6–588.2 MiB/s; baseline results ranged from 392.8–587.3 MiB/s. Other exploratory repetitions also showed sizable variability, so this is not a statistically established universal improvement. At a per-connection network ceiling, all tools approached the imposed limit.

### Real-network HTTPS

Source: `https://proof.ovh.net/files/10Mb.dat`, 10 MiB. Reference SHA-256: `e5b844cc57f57094ea4585e235f36c78c1cd222262bb89d53c94dcb4d6b3e55d`.

| Configured connections/workers | Baseline Hydra | Audited Hydra | aria2c |
| --- | ---: | ---: | ---: |
| 1 | 51.2 | 50.2 | 53.9 |
| 8 | 46.5 | 45.8 | 40.1 |

Units: MiB/s, including startup, probing, TLS and file fsync. This small-file test measures end-to-end completion on one server/route, **not sustained link capacity**. HTTP versions and physical connection counts can differ: Go's transport may multiplex range requests over HTTP/2, whereas aria2c uses its own connection strategy. Audited eight-worker runs ranged from 44.8 to 50.9 MiB/s. **aria2c won the single-worker median**, while Hydra won the eight-worker median; the old Hydra's WAN medians were slightly higher than the audited version. Small differences and three-sample results must not be overinterpreted.

Full per-run timings, commands, executable fingerprints/build settings and hash checks remain available in this audit environment at `/tmp/opencode/hydra-final-local.json` and `/tmp/opencode/hydra-final-wan.json`. Reproduce them using the commands below; temporary files are not permanent repository artifacts.

**Conclusion:** correctness and helper overhead improved; Hydra beat aria2c in the local medians and the eight-worker public-server median, but not the single-worker WAN median. It is not uniformly faster than the old Hydra or aria2c, and this does not establish the world's fastest downloader.

## Reproduction

```sh
go test -race ./...
go vet ./...
go test -run '^$' -fuzz '^FuzzParseContentRange$' -fuzztime=10s
go test -run '^$' -fuzz '^FuzzPendingPieces$' -fuzztime=10s
go test -run '^$' -bench 'BenchmarkMetadataCheckpoint|BenchmarkCopyWithProgress' -benchmem -count=3
python3 -m unittest discover -s scripts -p 'test_*.py'
CGO_ENABLED=0 go build -o /tmp/opencode/hydra ./cmd/hydra
python3 scripts/benchmark.py --hydra /tmp/opencode/hydra --directory /tmp/opencode --size-mib 128 --repeats 3 --output /tmp/opencode/hydra-benchmark.json
python3 scripts/benchmark.py --hydra /tmp/opencode/hydra --url https://proof.ovh.net/files/10Mb.dat --size-mib 16 --repeats 3 --output /tmp/opencode/hydra-wan.json
```

The benchmark requires Python 3.9+ and aria2c, but adds no project dependencies. It creates and cleans up its own temporary download directory. Use `--baseline /path/to/older/hydra` to compare revisions. Output files are verified with SHA-256 after timing; timed runs include process startup and file fsync. Tool ordering rotates between repetitions. aria2c uses `--no-conf` to exclude personal configuration, its default disk cache, and `--file-allocation=none` to avoid slow eager allocation. Hydra retains its native preallocation/checkpoint behavior.

Process completion is measured with a blocking wait and a separate timeout watchdog. Early exploratory runs used Python's timeout-enabled polling wait, which quantized fast transfers; those end-to-end figures are discarded, not used as final results.

Use the same Go toolchain and build flags for the baseline and audited binaries. Release-style comparisons use `CGO_ENABLED=0` for both. JSON output records executable SHA-256 identities and, when Go is installed, `go version -m` build settings so mismatches can be inspected.

## Limits

- Loopback performance includes the Python HTTP server and OS page cache, not just downloader overhead. The throttled mode simulates a per-connection rate ceiling, not WAN latency, loss, or congestion.
- Buffered write plus fsync measures the selected filesystem; cached reads are **not** physical disk read bandwidth. No global cache dropping or raw-device writes are performed.
- Data-before-metadata ordering is regression-tested with a sync hook. Actual power-loss recovery requires filesystem/device-specific crash testing and is not established here.
- Resume always validates a matching strong ETag and saved-byte hashes. Changed or missing identity causes a restart; damaged blocks are redownloaded. A publisher-provided checksum additionally verifies final content against an independent reference.
- Combined resume bitfield/hash storage is limited to 64 MiB (approximately 1.94 TiB of data at the default 1 MiB block size); sidecar JSON is limited to 128 MiB. Increase `ResumeBlockSize` for larger segmented transfers. This is an explicit resource-safety limit, not an assertion of tested physical downloads at those sizes.
- The default 60-second timeout is a whole-request deadline, including the body. Very large requests on slow links may require a larger `Timeout`/`--timeout`. The sparse-file and arithmetic tests do not establish sustained multi-gigabyte WAN transfer performance.
- A real-network comparison can exercise one selected server/route, not establish performance across the internet. Its source must remain stable across range requests and repetitions; every output is checked against a reference SHA-256. The remote mode disables proxy environment settings for timed tools and bounds the reference fetch, not a server that changes unexpectedly afterward.
- HTTP3, torrents, FTP, browser extensions, and cross-platform runtime behavior are outside these measurements. No new protocol or browser extension is implemented by this audit.
