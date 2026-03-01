import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { initSearch, _resetSearchState } from './search.js';

// Mock articles.js
vi.mock('./articles.js', () => ({
    renderArticles: vi.fn(),
    applyUserPreferences: vi.fn(),
}));

import { renderArticles, applyUserPreferences } from './articles.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.useFakeTimers();
    vi.restoreAllMocks();
    vi.clearAllMocks();
    _resetSearchState();
    // Stub global fetch
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: true,
        json: async () => [],
    });
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    _resetSearchState();
});

describe('initSearch', () => {
    it('is a no-op when #search element is absent', () => {
        initSearch();
        // No error thrown, no listeners attached
    });

    it('attaches input listener to #search element', () => {
        document.body.innerHTML = '<input id="search">';
        const spy = vi.spyOn(document.getElementById('search'), 'addEventListener');
        initSearch();
        expect(spy).toHaveBeenCalledWith('input', expect.any(Function));
    });

    it('restores original HTML when query is cleared', async () => {
        document.body.innerHTML = `
            <input id="search">
            <div id="articles-list"><div class="original">Original</div></div>
        `;
        initSearch();
        const input = document.getElementById('search');

        // Simulate a search that replaces content
        input.value = 'test query';
        input.dispatchEvent(new Event('input'));

        // Advance past debounce
        await vi.advanceTimersByTimeAsync(300);
        expect(fetch).toHaveBeenCalled();

        // Now clear the input
        input.value = '';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        // applyUserPreferences should be called when restoring
        expect(applyUserPreferences).toHaveBeenCalled();
    });

    it('does not search for queries shorter than 2 chars', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        initSearch();

        const input = document.getElementById('search');
        input.value = 'a';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);
        expect(fetch).not.toHaveBeenCalled();
    });

    it('searches with debounce on valid query', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        const articles = [{ id: 1, title: 'Found' }];
        fetch.mockResolvedValue({ ok: true, json: async () => articles });

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        // Before debounce fires
        expect(fetch).not.toHaveBeenCalled();

        // After debounce
        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=test',
            expect.objectContaining({ signal: expect.any(AbortSignal) })
        );
        expect(renderArticles).toHaveBeenCalledWith(articles);
    });

    it('scopes search to current feed context', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        // Mock pathname
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/feed/42' },
            writable: true,
            configurable: true,
        });

        initSearch();
        const input = document.getElementById('search');
        input.value = 'query';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=query&feed_id=42',
            expect.any(Object)
        );

        // Restore
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
    });

    it('scopes search to current category context', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/category/7' },
            writable: true,
            configurable: true,
        });

        initSearch();
        const input = document.getElementById('search');
        input.value = 'query';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=query&category_id=7',
            expect.any(Object)
        );

        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
    });

    it('aborts previous search when new input arrives', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        let resolveFirst;
        const firstFetch = new Promise(r => { resolveFirst = r; });
        fetch.mockImplementationOnce((url, opts) => {
            // Return a promise that we control
            return firstFetch;
        });

        initSearch();
        const input = document.getElementById('search');

        // First search
        input.value = 'first';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        // Second search should abort the first
        input.value = 'second';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        // The second fetch should have been called
        expect(fetch).toHaveBeenCalledTimes(2);
    });

    it('handles fetch errors gracefully', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        fetch.mockResolvedValue({
            ok: false,
            json: async () => ({ error: 'Server error' }),
        });
        const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(consoleSpy).toHaveBeenCalledWith('Search failed:', expect.any(Error));
        expect(renderArticles).not.toHaveBeenCalled();
    });
});
