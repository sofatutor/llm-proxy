# Release Workflow: GitHub Releases, Docker, Versioning, and Automation

## Summary
Set up a robust release workflow for the LLM proxy project, including GitHub Releases, Docker image publishing to GitHub Container Registry (GHCR), semantic versioning/tagging, automated CI/CD, and Makefile targets to streamline release drafting and operational chores.

```mermaid
flowchart TD
    Dev["Code/Docs/Issue Merged"]
    Bump["make release-bump (Bump Version & Tag)"]
    Push["Push Tag to GitHub"]
    CI["CI/CD Workflow Runs"]
    Build["Build Binaries & Docker Image"]
    Publish["Publish to GHCR & GitHub Releases"]
    Changelog["make changelog (Generate Changelog from PRs/Issues)"]
    Release["Draft/Publish GitHub Release"]
    Notify["Notify Team/Users"]

    Dev --> Bump --> Push --> CI --> Build --> Publish --> Release --> Notify
    CI --> Changelog --> Release
```

## Rationale
- Automated, transparent releases ensure reliability, traceability, and ease of deployment for users.
- Semantic versioning and tagging provide clarity and consistency for all stakeholders.
- Publishing Docker images to GHCR enables easy, secure deployment in any environment.
- Makefile targets reduce manual errors and streamline operational tasks.

## Tasks
- [ ] Configure GitHub Actions workflow for building and publishing Docker images to GHCR on every tagged release
- [ ] Set up GitHub Releases to attach binaries, Docker images, and changelogs
- [ ] Enforce semantic versioning for all release tags (e.g., v1.2.3)
- [ ] Automate changelog generation from merged PRs and issues (Makefile target)
- [ ] Add Makefile targets for:
    - Version bumping and tagging (`make release-bump`)
    - Changelog generation (`make changelog`)
    - Pushing tags and triggering CI/CD
    - Drafting/publishing GitHub Releases
- [ ] Document the release process in `/docs/release.md`
- [ ] Add tests for Makefile release targets and CI/CD workflows

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant MF as Makefile
    participant GH as GitHub
    participant CI as CI/CD Workflow
    participant GHCR as GHCR
    participant Rel as GitHub Release
    Dev->>MF: make release-bump
    MF->>GH: Create tag & push
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
- Changelogs are generated automatically from PRs/issues (Makefile target)
- Makefile targets for release chores are available and documented
- The release process is fully automated, tested, and documented

```mermaid
flowchart TD
    Start([Start Release])
    Draft["make release-bump"]
    GenLog["make changelog"]
    Tag["Create & Push Tag"]
    Trigger["Trigger CI/CD"]
    Done([Release Published])

    Start --> Draft --> GenLog --> Tag --> Trigger --> Done
``` 