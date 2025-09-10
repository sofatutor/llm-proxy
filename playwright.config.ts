import { defineConfig, devices } from '@playwright/test';

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './e2e/specs',
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: [
    ['html'],
    ['line'],
    process.env.CI ? ['github'] : null
  ].filter(Boolean),
  
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: process.env.ADMIN_BASE_URL || `http://localhost:${process.env.ADMIN_PORT || '8099'}`,

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
    command: 'npm run start:e2e',
    url: process.env.ADMIN_BASE_URL || `http://localhost:${process.env.ADMIN_PORT || '8099'}`,
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});