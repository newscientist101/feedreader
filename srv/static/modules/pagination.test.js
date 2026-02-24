import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    updateEndOfArticlesIndicator, updatePaginationCursor, getPaginationUrl,
    loadMoreArticles, checkScrollForMore,
    setPaginationState, getPaginationState, PAGE_SIZE,
    _resetPaginationState,
} from './pagination.js';
import { _resetArticleActionsState, setQueuedArticleIds, setQueuedIdsReady } from './article-actions.js';

beforeEach(() => {
    vi.spyOn(console, 'debug').mockImplementation(() => {});
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
});

describe('checkScrollForMore', () => {
    it('does nothing when pagination is done', () => {
        setPaginationState({ done: true });
        checkScrollForMore();
        expect(window.fetch).not.toHaveBeenCalled();
    });
});
