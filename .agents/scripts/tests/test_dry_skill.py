"""Behavioral contract for the workflow's dry-skill smoke step.

Keep the named step as the entry point, whether its shell is inline or invokes
an owned helper under .github/ or .agents/. No real Axe binary is executed.
"""

import os
from pathlib import Path
import re
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[3]


def dry_skill_shell():
    """Extract this workflow's named step and literal run block, without YAML deps."""
    workflow = (ROOT / ".github/workflows/smoke-test.yml").read_text()
    step = re.search(r"(?m)^( +)- name: dry-skill\s*$", workflow)
    if step is None:
        raise ValueError("smoke workflow must retain the named dry-skill step")
    indent = len(step.group(1))
    lines = workflow[step.end():].splitlines()
    block = []
    in_run = False
    for line in lines:
        if line.strip() and len(line) - len(line.lstrip()) <= indent:
            break
        if not in_run:
            if line.strip() == "run: |":
                in_run = True
            continue
        if line.strip() and len(line) - len(line.lstrip()) <= indent + 2:
            break
        block.append(line)
    shell = textwrap.dedent("\n".join(block)).strip()
    if not shell:
        raise ValueError("dry-skill must have a literal run: | block")
    return shell


def run_smoke_step(shell, output):
    with tempfile.TemporaryDirectory(prefix="axe-dry-skill-") as directory:
        workspace = Path(directory)
        # Relative helper invocations keep exercising the workflow-owned code.
        for name in (".github", ".agents"):
            (workspace / name).symlink_to(ROOT / name, target_is_directory=True)
        axe = workspace / "axe"
        axe.write_text('#!/bin/bash\nprintf "%s\\n" "$AXE_TEST_OUTPUT"\n')
        axe.chmod(0o700)
        env = {
            "PATH": f"{workspace}:{os.defpath}",
            "HOME": directory,
            "GITHUB_WORKSPACE": directory,
            "AXE": str(axe),
            "AXE_TEST_OUTPUT": output,
        }
        return subprocess.run(
            ["bash", "--noprofile", "--norc", "-e", "-c", shell],
            cwd=workspace, env=env, capture_output=True, text=True, timeout=10,
        )


class DrySkillSmokeTest(unittest.TestCase):
    def test_should_reject_empty_skill_when_only_files_contain_skill_marker(self):
        # Arrange: an empty Skill ends immediately at the next section header.
        shell = dry_skill_shell()
        template = """=== Dry Run ===
--- Skill ---
{skill}

--- Files (1) ---
smoke-skill-loaded
"""
        loaded = template.format(skill="# Smoke Test Skill\nsmoke-skill-loaded")
        missing = template.format(skill="(none)")
        empty = "=== Dry Run ===\n--- Skill ---\n--- Files (1) ---\nsmoke-skill-loaded\n"

        # Act through the unchanged workflow shell, retaining both controls.
        positive = run_smoke_step(shell, loaded)
        negative = run_smoke_step(shell, missing)
        empty_result = run_smoke_step(shell, empty)

        # Assert: marker text in Files must not substitute for a loaded Skill.
        self.assertEqual(positive.returncode, 0, positive.stdout + positive.stderr)
        self.assertNotEqual(
            negative.returncode, 0,
            "dry-skill accepted '(none)' in Skill with the marker in Files\n"
            f"stdout:\n{negative.stdout}\nstderr:\n{negative.stderr}",
        )
        self.assertNotEqual(
            empty_result.returncode, 0,
            "dry-skill accepted empty Skill with the marker only in Files\n"
            f"stdout:\n{empty_result.stdout}\nstderr:\n{empty_result.stderr}",
        )


if __name__ == "__main__":
    unittest.main()
