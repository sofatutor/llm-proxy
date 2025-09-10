/**
 * HTTP utilities for E2E tests
 */
export class HttpUtils {
  constructor(
    private readonly baseUrl: string,
    private readonly managementToken: string
  ) {}

  /**
   * Make an authenticated request to the Management API
   */
  async request(path: string, options: RequestInit = {}): Promise<Response> {
    const url = `${this.baseUrl}${path}`;
    
    const headers = {
      'Authorization': `Bearer ${this.managementToken}`,
      'Content-Type': 'application/json',
      ...options.headers,
    };

    return fetch(url, {
      ...options,
      headers,
    });
  }

  /**
   * GET request to Management API
   */
  async get(path: string): Promise<Response> {
    return this.request(path, { method: 'GET' });
  }

  /**
   * POST request to Management API
   */
  async post(path: string, body?: any): Promise<Response> {
    return this.request(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  /**
   * PATCH request to Management API
   */
  async patch(path: string, body?: any): Promise<Response> {
    return this.request(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  /**
   * DELETE request to Management API
   */
  async delete(path: string): Promise<Response> {
    return this.request(path, { method: 'DELETE' });
  }

  /**
   * Parse JSON response with error handling
   */
  async parseJson(response: Response): Promise<any> {
    if (!response.ok) {
      const text = await response.text();
      throw new Error(`HTTP ${response.status}: ${text}`);
    }
    
    return response.json();
  }

  /**
   * Wait for a condition to be true
   */
  async waitForCondition(
    condition: () => Promise<boolean>,
    timeoutMs: number = 10000,
    intervalMs: number = 500
  ): Promise<void> {
    const startTime = Date.now();
    
    while (Date.now() - startTime < timeoutMs) {
      if (await condition()) {
        return;
      }
      await new Promise(resolve => setTimeout(resolve, intervalMs));
    }
    
    throw new Error(`Condition not met within ${timeoutMs}ms`);
  }
}