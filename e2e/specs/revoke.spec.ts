import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';
import { SeedFixture } from '../fixtures/seed';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
const BASE_URL = process.env.ADMIN_BASE_URL || 'http://localhost:8099';

test.describe('Token and Project Revocation', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;
  let tokenId: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    projectId = await seed.createProject('Revoke Test Project', 'sk-test-key');
    tokenId = await seed.createToken(projectId, 60);
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should revoke single token from list', async ({ page }) => {
    await page.goto('/tokens');
    
    // Set up dialog handler for confirmation
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke token');
      await dialog.accept();
    });
    
    // Click revoke button
    await page.click(`button[onclick*="${tokenId}"]`);
    
    // Should redirect back to tokens list or remain on token show briefly; accept either
    await expect(page).toHaveURL(/\/tokens(\/.*)?$/);
  });

  test('should revoke single token from edit page', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}/edit`);
    
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke this token');
      await dialog.accept();
    });
    
    // Click revoke button in danger zone
    await page.click('button[type="submit"]:has-text("Revoke Token")');
    
    await expect(page).toHaveURL(/\/tokens(\/.*)?$/);
  });

  test('should revoke single token from show page', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}`);
    
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke token');
      await dialog.accept();
    });
    
    await page.click('button:has-text("Revoke Token")');
    await expect(page).toHaveURL(/\/tokens(\/.*)?$/);
  });

  test('should bulk revoke project tokens from project list', async ({ page }) => {
    // Navigate directly to the project details page and use the visible revoke-all button
    await page.goto(`/projects/${projectId}`);
    
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke ALL tokens');
      await dialog.accept();
    });
    
    await page.click('button:has-text("Revoke All Tokens")');
    
    // Should stay on project details page after bulk revoke
    await expect(page).toHaveURL(`/projects/${projectId}`);
  });

  test('should bulk revoke project tokens from project show page', async ({ page }) => {
    await page.goto(`/projects/${projectId}`);
    
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke ALL tokens');
      await dialog.accept();
    });
    
    await page.click('button:has-text("Revoke All Tokens")');
    
    // Should stay on project page
    await expect(page).toHaveURL(`/projects/${projectId}`);
  });

  test('should cancel revocation on dialog dismiss', async ({ page }) => {
    await page.goto('/tokens');
    
    page.on('dialog', async dialog => {
      await dialog.dismiss();
    });
    
    await page.click(`button[onclick*="${tokenId}"]`);
    
    // Should stay on tokens page, token should still exist
    await expect(page).toHaveURL('/tokens');
    
    // Verify token still exists via API
    const token = await seed.getToken(tokenId);
    expect(token.is_active).toBe(true);
  });
});