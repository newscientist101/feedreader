import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    markReturningFromArticleList, consumeReturningFromArticleList,
    peekReturningFromArticleList, clearReturningFromArticleList,
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

    describe('peekReturningFromArticleList', () => {
        it('returns true when marker is set', () => {
            markReturningFromArticleList();
            expect(peekReturningFromArticleList()).toBe(true);
        });

        it('returns false when marker is not set', () => {
            expect(peekReturningFromArticleList()).toBe(false);
        });

        it('does NOT remove the marker', () => {
            markReturningFromArticleList();
            peekReturningFromArticleList();
            expect(peekReturningFromArticleList()).toBe(true);
        });

        it('gracefully handles unavailable sessionStorage', () => {
            const original = globalThis.sessionStorage;
            Object.defineProperty(globalThis, 'sessionStorage', {
                configurable: true,
                value: { getItem() { throw new Error('blocked'); } },
            });
            expect(peekReturningFromArticleList()).toBe(false);
            Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: original });
        });
    });

    describe('clearReturningFromArticleList', () => {
        it('removes the marker; subsequent peek returns false', () => {
            markReturningFromArticleList();
            expect(clearReturningFromArticleList()).toBe(true);
            expect(peekReturningFromArticleList()).toBe(false);
        });

        it('returns true when marker was not set', () => {
            expect(clearReturningFromArticleList()).toBe(true);
        });

        it('gracefully handles unavailable sessionStorage', () => {
            const original = globalThis.sessionStorage;
            Object.defineProperty(globalThis, 'sessionStorage', {
                configurable: true,
                value: { removeItem() { throw new Error('blocked'); } },
            });
            expect(clearReturningFromArticleList()).toBe(false);
            Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: original });
        });
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


