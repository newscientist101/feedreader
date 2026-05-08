import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import { fileURLToPath } from 'url';
import path from 'path';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

export default defineConfig({
  test: {
    root,
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
            await page.goto(full, { waitUntil: 'domcontentloaded' });
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
            await page.goto(full, { waitUntil: 'domcontentloaded' });

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

        /**
         * Measure bounding rects at multiple viewport widths without
         * reopening the page. Navigates once, then resizes and re-measures.
         * Returns: { [width]: { [selector]: box | null } }
         */
        async measureLayoutMultiWidth(ctx, url, selectors, widths, viewportHeight) {
          const baseURL = 'http://localhost:3000';
          const full = url.startsWith('http') ? url : `${baseURL}${url}`;
          const vh = viewportHeight || 720;

          const page = await ctx.context.newPage();
          try {
            // Start at the first width and navigate
            await page.setViewportSize({ width: widths[0], height: vh });
            await page.goto(full, { waitUntil: 'domcontentloaded' });

            const allResults = {};
            for (const w of widths) {
              await page.setViewportSize({ width: w, height: vh });
              // Small delay for layout to settle after resize
              await page.waitForTimeout(100);

              const results = {};
              for (const sel of selectors) {
                const el = page.locator(sel).first();
                const box = await el.boundingBox().catch(() => null);
                results[sel] = box;
              }
              allResults[w] = results;
            }
            return allResults;
          } finally {
            await page.close();
          }
        },


        /**
         * Run the auto-mark-read navigation regression scenario against an
         * isolated feedreader server and DB fixture.
         */
        async runAutoMarkReadBackNavigationScenario(ctx) {
          const fs = await import('node:fs/promises');
          const os = await import('node:os');
          const path = await import('node:path');
          const childProcess = await import('node:child_process');
          const { promisify } = await import('node:util');
          const execFile = promisify(childProcess.execFile);

          const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'feedreader-browser-'));
          const dbPath = path.join(tmpDir, 'test.sqlite3');
          let server;

          const waitForServer = async () => {
            const deadline = Date.now() + 15000;
            let lastErr;
            while (Date.now() < deadline) {
              try {
                const resp = await fetch('http://localhost:3200/api/counts', {
                  headers: {
                    'X-Exedev-Userid': 'browser-integ-user',
                    'X-Exedev-Email': 'browser-integ@example.com',
                  },
                });
                if (resp.ok) return;
                lastErr = new Error(`status ${resp.status}`);
              } catch (err) {
                lastErr = err;
              }
              await new Promise(resolve => setTimeout(resolve, 100));
            }
            throw lastErr || new Error('server did not start');
          };

          const cleanup = async () => {
            if (server && !server.killed) {
              server.kill('SIGTERM');
              await new Promise(resolve => {
                const timer = setTimeout(() => {
                  if (!server.killed) server.kill('SIGKILL');
                  resolve();
                }, 3000);
                server.once('exit', () => {
                  clearTimeout(timer);
                  resolve();
                });
              });
            }
            await fs.rm(tmpDir, { recursive: true, force: true });
          };

          try {
            await execFile('go', ['build', '-o', path.join(tmpDir, 'browser-integration-server'), './tests/config/browser-integration-server.go'], {
              cwd: root,
            });
            server = childProcess.spawn(path.join(tmpDir, 'browser-integration-server'), [dbPath], {
              cwd: root,
              env: { ...process.env, DEV: '' },
              stdio: ['ignore', 'pipe', 'pipe'],
            });
            server.stdout.on('data', chunk => process.stdout.write(chunk));
            server.stderr.on('data', chunk => process.stderr.write(chunk));
            await waitForServer();

            const page = await ctx.context.newPage();
            try {
              await page.setExtraHTTPHeaders({
                'X-Exedev-Userid': 'browser-integ-user',
                'X-Exedev-Email': 'browser-integ@example.com',
              });
              await page.setViewportSize({ width: 900, height: 520 });
              await page.goto('http://localhost:3200/', { waitUntil: 'domcontentloaded' });
              await page.waitForSelector('.article-card');

              const allIds = await page.$$eval('.article-card', cards => cards.map(card => card.dataset.id));
              const allTitles = await page.$$eval('.article-card', cards => cards.map(card => card.querySelector('.article-title')?.textContent?.trim() || ''));
              const state = {
                allIds,
                allTitles,
                expectedRead: [],
                expectedVisible: allIds.slice(),
              };

              const visibleIds = async () => page.$$eval('.article-card', cards => cards
                .filter(card => getComputedStyle(card).display !== 'none')
                .map(card => card.dataset.id));
              const readIds = async () => page.$$eval('.article-card.read', cards => cards.map(card => card.dataset.id));
              const unreadApiIds = async () => page.evaluate(async () => {
                const resp = await fetch('/api/articles/unread');
                const data = await resp.json();
                return (data.articles || []).map(article => String(article.id));
              });
              const flush = async () => page.waitForTimeout(700);
              const rememberRead = async () => {
                const ids = await readIds();
                for (const id of ids) {
                  if (!state.expectedRead.includes(id)) state.expectedRead.push(id);
                }
                state.expectedVisible = state.allIds.filter(id => !state.expectedRead.includes(id));
              };
              const assertPageState = async (label) => {
                const visible = await visibleIds();
                const apiIds = await unreadApiIds();
                return {
                  label,
                  expectedRead: state.expectedRead.slice(),
                  expectedVisible: state.expectedVisible.slice(),
                  visible,
                  apiIds,
                };
              };

              await page.evaluate(() => window.scrollTo(0, 1450));
              await flush();
              await rememberRead();
              const afterFirstScroll = await assertPageState('after first scroll');

              const clickId = state.expectedVisible[1] || state.expectedVisible[0];
              await page.locator(`.article-card[data-id="${clickId}"] .article-body.clickable`).click();
              await page.waitForURL(/\/article\/\d+$/);
              if (!state.expectedRead.includes(clickId)) state.expectedRead.push(clickId);
              state.expectedVisible = state.allIds.filter(id => !state.expectedRead.includes(id));

              await page.goBack({ waitUntil: 'domcontentloaded' });
              await page.waitForSelector('.article-card');
              await flush();
              const afterBack = await assertPageState('after back');

              await page.evaluate(() => window.scrollTo(0, 1800));
              await flush();
              await rememberRead();
              const afterSecondScroll = await assertPageState('after second scroll');

              return {
                allIds: state.allIds,
                allTitles: state.allTitles,
                clickedId: clickId,
                afterFirstScroll,
                afterBack,
                afterSecondScroll,
              };
            } finally {
              await page.close();
            }
          } finally {
            await cleanup();
          }
        },

        /**
         * Get the current name of a feed via the API.
         */
        async getFeedName(ctx, feedId) {
          const resp = await fetch(`http://localhost:3000/api/feeds/${feedId}`);
          const data = await resp.json();
          return data.name;
        },

        /**
         * Set the name of a feed via the API.
         */
        async setFeedName(ctx, feedId, name) {
          const resp = await fetch(`http://localhost:3000/api/feeds/${feedId}`, {
            method: 'PUT',
            headers: {
              'Content-Type': 'application/json',
              'X-Requested-With': 'vitest',
            },
            body: JSON.stringify({ name }),
          });
          if (!resp.ok) {
            throw new Error(`Failed to set feed name: ${resp.status}`);
          }
          return true;
        },

        /**
         * Measure layout for multiple feed names on a single Playwright page.
         * For each name: sets the feed name via API, reloads the page, and
         * measures bounding rects. Returns results keyed by name.
         *
         * @param {object} ctx - Vitest command context
         * @param {string} url - Page URL to measure
         * @param {string[]} selectors - CSS selectors to measure
         * @param {number} feedId - Feed ID to rename
         * @param {string[]} names - Feed names to test
         * @param {number} viewportWidth - Viewport width
         * @param {number} [viewportHeight] - Viewport height (default 720)
         * @returns {Record<string, Record<string, object|null>>}
         */
        async measureLayoutForNames(ctx, url, selectors, feedId, names, viewportWidth, viewportHeight) {
          const baseURL = 'http://localhost:3000';
          const full = url.startsWith('http') ? url : `${baseURL}${url}`;
          const vw = viewportWidth || 1280;
          const vh = viewportHeight || 720;

          // Set the first name via API so the initial page load is correct
          const firstResp = await fetch(`${baseURL}/api/feeds/${feedId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'vitest' },
            body: JSON.stringify({ name: names[0] }),
          });
          if (!firstResp.ok) throw new Error(`Failed to set feed name: ${firstResp.status}`);

          const page = await ctx.context.newPage();
          try {
            await page.setViewportSize({ width: vw, height: vh });
            await page.goto(full, { waitUntil: 'domcontentloaded' });

            const allResults = {};
            for (const name of names) {
              // Update h1 text directly via JS (avoids page reload)
              await page.evaluate((n) => {
                const h1 = document.querySelector('.view-header h1');
                if (h1) h1.textContent = n;
              }, name);

              const results = {};
              for (const sel of selectors) {
                const el = page.locator(sel).first();
                const box = await el.boundingBox().catch(() => null);
                results[sel] = box;
              }
              allResults[name] = results;
            }
            return allResults;
          } finally {
            await page.close();
          }
        },

        /**
         * Measure layout for multiple feed names at multiple viewport widths
         * on a single Playwright page. For each name: sets the feed name,
         * then measures at each width. Returns results keyed by name then width.
         *
         * @param {object} ctx - Vitest command context
         * @param {string} url - Page URL to measure
         * @param {string[]} selectors - CSS selectors to measure
         * @param {number} feedId - Feed ID to rename
         * @param {string[]} names - Feed names to test
         * @param {number[]} widths - Viewport widths to test
         * @param {number} [viewportHeight] - Viewport height (default 720)
         * @returns {Record<string, Record<number, Record<string, object|null>>>}
         */
        async measureLayoutForNamesMultiWidth(ctx, url, selectors, feedId, names, widths, viewportHeight) {
          const baseURL = 'http://localhost:3000';
          const full = url.startsWith('http') ? url : `${baseURL}${url}`;
          const vh = viewportHeight || 720;

          // Set the first name via API so the initial page load is correct
          const firstResp = await fetch(`${baseURL}/api/feeds/${feedId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'vitest' },
            body: JSON.stringify({ name: names[0] }),
          });
          if (!firstResp.ok) throw new Error(`Failed to set feed name: ${firstResp.status}`);

          const page = await ctx.context.newPage();
          try {
            // Navigate once at the first width
            await page.setViewportSize({ width: widths[0], height: vh });
            await page.goto(full, { waitUntil: 'domcontentloaded' });

            const allResults = {};
            for (const name of names) {
              // Update h1 text directly via JS (avoids page reload)
              await page.evaluate((n) => {
                const h1 = document.querySelector('.view-header h1');
                if (h1) h1.textContent = n;
              }, name);

              const nameResults = {};
              for (const w of widths) {
                await page.setViewportSize({ width: w, height: vh });
                // Brief wait for layout to settle after resize
                await page.waitForTimeout(50);

                const results = {};
                for (const sel of selectors) {
                  const el = page.locator(sel).first();
                  const box = await el.boundingBox().catch(() => null);
                  results[sel] = box;
                }
                nameResults[w] = results;
              }
              allResults[name] = nameResults;
            }
            return allResults;
          } finally {
            await page.close();
          }
        },
      },
    },
  },
});
