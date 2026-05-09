/**
 * Browser integration regression test for fast-Back pending-read replay.
 *
 * Exercises the race where the user returns from /article/{id} before the
 * navigation-time keepalive batch-read has persisted on the server. Playwright
 * intercepts POST /api/articles/batch-read for the navigation flush and holds
 * it until after Back completes. The pageshow handler must detect the pending
 * IDs in sessionStorage, immediately hide them in the DOM, and re-POST them.
 *
 * After the replay, all auto-mark-read (scroll) and click-read articles must
 * be absent from both the live DOM and /api/articles/unread.
 */
import { describe, expect, test } from 'vitest';
import { commands } from 'vitest/browser';

function expectState(checkpoint) {
  expect(
    checkpoint.expectedRead.length,
    `${checkpoint.label}: should have at least one read article`,
  ).toBeGreaterThan(0);

  for (const id of checkpoint.expectedRead) {
    expect(
      checkpoint.visible,
      `${checkpoint.label}: read article ${id} should be hidden from the DOM`,
    ).not.toContain(id);
    expect(
      checkpoint.apiIds,
      `${checkpoint.label}: read article ${id} should not appear in /api/articles/unread`,
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

describe('fast Back: pending-read replay after intercepted navigation batch-read', () => {
  test(
    'replays pending IDs via body click when navigation batch-read is delayed past Back',
    async () => {
      const result = await commands.runFastBackPendingReadReplayScenario();

      expect(result.allIds.length).toBe(12);
      expect(result.allTitles[0]).toBe('Browser Integration Article 01');

      // Clicked article must be in the read set.
      expect(
        result.afterBack.expectedRead,
        'clicked article should be tracked as read after replay',
      ).toContain(result.clickedId);

      // All scroll-read articles must also be absent.
      for (const id of result.scrolledReadIds) {
        expect(
          result.afterBack.expectedRead,
          `scroll-read article ${id} should be in expectedRead`,
        ).toContain(id);
      }

      expectState({ ...result.afterBack, label: 'after back (body click, fast Back)' });
    },
    30000,
  );

  test(
    'replays pending IDs via title-link click when navigation batch-read is delayed past Back',
    async () => {
      const result = await commands.runFastBackPendingReadReplayScenario({ clickMode: 'title-link' });

      expect(result.allIds.length).toBe(12);
      expect(result.allTitles[0]).toBe('Browser Integration Article 01');

      expect(
        result.afterBack.expectedRead,
        'clicked article should be tracked as read after replay',
      ).toContain(result.clickedId);

      for (const id of result.scrolledReadIds) {
        expect(
          result.afterBack.expectedRead,
          `scroll-read article ${id} should be in expectedRead`,
        ).toContain(id);
      }

      expectState({ ...result.afterBack, label: 'after back (title-link click, fast Back)' });
    },
    30000,
  );
});
