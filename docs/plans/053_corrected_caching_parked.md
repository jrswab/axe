# Corrected caching — parked

Milestone 002's narrow A006 T1 fix was committed as `67fef1d41d9ac87ebddafe10f4db5943ad9c8ce2`; 002 and 003 remain unchecked, and 003 is blocked strictly on 002 completion. No external probes were authorized or run.

## Current outcome and review direction

P206's two A005 reviews both APPROVED; F5 and F6 are resolved and the planning gate passed. The shared planning count is two failed rounds (P203 and combined P205); the two P204/P204b incomplete events remain historical. A004 exhausted 4/4 calls and A005 consumed 2/2; no retries are authorized. T1 is checked and all axes passed: Design risk APPROVED in round 2 at `67fef1d41d9ac87ebddafe10f4db5943ad9c8ce2`; Standards and Spec APPROVED WITH NITS in round 3 at `7b50c9651bde3395bb786aff3cb10b7247351380`. No T1 reviews remain pending. The whole-suite factory-gate test passed at the round-3 SHA (poll exit 0; `/tmp/factory-gate-1001/gate-test-7b50c9651bde-20261123T173358Z.log`); final lint and affected tests remain pending. T2 and T3 remain unchecked; T2 is underway locally after the four scoped authorizations below, and 003 is blocked by 002. Publication remains on hold: no push or external communication.

- **F5 — HISTORICAL P205, resolved P206 (S2/R4):** At P205, `docs/plans/053_corrected_caching_002_implement.md:37,39` checked `json-output.md` but neither assigned documentation edits nor named `run-flags.md`. Both `docs/src/cli/json-output.md:80` and `docs/src/cli/run-flags.md:46` then said `--stream` and `--json` always stream, contrary to `docs/src/providers.md:109` and `pkg/runner/run.go:529-533`. P205 recommended assigning T2 both documentation edits to explain fallback while retaining a single JSON envelope.
- **F6 — HISTORICAL P205, resolved P206 (S4/R3–R4):** At P205, `docs/plans/053_corrected_caching_002_spec.md:52,60` versus `docs/plans/053_corrected_caching_002_implement.md:28,38,70` left omitted usage and independently zero cache usage untested through Bedrock Run. P205 recommended assigning T2 literal Converse fixtures that assert Result values and JSON omission for those cases, plus numerical nonzero counterparts, without changing accounting.

### Historical direction — handoff before T2

This handoff is historical; see the [current local-only T2 follow-up](#current-follow-up--four-scoped-approvals-t2-underway).

A006 in REC011 resolved the empty-description scope blocker. T1 passed all axes and is checked; no T1 review remains pending. The user interrupted this session for handoff. After restart, start T2 RED in the next session, without starting T2 now. Milestone 003 stays blocked only until 002 completes.

### High stakes question — empty Bedrock tool descriptions

The following question and recommendation record the earlier state; A006 in REC011 resolved it.

Optional empty tool descriptions are serialized as an empty string (`internal/provider/bedrock.go:136–140,257–259`), but AWS permits omission and requires length at least 1 when present (`/tmp/issue83-docs/aws-spec-md:23–27`). This behavior predates T1. Accepted R1 (`docs/plans/053_corrected_caching_002_spec.md:38–40`) asks to preserve descriptions and review against AWS contracts; the checklist (`docs/plans/053_corrected_caching_002_implement.md:29`) limits fixes to union/schema/order. An agent must not resolve this scope ambiguity unilaterally.

**Recommendation:** Explicitly authorize the narrow omission of descriptions only when empty, plus a new additive RED test in both cache modes. Preserve all existing assertions and nonempty descriptions; do not touch MCP or config. Then repeat only the required T1 blocking reviews. Alternatively, leave 002 parked and track this issue separately; 003 remains parked by dependency.

## Historical parking questions

The following questions and recommendation record the earlier state; they do not authorize current resumption.

### Q1 — High stakes: milestone 002 incomplete-review cap

**Recommendation:** In a later run, seek explicit new direction for a bounded, independent source-verification strategy; do not bypass the gate or the incomplete cap. No new direction or review is authorized now.

The complete-checklist gate for 002 has one shared failing planning round, P203. F204 fixed F1–F4 in the checklist and the text fixes were checked, but P204 and its P204b source-verification retry were consecutive incompletes in the same round 2. No new blocker was established by either partial review; the gate did not pass. The original 002 review base is `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`.

P204b recorded the remaining coverage verbatim ([002 checkpoint, P204b](053_corrected_caching_002_checkpoint.md#p204b-incomplete-round2-incomplete-cap-reached-park002003-in-flight)):

> Source verification still unfinished for milestone document, both checkpoints, 001 checklist, .scratch/issue-83 records/AWS research, GLOSSARY, COMPATIBILITY, CONTRIBUTING, TAGS, CHANGELOG, docs/src/configuration/agent-config.md, docs/src/cli/json-output.md, cmd/run.go, cited internal/pkg implementation/tests except bedrock.go; README/provider-guide surrounding passages unfinished.

Instead of claiming a complete review or retrying beyond the cap, the run parks 002 and blocks 003. See `docs/plans/053_corrected_caching_002_checkpoint.md:509-554` for P204 and P204b outcomes.

### Q2 — Low stakes: milestone 003 dependency

Milestone 003 is blocked by 002. Recommend resuming 003 after 002 completes.

### Q3 — Assertion approval

Already decided, not a live question: Jaron's `Q3 I approve` permits only the contradicted `TestBedrock_Send_Cache` assertions in `internal/provider/bedrock_test.go:638-645,652-656` to be replaced in future implementation: separate system text and default cache-point union members, and a separate default cache-point tools-array member while retaining the tool specification. Preserve ToolConfig existence, cache read/write usage, and all unrelated assertions/tests; independently cover raw-wire `inputSchema.json` and cache-disabled requests. No other assertion or acceptance change is approved; no assertion changes are authorized or made in this administrative task. See `docs/plans/053_corrected_caching_002_checkpoint.md:46`.

## Decided for you

- **D1 — Writable module cache:** For the existing Go preflight, use `GOMODCACHE=/tmp/axe-v006-cache.itoPgN/go-mod`: B008 failed because the default module cache was read-only and `XDG_CACHE_HOME` does not redirect module downloads. B009 retained all existing checks, including the vulnerability scan, and passed. This was a reversible cache choice, not a waiver (`docs/plans/053_corrected_caching_002_checkpoint.md:47,63-73`).
- **D2 — Three coherent checklist tasks:** F203 grouped 002 into T1 wire/stability, T2 Main/Nested AWS parity, and T3 unchanged regression/accounting. This gives the new behavioral contracts genuine test-first RED paths and preserves independently grounded contract evidence; new passing-only tests cannot use the unchanged-test existing-GREEN route. P206 approved the checklist after resolving F5 and F6 (`docs/plans/053_corrected_caching_002_checkpoint.md`).

## Run evidence

- Milestone 001 completed historically; its completion evidence is retained. Milestones 002 and 003 remain unchecked. T1 of 002 is checked; T2 and T3 remain unchecked. Partial T1 implementation and tests were committed as `4fa25a0` after RED and independent VERIFY, followed by the A006 fix at `67fef1d41d9ac87ebddafe10f4db5943ad9c8ce2`.
- B008 failed, B009 passed with the writable module cache, E009 host preflight passed, and Y012 sync passed without moving HEAD. These are recorded historical outcomes, not checks rerun by this administrative task (`docs/plans/053_corrected_caching_002_checkpoint.md:63-73`).
- No 003 spec, checklist, task, or review has ever been launched; there is no 003 implementation range. Its checkpoint identifies the historical administrative creation source, not an implementation review base.
- No external probes were authorized or run. The recorded whole-suite gate `go test -race ./...` passed at `7b50c9651bde3395bb786aff3cb10b7247351380`; final lint and affected tests remain pending. The draft PR is not completion.

The following handoff footer is historical; its unconsumed-call recommendation was superseded by P205 above.

## 2026-11-20 — Current authorization and handoff interruption

The prior parking evidence above remains historical. The user subsequently explicitly approved A004: one additional batch of exactly four independent read-only source-review calls beyond the reached incomplete cap, for the original complete-checklist round 2. S1 covers records/research/AWS; S2 standards/docs; S3 provider/config; S4 execution/errors/retries/budget. Shared planning failures remain 1 and prior consecutive incompletes remain 2; no counters reset and no incomplete retries are authorized. All four complete passes are required before continuing 002 and then 003; any nonpass parks.

P205 recorded intent, but the user requested handoff before any review dispatch. The established outcome is **NOT LAUNCHED**: zero calls launched or consumed, zero results, and all four approved calls remain available. The earlier P205 intent's “reserved/consumed” wording means reserved only, not consumed. There is no unknown launch identity or counter. The interruption ends this run; future execution must follow checkpoint recovery and the recorded authorization. L205 spec/docs lint and E010 and Y015 passed; source HEAD was `1bea88f7aa8c2fec55ac73517033914c4c9c2408`. Milestone 001 is complete, 002 and 003 are unchecked, no 002 implementation has launched, and 003 remains dependency-blocked. The explicit approval reference is `/home/jaron/.pi/agent/sessions/--home-jaron-dev-axe-iss-83-finish-caching--/2026-11-20T19-56-16-550Z_01a1fa87-f525-75f1-a143-b33079ea9dda.jsonl:197-198` (proposal and `I approve`).

## Local-only — pending security disposition

This section records the historical parking direction; the authorized follow-up below is current. Milestone 002 remains unchecked. T1 is checked; T2 and T3 are unchecked. T2's RED and GREEN are committed; its independent Standards, Spec, and Design risk round-1 reviews each returned **CHANGES REQUESTED**, with no incomplete results. Integrity retry passed at `9fe4129510d4078e5aabfcd6f79bb68aed7cd05f`; the earlier offline failure remains history. Review identified potential newly reachable Nested risks, confirmed by source triage; no actual exposure was observed ([002 checkpoint](053_corrected_caching_002_checkpoint.md), T2 round 1 and integrity retry entries).

The following three questions and recommendations preserve the historical decision wording. They are no longer open: see the current follow-up below.

### 1 — High stakes question: validate the resolved region?

Nested now passes a resolved region to provider construction (`internal/tool/tool.go:227-245`); the same underlying endpoint weakness affects Main. The provider constructs the endpoint from that region and signs requests (`internal/provider/bedrock.go:42-55,309-324`). This security-boundary change is not authorized by the current T2 scope.

**Recommendation:** Authorize provider-scope validation of the resolved region before endpoint construction for both Main and Nested. **Alternative:** Keep parity unpublished pending a separate fix.

### 2 — High stakes question: protect credential-path details?

A provider-construction failure may carry local credential-path details into parent-model-facing tool output (`internal/provider/awscreds.go:31-49`, `internal/tool/tool.go:241-245,497-506`, `pkg/runner/run.go:758-765`). The current scope does not settle the diagnostic/security boundary.

**Recommendation:** Authorize keeping credential-path details out of parent-model-facing failure while retaining a safe actionable diagnostic. **Alternative:** Keep parity unpublished pending a separate fix.

### 3 — High stakes question: correct the Converse tool-result role?

The provider currently maps a non-user/assistant role to assistant while serializing tool results (`internal/provider/bedrock.go:204-235`); the existing role assertion at `pkg/runner/large_integer_args_test.go:474` is outside T2 and Q3. Q3 permits only the specified `TestBedrock_Send_Cache` union/checkpoint assertion replacements, preserving ToolConfig existence, usage, all unrelated assertions, and independent schema/disabled-request coverage ([002 checkpoint](053_corrected_caching_002_checkpoint.md), Decision Q3).

**Recommendation:** Authorize provider-scoped Converse tool-result role correction and only the existing role assertion at `pkg/runner/large_integer_args_test.go:474`, plus tests preserving large-integer checks. **Alternative:** Leave the role and assertions untouched and keep 002 blocked.

In-scope review coverage remains open: explicit comparison of serialized static bytes across fresh Runs with varying task, context, and history, and all omitted static-shape cases. These are blockers, not waived by the integrity pass. Nit candidates only: nonexclusive ordinary-provider lookup, indexed lost-response fixture, cross-run comparison, and inherited credential I/O timing; the cross-run byte-comparison requirement remains a Spec blocker. All required coverage, reviews, and gates remain intact ([002 spec](053_corrected_caching_002_spec.md), R1–R4; [002 checklist](053_corrected_caching_002_implement.md), T2 and final completion gate).

Milestone 003 is blocked by 002, with no 003 spec, checklist, task, or review launched. Existing draft #108 remains at verified `d18abf7a2bd50cd9f99112243482ba1b10974d4c`; no push of local commits has occurred. One-way doors, external communications, and all pushes remain on hold.

### Current follow-up — four scoped approvals, T2 underway

Jaron authorized (1) provider-scope resolved-region validation before endpoint construction for Main and Nested, (2) protecting local credential-path details in parent-model-facing failures while retaining safe actionable guidance, and (3) the narrow Converse tool-result role correction, only the existing role assertion in `pkg/runner/large_integer_args_test.go` and tests preserving large-integer checks ([security disposition](053_corrected_caching_002_checkpoint.md#jaron-security-disposition-and-read-only-restart)). His exact `Q4 -  yes` separately authorized (4) splitting only the missing-file assertion: preserve Main filename/cause checks and require actionable Nested guidance without path or filename ([Q4](053_corrected_caching_002_checkpoint.md#q4-authorization-and-restart)). Original Q3 and all other acceptance and assertions remain intact. No other scope expansion, push or external communication is authorized.

T2 is underway locally. The scoped RED, including cross-run static bytes and omitted shapes, is recorded in [the checkpoint](053_corrected_caching_002_checkpoint.md#authorized-red-and-fix-intent); preserve the two RED test blobs and rerun its exact command GREEN. Passing those checks does not waive the round-1 findings, required independent reviews, T3 or completion gates. T2 and 002 remain unchecked; 003 remains blocked.
