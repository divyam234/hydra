#!/usr/bin/env python3
"""Black-box CLI resume audit. No Hydra internals/imports or third-party packages.

Kills real processes after observing completed checkpoints or growing single-
stream partial files; compares final files with an independent source SHA-256.
Exits 1 on corruption, unexpected rejection, or an unsafe version change.
"""

import argparse
import base64
import hashlib
import http.server
import json
import pathlib
import re
import signal
import subprocess
import tempfile
import threading
import time


def payload(version, size):
    # Non-repeating data, with both version and offset affecting every block.
    return b"".join(hashlib.sha256(f"{version}:{i}".encode()).digest()
                    for i in range((size + 31) // 32))[:size]


class Origin(http.server.ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, data):
        super().__init__(("127.0.0.1", 0), Handler)
        self.data = data
        self.version = 0
        self.validators = True
        self.change_after_head = False


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_):
        pass

    def handle(self):
        try:
            super().handle()
        except (BrokenPipeError, ConnectionResetError):
            pass  # Expected when the audit kills a live download process.

    def do_HEAD(self):
        self.serve(False)

    def do_GET(self):
        self.serve(True)

    def serve(self, body):
        version = self.server.version
        if body and self.server.change_after_head:
            version = 1
        data = self.server.data[version]
        etag = f'"version-{version}"'
        if self.server.validators and self.headers.get("If-Match") not in (None, etag):
            self.send_response(412)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        start, end = 0, len(data) - 1
        ranged = self.headers.get("Range")
        if ranged:
            match = re.fullmatch(r"bytes=(\d+)-(\d*)", ranged)
            if not match:
                self.send_error(416)
                return
            start = int(match[1])
            end = int(match[2]) if match[2] else end
            if not 0 <= start <= end < len(data):
                self.send_error(416)
                return
        self.send_response(206 if ranged else 200)
        self.send_header("Content-Length", str(end - start + 1))
        self.send_header("Accept-Ranges", "bytes")
        if self.server.validators:
            self.send_header("ETag", etag)
        if ranged:
            self.send_header("Content-Range", f"bytes {start}-{end}/{len(data)}")
        self.end_headers()
        if body:
            try:
                for off in range(start, end + 1, 16384):
                    self.wfile.write(data[off:min(off + 16384, end + 1)])
                    self.wfile.flush()
                    time.sleep(0.003)
            except (BrokenPipeError, ConnectionResetError):
                pass


def checkpoint(path):
    try:
        meta = json.loads(path.read_text())
        bits = base64.b64decode(meta.get("bitfield", ""), validate=True)
        completed = [i for i in range(meta["block_count"]) if bits[i // 8] & (1 << (i % 8))]
        return meta, completed
    except (OSError, ValueError, KeyError, IndexError):
        return None, []


def run_case(binary, server, root, name, splits, mode="stable", checksum=False):
    server.version = 0
    server.validators = mode != "changed_without_validator"
    server.change_after_head = False
    directory = root / name
    directory.mkdir()
    target = directory / "download.bin"
    sidecar = directory / "download.bin.hydra"
    part = directory / "download.bin.part"
    records = []
    expected = hashlib.sha256(server.data[0 if mode in ("stable", "local_damage") else 1]).hexdigest()

    def command(split):
        cmd = [binary, "--quiet", "--no-env-proxy", "-s", str(split), "-x", "16",
               "-k", "64K", "--resume-block-size", "64K", "--metadata-flush-interval", "30ms",
               "--retry", "0", "-d", str(directory), "-o", target.name]
        if checksum:
            cmd += ["--checksum", f"sha256:{expected}"]
        return cmd + [f"http://127.0.0.1:{server.server_port}/file"]

    # Observe actual checkpoints/partial bytes, not a fixed readiness sleep.
    for index, split in enumerate(splits[:-1]):
        baseline_meta, baseline_bits = checkpoint(sidecar)
        threshold = len(baseline_bits) + 2
        prefix_threshold = (part.stat().st_size if part.exists() else 0) + (128 << 10)
        with subprocess.Popen(command(split), stdout=subprocess.DEVNULL, stderr=subprocess.PIPE) as process:
            deadline = time.monotonic() + 15
            try:
                while True:
                    meta, bits = checkpoint(sidecar)
                    partial_bytes = part.stat().st_size if part.exists() else 0
                    if (meta and threshold <= len(bits) < meta["block_count"]) or (
                            prefix_threshold <= partial_bytes < len(server.data[0])):
                        break
                    if process.poll() is not None:
                        raise RuntimeError(f"{name}: process ended before checkpoint: {process.stderr.read()!r}")
                    if time.monotonic() >= deadline:
                        raise RuntimeError(f"{name}: no advancing checkpoint")
                    time.sleep(0.005)
                # Exercise both graceful cancellation and unhandled process death.
                hard = index % 2 == 0
                process.kill() if hard else process.send_signal(signal.SIGTERM)
                process.wait(timeout=10)
                records.append({"split": split, "hard_kill": hard, "checkpoint_blocks": len(bits),
                                "partial_bytes": partial_bytes})
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
        # Hydra's exclusive-create lock is stale after SIGKILL. Remove only our
        # test lock, after wait() has established that its owning process died.
        lock = directory / "download.bin.hydra.lock"
        if lock.exists():
            lock.unlink()

    if mode in ("changed_validator", "changed_without_validator"):
        server.version = 1
    elif mode == "local_damage":
        meta, bits = checkpoint(sidecar)
        if bits:
            damaged = target
            off = bits[0] * meta["block_size"] + 123
        elif meta and meta.get("single") and meta.get("prefix_size", 0) > 123:
            damaged = part
            off = 123
        else:
            raise RuntimeError(f"{name}: no completed block available to damage")
        with damaged.open("r+b") as file:
            file.seek(off)
            original = file.read(1)
            file.seek(off)
            file.write(bytes([original[0] ^ 255]))
    elif mode == "change_after_probe":
        server.change_after_head = True

    result = subprocess.run(command(splits[-1]), capture_output=True, timeout=30)
    digest = hashlib.sha256(target.read_bytes()).hexdigest() if target.exists() else None
    status = "hash_match" if result.returncode == 0 and digest == expected else (
        "silent_corruption" if result.returncode == 0 else "rejected")
    if mode == "stable" and status != "hash_match":
        raise RuntimeError(f"{name}: healthy resume failed: {result.stderr!r}")
    return {"case": name, "status": status, "exit_code": result.returncode,
            "expected_status": "rejected" if mode == "change_after_probe" else "hash_match",
            "sha256": digest, "expected_sha256": expected, "interruptions": records,
            "stderr": result.stderr.decode(errors="replace"), "checksum_supplied": checksum}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hydra", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--size-mib", type=int, default=16)
    args = parser.parse_args()
    if args.size_mib < 8:
        parser.error("--size-mib must be at least 8 for interrupted checkpoints")
    binary = str(pathlib.Path(args.hydra).resolve())
    server = Origin([payload(version, args.size_mib << 20) for version in (0, 1)])
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    results = []
    try:
        with tempfile.TemporaryDirectory(prefix="hydra-resume-audit-", dir=args.output.parent) as directory:
            cases = [("interrupt_2_workers", [2, 2, 2, 2], "stable", False),
                     ("interrupt_8_workers", [8, 8, 8, 8], "stable", False),
                     ("change_worker_count", [2, 8, 16, 4], "stable", False),
                     ("switch_to_single", [8, 8, 1], "stable", False),
                     ("single_stream_interruptions", [1, 1, 1, 1], "stable", False),
                     ("single_stream_damaged_prefix", [1, 1, 1], "local_damage", False),
                     ("changed_etag", [8, 8], "changed_validator", False),
                     ("changed_after_probe", [8], "change_after_probe", False),
                     ("damaged_local_block", [8, 8], "local_damage", False),
                     ("damaged_local_with_checksum", [8, 8], "local_damage", True),
                     ("changed_without_validator", [8, 8], "changed_without_validator", False),
                     ("changed_without_validator_with_checksum", [8, 8], "changed_without_validator", True)]
            for name, splits, mode, checksum in cases:
                result = run_case(binary, server, pathlib.Path(directory), name, splits, mode, checksum)
                results.append(result)
                print(f"{name}: {result['status']}", flush=True)
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
        args.output.write_text(json.dumps({"binary": binary,
            "binary_sha256": hashlib.sha256(pathlib.Path(binary).read_bytes()).hexdigest(),
            "size_bytes": args.size_mib << 20, "results": results}, indent=2) + "\n")
    return 1 if any(r["status"] != r["expected_status"] for r in results) else 0


if __name__ == "__main__":
    raise SystemExit(main())
