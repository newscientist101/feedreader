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
