import os
import hashlib
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import benchmark


class DownloadTimingTests(unittest.TestCase):
    def test_binary_identity_does_not_require_go(self):
        with tempfile.NamedTemporaryFile(delete=False) as binary:
            binary.write(b"benchmark binary")
            path = binary.name
        try:
            with mock.patch("benchmark.shutil.which", return_value=None):
                info = benchmark.executable_info(path, inspect_go=True)
            self.assertEqual(info["sha256"], hashlib.sha256(b"benchmark binary").hexdigest())
            self.assertNotIn("go_build_info", info)
        finally:
            os.unlink(path)

    @mock.patch("benchmark.threading.Timer")
    @mock.patch("benchmark.subprocess.Popen")
    def test_completion_wait_has_no_polling_timeout(self, popen, timer):
        process = popen.return_value.__enter__.return_value
        process.wait.return_value = 0
        benchmark.run_download(["example"], {})
        process.wait.assert_called_once_with()
        timer.assert_called_once_with(180, process.kill)
        timer.return_value.start.assert_called_once_with()
        timer.return_value.cancel.assert_called_once_with()
        timer.return_value.join.assert_called_once_with()

    def test_successful_process(self):
        benchmark.run_download([sys.executable, "-c", "pass"], dict(os.environ))

    def test_failed_process_is_not_a_benchmark_result(self):
        with self.assertRaises(subprocess.CalledProcessError) as failure:
            benchmark.run_download([sys.executable, "-c", "raise SystemExit(7)"], dict(os.environ))
        self.assertEqual(failure.exception.returncode, 7)

    def test_watchdog_kills_a_stalled_process(self):
        with self.assertRaises(subprocess.CalledProcessError) as failure:
            benchmark.run_download([sys.executable, "-c", "import time; time.sleep(30)"], dict(os.environ), timeout=0.1)
        self.assertNotEqual(failure.exception.returncode, 0)


if __name__ == "__main__":
    unittest.main()
