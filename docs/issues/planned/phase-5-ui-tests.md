# UI and Integration Tests

Tracking: [Issue #198](https://github.com/sofatutor/llm-proxy/issues/198) (follow-up to [Issue #45](https://github.com/sofatutor/llm-proxy/issues/45))

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
- [x] Add Playwright E2E tests for critical user flows
- [ ] Set up Playwright to run in GitHub Actions with a matrix for Chromium, Firefox, and WebKit
- [ ] Maintain and improve test coverage reporting
- [ ] Document UI, integration, and E2E testing best practices

## Acceptance Criteria
- UI and integration tests are DRY, efficient, and comprehensive
- Playwright E2E tests cover critical user flows and run in CI across all major browsers
- Coverage remains above 90% at all times
- Documentation and tests are updated accordingly 

## Recorded Playwright Action Log (for later test authoring)

The following captures the exact UI actions executed during manual Playwright-driven exploration. Use these as seed steps for writing deterministic Playwright tests. Replace secrets with environment variables before committing test code.

```json
[
  { "step": 1, "action": "navigate", "url": "http://localhost:8081/auth/login" },
  { "step": 2, "action": "fill", "selector": "#management_token", "value": "${MANAGEMENT_TOKEN}" },
  { "step": 3, "action": "click", "selector": "button[type=submit]" },
  { "step": 4, "action": "screenshot", "name": "dashboard", "fullPage": true },
  { "step": 5, "action": "click", "selector": "a[href='/projects']" },
  { "step": 6, "action": "screenshot", "name": "projects", "fullPage": true },
  { "step": 7, "action": "click", "selector": "a[href='/tokens']" },
  { "step": 8, "action": "screenshot", "name": "tokens", "fullPage": true },
  { "step": 9, "action": "click", "selector": "a[href='/audit']" },
  { "step": 10, "action": "screenshot", "name": "audit", "fullPage": true },

  { "step": 11, "action": "navigate", "url": "http://localhost:8081/auth/logout" },
  { "step": 12, "action": "navigate", "url": "http://localhost:8081/auth/login" },
  { "step": 13, "action": "screenshot", "name": "login", "fullPage": true },

  { "step": 14, "action": "navigate", "url": "http://localhost:8081/auth/login" },
  { "step": 15, "action": "fill", "selector": "#management_token", "value": "${MANAGEMENT_TOKEN}" },
  { "step": 16, "action": "click", "selector": "button[type=submit]" },
  { "step": 17, "action": "click", "selector": "a[href='/projects']" },
  { "step": 18, "action": "click", "selector": "a[href='/projects/new']" },
  { "step": 19, "action": "screenshot", "name": "project-new", "fullPage": true },
  { "step": 20, "action": "fill", "selector": "#name", "value": "Demo Project" },
  { "step": 21, "action": "fill", "selector": "#openai_api_key", "value": "sk-test-1234567890" },
  { "step": 22, "action": "screenshot", "name": "project-new-filled", "fullPage": true },
  { "step": 23, "action": "click", "selector": "form[action='/projects'] button[type=submit]" },
  { "step": 24, "action": "screenshot", "name": "project-show-after-create", "fullPage": true },

  { "step": 25, "action": "click", "selector": "a[href^='/tokens/new']" },
  { "step": 26, "action": "screenshot", "name": "token-new", "fullPage": true },
  { "step": 27, "action": "select", "selector": "#duration_minutes", "value": "60" },
  { "step": 28, "action": "click", "selector": "form[action='/tokens'] button[type=submit]" },
  { "step": 29, "action": "screenshot", "name": "token-created", "fullPage": true },

  { "step": 30, "action": "click", "selector": "a[href='/audit']" },
  { "step": 31, "action": "click", "selector": "a[href^='/audit/']" },
  { "step": 32, "action": "screenshot", "name": "audit-show", "fullPage": true }
]
```

### Playwright test skeletons mapping to the action log

```ts
import { test, expect } from '@playwright/test';

const ADMIN_BASE = process.env.ADMIN_BASE_URL ?? 'http://localhost:8081';
const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN ?? '';

async function login(page) {
  await page.goto(`${ADMIN_BASE}/auth/login`);
  await page.fill('#management_token', MANAGEMENT_TOKEN);
  await page.click('button[type="submit"]');
}

test.describe('Admin UI smoke flows', () => {
  test('login and core navigation', async ({ page }) => {
    await login(page);
    await page.screenshot({ path: 'artifacts/dashboard.png', fullPage: true });

    await page.click('a[href="/projects"]');
    await page.screenshot({ path: 'artifacts/projects.png', fullPage: true });

    await page.click('a[href="/tokens"]');
    await page.screenshot({ path: 'artifacts/tokens.png', fullPage: true });

    await page.click('a[href="/audit"]');
    await page.screenshot({ path: 'artifacts/audit.png', fullPage: true });
  });

  test('project create and show', async ({ page }) => {
    await login(page);
    await page.click('a[href="/projects"]');
    await page.click('a[href="/projects/new"]');
    await page.screenshot({ path: 'artifacts/project-new.png', fullPage: true });

    await page.fill('#name', 'Demo Project');
    await page.fill('#openai_api_key', 'sk-test-1234567890'); // Use env or fixture in CI
    await page.screenshot({ path: 'artifacts/project-new-filled.png', fullPage: true });
    await page.click('form[action="/projects"] button[type="submit"]');
    await page.screenshot({ path: 'artifacts/project-show.png', fullPage: true });
  });

  test('token generate and created view', async ({ page }) => {
    await login(page);
    // Navigate to any project show first if needed; this assumes a project exists
    await page.click('a[href^="/tokens/new"]');
    await page.screenshot({ path: 'artifacts/token-new.png', fullPage: true });
    await page.selectOption('#duration_minutes', '60');
    await page.click('form[action="/tokens"] button[type="submit"]');
    await page.screenshot({ path: 'artifacts/token-created.png', fullPage: true });
  });

  test('audit detail view', async ({ page }) => {
    await login(page);
    await page.click('a[href="/audit"]');
    // Click first audit entry if present
    await page.click('a[href^="/audit/"]');
    await page.screenshot({ path: 'artifacts/audit-show.png', fullPage: true });
  });
});
```

Notes:
- Do not commit real tokens; prefer `MANAGEMENT_TOKEN` via CI secrets.
- Consider seeding a project via API before E2E to avoid order dependency.
- Store screenshots under a CI artifacts dir (not under `docs/`).
