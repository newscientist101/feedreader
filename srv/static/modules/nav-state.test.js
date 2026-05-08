import { describe, it, expect, beforeEach, vi } from 'vitest';
import { markReturningFromArticleList, consumeReturningFromArticleList } from './nav-state.js';

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
