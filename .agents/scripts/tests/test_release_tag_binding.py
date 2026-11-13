"""Release commands must receive the triggering tag, even for same-commit tags.

This tests workflow wiring, not GoReleaser's independently verified tag selection.
Only block env maps, scalar values, and inline/literal run commands are supported;
unsupported syntax fails explicitly rather than approximating GitHub Actions YAML.
"""

import json
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[3]
STEPS = ("Build release packages without publishing", "Publish release")


def field(lines, indent, key):
    """Read one direct field at the workflow's known indentation, not nested fields."""
    fields = {}
    for i, line in enumerate(lines):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if len(line) - len(line.lstrip()) != indent:
            continue
        match = re.fullmatch(r" {" + str(indent) + r"}([A-Za-z_][A-Za-z0-9_-]*):(?:\s+(.*))?", line)
        if not match:
            raise ValueError(f"unsupported mapping field at indentation {indent}: {line!r}")
        name, value = match.groups()
        if name in fields:
            raise ValueError(f"duplicate {name!r} at indentation {indent}")
        fields[name] = i, (value or "").strip()
    for start, value in fields.values():
        if value and value not in ("|", ">"):
            for line in lines[start + 1:]:
                if not line.strip() or line.lstrip().startswith("#"):
                    continue
                if len(line) - len(line.lstrip()) <= indent:
                    break
                raise ValueError(f"unsupported nested content after {lines[start]!r}: {line!r}")
    if key not in fields:
        return None, []
    start, value = fields[key]
    end = start + 1
    while end < len(lines):
        line = lines[end]
        if (line.strip() and not line.lstrip().startswith("#")
                and len(line) - len(line.lstrip()) <= indent):
            break
        end += 1
    return value, lines[start + 1:end]


def scalar(value):
    if value.startswith('"'):
        return json.loads(value)
    if re.fullmatch(r"'(?:[^']|'')*'", value):
        return value[1:-1].replace("''", "'")
    if value.startswith(("|", ">", "&", "*", "{", "[", "!", "'")) or " #" in value:
        raise ValueError(f"unsupported workflow scalar: {value!r}")
    return value


def command_args(command):
    try:
        argv = shlex.split(command or "")
    except ValueError as error:
        raise ValueError(f"unsupported release command: {command!r}") from error
    if (not command or not re.fullmatch(
            r'goreleaser[ \t]+release(?:[ \t]+(?:--[a-z0-9-]+(?:=[A-Za-z0-9_./-]+)?'
            r'|--release-notes="\$RUNNER_TEMP/release-notes\.md"))*', command)
            or argv[:2] != ["goreleaser", "release"]):
        raise ValueError(f"unsupported release command: {command!r}")
    return argv


def environment(lines, indent, context):
    value, body = field(lines, indent, "env")
    if value is None:
        return {}
    if value:
        raise ValueError("expected a block env map")
    result = {}
    for line in body:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        match = re.fullmatch(r" {" + str(indent + 2) + r"}([A-Z_][A-Z0-9_]*):\s*(.*)", line)
        if not match:
            raise ValueError(f"unsupported env entry: {line!r}")
        key, value = match.groups()
        if key in result:
            raise ValueError(f"duplicate env key: {key}")
        if not value:
            raise ValueError(f"expected a scalar value for env key {key}")
        quoted = value.startswith(("'", '"'))
        value = scalar(value)
        expression = re.fullmatch(r"\$\{\{\s*([^{}]+?)\s*\}\}", value)
        if expression:
            expression_key = expression.group(1).strip()
            if expression_key not in context:
                raise ValueError(f"unsupported expression: {value!r}")
            value = context[expression_key]
        elif "${{" in value or (not quoted and (
                not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_./-]*", value)
                or value.lower() in {"true", "false", "null"})):
            raise ValueError(f"unsupported env scalar: {value!r}")
        result[key] = value
    return result


def release_invocations(workflow, context):
    lines = workflow.splitlines()
    jobs_value, jobs = field(lines, 0, "jobs")
    if jobs_value != "":
        raise ValueError("expected a block jobs map")
    job_value, job = field(jobs, 2, "goreleaser")
    if job_value != "" or not job:
        raise ValueError("expected a block jobs.goreleaser map")
    inherited = environment(lines, 0, context)
    inherited.update(environment(job, 4, context))
    steps_value, steps = field(job, 4, "steps")
    if steps_value != "":
        raise ValueError("expected a block steps list")
    # Known block step-list layout; do not interpret unrelated actions or run them.
    for line in steps:
        if line.strip() and not line.lstrip().startswith("#"):
            spaces = len(line) - len(line.lstrip())
            if spaces < 6 or (spaces == 6 and not line.startswith("      - ")):
                raise ValueError(f"unsupported step list entry: {line!r}")
    blocks = re.split(r"(?m)^      - ", "\n".join(steps))[1:]
    found = {}
    for block in blocks:
        lines = ("        " + block).splitlines()
        name, _ = field(lines, 8, "name")
        if name is not None:
            name = scalar(name)
        if name not in STEPS:
            continue
        if name in found:
            raise ValueError(f"duplicate release step: {name}")
        command, body = field(lines, 8, "run")
        if command == "|":
            command = textwrap.dedent("\n".join(body)).strip()
        elif command is not None:
            if any(line.strip() and not line.lstrip().startswith("#") for line in body):
                raise ValueError(f"unsupported run body: {body!r}")
            command = scalar(command)
        command_args(command)
        env = inherited.copy()
        env.update(environment(lines, 8, context))
        found[name] = command, env
    if set(found) != set(STEPS):
        raise ValueError(f"expected both release steps, found {list(found)}")
    return found


def record_invocation(command, workflow_env):
    argv = command_args(command)
    with tempfile.TemporaryDirectory(prefix="axe-release-tag-", dir="/tmp") as directory:
        root = Path(directory)
        recorder = (
            "import json, os, sys\n"
            "print(json.dumps({'argv': sys.argv[1:], "
            "'tag': os.environ.get('GORELEASER_CURRENT_TAG'), "
            "'sha': os.environ['GITHUB_SHA']}))\n"
        )
        # Pass only evidence fields: workflow env must not affect interpreter startup.
        env = {key: workflow_env[key] for key in ("GORELEASER_CURRENT_TAG", "GITHUB_SHA")
               if key in workflow_env}
        return subprocess.run(
            [sys.executable, "-I", "-S", "-c", recorder, *argv[2:]],
            cwd=root, env=env, capture_output=True, text=True, timeout=10,
        )


class ReleaseTagBindingTest(unittest.TestCase):
    def test_should_reject_unsupported_relevant_workflow_shapes(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text()
        binding = "      GORELEASER_CURRENT_TAG: ${{ github.ref_name }}"
        publish = "      - name: Publish release"
        run = '        run: goreleaser release --clean --release-notes="$RUNNER_TEMP/release-notes.md"'
        cases = {
            "root flow env": ("jobs:\n", "env: {GORELEASER_CURRENT_TAG: x}\njobs:\n"),
            "job flow env": ("    env:\n" + binding, "    env: {GORELEASER_CURRENT_TAG: x}"),
            "job env indented under scalar": ("    env:\n" + binding,
                                               "      env:\n  " + binding),
            "job quoted env key": ("    env:\n" + binding, "    'env':\n" + binding),
            "job merge env": ("    env:\n" + binding, "    <<: *environment"),
            "comment before malformed env": (binding, "# comment\n      'GORELEASER_CURRENT_TAG': x"),
            "empty env value": (binding, "      GORELEASER_CURRENT_TAG:"),
            "nested env value": (binding, "      GORELEASER_CURRENT_TAG:\n        nested: x"),
            "env list": (binding, "      - GORELEASER_CURRENT_TAG"),
            "env flow value": (binding, "      GORELEASER_CURRENT_TAG: [x]"),
            "unknown expression": (binding, "      GORELEASER_CURRENT_TAG: ${{ github.unknown }}"),
            "mixed expression": (binding, "      GORELEASER_CURRENT_TAG: prefix-${{ github.ref_name }}"),
            "step flow env": ("        env:\n          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}",
                              "        env: {GORELEASER_CURRENT_TAG: x}"),
            "step quoted env key": (publish, publish + "\n        'env': {GORELEASER_CURRENT_TAG: x}"),
            "step merge env": (publish, publish + "\n        <<: *environment"),
            "step env indented under name": (publish, publish + "\n          env: {GORELEASER_CURRENT_TAG: x}"),
            "step list flow": ("    permissions:\n      contents: write\n    steps:",
                               "    permissions:\n      contents: write\n    steps: []"),
            "step list merge": (publish, "      <<: *release\n" + publish),
            "command substitution": (run, "        run: goreleaser release --clean=$(touch /tmp/never)"),
            "command variable": (run, "        run: goreleaser release --clean=$OTHER"),
            "command glob": (run, "        run: goreleaser release --clean=*"),
            "command second line": (run, "        run: |\n          goreleaser release\n          --clean"),
            "command continuation": (run, "        run: goreleaser release\n          --clean"),
        }
        context = {"github.ref_name": "v1.12.0", "secrets.GITHUB_TOKEN": ""}
        for name, (old, new) in cases.items():
            with self.subTest(case=name):
                self.assertEqual(workflow.count(old), 1, name)
                with self.assertRaisesRegex(ValueError, "unsupported|expected"):
                    release_invocations(workflow.replace(old, new, 1), context)

    def test_should_not_execute_shell_or_workflow_controlled_startup(self):
        with tempfile.TemporaryDirectory(prefix="axe-release-safety-", dir="/tmp") as directory:
            root = Path(directory)
            marker = root / "executed"
            startup = root / "startup"
            startup.write_text(f": > {marker}\n")
            command = 'goreleaser release --clean --release-notes="$RUNNER_TEMP/release-notes.md"'
            result = record_invocation(command, {
                "BASH_ENV": str(startup), "PATH": str(root), "PYTHONPATH": str(root),
                "GORELEASER_CURRENT_TAG": "v1.12.0", "GITHUB_SHA": "same-sha",
            })
            self.assertFalse(marker.exists(), "workflow startup script was executed")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)["argv"], shlex.split(command)[2:])

    def test_should_preserve_event_tag_for_preflight_and_publish_at_same_commit(self):
        # Arrange: stable and RC tag events share a commit; SHA cannot identify a tag.
        workflow = (ROOT / ".github/workflows/release.yml").read_text()
        sha = "13229a11fcb779408a6d1b3b7b95adbdd507765b"
        for tag in ("v1.12.0", "v1.12.0-rc.1"):
            context = {"github.ref_name": tag, "github.ref": f"refs/tags/{tag}",
                       "github.sha": sha, "secrets.GITHUB_TOKEN": ""}
            for step, (command, env) in release_invocations(workflow, context).items():
                with self.subTest(event_tag=tag, step=step):
                    # Act: record parsed arguments and resolved env without a shell.
                    result = record_invocation(command, dict(env, GITHUB_SHA=sha))
                    evidence = (f"command: {command}\nexit: {result.returncode}\n"
                                f"stdout: {result.stdout}\nstderr: {result.stderr}")
                    self.assertEqual(result.returncode, 0, evidence)
                    observed = json.loads(result.stdout)
                    self.assertEqual(observed["argv"], command_args(command)[2:], evidence)
                    # Assert: observe the resolved env, with no tag fallback.
                    self.assertEqual(
                        (observed["tag"], observed["sha"]), (tag, sha),
                        "GORELEASER_CURRENT_TAG must bind to github.ref_name for "
                        f"{step}; same-commit tags must retain event identity\n{evidence}",
                    )


if __name__ == "__main__":
    unittest.main()
