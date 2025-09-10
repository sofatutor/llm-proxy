import { FullConfig } from '@playwright/test';

/**
 * Global setup for Playwright E2E tests
 * This runs once before all tests
 */
async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting global setup for E2E tests...');
  
  // Environment variables are already set in playwright.config.ts webServer
  // The server will be started automatically by Playwright's webServer config
  
  console.log('✅ Global setup complete');
}

export default globalSetup;