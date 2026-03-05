import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';

export default defineConfig({
  test: {
    include: ['tests/layout/**/*.test.js'],
    browser: {
      enabled: true,
      provider: playwright(),
      headless: true,
      instances: [{ browser: 'chromium' }],
      commands: {
        /**
         * Navigate the top-level Playwright page to a URL and return the
         * full HTML. Tests can inject this into the iframe document to
         * measure layout without cross-origin restrictions.
         */
        async fetchPageHTML(ctx, url) {
          const baseURL = 'http://localhost:3000';
          const full = url.startsWith('http') ? url : `${baseURL}${url}`;

          // Use a new context with the same auth headers the mitm proxy adds
          const page = await ctx.context.newPage();
          try {
            await page.goto(full, { waitUntil: 'networkidle' });
            return await page.content();
          } finally {
            await page.close();
          }
        },

        /**
         * Navigate to a URL in a fresh Playwright page, set the viewport,
         * and return bounding rects for the given selectors.
         * Returns: { [selector]: { x, y, width, height } | null }
         */
        async measureLayout(ctx, url, selectors, viewportWidth, viewportHeight) {
          const baseURL = 'http://localhost:3000';
          const full = url.startsWith('http') ? url : `${baseURL}${url}`;
          const vw = viewportWidth || 1280;
          const vh = viewportHeight || 720;

          const page = await ctx.context.newPage();
          try {
            await page.setViewportSize({ width: vw, height: vh });
            await page.goto(full, { waitUntil: 'networkidle' });

            const results = {};
            for (const sel of selectors) {
              const el = page.locator(sel).first();
              const box = await el.boundingBox().catch(() => null);
              results[sel] = box;
            }
            return results;
          } finally {
            await page.close();
          }
        },
      },
    },
  },
});
