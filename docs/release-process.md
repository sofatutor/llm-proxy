# Release Process

This document describes how to publish a new release of LLM Proxy, including Docker images and Helm charts.

## Overview

Releases are automated through GitHub Actions workflows that trigger on git tags:

- **Docker Images**: Published to `ghcr.io/sofatutor/llm-proxy` on pushes to `main` and tags `v*`
- **Helm Charts**: Published to `oci://ghcr.io/sofatutor/llm-proxy` on tags `v*`

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

### 2. Create and Push a Version Tag

Use semantic versioning (e.g., `v1.0.0`, `v1.2.3`):

```bash
# Create an annotated tag
VERSION="1.0.0"
git tag -a "v${VERSION}" -m "Release v${VERSION}"

# Push the tag to GitHub
git push origin "v${VERSION}"
```

### 3. Automated Workflows

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

### 4. Verify the Release

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

### 5. Create GitHub Release (Optional)

While Docker and Helm publishing is automated, you may want to create a GitHub Release for visibility:

1. Go to https://github.com/sofatutor/llm-proxy/releases/new
2. Select the tag you just pushed
3. Add release notes (what's new, breaking changes, etc.)
4. Publish the release

## Versioning Strategy

We follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** version (v1.0.0 → v2.0.0): Incompatible API changes
- **MINOR** version (v1.0.0 → v1.1.0): New functionality, backwards-compatible
- **PATCH** version (v1.0.0 → v1.0.1): Bug fixes, backwards-compatible

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
- Tag doesn't match `v*` pattern
- Chart dependencies not building (workflow uses `helm dependency build`)

### Chart Not Found in GHCR

Charts are published to the organization's GHCR, not the repository:
- Correct: `oci://ghcr.io/sofatutor/llm-proxy`
- Incorrect: `oci://ghcr.io/sofatutor/llm-proxy/llm-proxy`

### Version Mismatch

If the chart version doesn't match the tag:
1. Check the workflow logs to see what version was extracted
2. Ensure tag follows the `v1.2.3` format (leading `v`, semantic version)
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

See [docs/issues/backlog/phase-7-release-plan.md](issues/backlog/phase-7-release-plan.md) for planned enhancements:
- Automated changelog generation
- GitHub Release creation from CI
- Makefile targets for version bumping
- Release validation tests
