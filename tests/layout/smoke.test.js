/**
 * Smoke test — verifies the layout test infrastructure works.
 *
 * Requires a running server behind the mitm auth proxy on port 3000.
 * Start the proxy with:
 *   mitmdump -p 3000 --mode reverse:http://localhost:8000 \
 *     --set modify_headers='/~q/X-Exedev-Userid/dev-user-1' \
 *     --set modify_headers='/~q/X-Exedev-Email/test@example.com'
 */
import { test, expect } from '@playwright/test';
import { assertVisible } from './helpers.js';

test('loads the home page and has the expected title', async ({ page }) => {
  await page.goto('/');
  const title = await page.title();
  expect(title).toMatch(/feed/i);
});

test('renders a visible body element', async ({ page }) => {
  await page.goto('/');
  await assertVisible(page, 'body');
});
