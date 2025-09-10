import { Page } from '@playwright/test';

/**
 * Auth fixture for handling Admin UI authentication
 */
export class AuthFixture {
  constructor(private page: Page) {}

  /**
   * Login to the Admin UI with the management token
   */
  async login(managementToken: string): Promise<void> {
    // Navigate to login page
    await this.page.goto('/auth/login');

    // Fill in the management token
    await this.page.fill('#management_token', managementToken);

    // Submit the login form
    await this.page.click('button[type="submit"]');

    // Wait for redirect to dashboard
    await this.page.waitForURL('/dashboard', { timeout: 10000 });
  }

  /**
   * Logout from the Admin UI
   */
  async logout(): Promise<void> {
    await this.page.goto('/auth/logout');
    await this.page.waitForURL('/auth/login', { timeout: 5000 });
  }

  /**
   * Check if currently logged in (on dashboard page)
   */
  async isLoggedIn(): Promise<boolean> {
    try {
      await this.page.goto('/dashboard');
      await this.page.waitForURL('/dashboard', { timeout: 2000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Navigate to a specific page, ensuring authentication first
   */
  async navigateAuthenticated(path: string, managementToken: string): Promise<void> {
    const isAuthenticated = await this.isLoggedIn();
    
    if (!isAuthenticated) {
      await this.login(managementToken);
    }
    
    await this.page.goto(path);
  }
}