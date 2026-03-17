# Specification: `run_command` Path Sandbox Enforcement

**Status:** Draft  
**Version:** 1.0  
**Created:** 2026-03-16  
**GitHub Issue:** [#31 — bug: path sandbox can be bypassed via shell commands in tool calls](https://github.com/jrswab/axe/issues/31)  
**Scope:** Heuristic command validation and environment restriction for the `run_command` tool to close the path sandbox bypass gap

---

## 1. Context & Constraints

### Associated Milestone

This is a standalone bug fix. There is no milestone document — the work is scoped by GitHub Issue #31.

The original `run_command` tool was implemented in Tool Call Milestone M7 (`docs/plans/000_tool_call_milestones.md`, lines 107–124) with the explicit design decision that "the tool does not restrict file access. Sandboxing is the agent config's responsibility" (018 spec, Section 5.6). Issue #31 identifies this as a concrete security gap: the path sandbox protects built-in file tools (`list_directory`, `read_file`, `write_file`, `edit_file`) but does not restrict shell-level file access via `run_command`.

### Codebase Structure Relevant to This Work

| File | Role | Lines |
|------|------|-------|
| `internal/tool/run_command.go` | Tool definition + execution logic | 110 lines |
| `internal/tool/run_command_test.go` | 10 existing test cases | 215 lines |
| `internal/tool/path_validation.go` | `validatePath()` and `isWithinDir()` — used by file tools | 56 lines |
| `internal/tool/path_validation_test.go` | 11 tests for path validation | 180 lines |
| `internal/tool/registry.go` | `ExecContext` struct, `Registry`, `RegisterAll` | 100 lines |

**Current `run_command` behavior:**
- Executes via `exec.CommandContext(ctx, "sh", "-c", command)` with `cmd.Dir = ec.Workdir`
- No validation of the command string
- Inherits the full parent process environment
- The shell process starts in the workdir but is not confined to it

**Current file tool sandbox behavior (`path_validation.go`):**
- `validatePath(workdir, relPath)` rejects: empty paths, absolute paths, `..` traversal escaping workdir, symlink escapes
- `isWithinDir(child, parent)` does boundary-safe prefix checking (handles `/tmp/work` vs `/tmp/worker`)
- Used by `list_directory`, `read_file`, `edit_file`; `write_file` has inline duplicate logic

### Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| **Combined approach: heuristic path guard + environment restriction** | User chose "belt and suspenders" — both layers of defense. Heuristic catches common bypasses; env restriction closes `$HOME` expansion and reduces attack surface. |
| **This is heuristic, not airtight** | Perfect shell command sandboxing is provably impossible without OS-level containment (namespaces, seccomp, Docker). The heuristic raises the bar significantly but cannot prevent all bypasses (variable expansion, encoding tricks, subshell indirection). This must be documented. |
| **`run_command` tool description updated** | The LLM-facing tool description must state the sandbox constraints so the model is less likely to attempt out-of-bounds commands. |
| **Axe is Unix-first** | `sh -c` is the execution model. No Windows considerations. |

### Approaches Ruled Out

| Approach | Why Ruled Out |
|----------|--------------|
| **Full shell AST parsing** | Shell is Turing-complete. Parsing all possible command constructs to extract file paths is impossible in the general case. Variable expansion, command substitution, eval, aliases, and sourced scripts all defeat static analysis. |
| **chroot / namespace isolation** | Requires root privileges or CAP_SYS_ADMIN. Violates "single binary, zero runtime" — axe should not require elevated permissions. |
| **seccomp-bpf filtering** | Linux-only, requires careful syscall allowlisting, fragile across kernel versions, and would break legitimate commands that need syscalls outside the allowlist. |
| **Removing `run_command` entirely** | The tool is essential for agent utility. The fix should restrict it, not remove it. |
| **Documentation-only fix** | Insufficient. The heuristic guard catches the common case and provides defense-in-depth. Documentation alone leaves users vulnerable to the obvious attack vector. |

### Constraints

- **No new external dependencies.** All changes use Go stdlib only.
- **No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/` packages.**
- **Existing tests must continue to pass without modification.** The 10 existing `run_command` tests and all other tests in the project must remain green.
- **Red/green TDD required.** Tests are written first (red), then implementation (green).
- **`ExecContext` struct may be extended** if needed (e.g., to carry sandbox configuration), but changes must be backward-compatible.
- **Error messages must be actionable.** When a command is rejected, the error must tell the user what was rejected and why, so they can restructure the command.
- **Conservative rejection policy.** False positives (rejecting a valid command) are preferred over false negatives (allowing an escape). Users can restructure commands to work within the sandbox.

### Open Questions Resolved

| # | Question | Answer |
|---|----------|--------|
| 1 | Should the heuristic be bypassable via config? | **No.** The heuristic is always active when `run_command` is used. If a user needs unrestricted shell access, they should use Docker (as recommended in docs). |
| 2 | Should `~` (tilde) expansion be blocked? | **Yes.** `~` expands to `$HOME` which, without env restriction, points outside the sandbox. With env restriction `HOME` is set to workdir, but blocking `~` in the heuristic provides defense-in-depth. |
| 3 | Should system binary paths like `/usr/bin/grep` be allowed? | **No.** Commands like `grep`, `cat`, `ls` are available via `PATH` without absolute paths. Allowing `/usr/bin/` would create an exception that's hard to reason about and could be exploited (e.g., `/usr/bin/../../../etc/passwd`). |
| 4 | What about commands that use `cd`? | The heuristic scans the full command string. `cd /etc && cat passwd` would be caught because `/etc` is an absolute path outside workdir. `cd subdir && cat file` is fine because it uses relative paths. |
| 5 | What environment variables should be preserved? | `PATH` (system default), `HOME` (set to workdir), `TMPDIR` (set to workdir), `LANG`, `LC_ALL`, `USER`, `LOGNAME`, `TERM`. All others are stripped. |

---

## 2. Requirements

### 2.1 Command Validation

**Requirement 2.1.1 (Absolute Path Detection):** Before executing a command, the system must scan the command string for absolute paths (tokens beginning with `/`). If any absolute path is found that does not resolve to a location within the agent's workdir, the command must be rejected.

**Requirement 2.1.2 (Path Resolution for Validation):** Absolute paths found in the command string must be cleaned via `filepath.Clean` before checking whether they fall within the workdir. The workdir itself must also be cleaned. The `isWithinDir` function (from `path_validation.go`) must be reused for the boundary check.

**Requirement 2.1.3 (Parent Traversal Detection):** The system must scan the command string for `..` path segments that, when resolved against the workdir, would escape the workdir boundary. The sequence `..` must be detected as a path component (preceded by `/` or start-of-token, followed by `/` or end-of-token), not as a substring of a filename (e.g., `file..bak` must NOT be rejected).

**Requirement 2.1.4 (Tilde Expansion Detection):** The system must reject commands containing `~` when it appears in a position where the shell would expand it to `$HOME`. Specifically: `~` at the start of a token (preceded by whitespace, `=`, `:`, or start-of-string) followed by `/` or end-of-token. The literal string `~` inside quotes or as part of a larger word (e.g., `file~backup`) must NOT be rejected.

**Requirement 2.1.5 (Rejection Error Format):** When a command is rejected, the tool must return a `ToolResult` with `IsError: true` and a `Content` string that includes: (a) what pattern was detected (e.g., "absolute path", "parent traversal", "home directory expansion"), (b) the specific offending token, and (c) a brief explanation that the command is restricted to the workdir. The error must be clear enough for the LLM to reformulate the command.

**Requirement 2.1.6 (Validation Ordering):** Command validation must occur after the empty-command check and before command execution. The validation order is: (1) empty check, (2) heuristic path validation, (3) execution.

**Requirement 2.1.7 (Workdir-Relative Absolute Paths Allowed):** Absolute paths that resolve to locations within the workdir must be allowed. For example, if workdir is `/home/user/project`, the command `cat /home/user/project/README.md` must be permitted.

### 2.2 Environment Restriction

**Requirement 2.2.1 (Explicit Environment):** The shell process must NOT inherit the full parent process environment. Instead, `cmd.Env` must be set to an explicit list of environment variables.

**Requirement 2.2.2 (HOME Override):** The `HOME` environment variable must be set to the agent's workdir. This prevents `$HOME` expansion from escaping the sandbox.

**Requirement 2.2.3 (TMPDIR Override):** The `TMPDIR` environment variable must be set to the agent's workdir. This prevents temporary file creation outside the sandbox.

**Requirement 2.2.4 (PATH Preservation):** The `PATH` environment variable must be inherited from the parent process. Commands need access to system binaries.

**Requirement 2.2.5 (Locale Preservation):** The `LANG` and `LC_ALL` environment variables must be inherited from the parent process (if set). This prevents locale-related errors in commands.

**Requirement 2.2.6 (User Identity Preservation):** The `USER` and `LOGNAME` environment variables must be inherited from the parent process (if set). Some commands require user identity.

**Requirement 2.2.7 (Terminal Preservation):** The `TERM` environment variable must be inherited from the parent process (if set). This prevents terminal-related errors.

**Requirement 2.2.8 (All Other Variables Stripped):** Any environment variable not explicitly listed in Requirements 2.2.2–2.2.7 must NOT be passed to the shell process. This includes but is not limited to: `SHELL`, `EDITOR`, `SSH_AUTH_SOCK`, API keys, tokens, and credentials.

### 2.3 Tool Description Update

**Requirement 2.3.1 (LLM-Facing Description):** The `run_command` tool description (returned by the `Definition` function) must be updated to inform the LLM that: (a) commands are restricted to the agent's working directory, (b) absolute paths outside the working directory are not allowed, (c) parent traversal (`..`) escaping the working directory is not allowed, and (d) all file operations should use relative paths from the working directory.

### 2.4 Documentation

**Requirement 2.4.1 (Heuristic Limitation Documented):** The command validation is heuristic and can be bypassed by sufficiently creative shell constructs (variable expansion, command substitution, encoding tricks). This limitation must be documented in a code comment on the validation function.

**Requirement 2.4.2 (Docker Recommendation):** The code comment must note that Docker/container isolation is the recommended approach for full sandboxing.

### 2.5 Behavioral Compatibility

**Requirement 2.5.1 (Relative Commands Unaffected):** Commands that use only relative paths (e.g., `ls`, `cat file.txt`, `grep -r pattern .`, `echo hello`) must continue to work exactly as before.

**Requirement 2.5.2 (Pipe and Redirect Unaffected):** Shell features like pipes (`|`), redirects (`>`, `>>`, `<`), command chaining (`;`, `&&`, `||`), and subshells (`$(...)`) must continue to work when they do not involve out-of-bounds paths.

**Requirement 2.5.3 (Exit Code Behavior Unchanged):** The exit code handling (success, `ExitError`, non-exit errors) must remain identical to the current behavior.

**Requirement 2.5.4 (Output Truncation Unchanged):** The 100KB output truncation behavior must remain identical.

**Requirement 2.5.5 (Context Timeout Unchanged):** The context-based timeout behavior must remain identical.

### 2.6 Edge Cases

#### 2.6.1 Command Validation Edge Cases

| Scenario | Command Example | Expected Behavior |
|----------|----------------|-------------------|
| Simple relative command | `echo hello` | **Allowed.** No absolute paths or traversal. |
| Relative path to file | `cat subdir/file.txt` | **Allowed.** Relative to workdir. |
| Absolute path within workdir | `cat /home/user/project/file.txt` (workdir is `/home/user/project`) | **Allowed.** Path is within workdir. |
| Absolute path outside workdir | `cat /etc/passwd` | **Rejected.** Absolute path outside workdir. |
| Parent traversal escaping | `cat ../../etc/passwd` | **Rejected.** `..` resolves outside workdir. |
| Parent traversal staying inside | `cat subdir/../file.txt` | **Allowed.** Resolves to `file.txt` within workdir. |
| Tilde expansion | `cat ~/secrets` | **Rejected.** `~` expands to home directory. |
| Tilde in filename | `cat file~backup` | **Allowed.** `~` is not in expansion position. |
| Double-dot in filename | `cat file..bak` | **Allowed.** `..` is not a path component. |
| Multiple commands with pipe | `ls \| grep foo` | **Allowed.** No out-of-bounds paths. |
| Multiple commands, one bad | `echo ok; cat /etc/passwd` | **Rejected.** Contains absolute path outside workdir. |
| Absolute path in redirect | `echo hello > /tmp/out` | **Rejected.** `/tmp/out` is outside workdir. |
| Redirect to relative path | `echo hello > output.txt` | **Allowed.** Relative to workdir. |
| Command with equals sign | `FOO=/etc/passwd command` | **Rejected.** `/etc/passwd` is an absolute path outside workdir. |
| Empty command | `""` | **Rejected** by existing empty-command check (before validation). |
| Whitespace-only command | `"   "` | **Allowed.** Passes to shell as before (no absolute paths or traversal detected). |
| Workdir is root `/` | Any command | **Allowed.** All absolute paths are within `/`. (Degenerate case — user configured root as workdir.) |
| Path with trailing slash | `ls /etc/` | **Rejected.** `/etc/` is outside workdir. |
| Quoted absolute path | `echo "/etc/passwd"` | **Rejected.** The heuristic scans the raw command string, not shell-parsed tokens. Quoted paths are still detected. This is a known conservative false positive — the path is a string argument to `echo`, not a file access. Conservative rejection is preferred. |
| Backtick command substitution with bad path | `` echo `cat /etc/passwd` `` | **Rejected.** `/etc/passwd` is detected in the raw string. |
| Dollar command substitution with bad path | `echo $(cat /etc/passwd)` | **Rejected.** `/etc/passwd` is detected in the raw string. |

#### 2.6.2 Environment Restriction Edge Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| Command uses `$HOME` | Expands to workdir (set by env restriction). |
| Command uses `$TMPDIR` | Expands to workdir (set by env restriction). |
| Command uses `$PATH` | Works normally (inherited from parent). |
| Command uses `$SHELL` | Empty/unset (stripped). |
| Command uses `$EDITOR` | Empty/unset (stripped). |
| Command uses `$API_KEY` or similar | Empty/unset (stripped). Prevents credential leakage. |
| Command uses `env` to list vars | Shows only the restricted set. |
| Parent process has no `LANG` set | `LANG` is simply absent from child env. No error. |

#### 2.6.3 Interaction Between Validation and Environment

| Scenario | Expected Behavior |
|----------|-------------------|
| `cat ~/file` | **Rejected** by heuristic (tilde detection), even though `HOME` is set to workdir. Defense-in-depth. |
| `cat $HOME/file` | **Allowed** by heuristic (no absolute path literal detected). `$HOME` expands to workdir at runtime due to env restriction. This is the intended safe path for home-relative access. |
| `cat $(echo /etc/passwd)` | **Rejected.** `/etc/passwd` is detected as an absolute path literal in the raw command string. The heuristic scans the full string regardless of shell quoting or substitution context. |
