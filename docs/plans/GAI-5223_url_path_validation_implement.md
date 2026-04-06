# GAI-5223: URL Path Validation — Implementation Guide

**Spec:** [docs/plans/GAI-5223_url_path_validation_spec.md](GAI-5223_url_path_validation_spec.md)

---

## Section 1: Context Summary

The `run_command` tool's `validateCommand()` function uses a regex to find absolute paths in shell commands and rejects any that fall outside the workdir. This regex also matches URL path segments (e.g., `/api/v2/data` from `https://example.com/api/v2/data`), causing legitimate HTTP requests to be rejected. The fix masks HTTP/HTTPS URLs before the absolute-path scan so their path components are invisible to the check, while preserving all existing sandbox protections for real filesystem paths, `/dev/*` allowlisting, tilde expansion, and `..` traversal detection.

---

## Section 2: Implementation Checklist

### Implementation (can run in parallel with tests)

- [x] **Add URL regex** — `internal/tool/command_validation.go`: Add a package-level `var urlRe` compiled regex matching `https?://` followed by one or more non-whitespace, non-quote (single, double, backtick) characters. Place it alongside the existing `absPathRe`, `dotDotRe`, and `tildeRe` declarations (lines 8–20).

- [x] **Mask URLs before absolute-path scan** — `internal/tool/command_validation.go: validateCommand()`: Before the `absPathRe.FindAllString(command, -1)` call (line 74), create a masked copy of `command` where all `urlRe` matches are replaced with underscore characters of equal length. Pass the masked string to `absPathRe.FindAllString` instead of the raw `command`. Do not change the input to the tilde or `..` checks — they must continue using the original `command`.

### Tests (can run in parallel with implementation)

- [x] **Test URL-only commands pass** — `internal/tool/command_validation_test.go`: Add `TestValidateCommand_URLPathsAllowed` with table-driven subtests:
  - `"curl https://api.example.com/api/v2/data"` → `nil`
  - `"curl http://localhost:8080/graphql"` → `nil`
  - `"node -e 'fetch(\"https://youtube.com/api/v2/transcripts\")'"` → `nil`

- [x] **Test mixed URL + bad filesystem path** — `internal/tool/command_validation_test.go`: Add `TestValidateCommand_URLWithBadFilesystemPath` asserting `"curl https://example.com/api/v2 > /tmp/out"` returns an error containing `"absolute path"` and `"/tmp/out"`.

- [x] **Test non-HTTP scheme not masked** — `internal/tool/command_validation_test.go`: Add `TestValidateCommand_NonHTTPSchemeNotMasked` asserting `"curl ftp://example.com/etc/passwd"` returns an error containing `"absolute path"`.

- [x] **Test URL with sensitive-looking path passes** — `internal/tool/command_validation_test.go`: Add `TestValidateCommand_URLWithSensitivePath` asserting `"curl https://example.com/etc/passwd"` returns `nil`.

### Verification

- [x] **Run full test suite** — Execute `go test ./internal/tool/...` and confirm all existing tests pass alongside new tests. Zero test modifications to existing tests.
