# Axe

A lightweight CLI for running single-purpose LLM agents: agents defined in TOML, focused context via SKILL.md, triggered from anywhere.

**Order**:
- **Agent Run**: One conceptual execution of an agent, from context resolution through final result — regardless of what surface triggered it.
- **Nested Run**: An Agent Run initiated inside another Agent Run via delegation.
- **Sub-agent**: The agent executing a Nested Run, invoked by its parent.

**Agent Run**:
A single conceptual execution of an agent definition: the agent's config, skill, and files are resolved, the LLM conversation loop executes (with tools, MCP, memory, artifacts), and a final result is produced. The `axe run` CLI command, the public `Run` function, and the `call_agent` tool are all adapters into the same Agent Run; they are entry points, not separate kinds of execution. A Nested Run launched via `call_agent` is the same kind of Agent Run, just entered through delegation.

Execution policy — retry, streaming transport, tool parallelism, turn limits, provider setup, and memory lifecycle — is intrinsic to the Agent Run, not to the entry adapter. The `axe run` CLI command, the public `Run` function, and the `call_agent` tool translate inputs and project outputs but do not redefine these concerns. Internal streaming never crosses the Nested Outcome boundary.
_Avoid_: session, invocation, execution (as a noun for the whole run)

**Nested Run**:
An Agent Run that is started from within another Agent Run, counting one level of depth toward the depth limit. It executes independently with its own resolved context. Its auto-created artifact directory is an internal detail: it is removed before the Nested Outcome returns. An artifact directory the sub-agent's own configuration explicitly sets remains persistent.
_Avoid_: child run, inner run, recursive run

**Sub-agent**:
The agent (an execution, not a definition) performing a Nested Run on behalf of the parent Agent Run. A sub-agent is opaque to its parent: only the Nested Outcome crosses the boundary — never its internal turns, tool calls, or files.
_Avoid_: child agent, delegated agent

**Nested Outcome**:
The information that crosses from a Nested Run to its parent Agent Run: the final text plus a closed cause — one of `ok`, `ceiling`, `config`, `runtime`, `cancelled`. The Nested Run's internal turns, tool activity, files, token counts, and artifacts do not cross this seam. The `call_agent` adapter projects the Nested Outcome onto a tool result for the parent's conversation; it is a projection at the adapter, not a Go type or a final interface — nothing in `pkg` encodes the outcome as a concrete type.
_Avoid_: sub-agent result (ambiguous about the cause), return value

**Ceiling**:
A parent-imposed upper constraint on a Nested Run, covering remaining tokens, remaining wall-clock time, remaining nesting depth, and cancellation. Beneath its Ceilings, a Nested Run resolves its own Agent definition, context, and configuration independently — the parent constrains only what it imposes, and the sub-agent's internals stay opaque. A Nested Run's own timeout, token budget, and maximum nesting may tighten the corresponding remaining Ceiling but never loosen it; cancellation is always inherited.
_Avoid_: budget (for the imposed portion), quota, limit

**Agent definition**:
The declarative configuration of an agent — its TOML file (name, model, system prompt, skill, files, tools, sub-agents). A definition is static; an Agent Run is one execution of it. Never use "agent" alone when the distinction between definition and execution matters.
_Avoid_: agent spec, agent config (when referring to the executed instance)