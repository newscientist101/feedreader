/**
 * Vitest browser-mode config for unit tests.
 *
 * Runs module tests that benefit from real browser layout (getBoundingClientRect,
 * scroll measurements, etc.) in headless Chromium. Unlike the layout tests
 * (vitest.browser.config.mjs), these tests do NOT require a running app server.
 *
 * Usage:
 *   npx vitest run --config vitest.browser-unit.config.mjs
 */
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';

export default defineConfig({
  test: {
    include: ['srv/static/**/*.browser.test.js'],
    teardownTimeout: 5000,
    testTimeout: 10000,
    fileParallelism: false,
    browser: {
      enabled: true,
      provider: playwright(),
      headless: true,
      instances: [{ browser: 'chromium' }],
    },
  },
});
