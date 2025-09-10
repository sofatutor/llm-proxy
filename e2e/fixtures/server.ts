import { spawn, ChildProcess } from 'child_process';
import { request } from 'http';
import * as fs from 'fs';
import * as path from 'path';

/**
 * Server fixture for managing the LLM proxy server during E2E tests
 */
export class ServerFixture {
  private serverProcess: ChildProcess | null = null;
  private readonly baseUrl: string;
  private readonly managementToken: string;
  private readonly dbPath: string;

  constructor() {
    this.baseUrl = process.env.ADMIN_BASE_URL || 'http://localhost:8099';
    this.managementToken = process.env.MANAGEMENT_TOKEN || 'e2e-management-token';
    this.dbPath = process.env.DATABASE_PATH || './tmp/e2e-db.sqlite';
  }

  /**
   * Start the LLM proxy server with E2E configuration
   */
  async start(): Promise<void> {
    // Ensure bin directory exists and binary is built
    if (!fs.existsSync('./bin/llm-proxy')) {
      throw new Error('LLM proxy binary not found. Run "make build" first.');
    }

    // Ensure tmp directory exists and clean up old database
    const tmpDir = path.dirname(this.dbPath);
    if (!fs.existsSync(tmpDir)) {
      fs.mkdirSync(tmpDir, { recursive: true });
    }
    if (fs.existsSync(this.dbPath)) {
      fs.unlinkSync(this.dbPath);
    }

    // Start the server process
    this.serverProcess = spawn('./bin/llm-proxy', ['server'], {
      env: {
        ...process.env,
        MANAGEMENT_TOKEN: this.managementToken,
        LISTEN_ADDR: ':8099',
        DATABASE_PATH: this.dbPath,
        LOG_LEVEL: 'warn',
        ADMIN_UI_ENABLED: 'true',
        ADMIN_UI_LISTEN_ADDR: ':8099',
        LLM_PROXY_EVENT_BUS: 'in-memory',
      },
      stdio: 'pipe',
    });

    if (!this.serverProcess.pid) {
      throw new Error('Failed to start LLM proxy server');
    }

    // Wait for server to be ready
    await this.waitForHealth();
    console.log('LLM proxy server started successfully');
  }

  /**
   * Stop the LLM proxy server
   */
  async stop(): Promise<void> {
    if (this.serverProcess) {
      this.serverProcess.kill('SIGTERM');
      
      // Wait for process to exit
      await new Promise<void>((resolve) => {
        this.serverProcess!.on('exit', () => {
          resolve();
        });
        
        // Force kill after 5 seconds
        setTimeout(() => {
          if (this.serverProcess && !this.serverProcess.killed) {
            this.serverProcess.kill('SIGKILL');
          }
          resolve();
        }, 5000);
      });

      this.serverProcess = null;
      console.log('LLM proxy server stopped');
    }

    // Clean up database file
    if (fs.existsSync(this.dbPath)) {
      fs.unlinkSync(this.dbPath);
    }
  }

  /**
   * Wait for the server health endpoint to respond
   */
  private async waitForHealth(): Promise<void> {
    const maxAttempts = 30;
    const delayMs = 1000;

    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        await this.checkHealth();
        return;
      } catch (error) {
        if (attempt === maxAttempts) {
          throw new Error(`Server did not become healthy after ${maxAttempts} attempts: ${error}`);
        }
        await new Promise(resolve => setTimeout(resolve, delayMs));
      }
    }
  }

  /**
   * Check if the server health endpoint is responding
   */
  private async checkHealth(): Promise<void> {
    return new Promise((resolve, reject) => {
      const req = request(`${this.baseUrl}/health`, { timeout: 2000 }, (res) => {
        if (res.statusCode === 200) {
          resolve();
        } else {
          reject(new Error(`Health check failed with status ${res.statusCode}`));
        }
      });

      req.on('error', reject);
      req.on('timeout', () => {
        req.destroy();
        reject(new Error('Health check timeout'));
      });
      
      req.end();
    });
  }

  /**
   * Get the management token for API calls
   */
  getManagementToken(): string {
    return this.managementToken;
  }

  /**
   * Get the base URL for the server
   */
  getBaseUrl(): string {
    return this.baseUrl;
  }
}