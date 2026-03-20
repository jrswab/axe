# 039 -- Consolidate Docker Release Implementation Guide

Spec: `docs/plans/039_consolidate_docker_release_spec.md`

## Context Summary

Two GitHub Actions workflows currently trigger on `v*` tags: `release.yml` (GoReleaser, binaries) and `docker-publish.yml` (Docker build/push to GHCR). They are decoupled — the Docker image is invisible on the GitHub Releases page and there are two files to maintain for one logical event. The fix is to move the Docker build/push into `release.yml` as a parallel second job (no `needs:` dependency), add a `release.footer` block to `.goreleaser.yml` so the Docker pull command appears on every GitHub Release, and delete `docker-publish.yml`. The existing multi-stage `Dockerfile` is used unchanged. All current Docker behavior is preserved: four semver tag patterns, default-branch guard on `latest`, Buildx with GHA caching, and build provenance attestations.

## Implementation Checklist

### Task 1 — Expand top-level permissions in `.github/workflows/release.yml`

- [x] `.github/workflows/release.yml`: Replace the existing top-level `permissions` block (currently only `contents: write`) with the union of permissions required by both jobs:
  ```yaml
  permissions:
    contents: write      # GoReleaser: create GitHub Release
    packages: write      # Docker: push to GHCR
    attestations: write  # Docker: build provenance attestation
    id-token: write      # Docker: OIDC token for attestation signing
  ```
  The existing `goreleaser` job body is not changed.

### Task 2 — Add the `docker` job to `.github/workflows/release.yml`

- [x] `.github/workflows/release.yml`: Add a new top-level job named `docker` under the `jobs:` key. It must have no `needs:` field (runs in parallel with `goreleaser`). The job body is the exact steps from `.github/workflows/docker-publish.yml`, in order:

  1. **Checkout** — `actions/checkout@v4`
  2. **Log in to GHCR** — `docker/login-action@v4` with:
     - `registry: ghcr.io`
     - `username: ${{ github.actor }}`
     - `password: ${{ secrets.GITHUB_TOKEN }}`
  3. **Extract metadata** — `docker/metadata-action@v6` with:
     - `images: ghcr.io/${{ github.repository }}`
     - `tags:` block containing all four patterns from R2:
       ```
       type=semver,pattern={{version}}
       type=semver,pattern={{major}}.{{minor}}
       type=semver,pattern={{major}}
       type=raw,value=latest,enable={{is_default_branch}}
       ```
  4. **Set up Docker Buildx** — `docker/setup-buildx-action@v4`
  5. **Build and push** — `docker/build-push-action@v7` with:
     - `context: .`
     - `push: true`
     - `tags: ${{ steps.meta.outputs.tags }}`
     - `labels: ${{ steps.meta.outputs.labels }}`
     - `cache-from: type=gha`
     - `cache-to: type=gha,mode=max`
     - Step `id: push`
  6. **Generate artifact attestation** — `actions/attest-build-provenance@v2` with:
     - `subject-name: ghcr.io/${{ github.repository }}`
     - `subject-digest: ${{ steps.push.outputs.digest }}`
     - `push-to-registry: true`

  The `runs-on:` value must be `ubuntu-latest`.

### Task 3 — Add `release.footer` to `.goreleaser.yml`

- [x] `.goreleaser.yml`: Add a `release:` block at the end of the file with a `footer` field. The footer must render a Docker section containing a `docker pull` command using `{{ .Version }}` (no `v` prefix). Example:
  ```yaml
  release:
    footer: |
      ## Docker
      ```
      docker pull ghcr.io/jrswab/axe:{{ .Version }}
      ```
  ```
  Do not modify any other section of `.goreleaser.yml`.

### Task 4 — Delete `.github/workflows/docker-publish.yml`

- [x] Delete the file `.github/workflows/docker-publish.yml` entirely. It must not exist after this task.

### Task 5 — Verify GoReleaser template syntax (manual)

- [x] From the repository root, run:
  ```
  goreleaser release --snapshot --clean --skip=publish
  ```
  Confirm the command exits 0 and the release notes output contains the Docker footer section with a rendered version string (not a raw template literal). If `goreleaser` is not installed locally, this step may be deferred to CI — but the PR description must note it.

### Task 6 — Verify final state of `.github/workflows/release.yml` (manual review)

- [x] Confirm the file contains exactly two jobs: `goreleaser` and `docker`.
- [x] Confirm neither job has a `needs:` field referencing the other.
- [x] Confirm the top-level `permissions` block contains all four entries from Task 1.
- [x] Confirm the `goreleaser` job body is identical to the original (no accidental edits).
- [x] Confirm `.github/workflows/docker-publish.yml` does not exist.
