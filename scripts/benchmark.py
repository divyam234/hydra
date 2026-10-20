#!/usr/bin/env python3
"""Repeatable, checksum-verified HTTP/disk comparison; Python stdlib only."""
import argparse
import hashlib
import http.server
import json
import os
import pathlib
import platform
import shutil
import statistics
import subprocess
import tempfile
import threading
import time
import urllib.request


def executable_info(path, inspect_go=False):
    digest = hashlib.sha256()
    with open(path, "rb") as binary:
        for chunk in iter(lambda: binary.read(1 << 20), b""):
            digest.update(chunk)
    info = {"path": path, "sha256": digest.hexdigest()}
    go = shutil.which("go") if inspect_go else None
    if go:
        try:
            info["go_build_info"] = subprocess.check_output([go, "version", "-m", path], text=True, stderr=subprocess.STDOUT)
        except subprocess.CalledProcessError:
            info["go_build_info"] = "unavailable (not a Go executable)"
    return info


def run_download(command, env, timeout=180):
    # Popen.wait(timeout=...) polls on POSIX, quantizing short benchmarks by
    # tens of milliseconds. Wait without a timeout and use a separate watchdog.
    with subprocess.Popen(command, env=env, stdout=subprocess.DEVNULL) as process:
        watchdog = threading.Timer(timeout, process.kill)
        watchdog.daemon = True
        watchdog.start()
        try:
            returncode = process.wait()
        finally:
            watchdog.cancel()
            watchdog.join()
        if returncode:
            raise subprocess.CalledProcessError(returncode, command)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hydra", required=True, help="Hydra executable")
    parser.add_argument("--baseline", help="Optional pre-change Hydra executable")
    parser.add_argument("--directory", default="/tmp/opencode", help="Filesystem to measure")
    parser.add_argument("--size-mib", type=int, default=128)
    parser.add_argument("--repeats", type=int, default=3)
    parser.add_argument("--modes", nargs="+", choices=["loopback", "throttled"], default=["loopback", "throttled"])
    parser.add_argument("--output", help="Write raw JSON results")
    parser.add_argument("--url", help="Optional stable HTTP(S) file for a real-network comparison; --size-mib bounds reference download size")
    args = parser.parse_args()
    if args.size_mib < 1 or args.repeats < 1:
        parser.error("size and repeats must be positive")
    hydra = str(pathlib.Path(args.hydra).resolve())
    aria = shutil.which("aria2c")
    if not aria:
        parser.error("aria2c must be installed")
    executables = {"hydra": executable_info(hydra, True), "aria2c": executable_info(aria)}
    if args.baseline:
        executables["baseline"] = executable_info(str(pathlib.Path(args.baseline).resolve()), True)
    if args.url:
        if not args.url.startswith(("https://", "http://")):
            parser.error("remote URL must use HTTP(S)")
        limit = args.size_mib << 20
        with urllib.request.urlopen(args.url, timeout=30) as response:
            payload = response.read(limit + 1)
        if not payload or len(payload) > limit:
            parser.error("remote reference is empty or exceeds --size-mib limit")
        args.size_mib = len(payload) / (1 << 20)
    else:
        # Nonzero deterministic content detects misplaced ranges and sparse holes.
        payload = hashlib.shake_256(b"hydra-benchmark-v1").digest(args.size_mib << 20)
    expected = hashlib.sha256(payload).hexdigest()
    rate = 0

    class Handler(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, *unused):
            pass

        def do_HEAD(self):
            self.respond(False)

        def do_GET(self):
            self.respond(True)

        def respond(self, body):
            start, end = 0, len(payload) - 1
            value = self.headers.get("Range")
            if value:
                bounds = value.removeprefix("bytes=").split("-", 1)
                start = int(bounds[0])
                if bounds[1]:
                    end = min(int(bounds[1]), end)
            self.send_response(206 if value else 200)
            self.send_header("Content-Length", str(end - start + 1))
            self.send_header("Accept-Ranges", "bytes")
            if value:
                self.send_header("Content-Range", f"bytes {start}-{end}/{len(payload)}")
            self.end_headers()
            if not body:
                return
            try:
                sent = 0
                began = time.monotonic()
                for offset in range(start, end + 1, 256 << 10):
                    chunk = memoryview(payload)[offset:min(offset + (256 << 10), end + 1)]
                    self.wfile.write(chunk)
                    sent += len(chunk)
                    if rate:
                        time.sleep(max(0, sent / rate - (time.monotonic() - began)))
            except (BrokenPipeError, ConnectionResetError):
                pass

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    result = {
        "platform": platform.platform(), "filesystem_path": str(pathlib.Path(args.directory).resolve()),
        "size_mib": args.size_mib, "repeats": args.repeats,
        "aria2_version": subprocess.check_output([aria, "--version"], text=True).splitlines()[0],
        "notes": "Loopback, warm memory source, HTTP/1.1; end-to-end includes file fsync; hashes checked outside timing. Disk reads are cached, not physical device bandwidth.",
        "runs": [], "disk": [],
        "executables": executables,
        "source_url": args.url, "source_sha256": expected,
    }
    try:
        with tempfile.TemporaryDirectory(prefix="hydra-bench-", dir=args.directory) as directory:
            output = pathlib.Path(directory) / "download.bin"
            # Filesystem baseline: buffered write + fsync, and explicitly cached read.
            for _ in range(0 if args.url else args.repeats):
                began = time.monotonic()
                with output.open("wb") as f:
                    f.write(payload)
                    f.flush()
                    os.fsync(f.fileno())
                write = time.monotonic() - began
                began = time.monotonic()
                with output.open("rb") as f:
                    while f.read(1 << 20):
                        pass
                read = time.monotonic() - began
                result["disk"].append({"write_fsync_mib_s": args.size_mib / write, "cached_read_mib_s": args.size_mib / read})
            tools = [("hydra", hydra), ("aria2c", aria)]
            if args.baseline:
                tools.insert(0, ("baseline", str(pathlib.Path(args.baseline).resolve())))
            url = args.url or f"http://127.0.0.1:{server.server_port}/file.bin"
            env = dict(os.environ)
            for key in list(env):
                if key.lower() in ("http_proxy", "https_proxy", "all_proxy"):
                    del env[key]
            env["NO_PROXY"] = "127.0.0.1,localhost"
            modes = [("remote", 0)] if args.url else [("loopback", 0), ("8MiB_per_connection", 8 << 20)]
            for mode, rate in modes:
                if not args.url and ("loopback" if rate == 0 else "throttled") not in args.modes:
                    continue
                for connections in [1, 8]:
                    for repeat in range(args.repeats):
                        # Rotate ordering to reduce systematic warmup/order bias.
                        ordered = tools[repeat % len(tools):] + tools[:repeat % len(tools)]
                        for name, executable in ordered:
                            for path in pathlib.Path(directory).glob("download.bin*"):
                                path.unlink()
                            if name == "aria2c":
                                command = [executable, "--no-conf", "--quiet=true", "--file-allocation=none", "--auto-file-renaming=false", "-s", str(connections), "-x", str(connections), "-k", "1M", "-d", directory, "-o", output.name, url]
                            else:
                                command = [executable, "--quiet", "--no-env-proxy", "-s", str(connections), "-x", str(connections), "-k", "1M", "-d", directory, "-o", output.name, url]
                            began = time.monotonic()
                            run_download(command, env)
                            with output.open("rb") as f:
                                os.fsync(f.fileno())
                            elapsed = time.monotonic() - began
                            digest = hashlib.sha256()
                            with output.open("rb") as f:
                                for chunk in iter(lambda: f.read(1 << 20), b""):
                                    digest.update(chunk)
                            if digest.hexdigest() != expected:
                                raise RuntimeError(f"{name}: checksum mismatch")
                            result["runs"].append({"tool": name, "mode": mode, "connections": connections, "seconds": elapsed, "mib_s": args.size_mib / elapsed, "sha256_ok": True, "command": command})
                    for name, _ in tools:
                        samples = [r["mib_s"] for r in result["runs"] if r["tool"] == name and r["mode"] == mode and r["connections"] == connections]
                        print(f"{mode:24} connections={connections} {name:10} median={statistics.median(samples):.1f} MiB/s", flush=True)
    finally:
        server.shutdown()
        server.server_close()
    if args.url:
        result["notes"] = "Real-network HTTP(S), stable reference fetched before timing; process startup and file fsync included; SHA-256 checked after timing. Server load, routing, TLS, protocol negotiation and caching affect results. Proxy environment disabled for timed tools."
    if result["disk"]:
        for key in result["disk"][0]:
            print(f"{key}: {statistics.median(row[key] for row in result['disk']):.1f}")
    if args.output:
        pathlib.Path(args.output).write_text(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
