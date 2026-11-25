# Milestone 003: Cache usage without a token budget — implementation checklist

## Context summary

The runner already keeps each Run's provider-reported cache counts and reports them in `--json`, but verbose stderr shows them only when a positive token budget is set. Milestone 003 lets a user inspect those counts with `--verbose` alone, showing only nonzero counts on the existing `Cache:` line, while keeping the budget-path output, JSON, budget accounting, error behavior and Nested Outcome opacity unchanged ([spec](053_corrected_caching_003_spec.md), Context & Constraints). Provider mapping, retries and replay do not change; OpenAI Chat reports cache reads only.

## Execution and evidence rules

- Seam: public `runner.Run` with literal provider-shaped HTTP and SSE fixtures from `internal/testutil` (`NewMockLLMServer`, `MockLLMResponse`) through `AXE_ANTHROPIC_BASE_URL` and `AXE_OPENAI_BASE_URL`, and for Bedrock the existing package-local parity intercept in `pkg/runner/bedrock_parity_test.go` (`newParityFixture`, `parityIntercept`, `parityHTTP`, `parityReply`). Assert on the `Stderr` and `Stdout` buffers and the returned `Result`. Do not add production injection points or change provider packages. `TestRun_VerboseCacheCounts` and its subtests run sequentially with no `t.Parallel()`: `parityIntercept` replaces the process-wide `http.DefaultTransport`, and base URLs are set with `t.Setenv`. The Bedrock row creates its own `newParityFixture` and `parityIntercept` inside its subtest. Stream rows set `Options.Stream = true`; tool-loop rows reuse the `list_directory` tool and the fixture shapes of `TestRun_CacheUsageBoundaries` (`pkg/runner/run_test.go:250-309`).
- T1 creates the behavioral contract before production changes. Record its exact focused command and actual RED output, and show that the failures are the missing budget-free `Cache:` line, not compile or setup failures. Preservation rows live in the same table-driven contract; they are not separate passing-only tests.
- No new public Go, CLI, TOML or JSON interface, dependency, provider mapping, retry, replay or streaming behavior. No paid or live requests, credentials, deployment or release.
- Apply the `AGENTS.md` / `TAGS.md` anchor policy only if a new boundary is introduced; the summary branches are inside `run`, which has no new boundary, so no anchor is expected. No `.mem/` corpus exists.
- **Per task:** run `go test -race` only for touched Go packages, derived with the Factory per-task Go gate; do not run `go test -race ./...` per task. Format with `gofmt` and run `golangci-lint run` on touched packages.
- **Reviews:** T1 is standard risk: one parallel batch of independent read-only Standards and Spec reviews pinned to the fixed point and task commit, with SHA-matched whole-suite gate evidence. T2 is also classed standard, a deliberately stricter choice than lint-only because its user-facing wording carries the no-savings and OpenAI-reads-only qualifications: Standards and Spec reviews with `test evidence: not applicable`, allowed because the reviewed range changes Markdown only.
- T2 depends on T1's final line format. The final milestone review and R4 completion gates follow T2.

## Implementation checklist

### Verbose cache counts without a budget

- [x] **T1 — Budget-free verbose cache line and preservation** — standard; **R1, R2**. Budget: 30 min per incremental cycle.
  - **Files:** production `pkg/runner/run.go: run` (the single-response verbose summary and the tool-loop verbose summary, the `tracker.Max() > 0` else branches only). New test file `pkg/runner/verbose_cache_test.go: TestRun_VerboseCacheCounts`. Leave the positive-budget branches byte-identical.
  - **RED first:** write the table-driven contract with `Verbose: true`, no budget and `JSON: true`, asserting the exact `Cache:` line (or its absence) on stderr, its position after `Tokens:` and before `Stop:`, the unchanged budget-free `Tokens:` line, and that the counts equal `Result.CacheReadTokens`/`CacheWriteTokens` and the JSON `cache_read_tokens`/`cache_write_tokens` (omitted when zero). Rows from spec R1:
    - Anthropic single-response, normal and stream, read and write nonzero → `Cache:    <read> read, <written> written`.
    - Anthropic tool loop, normal and stream, two turns → cumulative read and write totals.
    - Bedrock single-response through the parity intercept, read and write nonzero → both counts.
    - OpenAI Chat single-response, normal (`usage.prompt_tokens_details.cached_tokens`) and stream (final usage chunk), cached read nonzero → `Cache:    <read> read` only.
    - OpenAI Chat tool loop, two turns with cached reads → cumulative read total only.
    - Write only nonzero (Anthropic) → `Cache:    <written> written` only.
    - Both counts zero, normal and stream → no `Cache:` line.
    - Usage absent: Anthropic normal response without cache fields, and an OpenAI stream that ends without its usage chunk and without error → no `Cache:` line and no JSON cache fields.
  - **Preservation rows in the same contract (spec R2):**
    - Positive budget with one nonzero cache count and the other zero → the existing `Tokens:` budget line and `Cache:    <read> read, <written> written` with the zero; `budget: <used>/<max>` and JSON `budget_used_tokens` equal input plus output only.
    - `Verbose: false` with nonzero read and write → no `Cache:` text on stderr.
    - Delegated Run: Main calls `call_agent`; the Nested provider responses report nonzero cache counts and Main's report none → no `Cache:` line on stderr and Main JSON cache fields absent.
    - Anthropic stream with `Verbose: true`, `Stream: true` and `JSON: false` whose SSE sends `message_start`, a `text_delta`, then `event: error` → stdout holds the partial text, the error is returned, and neither `Tokens:` nor `Cache:` appears on stderr.
  - Before the fix, the R1 rows with nonzero counts and no budget must fail for the missing line; the zero, absent and preservation rows are expected to pass. Once the test exists, construct the exact focused `go test -count=1 ./pkg/runner` command selecting only the new test, and record it and its output at RED dispatch.
  - **Fix:** in both budget-free branches, after the `Tokens:` line, print one `Cache:` line listing only nonzero counts in read-then-written order, or nothing when both are zero. Rerun the focused command GREEN, then the unchanged `go test ./pkg/runner -run '^(TestRun_CacheUsageBoundaries|TestResultMarshalJSON_CacheTokens(OmittedWhenZero)?)$'`.

### Documentation and probe instructions

- [ ] **T2 — Verbose cache documentation and optional probe** — standard (Markdown only); **R3**. Budget: 30 min.
  - `README.md` and `docs/src/cli/run-flags.md`: the `--verbose` row says it prints nonzero provider-reported cache counts, with or without a budget. Update `README.md`'s budget paragraph ("With `--verbose`, each turn logs cumulative usage…") only if it would otherwise contradict the new behavior.
  - `README.md` caching section and `docs/src/providers.md` caching section: verbose stderr and `--json` report the same own-Run cumulative counts; counts are provider reports, not proof of hits or savings; OpenAI Chat reports cache reads only.
  - Add optional probe instructions in the same caching sections: run the same agent twice with an identical static prefix and a changed task using `--verbose` or `--json`; the probe makes paid calls with the user's own credentials; results are point-in-time; zero does not prove caching is disabled or unsupported; recorded results must not include secrets.
  - `CHANGELOG.md` Unreleased: one entry for budget-free verbose cache counts.
  - Re-read each target region immediately before editing. Verify with `~/.agents/scripts/docs-lint --since 93627c1de9a4bbd55715e0651aa3f3aa5121f339` exit 0. `mdbook build docs/` is applicable but `mdbook` is not installed locally; report it as not run with that reason unless available.

## Requirement mapping and phase-2 audit

| Requirement | Task | Evidence |
| --- | --- | --- |
| R1 | T1 | RED then GREEN `TestRun_VerboseCacheCounts` |
| R2 | T1 | preservation rows plus unchanged existing tests |
| R3 | T2 | docs review, `docs-lint` |
| R4 | completion gate | gate results recorded in the checkpoint |

- **Changed signatures:** none. `run` keeps its signature; no exported symbol changes, so no caller updates are required (`rg -n 'func run\(' pkg/runner` finds only `pkg/runner/run.go`).
- **Extreme-input probes (read-only, against current code at base `93627c1`):**
  - Both counts zero, no budget: `sed -n '629,637p' pkg/runner/run.go` shows the else branch prints only `Tokens:`; expected after the fix: no `Cache:` line. Matches.
  - Nonzero counts, no budget: same branches print no `Cache:` line today; expected after the fix: the nonzero-only line. This is the RED.
  - Positive budget with zero counts: `sed -n '779,787p' pkg/runner/run.go` prints `Cache:    0 read, 0 written`; expected: unchanged.
  - Very large counts: values are Go `int` printed with `%d`; no truncation or formatting boundary applies.
- No checklist command runs `go install` or sets `GOPROXY=`, so no Go toolchain pre-flight is required.

## Final milestone review and completion gate

After T1–T2, run the final independent milestone review of the full range against R1–R4, with SHA-matched whole-suite evidence. Then run the spec R4 gates at the completion HEAD after milestone sync: focused touched-package race tests; `go test -race ./...` and `golangci-lint run` through the declared gates; `go test ./...` and `make apidiff`; `.agents/scripts/ci-check integrity`, `vet`, `build` and `test`. Name every applicable gate not run with its reason (`mdbook build docs/` if `mdbook` is unavailable; Docker smoke has no declared host gate). No credentialed live probe is run.
