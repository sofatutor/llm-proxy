# UI and Integration Tests

Tracking: [Issue #45](https://github.com/sofatutor/llm-proxy/issues/45)

## Summary
Some integration tests are already present due to ongoing TDD. This issue remains open to further DRY up, refactor, and ensure comprehensive and efficient coverage of all UI and integration flows, keeping coverage above 90% at all times. In addition, Playwright E2E testing will be added, with tests running in GitHub Actions using a matrix for different browsers (e.g., Chromium, Firefox, WebKit).

## Rationale
- TDD has ensured a strong baseline, but further refactoring can improve test efficiency and maintainability.
- DRYing up and consolidating tests reduces duplication and improves reliability.
- Playwright E2E tests provide robust, cross-browser coverage and catch UI regressions.
- Running E2E tests in CI across multiple browsers ensures reliability for all users.
- Coverage must remain above 90% at all times.

## Tasks
- [ ] Review and refactor existing UI and integration tests for DRYness and efficiency
- [ ] Ensure all UI and integration flows and edge cases are covered
- [ ] Add Playwright E2E tests for critical user flows
- [ ] Set up Playwright to run in GitHub Actions with a matrix for Chromium, Firefox, and WebKit
- [ ] Maintain and improve test coverage reporting
- [ ] Document UI, integration, and E2E testing best practices

## Acceptance Criteria
- UI and integration tests are DRY, efficient, and comprehensive
- Playwright E2E tests cover critical user flows and run in CI across all major browsers
- Coverage remains above 90% at all times
- Documentation and tests are updated accordingly 