import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { initSearch, _resetSearchState } from './search.js';

vi.mock('./articles.js');

vi.mock('./toast.js');

import { renderArticles, applyUserPreferences, setShowingHiddenArticles } from './articles.js';
import { showToast } from './toast.js';

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
    // Ensure location is restorable
    Object.defineProperty(window, 'location', {
        value: { ...window.location, pathname: '/' },
        writable: true,
        configurable: true,
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
        // setShowingHiddenArticles(false) should be called when restoring
        expect(setShowingHiddenArticles).toHaveBeenCalledWith(false);
        // original HTML should be restored
        expect(document.getElementById('articles-list').innerHTML).toContain('Original');
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

    it('treats whitespace-only input as short query', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        initSearch();

        const input = document.getElementById('search');
        input.value = '   ';
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

    it('calls setShowingHiddenArticles(true) on successful search', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        fetch.mockResolvedValue({ ok: true, json: async () => [{ id: 1 }] });

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(setShowingHiddenArticles).toHaveBeenCalledWith(true);
    });

    it('scopes search to current feed context', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        window.location.pathname = '/feed/42';

        initSearch();
        const input = document.getElementById('search');
        input.value = 'query';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=query&feed_id=42',
            expect.any(Object)
        );
    });

    it('scopes search to current category context', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        window.location.pathname = '/category/7';

        initSearch();
        const input = document.getElementById('search');
        input.value = 'query';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=query&category_id=7',
            expect.any(Object)
        );
    });

    it('does not scope search on non-feed/category paths', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        window.location.pathname = '/settings';

        initSearch();
        const input = document.getElementById('search');
        input.value = 'query';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=query',
            expect.any(Object)
        );
    });

    it('aborts previous search when new input arrives', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        let abortSignal;
        fetch.mockImplementationOnce((_url, opts) => {
            abortSignal = opts.signal;
            return new Promise(() => {}); // never resolves
        });

        initSearch();
        const input = document.getElementById('search');

        // First search
        input.value = 'first';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);
        expect(abortSignal).toBeDefined();

        // Second search should abort the first
        input.value = 'second';
        input.dispatchEvent(new Event('input'));

        // The first signal should be aborted
        expect(abortSignal.aborted).toBe(true);

        await vi.advanceTimersByTimeAsync(300);

        // The second fetch should have been called
        expect(fetch).toHaveBeenCalledTimes(2);
    });

    it('silently ignores AbortError (no toast, no console.error)', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        const abortErr = new DOMException('The operation was aborted.', 'AbortError');
        fetch.mockRejectedValue(abortErr);
        const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(consoleSpy).not.toHaveBeenCalled();
        expect(showToast).not.toHaveBeenCalled();
        expect(renderArticles).not.toHaveBeenCalled();
    });

    it('handles non-ok response with error field', async () => {
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
        expect(showToast).toHaveBeenCalledWith('Search failed');
        expect(renderArticles).not.toHaveBeenCalled();
    });

    it('handles non-ok response without error field (fallback message)', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        fetch.mockResolvedValue({
            ok: false,
            json: async () => ({}),
        });
        const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(consoleSpy).toHaveBeenCalledWith('Search failed:', expect.objectContaining({
            message: 'Search failed',
        }));
        expect(showToast).toHaveBeenCalledWith('Search failed');
    });

    it('handles network error (fetch rejection)', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        fetch.mockRejectedValue(new TypeError('Failed to fetch'));
        const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(consoleSpy).toHaveBeenCalledWith('Search failed:', expect.any(TypeError));
        expect(showToast).toHaveBeenCalledWith('Search failed');
    });

    it('encodes special characters in search query', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';

        initSearch();
        const input = document.getElementById('search');
        input.value = 'foo & bar';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=foo%20%26%20bar',
            expect.any(Object)
        );
    });

    it('does not save originalHTML when articles-list is missing', async () => {
        document.body.innerHTML = '<input id="search">';
        // No #articles-list element

        initSearch();
        const input = document.getElementById('search');
        input.value = 'test';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        // Should still search without error
        expect(fetch).toHaveBeenCalled();
    });

    it('does not restore when short query typed but no prior search', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"><p>initial</p></div>';

        initSearch();
        // Clear any calls from beforeEach/_resetSearchState
        vi.clearAllMocks();

        const input = document.getElementById('search');
        // Type short query without ever doing a full search
        input.value = 'a';
        input.dispatchEvent(new Event('input'));

        await vi.advanceTimersByTimeAsync(300);

        // originalHTML is null, so no restoration should happen
        expect(applyUserPreferences).not.toHaveBeenCalled();
        expect(setShowingHiddenArticles).not.toHaveBeenCalled();
    });

    it('debounces rapid input events', async () => {
        document.body.innerHTML = '<input id="search"><div id="articles-list"></div>';
        fetch.mockResolvedValue({ ok: true, json: async () => [] });

        initSearch();
        const input = document.getElementById('search');

        // Rapid-fire inputs
        input.value = 'te';
        input.dispatchEvent(new Event('input'));
        await vi.advanceTimersByTimeAsync(100);

        input.value = 'tes';
        input.dispatchEvent(new Event('input'));
        await vi.advanceTimersByTimeAsync(100);

        input.value = 'test';
        input.dispatchEvent(new Event('input'));
        await vi.advanceTimersByTimeAsync(300);

        // Only the final value should trigger a fetch
        expect(fetch).toHaveBeenCalledTimes(1);
        expect(fetch).toHaveBeenCalledWith(
            '/api/search?q=test',
            expect.any(Object)
        );
    });
});
