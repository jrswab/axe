"""Release admission and package checks using local Git and synthetic archives."""

import hashlib
import io
from pathlib import Path
import runpy
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock
import zipfile


ROOT = Path(__file__).resolve().parents[3]
CHECK = runpy.run_path(str(ROOT / ".agents/scripts/release-check"))


def git(root, *args):
    return subprocess.run(["git", *args], cwd=root, check=True, capture_output=True, text=True).stdout.strip()


class AdmissionTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="axe-release-admit-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        git(self.root, "init", "-q")
        git(self.root, "config", "user.name", "Fixture")
        git(self.root, "config", "user.email", "fixture@example.invalid")
        (self.root / "CHANGELOG.md").write_text("## [1.2.3]\n### Added\n- A change\n\n## [Older]\nold\n")
        git(self.root, "add", "CHANGELOG.md")
        git(self.root, "commit", "-qm", "fixture")
        self.head = git(self.root, "rev-parse", "HEAD")
        git(self.root, "tag", "-a", "v1.2.3", "-m", "fixture")
        git(self.root, "update-ref", "refs/remotes/origin/master", self.head)
        self.notes = self.root / "notes.md"

    def admit(self, tag="v1.2.3", ref="refs/tags/v1.2.3", sha=None):
        CHECK["admit"](tag, ref, self.head if sha is None else sha, self.notes, self.root)

    def test_admission_and_ancestry(self):
        scripts = self.root / ".agents/scripts"
        scripts.mkdir(parents=True)
        shutil.copy(ROOT / ".agents/scripts/release-check", scripts)
        command = [str(scripts / "release-check"), "admit", "--tag", "v1.2.3", "--ref",
                   "refs/tags/v1.2.3", "--sha", self.head, "--notes", str(self.notes)]
        result = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.notes.read_text(), "### Added\n- A change\n")
        result = subprocess.run(command[:-4] + ["--sha", "0" * 40, "--notes", str(self.notes)],
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn("must match", result.stderr)
        (self.root / "new").write_text("new")
        git(self.root, "add", "new")
        git(self.root, "commit", "-qm", "off branch")
        git(self.root, "tag", "v1.2.4")
        new_head = git(self.root, "rev-parse", "HEAD")
        with self.assertRaisesRegex(ValueError, "not reachable"):
            self.admit("v1.2.4", "refs/tags/v1.2.4", new_head)
        with self.assertRaisesRegex(ValueError, "must match"):
            self.admit()
        git(self.root, "reset", "--hard", self.head)
        with self.assertRaisesRegex(ValueError, "must match"):
            self.admit(sha="0" * 40)

    def test_reference_definitions_are_not_release_notes(self):
        text = "## [1.2.3]\n### Added\n\n[1.2.3]: https://example.invalid/release\n"
        with self.assertRaisesRegex(ValueError, "no meaningful body"):
            CHECK["notes_for"]("v1.2.3", text)
        for definition in ("   [a\\]b]: <https://example.invalid>",
                           '[ref]: https://example.invalid\n  "Link title"',
                           '[ref]: /url "Title with \\"escaped\\" quotes"',
                           '[ref]: /url "A title\non two lines"',
                           r"[ref]: <https://example.invalid/a\>b>",
                           "[ref]:\nhttps://example.invalid"):
            with self.subTest(definition=definition), self.assertRaisesRegex(ValueError, "no meaningful body"):
                CHECK["notes_for"]("v1.2.3", "## [1.2.3]\n" + definition + "\n")
        prose = "## [1.2.3]\n### Added\nA plain prose note.\n\n[1.2.3]: https://example.invalid/release\n"
        self.assertEqual(CHECK["notes_for"]("v1.2.3", prose), prose.split("\n", 1)[1])
        self.assertEqual(CHECK["notes_for"]("v1.2.3", "## [1.2.3]\n[Note]: plain prose here\n"),
                         "[Note]: plain prose here\n")
        paragraphs = '## [1.2.3]\n[ref]: /url\n"Prose\n\ncontinues"\n'
        self.assertEqual(CHECK["notes_for"]("v1.2.3", paragraphs), paragraphs.split("\n", 1)[1])
        self.assertEqual(CHECK["notes_for"]("v1.2.3", "## [1.2.3]\n- A bullet\n"), "- A bullet\n")

    def test_invalid_ref_and_changelog(self):
        for tag, ref in (("1.2.3", "refs/tags/1.2.3"), ("v1.2.3", "refs/heads/master")):
            with self.subTest(tag=tag), self.assertRaisesRegex(ValueError, "event must"):
                self.admit(tag, ref)
        with self.assertRaises(subprocess.CalledProcessError):
            self.admit("vbad..ref", "refs/tags/vbad..ref")
        cases = {
            "missing": "## [1.2.30]\n- wrong\n",
            "duplicate": "## [1.2.3]\n- first\n## [1.2.3]\n- second\n",
            "empty": "## [1.2.3]\n### Added\n<!-- hidden -->\n## Other\n- not this section\n",
            "near match": "## [1X2X3]\n- wrong\n",
        }
        for name, text in cases.items():
            with self.subTest(name=name):
                (self.root / "CHANGELOG.md").write_text(text)
                with self.assertRaises(ValueError):
                    self.admit()
        self.assertEqual(CHECK["notes_for"]("v1.2.3+rc.1", "## [1.2.3+rc.1]\n- literal\n"), "- literal\n")
        with self.assertRaises(ValueError):
            CHECK["notes_for"]("v1.2.3+rc.1", "## [1.2.33rc.1]\n- wrong\n")


class ArtifactsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="axe-release-artifacts-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.dist = self.root / "dist"
        self.dist.mkdir()
        (self.root / "cmd").mkdir()
        (self.root / "cmd/root.go").write_text('const Version = "1.2.3"\n')
        scripts = self.root / ".agents/scripts"
        scripts.mkdir(parents=True)
        shutil.copy(ROOT / ".agents/scripts/check-dry-skill", scripts)
        self.binary = b'#!/bin/sh\nif [ "$1" = version ]; then echo "axe version 1.2.3"; else printf "%s\\n" "--- Skill ---" "smoke-skill-loaded" "--- Files (0) ---"; fi\n'
        self.names = [f"axe_1.2.3_{system}_{arch}.{'zip' if system == 'windows' else 'tar.gz'}"
                      for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")]
        for name in self.names:
            self.archive(name)
        self.checksums()

    def archive(self, name, *, binary=None, extra=None, missing=None, mode=0o755):
        binary = self.binary if binary is None else binary
        windows = name.endswith(".zip")
        members = {"axe.exe" if windows else "axe": binary, "LICENSE": b"license", "README.md": b"readme"}
        if missing:
            del members[missing]
        if extra:
            members[extra] = b"unsafe"
        path = self.dist / name
        if windows:
            with zipfile.ZipFile(path, "w") as archive:
                for member, contents in members.items():
                    archive.writestr(member, contents)
        else:
            with tarfile.open(path, "w:gz") as archive:
                for member, contents in members.items():
                    info = tarfile.TarInfo(member)
                    info.size = len(contents)
                    info.mode = mode if member == "axe" else 0o644
                    archive.addfile(info, io.BytesIO(contents))

    def checksums(self):
        (self.dist / "checksums.txt").write_text("".join(
            f"{hashlib.sha256((self.dist / name).read_bytes()).hexdigest()}  {name}\n" for name in sorted(self.names)))

    def check(self):
        CHECK["artifacts"]("v1.2.3", self.dist, self.root)

    def test_valid_and_archive_set(self):
        self.check()
        (self.dist / self.names[0]).unlink()
        with self.assertRaisesRegex(ValueError, "archive set mismatch"):
            self.check()
        self.archive(self.names[0])
        (self.dist / "unexpected.zip").write_bytes(b"")
        with self.assertRaisesRegex(ValueError, "unexpected"):
            self.check()
        (self.dist / "unexpected.zip").unlink()
        path = self.dist / self.names[0]
        path.unlink()
        path.symlink_to(self.dist / self.names[1])
        with self.assertRaisesRegex(ValueError, "not a regular file"):
            self.check()

    def test_checksum_failures(self):
        (self.dist / self.names[0]).write_bytes(b"corrupt")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            self.check()
        self.checksums()
        with self.assertRaisesRegex(ValueError, "invalid gzip archive"):
            self.check()
        self.archive(self.names[0])
        self.checksums()
        manifest = self.dist / "checksums.txt"
        lines = manifest.read_text().splitlines()
        manifest.write_text("\n".join(lines[:-1]) + "\n")
        with self.assertRaisesRegex(ValueError, "exactly the six"):
            self.check()
        manifest.write_text("\n".join(lines + [lines[0]]) + "\n")
        with self.assertRaisesRegex(ValueError, "duplicate"):
            self.check()
        manifest.write_text("not a checksum\n")
        with self.assertRaisesRegex(ValueError, "invalid"):
            self.check()

    def test_unsafe_or_missing_members(self):
        for name, kwargs in ((self.names[0], {"extra": "../evil"}),
                             (self.names[4], {"extra": "../evil"}),
                             (self.names[0], {"missing": "LICENSE"})):
            with self.subTest(name=name, kwargs=kwargs):
                self.archive(name, **kwargs)
                self.checksums()
                with self.assertRaises(ValueError):
                    self.check()
                self.archive(name)
        path = self.dist / self.names[0]
        with tarfile.open(path, "w:gz") as archive:
            info = tarfile.TarInfo("axe")
            info.type = tarfile.SYMTYPE
            info.linkname = "../outside"
            archive.addfile(info)
        self.checksums()
        with self.assertRaisesRegex(ValueError, "unsafe"):
            self.check()
        self.archive(self.names[4])
        with zipfile.ZipFile(self.dist / self.names[4], "w") as archive:
            info = zipfile.ZipInfo("axe.exe")
            info.create_system = 3
            info.external_attr = (0o120777 << 16)
            archive.writestr(info, "../outside")
        self.archive(self.names[0])
        self.checksums()
        with self.assertRaisesRegex(ValueError, "unsafe"):
            self.check()

    def test_unix_binary_owner_permissions_and_preserved_mode(self):
        for name, mode in ((self.names[0], 0o645), (self.names[0], 0o644),
                           (self.names[1], 0o644), (self.names[3], 0o644),
                           (self.names[0], 0o111)):
            with self.subTest(name=name, mode=oct(mode)):
                self.archive(name, mode=mode)
                self.checksums()
                with self.assertRaisesRegex(ValueError, "owner-readable and executable"):
                    self.check()
                self.archive(name)
        self.checksums()
        self.check()  # All Unix binaries have the normal 0755 mode.
        self.archive(self.names[0], mode=0o705)
        self.checksums()
        original_chmod = Path.chmod
        with mock.patch.object(Path, "chmod", autospec=True, side_effect=original_chmod) as chmod:
            self.check()
        self.assertEqual(chmod.call_count, 1)
        self.assertEqual(chmod.call_args.args[1], 0o705)

    def test_gzip_trailers_are_validated(self):
        path = self.dist / self.names[0]
        for corruption in ("truncated", "crc", "size"):
            with self.subTest(corruption=corruption):
                self.archive(self.names[0])
                data = bytearray(path.read_bytes())
                if corruption == "truncated":
                    del data[-8:]
                else:
                    data[-8 if corruption == "crc" else -4] ^= 1
                path.write_bytes(data)
                self.checksums()
                with self.assertRaisesRegex(ValueError, "invalid gzip archive"):
                    self.check()
                result = subprocess.run([str(ROOT / ".agents/scripts/release-check"), "artifacts",
                                         "--tag", "v1.2.3", "--dist", str(self.dist)],
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 1)
                self.assertIn("invalid gzip archive", result.stderr)
                self.assertNotIn("Traceback", result.stderr)

    def test_binary_version_execution_and_skill(self):
        name = "axe_1.2.3_linux_amd64.tar.gz"
        cases = ((self.binary.replace(b"1.2.3", b"9.9.9"), 0o755, "version mismatch"),
                 (b"not executable\n", 0o755, None),
                 (self.binary.replace(b"smoke-skill-loaded", b"(none)"), 0o755, None))
        for binary, mode, message in cases:
            with self.subTest(message=message, binary=binary[:20]):
                self.archive(name, binary=binary, mode=mode)
                self.checksums()
                with self.assertRaises((ValueError, OSError, subprocess.CalledProcessError)) as error:
                    self.check()
                if message:
                    self.assertIn(message, str(error.exception))


if __name__ == "__main__":
    unittest.main()
