#!/usr/bin/env python3
"""Tests for verify_go_coverage.py, run in-process against in-memory streams."""

import contextlib
import io
import runpy
import sys
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import verify_go_coverage  # noqa: E402
from verify_go_coverage import changed_lines, main, parse_profile, split_input, summarize  # noqa: E402

PROFILE = "mode: atomic\nmod/pkg/a.go:1.1,2.2 2 1\nmod/pkg/a.go:4.1,6.2 3 0\nmod/other/b.go:1.1,2.2 1 7\n"
DIFF = """diff --git a/pkg/a.go b/pkg/a.go
--- a/pkg/a.go
+++ b/pkg/a.go
@@ -5 +5,2 @@ func a() {
-	old()
+	changed()
+	added()
"""


def run(argv, text):
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stderr(err):
        code = main(argv, io.StringIO(text), out)
    return code, out.getvalue()


class ProfileTests(unittest.TestCase):
    def test_merges_blocks_and_counts_function_literals(self):
        profile = "mode: atomic\nmodule/pkg/a.go:1.1,2.2 2 0\nmodule/pkg/a.go:1.1,2.2 2 1\nmodule/pkg/a.go:4.1,5.2 3 0\nmodule/other/b.go:1.1,2.2 1 7\nmodule/empty/c.go:1.1,1.2 0 0\n"
        packages = summarize(parse_profile(profile.splitlines()))
        self.assertEqual({"module/pkg": (2, 5), "module/other": (1, 1), "module/empty": (0, 0)}, packages)

    def test_invalid_profiles(self):
        for profile in ["", "mode: invalid", "mode: set", "mode: set\npkg/a.go:1.1,2.2 -1 1", "mode: set\npkg/a.go:1.1,2.2 1 -1", "mode: set\nbad 1 1", "mode: set\npkg/a.go 1 1", "mode: set\npkg/a.go:1.1,2.2 x 1", "mode: set\npkg/a.go:1.1,2.2 1 1\npkg/a.go:1.1,2.2 2 0"]:
            lines = profile.splitlines()
            with self.subTest(profile=profile), self.assertRaises(ValueError):
                parse_profile(lines)

    def test_split_input_separates_trailing_diff(self):
        self.assertEqual((PROFILE.splitlines(), []), split_input(PROFILE))
        profile, diff = split_input(PROFILE + DIFF)
        self.assertEqual(PROFILE.splitlines(), profile)
        self.assertEqual(DIFF.splitlines(), diff)


class ChangedLinesTests(unittest.TestCase):
    def test_tracks_added_lines_across_hunks_and_context(self):
        diff = """diff --git a/pkg/a.go b/pkg/a.go
--- a/pkg/a.go
+++ b/pkg/a.go
@@ -1,3 +1,4 @@
 keep
-gone
++++ added line that looks like a header
+second
 keep
\\ No newline at end of file
@@ -10,0 +12 @@
+single
diff --git a/pkg/gone.go b/pkg/gone.go
--- a/pkg/gone.go
+++ /dev/null
@@ -1 +0,0 @@
-removed
diff --git a/pkg/a_test.go b/pkg/a_test.go
--- a/pkg/a_test.go
+++ b/pkg/a_test.go
@@ -1 +1 @@
-old
+new
diff --git a/README.md b/README.md
+++ b/README.md
@@ -0,0 +1 @@
+docs
"""
        self.assertEqual({"pkg/a.go": {2, 3, 12}}, changed_lines(diff.splitlines()))

    def test_truncated_hunk_keeps_lines_seen(self):
        self.assertEqual({"pkg/a.go": {1}}, changed_lines(["+++ b/pkg/a.go", "@@ -1 +1,3 @@", "+one"]))


class MainTests(unittest.TestCase):
    def test_reports_total_and_package_coverage(self):
        code, out = run([], PROFILE + "mod/empty/c.go:1.1,1.2 0 0\n")
        self.assertEqual(0, code)
        self.assertIn("Go statement coverage: 50.00% (3/6)", out)
        self.assertIn("| mod/pkg | 2 / 5 | 40.00% |", out)
        self.assertIn("| mod/empty | 0 / 0 | 100.00% |", out)
        self.assertNotIn("Changed", out)

    def test_threshold_does_not_round_up(self):
        code, out = run(["--minimum", "100"], "mode: set\nmod/pkg/a.go:1.1,2.2 99999 1\nmod/pkg/a.go:3.1,4.2 1 0\n")
        self.assertEqual(1, code)
        self.assertIn("99999/100000", out)
        self.assertIn("below the required 100%", out)
        self.assertEqual(0, run(["--minimum", "100"], "mode: set\nmod/pkg/a.go:1.1,2.2 1 1\n")[0])

    def test_changed_lines_report_uncovered_blocks(self):
        code, out = run(["--module", "mod/", "--changed-minimum", "100"], PROFILE + DIFF)
        self.assertEqual(1, code)
        self.assertIn("Changed Go statement coverage: 0.00% (0/3)", out)
        self.assertIn("| pkg/a.go:4-6 | 3 |", out)
        self.assertIn("Changed-line coverage is below the required 100%.", out)

    def test_changed_lines_pass_when_covered(self):
        code, out = run(["--module", "mod", "--changed-minimum", "100"], PROFILE.replace("4.1,6.2 3 0", "4.1,6.2 3 2") + DIFF)
        self.assertEqual(0, code)
        self.assertIn("Changed Go statement coverage: 100.00% (3/3)", out)
        self.assertNotIn("Uncovered", out)

    def test_changed_files_outside_module_or_blocks_are_ignored(self):
        diff = DIFF.replace("@@ -5 +5,2 @@", "@@ -9 +9,2 @@")
        self.assertIn("no changed statements", run(["--module", "mod", "--changed-minimum", "100"], PROFILE + diff)[1])
        self.assertIn("no changed statements", run(["--module", "other", "--changed-minimum", "100"], PROFILE + DIFF)[1])

    def test_total_failure_survives_changed_success(self):
        code, out = run(["--minimum", "100", "--module", "mod"], PROFILE + DIFF)
        self.assertEqual(1, code)
        self.assertIn("Coverage is below the required 100%.", out)

    def test_usage_errors(self):
        for argv, text in [
            ([], ""),
            ([], DIFF),
            (["--minimum", "101"], PROFILE),
            (["--minimum", "NaN"], PROFILE),
            (["--changed-minimum", "abc"], PROFILE),
        ]:
            with self.subTest(argv=argv, text=text[:12]), self.assertRaises(SystemExit) as raised:
                run(argv, text)
            self.assertEqual(2, raised.exception.code)

    def test_script_entry_point_reads_standard_streams(self):
        out = io.StringIO()
        with mock.patch.object(sys, "argv", ["verify_go_coverage.py"]), mock.patch.object(sys, "stdin", io.StringIO(PROFILE)), contextlib.redirect_stdout(out):
            with self.assertRaises(SystemExit) as raised:
                runpy.run_path(verify_go_coverage.__file__, run_name="__main__")
        self.assertEqual(0, raised.exception.code)
        self.assertIn("Go statement coverage", out.getvalue())


if __name__ == "__main__":
    unittest.main()
