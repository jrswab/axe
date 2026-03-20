# 039 -- Consolidate Docker Release Spec

GitHub Issue: https://github.com/jrswab/axe/issues/36

## Context & Constraints

### Problem

The repository has two separate GitHub Actions workflows that both trigger on `v*` tags:

- `.github/workflows/release.yml` — runs GoReleaser, creates the GitHub Release with binaries.
- `.github/workflows/docker-publish.yml` — builds and pushes the Docker image to `ghcr.io/jrswab/axe`.

These are decoupled: the GHCR image is not surfaced on the GitHub Releases page, and there are two workflows to maintain for a single release event.

### Decisions Made

1. **Do not use GoReleaser's `dockers:` block.** GoReleaser's Docker support requires a simplified Dockerfile that copies a pre-built binary. The existing `Dockerfile` is a multi-stage build (compiles Go inside Docker) and is also used by `docker-compose.yml` for local development. Introducing a second `Dockerfile.goreleaser` adds maintenance burden for no clear gain.

2. **Move Docker build/push into `release.yml` as a second job.** The `docker-publish.yml` logic becomes a `docker` job in `release.yml`, running in parallel with the existing `goreleaser` job. Both jobs trigger on the same `v*` tag. This consolidates into a single workflow without changing the Docker build process.

3. **Add a `release.footer` to `.goreleaser.yml`.** This appends a Docker pull command to every GitHub Release description, surfacing the Docker image on the Releases page without additional workflow steps.

4. **Preserve all existing Docker behavior.** Build provenance attestations, multi-tag strategy, Buildx with GHA caching, and the existing multi-stage Dockerfile all remain unchanged.

### Approaches Ruled Out

- **GoReleaser `dockers:` block** — Requires a separate slim Dockerfile, changes the Docker build process, and doesn't preserve attestations. More complexity for the same outcome.
- **`gh release edit` post-step** — Adding Docker info to the release body via CLI after both jobs complete. More moving parts than `release.footer` and introduces a dependency between jobs.

### Constraints

- The `goreleaser` job and `docker` job have no dependency on each other. They run in parallel. The Docker job builds from source via the multi-stage Dockerfile, not from GoReleaser artifacts.
- The `release.footer` in `.goreleaser.yml` uses GoReleaser template syntax (`{{ .Version }}`), which strips the `v` prefix from the tag. The Docker image tag `{{version}}` from `docker/metadata-action` also strips the `v` prefix. These must match.
- Top-level `permissions` in the workflow must be the union of what both jobs need. Per-job `permissions` blocks can restrict further if desired.

### Files Affected

| File | Action |
|---|---|
| `.github/workflows/release.yml` | Modify — add Docker job, expand permissions |
| `.goreleaser.yml` | Modify — add `release.footer` |
| `.github/workflows/docker-publish.yml` | Delete |

### Files NOT Affected

| File | Reason |
|---|---|
| `Dockerfile` | Unchanged — used as-is by the new Docker job |
| `docker-compose.yml` | Unchanged — local development only |
| `.dockerignore` | Unchanged |

---

## Requirements

### R1: Single Workflow Trigger

Pushing a `v*` tag to the repository must trigger exactly one workflow (`release.yml`) that produces both:
- GitHub Release with binary archives (via GoReleaser)
- Docker image pushed to GHCR (via Docker build/push)

No other workflow may trigger on `v*` tags after this change.

### R2: Docker Image Tags

The Docker image must be pushed to `ghcr.io/jrswab/axe` with the following tags on each release:

| Tag Pattern | Example for v1.2.3 | Condition |
|---|---|---|
| `type=semver,pattern={{version}}` | `1.2.3` | Always |
| `type=semver,pattern={{major}}.{{minor}}` | `1.2` | Always |
| `type=semver,pattern={{major}}` | `1` | Always |
| `type=raw,value=latest` | `latest` | Only when the tag is on the default branch |

These match the current `docker-publish.yml` behavior exactly.

### R3: Docker Build Process

The Docker build must:
- Use the existing multi-stage `Dockerfile` at the repository root.
- Use Docker Buildx.
- Use GitHub Actions cache (`type=gha`) for layer caching.
- Build and push in a single step.

### R4: Build Provenance Attestation

Each Docker image push must generate a build provenance attestation using `actions/attest-build-provenance`. The attestation must:
- Reference the image by `subject-name` (`ghcr.io/jrswab/axe`) and `subject-digest` (from the build step output).
- Be pushed to the registry (`push-to-registry: true`).

### R5: GHCR Authentication

The workflow must authenticate to GHCR before pushing. Authentication uses:
- Registry: `ghcr.io`
- Username: `github.actor`
- Password: `secrets.GITHUB_TOKEN`

### R6: Workflow Permissions

The workflow must declare permissions sufficient for both jobs:
- `contents: write` — GoReleaser creates the GitHub Release.
- `packages: write` — Docker push to GHCR.
- `attestations: write` — Build provenance attestation.
- `id-token: write` — OIDC token for attestation signing.

### R7: Job Independence

The `goreleaser` and `docker` jobs must not depend on each other. They run in parallel. If one fails, the other still completes (or fails independently). There is no `needs:` relationship between them.

### R8: Release Footer

Every GitHub Release created by GoReleaser must include a Docker section at the bottom of the release body. The footer must contain a `docker pull` command with the image name and version tag.

The version in the footer must match the Docker image's `{{version}}` tag (no `v` prefix). For example, for tag `v1.2.3`, the footer shows `ghcr.io/jrswab/axe:1.2.3`.

### R9: Delete docker-publish.yml

The file `.github/workflows/docker-publish.yml` must be deleted. It must not exist after this change.

---

## Edge Cases

### E1: GoReleaser Job Fails, Docker Job Succeeds

The Docker image is pushed to GHCR but no GitHub Release is created. The release footer does not appear (since GoReleaser didn't run). The Docker image is still usable via its GHCR tags. This is acceptable — the user can re-run the workflow or create the release manually.

### E2: Docker Job Fails, GoReleaser Job Succeeds

The GitHub Release is created with binaries and the Docker footer, but the Docker image is not pushed. The footer's `docker pull` command will reference an image that doesn't exist for this version. This is a partial failure — the user sees the release but the Docker image is missing. This is acceptable and matches the current decoupled behavior (where either workflow could fail independently).

### E3: Pre-release Tags on Non-default Branches

If a `v*` tag is pushed from a branch that is not the default branch, the `latest` tag is not applied to the Docker image (guarded by `enable={{is_default_branch}}`). The version-specific tags (`1.2.3`, `1.2`, `1`) are still pushed. The GoReleaser release footer still appears.

### E4: Tag Deletion and Re-push

If a `v*` tag is deleted and re-pushed, both jobs run again. GoReleaser may fail if the release already exists (depending on its `--clean` behavior). The Docker job will overwrite the existing image tags. No special handling is needed.

### E5: Concurrent Workflow Runs

If two `v*` tags are pushed in rapid succession, two workflow runs execute concurrently. Each run's Docker job pushes its own version tags. The `latest` tag may be overwritten by whichever run finishes last. This is standard Docker registry behavior and is acceptable.

### E6: release.footer Template Rendering

GoReleaser renders the footer template at release time. If the template syntax is invalid, GoReleaser will fail the entire release. The template must be validated by testing with `goreleaser release --snapshot --skip=publish` before merging.
