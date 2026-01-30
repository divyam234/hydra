#!/usr/bin/env python3
import os
import sys
import time
import shutil
import subprocess
import argparse
import statistics

# Default high-speed test files (User can override these)
DEFAULT_URL_1GB = "https://speedtest.wtnet.de/files/1000mb.bin"
DEFAULT_URL_5GB = "https://speedtest.wtnet.de/files/5000mb.bin"

COLORS = {
    "HEADER": "\033[95m",
    "BLUE": "\033[94m",
    "GREEN": "\033[92m",
    "RED": "\033[91m",
    "RESET": "\033[0m",
    "BOLD": "\033[1m",
}


def log(msg, color="RESET"):
    if sys.stdout.isatty():
        print(f"{COLORS.get(color, COLORS['RESET'])}{msg}{COLORS['RESET']}")
    else:
        print(msg)


def check_tools():
    tools = {}
    for tool in ["hydra", "aria2c", "curl", "wget"]:
        path = shutil.which(tool)
        if not path and tool == "hydra":
            # Check local build directory
            if os.path.exists("./build/hydra"):
                path = os.path.abspath("./build/hydra")
            elif os.path.exists("./hydra"):
                path = os.path.abspath("./hydra")

        if path:
            tools[tool] = path
        else:
            log(f"Warning: {tool} not found.", "RED")
    return tools


def get_file_size_mb(url):
    # Try to get size via curl head request
    try:
        result = subprocess.run(
            ["curl", "-sI", url], capture_output=True, text=True, timeout=5
        )
        for line in result.stdout.splitlines():
            if "content-length:" in line.lower():
                bytes_val = int(line.split(":")[1].strip())
                return bytes_val / (1024 * 1024)
    except:
        pass
    return 0  # Unknown


def cleanup(filename):
    for f in [filename, f"{filename}.aria2", f"{filename}.hydra"]:
        if os.path.exists(f):
            try:
                os.remove(f)
            except OSError:
                pass


def run_benchmark(tool_name, tool_path, url, output_file, iterations):
    cmd = []
    if tool_name == "hydra":
        cmd = [
            tool_path,
            "download",
            url,
            "-o",
            output_file,
            "--dir",
            ".",
            "-s",
            "16",
            "--max-tries",
            "3",
        ]
    elif tool_name == "aria2c":
        cmd = [
            tool_path,
            url,
            "-o",
            output_file,
            "-d",
            ".",
            "-x",
            "16",
            "-s",
            "16",
            "--file-allocation=none",
            "--allow-overwrite=true",
            "-q",
        ]
    elif tool_name == "curl":
        cmd = [tool_path, "-L", "-o", output_file, "-s", url]
    elif tool_name == "wget":
        cmd = [tool_path, "-O", output_file, "-q", url]

    times = []
    log(f"  Running {tool_name}...", "BLUE")

    for i in range(iterations):
        cleanup(output_file)
        start = time.time()
        try:
            subprocess.run(
                cmd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE
            )
            duration = time.time() - start
            times.append(duration)
            # log(f"    Run {i+1}: {duration:.2f}s")
        except subprocess.CalledProcessError as e:
            log(f"    Run {i + 1} FAILED: {e}", "RED")
        finally:
            cleanup(output_file)

    if not times:
        return 0

    return statistics.mean(times)


def main():
    parser = argparse.ArgumentParser(description="Hydra vs The World - Benchmark Suite")
    parser.add_argument(
        "--url-1gb", default=DEFAULT_URL_1GB, help="URL for ~1GB test file"
    )
    parser.add_argument(
        "--url-5gb", default=DEFAULT_URL_5GB, help="URL for ~5GB test file"
    )
    parser.add_argument(
        "--iterations", type=int, default=3, help="Number of runs per tool"
    )
    parser.add_argument("--skip-5gb", action="store_true", help="Skip the 5GB test")
    args = parser.parse_args()

    tools = check_tools()
    if not tools:
        log("No tools found! Please install hydra, aria2c, curl, or wget.", "RED")
        sys.exit(1)

    benchmarks = [
        {"name": "1GB File", "url": args.url_1gb},
    ]
    if not args.skip_5gb:
        benchmarks.append({"name": "5GB File", "url": args.url_5gb})

    log(f"Starting Benchmark (Iterations: {args.iterations})", "HEADER")
    log(f"Tools detected: {', '.join(tools.keys())}\n", "GREEN")

    for bench in benchmarks:
        log(f"Benchmarking: {bench['name']}", "HEADER")
        log(f"URL: {bench['url']}", "BLUE")

        # approximate size for speed calc
        size_mb = get_file_size_mb(bench["url"])
        if size_mb == 0:
            if "1GB" in bench["name"]:
                size_mb = 1024
            elif "5GB" in bench["name"]:
                size_mb = 5120
            elif "100MB" in bench["name"]:
                size_mb = 100

        log(f"Target Size: ~{size_mb:.2f} MB\n")

        results = []

        for name, path in tools.items():
            avg_time = run_benchmark(
                name, path, bench["url"], "test_dl.dat", args.iterations
            )
            if avg_time > 0:
                speed = size_mb / avg_time
                results.append((name, avg_time, speed))
            else:
                results.append((name, float("inf"), 0))

        # Sort by speed (descending)
        results.sort(key=lambda x: x[2], reverse=True)

        # Print Table
        print("\n" + "=" * 65)
        print(f"{'Tool':<15} | {'Avg Time':<12} | {'Speed':<15} | {'Rel Speed':<10}")
        print("-" * 65)

        fastest_speed = results[0][2]

        for name, t, speed in results:
            rel = f"{speed / fastest_speed * 100:.0f}%" if fastest_speed > 0 else "N/A"
            print(f"{name:<15} | {t:<10.2f} s | {speed:<10.2f} MB/s | {rel:<10}")
        print("=" * 65 + "\n")

    cleanup("test_dl.dat")


if __name__ == "__main__":
    main()
