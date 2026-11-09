# Axe v1.12.0: runner tool hooks, options, and compatibility gate — Milestones

Outcome: axe `v1.12.0` ships additive `pkg/runner` options that a host program (nightshift milestone 003) needs to run an agent with per-call tool decisions, code-supplied MCP servers, tool list, turn cap, and provider credentials. Every change keeps existing users' behavior. A compatibility gate runs before feature work.

Source: the committed requirements draft `docs/plans/runner-tool-hooks-and-options.md` (change IDs A1–A9, question IDs Q1–Q12, conflict IDs C1–C14). This document carries every decision it depends on. It does not depend on any uncommitted file.

Evidence labels follow the draft: **Existing** items cite `path:line` at HEAD `ce6c096291d7cdd9209f3358a0fdd05b99c78539`. **Proposed** items are new and do not exist in axe.

---

## Section 1: Research Findings

### 1.1 Codebase structure and patterns

- Public library: `pkg/runner`. `Options` is at `pkg/runner/options.go:6-33`; it has no MCP, tool-list, turn, or credential field. Keyed literals are used in-repo: `cmd/run.go:80`, `pkg/runner/messages.go:146-149`, `:166-169`.
- Run entry and dry-run early return: `pkg/runner/run.go:279` (`if opts.DryRun`). Dry run returns before MCP connect and before provider construction.
- Timeout context: `pkg/runner/run.go:332`. Provider credentials: `pkg/runner/run.go:336-337` (`ResolveAPIKey`, `ResolveBaseURL`). Bedrock region branch: `pkg/runner/run.go:340-347`.
- Turn limit: `pkg/runner/run.go:33` (`defaultMaxConversationTurns = 50`), `:83-86`, loop at `:587`, turn-limit `RuntimeError` at `:695`.
- Tool dispatch (sequential): `pkg/runner/run.go:927-938`. MCP branch first (`:931`), `call_agent` (`:933`), builtin registry (`:936`). Parallel branch: goroutine at `:952`, builtin dispatch at `:960`.
- Shared call point: `executeToolCalls` at `pkg/runner/run.go:890`. MCP dispatch: `dispatchMCPToolCall` at `pkg/runner/run.go:979`.
- `call_agent` injection when `sub_agents` is set: `pkg/runner/run.go:423`. MCP connect loop: `:428`; verbose connect line: `:439` (prints URL only).
- Connect-error mapping to `ConfigError` by text match: `pkg/runner/run.go:443` (`"environment variable"`, `"unsupported MCP transport"`).
- Output truncation: `pkg/runner/run.go:992` (`truncateOutput`). Fatal provider error mapping: `pkg/runner/run.go:1019`.
- Public message types: `pkg/runner/messages.go:98-101` (`ToolCall`, `Arguments map[string]string`); copy paths `:136`, `:146`, `:166`.
- JSON envelope: `pkg/runner/result.go:6-8` (`ToolCallDetail.Input map[string]string`); `serializeMessages` at `:81`; `Result.MarshalJSON` at `:117`.
- Error helpers: `pkg/runner/errors.go:57` (`IsConfigError`), `:63`, `:69` use type assertion. `:77` (`ProviderCategory`) and `:86` (`AsProviderError`) expose `internal/provider` types.
- Sub-agents use a separate dispatch path: `internal/tool/tool.go:29` (`ExecuteOptions`), `:420` (MCP-first loop), `:455` (`dispatchToolCall`). Sub-agent credentials: `internal/tool/tool.go:224-225`.
- Agent TOML: `internal/agent/agent.go:77-79` (`Temperature`, `TemperatureSet`), `:114` (`MCPServerConfig`), `:131` (`MaxTurns`), `:135` (`MCPServers`), `:167` (negative `max_turns` check), `:183` (MCP name uniqueness), `:201` (transport check), `:268` (`TemperatureSet` detection).
- Environment and config: `internal/envinterp/envinterp.go:9` (regex), `:12` (`ExpandHeaders`), `:47` (unset-variable error text). `internal/config/config.go:30` (`Load`), `:38` (missing file tolerated), `:81` (`ResolveAPIKey`), `:99` (`ResolveBaseURL`).
- MCP client: `internal/mcpclient/mcpclient.go:26` (header expansion in `Connect`), `:43` (hardcoded `Version: "1.11.0"`), `:72` (`ListTools`), `:94` (`CallTool`), `:124` (`coerceArg`), `:140` (end of `coerceArg`).
- MCP router: `internal/mcpclient/router.go:22` (`Register`), `:26-28` (builtin name wins, MCP tool skipped), `:30-32` (cross-server collision is an error), `:34` (raw name registered), `:45` (`unknown MCP tool`).
- Provider argument flattening: `internal/provider/openai.go:338-344` (`args[k] = fmt.Sprintf("%v", v)` path); `internal/provider/anthropic.go:331` (same). Other providers use the same pattern (`bedrock.go:280`, `gemini.go:351`, `ollama.go:303`, `opencode.go:308`, `openrouter.go:237`, per the draft §3 A3).
- Retry: `internal/provider/retry.go:52` (zero retries pass through), `:135` (`isRetriable`).
- Release and version: `cmd/root.go:12` (`const Version = "1.11.0"`), `cmd/version_test.go:19` (asserted string), `CHANGELOG.md:8` (`## [Unreleased]`), `:10-11` (Unreleased `### Added` with the `provider-key-check` entry), `:13` (`## [1.11.0] - 2026-10-21`), `:24` (1.11.0 `temperature` behavior change).
- Build and CI: `Makefile:9` (`check: lint test`), `.github/workflows/go.yml:19`, `:32`, `:47` (`go-version: '1.24'`), `.github/workflows/smoke-test.yml:19`, `:70`, `:105`, `:164`, `:212` (`go-version: '1.24'`). `go.mod:3` (`go 1.25.0`). `.golangci.yml` uses the `standard` linter set (staticcheck included, so SA1019 flags deprecated uses). Release tag trigger: `.github/workflows/release.yml:5-8` (`v*`).
- Repository documents: `AGENTS.md:165` (tool errors are tool results, not fatal), `AGENTS.md:199` (namespacing claim, incorrect; see milestone 011). `docs/skills/release/SKILL.md` is the release procedure. It names three version strings: `cmd/root.go`, `internal/mcpclient/mcpclient.go`, `cmd/version_test.go`.
- Milestone file convention in this repo: `docs/plans/NNN_topic_<kind>.md`, sequential three-digit IDs. Highest existing ID is `051` (`docs/plans/051_runner_message_history_spec.md` and `_implement.md`). This document takes `052`. No `*_milestones.md` file uses the `052` prefix.
- Existence checks: every path cited above was verified at HEAD with `git cat-file -e HEAD:<path>`. `git log --diff-filter=D` on `AGENTS.md`, `CHANGELOG.md`, `Makefile`, and `pkg/runner` shows no deletions. `COMPATIBILITY.md` does not exist at HEAD; it is a milestone 001 deliverable, not an existing artifact.

### 1.2 Problems the release solves

- **No per-call decision point.** The MCP-only dispatch point (`pkg/runner/run.go:979`) cannot see builtin tools (`:936`, `:960`) or `call_agent` (`:933`). A host cannot deny or stub a write tool before it runs. (Draft §1.2, §3 A1.)
- **No code-supplied MCP servers, tool list, turn cap, or credentials.** `Options` has none of them (`pkg/runner/options.go:6-33`). A host must write TOML files and set process-wide environment variables, and `t.Setenv` blocks parallel tests. (Draft §1.2, §3 A4–A7.)
- **Host cannot see what happened per call.** `ToolCallDetails` exists only in JSON mode (`pkg/runner/run.go:664`) and is truncated (`:992`). (Draft §3 A2.)
- **Large integers reach MCP servers as strings.** Providers flatten arguments with `%v` (`internal/provider/openai.go:338-344`, `anthropic.go:331`). `coerceArg` (`internal/mcpclient/mcpclient.go:124`) cannot parse `"1e+06"`, so an `integer` argument of 1000000 becomes a string on the wire. Research confirmed it with Go 1.25.0 `json.Unmarshal` and `%v`: `1000000` → `1e+06`, `1234567` → `1.234567e+06`, `-1000000` → `-1e+06`, `9007199254740993` → `9.007199254740992e+15`. Integers below 1e6 are correct. (Asset: mcp-argument-fidelity research, Fidelity losses table.)
- **Compatibility is claimed, not enforced.** Axe claims SemVer (`CHANGELOG.md:6`). There is no compatibility gate in `Makefile:9` or CI, and no `// Deprecated:` marker in the code. The 1.11.0 `temperature = 0` change shipped in a minor release with no opt-out (`CHANGELOG.md:24`). (Asset: Go team compatibility practices, Gap list.)
- **CI builds with an older toolchain than the module declares.** CI uses `'1.24'` (`go.yml:19,32,47`; `smoke-test.yml` at five lines); `go.mod:3` says `go 1.25.0`.
- **AGENTS.md misstates MCP namespacing** (`AGENTS.md:199`). Code registers raw names (`internal/mcpclient/router.go:34`); the draft calls this C13.

### 1.3 Key decisions and rationale

Each decision carries its compatibility impact on each public surface: `pkg/runner` exported API (API), CLI flags and exit codes (CLI), agent TOML (TOML), `--json` output (JSON), and default runtime behavior (RT). Decisions are accepted by Jaron.

- **D1. Compatibility policy (Go 1 style, all v1 releases).** API: no exported identifier removed or changed; additions allowed; callers use keyed struct literals and do not compare exported structs with `==`. CLI, TOML, `--json`: additive only. RT: unchanged except documented bug fixes. Rationale: Go 1 compatibility and "Keeping Your Modules Compatible" (go.dev/doc/go1compat; go.dev/blog/module-compatibility). Deprecate, never remove, in v1; removal waits for `/v2`.
- **D2. Behavior-change gate.** New behavior is opt-in through a new TOML key or `Options` field. The zero value keeps today's behavior. No GODEBUG-style switch now. The 1.11.0 `temperature` change is recorded in the policy as a pre-policy exception and is not reverted.
- **D3. Enforcement.** A pinned `apidiff` check of `pkg/runner` against `v1.11.0` runs in `make check` and CI. It runs as `go run golang.org/x/exp/cmd/apidiff@<pinned version>`. It is not added to `go.mod`, so it stays out of users' module graphs. The existing golden tests keep guarding CLI and `--json`.
- **D4. Policy location.** Root `COMPATIBILITY.md`, linked from `README.md`, `AGENTS.md`, and the `CHANGELOG.md` header.
- **D5. CI Go version.** Read from `go.mod` with `go-version-file: go.mod` in every setup-go step (`go.yml` and `smoke-test.yml`). This replaces the hardcoded `'1.24'`.
- **D6. Tool hook (A1), accepted as proposed with these changes.**
  - Allow, deny, or stub only. No rewrite action (Q1).
  - `ToolAction` is `type ToolAction string` with values `"allow"`, `"deny"`, `"stub"`. The draft's `int` is replaced (API: new identifiers only).
  - A denied call returns an error result (`IsError: true`); the run continues. A stub returns a normal result. Neither is dispatched. `Result.ToolCalls` still counts denied and stubbed calls (Q2).
  - The hook decides every call of a turn, in model order, on the run goroutine, before any dispatch. Allowed calls then use the existing sequential or parallel path (Q12). With a nil hook the decision pass is skipped.
  - Hook error or panic: recovered, treated as deny with fixed text. Raw error text never reaches the model. Any unrecognized action is deny. `Content` is ignored for allow.
  - Context: if `ctx` is cancelled during the decision pass, no call in that turn is dispatched (neither decided nor undecided calls), and the run returns the context error. Cancellation returns the same shape as today's provider cancellation: a `RuntimeError` whose `Unwrap` chain reaches `ctx.Err()` (`pkg/runner/run.go:557` and `:616` via `mapProviderError`; `internal/provider/anthropic.go:278-282`).
  - Sub-agents: `ToolHook != nil` with a non-empty `sub_agents` list returns `ConfigError` right after agent load, before the `DryRun` return (Q3).
  - No `--json` or `Result` field for decisions in v1.12.0. The CLI cannot set a hook. Golden files stay unchanged.
  - `ToolCall` carries no MCP server name (`internal/mcpclient/router.go:34`). A built-in name silently shadows a same-named MCP tool (`router.go:26-28`). Per-server filtering stays out (Q5). A future additive `Server` field is the path.
- **D7. Tool observer (A2 replaced), accepted.** Ship `ToolObserver` in v1.12.0, metadata only: call, action, `CallID`, `IsError`, content length, duration, turn. No result text. It runs on the run goroutine after a turn's tool calls finish, in model order. No cross-run ordering guarantee. A panic is recovered and reported as one credential-free stderr line. This overrides the draft's "do not recover". The observer returns nothing. When no `ToolHook` is set, `Action` is `ToolAllow` for every event. Fail-closed logic belongs in the hook. No `call_id`/`action` in `ToolCallDetail` or `--json` in v1.12.0. Nightshift 003 records planned actions in its own hook closure (Q6).
- **D8. `Options.MCPServers` (A4), accepted.** Merged with TOML servers. A duplicate name (across both sources) is `ConfigError`. One shared validator runs before `DryRun` returns, with the same rules as `agent.go:183-206`. Internal sentinel `ErrUnsetEnv` in `internal/envinterp`, wrapped with `%w`; the runner maps it with `errors.Is`. The text match at `run.go:443` stays as a fallback for one release. Dry run shows merged server names and URLs, never headers. Q8: nightshift uses A4, not generated TOML.
- **D9. `Options.Tools` (A5), accepted.** `nil` uses the TOML list. Empty non-nil means no built-in tools, the same as TOML `tools = []` today; a test verifies this. Unknown names are `ConfigError`. The docs state that `Tools` does not limit MCP tools or `call_agent`.
- **D10. `Options.MaxTurns` (A6), accepted.** `> 0` overrides TOML; `0` keeps TOML (default 50); negative is `ConfigError`.
- **D11. `Options.APIKey` and `Options.BaseURL` (A7, Q7), accepted.** Applied to the run's own provider. Precedence: Options, then env var, then `config.toml` (`internal/config/config.go:81`, `:99`). Never logged. Sub-agents still resolve credentials from env and config (`internal/tool/tool.go:224-225`); plumbing is deferred. A missing `config.toml` stays tolerated (`config.go:38`). Provider name still comes from `Model`.
- **D12. Large-integer coercion fix (C-defect from research), fixed as its own task in v1.12.0.** It is not A3. It is recorded under **Fixed** in the changelog.
- **D13. AGENTS.md:199 namespacing (C13), corrected in v1.12.0.** New text must say: MCP tools keep raw names (`internal/mcpclient/router.go:34`); a built-in name wins (`router.go:26-28`); a cross-server name collision is an error (`router.go:30-32`).
- **D14. No axe test helper for nightshift.** `internal/testutil` is not importable by nightshift. A stubbed call is decided before dispatch and never reaches an MCP server. Nightshift proves the stub path with its own fake LLM server.
- **D15. Version `v1.12.0`** (minor; all accepted changes are additive). Jaron pushes the tag. Release follows `docs/skills/release/SKILL.md`.
- **D16. Release notes.** `gh release edit` notes are authoritative over the workflow awk extraction, per the release skill.

### 1.4 Rejected approaches and reasons

- **Rewrite action in the hook (Q1).** Rejected for v1. Makes the audit record unclear.
- **Abort the run on denial (Q2).** Rejected. The model can adapt, `max_turns` bounds the loop, and abort hides the reason.
- **Hook propagation to sub-agents (Q3).** Deferred. Fail closed with `ConfigError` instead.
- **Observer returning result text (draft A2 as written).** Rejected by D7. Nightshift's audit rule forbids raw tool outputs (nightshift ticket `oncall-offload-v1-t14`, Q1: metadata plus allowlisted inputs only).
- **Typed MCP arguments, A3 (`RawArguments json.RawMessage`).** Out of v1.12.0. Research shows it is not enough alone: MCP dispatch would also need to decode from raw bytes with `UseNumber`. The nightshift fields needed (note text, status, assignee) are believed to be top-level strings, so `Arguments` suffices. Vendor `tools/list` schemas were not captured (draft Q4 remains open; see 1.6).
- **Per-server MCP tool filter (Q5).** Rejected. Vendor-side limits and the hook cover it.
- **Generated TOML instead of `Options.MCPServers` (Q8).** Rejected as the primary path; it remains the workaround if A4 slips.
- **Axe test helper for nightshift (`internal/testutil`).** Rejected (D14).
- **apidiff in `go.mod`.** Rejected (D3). It would leak into users' module graphs.
- **Reverting the 1.11.0 `temperature` change.** Rejected (D2). It is recorded as an exception.
- **Retry changes (draft A9, `1009` classification, `MaxRetries`, `RetryClassifier`).** Out of scope (milestone 008 of nightshift). Nightshift can use `max_retries = 0` in TOML today.
- **Typed provider-error classification (draft A8, `ProviderErrorInfoOf`).** Out of scope (nightshift milestone 008). `ProviderCategory` and `AsProviderError` are not deprecated in v1.12.0.
- **Chain-aware `Is*` helpers (draft C-item from the Go-practices research, O7).** Out of scope; deferred. Nightshift 003 does not need it.
- **New CLI flags for any of the above.** Rejected. The CLI surface is unchanged.
- **Bedrock endpoint override via `Options.BaseURL` (1.5).** Out of scope for v1.12.0. `Options.BaseURL` is ignored for `bedrock`.
- **`--json` / `ToolCallDetail` changes for decisions or observer events.** Rejected for v1.12.0 (D6, D7).

### 1.5 Constraints and assumptions

- **Constraint, compatibility.** Every change is additive and keyed. The zero value of each new field keeps today's behavior. Existing identifiers keep their names and signatures.
- **Constraint, library scope.** `pkg/runner` is importable by external modules; `internal/` packages are not. Any public type must not use an `internal/` type in a new signature (`pkg/runner/errors.go:77`, `:86` are existing exceptions and are not changed).
- **Constraint, toolchain.** `go.mod` declares `go 1.25.0`. The `go run` pin must build with the toolchain CI resolves from `go.mod`.
- **Constraint, no `.scratch` dependency.** Decisions are restated in this document.
- **Assumption, apidiff pin.** The module cache holds `golang.org/x/exp@v0.0.0-20250620022241-b7579e27df2b` (zip present under `$(go env GOMODCACHE)/cache/download/golang.org/x/exp/@v/`). Building `cmd/apidiff` from it offline failed in this environment, so no pin is verified. Milestone 002 pins the latest `golang.org/x/exp` version at implementation time, with network access, and records it in the `Makefile`.
- **Assumption, `ToolEvent.Call`.** The observer's `call` field is the `ToolCall` as the hook saw it (name, call ID, arguments). Result text is excluded (D7). Nightshift's audit (its t14) allows only allowlisted write inputs, so nightshift must not persist arguments it did not allowlist. This is a consumer concern, not an axe change.
- **Assumption, large-integer fix scope.** The fix changes what the MCP server receives for `integer` arguments of 1e6 and above, across every provider that flattens arguments (1.1 list). It also changes the replay text to the model for affected integers: the model's original digits are preserved, so `1e+06` becomes `1000000` (`internal/provider/openai.go:168-177`, `anthropic.go:160-165`). This is part of the fix and is disclosed in the CHANGELOG Fixed entry (012).
- **Assumption, `ErrUnsetEnv` reach.** The sentinel covers only the env-expansion error in `envinterp.ExpandHeaders` (`internal/envinterp/envinterp.go:47`). Other connect-error text matches stay as they are.
- **Assumption, hook timeout.** The hook runs under the run context (`run.go:332`). Axe adds no per-hook timeout. The host's hook must return quickly.
- **Assumption, dry run and hooks.** `DryRun` is unchanged (`run.go:279`). Hooks run only on normal runs. Stubbing for dry runs goes through `ToolHook` on a normal run, as nightshift uses it.
- **Decision, Bedrock and `Options.APIKey` / `Options.BaseURL`.** `Options.APIKey` and `Options.BaseURL` are ignored when the provider is `bedrock`. Region still comes from `AWS_REGION` / `AWS_DEFAULT_REGION` / `AXE_BEDROCK_REGION` / `config.toml` `providers.bedrock.region` (`internal/config/config.go:117-140`); AWS credentials still come from `internal/provider/awscreds.go:17-44`. The implementation must NOT guard `apiKey = region` with `if apiKey == ""`; that would turn the key into the region/endpoint. Leave `pkg/runner/run.go:340-347`, `internal/provider/registry.go:72-75`, and `cmd/gc.go:132-141` unchanged.
- **Unresolved, vendor schemas (draft Q4).** `tools/list` schemas for Sentry `add_issue_note` and the Better Stack comment tool were not captured. A3 is out of scope because it depends on this. Consumer-side check, not an axe blocker.

### 1.6 Resolved questions and sources

| Question | Answer | Source |
|---|---|---|
| Q1 rewrite action | No, allow/deny/stub only | D6 |
| Q2 denial shape | Error result, run continues | D6 |
| Q3 sub-agents with a hook | `ConfigError` in v1.12.0 | D6 |
| Q4 argument fidelity | A3 out of v1.12.0; vendor schemas remain open | 1.4, 1.5 |
| Q5 per-server MCP filter | None | D6, 1.4 |
| Q6 observer or closure for nightshift 003 | Closure for 003; metadata observer ships in v1.12.0 | D7 |
| Q7 provider seam | Include A7 | D11 |
| Q8 MCP config path | `Options.MCPServers` (A4) | D8 |
| Q9 Gloo 1009 body sample | Out of scope (nightshift 008) | 1.4 |
| Q10 version | `v1.12.0` | D15 |
| Q11 option names | Accepted: `ToolHook`, `ToolObserver`, `MCPServers`, `Tools`, `MaxTurns`, `APIKey`, `BaseURL`; `ToolAction`, `ToolAllow`, `ToolDeny`, `ToolStub`, `ToolDecision` | D6–D11 |
| Q12 parallel dispatch | Decide every call first, in model order | D6 |
| C1 `ProviderCategory` usable externally | Not changed here; A8 out of scope | 1.4 |
| C2 env-only credentials | Works when `config.toml` is missing (`internal/config/config.go:38`); the run still needs an agent TOML or `Options.AgentsDirs` | D11, 1.5 |
| C3 MCP config path priority | A4 is strongly recommended, TOML is the workaround | D8 |
| C13 AGENTS.md namespacing | Corrected | D13 |
| C14 CI Go version | Read from `go.mod` | D5 |

---

## Section 2: Milestones

- [x] 000: Clear the govulncheck findings by pinning toolchain go1.26.10 and upgrading golang.org/x/net to v0.62.0
  - Slice enabled: govulncheck exits 0 on the module, so the release gate starts clean.
  - Depends on: none.
  - Scope: the baseline `~/.agents/scripts/go-toolchain-check ./` run reports 37 govulncheck findings. 36 list the Go standard library. 6 list `golang.org/x/net@v0.55.0`: GO-2026-6860 (reached at `internal/tool/url_fetch.go:85`, fixed in v0.62.0) and GO-2026-6603, 6610, 6611, 6612, 6617 (each also lists `net/http`). Ten standard-library findings have no go1.25 fix: GO-2026-6809 (`crypto/x509`, needs go1.26.10) and GO-2026-6603, 6605, 6607, 6608, 6610, 6611, 6612, 6613, 6617 (`net/http`, `crypto/tls`, `net/textproto`, needs go1.26.9). Changes: set the `go` directive at `go.mod:3` to `go 1.26.0` and add `toolchain go1.26.10` (decision Q4, option O4a: `golang.org/x/net` v0.62.0 declares `go 1.26.0`, so tidy requires the bump); run `go get golang.org/x/net@v0.62.0` and `go mod tidy`, accepting any transitive bumps tidy requires. `go.sum` changes with them. CI: change every `go-version: '1.24'` in `.github/workflows/go.yml` (lines 19, 32, 47) and `.github/workflows/smoke-test.yml` (lines 19, 70, 105, 164, 212) to `go-version-file: go.mod`, and change `.github/workflows/release.yml:24` (`go-version: stable`) to `go-version-file: go.mod` (decision Q5, B2). Milestone 003 verifies and covers only what remains after 000. Docker: change `Dockerfile:2` from `golang:1.25-alpine` to `golang:1.26-alpine` (decision Q6, B3). Docs: change `AGENTS.md:215` (`golang:1.25-alpine`) to `golang:1.26-alpine`, and change `README.md:40` (`Requires Go 1.25+.`) and `README.md:50` (`1.25`) to `1.26`. Also change `CONTRIBUTING.md:19` (`Go 1.25.0 or later`) to `Go 1.26.0 or later`, the same way as the README lines. Also change `docs/src/getting-started/installation.md:3` (`Requires **Go 1.25+**.`) to `Requires **Go 1.26+**.` and `docs/src/getting-started/installation.md:32` (`older than 1.25`) to `older than 1.26`. No other line in those files changes.
  - Compatibility impact: no public API change. The `go` directive moves to `1.26.0` (Q4, O4a), so the minimum Go version for importers of `pkg/runner` rises from 1.25.0 to 1.26.0. This is a pre-policy change, recorded in `COMPATIBILITY.md` (milestone 001). Reason: the security fix GO-2026-6860 needs x/net v0.62.0, and Go supports only its two newest releases. Go ignores a dependency's `toolchain` line. Builds inside this module and CI use go1.26.10.
  - Acceptance: `~/.agents/scripts/go-toolchain-check ./` exits 0 with no govulncheck findings. `go build ./...`, `go test -race ./...` (except the two environmental `cmd` cache tests, which fail the same way at baseline in the sandbox and need a writable `~/.cache/axe`), and `golangci-lint run` pass. `go.mod` contains `go 1.26.0` and `toolchain go1.26.10`. No `'1.24'` remains in `.github/workflows/go.yml` or `.github/workflows/smoke-test.yml`, and every setup-go step in them uses `go-version-file: go.mod`. `Dockerfile:2` reads `golang:1.26-alpine`. Host step, verified by the orchestrator through Jaron (not a subagent, the sandbox masks the Docker socket): `docker build .` exits 0 on a host with Docker.

- [x] 001: Publish the compatibility policy in `COMPATIBILITY.md` and link it from the README, AGENTS.md, and the CHANGELOG header
  - Slice enabled: every later feature milestone states its compatibility impact against a written policy.
  - Depends on: none.
  - Scope: new root file `COMPATIBILITY.md`. It records D1 (Go 1 style promise for all five surfaces: `pkg/runner` API, CLI flags and exit codes, agent TOML keys, `--json` fields, default runtime behavior), D2 (behavior changes are opt-in; zero value keeps behavior), D3 (the apidiff gate, its scope, and its pin process), D4 (deprecate, never remove, in v1; removal waits for `/v2`), the 1.11.0 `temperature` change as a pre-policy exception (`CHANGELOG.md:24`), and the minimum-Go raise from 1.25.0 to 1.26.0 made by milestone 000 as a second pre-policy exception next to it. Add links from `README.md`, `AGENTS.md`, and the `CHANGELOG.md` header (`CHANGELOG.md:1-6`).
  - Compatibility impact: none. Documentation only.
  - Acceptance: `COMPATIBILITY.md` exists at the repo root. The five surfaces and the exception are each stated. Links from README, AGENTS.md, and CHANGELOG resolve to it. `docs/plans/runner-tool-hooks-and-options.md` is not edited here.

- [ ] 002: Enforce the `pkg/runner` compatibility gate with a pinned apidiff in `make check` and CI
  - Slice enabled: an incompatible `pkg/runner` change fails locally and in CI before any feature merges.
  - Depends on: 001 (the policy the gate enforces).
  - Scope: a `Makefile` target that runs apidiff against `v1.11.0` for `pkg/runner`, wired into `check` (`Makefile:9`). A CI step in `.github/workflows/go.yml` runs the same target. The tool is invoked as `go run golang.org/x/exp/cmd/apidiff@<pinned version>`. It is not added to `go.mod`. The pin is the latest `golang.org/x/exp` version available at implementation time, verified with network access, and recorded in the `Makefile`. The base is `v1.11.0`; it is not moved until a later release. Workflow: (1) CI checkout uses `fetch-depth: 0` (or an explicit `git fetch --tags`) so tag `v1.11.0` exists. (2) Create a temporary worktree of the base (`git worktree add <tmp>/base v1.11.0`). In it, export the old API with apidiff's `-w` flag: `go run golang.org/x/exp/cmd/apidiff@<pin> -w <tmp>/old.export github.com/jrswab/axe/pkg/runner`. (3) From the current tree, compare: `go run golang.org/x/exp/cmd/apidiff@<pin> <tmp>/old.export github.com/jrswab/axe/pkg/runner`. The old operand is the `<tmp>/old.export` file; the new operand is the current `pkg/runner` import path. Failure: the report lists any incompatible change. The implementation step verifies apidiff's exit status and output format with network access and records the pin.
  - Compatibility impact: none on users. CI and `make check` gain one step.
  - Acceptance: `make check` and the CI job fail when `pkg/runner` has an incompatible change against `v1.11.0`, and pass on the unmodified tree. `go.mod` and `go.sum` are unchanged. The pin is recorded and is a concrete version, not `latest`. A test change that removes an exported identifier in a scratch copy makes the gate fail.

- [ ] 003: Read the CI Go version from `go.mod` in all workflows
  - Slice enabled: CI builds and tests on the toolchain the module declares (`go.mod:3`, `go 1.26.0`, with `toolchain go1.26.10`).
  - Depends on: none technically; ordered before features so later gates run on the declared toolchain.
  - Scope: milestone 000 (decision Q5, B2) already changes every setup-go step to `go-version-file: go.mod`: `.github/workflows/go.yml` (lines 19, 32, 47), `.github/workflows/smoke-test.yml` (lines 19, 70, 105, 164, 212), and `.github/workflows/release.yml:24` (`stable`). After 000 this milestone verifies and covers only the remainder: any other workflow or version read that 000 did not change. At `be94237` the remainder is none, so 003 is a verification step. The `go` directive stays `1.26.0` (decision Q4, O4a); `go.mod` carries `toolchain go1.26.10`, which `GOTOOLCHAIN=auto` honors at build time.
  - Compatibility impact: none on users. CI only.
  - Acceptance: `rg -n "go-version:" .github/workflows/` shows no `'1.24'` literal and no setup-go step without `go-version-file: go.mod`. Every workflow YAML parses. The CI run resolves Go 1.26.x and runs with `toolchain go1.26.10` from `go.mod`.

- [ ] 004: Let a host deny or stub any tool call before dispatch with `Options.ToolHook`
  - Slice enabled: nightshift's 003 policy can reject a write tool or stub it in dry run, for builtin, MCP, and `call_agent` calls, through one closure.
  - Depends on: 002 (the gate runs on this change).
  - Scope: new file `pkg/runner/hook.go`: `type ToolAction string` with `ToolAllow`, `ToolDeny`, `ToolStub`; `type ToolDecision struct{Action ToolAction; Content string}`; `type ToolHook func(ctx context.Context, call ToolCall) (ToolDecision, error)`. Add `ToolHook ToolHook` to `Options` (`pkg/runner/options.go:6-33`). Wire the decision pass into `executeToolCalls` (`pkg/runner/run.go:890`), before the sequential branch (`:927`) and the parallel branch (`:952`). Reject `ToolHook != nil` with a non-empty `sub_agents` list with `ConfigError` right after agent load, before the `DryRun` return (`run.go:279`).
  - Semantics: D6 in full (model-order decision pass on the run goroutine; deny is an error result; stub is a normal result; hook error or panic is a fixed-text deny; unknown action is deny; cancellation as in D6 (no call of the turn dispatched; context error returned); nil hook skips the pass).
  - Compatibility impact: API adds identifiers and one field. CLI, TOML, `--json`: unchanged. RT: unchanged when `ToolHook` is nil.
  - Acceptance tests (in `pkg/runner/run_test.go` using the `run()` seam and the capturing-provider pattern, and in an MCP integration test):
    1. No hook: output and tool results match the pre-change baseline.
    2. Deny a builtin write tool (`write_file`): the tool does not run; the model receives `IsError: true`.
    3. Deny an MCP tool: the MCP server records zero calls for it.
    4. Stub an MCP tool: zero MCP calls; the model receives the stub text with `IsError: false`.
    5. Allow a read MCP tool: the call reaches the MCP server.
    6. Hook receives `call.Arguments` exactly as the provider parsed them.
    7. Parallel turn with three calls: the hook runs three times in model order, before any dispatch.
    8. Hook returns an error: the call is denied, and the raw error text is absent from the tool result.
    9. Hook panics: the call is denied and the run completes.
    10. `ToolHook` with `sub_agents` configured: `Run` returns `ConfigError` before any provider call.
    11. The focused parallel-hook test `go test -race -run TestToolHookParallelTurn ./pkg/runner` passes. The full `go test -race ./...` run is Factory's completion gate, not this milestone's check.
    12. Hook cancels `ctx` while deciding call 1 of 2: no call executes (neither call is dispatched); the run returns an error whose `Unwrap` chain reaches the context error (`errors.Is(err, context.Canceled)`).
  - Doc comments state the decide-first order, the fixed-text deny, and the difference from existing tool-error behavior.

- [ ] 005: Give hosts a metadata-only `ToolObserver` for each tool call
  - Slice enabled: a host records what happened per call (decision, outcome, duration, turn) without receiving result text, for nightshift milestone 004.
  - Depends on: 004 (the decision value is the hook's action).
  - Scope: new public types in `pkg/runner/hook.go`: `ToolEvent` with `Call ToolCall`, `Action ToolAction`, `CallID`, `IsError`, `ContentLength`, `Duration`, `Turn`. `type ToolObserver func(ctx context.Context, ev ToolEvent)`. Add `ToolObserver ToolObserver` to `Options`. Call it on the run goroutine after a turn's tool calls finish, in model order, for every call including denied and stubbed ones. When `ToolHook` is nil, `Action` is `ToolAllow` for every event. Recover a panic from the observer and write one credential-free line to stderr.
  - Compatibility impact: API adds identifiers and one field. JSON, CLI, TOML: unchanged. RT: unchanged when nil. `ToolCallDetail` is not changed.
  - Acceptance tests:
    1. Observer called once per call, in model order, for a sequential turn and a parallel turn.
    2. A 4 KB tool output produces an event with `ContentLength` equal to the full length and no result text in any field.
    3. Observer sees `ToolDeny` and `ToolStub` events with the right `Action`.
    4. Observer panic: the run completes; exactly one stderr line; the line contains no argument or header text.
    5. Observer set and `JSON` false: `Result.ToolCallDetails` is `nil` (as `docs/plans/runner-tool-hooks-and-options.md:261`).
    6. The `--json` golden output is unchanged.
    7. Observer set, no hook: every event has `Action` `ToolAllow`.

- [ ] 006: Pass MCP servers from code with `Options.MCPServers`
  - Slice enabled: a host connects to MCP servers, including a Bearer header from an env var, with no TOML file and no `t.Setenv`.
  - Depends on: 002.
  - Scope: new exported `type MCPServer struct{Name, URL, Transport string; Headers map[string]string}` in `pkg/runner`, with the same fields as `agent.MCPServerConfig` (`internal/agent/agent.go:114`). Add `MCPServers []MCPServer` to `Options`. Merge with `cfg.MCPServers` (`agent.go:135`). Validate both sources with one shared validator using the rules at `internal/agent/agent.go:183-206`, before `DryRun` returns. Add sentinel `ErrUnsetEnv` in `internal/envinterp/envinterp.go` (the error at `:47`), wrapped with `%w`. The runner maps it with `errors.Is`. Keep the text match at `pkg/runner/run.go:443` for one release. Dry run shows merged names and URLs, never headers (`run.go:296-300`).
  - Compatibility impact: API adds one type and one field. The sentinel `ErrUnsetEnv` is internal (`internal/envinterp`) and adds no public error API. TOML and CLI unchanged. RT unchanged when the field is nil.
  - Acceptance tests:
    1. Server only in `Options.MCPServers`: its tools appear in the request.
    2. Server in both sources: both sets appear; a duplicate name is `ConfigError`.
    3. Invalid transport in `Options.MCPServers`: `ConfigError` before connect.
    4. Header `Bearer ${VAR}` with `VAR` unset: `ConfigError`, detected with `errors.Is(err, envinterp.ErrUnsetEnv)`.
    5. The HTTP test server receives the expanded header value.
    6. `DryRunInfo.MCPServers` includes the code-supplied server and no header text.
    7. TOML-only tests pass unchanged.

- [ ] 007: Limit built-in tools from code with `Options.Tools`
  - Slice enabled: a host sets the built-in tool list per run without a generated TOML file.
  - Depends on: 006 (MCP tools remain unaffected by this list).
  - Scope: add `Tools []string` to `Options`. `nil` uses the TOML list (`pkg/runner/run.go:413-419`). Validate names with the same rule as `internal/agent/agent.go:176-180`; unknown names are `ConfigError`. Document that the list does not limit MCP tools or `call_agent` (`run.go:423`).
  - Compatibility impact: API adds one field. TOML, CLI, `--json`: unchanged. RT unchanged when nil.
  - Acceptance tests:
    1. A non-nil list replaces the TOML list; the request carries only those tools.
    2. Unknown name: `ConfigError`.
    3. Empty non-nil slice: the request has no built-in tools; this matches TOML `tools = []` today.
    4. `nil` with TOML tools: the request carries the TOML tools (unchanged).

- [ ] 008: Override the turn cap per run with `Options.MaxTurns`
  - Slice enabled: a host sets a per-run turn cap (nightshift triage uses 30) without a generated TOML file.
  - Depends on: none.
  - Scope: add `MaxTurns int` to `Options`. `> 0` overrides the TOML value; `0` keeps it. The loop bound at `pkg/runner/run.go:587` and default at `run.go:83-86` are unchanged. A negative value is `ConfigError`, as at `internal/agent/agent.go:167`.
  - Compatibility impact: API adds one field. TOML, CLI, `--json`: unchanged. RT unchanged at zero.
  - Acceptance tests:
    1. `MaxTurns: 2` stops the loop after two turns with the existing `RuntimeError` (`run.go:695`).
    2. `MaxTurns: 0` uses the agent value; with both unset, the default is 50.
    3. Negative value: `ConfigError`.

- [ ] 009: Set the provider API key and base URL from code with `Options.APIKey` and `Options.BaseURL`
  - Slice enabled: a host runs against a test LLM server with a fake key and no environment changes, and tests can run in parallel.
  - Depends on: none.
  - Scope: add `APIKey string` and `BaseURL string` to `Options`. Precedence: Options, then env var, then `config.toml` (`internal/config/config.go:81`, `:99`). Apply them at `pkg/runner/run.go:336-337`. Sub-agent credentials are unchanged (`internal/tool/tool.go:224-225`). Never log either value. Bedrock: `Options.APIKey` and `Options.BaseURL` are ignored when the provider is `bedrock`. Region still comes from `AWS_REGION` / `AWS_DEFAULT_REGION` / `AXE_BEDROCK_REGION` / `config.toml` `providers.bedrock.region` (`internal/config/config.go:117-140`); AWS credentials still come from `internal/provider/awscreds.go:17-44`. The implementation must NOT guard `apiKey = region` with `if apiKey == ""`; that would turn the key into the region/endpoint. Doc comments on both fields say "Ignored when the provider is bedrock." Leave `pkg/runner/run.go:340-347`, `internal/provider/registry.go:72-75`, and `cmd/gc.go:132-141` unchanged.
  - Compatibility impact: API adds two fields. CLI, TOML, `--json`: unchanged. RT unchanged when both are empty.
  - Acceptance tests:
    1. `BaseURL` and `APIKey` set, no env vars: the request reaches an `httptest` server.
    2. Both set and env vars also set: the Options values are used.
    3. Both unset: current lookup behavior.
    4. A test with `t.Parallel()` that sets both fields and does not call `t.Setenv`.
    5. Logs and verbose output do not contain either value.
    6. Bedrock model with `Options.APIKey="fake"`, `Options.BaseURL="http://127.0.0.1:1"`, and `AWS_REGION` set via `t.Setenv`: the `newProvider` seam (`run.go:47`, `:358`) receives the region as its key argument and `""` as base URL.
    7. Bedrock model with `Options.APIKey` set and no region source: `ConfigError`, exit 2 (`run.go:340-343`).

- [ ] 010: Send integer MCP arguments of 1e6 and above as JSON integers
  - Slice enabled: an MCP tool with an `integer` parameter receives the exact integer the model sent, for every provider.
  - Depends on: none. Ordered after features so feature tests cover the path before this change.
  - Scope: stop providers from flattening integer-valued JSON numbers through `%v` before MCP dispatch. The providers flatten with `fmt.Sprintf("%v")` at `internal/provider/anthropic.go:331`, `bedrock.go:280`, `gemini.go:351` and `:530`, `ollama.go:303` and `:479`, `openai.go:342`, `opencode.go:308`, `:1045` and `:1211`, `openrouter.go:237`. The streaming parser at `pkg/runner/run.go:852` (`drainEventStream`) also flattens with `%v` and is in scope: it must keep the number text the same way (decode with `json.Decoder.UseNumber`). Keep the number text in both paths and keep `Arguments map[string]string` unchanged. `coerceArg` (`internal/mcpclient/mcpclient.go:124-140`) must parse the preserved digits as `int64`. The `Arguments` field type is not changed.
  - Compatibility impact: API: no identifier changes. RT: bug fix. Documented as a fix in the changelog. The value a server receives changes from a string to an integer for integers of 1e6 and above; that is the defect being fixed. Integers below 1e6 are unchanged. Replay text to the model changes for affected integers (1.5).
  - Acceptance tests:
    1. A mock MCP tool with an `integer` parameter receives `1000000`, `-1000000`, `1234567`, and `9007199254740993` as JSON numbers with the same digits.
    2. Each provider parser that flattens arguments has a test asserting the digits survive for an integer of 1e6 and above.
    3. Integers below 1e6, booleans, strings, and `number` parameters are unchanged (existing tests pass).
    4. `Arguments` values in `pkg/runner` results are unchanged for integers below 1e6.
    5. Replayed assistant tool-call arguments for an integer of 1e6 and above carry the original digits (`internal/provider/openai.go:168-177`, `anthropic.go:160-165`): the follow-up request contains `1000000`, not `1e+06`.
    6. Integers at or above 1e6 keep the model's original digit text in `Arguments` and in returned message histories for every provider decode path, not only the OpenAI and Anthropic replay serialization: `internal/provider/anthropic.go:331`, `bedrock.go:280`, `gemini.go:351` and `:530`, `ollama.go:303` and `:479`, `openai.go:342`, `opencode.go:308`, `:1045` and `:1211`, `openrouter.go:237`, and the streaming path `pkg/runner/run.go:852`. Each listed decode path has a test asserting the digits for both buffered and streaming responses.

- [ ] 011: Correct the MCP namespacing claim in AGENTS.md
  - Slice enabled: contributors and agents read the true MCP name rules before changing the router.
  - Depends on: none.
  - Scope: replace the text at `AGENTS.md:199` with the D13 wording: MCP tools keep raw names (`internal/mcpclient/router.go:34`); a built-in name wins (`router.go:26-28`); a cross-server name collision is an error (`router.go:30-32`). No other AGENTS.md text changes.
  - Compatibility impact: none. Documentation only.
  - Acceptance: `AGENTS.md:199` matches D13 and cites the three router lines; no other line in AGENTS.md changes.

- [ ] 012: Release v1.12.0 with changelog, three version strings, README, and a tag Jaron pushes
  - Slice enabled: a user installs `v1.12.0` and sees every accepted change in the changelog, release notes, and README.
  - Depends on: 000, 001–011.
  - Scope:
    - `CHANGELOG.md`: a `## [1.12.0]` section, dated the day Jaron confirms the tag, above `## [1.11.0]` (`CHANGELOG.md:13`). Keep the Unreleased provider-key-check entry (`CHANGELOG.md:11`) under `### Added`. Added: tool hook, `ToolObserver`, `Options.MCPServers`, `Options.Tools`, `Options.MaxTurns`, `Options.APIKey`, `Options.BaseURL`. Fixed: large-integer MCP argument coercion (010), including replay text to the model for affected integers (`1e+06` becomes `1000000`). Deprecated: none. Add the `[1.12.0]` compare link at the bottom. Add the `COMPATIBILITY.md` link in the header if not already present (001).
    - Version strings: `cmd/root.go:12`, `cmd/version_test.go:19`, and `internal/mcpclient/mcpclient.go:43`. All three read `1.12.0`.
    - `README.md`: document each user-facing change per `docs/skills/release/SKILL.md` step 5, covering `pkg/runner` options and the compatibility policy link. Do not guess flag or field names; read the implementation.
    - Tag: create the annotated tag `v1.12.0` (`git tag -a`) on the release commit only after Jaron confirms the tag message and changelog. Jaron pushes the tag. Set the GitHub release notes from the `[1.12.0]` section with `gh release edit`, per the release skill.
  - Compatibility impact: none beyond the listed changes. This milestone introduces no new behavior.
  - Acceptance:
    1. `go test ./cmd/ -run TestVersion -count=1` passes; `go build .` succeeds.
    2. `make check` passes, including the apidiff gate from 002.
    3. The three version strings match. No other version string changes.
    4. The changelog section lists every item in Added, Fixed, and Deprecated above. No item not listed here appears.
    5. The annotated tag exists only after Jaron confirms. The tag push is performed by Jaron.
