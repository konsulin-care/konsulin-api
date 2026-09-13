#!/usr/bin/env python3
"""Tests for zap2sarif.py CI path hardening.

SonarCloud flags `open()` calls that accept unvalidated CLI-argument paths
("can escape file system restrictions"). These tests pin the new contract:
paths must resolve inside the workspace and carry an allowed extension.
"""

import json
import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import zap2sarif  # noqa: E402


class ChdirTestCase(unittest.TestCase):
    def setUp(self):
        self._old_cwd = os.getcwd()

    def tearDown(self):
        os.chdir(self._old_cwd)


class ResolveCiPathTests(ChdirTestCase):
    def setUp(self):
        super().setUp()
        self._tmp = tempfile.TemporaryDirectory()
        os.chdir(self._tmp.name)

    def tearDown(self):
        super().tearDown()
        self._tmp.cleanup()

    def test_accepts_relative_paths_within_workspace(self):
        p = zap2sarif._resolve_ci_path("zap/report.json", {".json"})
        self.assertEqual(p, (Path.cwd() / "zap/report.json").resolve())
        self.assertTrue(p.is_relative_to(Path.cwd().resolve()))

    def test_rejects_paths_outside_workspace(self):
        with self.assertRaises(SystemExit):
            zap2sarif._resolve_ci_path("../outside.json", {".json"})

    def test_rejects_absolute_paths_outside_workspace(self):
        with self.assertRaises(SystemExit):
            zap2sarif._resolve_ci_path("/etc/passwd.json", {".json"})

    def test_rejects_unexpected_suffix(self):
        with self.assertRaises(SystemExit):
            zap2sarif._resolve_ci_path("zap/report.yaml", {".json"})


class ConvertTests(ChdirTestCase):
    def test_convert_writes_valid_sarif(self):
        with tempfile.TemporaryDirectory() as d:
            zap = {
                "@version": "X",
                "site": [
                    {
                        "alerts": [
                            {
                                "pluginid": "1",
                                "alert": "title",
                                "desc": "desc",
                                "riskcode": "3",
                                "instances": [{"uri": "http://example.test/"}],
                            }
                        ]
                    }
                ],
            }
            out = Path(d) / "zap-report.sarif"
            zap2sarif.convert(zap, out)
            sarif = json.loads(out.read_text(encoding="utf-8"))
            self.assertEqual(sarif["version"], "2.1.0")
            self.assertEqual(sarif["runs"][0]["results"][0]["level"], "error")

    def test_main_roundtrip_with_relative_paths(self):
        with tempfile.TemporaryDirectory() as d:
            os.chdir(d)
            inp = "report.json"
            outp = "report.sarif"
            Path(inp).write_text(json.dumps({"site": []}), encoding="utf-8")
            zap2sarif.main(["zap2sarif.py", inp, outp])
            self.assertTrue(Path(outp).exists())
            sarif = json.loads(Path(outp).read_text(encoding="utf-8"))
            self.assertEqual(sarif["runs"][0]["results"], [])

    def test_main_rejects_escaping_output_path(self):
        with tempfile.TemporaryDirectory() as d:
            os.chdir(d)
            Path("report.json").write_text("{}", encoding="utf-8")
            with self.assertRaises(SystemExit):
                zap2sarif.main(["zap2sarif.py", "report.json", "../escape.sarif"])

    def test_main_requires_two_args(self):
        with self.assertRaises(SystemExit):
            zap2sarif.main(["zap2sarif.py", "only-one"])

    def test_helpuri_omitted_when_reference_missing(self):
        with tempfile.TemporaryDirectory() as d:
            zap = {
                "@version": "X",
                "site": [
                    {
                        "alerts": [
                            {
                                "pluginid": "1",
                                "alert": "title",
                                "desc": "desc",
                                "riskcode": "2",
                                "instances": [
                                    {"uri": "http://example.test/"}
                                ],
                            }
                        ]
                    }
                ],
            }
            out = Path(d) / "zap-report.sarif"
            zap2sarif.convert(zap, out)
            raw = out.read_text(encoding="utf-8")
            sarif = json.loads(raw)
            rule = sarif["runs"][0]["tool"]["driver"]["rules"][0]
            self.assertNotIn("helpUri", rule)
            self.assertNotIn('"helpUri": null', raw)

    def test_helpuri_is_first_reference_token_when_present(self):
        with tempfile.TemporaryDirectory() as d:
            zap = {
                "@version": "X",
                "site": [
                    {
                        "alerts": [
                            {
                                "pluginid": "2",
                                "alert": "title",
                                "desc": "desc",
                                "reference": (
                                    "https://first.example.test/ref "
                                    "https://second.example.test/"
                                ),
                                "riskcode": "2",
                                "instances": [
                                    {"uri": "http://example.test/"}
                                ],
                            }
                        ]
                    }
                ],
            }
            out = Path(d) / "zap-report.sarif"
            zap2sarif.convert(zap, out)
            sarif = json.loads(out.read_text(encoding="utf-8"))
            rule = sarif["runs"][0]["tool"]["driver"]["rules"][0]
            self.assertEqual(rule["helpUri"], "https://first.example.test/ref")


if __name__ == "__main__":
    unittest.main()
