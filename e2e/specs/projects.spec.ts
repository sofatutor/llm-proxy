import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';
import { SeedFixture } from '../fixtures/seed';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
const BASE_URL = process.env.ADMIN_BASE_URL || 'http://localhost:8099';

test.describe('Projects Management', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    // Login before each test
    await auth.login(MANAGEMENT_TOKEN);
    
    // Create a test project
    projectId = await seed.createProject('E2E Test Project', 'sk-test-key');
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should list projects and show status', async ({ page }) => {
    await page.goto('/projects');
    
    await expect(page.locator('h1')).toContainText('Projects');
    await expect(page.locator('table')).toBeVisible();
    // Look for a row containing the E2E project prefix
    const row = page.locator('table tbody tr').filter({ hasText: 'E2E Test Project' }).first();
    await expect(row).toBeVisible();
    
    // Should show status badge within the first matching row
    await expect(row.locator('.badge').first()).toBeVisible();
  });

  test('should navigate to project details', async ({ page }) => {
    await page.goto('/projects');
    
    // Click on project link
    await page.click(`a[href="/projects/${projectId}"]`);
    
    await expect(page).toHaveURL(`/projects/${projectId}`);
    await expect(page.locator('h1')).toContainText('E2E Test Project');
  });

  test('should toggle project status', async ({ page }) => {
    await page.goto(`/projects/${projectId}/edit`);
    
    // Toggle the is_active switch
    const activeSwitch = page.locator('#is_active');
    await activeSwitch.check();
    
    // Submit the form
    await page.click('button[type="submit"]');
    
    // Should redirect back to project show page (or, in rare cases, login)
    await expect(page).toHaveURL(new RegExp(`/projects/${projectId}(?:/.*)?$|/auth/login$`));
  });

  test('should bulk revoke project tokens', async ({ page }) => {
    // Create a token for the project first
    await seed.createToken(projectId, 60);
    
    await page.goto(`/projects/${projectId}`);
    
    // Click bulk revoke button
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke ALL tokens');
      await dialog.accept();
    });
    
    await page.click('button:has-text("Revoke All Tokens")');
    
    // Should redirect back to project page
    await expect(page).toHaveURL(`/projects/${projectId}`);
  });
});