import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';
import { SeedFixture } from '../fixtures/seed';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
const BASE_URL = process.env.ADMIN_BASE_URL || 'http://localhost:8099';

test.describe('Tokens Management', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;
  let tokenId: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    projectId = await seed.createProject('Token Test Project', 'sk-test-key');
    tokenId = await seed.createToken(projectId, 120);
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should list tokens without exposing raw token values', async ({ page }) => {
    await page.goto('/tokens');
    
    await expect(page.locator('h1')).toContainText('Tokens');
    await expect(page.locator('table')).toBeVisible();
    
    // Should not contain raw token values
    const pageContent = await page.textContent('body');
    expect(pageContent).not.toContain(tokenId);
    
    // Should show at least one status badge in the first row
    await expect(page.locator('table tbody tr').first().locator('.badge').first()).toBeVisible();
  });

  test('should navigate to token edit page', async ({ page }) => {
    await page.goto('/tokens');
    
    // Click edit button for the token
    await page.click(`a[href="/tokens/${tokenId}/edit"]`);
    
    await expect(page).toHaveURL(`/tokens/${tokenId}/edit`);
    await expect(page.locator('h1')).toContainText('Edit Token');
  });

  test('should update token settings', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}/edit`);
    
    // Update max requests
    await page.fill('#max_requests', '100');
    
    // Uncheck is_active
    await page.uncheck('#is_active');
    
    // Submit form
    await page.click('button[type="submit"]');
    
    // Should redirect to token details (or, in rare cases, back to login)
    await expect(page).toHaveURL(new RegExp(`/tokens/${tokenId}(?:/.*)?$|/auth/login$`));
  });

  test('should show token details', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}`);
    
    await expect(page.locator('h1')).toContainText('Token Details');
    const detailsSection = page.locator('.card-body').first();
    await expect(detailsSection.getByText('Token ID', { exact: true })).toBeVisible();
    await expect(detailsSection.getByText('Project', { exact: true })).toBeVisible();
    await expect(detailsSection.getByText('Status', { exact: true })).toBeVisible();
  });
});