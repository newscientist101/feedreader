/**
 * Header layout tests at 1920px (wide desktop).
 *
 * At wide desktop width (>1024px breakpoint) the full header is shown:
 * mark-read dropdown is visible, view-toggle labels are shown, and
 * feed-specific action buttons appear. The overflow menu is hidden.
 *
 * Tests verify:
 * - All header controls visible: h1, view-toggle (4 buttons), search,
 *   mark-read dropdown
 * - No overlap between any header sections
 * - Feed names of all tested lengths fit without layout breakage
 * - All controls are on a single row (no wrapping)
 *
 * Requires a running server on :8000 behind mitm proxy on :3000.
 * See tests/layout/README.md for setup instructions.
 */
import { afterAll, beforeAll, describe, expect, test } from 'vitest';
import {
  assertMinWidth,
  assertNoOverlap,
  assertVisible,
  getFeedName,
  measure,
  setFeedName,
} from './helpers.js';

const FEED_ID = 3;
const FEED_URL = `/feed/${FEED_ID}`;
const WIDTH = 1920;

/** Selectors for header layout elements */
const HEADER_SELECTORS = [
  '.view-header',
  '.view-header h1',
  '.view-toggle',
  '.view-toggle button[data-view="card"]',
  '.view-toggle button[data-view="list"]',
  '.view-toggle button[data-view="magazine"]',
  '.view-toggle button[data-view="expanded"]',
  '.header-actions',
  '#search',
  '.mark-read-dropdown',
];

/** Test feed names — realistic mixed-case text at various lengths */
const TEST_NAMES = [
  { chars: 10, name: 'Hacker New' },
  { chars: 20, name: 'Hacker News Frontpag' },
  { chars: 26, name: 'Ars Technica - All Content' },
  { chars: 27, name: 'simular-ai/Agent-S releases' },
  { chars: 30, name: 'Machine Learning Weekly Diges' },
  { chars: 40, name: 'Artificial Intelligence Research Papers' },
  { chars: 50, name: 'The Very Long Named Technology and Science FeedXYZ' },
];

describe('header layout at 1920px (wide desktop)', () => {
  let originalName;

  beforeAll(async () => {
    originalName = await getFeedName(FEED_ID);
  });

  afterAll(async () => {
    await setFeedName(FEED_ID, originalName);
  });

  for (const { chars, name } of TEST_NAMES) {
    test(`${chars}-char name: "${name}"`, async () => {
      await setFeedName(FEED_ID, name);
      const rects = await measure(FEED_URL, HEADER_SELECTORS, WIDTH, 720);

      // All header controls must be visible
      assertVisible(rects, '.view-header h1');
      assertVisible(rects, '.view-toggle');
      assertVisible(rects, '.header-actions');
      assertVisible(rects, '#search');

      // All 4 view toggle buttons must be visible
      assertVisible(rects, '.view-toggle button[data-view="card"]');
      assertVisible(rects, '.view-toggle button[data-view="list"]');
      assertVisible(rects, '.view-toggle button[data-view="magazine"]');
      assertVisible(rects, '.view-toggle button[data-view="expanded"]');

      // Mark-read dropdown should be visible at >1024px
      assertVisible(rects, '.mark-read-dropdown');

      // Search box should have at least CSS min-width (80px)
      assertMinWidth(rects, '#search', 80);

      // No overlap between major header sections
      assertNoOverlap(rects, '.view-header h1', '.view-toggle');
      assertNoOverlap(rects, '.view-header h1', '.header-actions');
      assertNoOverlap(rects, '.view-toggle', '.header-actions');

      // No overlap between search and mark-read dropdown
      assertNoOverlap(rects, '#search', '.mark-read-dropdown');

      // Header should not wrap: h1, view-toggle, and header-actions
      // should share the same vertical band (all within header bounds)
      const h1 = rects['.view-header h1'];
      const toggle = rects['.view-toggle'];
      const actions = rects['.header-actions'];
      const headerBox = rects['.view-header'];
      if (h1 && toggle && actions && headerBox) {
        const headerBottom = headerBox.y + headerBox.height;
        expect(
          h1.y + h1.height,
          'h1 extends below header (wrapped?)',
        ).toBeLessThanOrEqual(headerBottom + 2);
        expect(
          toggle.y + toggle.height,
          'view-toggle extends below header (wrapped?)',
        ).toBeLessThanOrEqual(headerBottom + 2);
        expect(
          actions.y + actions.height,
          'header-actions extends below header (wrapped?)',
        ).toBeLessThanOrEqual(headerBottom + 2);
      }

      // Header should not overflow the viewport
      if (headerBox) {
        expect(
          headerBox.x + headerBox.width,
          `Header right edge exceeds viewport`,
        ).toBeLessThanOrEqual(WIDTH + 2);
      }
    });
  }
});
