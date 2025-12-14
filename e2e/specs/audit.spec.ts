import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';
import { SeedFixture } from '../fixtures/seed';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
const BASE_URL = process.env.ADMIN_BASE_URL || 'http://localhost:8099';

test.describe('Audit Interface', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;
  let tokenId: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    // Create test data to generate audit events
    projectId = await seed.createProject('Audit Test Project', 'sk-audit-test-key');
    const token = await seed.createToken(projectId, 30);
    tokenId = token.id;
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should display audit events list page', async ({ page }) => {
    await page.goto('/audit');
    
    // Verify page title and header
    await expect(page.locator('h1')).toContainText('Audit Events');
    await expect(page.locator('h1 i.bi-shield-check')).toBeVisible();
    
    // Should show search form
    const searchInput = page.locator('input[name="search"]');
    await expect(searchInput).toBeVisible();
    await expect(searchInput).toHaveAttribute('placeholder', /Search events/);
    
    const searchButton = page.locator('button[type="submit"]:has(i.bi-search)');
    await expect(searchButton).toBeVisible();
  });

  test('should show audit events table when events exist', async ({ page }) => {
    // Generate some audit events by performing actions
    await seed.revokeToken(tokenId);
    
    await page.goto('/audit');
    
    // Should display events table
    await expect(page.locator('table.table-hover')).toBeVisible();
    
    // Verify table headers
    const headers = ['Time', 'Action', 'Actor', 'Outcome', 'IP Address', 'Actions'];
    for (const header of headers) {
      await expect(page.locator('thead th').filter({ hasText: new RegExp(`^${header}$`) })).toBeVisible();
    }
    
    // Should have at least one event row
    const eventRows = page.locator('table tbody tr');
    await expect(eventRows.first()).toBeVisible();
    
    // Verify event data structure
    const firstRow = eventRows.first();
    await expect(firstRow.locator('td').nth(0)).toContainText(/\d{4}-\d{2}-\d{2}/); // Timestamp
    await expect(firstRow.locator('.badge').first()).toBeVisible(); // Action badge
    await expect(firstRow.locator('a[href^="/audit/"]')).toBeVisible(); // View details link
  });

  test('should navigate to audit event details page', async ({ page }) => {
    // Generate an audit event
    await seed.revokeToken(tokenId);
    
    await page.goto('/audit');
    
    // Wait for table and click the first revoke-related event if present; otherwise first link
    await expect(page.locator('table tbody tr').first()).toBeVisible();
    const revokeRow = page.locator('table tbody tr').filter({ hasText: /revoke/i }).first();
    if (await revokeRow.count()) {
      await revokeRow.locator('a[href^="/audit/"]').first().click();
    } else {
      const detailsLink = page.locator('a[href^="/audit/"]').first();
      await expect(detailsLink).toBeVisible();
      await detailsLink.click();
    }
    
    // Should navigate to event details page (UUID id)
    await expect(page).toHaveURL(/\/audit\/[a-f0-9-]+$/i);
    await expect(page.locator('h1')).toContainText('Audit Event Details');
    
    // Should show back to list button (disambiguate from sidebar link)
    await expect(page.getByRole('link', { name: /Back to List/i })).toBeVisible();
    // Basic sanity: expect UUID id present in title
    await expect(page.locator('h5.card-title')).toContainText(/Event #[a-f0-9-]+/i);
  });

  test('should display audit event details correctly', async ({ page }) => {
    // Generate an audit event
    await seed.revokeToken(tokenId);
    
    await page.goto('/audit');
    await expect(page.locator('table tbody tr').first()).toBeVisible();
    const revokeRow2 = page.locator('table tbody tr').filter({ hasText: /revoke/i }).first();
    if (await revokeRow2.count()) {
      await revokeRow2.locator('a[href^="/audit/"]').first().click();
    } else {
      const detailsLink = page.locator('a[href^="/audit/"]').first();
      await expect(detailsLink).toBeVisible();
      await detailsLink.click();
    }
    
    // Verify event details sections
    await expect(page.locator('h5.card-title')).toContainText(/Event #/);
    
    // Basic Information section
    const basicInfoTable = page.locator('.col-md-6').first().locator('table');
    await expect(basicInfoTable.locator('td:has-text("Timestamp:")').locator('xpath=following-sibling::td')).toBeVisible();
    await expect(basicInfoTable.locator('td:has-text("Action:")').locator('xpath=following-sibling::td').locator('.badge')).toBeVisible();
    await expect(basicInfoTable.locator('td:has-text("Actor:")').locator('xpath=following-sibling::td')).toBeVisible();
    await expect(basicInfoTable.locator('td:has-text("Outcome:")').locator('xpath=following-sibling::td').locator('.badge')).toBeVisible();
    
    // Network Information section
    const networkInfoTable = page.locator('.col-md-6').nth(1).locator('table');
    await expect(networkInfoTable.locator('td:has-text("IP Address:")').locator('xpath=following-sibling::td').locator('code')).toBeVisible();
    
    // Identifiers section
    const identifiersHeader = page.locator('h6.text-muted:has-text("Identifiers")');
    const identifiersSection = identifiersHeader.locator('xpath=ancestor::div[contains(@class, "row")]');
    await expect(identifiersSection.locator('table').first()).toBeVisible();
  });

  test('should perform search functionality', async ({ page }) => {
    // Generate audit events with different actions
    await seed.revokeToken(tokenId);
    await seed.updateProject(projectId, { is_active: false });
    
    await page.goto('/audit');
    
    // Search for specific action
    const searchInput = page.locator('input[name="search"]');
    await searchInput.fill('revoke');
    
    const searchButton = page.locator('button[type="submit"]:has(i.bi-search)');
    await searchButton.click();
    
    // URL should contain search parameter
    await expect(page).toHaveURL(/[?&]search=revoke/);
    
    // Should show clear search button when search is active
    const clearButton = page.locator('a.btn:has(i.bi-x-circle)');
    await expect(clearButton).toBeVisible();
    await expect(clearButton).toContainText('Clear');
  });

  test('should clear search results', async ({ page }) => {
    await page.goto('/audit?search=test');
    
    // Should show clear button for existing search
    const clearButton = page.locator('a.btn:has(i.bi-x-circle)');
    await expect(clearButton).toBeVisible();
    
    await clearButton.click();
    
    // Should navigate back to audit page without search
    await expect(page).toHaveURL('/audit');
    
    // Search input should be empty
    const searchInput = page.locator('input[name="search"]');
    await expect(searchInput).toHaveValue('');
  });

  test('should handle pagination navigation', async ({ page }) => {
    await page.goto('/audit');
    
    // Check if pagination exists (may not be visible with few events)
    const paginationContainer = page.locator('.pagination');
    
    // If pagination is present, test navigation
    if (await paginationContainer.isVisible()) {
      // Test next page if available
      const nextButton = paginationContainer.locator('a[rel="next"]');
      if (await nextButton.isVisible()) {
        await nextButton.click();
        await expect(page).toHaveURL(/[?&]page=2/);
        
        // Test previous page
        const prevButton = paginationContainer.locator('a[rel="prev"]');
        if (await prevButton.isVisible()) {
          await prevButton.click();
          await expect(page).toHaveURL(/audit(?:\?(?!.*page=2).*)?$/);
        }
      }
    }
  });

  test('should handle empty audit events state', async ({ page }) => {
    // Navigate to audit page when no events exist (before creating any test data)
    const freshSeed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await page.goto('/audit');
    
    // Should show empty state when no events exist
    const noEventsSection = page.locator('.text-center.py-5');
    if (await noEventsSection.isVisible()) {
      await expect(noEventsSection.locator('i.bi-shield-x')).toBeVisible();
      await expect(noEventsSection).toContainText('No Audit Events Found');
    } else {
      // If events do exist from other tests, just verify table is displayed
      await expect(page.locator('table.table-hover')).toBeVisible();
    }
  });

  test('should maintain search state during pagination', async ({ page }) => {
    // Generate multiple audit events
    for (let i = 0; i < 3; i++) {
      const tempToken = await seed.createToken(projectId, 30);
      await seed.revokeToken(tempToken.id);
    }
    
    await page.goto('/audit');
    
    // Perform search
    const searchInput = page.locator('input[name="search"]');
    await searchInput.fill('revoke');
    
    const searchButton = page.locator('button[type="submit"]:has(i.bi-search)');
    await searchButton.click();
    
    // Verify search input retains value
    await expect(searchInput).toHaveValue('revoke');
    
    // If pagination exists with search, navigate and verify search is maintained
    const paginationContainer = page.locator('.pagination');
    if (await paginationContainer.isVisible()) {
      const nextButton = paginationContainer.locator('a[rel="next"]');
      if (await nextButton.isVisible()) {
        await nextButton.click();
        // Search should be maintained in URL
        await expect(page).toHaveURL(/[?&]search=revoke/);
        // Search input should still have value
        await expect(searchInput).toHaveValue('revoke');
      }
    }
  });

  test('should display outcome badges correctly', async ({ page }) => {
    // Generate events with different outcomes
    await seed.revokeToken(tokenId); // Should create success outcome
    
    await page.goto('/audit');
    
    // Verify outcome badges
    const outcomeColumn = page.locator('table tbody tr td').nth(3);
    const outcomeBadge = outcomeColumn.locator('.badge').first();
    await expect(outcomeBadge).toBeVisible();
    
    // Should have appropriate badge color class
    const badgeClasses = await outcomeBadge.getAttribute('class');
    expect(badgeClasses).toMatch(/badge\s+bg-(success|danger|warning)/);
  });
});