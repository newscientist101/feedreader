import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    updateEndOfArticlesIndicator, updatePaginationCursor, getPaginationUrl,
    loadMoreArticles, checkScrollForMore, initPagination,
    setPaginationState, getPaginationState, PAGE_SIZE,
    _resetPaginationState,
} from './pagination.js';
import { _resetArticleActionsState, setQueuedArticleIds, setQueuedIdsReady } from './article-actions.js';
import { getShowingHiddenArticles, buildArticleCardHtml } from './articles.js';
import { showToast } from './toast.js';
import { makeFetchResponse } from './test-helpers.js';

vi.mock('./toast.js');
vi.mock('./articles.js');

beforeEach(() => {
    vi.useFakeTimers();
    _resetPaginationState();
    _resetArticleActionsState();
    setQueuedArticleIds(new Set());
    setQueuedIdsReady(Promise.resolve());
    getShowingHiddenArticles.mockReturnValue(false);
    buildArticleCardHtml.mockImplementation(
        (a) => `<div class="article-card" data-id="${a.id}" data-sort-time="${a.published_at || a.fetched_at || ''}"><div class="article-content-preview"></div></div>`,
    );
    window.__settings = {};
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({ articles: [] }));
    document.body.innerHTML = `
        <div class="articles-view">
            <div id="articles-list" class="articles-list"></div>
            <div class="end-of-articles" id="end-of-articles"></div>
        </div>
    `;
    vi.clearAllMocks();
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
});

describe('PAGE_SIZE', () => {
    it('is 50', () => {
        expect(PAGE_SIZE).toBe(50);
    });
});

describe('setPaginationState / getPaginationState', () => {
    it('sets and gets pagination state', () => {
        setPaginationState({ cursorTime: '2025-01-01', cursorId: 42, done: true, loading: false });
        const state = getPaginationState();
        expect(state.cursorTime).toBe('2025-01-01');
        expect(state.cursorId).toBe(42);
        expect(state.done).toBe(true);
        expect(state.loading).toBe(false);
    });

    it('partially updates state', () => {
        setPaginationState({ done: true });
        expect(getPaginationState().done).toBe(true);
        expect(getPaginationState().cursorTime).toBeNull();
    });
});

describe('updatePaginationCursor', () => {
    it('sets cursor from last article', () => {
        updatePaginationCursor([
            { id: 1, published_at: '2025-01-01T00:00:00Z' },
            { id: 2, published_at: '2025-01-02T00:00:00Z' },
        ]);
        const state = getPaginationState();
        expect(state.cursorTime).toBe('2025-01-02T00:00:00Z');
        expect(state.cursorId).toBe(2);
    });

    it('does nothing for empty array', () => {
        updatePaginationCursor([]);
        expect(getPaginationState().cursorTime).toBeNull();
    });

    it('does nothing for null', () => {
        updatePaginationCursor(null);
        expect(getPaginationState().cursorTime).toBeNull();
    });

    it('does nothing for undefined', () => {
        updatePaginationCursor(undefined);
        expect(getPaginationState().cursorTime).toBeNull();
    });

    it('uses fetched_at when published_at is null', () => {
        updatePaginationCursor([
            { id: 1, fetched_at: '2025-06-01T00:00:00Z' },
        ]);
        expect(getPaginationState().cursorTime).toBe('2025-06-01T00:00:00Z');
    });
});

describe('getPaginationUrl', () => {
    it('returns unread URL for root', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        expect(getPaginationUrl()).toBe('/api/articles/unread');
    });

    it('returns feed URL for /feed/:id', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/feed/42' }, writable: true, configurable: true,
        });
        expect(getPaginationUrl()).toBe('/api/feeds/42/articles');
    });

    it('returns category URL for /category/:id', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/category/7' }, writable: true, configurable: true,
        });
        expect(getPaginationUrl()).toBe('/api/categories/7/articles');
    });

    it('returns starred URL for /starred', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/starred' }, writable: true, configurable: true,
        });
        expect(getPaginationUrl()).toBe('/api/articles/starred');
    });

    it('returns null for unknown paths', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/settings' }, writable: true, configurable: true,
        });
        expect(getPaginationUrl()).toBeNull();
    });
});

describe('updateEndOfArticlesIndicator', () => {
    it('shows indicator when pagination is done and has articles', () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card"></div>';
        setPaginationState({ done: true });
        updateEndOfArticlesIndicator();
        expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(true);
    });

    it('hides indicator when pagination is not done', () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card"></div>';
        setPaginationState({ done: false });
        updateEndOfArticlesIndicator();
        expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(false);
    });

    it('hides indicator when no articles exist', () => {
        setPaginationState({ done: true });
        updateEndOfArticlesIndicator();
        expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(false);
    });

    it('does nothing when element is missing', () => {
        document.body.innerHTML = '<div id="articles-list"></div>';
        setPaginationState({ done: true });
        updateEndOfArticlesIndicator(); // should not throw
    });
});

describe('loadMoreArticles', () => {
    it('does nothing when pagination is done', async () => {
        setPaginationState({ done: true });
        await loadMoreArticles();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('does nothing when loading', async () => {
        setPaginationState({ loading: true });
        await loadMoreArticles();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('does nothing without cursor', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        await loadMoreArticles();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('does nothing when URL is null (unknown path)', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/settings' }, writable: true, configurable: true,
        });
        setPaginationState({
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        await loadMoreArticles();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('appends new articles and observes them', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/feed/8' }, writable: true, configurable: true,
        });

        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        setQueuedIdsReady(Promise.resolve());

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({
                articles: [{
                    id: 1,
                    title: 'A',
                    is_read: 0,
                    is_starred: 0,
                    published_at: new Date().toISOString(),
                    content: '<p>hi</p>',
                }],
            }),
            text: async () => '',
        }));

        await loadMoreArticles();

        expect(document.querySelectorAll('.article-card').length).toBe(1);
        expect(document.querySelector('.article-content-preview')).not.toBeNull();
        // loading should be reset in finally block
        expect(getPaginationState().loading).toBe(false);
        // 1 article < PAGE_SIZE, so done should be true
        expect(getPaginationState().done).toBe(true);
        expect(showToast).not.toHaveBeenCalled();
    });

    it('sets done when API returns empty articles', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({ articles: [] }),
            text: async () => '',
        }));

        await loadMoreArticles();

        expect(getPaginationState().done).toBe(true);
        expect(getPaginationState().loading).toBe(false);
    });

    it('does not set done when articles.length equals PAGE_SIZE', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        setQueuedIdsReady(Promise.resolve());
        // Prevent checkScrollForMore in finally block from cascading
        Object.defineProperty(document.body, 'offsetHeight', { value: 100000, configurable: true });

        const articles = Array.from({ length: PAGE_SIZE }, (_, i) => ({
            id: i + 1,
            title: `Article ${i}`,
            is_read: 0,
            is_starred: 0,
            published_at: '2025-01-01T00:00:00Z',
            content: '',
        }));

        // Use minimal HTML to avoid heavy DOM rendering — test only checks
        // done state and card count, not rendering fidelity.
        buildArticleCardHtml.mockImplementation(
            (a) => `<div class="article-card" data-id="${a.id}"></div>`,
        );

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({ articles }),
            text: async () => '',
        }));

        await loadMoreArticles();

        expect(getPaginationState().done).toBe(false);
        expect(document.querySelectorAll('.article-card').length).toBe(PAGE_SIZE);
    });

    it('includes include_read param when showing hidden articles', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/feed/8' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        getShowingHiddenArticles.mockReturnValue(true);

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({ articles: [] }),
            text: async () => '',
        }));

        await loadMoreArticles();

        const url = globalThis.fetch.mock.calls[0][0];
        expect(url).toContain('include_read=1');
    });

    it('does not include include_read param by default', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/feed/8' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({ articles: [] }),
            text: async () => '',
        }));

        await loadMoreArticles();

        const url = globalThis.fetch.mock.calls[0][0];
        expect(url).not.toContain('include_read');
    });

    it('shows toast and logs error on API failure', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        vi.spyOn(console, 'error').mockImplementation(() => {});
        // Ensure checkScrollForMore in finally block doesn't re-trigger load
        Object.defineProperty(document.body, 'offsetHeight', { value: 100000, configurable: true });

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: false,
            text: async () => JSON.stringify({ error: 'Server error' }),
        }));

        await loadMoreArticles();

        expect(console.error).toHaveBeenCalledWith('Failed to load more articles:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to load more articles');
        // loading should still be reset in finally block
        expect(getPaginationState().loading).toBe(false);
    });

    it('handles missing articles-list element gracefully', async () => {
        document.body.innerHTML = ''; // remove articles-list
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        setQueuedIdsReady(Promise.resolve());

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({
                articles: [{
                    id: 1, title: 'A', is_read: 0, is_starred: 0,
                    published_at: new Date().toISOString(), content: '<p>hi</p>',
                }],
            }),
            text: async () => '',
        }));

        await loadMoreArticles();

        // Should not throw, loading should still be reset
        expect(getPaginationState().loading).toBe(false);
    });

    it('updates cursor from loaded articles', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        setQueuedIdsReady(Promise.resolve());

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({
                articles: [{
                    id: 42, title: 'B', is_read: 0, is_starred: 0,
                    published_at: '2025-06-15T12:00:00Z', content: '<p>text</p>',
                }],
            }),
            text: async () => '',
        }));

        await loadMoreArticles();

        expect(getPaginationState().cursorTime).toBe('2025-06-15T12:00:00Z');
        expect(getPaginationState().cursorId).toBe(42);
    });
});

describe('checkScrollForMore', () => {
    it('does nothing when pagination is done', () => {
        setPaginationState({ done: true });
        checkScrollForMore();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('does nothing when loading', () => {
        setPaginationState({ loading: true });
        checkScrollForMore();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('does nothing when far from bottom', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        // Far from bottom: scrollY + innerHeight is much less than offsetHeight - 600
        Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true });
        Object.defineProperty(window, 'scrollY', { value: 0, configurable: true, writable: true });
        Object.defineProperty(document.body, 'offsetHeight', { value: 5000, configurable: true });

        checkScrollForMore();

        expect(globalThis.fetch).not.toHaveBeenCalled();
    });

    it('calls loadMoreArticles when near the bottom of the page', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/feed/8' }, writable: true, configurable: true,
        });

        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });
        setQueuedIdsReady(Promise.resolve());

        // Simulate being near the bottom: scrollY + innerHeight >= offsetHeight - 600
        Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true });
        Object.defineProperty(window, 'scrollY', { value: 600, configurable: true, writable: true });
        Object.defineProperty(document.body, 'offsetHeight', { value: 1000, configurable: true });

        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({ articles: [] }),
            text: async () => '',
        }));

        checkScrollForMore();
        // loadMoreArticles is async — flush microtasks via fake timers
        await vi.advanceTimersByTimeAsync(0);

        expect(globalThis.fetch).toHaveBeenCalled();
    });
});

describe('initPagination', () => {
    let scrollHandler;

    beforeEach(() => {
        _resetPaginationState();
        // Capture the scroll listener added by initPagination
        scrollHandler = null;
        const origAddEventListener = window.addEventListener;
        vi.spyOn(window, 'addEventListener').mockImplementation((type, handler) => {
            if (type === 'scroll') scrollHandler = handler;
            origAddEventListener.call(window, type, handler);
        });
    });

    it('sets pagination state from last article card', () => {
        document.getElementById('articles-list').innerHTML = `
            <div class="article-card" data-sort-time="2025-01-01T00:00:00Z" data-id="10"></div>
            <div class="article-card" data-sort-time="2025-01-02T00:00:00Z" data-id="20"></div>
        `;
        initPagination();
        const state = getPaginationState();
        expect(state.cursorTime).toBe('2025-01-02T00:00:00Z');
        expect(state.cursorId).toBe('20');
        expect(state.done).toBe(true); // 2 < PAGE_SIZE
    });

    it('marks not done when article count equals PAGE_SIZE', () => {
        let html = '';
        for (let i = 0; i < PAGE_SIZE; i++) {
            html += `<div class="article-card" data-sort-time="2025-01-01T00:00:00Z" data-id="${i}"></div>`;
        }
        document.getElementById('articles-list').innerHTML = html;
        initPagination();
        expect(getPaginationState().done).toBe(false);
    });

    it('sets done=true when no articles exist', () => {
        initPagination();
        expect(getPaginationState().done).toBe(true);
    });

    it('registers scroll listener', () => {
        initPagination();
        expect(scrollHandler).toBeInstanceOf(Function);
    });

    it('updates end-of-articles indicator', () => {
        // With no articles and done=true, indicator should NOT be visible
        initPagination();
        expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(false);

        // Reset and add articles
        _resetPaginationState();
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-sort-time="2025-01-01" data-id="1"></div>';
        initPagination();
        // done=true (1 < PAGE_SIZE) and has articles, so indicator should be visible
        expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(true);
    });

    it('uses null for missing data attributes', () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card"></div>';
        initPagination();
        const state = getPaginationState();
        expect(state.cursorTime).toBeNull();
        expect(state.cursorId).toBeNull();
    });
});
