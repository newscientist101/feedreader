import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/layout',
  testMatch: '**/*.test.js',
  fullyParallel: false,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:3000',
    headless: true,
    // No automatic screenshots — tests measure bounding rects
    screenshot: 'off',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
