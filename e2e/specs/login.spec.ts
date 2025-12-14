import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';

test.describe('Admin UI Login', () => {
  test('should successfully login with valid management token', async ({ page }) => {
    const auth = new AuthFixture(page);

    // Navigate to login page
    await page.goto('/auth/login');
    
    // Verify we're on the login page
    await expect(page).toHaveTitle(/Sign In/);
    // Login page uses a styled button with text "Sign In" rather than an <h1>
    await expect(page.locator('button:has-text("Sign In")')).toBeVisible();

    // Fill in the management token
    await page.fill('#management_token', MANAGEMENT_TOKEN);
    
    // Submit the form
    await page.click('button[type="submit"]');

    // Should redirect to dashboard
    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('h1')).toContainText('Dashboard');

    // Admin assets should be cache-busted via ?v=...
    const adminCSS = page.locator('link[rel="stylesheet"][href^="/static/css/admin.css?v="]');
    await expect(adminCSS).toHaveCount(1);
    await expect(adminCSS).toHaveAttribute('href', /\/static\/css\/admin\.css\?v=\d+$/);

    const adminJS = page.locator('script[src^="/static/js/admin.js?v="]');
    await expect(adminJS).toHaveCount(1);
    await expect(adminJS).toHaveAttribute('src', /\/static\/js\/admin\.js\?v=\d+$/);
    
    // Should see navigation menu
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('nav a[href="/projects"]')).toBeVisible();
    await expect(page.locator('nav a[href="/tokens"]')).toBeVisible();
  });

  test('should show error for invalid management token', async ({ page }) => {
    // Navigate to login page
    await page.goto('/auth/login');
    
    // Fill in invalid token
    await page.fill('#management_token', 'invalid-token');
    
    // Submit the form
    await page.click('button[type="submit"]');

    // Should stay on login page with error
    await expect(page).toHaveURL('/auth/login');
    await expect(page.locator('.alert-danger, .error')).toBeVisible();
  });

  test('should require management token', async ({ page }) => {
    // Navigate to login page
    await page.goto('/auth/login');
    
    // Try to submit without token
    await page.click('button[type="submit"]');

    // Should show validation error or stay on page
    const url = page.url();
    expect(url).toContain('/auth/login');
  });

  test('should logout successfully', async ({ page }) => {
    const auth = new AuthFixture(page);

    // Login first
    await auth.login(MANAGEMENT_TOKEN);
    await expect(page).toHaveURL('/dashboard');

    // Logout
    await auth.logout();
    await expect(page).toHaveURL('/auth/login');
  });

  test('should redirect to login when accessing protected pages without auth', async ({ page }) => {
    // Try to access dashboard without logging in
    await page.goto('/dashboard');
    
    // Should redirect to login
    await expect(page).toHaveURL('/auth/login');

    // Try to access projects page
    await page.goto('/projects');
    await expect(page).toHaveURL('/auth/login');

    // Try to access tokens page
    await page.goto('/tokens');
    await expect(page).toHaveURL('/auth/login');
  });
});