import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { initSpaNav } from './spa-nav.js';

vi.mock('./api.js');
vi.mock('./articles.js');
vi.mock('./sidebar.js');
vi.mock('./views.js');
vi.mock('./feed-errors.js');
vi.mock('./counts.js');
vi.mock('./nav-state.js');
vi.mock('./settings.js');

import { api } from './api.js';
import { showArticlesLoading, renderArticles } from './articles.js';
import { setSidebarActive } from './sidebar.js';
import { applyDefaultViewForScope } from './views.js';
import { removeFeedErrorBanner } from './feed-errors.js';
import { updateCounts } from './counts.js';
import { markReturningFromArticleList, consumeReturningFromArticleList, peekPendingReadIds, clearPendingReadIds } from './nav-state.js';
import { getSetting } from './settings.js';

/** Set up the standard article-list page DOM for SPA navigation tests. */
function setupArticleListPage() {
    document.body.innerHTML = `
        <div class="articles-view" data-view-scope="all">
            <header class="view-header"><h1>All Unread</h1></header>
            <div id="articles-list"></div>
            <div class="dropdown" data-feed-id="5" data-category-id="3"></div>
            <button data-feed-action="refresh" style="display: block">Refresh</button>
        </div>
        <a href="/" class="nav-item">All Unread</a>
        <a href="/starred" class="nav-item">Starred</a>
        <a href="/feed/1" class="nav-item">Feed 1</a>
        <a href="/category/2" class="nav-item">Category 2</a>
    `;
}

beforeEach(() => {
    document.body.innerHTML = '';
    vi.clearAllMocks();
    api.mockResolvedValue({ articles: [{ id: 1, title: 'Test' }] });
    markReturningFromArticleList.mockReturnValue(true);
    consumeReturningFromArticleList.mockReturnValue(false);
    peekPendingReadIds.mockReturnValue(new Set());
    clearPendingReadIds.mockReturnValue(true);
    getSetting.mockReturnValue(null);
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true });
    Object.defineProperty(window, 'location', {
        value: { ...window.location, pathname: '/', reload: vi.fn() },
        writable: true,
        configurable: true,
    });
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('initSpaNav', () => {
    it('sets up listeners without error on empty page', () => {
        initSpaNav();
    });

    it('replaces history state on article-list pages', () => {
        setupArticleListPage();
        const replaceSpy = vi.spyOn(history, 'replaceState');

        initSpaNav();

        expect(replaceSpy).toHaveBeenCalledWith(
            { spaNav: true, path: '/', view: 'unread' },
            expect.any(String),
            '/',
        );
        replaceSpy.mockRestore();
    });

    it('does not replace history state on non-article pages', () => {
        document.body.innerHTML = '<div class="settings-page"></div>';
        const replaceSpy = vi.spyOn(history, 'replaceState');

        initSpaNav();

        expect(replaceSpy).not.toHaveBeenCalled();
        replaceSpy.mockRestore();
    });

    it('replaces history state for /starred route', () => {
        setupArticleListPage();
        window.location.pathname = '/starred';
        const replaceSpy = vi.spyOn(history, 'replaceState');

        initSpaNav();

        expect(replaceSpy).toHaveBeenCalledWith(
            expect.objectContaining({ view: 'starred', path: '/starred' }),
            expect.any(String),
            '/starred',
        );
        replaceSpy.mockRestore();
    });
});

describe('nav-item click interception', () => {
    it('SPA-navigates to /starred and calls all orchestration steps', async () => {
        setupArticleListPage();
        const pushSpy = vi.spyOn(history, 'pushState');

        initSpaNav();
        document.querySelector('.nav-item[href="/starred"]').click();

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/starred');
        }, { interval: 1 });

        // Loading state shown before API call
        expect(showArticlesLoading).toHaveBeenCalled();

        // Articles rendered with API response
        expect(renderArticles).toHaveBeenCalledWith([{ id: 1, title: 'Test' }]);

        // Sidebar updated
        const starredLink = document.querySelector('.nav-item[href="/starred"]');
        expect(setSidebarActive).toHaveBeenCalledWith(starredLink);

        // View scope and title updated
        expect(applyDefaultViewForScope).toHaveBeenCalledWith('all');
        expect(document.querySelector('.view-header h1').textContent).toBe('Starred');
        expect(document.title).toContain('Starred');

        // Feed error banner removed
        expect(removeFeedErrorBanner).toHaveBeenCalled();

        // Counts refreshed
        expect(updateCounts).toHaveBeenCalled();

        // History updated
        expect(pushSpy).toHaveBeenCalledWith(
            { spaNav: true, path: '/starred', view: 'starred' },
            'Starred',
            '/starred',
        );

        // Dropdown feed/category IDs cleared
        const dropdown = document.querySelector('.dropdown');
        expect(dropdown.dataset.feedId).toBe('');
        expect(dropdown.dataset.categoryId).toBe('');

        // Feed-action buttons hidden
        expect(document.querySelector('[data-feed-action]').style.display).toBe('none');

        pushSpy.mockRestore();
    });

    it('SPA-navigates to / (home) route', async () => {
        setupArticleListPage();
        window.location.pathname = '/starred';

        initSpaNav();
        document.querySelector('.nav-item[href="/"]').click();

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/unread');
        }, { interval: 1 });

        expect(document.querySelector('.view-header h1').textContent).toBe('All Unread');
        expect(renderArticles).toHaveBeenCalled();
    });

    it('does not intercept nav-item clicks on non-article pages', () => {
        document.body.innerHTML = `
            <div class="settings-page"></div>
            <a href="/starred" class="nav-item">Starred</a>
        `;

        initSpaNav();
        const link = document.querySelector('.nav-item[href="/starred"]');
        link.addEventListener('click', (e) => e.preventDefault());
        link.click();

        expect(api).not.toHaveBeenCalled();
        expect(showArticlesLoading).not.toHaveBeenCalled();
    });

    it('does not intercept feed route clicks (handled elsewhere)', () => {
        setupArticleListPage();

        initSpaNav();
        const link = document.querySelector('.nav-item[href="/feed/1"]');
        link.addEventListener('click', (e) => e.preventDefault());
        link.click();

        expect(api).not.toHaveBeenCalled();
    });

    it('does not intercept category route clicks (handled elsewhere)', () => {
        setupArticleListPage();

        initSpaNav();
        const link = document.querySelector('.nav-item[href="/category/2"]');
        link.addEventListener('click', (e) => e.preventDefault());
        link.click();

        expect(api).not.toHaveBeenCalled();
    });

    it('ignores clicks on non-nav-item elements', () => {
        setupArticleListPage();
        document.body.innerHTML += '<button class="other">Click</button>';

        initSpaNav();
        document.querySelector('.other').click();

        expect(api).not.toHaveBeenCalled();
    });

    it('handles API error gracefully during navigation', async () => {
        setupArticleListPage();
        api.mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        initSpaNav();
        document.querySelector('.nav-item[href="/starred"]').click();

        await vi.waitFor(() => {
            expect(console.error).toHaveBeenCalledWith(
                'SPA navigation failed:',
                expect.any(Error),
            );
        }, { interval: 1 });

        // renderArticles should not be called on error
        expect(renderArticles).not.toHaveBeenCalled();
        // But showArticlesLoading was called before the API call
        expect(showArticlesLoading).toHaveBeenCalled();
    });
});

describe('popstate (browser back/forward)', () => {
    it('restores unread/starred route on popstate', async () => {
        setupArticleListPage();

        initSpaNav();
        window.location.pathname = '/starred';
        window.dispatchEvent(new PopStateEvent('popstate', {
            state: { spaNav: true, path: '/starred', view: 'starred' },
        }));

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/starred');
        }, { interval: 1 });

        expect(showArticlesLoading).toHaveBeenCalled();
        expect(renderArticles).toHaveBeenCalledWith([{ id: 1, title: 'Test' }]);
        expect(setSidebarActive).toHaveBeenCalled();
        expect(removeFeedErrorBanner).toHaveBeenCalled();
        expect(applyDefaultViewForScope).toHaveBeenCalledWith('all');
        expect(updateCounts).toHaveBeenCalled();
        expect(document.querySelector('.view-header h1').textContent).toBe('Starred');
    });

    it('reloads when current page is not an article-list page', () => {
        document.body.innerHTML = '<div class="settings-page"></div>';

        initSpaNav();
        window.location.pathname = '/starred';
        window.dispatchEvent(new PopStateEvent('popstate', { state: null }));

        expect(window.location.reload).toHaveBeenCalled();
    });

    it('reloads when popstate path does not match any route', () => {
        setupArticleListPage();

        initSpaNav();
        window.location.pathname = '/settings';
        window.dispatchEvent(new PopStateEvent('popstate', { state: null }));

        expect(window.location.reload).toHaveBeenCalled();
    });

    it('handles API error during popstate restore', async () => {
        setupArticleListPage();
        api.mockRejectedValue(new Error('Server error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        initSpaNav();
        window.location.pathname = '/';
        window.dispatchEvent(new PopStateEvent('popstate', {
            state: { spaNav: true, path: '/', view: 'unread' },
        }));

        await vi.waitFor(() => {
            expect(console.error).toHaveBeenCalledWith(
                'SPA popstate navigation failed:',
                expect.any(Error),
            );
        }, { interval: 1 });

        expect(renderArticles).not.toHaveBeenCalled();
    });

    it('clears dropdown and hides feed buttons on popstate restore', async () => {
        setupArticleListPage();

        initSpaNav();
        window.location.pathname = '/';
        window.dispatchEvent(new PopStateEvent('popstate', {
            state: { spaNav: true, path: '/', view: 'unread' },
        }));

        await vi.waitFor(() => {
            expect(renderArticles).toHaveBeenCalled();
        }, { interval: 1 });

        const dropdown = document.querySelector('.dropdown');
        expect(dropdown.dataset.feedId).toBe('');
        expect(dropdown.dataset.categoryId).toBe('');
        expect(document.querySelector('[data-feed-action]').style.display).toBe('none');
    });

    it('scrolls to top and refreshes counts when restored from back/forward cache', () => {
        setupArticleListPage();
        initSpaNav();

        window.scrollTo = vi.fn();
        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: true });
        window.dispatchEvent(event);

        expect(window.scrollTo).toHaveBeenCalledWith(0, 0);
        expect(updateCounts).toHaveBeenCalled();
    });

    it('refreshes counts but does not scroll on non-bfcache pageshow', () => {
        setupArticleListPage();
        initSpaNav();

        window.scrollTo = vi.fn();
        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        expect(window.scrollTo).not.toHaveBeenCalled();
        expect(updateCounts).toHaveBeenCalled();
    });

    it('re-fetches the current article list after returning from an article page', async () => {
        setupArticleListPage();
        consumeReturningFromArticleList.mockReturnValue(true);
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/unread');
        }, { interval: 1 });

        expect(renderArticles).toHaveBeenCalledWith([{ id: 1, title: 'Test' }]);
        expect(updateCounts).toHaveBeenCalled();
    });

    it('does not consume article-return state on non-list pages', () => {
        document.body.innerHTML = '<div class="article-view"></div>';
        consumeReturningFromArticleList.mockReturnValue(true);
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        expect(api).not.toHaveBeenCalled();
        expect(updateCounts).toHaveBeenCalled();
        expect(consumeReturningFromArticleList).not.toHaveBeenCalled();
    });

    it('replays pending read IDs on Back: marks cards read and POSTs to batch-read', async () => {
        setupArticleListPage();
        document.getElementById('articles-list').innerHTML = `
            <div class="article-card" data-id="10"></div>
            <div class="article-card" data-id="11"></div>
        `;
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set([10, 11]));
        globalThis.fetch.mockResolvedValue({ ok: true });
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(globalThis.fetch).toHaveBeenCalledWith(
                '/api/articles/batch-read',
                expect.objectContaining({
                    method: 'POST',
                    body: expect.stringContaining('"ids"'),
                }),
            );
        }, { interval: 1 });

        // Cards must be marked read in the DOM
        expect(document.querySelector('.article-card[data-id="10"]').classList.contains('read')).toBe(true);
        expect(document.querySelector('.article-card[data-id="11"]').classList.contains('read')).toBe(true);

        // Pending IDs cleared after successful POST
        expect(clearPendingReadIds).toHaveBeenCalled();
    });

    it('hides pending read cards when hideReadArticles=hide', async () => {
        setupArticleListPage();
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="20"></div>';
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set([20]));
        getSetting.mockReturnValue('hide');
        globalThis.fetch.mockResolvedValue({ ok: true });
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(clearPendingReadIds).toHaveBeenCalled();
        }, { interval: 1 });

        const card = document.querySelector('.article-card[data-id="20"]');
        expect(card.style.display).toBe('none');
    });

    it('does not clear pending IDs when replay POST fails', async () => {
        setupArticleListPage();
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set([30]));
        globalThis.fetch.mockResolvedValue({ ok: false, status: 500 });
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/unread');
        }, { interval: 1 });

        expect(clearPendingReadIds).not.toHaveBeenCalled();
    });

    it('re-sets the return marker when replay POST returns non-2xx so a later pageshow retries', async () => {
        setupArticleListPage();
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set([30]));
        globalThis.fetch.mockResolvedValue({ ok: false, status: 500 });
        vi.spyOn(console, 'error').mockImplementation(() => {});
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(markReturningFromArticleList).toHaveBeenCalled();
        }, { interval: 1 });

        expect(clearPendingReadIds).not.toHaveBeenCalled();
        expect(console.error).toHaveBeenCalledWith(
            'Failed to replay pending read IDs: HTTP',
            500,
        );
    });

    it('does not call batch-read when no pending IDs exist', async () => {
        setupArticleListPage();
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set());
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/unread');
        }, { interval: 1 });

        expect(globalThis.fetch).not.toHaveBeenCalled();
        expect(clearPendingReadIds).not.toHaveBeenCalled();
    });

    it('leaves pending IDs intact when replay POST throws a network error', async () => {
        setupArticleListPage();
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set([40]));
        globalThis.fetch.mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/articles/unread');
        }, { interval: 1 });

        expect(clearPendingReadIds).not.toHaveBeenCalled();
        expect(console.error).toHaveBeenCalledWith(
            'Failed to replay pending read IDs:',
            expect.any(Error),
        );
    });

    it('re-sets the return marker when replay POST throws a network error so a later pageshow retries', async () => {
        setupArticleListPage();
        consumeReturningFromArticleList.mockReturnValue(true);
        peekPendingReadIds.mockReturnValue(new Set([40]));
        globalThis.fetch.mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});
        initSpaNav();

        const event = new Event('pageshow');
        Object.defineProperty(event, 'persisted', { value: false });
        window.dispatchEvent(event);

        await vi.waitFor(() => {
            expect(markReturningFromArticleList).toHaveBeenCalled();
        }, { interval: 1 });

        expect(clearPendingReadIds).not.toHaveBeenCalled();
    });
});
