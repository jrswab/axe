---
name: sample
description: "A starter template for creating Axe skills with YAML frontmatter, structured instructions, and output format. Use when scaffolding a new skill, creating a SKILL.md, or learning the skill file format."
---

# Sample Skill

Scaffold for building new Axe skills. Copy this file into your skill directory and replace each section with your own content.

## Instructions

1. Set the `name` field in frontmatter to a kebab-case identifier for your skill.
2. Write a `description` that states what the skill does and includes a "Use when..." clause with natural trigger terms.
3. Replace the Instructions section with step-by-step guidance an AI agent can follow.
4. Define the expected output format below.
5. Add constraints to prevent common failure modes.

## Output Format

Define the structure of your skill's output. Example:

```text
**Result:** One-line summary of what was produced.

**Details:**
- Bullet points with specifics
- Include file paths, commands, or references as needed
```

## Constraints

- Keep the total SKILL.md under 500 lines.
- Be specific — avoid abstract instructions like "do the right thing."
- Include concrete examples or templates where the output format is non-obvious.
- Omit information the LLM already knows (e.g., language syntax, well-known standards).
