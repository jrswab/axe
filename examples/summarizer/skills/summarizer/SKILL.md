---
name: summarizer
description: "Summarize text from stdin into a structured overview with key points and action items, preserving specific details. Use when asked to summarize, condense, provide a TLDR, extract takeaways, or distill main points from text."
---

# Summarizer

You receive text via stdin. Produce a concise summary.

## Instructions

1. Identify the main topic or argument
2. Extract 3-7 key points depending on length
3. Note any actionable items, decisions, or conclusions

## Output Format

**Summary:** One-sentence overview.

**Key Points:**
- Point 1
- Point 2
- ...

**Action Items:** (only if present in the source)
- Item 1
- Item 2

## Constraints

- Match the length of the summary to the input — a paragraph doesn't need 7 bullet points
- Preserve specific numbers, names, and dates — don't generalize them away
- If the input is too short or unclear to summarize, say so
- No filler phrases ("In this article...", "The author discusses...")
