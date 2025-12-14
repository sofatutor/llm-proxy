import { test, expect } from '@playwright/test';
import { AuthFixture } from '../fixtures/auth';
import { SeedFixture } from '../fixtures/seed';

const MANAGEMENT_TOKEN = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
const BASE_URL = process.env.ADMIN_BASE_URL || 'http://localhost:8099';

test.describe('Timestamp Localization', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;
  let tokenId: string;

  test.beforeEach(async ({ page }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    // Create test data
    projectId = await seed.createProject('Timestamp Test Project', 'sk-test-key');
    const token = await seed.createToken(projectId, 120);
    tokenId = token.id;
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should render timestamps with data-local-time attributes on token show page', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}`);
    
    // Verify that timestamps have the localization attributes
    const timestampElements = await page.locator('[data-local-time="true"][data-ts]').all();
    
    // Should have at least 2 timestamp elements (expires + created)
    expect(timestampElements.length).toBeGreaterThanOrEqual(2);
    
    // Verify each has a valid RFC3339 timestamp
    for (const el of timestampElements) {
      const tsValue = await el.getAttribute('data-ts');
      expect(tsValue).toBeTruthy();
      expect(tsValue).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/);

      // Should preserve the canonical UTC ISO timestamp in title
      const title = await el.getAttribute('title');
      expect(title).toBeTruthy();
      expect(title).toBe(tsValue);
      
      // Should end with Z (UTC) or have timezone offset
      expect(tsValue).toMatch(/Z$|[+-]\d{2}:\d{2}$/);
    }
  });

  test('should render timestamps with data-local-time attributes on token list page', async ({ page }) => {
    await page.goto('/tokens');
    
    // Verify that timestamps have the localization attributes
    const timestampElements = await page.locator('[data-local-time="true"][data-ts]').all();
    
    // Should have timestamp elements for created, expires, last used
    expect(timestampElements.length).toBeGreaterThanOrEqual(1);
    
    // Verify format attribute
    const firstTimestamp = timestampElements[0];
    const formatAttr = await firstTimestamp.getAttribute('data-format');
    expect(['ymd_hm', 'ymd_hms', 'ymd_hms_tz', 'long', 'date_only']).toContain(formatAttr);

    // Tables should keep the UTC ISO timestamp in the title attribute
    const dataTs = await firstTimestamp.getAttribute('data-ts');
    const title = await firstTimestamp.getAttribute('title');
    expect(title).toBeTruthy();
    expect(title).toBe(dataTs);
  });

  test('should keep UTC ISO timestamp as title on project list table', async ({ page }) => {
    await page.goto('/projects');

    const firstTimestamp = await page.locator('[data-local-time="true"][data-ts]').first();
    await expect(firstTimestamp).toBeVisible();

    const dataTs = await firstTimestamp.getAttribute('data-ts');
    const title = await firstTimestamp.getAttribute('title');
    expect(title).toBeTruthy();
    expect(title).toBe(dataTs);
  });

  test('should render timestamps with data-local-time attributes on project show page', async ({ page }) => {
    await page.goto(`/projects/${projectId}`);
    
    // Verify that timestamps have the localization attributes
    const timestampElements = await page.locator('[data-local-time="true"][data-ts]').all();
    
    // Should have at least 2 timestamp elements (created and updated)
    expect(timestampElements.length).toBeGreaterThanOrEqual(2);

    // Spot-check first timestamp keeps canonical UTC ISO in title
    const first = timestampElements[0];
    const dataTs = await first.getAttribute('data-ts');
    const title = await first.getAttribute('title');
    expect(title).toBeTruthy();
    expect(title).toBe(dataTs);
  });

  test('should render timestamps with data-local-time attributes on dashboard', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Verify server time has localization attribute
    const serverTime = await page.locator('#server-time[data-local-time="true"]');
    await expect(serverTime).toBeVisible();
    
    const tsValue = await serverTime.getAttribute('data-ts');
    expect(tsValue).toBeTruthy();

    const title = await serverTime.getAttribute('title');
    expect(title).toBeTruthy();
    expect(title).toBe(tsValue);
  });

  test('should localize timestamps using JavaScript on page load', async ({ page }) => {
    // Go to a page with timestamps
    await page.goto(`/tokens/${tokenId}`);
    
    // Wait for JavaScript to execute
    await page.waitForLoadState('networkidle');
    
    // Get a timestamp element
    const timestampEl = await page.locator('[data-local-time="true"][data-ts]').first();
    const text = await timestampEl.textContent();
    
    // The text should be populated (not empty)
    expect(text).toBeTruthy();
    expect(text?.length).toBeGreaterThan(0);
    
    // Text should contain date-like content (year YYYY)
    expect(text).toMatch(/202\d/);
  });

  test('should format timestamps according to data-format attribute', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}`);
    await page.waitForLoadState('networkidle');
    
    // Find a timestamp with long format and ensure it matches our human-friendly style.
    const longTimestamp = await page.locator('[data-format="long"]').first();
    const text = await longTimestamp.textContent();

    expect(text).toBeTruthy();
    expect(text).toContain(' at ');
    expect(text).toMatch(/\b\d{1,2}:\d{2}\b/);
    expect(text).toMatch(/\bAM\b|\bPM\b/i);
  });

  test('should maintain preserved UTC timestamps in edit form', async ({ page }) => {
    await page.goto(`/tokens/${tokenId}/edit`);
    
    // The expires_at input field should show UTC and not have localization attributes
    const expiresInput = await page.locator('#expires_at');
    const value = await expiresInput.inputValue();
    
    // Should contain "UTC" in the value
    expect(value).toContain('UTC');
    
    // This input should NOT have data-local-time attribute (it's preserved as UTC)
    const hasLocalTime = await expiresInput.getAttribute('data-local-time');
    expect(hasLocalTime).toBeNull();
  });
});

test.describe('Timestamp Localization - Timezone Variations', () => {
  let auth: AuthFixture;
  let seed: SeedFixture;
  let projectId: string;
  let tokenId: string;

  test.beforeEach(async ({ page, context }) => {
    auth = new AuthFixture(page);
    seed = new SeedFixture(BASE_URL, MANAGEMENT_TOKEN);
    
    await auth.login(MANAGEMENT_TOKEN);
    
    projectId = await seed.createProject('TZ Test Project', 'sk-test-key');
    const token = await seed.createToken(projectId, 120);
    tokenId = token.id;
  });

  test.afterEach(async () => {
    await seed.cleanup();
  });

  test('should show different time for different timezones', async ({ browser }) => {
    // Test in Pacific timezone
    const pacificContext = await browser.newContext({
      timezoneId: 'America/Los_Angeles'
    });
    const pacificPage = await pacificContext.newPage();
    const pacificAuth = new AuthFixture(pacificPage);
    await pacificAuth.login(MANAGEMENT_TOKEN);
    await pacificPage.goto(`/tokens/${tokenId}`);
    await pacificPage.waitForLoadState('networkidle');
    const pacificTimestampEl = pacificPage.locator('[data-local-time="true"][data-ts]').first();
    const pacificDataTs = await pacificTimestampEl.getAttribute('data-ts');
    const pacificTitle = await pacificTimestampEl.getAttribute('title');
    const pacificText = await pacificTimestampEl.textContent();
    const pacificDisplayedHm = pacificText?.match(/\b\d{1,2}:\d{2}\b/)?.[0];
    
    // Test in Tokyo timezone  
    const tokyoContext = await browser.newContext({
      timezoneId: 'Asia/Tokyo'
    });
    const tokyoPage = await tokyoContext.newPage();
    const tokyoAuth = new AuthFixture(tokyoPage);
    await tokyoAuth.login(MANAGEMENT_TOKEN);
    await tokyoPage.goto(`/tokens/${tokenId}`);
    await tokyoPage.waitForLoadState('networkidle');
    const tokyoTimestampEl = tokyoPage.locator('[data-local-time="true"][data-ts]').first();
    const tokyoDataTs = await tokyoTimestampEl.getAttribute('data-ts');
    const tokyoTitle = await tokyoTimestampEl.getAttribute('title');
    const tokyoText = await tokyoTimestampEl.textContent();
    const tokyoDisplayedHm = tokyoText?.match(/\b\d{1,2}:\d{2}\b/)?.[0];
    
    // The times should be different (Pacific is typically 17 hours behind Tokyo)
    expect(pacificDataTs).toBeTruthy();
    expect(tokyoDataTs).toBeTruthy();
    expect(pacificDataTs).toBe(tokyoDataTs);

    // If a title is present, it should preserve UTC ISO (and be identical across timezones)
    if (pacificTitle !== null && tokyoTitle !== null) {
      expect(pacificTitle).toBeTruthy();
      expect(tokyoTitle).toBeTruthy();
      expect(pacificTitle).toBe(tokyoTitle);
      expect(pacificTitle).toBe(pacificDataTs);
    }

    // Visible text should differ because the browser timezone differs
    expect(pacificDisplayedHm).toBeTruthy();
    expect(tokyoDisplayedHm).toBeTruthy();
    expect(pacificDisplayedHm).not.toBe(tokyoDisplayedHm);
    
    await pacificContext.close();
    await tokyoContext.close();
  });

  test('should show same instant for UTC and local timezone', async ({ browser }) => {
    // Test in UTC
    const utcContext = await browser.newContext({
      timezoneId: 'UTC'
    });
    const utcPage = await utcContext.newPage();
    const utcAuth = new AuthFixture(utcPage);
    await utcAuth.login(MANAGEMENT_TOKEN);
    await utcPage.goto(`/tokens/${tokenId}`);
    await utcPage.waitForLoadState('networkidle');
    
    // Get the data-ts attribute (which is in UTC) and the displayed text
    const timestampEl = await utcPage.locator('[data-local-time="true"][data-ts]').first();
    const dataTs = await timestampEl.getAttribute('data-ts');
    const displayedText = await timestampEl.textContent();
    
    // Parse both and verify they represent the same instant
    expect(dataTs).toBeTruthy();
    expect(displayedText).toBeTruthy();

    const title = await timestampEl.getAttribute('title');
    if (title !== null) {
      expect(title).toBe(dataTs);
    }
    
    // Since we're in UTC timezone, the displayed time should match the UTC time (in 12h format).
    const utcDate = new Date(dataTs as string);
    const utcMinutes = utcDate.getUTCMinutes().toString().padStart(2, '0');
    const utcHour24 = utcDate.getUTCHours();
    const utcAmPm = utcHour24 >= 12 ? 'PM' : 'AM';
    let utcHour12 = utcHour24 % 12;
    if (utcHour12 === 0) utcHour12 = 12;
    const expectedDisplayedHm = `${utcHour12}:${utcMinutes} ${utcAmPm}`;
    expect(displayedText).toContain(expectedDisplayedHm);
    
    await utcContext.close();
  });
});
