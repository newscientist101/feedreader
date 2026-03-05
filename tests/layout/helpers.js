/**
 * Layout test helpers for Playwright-based layout tests.
 *
 * These utilities work with real DOM bounding rectangles to detect
 * overlap, crushed elements, and broken layouts.
 */
import { expect } from '@playwright/test';

/**
 * Returns true if two bounding-box objects intersect.
 * A zero-area box (width or height === 0) never overlaps.
 */
export function overlaps(a, b) {
  if (a.width === 0 || a.height === 0 || b.width === 0 || b.height === 0) {
    return false;
  }
  const aRight = a.x + a.width;
  const aBottom = a.y + a.height;
  const bRight = b.x + b.width;
  const bBottom = b.y + b.height;
  return a.x < bRight && aRight > b.x && a.y < bBottom && aBottom > b.y;
}

/**
 * Assert that two elements (by CSS selector) do not overlap.
 * Skips the check silently if either element is not visible.
 */
export async function assertNoOverlap(page, selectorA, selectorB) {
  const elA = page.locator(selectorA).first();
  const elB = page.locator(selectorB).first();

  if ((await elA.count()) === 0 || (await elB.count()) === 0) return;

  const boxA = await elA.boundingBox();
  const boxB = await elB.boundingBox();
  if (!boxA || !boxB) return;

  expect(
    overlaps(boxA, boxB),
    `Expected no overlap between "${selectorA}" and "${selectorB}"\n` +
      `  A: ${JSON.stringify(boxA)}\n  B: ${JSON.stringify(boxB)}`
  ).toBe(false);
}

/**
 * Assert that an element has at least the given minimum width.
 * Skips silently if the element is not visible.
 */
export async function assertMinWidth(page, selector, minPx) {
  const el = page.locator(selector).first();
  if ((await el.count()) === 0) return;

  const box = await el.boundingBox();
  if (!box) return;

  expect(
    box.width,
    `Expected "${selector}" width >= ${minPx}px, got ${box.width}px`
  ).toBeGreaterThanOrEqual(minPx);
}

/**
 * Assert that an element is visible (non-zero width AND height).
 * Skips silently if the element is not present in the DOM.
 */
export async function assertVisible(page, selector) {
  const el = page.locator(selector).first();
  if ((await el.count()) === 0) {
    expect(null, `Expected "${selector}" to exist in the DOM`).not.toBeNull();
    return;
  }

  const box = await el.boundingBox();
  expect(
    box,
    `Expected "${selector}" to have a bounding box`
  ).not.toBeNull();

  expect(
    box.width > 0 && box.height > 0,
    `Expected "${selector}" to be visible (got ${box.width}x${box.height})`
  ).toBe(true);
}
