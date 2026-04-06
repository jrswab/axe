# GAI-5223: URL Path Segments Must Not Trigger Command Sandbox Rejection

**Linear issue:** https://linear.app/gloo/issue/GAI-5223/unblock-external-api-calls-to-apiv

---

## Section 1: Context & Constraints

### Codebase Structure

- `internal/tool/command_validation.go` contains `validateCommand()`, the sole heuristic guard for the `run_command` tool.
- `validateCommand` is called from exactly one place: `internal/tool/run_command.go:75`.
- The absolute-path check uses `absPathRe` (line 12): `(/[\w./_-]*)` — it matches any `/` followed by path characters.
- After matching, each candidate is checked via `isWithinDir(cleanPath, cleanWorkdir)`. Paths outside the workdir are rejected.
- `internal/tool/path_validation.go` (`validatePath`) is a separate code path used by file tools (`read_file`, `write_file`, `edit_file`, `list_directory`). It is not affected by this change.

### The Problem

When a command contains a URL (e.g., `curl https://api.youtube.com/api/v2/transcripts`), `absPathRe` extracts the URL's path segment (`/api/v2/transcripts`) and `isWithinDir` rejects it because it is not inside the workdir. The validator cannot distinguish a URL path from a filesystem path.

The existing test `TestValidateCommand_QuotedAbsolutePathRejected` (line 340) documents this class of false positive as "known conservative."

### Decisions Already Made

- The command validator is explicitly documented as heuristic, not airtight (see docstring lines 22–31).
- `/dev/*` paths are already allowlisted (line 82–84).
- The `sandboxEnv` function strips environment variables and sets `HOME`/`TMPDIR` to workdir, limiting what a URL-shaped string could do even if it somehow became a file operation.

### Approaches Ruled Out

- **Full shell parsing:** Shell is Turing-complete; the docstring already acknowledges this. Not worth the complexity.
- **Allowlisting specific URL hostnames:** Too brittle; doesn't scale.
- **Removing the absolute path check entirely:** Would remove a real safety guard for filesystem paths.

### Constraints

- The fix must not weaken the sandbox for actual filesystem absolute paths.
- The tilde (`~`) and `..` traversal checks operate on the raw command string and must continue to do so (URLs do not trigger these patterns).
- `absPathRe` is used in exactly one place (line 74). The masking only needs to affect the input to that one `FindAllString` call.

---

## Section 2: Requirements

### R1: URL Masking Before Absolute-Path Scan

Before `absPathRe.FindAllString` is called, HTTP/HTTPS URLs in the command string must be replaced with inert placeholder characters so their path segments are not scanned.

- A URL is defined as: `https?://` followed by one or more non-whitespace, non-quote characters (single quote, double quote, backtick).
- The replacement must preserve string length (same number of characters) so that no positional side effects occur if future logic ever depends on match indices.
- Only the `absPathRe` scan uses the masked string. The tilde check and `..` traversal check must continue to operate on the original, unmasked command string.

### R2: Existing Filesystem Path Rejection Must Be Preserved

Commands containing actual filesystem absolute paths outside the workdir must still be rejected, even if the command also contains URLs.

**Example:** `curl https://example.com/api/v2 > /tmp/out` — the URL is allowed, but `/tmp/out` must still be rejected.

### R3: Existing Allowlists Must Be Preserved

- `/dev/*` paths must remain allowed.
- Paths inside the workdir must remain allowed.
- Root workdir (`/`) must continue to allow all absolute paths.

### R4: Edge Cases

| Input | Expected |
|---|---|
| `curl https://api.example.com/api/v2/data` | Pass (URL path masked) |
| `curl http://localhost:8080/graphql` | Pass (URL path masked) |
| `node -e 'fetch("https://youtube.com/api/v2/transcripts")'` | Pass (URL inside quotes, still masked) |
| `curl https://example.com/api/v2 > /tmp/out` | Fail on `/tmp/out` (URL masked, filesystem path still caught) |
| `cat /etc/passwd` | Fail (no URL, unchanged behavior) |
| `echo hello > /dev/null` | Pass (allowlisted, unchanged behavior) |
| `curl ftp://example.com/etc/passwd` | Fail on `/etc/passwd` (only `http://` and `https://` are masked) |
| `curl https://example.com/etc/passwd` | Pass (the `/etc/passwd` is part of the URL, not a filesystem access) |
| `cat ../../etc/passwd` | Fail (traversal check, unchanged behavior) |
| `cat ~/secrets` | Fail (tilde check, unchanged behavior) |

### R5: Test Coverage

New test cases must cover:
1. URL-only commands that previously failed (R4 rows 1–3) — must now pass.
2. Mixed URL + bad filesystem path (R4 row 4) — URL passes, filesystem path still rejected.
3. Non-HTTP scheme URLs are not masked (R4 row 7) — `ftp://` path segments still scanned.
4. URL containing a path that looks like a sensitive filesystem path (R4 row 8) — must pass because it's inside a URL.

All existing tests must continue to pass without modification.

### Parallelism

R1 (implementation) and R5 (tests) can be developed in parallel since the test cases are fully specified above. The tests will initially fail (red) until R1 is applied (green).
