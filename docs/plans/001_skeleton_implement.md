# Implementation Checklist: M1 - Skeleton CLI

**Based on:** 001_skeleton_spec.md  
**Status:** In Progress  
**Created:** 2026-02-26  

---

## Phase 1: Project Setup

- [ ] Initialize Go module with `go mod init github.com/jrswab/axe`
- [ ] Install Cobra dependency with `go get github.com/spf13/cobra`
- [ ] Create project directory structure:
  - [ ] `cmd/` directory
  - [ ] `internal/xdg/` directory
  - [ ] `skills/sample/` directory
- [ ] Create `main.go` entry point file

---

## Phase 2: Core Infrastructure

- [ ] Implement `internal/xdg/xdg.go`:
  - [ ] Create `GetConfigDir()` function using `os.UserConfigDir()`
  - [ ] Handle `XDG_CONFIG_HOME` environment variable override
  - [ ] Use `filepath.Join()` for OS-appropriate path separators
  - [ ] Return error if home directory cannot be determined

- [ ] Create `cmd/root.go`:
  - [ ] Define root command structure with Cobra
  - [ ] Set command name to "axe"
  - [ ] Add brief description
  - [ ] Implement version constant (hardcoded "0.1.0")
  - [ ] Wire up subcommands

---

## Phase 3: Commands Implementation

- [ ] Implement `cmd/version.go`:
  - [ ] Create `version` command
  - [ ] Print exactly "axe version 0.1.0" to stdout
  - [ ] Return exit code 0
  - [ ] Add command to root

- [ ] Implement `cmd/config.go`:
  - [ ] Create `config` parent command (no direct action)
  - [ ] Create `config path` subcommand
    - [ ] Call `xdg.GetConfigDir()` to get path
    - [ ] Print full absolute path to stdout (single line)
    - [ ] Handle errors (print to stderr, exit 1)
    - [ ] Return exit code 0 on success
  - [ ] Create `config init` subcommand
    - [ ] Get config directory path
    - [ ] Create `agents/` subdirectory (recursively if needed)
    - [ ] Create `skills/sample/` subdirectory (recursively if needed)
    - [ ] Copy embedded `skills/sample/SKILL.md` to config directory
    - [ ] Implement idempotency (silent success if exists)
    - [ ] Print resulting config path on success
    - [ ] Handle permission errors (exit 1, print to stderr)
    - [ ] Handle file copy errors (exit 1, print to stderr)
  - [ ] Add all config subcommands to root

---

## Phase 4: Assets

- [ ] Create `skills/sample/SKILL.md` template:
  - [ ] Add skill name/title header
  - [ ] Add Purpose section
  - [ ] Add Instructions section
  - [ ] Add Output Format section
  - [ ] Ensure file is embedded in binary (use embed package)

---

## Phase 5: Testing

- [ ] Verify `go build` produces binary without errors
- [ ] Test `axe version`:
  - [ ] Output matches exactly "axe version 0.1.0"
  - [ ] Exit code is 0
- [ ] Test `axe help`:
  - [ ] Displays available commands
  - [ ] Displays usage examples
  - [ ] Exit code is 0
- [ ] Test `axe config path`:
  - [ ] Prints valid path for current platform
  - [ ] Works when XDG_CONFIG_HOME is set
  - [ ] Works when XDG_CONFIG_HOME is unset
  - [ ] Exit code is 0
- [ ] Test `axe config init`:
  - [ ] Creates `agents/` directory
  - [ ] Creates `skills/sample/` directory
  - [ ] Copies SKILL.md template
  - [ ] Idempotent: running twice succeeds silently
  - [ ] Does not overwrite existing files
  - [ ] Exit code is 0 on success
  - [ ] Exit code is 1 on permission errors
- [ ] Verify exit codes:
  - [ ] All successful commands return 0
  - [ ] All error conditions return 1

---

## Phase 6: Verification

- [ ] Run `go mod tidy` to clean dependencies
- [ ] Verify only Cobra is in go.mod (plus stdlib)
- [ ] Check go.sum is generated
- [ ] Verify all files compile on:
  - [ ] Linux
  - [ ] macOS (if available)
  - [ ] Windows cross-compilation check (optional)
- [ ] Final binary test:
  - [ ] Binary runs from any directory
  - [ ] All commands work as specified
  - [ ] No hardcoded paths
  - [ ] Proper XDG path resolution

---

## Definition of Done

- [ ] All checkboxes in Phase 1-6 are completed
- [ ] All acceptance criteria from 001_skeleton_spec.md are met
- [ ] Binary builds successfully with `go build`
- [ ] All tests pass (manual verification)
- [ ] Ready for M2: Agent Config implementation
