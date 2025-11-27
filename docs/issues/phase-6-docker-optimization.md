# Docker Optimization

Tracking: [Issue #47](https://github.com/sofatutor/llm-proxy/issues/47)

## Summary
Optimize the Dockerfile and containerization setup for the LLM proxy. This includes creating a multi-stage Dockerfile, improving volume configuration, adding container health checks, and implementing security best practices. This issue can be worked on in parallel with other deployment and documentation issues.

## Rationale
- Optimized Docker images improve build speed, security, and runtime performance.
- Proper volume and health check configuration is essential for production deployments.

## Tasks
- [x] Create a multi-stage Dockerfile for the LLM proxy
- [x] Improve volume configuration for data, logs, and configuration
- [x] Add container health checks
- [x] Implement Docker security best practices (non-root user, minimal image, etc.)
- [x] Document Docker build and deployment process
- [x] Add tests for Docker builds and health checks (CI smoke test in `.github/workflows/docker.yml`)
- [x] Build and publish Docker image to GitHub Container Registry (GHCR)
- [x] Add Trivy security scanning to CI pipeline and Makefile

## Acceptance Criteria
- Dockerfile is multi-stage and optimized
- Volumes and health checks are properly configured
- Security best practices are followed
- Security scanning (Trivy) integrated in CI
- Documentation and tests are updated accordingly 
- CI builds and publishes multi-arch images to `ghcr.io/sofatutor/llm-proxy`

## Notes
- Removed unnecessary `redis` package from runtime image; Redis runs as a separate service.
- Added `.dockerignore` to reduce build context.
- Makefile targets: `docker-build`, `docker-run`, `docker-smoke`, `docker-scan` for local validation.
- Added GitHub Actions workflow `.github/workflows/docker.yml` to build multi-arch image and push to GHCR on `main` and tags.
- Integrated Trivy security scanning in CI pipeline with results uploaded to GitHub Security tab.
- Enhanced Dockerfile with additional security build flags.