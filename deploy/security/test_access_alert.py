import importlib.util
import json
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("access_alert", Path(__file__).with_name("access_alert.py"))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class DetectionTests(unittest.TestCase):
    def row(self, path="/v1/usage", status=200, ts=1000, scan=False, ip="8.8.8.8"):
        return {"ts": ts, "request": {"remote_ip": ip, "uri": path},
                "status": status, "security_rule": "sensitive_path" if scan else ""}

    def test_normal_traffic_and_shared_egress_are_not_scans(self):
        rows = [self.row() for _ in range(200)]
        self.assertEqual(list(guard.detect(rows, 1000)), [])

    def test_requires_five_distinct_recent_blocked_paths(self):
        rows = [self.row("/.env", scan=True) for _ in range(10)]
        self.assertEqual(list(guard.detect(rows, 1000)), [])
        rows += [self.row("/.env." + str(i), scan=True) for i in range(4)]
        alerts = list(guard.detect(rows, 1000))
        self.assertEqual(alerts[0]["count"], 5)
        self.assertNotIn("paths", alerts[0])
        self.assertEqual(list(guard.detect(rows, 1061)), [])

    def test_auth_threshold_and_private_source_exclusion(self):
        rows = [self.row(status=401) for _ in range(99)]
        self.assertEqual(list(guard.detect(rows, 1000)), [])
        rows.append(self.row(status=401))
        self.assertEqual(list(guard.detect(rows, 1000))[0]["rule"], "repeated_401")
        for row in rows:
            row["request"]["remote_ip"] = "127.0.0.1"
        self.assertEqual(list(guard.detect(rows, 1000)), [])

    def test_timer_run_writes_alert_and_deduplicates(self):
        with tempfile.TemporaryDirectory() as folder:
            folder = Path(folder)
            log = folder / "access.jsonl"
            rows = [self.row("/.env." + str(i), ts=time.time(), scan=True) for i in range(5)]
            log.write_text("\n".join(json.dumps(row) for row in rows) + "\n")
            command = [sys.executable, str(Path(guard.__file__)), "--log", str(log), "--state-dir", str(folder / "state")]
            first = subprocess.run(command, capture_output=True, text=True, check=True)
            self.assertEqual(json.loads(first.stdout)["action"], "alert_only")
            second = subprocess.run(command, capture_output=True, text=True, check=True)
            self.assertEqual(second.stdout, "")
            self.assertEqual(len((folder / "state" / "alerts.jsonl").read_text().splitlines()), 1)


if __name__ == "__main__":
    unittest.main()
