---

# 045 — Agent Run Timeout Configuration: Implementation Guide

## Section 1: Context Summary

**Spec:** `045_agent_run_timeout_config_spec.md`

Issue #60 reports that `sub_agents_config.timeout` has no effect on the main agent run — it only controls sub-agent delegation timeouts. The fix is a new top-level `timeout` field in `AgentConfig` that sets the default run timeout per-agent in TOML. Resolution order is: explicit `--timeout` flag > TOML `timeout` field > hard default of 120 seconds. Cobra's `cmd.Flags().Changed("timeout")` distinguishes an explicit user-supplied flag from the flag sitting at its default value. The `--timeout` flag default (120) and `sub_agents_config.timeout` behavior are both unchanged.

---

## Section 2: Implementation Checklist

### Task 1 — Add `Timeout` field to `AgentConfig`

- [x] `internal/agent/agent.go`: Add `Timeout int \`toml:"timeout"\`` as a top-level field on the `AgentConfig` struct, after the `Workdir` field and before `Tools`.

---

### Task 2 — Validate the new field

- [x] `internal/agent/agent.go`: `Validate()` — add a guard immediately after the existing `SubAgentsConf.Timeout` validation block:
  ```
  if cfg.Timeout < 0 {
      return &ValidationError{msg: "timeout must be non-negative"}
  }
  ```

---

### Task 3 — Update the scaffold template

- [x] `internal/agent/agent.go`: `Scaffold()` — add `# timeout = 120` as a commented line near the top of the template, after `# workdir = ""` and before the `# sub_agents = []` line.

---

### Task 4 — Resolve effective timeout in `runAgent`

- [x] `cmd/run.go`: `runAgent()` — replace the single-line flag read at line ~298:
  ```go
  timeout, _ := cmd.Flags().GetInt("timeout")
  ```
  with a three-step resolution block:
  ```go
  timeout := 120
  if cfg.Timeout > 0 {
      timeout = cfg.Timeout
  }
  if cmd.Flags().Changed("timeout") {
      timeout, _ = cmd.Flags().GetInt("timeout")
  }
  ```
  The `timeout` variable is already used downstream at lines ~323 (dry-run call) and ~326 (context creation) — no further changes to those call sites are needed.

---

### Task 5 — Display `Timeout` in `agents show`

- [x] `cmd/agents.go`: inside the `showCmd` `RunE` closure — add a conditional display block for the new top-level field, placed after the `Workdir` block (around line ~93) and before the `Tools` block:
  ```go
  if cfg.Timeout > 0 {
      _, _ = fmt.Fprintf(w, "%-16s%d\n", "Timeout:", cfg.Timeout)
  }
  ```

---

### Task 6 — Test: `AgentConfig` parsing of new field

- [x] `internal/agent/agent_test.go`: add `TestLoad_TopLevelTimeout` — write a TOML with `timeout = 300`, load it, assert `cfg.Timeout == 300`.

---

### Task 7 — Test: `Validate()` rejects negative timeout

- [x] `internal/agent/agent_test.go`: add `TestValidate_TopLevelTimeoutNegative` — construct `AgentConfig{Name: "x", Model: "p/m", Timeout: -1}`, call `Validate()`, assert error message equals `"timeout must be non-negative"`.

---

### Task 8 — Test: `Validate()` accepts zero and positive timeout

- [x] `internal/agent/agent_test.go`: add `TestValidate_TopLevelTimeoutZeroAndPositive` — table-driven test with `timeout = 0` and `timeout = 300`, assert `Validate()` returns nil for both.

---

### Task 9 — Test: `Scaffold()` contains new commented line

- [x] `internal/agent/agent_test.go`: extend `TestScaffold_IncludesSubAgentsConfig` (or add `TestScaffold_IncludesTopLevelTimeout`) — assert scaffold output contains `"# timeout = 120"` as a top-level line (distinct from the one inside `# [sub_agents_config]`).

---

### Task 10 — Test: TOML timeout used when flag not explicitly set

- [x] `cmd/run_test.go`: add `TestRun_TomlTimeoutUsedWhenFlagAbsent` — set up a slow mock server (sleeps >1s), configure agent TOML with `timeout = 1`, do **not** pass `--timeout` flag, assert the run returns a timeout `ExitError` with code 3. This proves the TOML value was applied.

---

### Task 11 — Test: explicit `--timeout` flag overrides TOML timeout

- [x] `cmd/run_test.go`: add `TestRun_FlagTimeoutOverridesTomlTimeout` — set up a slow mock server (sleeps >1s), configure agent TOML with `timeout = 300`, pass `--timeout 1` explicitly, assert the run returns a timeout `ExitError` with code 3. This proves the flag wins over the TOML value.

---

### Task 12 — Test: `--timeout` flag equal to default still overrides TOML

- [x] `cmd/run_test.go`: add `TestRun_FlagTimeoutDefaultValueOverridesToml` — set up a mock server that responds immediately with a valid LLM response, configure agent TOML with `timeout = 1`, pass `--timeout 120` explicitly, assert the run **succeeds** (no timeout error). This proves that an explicit `--timeout 120` wins over `timeout = 1` in TOML.

---

### Task 13 — Test: `agents show` displays `Timeout` when non-zero

- [x] `cmd/agents_test.go`: add `TestAgentsShow_TopLevelTimeout` — write an agent TOML with `timeout = 300` (no `sub_agents`), run `agents show`, assert output contains `"Timeout:"` and `"300"`.

---

### Task 14 — Test: `agents show` omits `Timeout` when zero

- [x] `cmd/agents_test.go`: add `TestAgentsShow_TopLevelTimeoutOmittedWhenZero` — write an agent TOML with no `timeout` field, run `agents show`, assert output does **not** contain `"Timeout:"` (the top-level one; sub-agents section is absent too).

---

### Task 15 — Verify golden files remain correct

- [x] `cmd/testdata/golden/dry-run/`: confirm all six golden files still show `Timeout:  120s` — no edits needed since the fixture agents in `cmd/testdata/agents/` do not set the new top-level `timeout` field. Run `go test ./cmd/... -run TestGolden` to confirm.

---

*End of implementation guide.*
