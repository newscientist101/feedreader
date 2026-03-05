/**
 * Shared test utilities for the JS test suite.
 *
 * Import in test files:
 *   import { MockIntersectionObserver } from './test-helpers.js';
 */

/**
 * Minimal IntersectionObserver mock for happy-dom.
 * Stores observed elements and supports manual _fire() to trigger callbacks.
 *
 * Usage:
 *   window.IntersectionObserver = MockIntersectionObserver;
 */
export class MockIntersectionObserver {
    constructor(callback) {
        this._callback = callback;
        this._entries = [];
    }
    observe(el) { this._entries.push(el); }
    unobserve() {}
    disconnect() { this._entries = []; }
    _fire(entries) { this._callback(entries, this); }
}


/**
 * Build a fake Response-like object for mocking fetch() calls.
 *
 * Usage:
 *   vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({ articles: [] }));
 *   vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse(null, { ok: false, status: 500 }));
 */
export function makeFetchResponse(data = {}, { ok = true, status = 200 } = {}) {
    return { ok, status, json: () => Promise.resolve(data), text: () => Promise.resolve(JSON.stringify(data)) };
}

/**
 * Build a counts API response with sensible defaults.
 *
 * Usage:
 *   makeFetchResponse(makeCountsResponse({ unread: 10, starred: 3 }))
 */
export function makeCountsResponse(overrides = {}) {
    return {
        unread: 0, starred: 0, queue: 0, alerts: 0,
        categories: {}, feeds: {}, feedErrors: {},
        ...overrides,
    };
}

/**
 * Build an article object with sensible defaults.
 *
 * Usage:
 *   makeArticle({ id: 42, title: 'Custom', is_read: true })
 */
export function makeArticle(overrides = {}) {
    return {
        id: 1,
        title: 'Test Article',
        url: 'https://example.com/article',
        feed_id: 1,
        feed_title: 'Test Feed',
        published_at: '2025-01-01T00:00:00Z',
        is_read: false,
        is_starred: false,
        ...overrides,
    };
}

/** Flush the microtask queue by scheduling a macrotask. */
export function flushPromises() {
    return new Promise((resolve) => { setTimeout(resolve, 0); });
}
