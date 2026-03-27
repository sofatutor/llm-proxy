# Release Process

This document describes how to publish a new release of LLM Proxy, including Docker images, Helm charts, and the GitHub release created for production-ready tags.

## Overview

Releases are automated through GitHub Actions workflows that trigger on git tags:

- **Version tags** (`v*`): Build Docker images and publish Helm charts for a semver tag
- **Stable tags** (`v*-stable`): Run release validation and create a GitHub release with generated release notes

## Creating a Release

### 1. Prepare the Release

Ensure all changes are merged to `main` and tests pass:

```bash
# Update your local main branch
git checkout main
git pull origin main

# Verify all tests pass
make test
make lint
```

### 2. Choose the Correct Tag Type

The repository uses two tag classes:

- `vMAJOR.MINOR.PATCH`: a valid version tag that publishes build artifacts, but does **not** create a GitHub release.
- `vMAJOR.MINOR.PATCH-stable`: a production-eligible stable tag that creates a GitHub release after validation passes.

Tags outside these formats fail the release validation workflow with a clear error.

Examples:

- Valid: `v1.4.0`
- Valid: `v1.4.0-stable`
- Invalid: `1.4.0`
- Invalid: `v1.4`
- Invalid: `v1.4.0stable`
- Invalid: `v1.4.0-rc1`

### 3. Create and Push a Tag

Use semantic versioning (for example `v1.0.0` during release preparation, then `v1.0.0-stable` once the release is approved):

```bash
# Create an annotated tag
VERSION="1.0.0"
git tag -a "v${VERSION}" -m "Release v${VERSION}"

# Push the tag to GitHub
git push origin "v${VERSION}"
```

For the stable release cutover:

```bash
# Promote the validated version tag to a production release
VERSION="1.0.0"
git tag -a "v${VERSION}-stable" -m "Stable release v${VERSION}"
git push origin "v${VERSION}-stable"
```

### 4. Automated Workflows

Pushing a tag triggers automated workflows:

#### Docker Workflow (`.github/workflows/docker.yml`)
- Builds multi-arch images (`linux/amd64`, `linux/arm64`)
- Publishes to `ghcr.io/sofatutor/llm-proxy` with tags:
  - `v1.0.0` (exact version)
  - `1.0` (major.minor)
  - `sha-xxxxxxx` (git commit SHA)
  - `latest` (if from default branch)

#### Helm Chart Workflow (`.github/workflows/helm-publish.yml`)
- Updates `Chart.yaml` version and appVersion to match the git tag
- Runs `helm lint` and validation tests
- Packages the chart
- Publishes to `oci://ghcr.io/sofatutor/llm-proxy` as version `1.0.0`

#### Release Workflow (`.github/workflows/release.yml`)
- Validates that the tag matches either `vMAJOR.MINOR.PATCH` or `vMAJOR.MINOR.PATCH-stable`
- Rejects invalid stable-like tags such as `v1.0-stable`
- Skips GitHub release creation for plain `vMAJOR.MINOR.PATCH` tags
- Runs `make build`, `make lint`, and `make test` before creating a release for `vMAJOR.MINOR.PATCH-stable`
- Creates the GitHub release with generated release notes for stable tags

### 5. Verify the Release

After the workflows complete (check GitHub Actions), verify:

#### Docker Image
```bash
# Pull the image
docker pull ghcr.io/sofatutor/llm-proxy:v1.0.0

# Verify it runs
docker run --rm ghcr.io/sofatutor/llm-proxy:v1.0.0 --version
```

#### Helm Chart

You can view available chart versions in the GitHub Container Registry UI at https://github.com/sofatutor/llm-proxy/pkgs/container/llm-proxy, or verify a specific version exists:

```bash
# Pull the chart
helm pull oci://ghcr.io/sofatutor/llm-proxy --version 1.0.0

# Verify the chart
helm show chart oci://ghcr.io/sofatutor/llm-proxy --version 1.0.0
```

#### GitHub Release

For stable tags, verify that GitHub created the release automatically and generated release notes:

1. Open https://github.com/sofatutor/llm-proxy/releases
2. Confirm there is a release for `v1.0.0-stable`
3. Confirm the release notes were generated and summarize the merged changes

### 6. Release Checklist

Use this checklist for every production release:

1. Merge approved changes to `main`
2. Run `make build`, `make lint`, and `make test` locally
3. Push a version tag `vMAJOR.MINOR.PATCH` and confirm artifact workflows complete
4. Push the corresponding stable tag `vMAJOR.MINOR.PATCH-stable`
5. Confirm `.github/workflows/release.yml` succeeds
6. Confirm the GitHub release exists and includes generated release notes
7. Record the successful stable-tag run and any invalid-tag rejection run in the issue or release notes

## Versioning Strategy

We follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** version (v1.0.0 → v2.0.0): Incompatible API changes
- **MINOR** version (v1.0.0 → v1.1.0): New functionality, backwards-compatible
- **PATCH** version (v1.0.0 → v1.0.1): Bug fixes, backwards-compatible

The stable production contract is layered on top of semver:

- `v1.2.3` means “build and publish artifacts for version `1.2.3`”
- `v1.2.3-stable` means “this exact version is approved for production and must create a GitHub release”

### Helm Chart Versioning

The Helm chart version matches the application version. When you push a tag `v1.2.3`:

- Chart `version` is set to `1.2.3`
- Chart `appVersion` is set to `1.2.3`
- Docker image default is `ghcr.io/sofatutor/llm-proxy:v1.2.3`

## Troubleshooting

### Workflow Fails to Publish

Check the GitHub Actions logs for errors:
- https://github.com/sofatutor/llm-proxy/actions

Common issues:
- Missing `packages: write` permission (should be set in workflow)
- Tag doesn't match the allowed patterns `vMAJOR.MINOR.PATCH` or `vMAJOR.MINOR.PATCH-stable`
- Chart dependencies not building (workflow uses `helm dependency build`)
- Stable tag failed local-equivalent validation (`make build`, `make lint`, `make test`)

### Chart Not Found in GHCR

Charts are published to the organization's GHCR, not the repository:
- Correct: `oci://ghcr.io/sofatutor/llm-proxy`
- Incorrect: `oci://ghcr.io/sofatutor/llm-proxy/llm-proxy`

### Version Mismatch

If the chart version doesn't match the tag:
1. Check the workflow logs to see what version was extracted
2. Ensure tag follows `v1.2.3` or `v1.2.3-stable`
3. Re-run the workflow or delete and recreate the tag

## Rolling Back a Release

If you need to roll back:

1. **Docker**: Users can pull previous versions:
   ```bash
   docker pull ghcr.io/sofatutor/llm-proxy:v1.0.0
   ```

2. **Helm**: Users can install previous chart versions:
   ```bash
   helm install llm-proxy oci://ghcr.io/sofatutor/llm-proxy --version 1.0.0
   ```

3. **Delete the tag** (if release should not exist):
   ```bash
   git tag -d v1.0.1
   git push origin :refs/tags/v1.0.1
   ```
   Note: This doesn't remove already-published artifacts from GHCR.

## Future Improvements

See [docs/issues/backlog/phase-7-release-plan.md](issues/backlog/phase-7-release-plan.md) for follow-up enhancements beyond the stable-tag release contract.
