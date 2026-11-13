Hullo @jrswab 👋

I ran your skills through `tessl skill review` at work and found some targeted improvements. Here's the full before/after:

| Skill | Before | After | Change |
|-------|--------|-------|--------|
| sample | 34% | 88% | +54% |
| release | 76% | 100% | +24% |
| code-review | 84% | 100% | +16% |
| commit-msg | 84% | 100% | +16% |
| summarizer | 80% | 93% | +13% |

![Score Card](score_card.png)

**Note:** Four skills (code-review, commit-msg, summarizer, sample) originally scored 0% because they lacked YAML frontmatter entirely. The "before" scores above reflect a revised baseline after adding minimal `name` and `description` frontmatter, so the comparison is against meaningful scores rather than a structural zero.

<details>
<summary>Changes made</summary>

**All 5 skills — frontmatter & description improvements:**
- Added YAML frontmatter (`name`, `description`) to the 4 example/sample skills that were missing it
- Added explicit "Use when..." clauses with natural trigger terms to every skill description (e.g., "ship", "publish", "cut a release" for the release skill; "TLDR", "condense", "takeaways" for the summarizer)
- Ensured all descriptions use quoted-string format (not YAML chevron blocks)

**release skill — content streamlining:**
- Consolidated three separate "DO NOT" rules into a single "User confirmation required" line
- Removed external links to Semantic Versioning and Keep a Changelog specs (Claude already knows these standards) — just references them by name

**sample skill — content overhaul:**
- Replaced abstract meta-instructions ("Define a clear, focused purpose") with concrete, actionable guidance
- Removed redundant explanation of what skills are (the LLM already knows)
- Added practical constraints section with specific guardrails (500-line limit, avoid abstract instructions, include examples)
- Added a concrete output format example

</details>

Honest disclosure — I work at @tesslio where we build tooling around skills like these. Not a pitch - just saw room for improvement and wanted to contribute.

Want to self-improve your skills? Just point your agent (Claude Code, Codex, etc.) at [this Tessl guide](https://docs.tessl.io/evaluate/optimize-a-skill-using-best-practices) and ask it to optimize your skill. Ping me - [@popey](https://github.com/popey) - if you hit any snags.

Thanks in advance 🙏
