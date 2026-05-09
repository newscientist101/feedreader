import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    markReturningFromArticleList, consumeReturningFromArticleList,
    mergePendingReadIds, peekPendingReadIds, clearPendingReadIds,
} from './nav-state.js';

beforeEach(() => {
    sessionStorage.clear();
    vi.restoreAllMocks();
});

describe('article-list return marker', () => {
    it('stores and consumes the marker once', () => {
        expect(markReturningFromArticleList()).toBe(true);
        expect(consumeReturningFromArticleList()).toBe(true);
        expect(consumeReturningFromArticleList()).toBe(false);
    });

    it('gracefully handles unavailable sessionStorage on write', () => {
        const originalSessionStorage = globalThis.sessionStorage;
        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: {
                setItem() { throw new Error('blocked'); },
            },
        });

        expect(markReturningFromArticleList()).toBe(false);

        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: originalSessionStorage,
        });
    });

    it('gracefully handles unavailable sessionStorage on read', () => {
        const originalSessionStorage = globalThis.sessionStorage;
        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: {
                getItem() { throw new Error('blocked'); },
            },
        });

        expect(consumeReturningFromArticleList()).toBe(false);

        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: originalSessionStorage,
        });
    });
});

describe('pending read IDs', () => {
    it('merges IDs into an empty store', () => {
        mergePendingReadIds([1, 2, 3]);
        const ids = peekPendingReadIds();
        expect(ids).toEqual(new Set([1, 2, 3]));
    });

    it('deduplicates IDs across multiple merges', () => {
        mergePendingReadIds([1, 2]);
        mergePendingReadIds([2, 3]);
        const ids = peekPendingReadIds();
        expect(ids).toEqual(new Set([1, 2, 3]));
    });

    it('peekPendingReadIds does not clear the store', () => {
        mergePendingReadIds([5]);
        peekPendingReadIds();
        expect(peekPendingReadIds()).toEqual(new Set([5]));
    });

    it('clearPendingReadIds removes the key', () => {
        mergePendingReadIds([7]);
        clearPendingReadIds();
        expect(peekPendingReadIds()).toEqual(new Set());
    });

    it('returns empty Set when storage is empty', () => {
        expect(peekPendingReadIds()).toEqual(new Set());
    });

    it('returns empty Set on invalid JSON without throwing', () => {
        sessionStorage.setItem('feedreader:article-list-pending-read-ids', '{not valid json}');
        expect(() => peekPendingReadIds()).not.toThrow();
        expect(peekPendingReadIds()).toEqual(new Set());
    });

    it('returns empty Set when stored value is not an array', () => {
        sessionStorage.setItem('feedreader:article-list-pending-read-ids', '{"a":1}');
        expect(peekPendingReadIds()).toEqual(new Set());
    });

    it('gracefully handles storage exception in mergePendingReadIds', () => {
        const original = globalThis.sessionStorage;
        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: { getItem() { return null; }, setItem() { throw new Error('blocked'); }, removeItem() {} },
        });
        expect(mergePendingReadIds([1])).toBe(false);
        Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: original });
    });

    it('gracefully handles storage exception in clearPendingReadIds', () => {
        const original = globalThis.sessionStorage;
        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: { removeItem() { throw new Error('blocked'); } },
        });
        expect(clearPendingReadIds()).toBe(false);
        Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: original });
    });

    it('gracefully handles storage exception in peekPendingReadIds', () => {
        const original = globalThis.sessionStorage;
        Object.defineProperty(globalThis, 'sessionStorage', {
            configurable: true,
            value: { getItem() { throw new Error('blocked'); } },
        });
        expect(peekPendingReadIds()).toEqual(new Set());
        Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: original });
    });
});
