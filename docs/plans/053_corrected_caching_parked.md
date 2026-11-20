# Corrected caching — parked

The run ends with a draft PR because milestones 002 and 003 are unchecked. No external probes were authorized or run.

## Parking questions

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
- **D2 — Three coherent checklist tasks:** F203 grouped 002 into T1 wire/stability, T2 Main/Nested AWS parity, and T3 unchanged regression/accounting. This gives the new behavioral contracts genuine test-first RED paths and preserves independently grounded contract evidence; new passing-only tests cannot use the unchanged-test existing-GREEN route. The checklist remains unapproved at its capped review (`docs/plans/053_corrected_caching_002_checkpoint.md:338-363`).

## Run evidence

- Milestone 001 completed historically; its completion evidence is retained. Milestones 002 and 003 remain unchecked. T1–T3 of 002 are unchecked; no 002 implementation or task execution was launched. No source or test edits were made in the current session.
- B008 failed, B009 passed with the writable module cache, E009 host preflight passed, and Y012 sync passed without moving HEAD. These are recorded historical outcomes, not checks rerun by this administrative task (`docs/plans/053_corrected_caching_002_checkpoint.md:63-73`).
- No 003 spec, checklist, task, or review has ever been launched; there is no 003 implementation range. Its checkpoint identifies the current HEAD only as the administrative creation source.
- No external probes were authorized or run. Source tests do not apply to these Markdown bookkeeping changes. The run ends with a draft PR, not completion; commit, sync, push and PR operations are outside this administrative task.
