"""Tests for the bounded dry-run skill assertion."""

from pathlib import Path
import subprocess
import unittest


HELPER = Path(__file__).resolve().parents[1] / "check-dry-skill"


class CheckDrySkillTest(unittest.TestCase):
    def test_skill_section(self):
        cases = {
            "loaded": ("--- Skill ---\n# Skill\nsmoke-skill-loaded\n--- Files (0) ---\n(none)\n", True),
            "missing section": ("--- Files (1) ---\nsmoke-skill-loaded\n", False),
            "empty section": ("--- Skill ---\n--- Files (1) ---\nsmoke-skill-loaded\n", False),
            "none": ("--- Skill ---\n(none)\n--- Files (1) ---\nsmoke-skill-loaded\n", False),
            "none with marker": ("--- Skill ---\n(none)\nsmoke-skill-loaded\n", False),
            "marker before skill": ("smoke-skill-loaded\n--- Skill ---\nother text\n", False),
            "marker after next section": ("--- Skill ---\nother text\n--- User Message ---\nsmoke-skill-loaded\n", False),
            "marker in second skill header": ("--- Skill ---\n--- Files (0) ---\n--- Skill ---\nsmoke-skill-loaded\n", False),
        }
        for name, (output, expected) in cases.items():
            with self.subTest(name=name):
                result = subprocess.run([str(HELPER)], input=output, text=True, capture_output=True)
                self.assertEqual(result.returncode == 0, expected, result.stderr)


if __name__ == "__main__":
    unittest.main()
