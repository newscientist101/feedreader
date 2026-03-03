import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    updateEndOfArticlesIndicator, updatePaginationCursor, getPaginationUrl,
    loadMoreArticles, checkScrollForMore, initPagination,
    setPaginationState, getPaginationState, PAGE_SIZE,
    _resetPaginationState,
} from './pagination.js';
import { _resetArticleActionsState, setQueuedArticleIds, setQueuedIdsReady } from './article-actions.js';

beforeEach(() => {
    vi.useFakeTimers();
    _resetPaginationState();
    _resetArticleActionsState();
    setQueuedArticleIds(new Set());
    setQueuedIdsReady(Promise.resolve());
    window.__settings = {};
    window.fetch = vi.fn(() => Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ articles: [] }),
    }));
    document.body.innerHTML = `
        <div class="articles-view">
            <div id="articles-list" class="articles-list"></div>
            <div class="end-of-articles" id="end-of-articles"></div>
        </div>
    `;
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
});

describe('loadMoreArticles', () => {
    it('does nothing when pagination is done', async () => {
        setPaginationState({ done: true });
        await loadMoreArticles();
        expect(window.fetch).not.toHaveBeenCalled();
    });

    it('does nothing when loading', async () => {
        setPaginationState({ loading: true });
        await loadMoreArticles();
        expect(window.fetch).not.toHaveBeenCalled();
    });

    it('does nothing without cursor', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        await loadMoreArticles();
        expect(window.fetch).not.toHaveBeenCalled();
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

        window.fetch = vi.fn(async () => ({
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
    });
});

describe('checkScrollForMore', () => {
    it('does nothing when pagination is done', () => {
        setPaginationState({ done: true });
        checkScrollForMore();
        expect(window.fetch).not.toHaveBeenCalled();
    });

    it('calls loadMoreArticles when near the bottom of the page', async () => {
        vi.useRealTimers();  // loadMoreArticles uses await, real timers work better
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

        window.fetch = vi.fn(async () => ({
            ok: true,
            json: async () => ({ articles: [] }),
            text: async () => '',
        }));

        checkScrollForMore();
        // loadMoreArticles is async — give it time to resolve
        await new Promise(r => setTimeout(r, 50));

        expect(window.fetch).toHaveBeenCalled();
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
});
