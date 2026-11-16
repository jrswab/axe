"""Behavioral tests for the shared CI command dispatcher."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "ci-check"


class CICheckTest(unittest.TestCase):
    def run_check(self, argument, *, unformatted="", fail_go=""):
        with tempfile.TemporaryDirectory(prefix="axe-ci-check-") as directory:
            root = Path(directory)
            tools = root / "bin"
            tools.mkdir()
            log = root / "go.log"
            commands = {
                "go": '#!/bin/bash\nprintf "%s\\n" "$*" >> "$CI_TEST_LOG"\n'
                      '[[ "$*" != "$CI_TEST_FAIL_GO" ]]\n',
                "git": '#!/bin/bash\nprintf "file.go\\0"\n',
                "gofmt": '#!/bin/bash\nprintf "%s" "$CI_TEST_UNFORMATTED"\n',
            }
            for name, text in commands.items():
                command = tools / name
                command.write_text(text)
                command.chmod(0o755)
            env = dict(os.environ, PATH=f"{tools}:{os.defpath}", CI_TEST_LOG=str(log),
                       CI_TEST_UNFORMATTED=unformatted, CI_TEST_FAIL_GO=fail_go)
            result = subprocess.run([str(SCRIPT), argument], env=env, capture_output=True, text=True)
            return result, log.read_text().splitlines() if log.exists() else []

    def test_dispatch(self):
        cases = {
            "test": ["test -race -v ./..."],
            "build": ["build -v ./..."],
            "vet": ["vet ./..."],
            "integrity": ["mod verify", "mod tidy -diff"],
        }
        for argument, expected in cases.items():
            with self.subTest(argument=argument):
                result, calls = self.run_check(argument)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, expected)

    def test_integrity_reports_format_without_skipping_module_checks(self):
        result, calls = self.run_check("integrity", unformatted="file.go\n")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("file.go", result.stderr)
        self.assertEqual(calls, ["mod verify", "mod tidy -diff"])

    def test_integrity_fails_when_module_checks_fail(self):
        for failed in ("mod verify", "mod tidy -diff"):
            with self.subTest(failed=failed):
                result, calls = self.run_check("integrity", fail_go=failed)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, ["mod verify", "mod tidy -diff"])

    def test_unknown_command_fails(self):
        result, calls = self.run_check("unknown")
        self.assertEqual(result.returncode, 2)
        self.assertIn("Usage:", result.stderr)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
