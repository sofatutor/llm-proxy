import { FullConfig } from '@playwright/test';

/**
 * Global teardown for Playwright E2E tests
 * This runs once after all tests
 */
async function globalTeardown(config: FullConfig) {
  console.log('🧹 Starting global teardown for E2E tests...');
  
  // Cleanup is handled by the webServer teardown in playwright.config.ts
  // and individual test cleanup in each test file
  
  console.log('✅ Global teardown complete');
}

export default globalTeardown;