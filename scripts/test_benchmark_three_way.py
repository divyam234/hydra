"""Fast, dependency-free unit tests for the public three-way benchmark harness."""

import argparse
import hashlib
from pathlib import Path
import tempfile
import unittest

import benchmark_three_way as bench


class ConfigurationTests(unittest.TestCase):
    def test_connections_are_validated(self):
        self.assertEqual(bench.parse_connections("4,8,16"), [4, 8, 16])
        for invalid in ("", "0", "1,1", "1,65", "foo", "8,garbage"):
            with self.subTest(invalid=invalid):
                with self.assertRaises(argparse.ArgumentTypeError):
                    bench.parse_connections(invalid)

    def test_all_three_command_lines(self):
        url = "http://127.0.0.1:1234/test.bin"
        dest = Path("benchmark-work")
        hydra = bench.command_for("hydra", "/tmp/hydra", 16, dest, url)
        self.assertIn("-quiet", hydra)
        self.assertEqual(hydra[hydra.index("-s") + 1], "16")
        self.assertEqual(hydra[hydra.index("-x") + 1], "16")
        self.assertEqual(hydra[-1], url)

        aria = bench.command_for("aria2c", "/tmp/aria2c", 8, dest, url)
        self.assertIn("--no-conf", aria)
        self.assertIn("--continue=false", aria)
        self.assertIn("--split=8", aria)
        self.assertIn("--max-connection-per-server=8", aria)

        surge = bench.command_for("surge", "/tmp/surge", 4, dest, url)
        self.assertEqual(surge[1], "fetch")
        self.assertIn("--quiet", surge)
        self.assertIn("--json", surge)

    def test_unsupported_client_fails(self):
        with self.assertRaises(bench.BenchmarkError):
            bench.command_for("nonexistent", "x", 4, Path("."), "http://example.com")

    def test_summary_groups_by_scenario_connections_and_tool(self):
        rows = [
            {
                "scenario": "throttled", "connections": 16, "tool": "hydra",
                "wall_s": 2, "throughput_mib_s": 128, "user_s": 0.1,
                "system_s": 0.2, "cpu_s": 0.3,
                "avg_cpu_percent_one_core": 15, "peak_rss_mib": 12,
                "http_gets": 16, "peak_gets": 16,
                "server_extra_percent": 0,
            },
            {
                "scenario": "throttled", "connections": 16, "tool": "hydra",
                "wall_s": 4, "throughput_mib_s": 64, "user_s": 0.1,
                "system_s": 0.4, "cpu_s": 0.5,
                "avg_cpu_percent_one_core": 12.5, "peak_rss_mib": 14,
                "http_gets": 18, "peak_gets": 16,
                "server_extra_percent": 0,
            },
        ]
        summary = bench.summarise(rows)["throttled/16/hydra"]
        self.assertEqual(summary["runs"], 2)
        self.assertEqual(summary["cpu_s"], 0.4)
        self.assertEqual(summary["peak_rss_mib"], 13)
        self.assertEqual(summary["wall_s"], 3)

    def test_file_digest_verified_outside_timing(self):
        with tempfile.TemporaryDirectory() as d:
            f = Path(d) / "payload.bin"
            sha = bench.create_payload(f, 1)
            expected = hashlib.sha256(bytes(1024 * 1024)).hexdigest()
            self.assertEqual(sha, expected)
            bench.verify_output(f, 1024 * 1024, expected)
            with self.assertRaises(bench.BenchmarkError):
                bench.verify_output(f, 1024 * 1024, "0" * 64)
            with self.assertRaises(bench.BenchmarkError):
                bench.verify_output(f, 2 * 1024 * 1024, expected)

    def test_time_metrics_parsing(self):
        with tempfile.TemporaryDirectory() as d:
            f = Path(d) / "test.time"
            f.write_text("user_s=0.12\nsystem_s=0.33\nmax_rss_kib=20480\n")
            self.assertEqual(bench.metrics_from_time_file(f), (0.12, 0.33, 20480))


if __name__ == "__main__":
    unittest.main()
