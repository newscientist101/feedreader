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
        /**
         * @param {object} [options]
         * @param {'body'|'title-link'} [options.clickMode='body'] - Which element to click to open the article.
         */
        async runAutoMarkReadBackNavigationScenario(ctx, options = {}) {
          const clickMode = options.clickMode || 'body';
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
              const clickSelector = clickMode === 'title-link'
                ? `.article-card[data-id="${clickId}"] .article-title a[href^="/article/"]`
                : `.article-card[data-id="${clickId}"] .article-body.clickable`;
              await page.locator(clickSelector).click();
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
         * Run the fast-Back pending-read replay regression scenario.
         *
         * Intercepts the navigation-time POST /api/articles/batch-read so it
         * resolves only after the Back navigation has already completed. This
         * reproduces the race where the user returns to the article-list page
         * before the keepalive flush has persisted. The pageshow replay path
         * must re-POST the pending IDs and exclude them from the unread list.
         *
         * @param {object} [options]
         * @param {'body'|'title-link'} [options.clickMode='body'] - Which element
         *   to click to open the article.
         */
        async runFastBackPendingReadReplayScenario(ctx, options = {}) {
          const clickMode = options.clickMode || 'body';
          const fs = await import('node:fs/promises');
          const os = await import('node:os');
          const path = await import('node:path');
          const childProcess = await import('node:child_process');
          const { promisify } = await import('node:util');
          const execFile = promisify(childProcess.execFile);

          const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'feedreader-fastback-'));
          const dbPath = path.join(tmpDir, 'test.sqlite3');
          let server;

          const waitForServer = async (port) => {
            const deadline = Date.now() + 15000;
            let lastErr;
            while (Date.now() < deadline) {
              try {
                const resp = await fetch(`http://localhost:${port}/api/counts`, {
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

          // Port 3201 to avoid collision with the scenario on 3200
          const serverPort = 3201;

          try {
            await execFile('go', [
              'build',
              '-o', path.join(tmpDir, 'browser-integ-fastback'),
              './tests/config/browser-integration-server.go',
            ], { cwd: root });

            server = childProcess.spawn(
              path.join(tmpDir, 'browser-integ-fastback'),
              [dbPath, String(serverPort)],
              {
                cwd: root,
                env: { ...process.env, DEV: '' },
                stdio: ['ignore', 'pipe', 'pipe'],
              },
            );
            server.stdout.on('data', chunk => process.stdout.write(chunk));
            server.stderr.on('data', chunk => process.stderr.write(chunk));
            await waitForServer(serverPort);

            const page = await ctx.context.newPage();

            // Track batch-read requests so we can control when they resolve.
            // We want the first navigation-time keepalive POST (from openArticle)
            // to be delayed past the Back navigation so the pageshow replay path
            // has to re-POST it. Subsequent replay POSTs from pageshow must go
            // through immediately.
            let navigationBatchReadBlocked = false;
            let unblockNavigationBatchRead = null;

            // Intercept POST /api/articles/batch-read:
            //   - The first call while navigationBatchReadBlocked is true gets
            //     held until unblockNavigationBatchRead() is called.
            //   - All other calls pass through immediately.
            await page.route('**/api/articles/batch-read', async (route) => {
              if (route.request().method() !== 'POST') {
                await route.continue();
                return;
              }
              if (navigationBatchReadBlocked) {
                // Hold this request: wait for the explicit unblock signal, then
                // let it pass through so the server actually persists the IDs.
                await new Promise(resolve => { unblockNavigationBatchRead = resolve; });
                navigationBatchReadBlocked = false;
                unblockNavigationBatchRead = null;
              }
              await route.continue();
            });

            try {
              await page.setExtraHTTPHeaders({
                'X-Exedev-Userid': 'browser-integ-user',
                'X-Exedev-Email': 'browser-integ@example.com',
              });
              await page.setViewportSize({ width: 900, height: 520 });
              await page.goto(`http://localhost:${serverPort}/`, { waitUntil: 'domcontentloaded' });
              await page.waitForSelector('.article-card');

              const allIds = await page.$$eval('.article-card', cards => cards.map(card => card.dataset.id));
              const allTitles = await page.$$eval('.article-card', cards => cards.map(card => card.querySelector('.article-title')?.textContent?.trim() || ''));

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

              // Step 1: scroll to mark some articles read
              await page.evaluate(() => window.scrollTo(0, 1450));
              await flush();

              const scrolledReadIds = await readIds();

              // Step 2: block the next batch-read POST so it doesn't persist
              // before Back navigation completes (simulates fast Back race).
              navigationBatchReadBlocked = true;

              // Click article to navigate away; this triggers openArticle() which
              // calls mergePendingReadIds + flushMarkReadQueue({ keepalive: true }).
              const clickTarget = allIds.find(id => !scrolledReadIds.includes(id));
              const clickSelector = clickMode === 'title-link'
                ? `.article-card[data-id="${clickTarget}"] .article-title a[href^="/article/"]`
                : `.article-card[data-id="${clickTarget}"] .article-body.clickable`;
              await page.locator(clickSelector).click();
              await page.waitForURL(/\/article\/\d+$/);

              // Step 3: go Back before the intercepted batch-read has resolved.
              // The pageshow handler will then need to replay the pending IDs.
              await page.goBack({ waitUntil: 'domcontentloaded' });
              await page.waitForSelector('.article-card');

              // Step 4: now unblock the held batch-read so the server persists.
              // (Simulates the keepalive eventually completing after Back.)
              if (unblockNavigationBatchRead) {
                unblockNavigationBatchRead();
              }

              // Wait for the pageshow replay path to complete.
              await flush();

              const afterBackVisible = await visibleIds();
              const afterBackApiIds = await unreadApiIds();

              // The clicked article and all scroll-read articles must be absent
              // from both the DOM view and the unread API response.
              const allExpectedRead = [...scrolledReadIds, clickTarget];

              return {
                allIds,
                allTitles,
                clickedId: clickTarget,
                scrolledReadIds,
                afterBack: {
                  visible: afterBackVisible,
                  apiIds: afterBackApiIds,
                  expectedRead: allExpectedRead,
                  expectedVisible: allIds.filter(id => !allExpectedRead.includes(id)),
                },
              };
            } finally {
              await page.close();
            }
          } finally {
            await cleanup();
          }
        },

        /**
         * Run the duplicate-updateCounts race regression scenario.
         *
         * Proves that only one /api/counts request is issued on the
         * return-from-article pageshow path.  A stale counts response is
         * held open until AFTER the fresh one has resolved; the test then
         * asserts the unread badge shows the fresh (lower) value.
         *
         * Without the T1 fix an unawaited updateCounts() was fired at the
         * top of the pageshow handler in addition to the one inside
         * restoreFromState(). If the stale call resolved last, the badge
         * would be left with the pre-batch-read count indefinitely.
         *
         * Returns:
         *   { staleCount, freshCount, finalBadgeText, countsCallCount }
         */
        async runDuplicateUpdateCountsRaceScenario(ctx) {
          const fs = await import('node:fs/promises');
          const os = await import('node:os');
          const path = await import('node:path');
          const childProcess = await import('node:child_process');
          const { promisify } = await import('node:util');
          const execFile = promisify(childProcess.execFile);

          const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'feedreader-counts-race-'));
          const dbPath = path.join(tmpDir, 'test.sqlite3');
          let server;

          // Use port 3202 to avoid collision with other integration servers.
          const serverPort = 3202;

          const waitForServer = async (port) => {
            const deadline = Date.now() + 15000;
            let lastErr;
            while (Date.now() < deadline) {
              try {
                const resp = await fetch(`http://localhost:${port}/api/counts`, {
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
            await execFile('go', [
              'build',
              '-o', path.join(tmpDir, 'browser-integ-counts'),
              './tests/config/browser-integration-server.go',
            ], { cwd: root });

            server = childProcess.spawn(
              path.join(tmpDir, 'browser-integ-counts'),
              [dbPath, String(serverPort)],
              {
                cwd: root,
                env: { ...process.env, DEV: '' },
                stdio: ['ignore', 'pipe', 'pipe'],
              },
            );
            server.stdout.on('data', chunk => process.stdout.write(chunk));
            server.stderr.on('data', chunk => process.stderr.write(chunk));
            await waitForServer(serverPort);

            const page = await ctx.context.newPage();

            // Count how many times /api/counts is called during the scenario.
            let countsCallCount = 0;

            // Intercept /api/counts to inject a stale-then-fresh race.
            //
            // The first call (triggered by the pageshow handler's top-level
            // updateCounts, which was removed by the T1 fix) would return the
            // stale count.  We hold it open and let the second call (inside
            // restoreFromState) resolve first with the fresh count.  After the
            // fresh response has landed we release the stale one.
            //
            // With the T1 fix in place, only ONE call is made per pageshow
            // (the one inside restoreFromState).  The unblockFirst resolver
            // will never be invoked in that case — the test asserts this by
            // checking countsCallCount === 1.
            let unblockFirstCountsCall = null;
            let firstCountsCallResolved = false;

            // Real counts values: stale = 12 (pre-read), fresh = 11 (post-read).
            // The stale intercept body is injected only if a race would occur;
            // the fresh count is always what the actual server returns.
            const staleCount = 12;
            const freshCount = 11;

            await page.route(`http://localhost:${serverPort}/api/counts`, async (route) => {
              if (route.request().method() !== 'GET') {
                await route.continue();
                return;
              }
              countsCallCount++;
              const callIndex = countsCallCount;

              if (callIndex === 1 && !firstCountsCallResolved) {
                // Hold the first call open — return stale data once unblocked.
                await new Promise(resolve => { unblockFirstCountsCall = resolve; });
                firstCountsCallResolved = true;
                await route.fulfill({
                  status: 200,
                  contentType: 'application/json',
                  body: JSON.stringify({
                    unread: staleCount,
                    starred: 0,
                    queue: 0,
                    alerts: 0,
                    categories: {},
                    feeds: {},
                    feedErrors: {},
                  }),
                });
              } else {
                // Pass all other calls through to the real server.
                await route.continue();
              }
            });

            try {
              await page.setExtraHTTPHeaders({
                'X-Exedev-Userid': 'browser-integ-user',
                'X-Exedev-Email': 'browser-integ@example.com',
              });
              await page.setViewportSize({ width: 900, height: 520 });
              await page.goto(`http://localhost:${serverPort}/`, { waitUntil: 'domcontentloaded' });
              await page.waitForSelector('.article-card');

              // Read the initial badge text (should reflect staleCount or real count).
              const getBadgeText = async () => page.evaluate(() => {
                const badge = document.querySelector('[data-count="unread"]');
                return badge ? badge.textContent.trim() : null;
              });

              // Mark one article read by clicking it, which sets the
              // return-from-article-list marker and flushes via keepalive.
              const allIds = await page.$$eval('.article-card', cards => cards.map(card => card.dataset.id));
              const clickId = allIds[0];
              const clickSelector = `.article-card[data-id="${clickId}"] .article-body.clickable`;
              await page.locator(clickSelector).click();
              await page.waitForURL(new RegExp('/article/\\d+$'));

              // Now go back.  The pageshow handler will fire.
              // Reset the countsCallCount so we only track calls from pageshow.
              countsCallCount = 0;
              firstCountsCallResolved = false;
              unblockFirstCountsCall = null;

              await page.goBack({ waitUntil: 'domcontentloaded' });
              await page.waitForSelector('.article-card');

              // Wait a brief moment for the synchronous portion of pageshow to
              // fire (so all /api/counts calls are in-flight) before we release
              // the held first call.
              await page.waitForTimeout(200);

              // Release the stale call (if it was held — with the T1 fix it
              // won't exist, so this is a no-op).
              if (unblockFirstCountsCall) {
                unblockFirstCountsCall();
              }

              // Wait for restoreFromState to complete and the badge to settle.
              await page.waitForTimeout(800);

              const finalBadgeText = await getBadgeText();

              return {
                staleCount,
                freshCount,
                finalBadgeText,
                countsCallCount,
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
