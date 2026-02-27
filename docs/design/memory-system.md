# Memory System

## Purpose

Memory is a run log — what the agent did, not what it should do. It gives agents awareness of past runs so they can spot patterns, avoid repeating work, and build context over time.

- **AGENTS.md / SKILL.md** = instructions (what to do)
- **Memory** = history (what happened)

## Storage

```
$XDG_DATA_HOME/axe/memory/<agent-name>.md
```

Plain markdown file. One entry per run, appended by axe automatically.

### Entry Format

```markdown
## 2026-02-27T03:15:00Z
**Task:** Review PR #42
**Result:** Found 3 issues: missing error handling in auth.go, unused import, test coverage gap in user_test.go

## 2026-02-27T14:30:00Z
**Task:** Review PR #45
**Result:** Clean. Minor style nit on line 82 of handler.go.
```

## Config

```toml
[memory]
enabled = true
last_n = 10          # Load last N entries into context (default: 10, 0 = all)
max_entries = 100    # Warn / trigger GC when exceeded
```

## How It Works

1. Agent runs
2. Axe appends a timestamped entry with the task and result summary
3. On next run, axe loads the last `last_n` entries into context
4. Agent sees its own history — patterns, past decisions, recurring issues

## What Gets Stored

- Timestamp (UTC)
- Task (what the agent was asked to do)
- Result summary (the agent's final output, truncated if needed)
- Stdin context is NOT stored (could be large, and the task description should capture intent)
- Sub-agent calls are NOT stored in the parent's memory (they have their own)

## Garbage Collection

When memory files grow large or the agent keeps making the same mistakes, GC surfaces patterns and trims the log.

### Command

```bash
axe gc <agent>
```

### What It Does

1. Reads the full memory log for the agent
2. Sends it to an LLM with a pattern-detection prompt
3. Outputs actionable suggestions to stdout
4. Trims the memory file (keeps last `last_n` entries, drops the rest)

### Example Output

```
📋 Patterns found in pr-reviewer (47 runs):

  Recurring issues:
  - Auth error handling flagged 12 times → consider adding to SKILL.md
  - Test coverage gaps found in 8/47 runs → add to review checklist

  Possible improvements:
  - Agent repeatedly suggests test coverage → already in SKILL.md, may need stronger wording
  - 3 false positives on import ordering → consider excluding from review scope

  Stats:
  - 47 total runs since 2026-01-15
  - Avg result length: 142 words
  - Most active day: Tuesday (12 runs)

Memory trimmed: 47 → 10 entries kept
```

### The Feedback Loop

```
Agent runs → logs to memory → GC finds patterns → suggests fixes → user updates SKILL.md → agent improves
```

The user is always the gatekeeper. Axe never auto-modifies SKILL.md or AGENTS.md — it only suggests. The human decides what becomes permanent instruction.

### GC Flags

```bash
axe gc <agent>              # Analyze + trim
axe gc <agent> --dry-run    # Analyze only, don't trim
axe gc <agent> --all        # Run GC on all agents
```

## Design Principles

- **Just a text file** — users can read, edit, grep, delete it
- **No database** — no SQLite, no embeddings, no magic
- **Axe writes, humans read** — memory is append-only from axe's perspective
- **Patterns graduate to config** — recurring lessons move from memory → SKILL.md/AGENTS.md via human decision
