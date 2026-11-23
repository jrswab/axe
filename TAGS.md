# Anchor tag registry

See [Anchor tags](AGENTS.md#anchor-tags) for the tagging policy.

## Discovery

Search tracked non-Markdown text files from the repository root:

```sh
git grep -n -I -F -e '@anchor:' -- . ':(exclude,glob)**/*.[mM][dD]' ':(exclude,glob)**/*.[mM][aA][rR][kK][dD][oO][wW][nN]'
```

This excludes Markdown documentation examples and binary files; exit 1 means no matches.

## Surfaces

- **provider** — Provider selection, credential configuration, nested Bedrock region and AWS credential setup, compatible API responses, and optional gateway cost policy. Lookup: `@anchor:provider:`.

Once a surface has its first anchor, register its name, a one-line meaning, and its exact `@anchor:<surface>:` lookup pattern with the actual surface name. Search by this stable pattern rather than cached line numbers. Update the entry when anchors are added or renamed, and remove it when the last anchor for that surface is removed.

Completion: every implemented source surface is listed, and every registry entry matches at least one source anchor.
