import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    enqueueRead,
    peekIds,
    size,
    clear,
    flush,
    replay,
    _resetReadQueueState,
} from './read-queue.js';

function makeFetchResponse(body, { ok = true, status = 200 } = {}) {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    });
}

beforeEach(() => {
    sessionStorage.clear();
    _resetReadQueueState();
    vi.restoreAllMocks();
});

describe('enqueueRead', () => {
    it('deduplicates: enqueue 1, 2, 1 → Set([1, 2])', () => {
        enqueueRead(1);
        enqueueRead(2);
        enqueueRead(1);
        expect(peekIds()).toEqual(new Set([1, 2]));
    });

    it('persists across module state reset (simulated reload via _resetReadQueueState + re-read)', () => {
        enqueueRead(42);
        // Only reset module-scope vars; sessionStorage survives
        _resetReadQueueState();
        // Do NOT clear sessionStorage this time
        sessionStorage.setItem('feedreader:read-queue', JSON.stringify([42]));
        expect(peekIds()).toEqual(new Set([42]));
    });

    it('ignores non-finite values', () => {
        enqueueRead(NaN);
        enqueueRead(Infinity);
        enqueueRead('abc');
        expect(size()).toBe(0);
    });

    it('coerces numeric strings', () => {
        enqueueRead('7');
        expect(peekIds().has(7)).toBe(true);
    });
});

describe('flush', () => {
    it('resolves true immediately on empty queue without calling fetch', async () => {
        vi.spyOn(globalThis, 'fetch');
        const result = await flush();
        expect(result).toBe(true);
        expect(fetch).not.toHaveBeenCalled();
    });

    it('success path: POSTs ids, clears only those ids, returns true', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({ ok: true }));
        enqueueRead(1);
        enqueueRead(2);

        const result = await flush();

        expect(result).toBe(true);
        expect(fetch).toHaveBeenCalledTimes(1);
        expect(fetch).toHaveBeenCalledWith(
            '/api/articles/batch-read',
            expect.objectContaining({
                method: 'POST',
                body: expect.stringContaining('1'),
            }),
        );
        expect(peekIds().size).toBe(0);
    });

    it('snapshot-and-remove: id added during in-flight POST is preserved', async () => {
        let resolveFlush;
        vi.spyOn(globalThis, 'fetch').mockReturnValue(
            new Promise(resolve => { resolveFlush = resolve; }),
        );

        enqueueRead(1);
        enqueueRead(2);
        const flushPromise = flush(); // started but not awaited

        // Add id 3 while fetch is in flight
        enqueueRead(3);

        // Now resolve the fetch with 200
        resolveFlush(makeFetchResponse({ ok: true }));
        await flushPromise;

        // Only 1 and 2 should have been removed; 3 should still be there
        expect(peekIds()).toEqual(new Set([3]));
    });

    it('HTTP 500 leaves queue unchanged, returns false', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue(
            makeFetchResponse('error', { ok: false, status: 500 }),
        );
        enqueueRead(10);

        const result = await flush();

        expect(result).toBe(false);
        expect(peekIds()).toEqual(new Set([10]));
    });

    it('thrown error leaves queue unchanged, returns false', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('network'));
        enqueueRead(99);

        const result = await flush();

        expect(result).toBe(false);
        expect(peekIds()).toEqual(new Set([99]));
    });

    it('passes keepalive:true to fetch when option set', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({}));
        enqueueRead(5);

        await flush({ keepalive: true });

        expect(fetch).toHaveBeenCalledWith(
            '/api/articles/batch-read',
            expect.objectContaining({ keepalive: true }),
        );
    });

    it('does NOT set keepalive when option is false/absent', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({}));
        enqueueRead(6);

        await flush();

        const callArgs = fetch.mock.calls[0][1];
        expect(callArgs.keepalive).toBeUndefined();
    });
});

describe('replay', () => {
    it('is an alias for flush without keepalive', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({}));
        enqueueRead(7);

        const result = await replay();

        expect(result).toBe(true);
        const callArgs = fetch.mock.calls[0][1];
        expect(callArgs.keepalive).toBeUndefined();
    });
});

describe('sessionStorage failure fallback', () => {
    function mockSetItemToThrow() {
        const original = globalThis.sessionStorage;
        const mock = {
            _store: {},
            getItem(k) { return this._store[k] ?? null; },
            setItem() { throw new Error('QuotaExceeded'); },
            removeItem(k) { delete this._store[k]; },
            clear() { this._store = {}; },
        };
        Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: mock });
        return () => {
            Object.defineProperty(globalThis, 'sessionStorage', { configurable: true, value: original });
        };
    }

    it('falls back to memory when setItem throws: enqueue still works', () => {
        const restore = mockSetItemToThrow();
        try {
            enqueueRead(11);
            expect(peekIds()).toEqual(new Set([11]));
        } finally {
            restore();
        }
    });

    it('flush still POSTs when using memory fallback', async () => {
        const restore = mockSetItemToThrow();
        try {
            vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({}));
            enqueueRead(12);

            const result = await flush();

            expect(result).toBe(true);
            expect(fetch).toHaveBeenCalledOnce();
            const body = JSON.parse(fetch.mock.calls[0][1].body);
            expect(body.ids).toContain(12);
        } finally {
            restore();
        }
    });

    it('peekIds reflects state after memory-mode enqueue', () => {
        const restore = mockSetItemToThrow();
        try {
            enqueueRead(13);
            enqueueRead(14);
            enqueueRead(13); // duplicate
            expect(peekIds()).toEqual(new Set([13, 14]));
        } finally {
            restore();
        }
    });
});

describe('size and clear', () => {
    it('size returns count of pending ids', () => {
        enqueueRead(1);
        enqueueRead(2);
        expect(size()).toBe(2);
    });

    it('clear empties the queue', () => {
        enqueueRead(1);
        clear();
        expect(size()).toBe(0);
        expect(peekIds().size).toBe(0);
    });
});
