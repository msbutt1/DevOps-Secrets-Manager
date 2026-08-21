import { defineConfig, devices } from '@playwright/test';

/**
 * Browser tests run against a running stack: `make dev` locally (the default URL), or any
 * deployment via E2E_BASE_URL. New accounts are verified with the development email fallback,
 * read from the API log (E2E_API_LOG, defaulting to the file `make api` writes).
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  timeout: 60_000,
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
