/**
 * Seeding fixture for creating test data via the Management API
 */
export class SeedFixture {
  private readonly baseUrl: string;
  private readonly managementToken: string;
  private readonly createdProjects: string[] = [];
  private readonly createdTokens: string[] = [];

  constructor(baseUrl: string, managementToken: string) {
    this.baseUrl = baseUrl;
    this.managementToken = managementToken;
  }

  /**
   * Create a test project
   */
  async createProject(name: string, openaiApiKey: string = 'sk-test-key-for-e2e'): Promise<string> {
    const response = await fetch(`${this.baseUrl}/manage/projects`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        name,
        openai_api_key: openaiApiKey,
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to create project: ${response.status} ${response.statusText}`);
    }

    const project = await response.json();
    this.createdProjects.push(project.id);
    return project.id;
  }

  /**
   * Create a test token for a project
   */
  async createToken(projectId: string, durationMinutes: number = 60): Promise<string> {
    const response = await fetch(`${this.baseUrl}/manage/tokens`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        project_id: projectId,
        duration_minutes: durationMinutes,
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to create token: ${response.status} ${response.statusText}`);
    }

    const tokenResponse = await response.json();
    this.createdTokens.push(tokenResponse.token);
    return tokenResponse.token;
  }

  /**
   * Update a project's status
   */
  async updateProject(projectId: string, updates: { name?: string; openai_api_key?: string; is_active?: boolean }): Promise<void> {
    const response = await fetch(`${this.baseUrl}/manage/projects/${projectId}`, {
      method: 'PATCH',
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(updates),
    });

    if (!response.ok) {
      throw new Error(`Failed to update project: ${response.status} ${response.statusText}`);
    }
  }

  /**
   * Get project details
   */
  async getProject(projectId: string): Promise<any> {
    const response = await fetch(`${this.baseUrl}/manage/projects/${projectId}`, {
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to get project: ${response.status} ${response.statusText}`);
    }

    return response.json();
  }

  /**
   * Get token details
   */
  async getToken(tokenId: string): Promise<any> {
    const response = await fetch(`${this.baseUrl}/manage/tokens/${tokenId}`, {
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to get token: ${response.status} ${response.statusText}`);
    }

    return response.json();
  }

  /**
   * Revoke a token
   */
  async revokeToken(tokenId: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}/manage/tokens/${tokenId}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to revoke token: ${response.status} ${response.statusText}`);
    }
  }

  /**
   * Bulk revoke all tokens for a project
   */
  async revokeProjectTokens(projectId: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}/manage/projects/${projectId}/tokens/revoke`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.managementToken}`,
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to revoke project tokens: ${response.status} ${response.statusText}`);
    }
  }

  /**
   * Clean up all created test data
   */
  async cleanup(): Promise<void> {
    // Revoke all created tokens first
    for (const tokenId of this.createdTokens) {
      try {
        await this.revokeToken(tokenId);
      } catch (error) {
        console.warn(`Failed to revoke token ${tokenId}:`, error);
      }
    }

    // Note: Projects cannot be deleted via the API (405 Method Not Allowed)
    // They will be cleaned up when the database is deleted

    this.createdProjects.length = 0;
    this.createdTokens.length = 0;
  }

  /**
   * Get all created project IDs
   */
  getCreatedProjects(): string[] {
    return [...this.createdProjects];
  }

  /**
   * Get all created token IDs
   */
  getCreatedTokens(): string[] {
    return [...this.createdTokens];
  }
}