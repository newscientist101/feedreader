/**
 * Header layout tests at narrow mobile widths (375px, 430px).
 *
 * At mobile widths (<=768px) the sidebar is hidden and the header uses
 * a wrapped layout: h1 takes full width on its own row, view-toggle
 * and header-actions share a second row. Tests verify:
 * - Header controls don't overlap
 * - Search box has usable width
 * - View toggles are visible with adequate tap targets
 * - Feed names of varying lengths don't cause horizontal overflow
 *
 * Requires a running server on :8000 behind mitm proxy on :3000.
 * See tests/layout/README.md for setup instructions.
 */
import { afterAll, beforeAll, describe, expect, test } from 'vitest';
import {
  assertNoOverlap,
  assertVisible,
  getFeedName,
  measureForNamesMultiWidth,
  setFeedName,
} from './helpers.js';

const FEED_ID = 3;
const FEED_URL = `/feed/${FEED_ID}`;

/** Selectors for header layout elements */
const HEADER_SELECTORS = [
  '.view-header',
  '.view-header h1',
  '.view-toggle',
  '.header-actions',
  '#search',
  '.dropdown.feed-overflow-menu',
];

/** Test feed names at various lengths with realistic mixed-case text */
const TEST_NAMES = [
  { chars: 10, name: 'Hacker New' },
  { chars: 20, name: 'Hacker News Frontpag' },
  { chars: 26, name: 'Ars Technica - All Content' },
  { chars: 27, name: 'simular-ai/Agent-S releases' },
  { chars: 30, name: 'Machine Learning Weekly Diges' },
  { chars: 40, name: 'Artificial Intelligence Research Papers' },
  { chars: 50, name: 'The Very Long Named Technology and Science FeedXYZ' },
];

const MOBILE_WIDTHS = [375, 430];

describe('header layout at mobile widths', () => {
  let originalName;
  /** @type {Record<string, Record<number, Record<string, object|null>>>} */
  let allRects;

  beforeAll(async () => {
    originalName = await getFeedName(FEED_ID);
    allRects = await measureForNamesMultiWidth(
      FEED_URL, HEADER_SELECTORS, FEED_ID,
      TEST_NAMES.map(t => t.name), MOBILE_WIDTHS, 720,
    );
  });

  afterAll(async () => {
    await setFeedName(FEED_ID, originalName);
  });

  for (const width of MOBILE_WIDTHS) {
    describe(`at ${width}px`, () => {
      for (const { chars, name } of TEST_NAMES) {
        test(`${chars}-char name: "${name}"`, () => {
          const rects = allRects[name][width];

          // View toggle should be visible at mobile
          assertVisible(rects, '.view-toggle');

          // Header actions should be visible
          assertVisible(rects, '.header-actions');

          // View toggle and header actions should not overlap
          assertNoOverlap(rects, '.view-toggle', '.header-actions');

          // h1 should not overlap view toggle or header actions
          // (at mobile, h1 is on its own row so they shouldn't overlap)
          assertNoOverlap(rects, '.view-header h1', '.view-toggle');
          assertNoOverlap(rects, '.view-header h1', '.header-actions');

          // Search should be visible and have usable width
          assertVisible(rects, '#search');
          const search = rects['#search'];
          if (search) {
            // At mobile, search-box gets good space (>= 100px)
            expect(
              search.width,
              `Search too narrow at ${width}px (${search.width}px)`,
            ).toBeGreaterThanOrEqual(100);
          }

          // No horizontal overflow: header-level elements within viewport
          const header = rects['.view-header'];
          if (header) {
            expect(
              header.x + header.width,
              `Header right edge (${(header.x + header.width).toFixed(0)}px) exceeds viewport (${width}px)`,
            ).toBeLessThanOrEqual(width + 2);
          }
        });
      }
    });
  }
});
