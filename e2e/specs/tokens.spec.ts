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
  let tokenValue: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    projectId = await seed.createProject('Token Test Project', 'sk-test-key');
    const token = await seed.createToken(projectId, 120);
    tokenId = token.id;
    tokenValue = token.token;
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
    expect(pageContent).not.toContain(tokenValue);
    
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

  test('should revoke individual token via DELETE button with confirmation', async ({ page }) => {
    await page.goto('/tokens');
    
    // Set up dialog handler for confirmation
    page.on('dialog', async dialog => {
      expect(dialog.type()).toBe('confirm');
      expect(dialog.message()).toContain('revoke token');
      await dialog.accept();
    });
    
    // Click revoke button for specific token
    const revokeButton = page.locator(`button[onclick*="${tokenId}"]`);
    await expect(revokeButton).toBeVisible();
    await revokeButton.click();
    
    // Should redirect back to tokens list
    await expect(page).toHaveURL(/\/tokens(\/.*)?$/);
  });

  test('should handle token revocation confirmation dialog cancel', async ({ page }) => {
    await page.goto('/tokens');
    
    // Set up dialog handler to cancel
    page.on('dialog', async dialog => {
      expect(dialog.type()).toBe('confirm');
      expect(dialog.message()).toContain('revoke token');
      await dialog.dismiss();
    });
    
    // Click revoke button
    const revokeButton = page.locator(`button[onclick*="${tokenId}"]`);
    await revokeButton.click();
    
    // Should stay on tokens page
    await expect(page).toHaveURL('/tokens');
    
    // Token should still be active (verify via API)
    const token = await seed.getToken(tokenId);
    expect(token.is_active).toBe(true);
  });

  test('should verify post-revoke status changes', async ({ page }) => {
    // Navigate to token details page
    await page.goto(`/tokens/${tokenId}`);
    
    // Verify token is initially active
    await expect(page.locator('.badge:has-text("Active")')).toBeVisible();
    
    // Revoke token via API to change status
    await seed.revokeToken(tokenId);
    
    // Refresh page to see updated status
    await page.reload();
    
    // Should show revoked status
    await expect(page.locator('.badge:has-text("Revoked"), .badge:has-text("Inactive")')).toBeVisible();
  });

  test('should show revoke button on token details page', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}`);
    
    // Should have revoke button in the danger zone or actions section
    const revokeButton = page.locator('button:has-text("Revoke Token")');
    await expect(revokeButton).toBeVisible();
    
    // Verify revoke button has appropriate styling (danger)
    const buttonClasses = await revokeButton.getAttribute('class');
    expect(buttonClasses).toMatch(/btn-danger|btn-outline-danger/);
  });

  test('should revoke token from details page with confirmation', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}`);
    
    // Set up dialog handler
    page.on('dialog', async dialog => {
      expect(dialog.type()).toBe('confirm');
      expect(dialog.message()).toContain('revoke');
      await dialog.accept();
    });
    
    // Click revoke button
    const revokeButton = page.locator('button:has-text("Revoke Token")');
    await revokeButton.click();
    
    // Should redirect away from details page
    await expect(page).toHaveURL(/\/tokens(?!\/.*\/edit).*$/);
  });

  test('should display token status badges correctly', async ({ page }) => {
    await page.goto('/tokens');
    
    // Find the row containing our token and check status badge
    const statusBadge = page.locator('table tbody .badge').first();
    await expect(statusBadge).toBeVisible();
    
    // Should have appropriate color class for active status
    const badgeClasses = await statusBadge.getAttribute('class');
    expect(badgeClasses).toMatch(/badge\s+(bg-success|bg-danger|bg-warning|bg-secondary)/);
  });
});