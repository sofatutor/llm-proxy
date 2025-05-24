# Release Workflow: GitHub Releases, Docker, Versioning, and Automation

## Summary
Set up a robust release workflow for the LLM proxy project, including GitHub Releases, Docker image publishing to GitHub Container Registry (GHCR), semantic versioning/tagging, automated CI/CD, and a CLI command to streamline release drafting and operational chores.

```mermaid
flowchart TD
    Dev["Code/Docs/Issue Merged"]
    Bump["Bump Version & Tag (vX.Y.Z)"]
    Push["Push Tag to GitHub"]
    CI["CI/CD Workflow Runs"]
    Build["Build Binaries & Docker Image"]
    Publish["Publish to GHCR & GitHub Releases"]
    Changelog["Generate Changelog from PRs/Issues"]
    Release["Draft/Publish GitHub Release"]
    Notify["Notify Team/Users"]

    Dev --> Bump --> Push --> CI --> Build --> Publish --> Release --> Notify
    CI --> Changelog --> Release
```

## Rationale
- Automated, transparent releases ensure reliability, traceability, and ease of deployment for users.
- Semantic versioning and tagging provide clarity and consistency for all stakeholders.
- Publishing Docker images to GHCR enables easy, secure deployment in any environment.
- A CLI release helper reduces manual errors and streamlines operational tasks.

## Tasks
- [ ] Configure GitHub Actions workflow for building and publishing Docker images to GHCR on every tagged release
- [ ] Set up GitHub Releases to attach binaries, Docker images, and changelogs
- [ ] Enforce semantic versioning for all release tags (e.g., v1.2.3)
- [ ] Automate changelog generation from merged PRs and issues
- [ ] Implement a CLI command (e.g., `llm-proxy release draft`) to:
    - Bump version and create a new tag
    - Generate and preview changelog
    - Push tag and trigger CI/CD
    - Optionally draft a GitHub Release with artifacts
- [ ] Document the release process in `/docs/release.md`
- [ ] Add tests for the CLI release helper and CI/CD workflows

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant CLI as CLI Release Helper
    participant GH as GitHub
    participant CI as CI/CD Workflow
    participant GHCR as GHCR
    participant Rel as GitHub Release
    Dev->>CLI: Run release draft command
    CLI->>GH: Create tag & push
    GH->>CI: Trigger CI/CD on tag
    CI->>CI: Build binaries & Docker image
    CI->>GHCR: Publish Docker image
    CI->>Rel: Attach binaries, changelog
    CI->>Rel: Draft/publish release
    Rel->>Dev: Release available
```

## Acceptance Criteria
- Releases are published via GitHub Releases with semantic version tags
- Docker images are built and published to GHCR for each release
- Changelogs are generated automatically from PRs/issues
- The CLI release command is available and documented
- The release process is fully automated, tested, and documented

```mermaid
flowchart TD
    Start([Start Release])
    Draft["llm-proxy release draft"]
    BumpV["Bump Version"]
    GenLog["Generate Changelog"]
    Preview["Preview Release Notes"]
    Tag["Create & Push Tag"]
    Trigger["Trigger CI/CD"]
    Done([Release Published])

    Start --> Draft --> BumpV --> GenLog --> Preview --> Tag --> Trigger --> Done
``` 