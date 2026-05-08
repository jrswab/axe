---
name: release
description: "Prepare code for release (version bumps, changelog, README updates) and create an annotated tag to trigger the GoReleaser workflow. Use when shipping, publishing, cutting, or preparing a new version or release."
---

Your job is to guide the user through a full release for this project. A release includes preparing the code (version bumps, README documentation, changelog), creating an annotated git tag with a `v` prefix (e.g. `v1.5.0`), pushing it to trigger the GoReleaser GitHub Actions workflow, setting release notes, and linking the GHCR Docker image.

## Rules

- Always create an **annotated** tag (`git tag -a`), never a lightweight tag.
- Tag format is `vMAJOR.MINOR.PATCH` (Semantic Versioning).
- Changelog follows Keep a Changelog format. Valid categories: Added, Changed, Deprecated, Removed, Fixed, Security.
- All three version string locations (listed in Reference) MUST be updated before committing. Run version tests after bumping to catch drift.
- README.md MUST document any user-facing changes — new config fields, CLI flags, behavior changes, exit codes.
- **User confirmation required** before: pushing to master, pushing the tag, and for the version number, tag annotation message, and changelog review.

## Steps

1. **Gather context**
   - Check the current branch and ensure it is clean (`git status`).
   - List existing tags (`git tag --sort=-v:refname`) to determine the next version.
   - Collect the commit log since the last tag (or all commits if this is the first release).

2. **Ask the user**
   - What version number to use.
   - What the annotated tag message should say.
   - Whether the auto-generated changelog looks correct or needs edits.

3. **Categorize commits** into Keep a Changelog sections:
   - `Added` — new features and capabilities.
   - `Changed` — changes to existing functionality.
   - `Deprecated` — features marked for removal.
   - `Removed` — features that were removed.
   - `Fixed` — bug fixes.
   - `Security` — vulnerability fixes.
   - Exclude commits prefixed with `docs:`, `test:`, `ci:`, or merge commits.

4. **Bump version strings**
   - Update all three version locations listed in the Reference section.
   - Run `go test ./cmd/ -run TestVersion -count=1` to verify the bump is consistent.

5. **Update README.md**
   - Explore new feature implementations (via subagents if available) to understand config fields, flags, defaults, and behavior before writing documentation.
   - For each user-facing change, update the relevant README sections: Features list, Run Flags table, Agent Configuration example TOML, and add new sections as needed.
   - Add or update the Exit Codes table if new exit codes were introduced.
   - Do NOT guess at config fields or flag names — read the actual implementation first.

6. **Update CHANGELOG.md**
   - Prepend the new release section under the `# Changelog` header.
   - Add a reference link at the bottom of the file using the Keep a Changelog format:
     - First release: `[1.0.0]: https://github.com/jrswab/axe/releases/tag/v1.0.0`
     - Subsequent releases: `[1.1.0]: https://github.com/jrswab/axe/compare/v1.0.0...v1.1.0`

7. **Commit and push prep changes**
   - Stage all modified files and commit with message: `Update changelog and version for vX.Y.Z`
   - Push to master only after the user confirms.

8. **Verify**
   - Run `go build .` to confirm the binary compiles.
   - Run `go test ./cmd/ -run TestVersion -count=1` at minimum.
   - Confirm tests pass before proceeding to tag creation.

9. **Ask user to confirm tag creation**
   - Show the tag name, annotation message, and commit hash.
   - Wait for explicit confirmation before creating the tag.

10. **Create the annotated tag** on the prep commit.

11. **Push the tag** to origin only after the user confirms. The `v*` tag push triggers the GoReleaser workflow which builds binaries and creates the GitHub release.

12. **Set release notes via `gh`**
    - Extract the new version's section from `CHANGELOG.md` — everything between the `## [X.Y.Z]` heading and the next `##` heading, excluding the heading itself and trailing blank lines.
    - Append a Docker section to the extracted notes:
      ````
      ## Docker

      ```bash
      docker pull ghcr.io/jrswab/axe:X.Y.Z
      ```

      See the [GHCR package page](https://github.com/jrswab/axe/pkgs/container/axe) for all available tags.
      ````
    - Write the combined content (changelog section + Docker section) to a temporary file.
    - Poll `gh release view vX.Y.Z` every 10 seconds (up to 120 seconds) until the GoReleaser workflow creates the release.
    - Run `gh release edit vX.Y.Z --notes-file <tmp-file>` to set the release description.
    - Clean up the temporary file.
    - This is the authoritative source for release notes — do NOT rely on the workflow's awk extraction.

## Reference

### Project Files
- GoReleaser config: `.goreleaser.yml` (auto-changelog is disabled)
- Release workflow: `.github/workflows/release.yml` (builds binaries and Docker image; do NOT rely on it for release notes)
- Changelog: `CHANGELOG.md`

### Version Strings (all three must match)
- `cmd/root.go` — `const Version` (source of truth)
- `internal/mcpclient/mcpclient.go` — hardcoded `Version:` in `mcp.NewClient` call
- `cmd/version_test.go` — test assertions for the version string

### GHCR Image
- Package page: https://github.com/jrswab/axe/pkgs/container/axe
- Pull command: `docker pull ghcr.io/jrswab/axe:<version>`
- Tags generated per release: `X.Y.Z`, `X.Y`, `X`, `latest` (on default branch)
