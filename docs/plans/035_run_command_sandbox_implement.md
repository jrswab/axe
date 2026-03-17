# Implementation Guide: `run_command` Path Sandbox Enforcement

**Spec:** `docs/plans/035_run_command_sandbox_spec.md`  
**GitHub Issue:** [#31](https://github.com/jrswab/axe/issues/31)

---

## 1. Context Summary

Associated milestone document: `docs/plans/035_run_command_sandbox_spec.md`

The `run_command` tool executes arbitrary shell commands via `sh -c` but only sets `cmd.Dir` to the workdir — it performs zero validation on the command string and inherits the full parent environment. This lets an agent trivially escape the path sandbox that protects all other file tools (`list_directory`, `read_file`, `write_file`, `edit_file`). The fix adds two layers of defense: a heuristic command validator that scans for absolute paths, `..` traversal, and tilde expansion outside the workdir, plus an environment restriction that sets `HOME`/`TMPDIR` to the workdir and strips all non-essential variables. This is explicitly heuristic — not airtight — and the spec documents that Docker is the recommended approach for full isolation.

---

## 2. Implementation Checklist

### Phase 1: Command Validation (new file + tests)

- [x] **Test: absolute path outside workdir rejected** — `internal/tool/command_validation_test.go`: `TestValidateCommand_AbsolutePathOutsideWorkdir`. Table-driven test. Call `validateCommand(workdir, "cat /etc/passwd")` where workdir is `t.TempDir()`. Assert non-nil error. Assert error string contains `"absolute path"` and `"/etc/passwd"`. Also test `"ls /tmp"`, `"echo hello > /tmp/out"`, `"FOO=/etc/passwd cmd"`.

- [x] **Test: absolute path inside workdir allowed** — `internal/tool/command_validation_test.go`: `TestValidateCommand_AbsolutePathInsideWorkdir`. Call `validateCommand(workdir, "cat "+workdir+"/file.txt")` where workdir is `t.TempDir()`. Assert nil error. Also test `workdir` alone (exact match) and `workdir+"/sub/deep"` (nested).

- [x] **Test: parent traversal escaping workdir rejected** — `internal/tool/command_validation_test.go`: `TestValidateCommand_ParentTraversalEscape`. Call `validateCommand(workdir, "cat ../../etc/passwd")`. Assert non-nil error. Assert error string contains `"parent traversal"`. Also test `"cat ../../../etc/shadow"`.

- [x] **Test: parent traversal staying inside workdir allowed** — `internal/tool/command_validation_test.go`: `TestValidateCommand_ParentTraversalInside`. Create a subdirectory inside `t.TempDir()`. Call `validateCommand(workdir, "cat subdir/../file.txt")`. Assert nil error.

- [x] **Test: tilde expansion rejected** — `internal/tool/command_validation_test.go`: `TestValidateCommand_TildeExpansion`. Call `validateCommand(workdir, "cat ~/secrets")`. Assert non-nil error. Assert error string contains `"home directory"` or `"tilde"`. Also test `"ls ~/"`, `"FOO=~/bar cmd"`, standalone `"~"` at start of token.

- [x] **Test: tilde in non-expansion position allowed** — `internal/tool/command_validation_test.go`: `TestValidateCommand_TildeInFilename`. Call `validateCommand(workdir, "cat file~backup")`. Assert nil error. Also test `"echo hello~world"`.

- [x] **Test: double-dot in filename allowed** — `internal/tool/command_validation_test.go`: `TestValidateCommand_DoubleDotInFilename`. Call `validateCommand(workdir, "cat file..bak")`. Assert nil error.

- [x] **Test: simple relative commands allowed** — `internal/tool/command_validation_test.go`: `TestValidateCommand_RelativeCommands`. Table-driven. Call `validateCommand(workdir, cmd)` for each of: `"echo hello"`, `"ls"`, `"cat file.txt"`, `"grep -r pattern ."`, `"ls | grep foo"`, `"echo hello > output.txt"`, `"echo a; echo b"`, `"echo a && echo b"`. Assert nil error for all.

- [x] **Test: workdir is root allows all absolute paths** — `internal/tool/command_validation_test.go`: `TestValidateCommand_RootWorkdir`. Call `validateCommand("/", "cat /etc/passwd")`. Assert nil error. (Degenerate case — all paths are within `/`.)

- [x] **Test: error message format is actionable** — `internal/tool/command_validation_test.go`: `TestValidateCommand_ErrorFormat`. Call `validateCommand(workdir, "cat /etc/passwd")`. Assert error string contains all three components: the pattern type, the offending token, and a workdir restriction message.

- [x] **Implement: `validateCommand()` function** — `internal/tool/command_validation.go`: `validateCommand(workdir, command string) error`. Create this new file in `package tool`. The function scans the raw command string for: (1) absolute paths starting with `/` that are not within `filepath.Clean(workdir)` per `isWithinDir()`, (2) `..` path components that resolve outside workdir when joined with workdir via `filepath.Clean(filepath.Join(workdir, token))`, (3) tilde `~` in shell expansion position (start of token preceded by whitespace/`=`/`:`/start-of-string, followed by `/` or end-of-token). Returns nil if the command is safe, or an error with an actionable message including the pattern type, offending token, and workdir restriction note. Include a doc comment stating this is a heuristic that can be bypassed by variable expansion, command substitution, and encoding tricks, and that Docker/container isolation is recommended for full sandboxing (spec requirements 2.4.1, 2.4.2). Reuse `isWithinDir()` from `path_validation.go` (same package, no import needed).

### Phase 2: Environment Restriction (helper + tests)

- [x] **Test: restricted env contains HOME set to workdir** — `internal/tool/command_validation_test.go`: `TestSandboxEnv_HomeSetToWorkdir`. Call `sandboxEnv(workdir)` where workdir is `t.TempDir()`. Assert the returned slice contains `"HOME="+workdir`.

- [x] **Test: restricted env contains TMPDIR set to workdir** — `internal/tool/command_validation_test.go`: `TestSandboxEnv_TmpdirSetToWorkdir`. Call `sandboxEnv(workdir)`. Assert the returned slice contains `"TMPDIR="+workdir`.

- [x] **Test: restricted env inherits PATH from parent** — `internal/tool/command_validation_test.go`: `TestSandboxEnv_PathInherited`. Set `os.Setenv("PATH", "/usr/bin:/bin")` (restore via `t.Setenv`). Call `sandboxEnv(workdir)`. Assert the returned slice contains `"PATH=/usr/bin:/bin"`.

- [x] **Test: restricted env strips non-allowlisted vars** — `internal/tool/command_validation_test.go`: `TestSandboxEnv_StripsNonAllowlisted`. Set `t.Setenv("SECRET_API_KEY", "leaked")` and `t.Setenv("EDITOR", "vim")`. Call `sandboxEnv(workdir)`. Assert the returned slice does NOT contain any entry starting with `"SECRET_API_KEY="` or `"EDITOR="`.

- [x] **Test: restricted env handles missing optional vars** — `internal/tool/command_validation_test.go`: `TestSandboxEnv_MissingOptionalVars`. Unset `LANG` and `LC_ALL` via `t.Setenv` (set to empty, or use `os.Unsetenv` with cleanup). Call `sandboxEnv(workdir)`. Assert no panic. Assert no entry starting with `"LANG="` or `"LC_ALL="` in the result.

- [x] **Implement: `sandboxEnv()` function** — `internal/tool/command_validation.go`: `sandboxEnv(workdir string) []string`. Builds an explicit `[]string` of `KEY=VALUE` entries for `cmd.Env`. Always includes: `HOME=<workdir>`, `TMPDIR=<workdir>`. Inherits from parent via `os.Getenv()` (only if non-empty): `PATH`, `LANG`, `LC_ALL`, `USER`, `LOGNAME`, `TERM`. Returns the slice. No other variables are included.

### Phase 3: Wire Into `runCommandExecute` (modify existing file + tests)

- [x] **Test: `runCommandExecute` rejects absolute path outside workdir** — `internal/tool/run_command_test.go`: `TestRunCommand_RejectsAbsolutePathOutsideWorkdir`. Call `runCommandEntry().Execute(ctx, ToolCall{Arguments: {"command": "cat /etc/passwd"}}, ExecContext{Workdir: tmpdir})`. Assert `IsError` is true. Assert `Content` contains `"absolute path"`.

- [x] **Test: `runCommandExecute` rejects parent traversal escape** — `internal/tool/run_command_test.go`: `TestRunCommand_RejectsParentTraversal`. Call with `"cat ../../etc/passwd"`. Assert `IsError` is true. Assert `Content` contains `"parent traversal"`.

- [x] **Test: `runCommandExecute` rejects tilde expansion** — `internal/tool/run_command_test.go`: `TestRunCommand_RejectsTildeExpansion`. Call with `"cat ~/secrets"`. Assert `IsError` is true.

- [x] **Test: `runCommandExecute` allows absolute path inside workdir** — `internal/tool/run_command_test.go`: `TestRunCommand_AllowsAbsolutePathInsideWorkdir`. Create a file inside `t.TempDir()`. Call with `"cat "+tmpdir+"/file.txt"`. Assert `IsError` is false. Assert `Content` contains the file contents.

- [x] **Test: `runCommandExecute` sets HOME to workdir** — `internal/tool/run_command_test.go`: `TestRunCommand_HomeSetToWorkdir`. Call with `"echo $HOME"` and `ExecContext{Workdir: tmpdir}`. Assert `IsError` is false. Assert `Content` (trimmed) equals the workdir path (resolve symlinks for comparison).

- [x] **Test: `runCommandExecute` strips non-allowlisted env vars** — `internal/tool/run_command_test.go`: `TestRunCommand_StripsEnvVars`. Set `t.Setenv("SECRET_TOKEN", "leaked")`. Call with `"echo $SECRET_TOKEN"` and `ExecContext{Workdir: tmpdir}`. Assert `IsError` is false. Assert `Content` (trimmed) is empty (variable is unset in child).

- [x] **Test: existing 10 tests still pass** — Run `go test ./internal/tool/ -run TestRunCommand` and verify all 10 original tests pass unchanged. (No code change needed — this is a verification step.)

- [x] **Modify: `runCommandExecute()`** — `internal/tool/run_command.go`: `runCommandExecute()`. Insert a call to `validateCommand(ec.Workdir, command)` between the empty-command check (line 67–73) and the `exec.CommandContext` call (line 75). If `validateCommand` returns a non-nil error, return `provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}`. After creating the `cmd` via `exec.CommandContext`, set `cmd.Env = sandboxEnv(ec.Workdir)` (new line after `cmd.Dir = ec.Workdir` on line 76).

- [x] **Modify: `runCommandDefinition()`** — `internal/tool/run_command.go`: `runCommandDefinition()`. Update the `Description` field (line 34) to: `"Execute a shell command in the agent's working directory via sh -c and return the combined stdout/stderr output. Commands are sandboxed to the working directory: absolute paths outside it are rejected, parent traversal (..) escaping it is rejected, and all file operations should use relative paths."`.

### Phase 4: Verify All Tests Pass

- [x] **Run full test suite** — Execute `go test ./...` from the project root. All existing tests (including the 10 original `run_command` tests, all `path_validation` tests, all `registry` tests, all integration tests) must pass with zero failures. Fix any regressions before considering the implementation complete.
