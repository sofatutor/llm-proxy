import { FullConfig } from '@playwright/test';

/**
 * Global setup for Playwright E2E tests
 * This runs once before all tests
 */
async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting global setup for E2E tests...');
  // Minimal: ensure required env is present in the Node test process
  const envRef: any = (globalThis as any).process?.env || {};
  envRef.MGMT_BASE_URL = envRef.MGMT_BASE_URL || 'http://localhost:8098';
  envRef.ADMIN_BASE_URL = envRef.ADMIN_BASE_URL || 'http://localhost:8099';
  envRef.MANAGEMENT_TOKEN = envRef.MANAGEMENT_TOKEN || 'e2e-management-token';
  
  console.log('✅ Global setup complete');
}

export default globalSetup;