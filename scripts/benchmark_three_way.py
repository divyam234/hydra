#!/usr/bin/env python3
"""Reproducible Hydra vs aria2c vs Surge HTTP download benchmark.

Runs three local HTTP scenarios: uniform per-GET throttling, unthrottled, and
one deliberately slow range. Records downloader process CPU seconds, peak
RSS, wall time, throughput, request counts, and independent SHA-256 checks.

Python 3.9+, Go, aria2c, Surge and GNU Time are required on Linux. Only Python
stdlib modules are used. See ../BENCHMARKS.md for exact methodology and caveats.
"""

import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import platform
import random
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
TOOLS = ("hydra", "aria2c", "surge")
SCENARIOS = ("throttled", "unthrottled", "skew")
MIB = 1024 * 1024
FIELDS = (
    "scenario", "tool", "connections", "repeat", "wall_s",
    "throughput_mib_s", "user_s", "system_s", "cpu_s",
    "avg_cpu_percent_one_core", "peak_rss_mib", "http_gets",
    "peak_gets", "server_bytes", "server_extra_percent", "sha256_valid",
)
TIME_FORMAT = "user_s=%U\nsystem_s=%S\nmax_rss_kib=%M\n"


class BenchmarkError(RuntimeError):
    """A benchmark prerequisite, process, or integrity check failed."""


def parse_connections(value):
    try:
        result = [int(s) for s in value.split(",")]
    except ValueError as exc:
        raise argparse.ArgumentTypeError("connections must be comma-separated integers") from exc
    if not result or any(n < 1 or n > 64 for n in result) or len(set(result)) != len(result):
        raise argparse.ArgumentTypeError("use distinct connection counts from 1 to 64")
    return result


def executable(value, description):
    path = shutil.which(value)
    if path is None and Path(value).expanduser().is_file():
        path = str(Path(value).expanduser().resolve())
    if path is None:
        raise BenchmarkError(f"{description} executable not found: {value!r}")
    return str(Path(path).resolve())


def gnu_time(value):
    candidates = ([value] if value else []) + [
        shutil.which("gtime"), shutil.which("time"),
        "/run/current-system/sw/bin/time", "/usr/bin/time",
    ]
    for candidate in candidates:
        if not candidate:
            continue
        try:
            check = subprocess.run([candidate, "--version"], text=True,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                   timeout=5, check=False)
        except (OSError, subprocess.TimeoutExpired):
            continue
        if check.returncode == 0 and "GNU Time" in check.stdout:
            return str(Path(candidate).resolve())
    raise BenchmarkError("GNU Time required; install it or set --gnu-time /path/to/time")


def binary_sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for part in iter(lambda: f.read(4 * MIB), b""):
            digest.update(part)
    return digest.hexdigest()


def create_payload(path, size_mib):
    """Write an actual zero-filled 256 MiB fixture and compute its SHA-256."""
    block = bytes(4 * MIB)
    sha = hashlib.sha256()
    with open(path, "wb") as output:
        remaining = size_mib * MIB
        while remaining:
            piece = block[: min(len(block), remaining)]
            output.write(piece)
            sha.update(piece)
            remaining -= len(piece)
    return sha.hexdigest()


def origin_json(url, endpoint):
    with urllib.request.urlopen(url + endpoint, timeout=6) as response:
        if endpoint == "/stats":
            return json.loads(response.read())
        response.read()
        return None


def wait_origin_idle(url, timeout=10):
    """Avoid cross-trial contamination from canceled overlapping GETs."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        stats = origin_json(url, "/stats")
        if stats["active"] == 0:
            return stats
        time.sleep(0.025)
    raise BenchmarkError("HTTP origin still has active requests after download exit")


def command_for(tool, executable_path, connections, directory, url):
    if tool == "hydra":
        return [
            executable_path, "-quiet", "-no-continue", "-if-exists", "overwrite",
            "-d", str(directory), "-o", "test.bin", "-s", str(connections),
            "-x", str(connections), "-k", "1M", url,
        ]
    if tool == "aria2c":
        return [
            executable_path, "--no-conf",
            f"--max-connection-per-server={connections}",
            f"--split={connections}", "--min-split-size=1M",
            "--file-allocation=none", "--auto-file-renaming=false",
            "--allow-overwrite=true", "--continue=false",
            "--summary-interval=0", "--console-log-level=error",
            "--download-result=hide", "--max-tries=1",
            "--enable-color=false", f"--dir={directory}",
            "--out=test.bin", url,
        ]
    if tool == "surge":
        return [
            executable_path, "fetch", url, "--output", str(directory),
            "--filename", "test.bin", "--quiet", "--progress", "none", "--json",
        ]
    raise BenchmarkError(f"unknown downloader: {tool}")


def configure_surge(binary, connections, env):
    for key, value in (
        ("Network.max_connections_per_host", str(connections)),
        ("Network.min_chunk_size", str(MIB)),
    ):
        p = subprocess.run([binary, "config", key, value], env=env,
                           text=True, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, timeout=20)
        if p.returncode:
            raise BenchmarkError(f"Surge config {key} failed: {p.stderr[-800:]}")


def metrics_from_time_file(path):
    values = dict(line.split("=", 1) for line in path.read_text().splitlines()
                  if "=" in line)
    return float(values["user_s"]), float(values["system_s"]), int(values["max_rss_kib"])


def verify_output(path, expected_size, expected_sha):
    if not path.is_file() or path.stat().st_size != expected_size:
        raise BenchmarkError(f"missing or wrong-sized downloaded output: {path}")
    sha = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(4 * MIB), b""):
            sha.update(chunk)
    if sha.hexdigest() != expected_sha:
        raise BenchmarkError(f"independent SHA-256 verification failed: {path}")


def start_origin(executable_path, payload, scenario, args, slow_start):
    rate = 0 if scenario == "unthrottled" else int(args.rate_mib * MIB)
    delay = 0 if scenario == "unthrottled" else args.latency_ms
    cmd = [
        executable_path, "-addr", "127.0.0.1:0", "-file", str(payload),
        "-rate", str(rate), "-latency", f"{delay}ms",
        "-slow-start", str(slow_start if scenario == "skew" else -1),
        "-slow-rate", str(int(args.slow_rate_mib * MIB)),
    ]
    process = subprocess.Popen(cmd, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, text=True, bufsize=1)
    line = process.stdout.readline()
    if not line.startswith("READY "):
        process.terminate()
        error = process.communicate(timeout=5)[1]
        raise BenchmarkError(f"test origin did not start: {line} {error[-800:]}")
    url = "http://" + line.removeprefix("READY ").strip()
    try:
        origin_json(url, "/stats")
    except Exception:
        process.terminate()
        process.wait(timeout=5)
        raise
    return process, url


def stop_origin(process):
    if process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)
    finally:
        process.stdout.close()
        process.stderr.close()


def summarise(rows):
    grouped = {}
    for row in rows:
        key = (row["scenario"], row["connections"], row["tool"])
        grouped.setdefault(key, []).append(row)
    result = {}
    for (scenario, connections, tool), samples in sorted(grouped.items()):
        metrics = (
            "wall_s", "throughput_mib_s", "user_s", "system_s", "cpu_s",
            "avg_cpu_percent_one_core", "peak_rss_mib", "http_gets",
            "peak_gets", "server_extra_percent",
        )
        result[f"{scenario}/{connections}/{tool}"] = {
            "runs": len(samples),
            **{key: round(statistics.mean(row[key] for row in samples), 4)
               for key in metrics},
        }
    return result


def save_results(destination, rows, metadata):
    with (destination / "raw.csv").open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=FIELDS)
        writer.writeheader()
        writer.writerows(rows)
    (destination / "summary.json").write_text(json.dumps(summarise(rows), indent=2) + "\n")
    (destination / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")


def benchmark(args):
    output = Path(args.output_dir).expanduser().resolve()
    output.mkdir(parents=True, exist_ok=True)
    connections = args.connections
    selected = SCENARIOS if "all" in args.scenarios else tuple(args.scenarios)
    size_bytes = args.size_mib * MIB
    # Original 256 MiB test uses byte offset 240 MiB.
    slow_start = (args.slow_start_mib if args.slow_start_mib is not None
                  else max(0, args.size_mib - max(1, args.size_mib // 16))) * MIB
    if not 0 <= slow_start < size_bytes:
        raise BenchmarkError("slow-start offset must be within the source file")
    if args.skew_connections > 64:
        raise BenchmarkError("skew-connections must be at most 64")

    timer = gnu_time(args.gnu_time)
    aria = executable(args.aria2c, "aria2c")
    surge = executable(args.surge, "Surge")
    go = executable("go", "Go")
    env = os.environ.copy()
    for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"):
        env[key] = ""
    env["NO_PROXY"] = env["no_proxy"] = "127.0.0.1,localhost"
    env["GOMAXPROCS"] = str(args.gomaxprocs)

    rows = []
    with tempfile.TemporaryDirectory(prefix="three-way-bench-", dir=output) as scratch:
        scratch = Path(scratch)
        env["XDG_CONFIG_HOME"] = str(scratch / "cfg")
        env["XDG_CACHE_HOME"] = str(scratch / "cache")
        env["XDG_STATE_HOME"] = str(scratch / "state")
        for key in ("XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"):
            Path(env[key]).mkdir(parents=True, exist_ok=True)

        server = scratch / "benchmark-server"
        build = subprocess.run([go, "build", "-o", str(server),
                                str(ROOT / "scripts" / "benchmark_server.go")],
                               cwd=ROOT, env=env, text=True,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               timeout=180)
        if build.returncode:
            raise BenchmarkError(f"could not build test origin: {build.stderr[-1600:]}")
        if args.hydra:
            hydra = executable(args.hydra, "Hydra")
        else:
            hydra = str(scratch / "hydra")
            build_env = dict(env, CGO_ENABLED="0")
            build = subprocess.run([go, "build", "-o", hydra, "./cmd/hydra"],
                                   cwd=ROOT, env=build_env, text=True,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   timeout=180)
            if build.returncode:
                raise BenchmarkError(f"could not build Hydra: {build.stderr[-1600:]}")
        binaries = {"hydra": hydra, "aria2c": aria, "surge": surge}

        payload = scratch / "payload.bin"
        expected_sha = create_payload(payload, args.size_mib)
        metadata = {
            "timestamp_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "platform": platform.platform(),
            "machine": platform.machine(),
            "python_version": sys.version.split()[0],
            "go_version": subprocess.run([go, "version"], text=True,
                                         stdout=subprocess.PIPE, check=True).stdout.strip(),
            "gomaxprocs": args.gomaxprocs,
            "size_bytes": size_bytes,
            "payload_sha256": expected_sha,
            "repeats": args.repeats,
            "connections": connections,
            "skew_connections": args.skew_connections,
            "scenarios": selected,
            "per_get_rate_mib_s": args.rate_mib,
            "slow_get_rate_mib_s": args.slow_rate_mib,
            "slow_start_bytes": slow_start,
            "request_latency_ms": args.latency_ms,
            "binary_sha256": {name: binary_sha256(path) for name, path in binaries.items()},
            "binary_paths": binaries,
            "notes": "Loopback only. GNU Time measures process CPU/RSS. Source server, filesystem page cache, and post-run SHA-256 are excluded from downloader metrics.",
        }
        save_results(output, rows, metadata)

        for scenario_index, scenario in enumerate(selected):
            origin, base_url = start_origin(server, payload, scenario, args, slow_start)
            url = base_url + "/test.bin"
            levels = [args.skew_connections] if scenario == "skew" else connections
            print(f"SCENARIO {scenario}: {args.repeats} repeats x {levels} connections x three tools", flush=True)
            try:
                for repeat in range(1, args.repeats + 1):
                    order = [(tool, level) for level in levels for tool in TOOLS]
                    random.Random(20261119 + repeat).shuffle(order)
                    for tool, level in order:
                        label = f"{scenario}-{repeat:02d}-{level:02d}-{tool}"
                        directory = scratch / "work" / label
                        directory.mkdir(parents=True, exist_ok=True)
                        if tool == "surge":
                            configure_surge(surge, level, env)
                        wait_origin_idle(base_url)
                        origin_json(base_url, "/reset")
                        time_file = output / (label + ".time")
                        command = command_for(tool, binaries[tool], level, directory, url)
                        start = time.perf_counter()
                        p = subprocess.run(
                            [timer, "-f", TIME_FORMAT, "-o", str(time_file),
                             "--", *command],
                            cwd=ROOT, env=env, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=args.timeout_seconds,
                        )
                        wall = time.perf_counter() - start
                        if p.returncode:
                            raise BenchmarkError(f"{label} exited {p.returncode}: "
                                                 f"stderr={p.stderr[-1400:]} stdout={p.stdout[-400:]}")
                        # The integrity check is intentionally OUTSIDE the timed window.
                        verify_output(directory / "test.bin", size_bytes, expected_sha)
                        user, system, rss = metrics_from_time_file(time_file)
                        stats = wait_origin_idle(base_url)
                        cpu = user + system
                        row = {
                            "scenario": scenario, "tool": tool, "connections": level,
                            "repeat": repeat, "wall_s": round(wall, 4),
                            "throughput_mib_s": round(args.size_mib / wall, 3),
                            "user_s": user, "system_s": system, "cpu_s": round(cpu, 4),
                            "avg_cpu_percent_one_core": round(cpu / wall * 100, 2),
                            "peak_rss_mib": round(rss / 1024, 3),
                            "http_gets": stats["gets"], "peak_gets": stats["peak_gets"],
                            "server_bytes": stats["bytes"],
                            "server_extra_percent": round((stats["bytes"] / size_bytes - 1) * 100, 3),
                            "sha256_valid": True,
                        }
                        rows.append(row)
                        print(json.dumps(row), flush=True)
                        save_results(output, rows, metadata)
                        shutil.rmtree(directory)
            finally:
                stop_origin(origin)

    print(f"VERIFIED {len(rows)}/{len(rows)} downloads; results in {output}", flush=True)
    for label, data in summarise(rows).items():
        print(f"{label}: {data['throughput_mib_s']:.2f} MiB/s, "
              f"{data['cpu_s']:.3f} CPU s, {data['peak_rss_mib']:.2f} MiB RSS", flush=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hydra", help="Hydra binary (default: compile current repository source)")
    parser.add_argument("--aria2c", default="aria2c", help="aria2c executable")
    parser.add_argument("--surge", default="surge", help="Surge CLI executable")
    parser.add_argument("--gnu-time", help="GNU Time executable (otherwise autodetected)")
    parser.add_argument("--scenarios", nargs="+", default=["all"],
                        choices=(*SCENARIOS, "all"),
                        help="default: run all three scenarios")
    parser.add_argument("--connections", type=parse_connections, default=[4, 8, 16],
                        help="connections for uniform cases, e.g. 4,8,16")
    parser.add_argument("--skew-connections", type=int, default=16,
                        help="connection count for one slow range")
    parser.add_argument("--size-mib", type=int, default=256)
    parser.add_argument("--repeats", type=int, default=5)
    parser.add_argument("--rate-mib", type=float, default=8.0)
    parser.add_argument("--slow-rate-mib", type=float, default=2.0)
    parser.add_argument("--slow-start-mib", type=int,
                        help="slow GET start offset in MiB (default: final 1/16 of file)")
    parser.add_argument("--latency-ms", type=int, default=40)
    parser.add_argument("--gomaxprocs", type=int, default=2)
    parser.add_argument("--timeout-seconds", type=int, default=120)
    parser.add_argument("--output-dir", default="benchmark-results",
                        help="persistent CSV/JSON/time files; scratch payloads removed")
    args = parser.parse_args(argv)
    if (args.size_mib < 1 or args.repeats < 1 or args.skew_connections < 1
            or args.gomaxprocs < 1 or args.timeout_seconds < 1
            or args.rate_mib <= 0 or args.slow_rate_mib <= 0
            or args.latency_ms < 0):
        parser.error("size, repeats, rates, concurrency and timeout must be positive")
    try:
        benchmark(args)
    except (BenchmarkError, OSError, subprocess.TimeoutExpired) as exc:
        parser.exit(1, f"ERROR: {exc}\n")


if __name__ == "__main__":
    main()
