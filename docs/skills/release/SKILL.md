---
name: release
description: Prepare code for release (version bumps, changelog, README updates) and create an annotated tag to trigger the GoReleaser workflow.
---

Your job is to guide the user through a full release for this project. Prepare the code (version bumps, README documentation, changelog), create an annotated `v` tag after approval, and push it to trigger the release workflow. The workflow uses the validated changelog section as release notes and appends the configured Docker footer.

## Rules

- Always create an **annotated** tag (`git tag -a`), never a lightweight tag.
- Tag format is `vMAJOR.MINOR.PATCH` following [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
- The changelog MUST follow [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) format.
- Valid changelog categories: Added, Changed, Deprecated, Removed, Fixed, Security.
- All version strings MUST be updated before committing. There are three locations (listed in Reference). Run version tests after bumping to catch drift.
- README.md MUST document any user-facing changes — new config fields, CLI flags, behavior changes, exit codes.
- DO NOT push to master until the user confirms.
- DO NOT push the tag until the user confirms.
- DO NOT skip asking the user for the version number, tag annotation message, and changelog review.

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

12. **Verify the workflow and release**
    - Confirm admission, reusable checks, package preflight, publication, and Docker publication succeed. Admission requires the tag commit on current master and exactly one nonempty matching changelog section.
    - Check that release notes contain the validated changelog section and configured Docker footer. Do not overwrite the notes after publication.
    - Preflight checks a nonpublishing build, but publication rebuilds packages and does not guarantee identical bytes. Docker builds separately; the workflow does not deploy or roll back.

## Reference

### Project Files
- GoReleaser config: `.goreleaser.yml` (auto-changelog is disabled)
- Release workflow: `.github/workflows/release.yml` (validates changelog notes and checks packages before publishing binaries and a separate Docker image)
- Changelog: `CHANGELOG.md`

### Version Strings (all three must match)
- `cmd/root.go` — `const Version` (source of truth)
- `internal/mcpclient/mcpclient.go` — hardcoded `Version:` in `mcp.NewClient` call
- `cmd/version_test.go` — test assertions for the version string

### GHCR Image
- Package page: https://github.com/jrswab/axe/pkgs/container/axe
- Pull command: `docker pull ghcr.io/jrswab/axe:<version>`
- Tags generated per release: `X.Y.Z`, `X.Y`, `X`, `latest` (on default branch)
