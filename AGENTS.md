# Axe — AGENTS.md

## What This Is

A lightweight Go library and CLI for running single-purpose LLM agents. Think `make` for AI, define agents in TOML, give them focused context via SKILL.md files, and trigger them from anywhere (cron, git hooks, pipes, file watchers).

## Design Constraints

- Give each agent focused context. Keep sub-agent execution opaque to parents. Return outcomes without exposing internal turns, tool calls, or files.
- Keep stdout pipeable, diagnostics on stderr, and exit codes meaningful. Accept stdin.
- Axe executes agents; scheduling and triggering belong to the caller.
- Ship a single Go binary without a runtime dependency.
- Define agents in TOML. Treat SKILL.md as a community format; place Axe-specific configuration in TOML.
- Use `provider/model-name` model strings and XDG locations for config, data, and cache.
- Treat delegation depth limits as safety rails.

## Contributions

- Test new packages, favor table-driven tests and real dependencies or test helpers where practical, and provide actionable errors.
- Pass dependencies explicitly.
- Preserve each setting's resolution order. Test explicit flag values separately from omitted flags.
- Before completion, follow [CONTRIBUTING.md](CONTRIBUTING.md), run the applicable local gates in [.agents/factory-gates.json](.agents/factory-gates.json) and [Makefile](Makefile), and report any gates not run and why.

## Custom Instructions
<!-- This section is for human and agent-maintained operational knowledge.
     Add repo-specific conventions, gotchas, and workflow rules here.
     This section is preserved exactly as-is when re-running codebase-summary. -->

### Project memory

When `.mem/mem-map.md` exists, read it and the linked records relevant to the change. Otherwise use source and the references below; creating a memory corpus is not required. Source is authoritative when records disagree.

When an existing record covers a persistent change to behavior, architecture, constraints, or operational conventions, update the relevant records to match current behavior. Use the `mem` skill if available; otherwise update the existing records directly. Done means the relevant records agree with the resulting behavior.

### Anchor tags

Use `// @anchor:<surface>:<role> [context]` in the file's native comment syntax for entry, policy, configuration, or integration boundaries introduced or changed by the task. Derive descriptive surface and role identifiers from existing tags and the boundary purpose; reuse identifiers when they fit. The one-line context states the boundary's purpose. Find existing tags with `grep -rn "@anchor:<surface>"`.

When locating or changing tagged boundaries, surfaces, or roles, read [TAGS.md](TAGS.md).

## Change-triggered references

- **Public Go API, CLI, TOML, JSON, or defaults:** read [COMPATIBILITY.md](COMPATIBILITY.md).
- **Execution or delegation:** read [GLOSSARY.md](GLOSSARY.md), [pkg/runner/run.go](pkg/runner/run.go), and [internal/tool/tool.go](internal/tool/tool.go); inspect both execution paths and explicit task/context transfer. For the CLI adapter, inspect [cmd/run.go](cmd/run.go).
- **Configuration or defaults:** inspect [internal/agent/agent.go](internal/agent/agent.go), its loader and tests, and the consuming implementation and tests.
- **Providers:** inspect [internal/provider/provider.go](internal/provider/provider.go).
- **MCP:** inspect [internal/agent/agent.go](internal/agent/agent.go), [internal/mcpclient/mcpclient.go](internal/mcpclient/mcpclient.go), and [internal/mcpclient/router.go](internal/mcpclient/router.go).
- **Execution boundaries:** inspect file-root selection and artifact routing in [internal/tool/read_file.go](internal/tool/read_file.go), [internal/tool/write_file.go](internal/tool/write_file.go), and [pkg/runner/run.go](pkg/runner/run.go); inspect [path validation](internal/tool/path_validation.go) and [heuristic shell validation](internal/tool/command_validation.go).
- **Memory or GC:** inspect [internal/memory](internal/memory), [cmd/gc.go](cmd/gc.go), and their tests.
- **Web search settings:** inspect [internal/tool/web_search.go](internal/tool/web_search.go).
- **Tests or CI:** follow [CONTRIBUTING.md](CONTRIBUTING.md) and inspect the affected tests and [.github/workflows](.github/workflows).
- **Containers:** inspect [Dockerfile](Dockerfile) and [docker-compose.yml](docker-compose.yml).
