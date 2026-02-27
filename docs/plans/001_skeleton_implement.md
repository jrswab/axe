# Implementation Checklist: M1 - Skeleton CLI

**Based on:** 001_skeleton_spec.md  
**Status:** In Progress  
**Created:** 2026-02-26  

---

## Phase 1: Project Setup

- [x] Initialize Go module with `go mod init github.com/jrswab/axe`
- [x] Install Cobra dependency with `go get github.com/spf13/cobra`
- [x] Create project directory structure:
  - [x] `cmd/` directory
  - [x] `internal/xdg/` directory
  - [x] `skills/sample/` directory
- [x] Create `main.go` entry point file

---

## Phase 2: Core Infrastructure

- [x] Implement `internal/xdg/xdg.go`:
  - [x] Create `GetConfigDir()` function using `os.UserConfigDir()`
  - [x] Handle `XDG_CONFIG_HOME` environment variable override
  - [x] Use `filepath.Join()` for OS-appropriate path separators
  - [x] Return error if home directory cannot be determined

- [x] Create `cmd/root.go`:
  - [x] Define root command structure with Cobra
  - [x] Set command name to "axe"
  - [x] Add brief description
  - [x] Implement version constant (hardcoded "0.1.0")
  - [x] Wire up subcommands

---

## Phase 3: Commands Implementation

- [x] Implement `cmd/version.go`:
  - [x] Create `version` command
  - [x] Print exactly "axe version 0.1.0" to stdout
  - [x] Return exit code 0
  - [x] Add command to root

- [x] Implement `cmd/config.go`:
  - [x] Create `config` parent command (no direct action)
  - [x] Create `config path` subcommand
    - [x] Call `xdg.GetConfigDir()` to get path
    - [x] Print full absolute path to stdout (single line)
    - [x] Handle errors (print to stderr, exit 1)
    - [x] Return exit code 0 on success
  - [x] Create `config init` subcommand
    - [x] Get config directory path
    - [x] Create `agents/` subdirectory (recursively if needed)
    - [x] Create `skills/sample/` subdirectory (recursively if needed)
    - [x] Copy embedded `skills/sample/SKILL.md` to config directory
    - [x] Implement idempotency (silent success if exists)
    - [x] Print resulting config path on success
    - [x] Handle permission errors (exit 1, print to stderr)
    - [x] Handle file copy errors (exit 1, print to stderr)
  - [x] Add all config subcommands to root

---

## Phase 4: Assets

- [x] Create `skills/sample/SKILL.md` template:
  - [x] Add skill name/title header
  - [x] Add Purpose section
  - [x] Add Instructions section
  - [x] Add Output Format section
  - [x] Ensure file is embedded in binary (use embed package)

---

## Phase 5: Testing

- [x] Verify `go build` produces binary without errors
- [x] Test `axe version`:
  - [x] Output matches exactly "axe version 0.1.0"
  - [x] Exit code is 0
- [x] Test `axe help`:
  - [x] Displays available commands
  - [x] Displays usage examples
  - [x] Exit code is 0
- [x] Test `axe config path`:
  - [x] Prints valid path for current platform
  - [x] Works when XDG_CONFIG_HOME is set
  - [x] Works when XDG_CONFIG_HOME is unset
  - [x] Exit code is 0
- [x] Test `axe config init`:
  - [x] Creates `agents/` directory
  - [x] Creates `skills/sample/` directory
  - [x] Copies SKILL.md template
  - [x] Idempotent: running twice succeeds silently
  - [x] Does not overwrite existing files
  - [x] Exit code is 0 on success
  - [x] Exit code is 1 on permission errors
- [x] Verify exit codes:
  - [x] All successful commands return 0
  - [x] All error conditions return 1

---

## Phase 6: Verification

- [x] Run `go mod tidy` to clean dependencies
- [x] Verify only Cobra is in go.mod (plus stdlib)
- [x] Check go.sum is generated
- [x] Verify all files compile on:
  - [x] Linux
  - [x] macOS (if available)
  - [x] Windows cross-compilation check (optional)
- [x] Final binary test:
  - [x] Binary runs from any directory
  - [x] All commands work as specified
  - [x] No hardcoded paths
  - [x] Proper XDG path resolution

---

## Definition of Done

- [x] All checkboxes in Phase 1-6 are completed
- [x] All acceptance criteria from 001_skeleton_spec.md are met
- [x] Binary builds successfully with `go build`
- [x] All tests pass (manual verification)
- [x] Ready for M2: Agent Config implementation
