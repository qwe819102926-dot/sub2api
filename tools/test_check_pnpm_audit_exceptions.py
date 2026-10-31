import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class AuditReportValidationTests(unittest.TestCase):
    def check_report(self, report):
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            audit = folder / "audit.json"
            exceptions = folder / "exceptions.yml"
            audit.write_text(json.dumps(report), encoding="utf-8")
            exceptions.write_text("version: 1\nexceptions:\n", encoding="utf-8")
            return subprocess.run(
                [sys.executable, str(Path(__file__).with_name("check_pnpm_audit_exceptions.py")),
                 "--audit", str(audit), "--exceptions", str(exceptions)],
                capture_output=True, text=True,
            )

    def test_rejects_registry_errors_and_missing_reports(self):
        for report in ({"error": {"code": "ERR_INVALID_PROTOCOL"}}, {}, [],
                       {"advisories": {}, "error": {"code": "ECONNRESET"}}):
            with self.subTest(report=report):
                result = self.check_report(report)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Audit failed", result.stderr)

    def test_accepts_clean_supported_reports(self):
        for report in ({"advisories": {}}, {"vulnerabilities": {}}):
            with self.subTest(report=report):
                self.assertEqual(self.check_report(report).returncode, 0)

    def test_unexcepted_high_vulnerability_still_fails(self):
        result = self.check_report({"advisories": {"1": {
            "module_name": "example", "severity": "high", "github_advisory_id": "GHSA-example"
        }}})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing exceptions", result.stderr)


if __name__ == "__main__":
    unittest.main()
