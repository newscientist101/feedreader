/**
 * Smoke test: verify the layout test infrastructure works.
 *
 * Requires a running server on :8000 behind a mitm proxy on :3000.
 * See tests/layout/README.md for setup instructions.
 */
import { describe, expect, test } from 'vitest';
import { commands } from 'vitest/browser';
import { assertVisible, measure } from './helpers.js';

describe('layout test infrastructure', () => {
  test('fetchPageHTML returns HTML from the running app', async () => {
    const html = await commands.fetchPageHTML('/');
    expect(html).toContain('<body');
    expect(html).toContain('FeedReader');
  });

  test('measureLayout returns bounding rects for elements', async () => {
    const rects = await measure('/', ['body'], 1280, 720);
    expect(rects).toHaveProperty('body');
    assertVisible(rects, 'body');
  });
});
