# Sub-Agent Communication Pattern

## Purpose

Sub-agents let a parent agent delegate work without bloating its own context window. The sub-agent does the work (which may involve many turns, tool calls, file reads, etc.) and only the final result text returns to the parent.

## Flow

```
User → axe run parent-agent
         ├── Parent LLM decides it needs help
         ├── Calls call_agent tool with task + context
         │     ├── Axe intercepts the tool call
         │     ├── Validates agent name is in parent's sub_agents list
         │     ├── Loads sub-agent's TOML (system_prompt, skill, files)
         │     ├── Passes parent's task + context to sub-agent
         │     ├── Sub-agent runs (may take multiple internal turns)
         │     └── Returns final result text to parent as tool response
         ├── Parent continues with just the summary
         └── Returns final result to user
```

## Injected Tool

Axe auto-injects this tool when `sub_agents` is defined:

```json
{
  "name": "call_agent",
  "description": "Delegate a task to a sub-agent. The sub-agent runs independently with its own context and returns only its final result.",
  "parameters": {
    "agent": {
      "type": "string",
      "description": "Name of the sub-agent (must be in this agent's sub_agents list)"
    },
    "task": {
      "type": "string",
      "description": "What you need the sub-agent to do"
    },
    "context": {
      "type": "string",
      "description": "Additional context from your conversation to pass along"
    }
  }
}
```

## What the Sub-Agent Receives

1. Its own `system_prompt` from its TOML
2. Its own `skill` (SKILL.md) from its TOML
3. Its own `files` resolved from its workdir
4. The `task` string from the parent
5. The `context` string from the parent (if provided)

The sub-agent does NOT receive the parent's full conversation history.

## What the Parent Receives Back

Plain text result only (v1). The parent never sees:
- The sub-agent's internal conversation turns
- Files the sub-agent read
- Tool calls the sub-agent made
- Any intermediate reasoning

This is the core value: the parent gets the benefit of the work without the context cost.

## Config

```toml
# In parent agent's TOML
sub_agents = ["test-runner", "lint-checker"]

[sub_agents_config]
max_depth = 3          # Max nesting depth (default: 3, max: 5)
parallel = true        # Run parallel tool calls concurrently (default: true)
timeout = 120          # Per sub-agent timeout in seconds (default: 120)
```

## Depth Limiting

- Default max depth: 3
- Hard max: 5
- When depth limit is reached, the agent does NOT get the `call_agent` tool injected
- This prevents runaway nesting chains

## Parallel Execution

If the parent LLM returns multiple `call_agent` tool calls in one response, axe runs them concurrently (goroutines). Results return together as separate tool responses.

## Failure Handling

If a sub-agent fails (API error, timeout, crash):
- Return an error message as the tool result
- Do NOT crash the parent agent
- Let the parent LLM decide how to proceed (retry, skip, report)

Example error tool response:
```
Error: sub-agent "test-runner" failed — API timeout after 120s. You may retry or proceed without this result.
```

## v2 Considerations

- Structured JSON output option per sub-agent
- Streaming partial results back to parent
- Sub-agent cost/token tracking
- Shared memory between parent and sub-agents
