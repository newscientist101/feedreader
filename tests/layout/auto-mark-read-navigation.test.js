/**
 * Browser integration regression test for auto-mark-read and Back navigation.
 *
 * Runs against an isolated feedreader server/SQLite DB fixture. The scenario is:
 * root unread view, autoMarkRead=true, hideReadArticles=hide; scroll past some
 * articles; click an internal article; go back; scroll again. At each checkpoint
 * the test tracks which fixture articles should still be visible vs already read.
 */
import { describe, expect, test } from 'vitest';
import { commands } from 'vitest/browser';

function expectState(checkpoint) {
  expect(
    checkpoint.expectedRead.length,
    `${checkpoint.label}: should have marked at least one article read`,
  ).toBeGreaterThan(0);

  for (const id of checkpoint.expectedRead) {
    expect(
      checkpoint.visible,
      `${checkpoint.label}: read article ${id} should be hidden from the DOM view`,
    ).not.toContain(id);
    expect(
      checkpoint.apiIds,
      `${checkpoint.label}: read article ${id} should not be returned by /api/articles/unread`,
    ).not.toContain(id);
  }

  for (const id of checkpoint.expectedVisible) {
    expect(
      checkpoint.visible,
      `${checkpoint.label}: unread article ${id} should remain visible`,
    ).toContain(id);
    expect(
      checkpoint.apiIds,
      `${checkpoint.label}: unread article ${id} should remain in /api/articles/unread`,
    ).toContain(id);
  }
}

describe('auto-mark-read scroll and article Back navigation', () => {
  test('keeps scrolled-read articles hidden after opening an article body and going back', async () => {
    const result = await commands.runAutoMarkReadBackNavigationScenario();

    expect(result.allIds.length).toBe(12);
    expect(result.allTitles[0]).toBe('Browser Integration Article 01');

    expectState(result.afterFirstScroll);

    expect(
      result.afterBack.expectedRead,
      'after back: clicked article should be tracked as read',
    ).toContain(result.clickedId);
    expect(
      result.afterBack.apiIds,
      'after back: clicked article should not be returned by /api/articles/unread',
    ).not.toContain(result.clickedId);

    const afterBackScrolledOnly = {
      ...result.afterBack,
      expectedRead: result.afterFirstScroll.expectedRead,
      expectedVisible: result.afterBack.expectedVisible.filter(id => id !== result.clickedId),
    };
    expectState(afterBackScrolledOnly);

    expect(
      result.afterSecondScroll.expectedRead.length,
      'second scroll should mark more articles read or keep the existing read set',
    ).toBeGreaterThanOrEqual(result.afterBack.expectedRead.length);
    expectState(result.afterSecondScroll);
  }, 30000);

  test('keeps scrolled-read articles hidden after clicking article title link and going back', async () => {
    const result = await commands.runAutoMarkReadBackNavigationScenario({ clickMode: 'title-link' });

    expect(result.allIds.length).toBe(12);
    expect(result.allTitles[0]).toBe('Browser Integration Article 01');

    expectState(result.afterFirstScroll);

    expect(
      result.afterBack.expectedRead,
      'after back: clicked article should be tracked as read',
    ).toContain(result.clickedId);
    expect(
      result.afterBack.apiIds,
      'after back: clicked article should not be returned by /api/articles/unread',
    ).not.toContain(result.clickedId);

    const afterBackScrolledOnly = {
      ...result.afterBack,
      expectedRead: result.afterFirstScroll.expectedRead,
      expectedVisible: result.afterBack.expectedVisible.filter(id => id !== result.clickedId),
    };
    expectState(afterBackScrolledOnly);

    expect(
      result.afterSecondScroll.expectedRead.length,
      'second scroll should mark more articles read or keep the existing read set',
    ).toBeGreaterThanOrEqual(result.afterBack.expectedRead.length);
    expectState(result.afterSecondScroll);
  }, 30000);
});
