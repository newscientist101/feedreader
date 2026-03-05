/**
 * Header layout tests at 860px — the Goldilocks zone.
 *
 * This is the critical viewport width where the feedreader-wei bug
 * manifested: feed names of ~26-29 mixed-case characters caused view
 * toggles to disappear and the search box to collapse.
 *
 * At 860px (≤1024px breakpoint), the layout hides mark-read dropdown
 * and shows the overflow menu instead. View-toggle labels are hidden.
 *
 * Tests verify for each name length:
 * - .view-toggle has width > 0 and all 4 buttons are visible
 * - #search has width >= 80px (the CSS min-width floor)
 * - No overlap between h1, .view-toggle, .header-actions
 * - No overlap between search box and overflow menu
 * - h1 truncates with ellipsis when needed
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
const WIDTH = 860;

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
  '.dropdown.feed-overflow-menu',
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

describe('header layout at 860px (Goldilocks zone)', () => {
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

      // View toggle must be visible with non-zero width
      assertVisible(rects, '.view-toggle');

      // All 4 view toggle buttons must be visible
      // (This was the key feedreader-wei regression: buttons disappeared)
      assertVisible(rects, '.view-toggle button[data-view="card"]');
      assertVisible(rects, '.view-toggle button[data-view="list"]');
      assertVisible(rects, '.view-toggle button[data-view="magazine"]');
      assertVisible(rects, '.view-toggle button[data-view="expanded"]');

      // Search box must have at least the CSS min-width (80px)
      assertMinWidth(rects, '#search', 80);

      // No overlap between major header sections
      assertNoOverlap(rects, '.view-header h1', '.view-toggle');
      assertNoOverlap(rects, '.view-header h1', '.header-actions');
      assertNoOverlap(rects, '.view-toggle', '.header-actions');

      // No overlap between search and overflow menu
      assertNoOverlap(rects, '#search', '.dropdown.feed-overflow-menu');

      // Header should not overflow the viewport
      const header = rects['.view-header'];
      if (header) {
        expect(
          header.x + header.width,
          `Header right edge (${(header.x + header.width).toFixed(0)}px) exceeds viewport (${WIDTH}px)`,
        ).toBeLessThanOrEqual(WIDTH + 2);
      }

      // h1 should truncate rather than push siblings off-screen
      const h1 = rects['.view-header h1'];
      const toggle = rects['.view-toggle'];
      if (h1 && toggle) {
        // h1 right edge + gap should be <= toggle left edge (no overlap)
        expect(
          h1.x + h1.width,
          `h1 right edge (${(h1.x + h1.width).toFixed(0)}px) extends past view-toggle (${toggle.x.toFixed(0)}px)`,
        ).toBeLessThanOrEqual(toggle.x + 1);
      }
    });
  }
});
