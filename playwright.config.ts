import { defineConfig, devices } from '@playwright/test';

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './e2e/specs',
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!(globalThis as any).process?.env?.CI,
  /* Retry on CI only */
  retries: ((globalThis as any).process?.env?.CI ? 2 : 0) as number,
  /* Opt out of parallel tests on CI. */
  workers: ((globalThis as any).process?.env?.CI ? 1 : undefined) as number | undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: [
    ['html'],
    ['line'],
    ...(((globalThis as any).process?.env?.CI ? [['github']] : []) as any[])
  ],
  
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    // Use explicit E2E admin URL; wrapper starts admin on 8099
    baseURL: 'http://localhost:8099',
    // Ensure tests themselves use the same management token and base URL as the spawned servers
    // @ts-expect-error Playwright supports use.env at runtime; typing may lag
    env: {
      // Keep token consistent with wrapper default
      MANAGEMENT_TOKEN: (((globalThis as any).process?.env?.MANAGEMENT_TOKEN) || 'e2e-management-token') as string,
      ADMIN_BASE_URL: 'http://localhost:8099' as string,
      MGMT_BASE_URL: 'http://localhost:8098' as string,
    },

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
    /* Take screenshot only on failures */
    screenshot: 'only-on-failure',
    /* Record video only on failures */
    video: 'retain-on-failure',
  },

  /* Configure global setup and teardown */
  globalSetup: './e2e/fixtures/global-setup.ts',
  globalTeardown: './e2e/fixtures/global-teardown.ts',

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },

    // Uncomment to test on other browsers
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },

    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],

  /* Run your local dev server before starting the tests */
  webServer: {
    // Force a known MANAGEMENT_TOKEN for both Admin UI and Management API during E2E
    // to avoid mismatches with developer local .env
    command: `MANAGEMENT_TOKEN=${(globalThis as any).process?.env?.MANAGEMENT_TOKEN || 'e2e-management-token'} npm run start:e2e`,
    url: 'http://localhost:8099' as string,
    // Always start fresh to avoid reusing a stray dev server
    reuseExistingServer: false,
    timeout: 30000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});