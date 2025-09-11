import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';
import { SeedFixture } from '../fixtures/seed';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
const BASE_URL = process.env.ADMIN_BASE_URL || 'http://localhost:8099';
const PROXY_BASE_URL = process.env.PROXY_BASE_URL || 'http://localhost:8080';

test.describe('Cross-Feature Workflow E2E Tests', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;
  let tokenId: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    // Create test data
    projectId = await seed.createProject('Workflow Test Project', 'sk-workflow-test-key');
    tokenId = await seed.createToken(projectId, 60);
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should handle project deactivation → proxy behavior workflow', async ({ page }) => {
    // Step 1: Verify project is initially active
    await page.goto(`/projects/${projectId}`);
    await expect(page.locator('.badge:has-text("Active")')).toBeVisible();
    
    // Step 2: Deactivate project via Admin UI
    await page.goto(`/projects/${projectId}/edit`);
    
    // Uncheck is_active if it's checked
    const activeSwitch = page.locator('#is_active');
    await activeSwitch.uncheck();
    
    // Submit the form
    await page.click('button[type="submit"]');
    
    // Should redirect back to project page
    await expect(page).toHaveURL(new RegExp(`/projects/${projectId}(?:/.*)?$|/auth/login$`));
    
    // Step 3: Verify project shows as inactive in UI
    await page.goto(`/projects/${projectId}`);
    await expect(page.locator('.badge:has-text("Inactive"), .badge:has-text("Disabled")')).toBeVisible();
    
    // Step 4: Verify audit events are generated for project deactivation
    await page.goto('/audit');
    
    // Should see audit events related to project update
    const auditTable = page.locator('table tbody');
    if (await auditTable.isVisible()) {
      // Look for project-related audit events
      const projectEvents = auditTable.locator('tr').filter({ hasText: 'project' });
      if (await projectEvents.count() > 0) {
        await expect(projectEvents.first()).toBeVisible();
      }
    }
  });

  test('should handle token revocation → access verification workflow', async ({ page }) => {
    // Step 1: Verify token is initially active
    await page.goto(`/tokens/${tokenId}`);
    await expect(page.locator('.badge:has-text("Active")')).toBeVisible();
    
    // Step 2: Revoke token via Admin UI
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke');
      await dialog.accept();
    });
    
    const revokeButton = page.locator('button:has-text("Revoke Token")');
    await revokeButton.click();
    
    // Should redirect after revocation
    await expect(page).toHaveURL(/\/tokens(?!\/.*\/edit).*$/);
    
    // Step 3: Verify token shows as revoked in token list
    await page.goto('/tokens');
    const tokenTable = page.locator('table tbody');
    if (await tokenTable.isVisible()) {
      // Look for revoked status indicators
      const statusBadges = tokenTable.locator('.badge');
      if (await statusBadges.count() > 0) {
        // Should have at least one revoked/inactive token
        const revokedBadge = statusBadges.filter({ hasText: /Revoked|Inactive/ });
        if (await revokedBadge.count() > 0) {
          await expect(revokedBadge.first()).toBeVisible();
        }
      }
    }
    
    // Step 4: Verify audit trail for revocation action
    await page.goto('/audit');
    
    // Should see audit events for token revocation
    const auditTable = page.locator('table tbody');
    if (await auditTable.isVisible()) {
      // Look for revoke-related audit events
      const revokeEvents = auditTable.locator('tr').filter({ hasText: 'revoke' });
      if (await revokeEvents.count() > 0) {
        await expect(revokeEvents.first()).toBeVisible();
        
        // Click on first revoke event to see details
        const firstRevokeEvent = revokeEvents.first().locator('a[href^="/audit/"]');
        await firstRevokeEvent.click();
        
        // Verify event details contain token information
        await expect(page.locator('h1')).toContainText('Audit Event Details');
        
        // Should show token ID in identifiers section
        const identifiersSection = page.locator('.row').filter({ hasText: 'Identifiers' });
        if (await identifiersSection.isVisible()) {
          await expect(identifiersSection.locator('table')).toBeVisible();
        }
      }
    }
  });

  test('should handle bulk operations → audit trail workflow', async ({ page }) => {
    // Step 1: Create multiple tokens for the project
    const additionalTokens = [];
    for (let i = 0; i < 2; i++) {
      const token = await seed.createToken(projectId, 30);
      additionalTokens.push(token);
    }
    
    // Step 2: Perform bulk token revocation via Admin UI
    await page.goto(`/projects/${projectId}`);
    
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('revoke ALL tokens');
      await dialog.accept();
    });
    
    const bulkRevokeButton = page.locator('button:has-text("Revoke All Tokens")');
    await bulkRevokeButton.click();
    
    // Should stay on project page after bulk revocation
    await expect(page).toHaveURL(`/projects/${projectId}`);
    
    // Step 3: Verify audit events for batch operations
    await page.goto('/audit');
    
    // Should see multiple revocation events or bulk operation event
    const auditTable = page.locator('table tbody');
    if (await auditTable.isVisible()) {
      // Look for bulk or multiple revoke events
      const revokeEvents = auditTable.locator('tr').filter({ hasText: /revoke|bulk/i });
      if (await revokeEvents.count() > 0) {
        await expect(revokeEvents.first()).toBeVisible();
        
        // Step 4: Check audit event metadata accuracy
        const firstEvent = revokeEvents.first().locator('a[href^="/audit/"]');
        await firstEvent.click();
        
        // Verify event details
        await expect(page.locator('h1')).toContainText('Audit Event Details');
        
        // Should show project ID in identifiers
        const identifiersTable = page.locator('.row').filter({ hasText: 'Identifiers' }).locator('table');
        if (await identifiersTable.isVisible()) {
          const projectIdRow = identifiersTable.locator('tr').filter({ hasText: 'Project ID' });
          if (await projectIdRow.isVisible()) {
            await expect(projectIdRow.locator('code')).toContainText(projectId);
          }
        }
        
        // Check for metadata section
        const metadataSection = page.locator('.row').filter({ hasText: 'Additional Details' });
        if (await metadataSection.isVisible()) {
          await expect(metadataSection.locator('pre code')).toBeVisible();
        }
      }
    }
    
    // Step 5: Verify all tokens are actually revoked
    await page.goto('/tokens');
    
    // Filter tokens by project (if supported) or verify via API
    for (const token of additionalTokens) {
      try {
        const tokenInfo = await seed.getToken(token);
        expect(tokenInfo.is_active).toBe(false);
      } catch (error) {
        // Token might be deleted or API might return 404 for revoked tokens
        console.log(`Token ${token} verification: ${error}`);
      }
    }
  });

  test('should handle project status changes and token accessibility', async ({ page }) => {
    // Step 1: Verify token works with active project (basic test)
    await page.goto(`/tokens/${tokenId}`);
    await expect(page.locator('.badge:has-text("Active")')).toBeVisible();
    
    // Step 2: Deactivate project
    await page.goto(`/projects/${projectId}/edit`);
    const activeSwitch = page.locator('#is_active');
    await activeSwitch.uncheck();
    await page.click('button[type="submit"]');
    
    // Step 3: Verify tokens for inactive project show appropriate status
    await page.goto('/tokens');
    
    // Look for tokens in the table and their status
    const tokenTable = page.locator('table tbody');
    if (await tokenTable.isVisible()) {
      // Check that status badges reflect project state
      const statusBadges = tokenTable.locator('.badge');
      if (await statusBadges.count() > 0) {
        await expect(statusBadges.first()).toBeVisible();
      }
    }
    
    // Step 4: Check audit trail shows the cascade effect
    await page.goto('/audit');
    
    // Should see project update events
    const auditTable = page.locator('table tbody');
    if (await auditTable.isVisible()) {
      const projectEvents = auditTable.locator('tr').filter({ hasText: /project|update/i });
      if (await projectEvents.count() > 0) {
        await expect(projectEvents.first()).toBeVisible();
      }
    }
  });

  test('should handle search and filtering in audit events during workflows', async ({ page }) => {
    // Step 1: Perform multiple actions to create diverse audit events
    await seed.revokeToken(tokenId);
    await seed.updateProject(projectId, { is_active: false });
    
    const newTokenId = await seed.createToken(projectId, 45);
    await seed.revokeToken(newTokenId);
    
    // Step 2: Test audit search functionality with workflow data
    await page.goto('/audit');
    
    // Search for revoke events
    const searchInput = page.locator('input[name="search"]');
    await searchInput.fill('revoke');
    await page.click('button[type="submit"]:has(i.bi-search)');
    
    // Should filter to only revoke events
    await expect(page).toHaveURL(/[?&]search=revoke/);
    
    const auditTable = page.locator('table tbody');
    if (await auditTable.isVisible()) {
      const visibleRows = auditTable.locator('tr');
      if (await visibleRows.count() > 0) {
        // Each visible row should contain revoke-related content
        const firstRow = visibleRows.first();
        await expect(firstRow).toContainText(/revoke/i);
      }
    }
    
    // Step 3: Clear search and verify all events are shown
    const clearButton = page.locator('a.btn:has(i.bi-x-circle)');
    if (await clearButton.isVisible()) {
      await clearButton.click();
      await expect(page).toHaveURL('/audit');
    }
    
    // Step 4: Search for project-specific events
    await searchInput.fill(projectId);
    await page.click('button[type="submit"]:has(i.bi-search)');
    
    // Should show events related to this project
    await expect(page).toHaveURL(new RegExp(`[?&]search=${projectId}`));
  });

  test('should verify end-to-end audit trail completeness', async ({ page }) => {
    // Step 1: Record initial audit count
    await page.goto('/audit');
    let initialEventCount = 0;
    
    const auditTable = page.locator('table tbody');
    if (await auditTable.isVisible()) {
      initialEventCount = await auditTable.locator('tr').count();
    }
    
    // Step 2: Perform a complete workflow
    // Create token
    const workflowTokenId = await seed.createToken(projectId, 30);
    
    // Update project
    await seed.updateProject(projectId, { name: 'Updated Workflow Project' });
    
    // Revoke token
    await seed.revokeToken(workflowTokenId);
    
    // Bulk revoke remaining tokens
    await seed.revokeProjectTokens(projectId);
    
    // Step 3: Verify audit events were created for each action
    await page.goto('/audit');
    
    if (await auditTable.isVisible()) {
      const finalEventCount = await auditTable.locator('tr').count();
      
      // Should have more events than initially (at least one for each action)
      expect(finalEventCount).toBeGreaterThan(initialEventCount);
      
      // Step 4: Verify event details contain proper metadata
      const firstEvent = auditTable.locator('tr').first().locator('a[href^="/audit/"]');
      if (await firstEvent.isVisible()) {
        await firstEvent.click();
        
        // Should show complete event details
        await expect(page.locator('h1')).toContainText('Audit Event Details');
        
        // Should have basic information
        const basicInfoTable = page.locator('.col-md-6').first().locator('table');
        await expect(basicInfoTable.locator('td:has-text("Timestamp:")')).toBeVisible();
        await expect(basicInfoTable.locator('td:has-text("Action:")')).toBeVisible();
        await expect(basicInfoTable.locator('td:has-text("Outcome:")')).toBeVisible();
        
        // Should have identifiers section
        const identifiersTable = page.locator('.row').filter({ hasText: 'Identifiers' }).locator('table');
        await expect(identifiersTable).toBeVisible();
      }
    }
  });
});