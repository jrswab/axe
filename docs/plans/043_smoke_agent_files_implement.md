# 043 — Smoke Test Agent Files: Implementation Guide

**Spec:** `docs/plans/043_smoke_agent_files_spec.md`

---

## Section 1: Context Summary

This milestone creates the 25 static files (23 TOML agent configs, 1 skill Markdown file, 1 context text file) that the smoke-test CI workflow (milestones 044–049) will consume. No Go code is changed. Every file lives under `.github/smoke-agents/`. Skill paths are intentionally omitted from the two skill-dependent agents (`cfg-skill.toml`, `dry-skill.toml`) because `resolve.Skill()` resolves relative paths against the XDG config dir, not the repo — the CI workflow will pass `--skill` at invocation time instead. File paths in `cfg-files.toml` and `dry-files.toml` use repo-root-relative paths (`.github/smoke-agents/smoke-context.txt`) because the `files` field resolves against the agent's workdir (cwd in CI). Validation is done by running `axe run --agents-dir .github/smoke-agents <agent-name> --dry-run` for each agent from the repo root — if TOML parsing or validation fails, the dry-run exits non-zero.

---

## Section 2: Implementation Checklist

### Phase 1 — Directory structure and supporting files

These two tasks have no dependencies and can be done in parallel with each other.

- [x] Create `.github/smoke-agents/skills/smoke-skill/SKILL.md` — Markdown file, ≤10 lines. Must contain a clear instruction for the LLM to include the exact phrase `smoke-skill-loaded` in its response. Example content:
  ```markdown
  # Smoke Test Skill

  You MUST include the exact phrase `smoke-skill-loaded` somewhere in your response.
  ```

- [x] Create `.github/smoke-agents/smoke-context.txt` — Plain text file, ≤5 lines. Must contain the exact phrase `smoke-context-loaded` on a line by itself. Example content:
  ```
  This file is used for smoke testing the files config section.
  smoke-context-loaded
  ```

### Phase 2 — Provider test agents

All three can be created in parallel. No dependencies on Phase 1.

- [x] Create `.github/smoke-agents/provider-anthropic.toml` — Fields: `name = "provider-anthropic"`, `model = "anthropic/claude-3-5-haiku-20241022"`, `description = "Smoke test: Anthropic provider connectivity"`. No other fields.

- [x] Create `.github/smoke-agents/provider-openai.toml` — Fields: `name = "provider-openai"`, `model = "openai/gpt-4o-mini"`, `description = "Smoke test: OpenAI provider connectivity"`. No other fields.

- [x] Create `.github/smoke-agents/provider-opencode.toml` — Fields: `name = "provider-opencode"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: OpenCode provider connectivity"`. No other fields.

### Phase 3 — Tool test agents

All seven can be created in parallel. No dependencies on Phase 1 or 2.

- [x] Create `.github/smoke-agents/tool-list-dir.toml` — Fields: `name = "tool-list-dir"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: list_directory tool"`, `tools = ["list_directory"]`. No other fields.

- [x] Create `.github/smoke-agents/tool-read-file.toml` — Fields: `name = "tool-read-file"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: read_file tool"`, `tools = ["read_file"]`. No other fields.

- [x] Create `.github/smoke-agents/tool-write-file.toml` — Fields: `name = "tool-write-file"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: write_file tool"`, `tools = ["write_file"]`. No other fields.

- [x] Create `.github/smoke-agents/tool-edit-file.toml` — Fields: `name = "tool-edit-file"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: edit_file tool"`, `tools = ["edit_file", "read_file"]`. Two tools — `read_file` is included so the LLM can verify its edit.

- [x] Create `.github/smoke-agents/tool-run-cmd.toml` — Fields: `name = "tool-run-cmd"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: run_command tool"`, `tools = ["run_command"]`. No other fields.

- [x] Create `.github/smoke-agents/tool-url-fetch.toml` — Fields: `name = "tool-url-fetch"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: url_fetch tool"`, `tools = ["url_fetch"]`. No other fields.

- [x] Create `.github/smoke-agents/tool-web-search.toml` — Fields: `name = "tool-web-search"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: web_search tool"`, `tools = ["web_search"]`. No other fields.

### Phase 4 — Config section test agents

All seven can be created in parallel. `cfg-subagents.toml` and `cfg-sub-child.toml` must be created together (parent references child by name). No dependencies on Phases 1–3.

- [x] Create `.github/smoke-agents/cfg-basic.toml` — Fields: `name = "cfg-basic"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: basic config (name + model only)"`. No other fields.

- [x] Create `.github/smoke-agents/cfg-skill.toml` — Fields: `name = "cfg-skill"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: skill config section"`. No `skill` field — the CI workflow passes `--skill` at invocation time.

- [x] Create `.github/smoke-agents/cfg-memory.toml` — Fields: `name = "cfg-memory"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: memory config section"`. Add `[memory]` table with `enabled = true`, `last_n = 5`, `max_entries = 50`. No `path` override.

- [x] Create `.github/smoke-agents/cfg-files.toml` — Fields: `name = "cfg-files"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: files config section"`, `files = [".github/smoke-agents/smoke-context.txt"]`. Path is relative to workdir (repo root).

- [x] Create `.github/smoke-agents/cfg-subagents.toml` — Fields: `name = "cfg-subagents"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: sub_agents config section"`, `sub_agents = ["cfg-sub-child"]`. Add `[sub_agents_config]` table with `max_depth = 2`, `parallel = true`, `timeout = 60`.

- [x] Create `.github/smoke-agents/cfg-sub-child.toml` — Fields: `name = "cfg-sub-child"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: sub-agent child"`, `system_prompt = "You are a simple responder. When called, reply with only: sub-agent-ok"`. No tools, no skill, no memory.

- [x] Create `.github/smoke-agents/cfg-params.toml` — Fields: `name = "cfg-params"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: params config section"`. Add `[params]` table with `temperature = 0.5`, `max_tokens = 100`.

### Phase 5 — Piped input test agent

No dependencies on prior phases.

- [x] Create `.github/smoke-agents/pipe-basic.toml` — Fields: `name = "pipe-basic"`, `model = "opencode/minimax-m2.5"`, `description = "Smoke test: piped stdin input"`. No other fields.

### Phase 6 — Dry-run test agents

All five can be created in parallel. No dependencies on prior phases (though `dry-files.toml` references `smoke-context.txt` which must exist for dry-run to resolve files — Phase 1 must complete before validation).

- [x] Create `.github/smoke-agents/dry-basic.toml` — Fields: `name = "dry-basic"`, `model = "openai/gpt-4o"`, `description = "Smoke test: dry-run baseline"`. No other fields.

- [x] Create `.github/smoke-agents/dry-tools.toml` — Fields: `name = "dry-tools"`, `model = "openai/gpt-4o"`, `description = "Smoke test: dry-run with tools"`, `tools = ["read_file", "list_directory"]`.

- [x] Create `.github/smoke-agents/dry-skill.toml` — Fields: `name = "dry-skill"`, `model = "openai/gpt-4o"`, `description = "Smoke test: dry-run with skill"`. No `skill` field — the CI workflow passes `--skill` at invocation time.

- [x] Create `.github/smoke-agents/dry-memory.toml` — Fields: `name = "dry-memory"`, `model = "openai/gpt-4o"`, `description = "Smoke test: dry-run with memory"`. Add `[memory]` table with `enabled = true`, `last_n = 5`, `max_entries = 50`.

- [x] Create `.github/smoke-agents/dry-files.toml` — Fields: `name = "dry-files"`, `model = "openai/gpt-4o"`, `description = "Smoke test: dry-run with files"`, `files = [".github/smoke-agents/smoke-context.txt"]`.

### Phase 7 — Validation

Depends on all prior phases. Run from the repo root. Each command must exit 0 and print `=== Dry Run ===` to stdout.

- [x] Validate all 23 agent TOMLs parse and pass `agent.Validate()` by running `--dry-run` for each. Execute the following commands from the repo root (build the binary first with `go build -o axe .`):
  ```sh
  # Build
  go build -o axe .

  # Provider agents
  ./axe run --agents-dir .github/smoke-agents provider-anthropic --dry-run
  ./axe run --agents-dir .github/smoke-agents provider-openai --dry-run
  ./axe run --agents-dir .github/smoke-agents provider-opencode --dry-run

  # Tool agents
  ./axe run --agents-dir .github/smoke-agents tool-list-dir --dry-run
  ./axe run --agents-dir .github/smoke-agents tool-read-file --dry-run
  ./axe run --agents-dir .github/smoke-agents tool-write-file --dry-run
  ./axe run --agents-dir .github/smoke-agents tool-edit-file --dry-run
  ./axe run --agents-dir .github/smoke-agents tool-run-cmd --dry-run
  ./axe run --agents-dir .github/smoke-agents tool-url-fetch --dry-run
  ./axe run --agents-dir .github/smoke-agents tool-web-search --dry-run

  # Config section agents
  ./axe run --agents-dir .github/smoke-agents cfg-basic --dry-run
  ./axe run --agents-dir .github/smoke-agents cfg-skill --dry-run
  ./axe run --agents-dir .github/smoke-agents cfg-memory --dry-run
  ./axe run --agents-dir .github/smoke-agents cfg-files --dry-run
  ./axe run --agents-dir .github/smoke-agents cfg-subagents --dry-run
  ./axe run --agents-dir .github/smoke-agents cfg-sub-child --dry-run
  ./axe run --agents-dir .github/smoke-agents cfg-params --dry-run

  # Piped input agent
  ./axe run --agents-dir .github/smoke-agents pipe-basic --dry-run

  # Dry-run agents
  ./axe run --agents-dir .github/smoke-agents dry-basic --dry-run
  ./axe run --agents-dir .github/smoke-agents dry-tools --dry-run
  ./axe run --agents-dir .github/smoke-agents dry-skill --skill "$(pwd)/.github/smoke-agents/skills/smoke-skill/SKILL.md" --dry-run
  ./axe run --agents-dir .github/smoke-agents dry-memory --dry-run
  ./axe run --agents-dir .github/smoke-agents dry-files --dry-run
  ```

- [x] Verify `dry-tools` dry-run output contains `read_file, list_directory` in the `--- Tools ---` section (not `(none)`).

- [x] Verify `dry-skill` dry-run output (with `--skill` flag) contains skill content in the `--- Skill ---` section (not `(none)`).

- [x] Verify `dry-memory` dry-run output contains a `--- Memory ---` section.

- [x] Verify `dry-files` dry-run output contains `smoke-context.txt` in the `--- Files ---` section (not `(none)`) and the file content includes `smoke-context-loaded`.

- [x] Verify `cfg-files` dry-run output contains `smoke-context.txt` in the `--- Files ---` section.

- [x] Verify all agent `name` fields are unique. Run: `grep -h '^name' .github/smoke-agents/*.toml | sort | uniq -d` — must produce no output.

- [x] Verify no existing tests are broken: `go test ./...` must pass with no failures.
