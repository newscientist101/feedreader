/**
 * Browser integration regression test for duplicate-updateCounts race (T1 fix).
 *
 * Before the T1 fix, the pageshow handler fired an unawaited updateCounts()
 * at the top of the return-from-article path in addition to the one inside
 * restoreFromState().  When the first (stale) call resolved after the second
 * (fresh) one, the unread badge would show the pre-batch-read count.
 *
 * This test:
 * 1. Intercepts /api/counts to hold the first call (stale) open.
 * 2. Navigates to an article and goes Back.
 * 3. Releases the stale response only after the fresh one has landed.
 * 4. Asserts the badge shows the fresh count and that only one
 *    /api/counts call was made during the pageshow path (T1 fix invariant).
 */
import { describe, expect, test } from 'vitest';
import { commands } from 'vitest/browser';

describe('pageshow duplicate-updateCounts race', () => {
  test(
    'badge shows fresh count after return-from-article even when stale /api/counts resolves last',
    async () => {
      const result = await commands.runDuplicateUpdateCountsRaceScenario();

      // With the T1 fix, exactly one /api/counts call should be made during
      // the return-from-article pageshow path (the one inside restoreFromState).
      // Without the fix, two calls would be made: one at the top of pageshow
      // and one inside restoreFromState.
      expect(
        result.countsCallCount,
        'exactly one /api/counts call should be made per pageshow (T1 invariant)',
      ).toBe(1);

      // The badge should show the fresh count (the real server response after
      // the article was marked read), not the stale injected value.
      // freshCount is the real server response; staleCount is 12 (all unread).
      expect(
        result.finalBadgeText,
        'badge should reflect fresh count, not stale injected value',
      ).not.toBe(String(result.staleCount));
    },
    30000,
  );
});
