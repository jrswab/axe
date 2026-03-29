---

# 045 — Agent Run Timeout Configuration

## Section 1: Context & Constraints

### Milestone Entry

**Issue #60:** Allow configuring the `run` timeout in agent TOML config, in addition to the `--timeout` CLI flag.

- The `--timeout` flag must still override the TOML value when explicitly provided by the user.
- Setting `sub_agents_config.timeout` currently has no effect on the main agent run timeout — only on sub-agent delegation timeouts.

### Research Findings

#### Codebase Structure

- **`internal/agent/agent.go`** — Defines `AgentConfig` (the TOML struct), `SubAgentsConfig`, and `Validate()`. All top-level agent fields live here.
- **`cmd/run.go`** — Main entry point for `axe run`. Reads the `--timeout` flag (default: 120), creates a `context.WithTimeout` for the entire agent execution, and passes `cfg.SubAgentsConf.Timeout` to sub-agent execution options.
- **`cmd/agents.go`** — `agents show` command; displays agent config fields.
- **`internal/tool/tool.go`** — Sub-agent execution; uses `ExecuteOptions.Timeout` (sourced from `cfg.SubAgentsConf.Timeout`) to create a per-sub-agent timeout context.
- **`cmd/testdata/golden/dry-run/`** — Golden files for dry-run output. All currently show `Timeout:  120s` on line 5.
- **`cmd/testdata/agents/`** — Fixture agent TOML files used in golden tests.

#### Decisions Already Made

- **Top-level `timeout` field** (not reuse of `sub_agents_config.timeout`) is the chosen approach. The run timeout is not sub-agent-specific; it controls the entire agent execution. Reusing `sub_agents_config.timeout` would be semantically incorrect.
- **Resolution order:** `--timeout` flag (when explicitly set by user) > TOML `timeout` field > default (120 seconds). This is consistent with how other flag/TOML overrides work in the codebase (e.g., `--model`, `--workdir`).
- **Cobra's `cmd.Flags().Changed("timeout")`** is the correct mechanism to distinguish "user explicitly passed `--timeout`" from "flag is at its default value of 120". This is the standard cobra pattern; no other mechanism is needed.

#### Approaches Ruled Out

- **Reusing `sub_agents_config.timeout`** for the main run: ruled out — semantically wrong; `sub_agents_config` controls sub-agent delegation only.
- **Removing the `--timeout` flag default**: ruled out — would be a breaking change.
- **Changing `sub_agents_config.timeout` behavior**: ruled out — its existing behavior (sub-agent delegation timeout) is correct and must not change.

#### Constraints

- The `--timeout` flag default remains 120 seconds. Backward compatibility must be preserved.
- `sub_agents_config.timeout` behavior is unchanged.
- The new `timeout` field is top-level in `AgentConfig`, not nested.
- Zero (`0`) means "not set" for the TOML field — the default (120s) applies. Negative values are invalid.
- The `--timeout` flag (when explicitly passed) always wins, regardless of the TOML value.
- All existing tests must continue to pass.
- Golden files must be updated to reflect any output changes.

#### Open Questions Resolved

- **What value means "not configured" in TOML?** Zero (`0`). TOML omits the field or sets it to `0` to mean "use default". This is consistent with other optional integer fields in `AgentConfig` (e.g., `params.max_tokens`, `budget.max_tokens`).
- **What is the default timeout?** 120 seconds — unchanged from the current `--timeout` flag default.
- **Does the new field appear in `agents show` output?** Yes, when non-zero.
- **Does the new field appear in dry-run output?** Yes — the dry-run `Timeout:` line already shows the effective timeout; it will now reflect the TOML value when the flag is not explicitly set.
- **Does the new field appear in verbose stderr output?** Yes — the verbose `Timeout:` line will reflect the effective timeout.
- **Does the scaffold template include the new field?** Yes, as a commented-out example.

---

## Section 2: Requirements

### 2.1 New TOML Field: `timeout`

A new optional integer field `timeout` must be added at the top level of the agent TOML configuration.

- Field name: `timeout`
- Type: integer (seconds)
- Default when absent or zero: 120 seconds (the existing default)
- Valid range: 0 (not set) or any positive integer
- Negative values are invalid and must be rejected at validation time with the error message: `"timeout must be non-negative"`

Example TOML:
```toml
name = "my-agent"
model = "anthropic/claude-sonnet-4-20250514"
timeout = 300
```

### 2.2 Timeout Resolution Order

The effective timeout for the main agent run must be resolved in the following order (highest precedence first):

1. **`--timeout` flag** — only when the user explicitly passes it on the command line (i.e., the flag was changed from its default)
2. **TOML `timeout` field** — when non-zero and the flag was not explicitly set
3. **Default: 120 seconds** — when neither the flag was explicitly set nor the TOML field is non-zero

The `--timeout` flag default value (120) must not be treated as an explicit user override. Only a user-supplied `--timeout` value takes precedence over the TOML field.

### 2.3 Validation

The `timeout` field must be validated as part of the existing `Validate()` function:

- If `timeout < 0`: return a `ValidationError` with message `"timeout must be non-negative"`
- If `timeout == 0`: valid (means "not set"; default applies)
- If `timeout > 0`: valid

### 2.4 Scaffold Template

The scaffold template (used by `axe agents init`) must include the new field as a commented-out example:

```toml
# timeout = 120
```

This line must appear near the top of the template, in the vicinity of other top-level fields (before section headers like `[sub_agents_config]`).

### 2.5 Display: `agents show`

The `agents show` command must display the `timeout` field when it is non-zero:

```
Timeout:        300
```

The field must only be shown when `cfg.Timeout > 0` (i.e., not shown when using the default).

### 2.6 Display: Dry-Run Output

The dry-run output line `Timeout:  <N>s` must reflect the **effective timeout** (after resolution per §2.2). No structural change to the dry-run format is required.

### 2.7 Display: Verbose Stderr

The verbose stderr line `Timeout:  <N>s` must reflect the **effective timeout** (after resolution per §2.2). No structural change to the verbose output format is required.

### 2.8 Golden File Updates

All golden files that contain `Timeout:  120s` must remain correct. Since the fixture agent TOML files do not set the new top-level `timeout` field, the effective timeout for those agents remains 120s and the golden files do not need content changes.

If a new golden file is added for a fixture agent that sets `timeout`, it must show the TOML-configured value.

### 2.9 No Change to Sub-Agent Timeout

`sub_agents_config.timeout` behavior is unchanged. It continues to control only the timeout applied to individual sub-agent delegations, not the main agent run.

### 2.10 Edge Cases

| Scenario | Expected Behavior |
|---|---|
| TOML `timeout` absent or `0`, no `--timeout` flag | Effective timeout = 120s |
| TOML `timeout = 300`, no `--timeout` flag | Effective timeout = 300s |
| TOML `timeout = 300`, `--timeout 60` explicitly passed | Effective timeout = 60s (flag wins) |
| TOML `timeout` absent, `--timeout 60` explicitly passed | Effective timeout = 60s |
| TOML `timeout = -1` | Validation error: `"timeout must be non-negative"` |
| TOML `timeout = 0` | Valid; treated as "not set"; effective timeout = 120s |
| TOML `timeout = 300`, `--timeout 120` explicitly passed | Effective timeout = 120s (flag wins, even though it equals the default) |

### 2.11 Test Coverage

The following test scenarios must be covered:

**`internal/agent/agent_test.go`:**
- Parsing a TOML with `timeout = 300` populates `cfg.Timeout = 300`
- `Validate()` rejects `timeout = -1` with message `"timeout must be non-negative"`
- `Validate()` accepts `timeout = 0`
- `Validate()` accepts `timeout = 300`
- `Scaffold()` output contains `# timeout = 120`

**`cmd/run_test.go`:**
- TOML `timeout = 300` with no `--timeout` flag → effective timeout is 300s (context deadline reflects 300s)
- `--timeout 60` explicitly passed with TOML `timeout = 300` → effective timeout is 60s (flag wins)
- No TOML timeout, no `--timeout` flag → effective timeout is 120s (default)
- `--timeout 120` explicitly passed with TOML `timeout = 300` → effective timeout is 120s (flag wins even when equal to default)

**`cmd/agents_test.go`:**
- `agents show` displays `Timeout:` line when `cfg.Timeout > 0`
- `agents show` does not display `Timeout:` line when `cfg.Timeout == 0`

---

*End of spec.*
