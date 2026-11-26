# Milestone 003: Cache usage without a token budget

## Context & Constraints

### Scope and current behavior

This specification covers milestone 003 alone of [the milestone record](053_corrected_caching_milestones.md), lines 47–49. Milestones 001 and 002 are complete: each Agent Run resolves its own cache choice, and Anthropic and Bedrock requests carry contract-valid static-prefix hints. This document describes intended behavior, not a claim it is implemented. Main is an Agent Run entered through the CLI or public runner; a Nested Run enters through `call_agent` and returns only its Nested Outcome to its parent (`GLOSSARY.md`).

At spec time (base `93627c1`), the runner keeps provider-reported counts for its own Run. A single-response Run copies the response's counts (`pkg/runner/run.go:619-622`); a tool-using Run adds each turn's counts (`pkg/runner/run.go:672-677`). Normal and existing stream paths both reach these totals, because the stream drain copies the final usage event's cache counts into the response (`pkg/runner/run.go:884-970`). `--json` reports the totals as `cache_read_tokens` and `cache_write_tokens`, omitted when zero (`pkg/runner/result.go:120-131`, `docs/src/cli/json-output.md:17-18,29`).

Verbose stderr prints a `Cache:    <read> read, <written> written` line only inside the positive-budget branch, for the single-response summary (`pkg/runner/run.go:629-637`) and the tool-loop summary (`pkg/runner/run.go:779-787`). Without a budget, verbose stderr shows tokens but no cache counts, so a user cannot inspect cache usage in verbose output without configuring a budget. Nested Runs print no token or cache counts on verbose stderr (`internal/tool/tool.go:340-376`). An error prints only a `Duration:` line and returns before the token and cache lines (`pkg/runner/run.go:606-613`). No existing test asserts the verbose `Cache:` line.

Provider mapping is unchanged by this milestone. Anthropic maps cache reads and writes, Bedrock maps cache reads and writes, and OpenAI Chat maps cache reads only. Shared adapters inherit their base provider's mapping (`internal/provider/minimax.go`, `internal/provider/litellm.go` and the other OpenAI-compatible adapters).

### Settled decisions and boundaries

- **OpenAI, Q6/Q7:** keep automatic Chat caching and cache-read reporting. OpenAI cache-write mapping, `prompt_cache_options` and `prompt_cache_breakpoint` are deferred to a separate effort. Do not add pricing, new JSON schema or a Responses migration ([milestone record](053_corrected_caching_milestones.md), Resolved decisions).
- **Recovery, Q5:** add no new automatic cache-disable retry or replay. Preserve existing retry behavior and original errors.
- **Evidence, Q8:** deterministic behavioral tests with literal provider-shaped fixtures and the applicable existing gates suffice. Credentialed live probes are optional, nonblocking and need separate approval; documenting how to run one is not authorization to run it. No paid calls, credentials, deployment or release are authorized by this specification.
- Counts are provider reports, not savings. Neither zero nor nonzero establishes cache eligibility, disabling, write absence, hit guarantees or monetary savings. Do not normalize counts across providers.
- Preserve public Go, CLI, TOML and JSON interfaces, pipeable stdout, diagnostics on stderr, and budget accounting: cache counts never add to budget usage (`COMPATIBILITY.md`, `CONTRIBUTING.md`). Do not aggregate Nested counts into the parent or expose Nested internals.
- Do not add a new verbose cache status line, zero-count line or cache-state wording. The existing positive-budget `Cache:` line stays exactly as it is, including its zero values.
- **Compatibility:** the change adds diagnostic text only to `--verbose` stderr. Verbose stderr is not one of the contract surfaces `COMPATIBILITY.md` lists (Go API, CLI flags and exit codes, TOML keys, `--json` fields). Against `COMPATIBILITY.md` "Default runtime behavior" and "Behavior changes are opt-in": the new line is already opt-in through the existing `--verbose` flag (`Options.Verbose`), and behavior without `--verbose` is unchanged, so no new TOML key or `Options` field is needed. The research finding to keep verbose stderr ([milestone record](053_corrected_caching_milestones.md), Existing seams) is superseded by milestone 003's acceptance for this budget-free cache line only; all other verbose output stays as it is.

## Requirements

### R1 — Verbose cache counts without a budget

When `--verbose` is set and the Run has no positive token budget, the verbose usage summary includes the Run's provider-reported cache counts, if at least one count is nonzero.

- Show only nonzero counts, in this order and wording, on one line using the existing `Cache:` label and alignment: `Cache:    <read> read`, `Cache:    <written> written`, or `Cache:    <read> read, <written> written`. Do not print a zero count, so an OpenAI read is never followed by an invented write count.
- When both counts are zero or absent, print no `Cache:` line.
- A single-response Run shows that response's counts. A tool-using Run shows the cumulative totals across its own turns. Values equal the `--json` `cache_read_tokens` and `cache_write_tokens` values for the same Run.
- The line belongs to the existing verbose usage summary, after the `Tokens:` line and before the `Stop:` line.
- Applies to the normal and existing stream paths alike.

Acceptance: deterministic Run-level tests with literal provider-shaped fixtures and `Verbose: true`, no budget, cover each case below. Each asserts the exact `Cache:` line or its absence, and that the counts equal the JSON result.

| Case | Expected stderr |
| --- | --- |
| Anthropic single-response, normal, read and write nonzero | `Cache:    <read> read, <written> written` |
| Anthropic single-response, stream, read and write nonzero | same as normal |
| Anthropic tool loop, normal and stream, two turns | cumulative read and write totals |
| Bedrock single-response, read and write nonzero | both counts |
| OpenAI Chat single-response, normal and stream, cached read nonzero | `Cache:    <read> read` only |
| OpenAI Chat tool loop, two turns with cached reads | cumulative read total only |
| Write only nonzero (any provider that reports writes) | `Cache:    <written> written` only |
| Both counts zero, normal and stream | no `Cache:` line |
| Usage absent: normal response without cache fields, and a stream that ends without its final usage event and without error | no `Cache:` line and no JSON cache fields |

### R2 — Preservation

- With a positive budget, verbose stderr is unchanged: the `Tokens:` line with budget and the `Cache:    <read> read, <written> written` line, zeros included.
- Without `--verbose`, stderr gains no cache output.
- `--json` field names, types, meanings and zero omission are unchanged. Stdout content is unchanged.
- Budget accounting still counts only input and output tokens.
- A Run that fails with an error prints no `Tokens:` or `Cache:` line and invents no counts. This includes a stream interrupted after partial output.
- Nested Runs print no cache counts on verbose stderr, and a Nested Run's counts never appear in the parent's verbose line or JSON totals.
- No change to provider request or response mapping, retries or replay.

Acceptance: the R1 tests also assert that the budget-free `Tokens:` line is unchanged. One test with a positive budget, one nonzero cache count and the other zero asserts the existing two lines, including the zero count, and that `budget: <used>/<max>` and the `--json` `budget_used_tokens` equal input plus output tokens only. One test with `Verbose: false` and nonzero read and write fixtures asserts no `Cache:` text on stderr. One delegated Run whose Nested provider reports nonzero cache counts, and whose Main provider reports none, asserts no `Cache:` line on stderr and zero Main JSON totals. One stream-error Run with `Verbose: true` that fails after partial output asserts an error and that neither `Tokens:` nor `Cache:` appears on stderr. Existing tests `TestRun_CacheUsageBoundaries`, `TestResultMarshalJSON_CacheTokens` and `TestResultMarshalJSON_CacheTokensOmittedWhenZero` pass unchanged.

### R3 — Documentation and optional probe instructions

- Update the `--verbose` description in `README.md` and `docs/src/cli/run-flags.md` to say verbose output includes nonzero provider-reported cache counts, with or without a budget.
- Where the README and `docs/src/providers.md` describe cache counts, state that verbose stderr and `--json` report the same own-Run cumulative counts; that the counts are provider reports, not proof of hits or savings; and that OpenAI Chat reports cache reads only.
- Add optional probe instructions: how a user can observe counts by running the same agent twice with an identical static prefix and a changed task, using `--verbose` or `--json`. State that the probe makes paid provider calls with the user's own credentials, that results are point-in-time, that zero does not prove caching is disabled or unsupported, and that recorded results must not include secrets.
- Add a CHANGELOG entry under Unreleased.

Acceptance: the documents match R1 and R2 behavior. `~/.agents/scripts/docs-lint --since <milestone base>` from the repository root exits 0, and `mdbook build docs/` (the repository's documentation CI, `.github/workflows/docs.yml:21`) passes or is reported as not run with its reason.

### R4 — Gate results

The milestone delivers applicable gate results or explicit reasons for omissions ([milestone record](053_corrected_caching_milestones.md), Verification policy).

Acceptance: these pass at the completion HEAD and their results are recorded:

- focused `go test -race` on every touched package;
- `go test -race ./...` and `golangci-lint run` (the declared gates in `.agents/factory-gates.json`);
- `make check` or its parts: `golangci-lint run`, `go test ./...` and `make apidiff`;
- `.agents/scripts/ci-check integrity`, `.agents/scripts/ci-check vet`, `.agents/scripts/ci-check build` and `.agents/scripts/ci-check test`.

Any applicable gate not run is named with its reason. Docker smoke is not claimed unless a host gate is declared, and no credentialed live probe is run.

### Dependencies and parallel work

R1 and R2 change the same two summary branches and are tested together. R3 depends on R1's final wording. R4 runs last. No part needs parallel work.
