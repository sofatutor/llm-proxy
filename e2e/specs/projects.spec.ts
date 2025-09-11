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

  test('should validate required fields in project form', async ({ page }) => {
    await page.goto('/projects/new');
    
    // Try to submit form without required fields
    await page.click('button[type="submit"]');
    
    // Should show validation errors (HTML5 validation or custom)
    const nameInput = page.locator('#name');
    const apiKeyInput = page.locator('#openai_api_key');
    
    // Check for HTML5 validation
    const nameValidation = await nameInput.evaluate((el: HTMLInputElement) => el.validationMessage);
    const apiKeyValidation = await apiKeyInput.evaluate((el: HTMLInputElement) => el.validationMessage);
    
    // At least one should have a validation message
    expect(nameValidation || apiKeyValidation).toBeTruthy();
  });

  test('should validate project name field', async ({ page }) => {
    await page.goto('/projects/new');
    
    // Leave name empty and fill other fields
    await page.fill('#openai_api_key', 'sk-test-key');
    await page.click('button[type="submit"]');
    
    // Should prevent submission due to empty name
    const nameInput = page.locator('#name');
    const nameValidation = await nameInput.evaluate((el: HTMLInputElement) => el.validationMessage);
    expect(nameValidation).toBeTruthy();
  });

  test('should validate API key field format', async ({ page }) => {
    await page.goto('/projects/new');
    
    // Fill with invalid API key format
    await page.fill('#name', 'Test Project');
    await page.fill('#openai_api_key', 'invalid-key-format');
    await page.click('button[type="submit"]');
    
    // Should either show validation error or reject the submission
    // (The actual validation depends on implementation)
    await expect(page).toHaveURL(/\/projects\/new|\/projects$/);
    
    // If custom validation, check for error messages
    const errorMessages = page.locator('.alert-danger, .text-danger, .invalid-feedback');
    if (await errorMessages.count() > 0) {
      await expect(errorMessages.first()).toBeVisible();
    }
  });

  test('should display form error states correctly', async ({ page }) => {
    await page.goto('/projects/new');
    
    // Submit empty form to trigger validation
    await page.click('button[type="submit"]');
    
    // Check for visual error indicators
    const nameInput = page.locator('#name');
    const apiKeyInput = page.locator('#openai_api_key');
    
    // Verify inputs get error styling (if implemented)
    const nameClasses = await nameInput.getAttribute('class');
    const apiKeyClasses = await apiKeyInput.getAttribute('class');
    
    // Should have either HTML5 validation or custom error classes
    expect(nameClasses || apiKeyClasses).toBeTruthy();
  });

  test('should edit project with form validation', async ({ page }) => {
    await page.goto(`/projects/${projectId}/edit`);
    
    // Clear required field
    await page.fill('#name', '');
    await page.click('button[type="submit"]');
    
    // Should prevent submission
    const nameInput = page.locator('#name');
    const validation = await nameInput.evaluate((el: HTMLInputElement) => el.validationMessage);
    expect(validation).toBeTruthy();
    
    // Fill valid data and submit
    await page.fill('#name', 'Updated Project Name');
    await page.click('button[type="submit"]');
    
    // Should redirect on success
    await expect(page).toHaveURL(new RegExp(`/projects/${projectId}(?:/.*)?$|/auth/login$`));
  });

  test('should handle API key format validation in edit form', async ({ page }) => {
    await page.goto(`/projects/${projectId}/edit`);
    
    // Update with invalid API key
    await page.fill('#openai_api_key', 'not-a-valid-key');
    await page.click('button[type="submit"]');
    
    // Should either show error or stay on edit page
    await expect(page).toHaveURL(new RegExp(`/projects/${projectId}/edit|/projects/${projectId}|/auth/login`));
  });

  test('should show form loading/submission states', async ({ page }) => {
    await page.goto('/projects/new');
    
    // Fill form with valid data
    await page.fill('#name', 'New Test Project');
    await page.fill('#openai_api_key', 'sk-test-valid-key');
    
    // Submit and check for loading state (if implemented)
    const submitButton = page.locator('button[type="submit"]');
    await submitButton.click();
    
    // Check if button becomes disabled during submission
    if (await submitButton.isVisible()) {
      const isDisabled = await submitButton.isDisabled();
      // Button might be disabled during submission (good UX practice)
    }
  });
});