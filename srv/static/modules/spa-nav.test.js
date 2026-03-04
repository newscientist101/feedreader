import { describe, it, expect, beforeEach, vi } from 'vitest';
import { initSpaNav } from './spa-nav.js';

vi.mock('./api.js', () => ({
    api: vi.fn(() => Promise.resolve({ articles: [] })),
}));

vi.mock('./articles.js', () => ({
    showArticlesLoading: vi.fn(),
    renderArticles: vi.fn(),
}));

vi.mock('./sidebar.js', () => ({
    setSidebarActive: vi.fn(),
}));

vi.mock('./views.js', () => ({
    applyDefaultViewForScope: vi.fn(),
}));

vi.mock('./feed-errors.js');

vi.mock('./counts.js');

describe('spa-nav', () => {
    beforeEach(() => {
        document.body.innerHTML = '';
        vi.clearAllMocks();
    });

    describe('initSpaNav', () => {
        it('sets up popstate listener without error', () => {
            initSpaNav();
        });

        it('replaces history state on article-list pages', () => {
            document.body.innerHTML = '<div class="articles-view"></div>';
            const replaceSpy = vi.spyOn(history, 'replaceState');

            // Set pathname to / for the test
            Object.defineProperty(window, 'location', {
                value: { ...window.location, pathname: '/' },
                writable: true,
            });

            initSpaNav();
            expect(replaceSpy).toHaveBeenCalled();
            replaceSpy.mockRestore();
        });

        it('intercepts nav-item clicks on article-list pages', async () => {
            const { api } = await import('./api.js');
            const { renderArticles } = await import('./articles.js');

            document.body.innerHTML = `
                <div class="articles-view" data-view-scope="all">
                    <header class="view-header"><h1>All Unread</h1></header>
                    <div id="articles-list"></div>
                </div>
                <a href="/starred" class="nav-item">Starred</a>
            `;

            Object.defineProperty(window, 'location', {
                value: { ...window.location, pathname: '/' },
                writable: true,
            });

            initSpaNav();

            // Click the starred nav item
            const link = document.querySelector('.nav-item[href="/starred"]');
            link.click();

            // Wait for async API call
            await vi.waitFor(() => {
                expect(api).toHaveBeenCalledWith('GET', '/api/articles/starred');
            });

            expect(renderArticles).toHaveBeenCalledWith([]);
        });

        it('does not intercept nav-item clicks on non-article pages', async () => {
            const { api } = await import('./api.js');

            document.body.innerHTML = `
                <div class="settings-page"></div>
                <a href="/starred" class="nav-item">Starred</a>
            `;

            initSpaNav();

            const link = document.querySelector('.nav-item[href="/starred"]');
            // Prevent actual navigation in test
            link.addEventListener('click', (e) => e.preventDefault());
            link.click();

            expect(api).not.toHaveBeenCalled();
        });
    });
});
