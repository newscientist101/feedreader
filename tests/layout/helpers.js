/**
 * Layout test helpers.
 *
 * These helpers use the `measureLayout` custom command to get bounding
 * rects from a real Playwright page (not the Vitest iframe). This avoids
 * cross-origin restrictions while giving us real browser layout measurements.
 */
import { commands } from 'vitest/browser';
import { expect } from 'vitest';

/**
 * Returns true if two bounding boxes overlap.
 * @param {{ x: number, y: number, width: number, height: number }} a
 * @param {{ x: number, y: number, width: number, height: number }} b
 * @returns {boolean}
 */
export function overlaps(a, b) {
  if (!a || !b) return false;
  return (
    a.x < b.x + b.width &&
    a.x + a.width > b.x &&
    a.y < b.y + b.height &&
    a.y + a.height > b.y
  );
}

/**
 * Measure bounding rects for selectors on a page at a given viewport width.
 *
 * @param {string} url - Path to load (e.g. '/')
 * @param {string[]} selectors - CSS selectors to measure
 * @param {number} [viewportWidth=1280] - Viewport width in px
 * @param {number} [viewportHeight=720] - Viewport height in px
 * @returns {Promise<Record<string, {x:number,y:number,width:number,height:number}|null>>}
 */
export async function measure(url, selectors, viewportWidth, viewportHeight) {
  return commands.measureLayout(url, selectors, viewportWidth, viewportHeight);
}

/**
 * Measure bounding rects at multiple viewport widths on a single page load.
 * More efficient than calling measure() multiple times.
 *
 * @param {string} url - Path to load
 * @param {string[]} selectors - CSS selectors to measure
 * @param {number[]} widths - Viewport widths to test
 * @param {number} [viewportHeight=720] - Viewport height
 * @returns {Promise<Record<number, Record<string, object|null>>>}
 */
export async function measureMultiWidth(url, selectors, widths, viewportHeight) {
  return commands.measureLayoutMultiWidth(url, selectors, widths, viewportHeight);
}

/**
 * Get the current name of a feed.
 * @param {number} feedId
 * @returns {Promise<string>}
 */
export async function getFeedName(feedId) {
  return commands.getFeedName(feedId);
}

/**
 * Set the name of a feed via the API.
 * @param {number} feedId
 * @param {string} name
 * @returns {Promise<boolean>}
 */
export async function setFeedName(feedId, name) {
  return commands.setFeedName(feedId, name);
}

/**
 * Assert that two elements do not overlap at a given viewport width.
 *
 * @param {Record<string, object|null>} rects - Output from measure()
 * @param {string} selectorA
 * @param {string} selectorB
 */
export function assertNoOverlap(rects, selectorA, selectorB) {
  const a = rects[selectorA];
  const b = rects[selectorB];
  if (!a || !b) return; // skip if either element is absent
  expect(
    overlaps(a, b),
    `Expected no overlap between "${selectorA}" and "${selectorB}"\n` +
      `  A: {x:${a.x}, y:${a.y}, w:${a.width}, h:${a.height}}\n` +
      `  B: {x:${b.x}, y:${b.y}, w:${b.width}, h:${b.height}}`,
  ).toBe(false);
}

/**
 * Assert an element has at least a minimum width.
 *
 * @param {Record<string, object|null>} rects - Output from measure()
 * @param {string} selector
 * @param {number} minPx
 */
export function assertMinWidth(rects, selector, minPx) {
  const box = rects[selector];
  if (!box) return; // skip if element is absent
  expect(
    box.width,
    `Expected "${selector}" width (${box.width}px) >= ${minPx}px`,
  ).toBeGreaterThanOrEqual(minPx);
}

/**
 * Assert an element is visible (has non-zero width and height).
 *
 * @param {Record<string, object|null>} rects - Output from measure()
 * @param {string} selector
 */
export function assertVisible(rects, selector) {
  const box = rects[selector];
  expect(
    box,
    `Expected "${selector}" to exist and have a bounding box`,
  ).not.toBeNull();
  expect(
    box.width,
    `Expected "${selector}" to have non-zero width`,
  ).toBeGreaterThan(0);
  expect(
    box.height,
    `Expected "${selector}" to have non-zero height`,
  ).toBeGreaterThan(0);
}

/**
 * Measure layout for multiple feed names on a single Playwright page.
 * Much faster than calling measure() per name since it avoids page open/close overhead.
 *
 * @param {string} url - Path to load
 * @param {string[]} selectors - CSS selectors to measure
 * @param {number} feedId - Feed ID to rename between measurements
 * @param {string[]} names - Feed names to test
 * @param {number} viewportWidth - Viewport width
 * @param {number} [viewportHeight=720] - Viewport height
 * @returns {Promise<Record<string, Record<string, object|null>>>}
 */
export async function measureForNames(url, selectors, feedId, names, viewportWidth, viewportHeight) {
  return commands.measureLayoutForNames(url, selectors, feedId, names, viewportWidth, viewportHeight);
}

/**
 * Measure layout for multiple feed names at multiple viewport widths on a single page.
 *
 * @param {string} url - Path to load
 * @param {string[]} selectors - CSS selectors to measure
 * @param {number} feedId - Feed ID to rename between measurements
 * @param {string[]} names - Feed names to test
 * @param {number[]} widths - Viewport widths to test
 * @param {number} [viewportHeight=720] - Viewport height
 * @returns {Promise<Record<string, Record<number, Record<string, object|null>>>>}
 */
export async function measureForNamesMultiWidth(url, selectors, feedId, names, widths, viewportHeight) {
  return commands.measureLayoutForNamesMultiWidth(url, selectors, feedId, names, widths, viewportHeight);
}
