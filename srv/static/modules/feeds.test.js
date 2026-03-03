import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    loadCategoryArticles, loadFeedArticles,
    filterFeeds, closeEditModal, saveFeed,
    deleteFeed, setFeedCategory, initFeedActionListeners,
    initAddFeedForm, initFeedItemClickListeners,
    refreshFeed, editFeed, createEditFeedModal,
} from './feeds.js';
import { _resetArticlesState } from './articles.js';
import { _resetArticleActionsState, setQueuedArticleIds } from './article-actions.js';
import { showToast } from './toast.js';

vi.mock('./toast.js', () => ({
    showToast: vi.fn(),
}));

// Mock pagination (articles.js directly imports from pagination.js)
vi.mock('./pagination.js', () => ({
    updatePaginationCursor: vi.fn(),
    updateEndOfArticlesIndicator: vi.fn(),
    setPaginationState: vi.fn(),
}));

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    _resetArticlesState();
    _resetArticleActionsState();
    setQueuedArticleIds(new Set());
    window.__settings = { autoMarkRead: 'true' };
    // Ensure dialog functions exist for happy-dom compatibility
    window.confirm ??= () => false;
    window.prompt ??= () => null;
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('loadCategoryArticles', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div class="articles-view">
                <div class="view-header"><h1>All Articles</h1></div>
                <div class="dropdown" data-feed-id="5" data-category-id=""></div>
                <button data-feed-action="edit" style="display: block">Edit</button>
                <div id="articles-list" class="articles-list"></div>
            </div>
            <div class="sidebar">
                <div class="folder-item" data-category-id="3">Tech</div>
            </div>
        `;
        vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
    });

    it('loads and renders category articles', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ articles: [{ id: 1, title: 'Test', feed_name: 'F' }] }),
        });

        await loadCategoryArticles(3, 'Tech');

        expect(document.querySelector('.view-header h1').textContent).toBe('Tech');
        expect(document.title).toBe('Tech - FeedReader');
        expect(history.pushState).toHaveBeenCalledWith({ spaNav: true, categoryId: 3 }, 'Tech', '/category/3');
        const dropdown = document.querySelector('.dropdown');
        expect(dropdown.dataset.feedId).toBe('');
        expect(dropdown.dataset.categoryId).toBe('3');
    });

    it('hides feed action buttons', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ articles: [] }),
        });

        await loadCategoryArticles(3, 'Tech');

        const editBtn = document.querySelector('[data-feed-action="edit"]');
        expect(editBtn.style.display).toBe('none');
    });

    it('handles API errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await loadCategoryArticles(3, 'Tech');

        expect(console.error).toHaveBeenCalledWith('Failed to load category articles:', expect.any(Error));
    });
});

describe('loadFeedArticles', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div class="articles-view">
                <div class="view-header"><h1>All Articles</h1></div>
                <div class="header-actions"></div>
                <div class="dropdown" data-feed-id="" data-category-id="5"></div>
                <div id="articles-list" class="articles-list"></div>
            </div>
            <div class="sidebar">
                <div class="feed-item" data-feed-id="7">Feed</div>
            </div>
        `;
        vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
    });

    it('loads and renders feed articles', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                articles: [{ id: 1, title: 'Test', feed_name: 'F' }],
                feed: { id: 7, name: 'Feed', last_error: null },
            }),
        });

        await loadFeedArticles(7, 'Feed');

        expect(document.querySelector('.view-header h1').textContent).toBe('Feed');
        expect(document.title).toBe('Feed - FeedReader');
        expect(history.pushState).toHaveBeenCalledWith({ spaNav: true, feedId: 7 }, 'Feed', '/feed/7');
        const dropdown = document.querySelector('.dropdown');
        expect(dropdown.dataset.feedId).toBe('7');
        expect(dropdown.dataset.categoryId).toBe('');
    });

    it('sets active state on matching feed items', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                articles: [],
                feed: { id: 7, name: 'Feed', last_error: null },
            }),
        });

        await loadFeedArticles(7, 'Feed');

        expect(document.querySelector('.feed-item[data-feed-id="7"]').classList.contains('active')).toBe(true);
    });

    it('shows error banner when feed has last_error', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                articles: [],
                feed: { id: 7, name: 'Feed', last_error: 'Timeout' },
            }),
        });

        await loadFeedArticles(7, 'Feed');

        const banner = document.querySelector('.feed-error-banner');
        expect(banner).not.toBeNull();
        expect(banner.innerHTML).toContain('Timeout');
    });

    it('creates edit and refresh buttons in header-actions', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                articles: [],
                feed: { id: 7, name: 'Feed', last_error: null },
            }),
        });

        await loadFeedArticles(7, 'Feed');

        const editBtn = document.querySelector('[data-feed-action="edit"]');
        expect(editBtn).not.toBeNull();
        const refreshBtn = document.querySelector('[data-feed-action="refresh"]');
        expect(refreshBtn).not.toBeNull();
        expect(refreshBtn.dataset.feedId).toBe('7');
    });

    it('handles API errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await loadFeedArticles(7, 'Feed');

        expect(console.error).toHaveBeenCalledWith('Failed to load feed articles:', expect.any(Error));
    });
});

describe('filterFeeds', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <input id="feeds-search" value="">
            <input id="filter-errors" type="checkbox">
            <table class="feeds-table"><tbody>
                <tr><td>Tech Blog</td><td class="url-cell">https://tech.com/rss</td></tr>
                <tr data-has-error="true"><td>Broken Feed</td><td class="url-cell">https://broken.com</td></tr>
                <tr><td>News Site</td><td class="url-cell">https://news.com/feed</td></tr>
            </tbody></table>
        `;
    });

    it('shows all rows when search is empty', () => {
        filterFeeds();

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        rows.forEach(row => expect(row.style.display).toBe(''));
    });

    it('filters rows by name', () => {
        document.getElementById('feeds-search').value = 'tech';

        filterFeeds();

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        expect(rows[0].style.display).toBe('');
        expect(rows[1].style.display).toBe('none');
        expect(rows[2].style.display).toBe('none');
    });

    it('filters rows by URL', () => {
        document.getElementById('feeds-search').value = 'news.com';

        filterFeeds();

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        expect(rows[0].style.display).toBe('none');
        expect(rows[1].style.display).toBe('none');
        expect(rows[2].style.display).toBe('');
    });

    it('shows only error rows when error filter checked', () => {
        document.getElementById('filter-errors').checked = true;

        filterFeeds();

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        expect(rows[0].style.display).toBe('none');
        expect(rows[1].style.display).toBe(''); // has error
        expect(rows[2].style.display).toBe('none');
    });

    it('combines search and error filter', () => {
        document.getElementById('feeds-search').value = 'broken';
        document.getElementById('filter-errors').checked = true;

        filterFeeds();

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        expect(rows[0].style.display).toBe('none');
        expect(rows[1].style.display).toBe(''); // matches both
        expect(rows[2].style.display).toBe('none');
    });
});

describe('closeEditModal', () => {
    it('hides the edit modal', () => {
        document.body.innerHTML = '<div id="edit-feed-modal" class="modal" style="display: flex"></div>';

        closeEditModal();

        expect(document.getElementById('edit-feed-modal').style.display).toBe('none');
    });

    it('does nothing when no modal exists', () => {
        document.body.innerHTML = '<div>Content</div>';
        closeEditModal(); // should not throw
    });
});

describe('saveFeed', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div id="edit-feed-modal" class="modal" style="display: flex"></div>
            <input id="edit-feed-id" value="5">
            <input id="edit-feed-name" value="Updated Feed">
            <input id="edit-feed-url" value="https://example.com/rss">
            <input id="edit-feed-interval" value="30">
            <textarea id="edit-feed-filters"></textarea>
            <table><tbody>
                <tr data-feed-id="5">
                    <td><a>Old Name</a></td>
                    <td><a href="https://old.com" class="url-cell">https://old.com</a></td>
                </tr>
            </tbody></table>
        `;
    });

    it('saves feed and updates row in place', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        expect(event.preventDefault).toHaveBeenCalled();
        expect(fetch).toHaveBeenCalledWith('/api/feeds/5', expect.objectContaining({
            method: 'PUT',
            body: expect.stringContaining('"Updated Feed"'),
        }));
        // Modal should be hidden
        expect(document.getElementById('edit-feed-modal').style.display).toBe('none');
        // Row should be updated
        const row = document.querySelector('tr[data-feed-id="5"]');
        expect(row.querySelector('td a').textContent).toBe('Updated Feed');
    });

    it('sends content filters as JSON', async () => {
        document.getElementById('edit-feed-filters').value = '.ad\n.sidebar';
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        const body = JSON.parse(fetch.mock.calls[0][1].body);
        const filters = JSON.parse(body.content_filters);
        expect(filters).toEqual([{ selector: '.ad' }, { selector: '.sidebar' }]);
    });

    it('handles save errors', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Not found' })),
        });
        vi.spyOn(console, 'error').mockImplementation(() => {});
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        expect(console.error).toHaveBeenCalledWith('Failed to save feed:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to save feed');
    });
});

describe('deleteFeed', () => {
    it('calls API and reloads on confirm', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        // Mock location.reload
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await deleteFeed(5, 'Test Feed');

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Test Feed'));
        expect(fetch).toHaveBeenCalledWith('/api/feeds/5', expect.objectContaining({ method: 'DELETE' }));
    });

    it('does nothing when user cancels', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(false);
        vi.spyOn(globalThis, 'fetch');

        await deleteFeed(5, 'Test Feed');

        expect(fetch).not.toHaveBeenCalled();
    });
});

describe('setFeedCategory', () => {
    it('calls the category API', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });

        await setFeedCategory(5, 3);

        expect(fetch).toHaveBeenCalledWith('/api/feeds/5/category', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ categoryId: 3 }),
        }));
    });

    it('handles errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'fail' })),
        });
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await setFeedCategory(5, 3);

        expect(console.error).toHaveBeenCalledWith('Failed to set category:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to move feed');
    });
});

describe('createEditFeedModal', () => {
    it('creates modal DOM with expected structure', () => {
        const modal = createEditFeedModal();

        expect(modal.id).toBe('edit-feed-modal');
        expect(modal.className).toBe('modal');
        expect(modal.style.display).toBe('none');
        expect(document.getElementById('edit-feed-id')).not.toBeNull();
        expect(document.getElementById('edit-feed-name')).not.toBeNull();
        expect(document.getElementById('edit-feed-url')).not.toBeNull();
        expect(document.getElementById('edit-feed-interval')).not.toBeNull();
        expect(document.getElementById('edit-feed-filters')).not.toBeNull();
        expect(modal.querySelector('[data-action="close-edit-modal"]')).not.toBeNull();
        expect(modal.querySelector('form#edit-feed-form')).not.toBeNull();
    });

    it('is idempotent — returns same element on second call', () => {
        const first = createEditFeedModal();
        const second = createEditFeedModal();

        expect(first).toBe(second);
        expect(document.querySelectorAll('#edit-feed-modal').length).toBe(1);
    });
});

describe('editFeed', () => {
    it('populates all form fields from API response', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                id: 42,
                name: 'Test Feed',
                url: 'https://example.com/rss',
                fetch_interval_minutes: 120,
                content_filters: null,
            }),
        });

        await editFeed(42);

        expect(document.getElementById('edit-feed-id').value).toBe('42');
        expect(document.getElementById('edit-feed-name').value).toBe('Test Feed');
        expect(document.getElementById('edit-feed-url').value).toBe('https://example.com/rss');
        expect(document.getElementById('edit-feed-interval').value).toBe('120');
        expect(document.getElementById('edit-feed-filters').value).toBe('');
        // Modal should be visible
        expect(document.getElementById('edit-feed-modal').style.display).toBe('flex');
    });

    it('parses content_filters JSON into newline-separated textarea', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                id: 5,
                name: 'Feed',
                url: 'https://example.com',
                fetch_interval_minutes: 60,
                content_filters: JSON.stringify([{ selector: '.ad' }, { selector: '#sidebar' }]),
            }),
        });

        await editFeed(5);

        expect(document.getElementById('edit-feed-filters').value).toBe('.ad\n#sidebar');
    });

    it('handles empty content_filters', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                id: 5,
                name: 'Feed',
                url: 'https://example.com',
                fetch_interval_minutes: 60,
                content_filters: '',
            }),
        });

        await editFeed(5);

        expect(document.getElementById('edit-feed-filters').value).toBe('');
    });

    it('handles invalid JSON in content_filters gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                id: 5,
                name: 'Feed',
                url: 'https://example.com',
                fetch_interval_minutes: 60,
                content_filters: 'not-valid-json',
            }),
        });

        await editFeed(5);

        // Should fall back to empty string on parse error
        expect(document.getElementById('edit-feed-filters').value).toBe('');
    });

    it('uses default interval of 60 when fetch_interval_minutes is null', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                id: 5,
                name: 'Feed',
                url: 'https://example.com',
                fetch_interval_minutes: null,
                content_filters: null,
            }),
        });

        await editFeed(5);

        expect(document.getElementById('edit-feed-interval').value).toBe('60');
    });

    it('shows toast on API failure', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Not found' })),
        });
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await editFeed(99);

        expect(console.error).toHaveBeenCalledWith('Failed to load feed:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to load feed details');
    });
});

describe('refreshFeed', () => {
    it('polls until fetch completes and updates the status cell', async () => {
        vi.useFakeTimers();

        let statusCall = 0;
        vi.spyOn(globalThis, 'fetch').mockImplementation(async (url) => {
            if (url === '/api/feeds/9/status') {
                statusCall += 1;
                return {
                    ok: true,
                    json: async () => ({
                        lastFetched: statusCall === 1 ? 't1' : 't2',
                        lastError: statusCall === 1 ? null : 'boom',
                    }),
                    text: async () => '',
                };
            }
            if (url === '/api/feeds/9/refresh') {
                return { ok: true, json: async () => ({}), text: async () => '' };
            }
            if (url === '/api/counts') {
                return {
                    ok: true,
                    json: async () => ({ unread: 0, starred: 0, queue: 0, categories: {}, feeds: {}, feedErrors: {} }),
                    text: async () => '',
                };
            }
            return { ok: true, json: async () => ({}), text: async () => '' };
        });

        document.body.innerHTML = `
            <table>
                <tbody>
                    <tr data-feed-id="9">
                        <td>Name</td>
                        <td>Status</td>
                        <td>Actions</td>
                    </tr>
                </tbody>
            </table>
            <button data-feed-id="9">Refresh</button>
        `;

        const promise = refreshFeed(9);
        await Promise.resolve();
        await vi.advanceTimersByTimeAsync(1000);
        await Promise.resolve();

        await promise;

        const row = document.querySelector('tr[data-feed-id="9"]');
        const statusCell = row.querySelectorAll('td')[1];
        expect(statusCell.innerHTML).toContain('Error');
        expect(row.dataset.hasError).toBe('true');

        await vi.advanceTimersByTimeAsync(2000);

        expect(document.querySelector('button[data-feed-id="9"]').disabled).toBe(false);

        vi.useRealTimers();
    });

    it('shows checkmark on success (no error) and restores button after 2s', async () => {
        vi.useFakeTimers();

        let statusCall = 0;
        vi.spyOn(globalThis, 'fetch').mockImplementation(async (url) => {
            if (url === '/api/feeds/3/status') {
                statusCall += 1;
                return {
                    ok: true,
                    json: async () => ({
                        lastFetched: statusCall === 1 ? 't1' : 't2',
                        lastError: null,
                    }),
                    text: async () => '',
                };
            }
            if (url === '/api/feeds/3/refresh') {
                return { ok: true, json: async () => ({}), text: async () => '' };
            }
            if (url === '/api/counts') {
                return {
                    ok: true,
                    json: async () => ({ unread: 0, starred: 0, queue: 0, categories: {}, feeds: {}, feedErrors: {} }),
                    text: async () => '',
                };
            }
            return { ok: true, json: async () => ({}), text: async () => '' };
        });

        document.body.innerHTML = `<button data-feed-id="3">Refresh</button>`;

        const promise = refreshFeed(3);
        await Promise.resolve();
        // Advance past the initial 1s polling delay
        await vi.advanceTimersByTimeAsync(1000);
        await Promise.resolve();
        await promise;

        const btn = document.querySelector('button[data-feed-id="3"]');
        // Button should show success icon
        expect(btn.innerHTML).toContain('✓');
        expect(btn.innerHTML).toContain('Done');
        expect(btn.disabled).toBe(false);

        // After 2s, button should restore original content
        await vi.advanceTimersByTimeAsync(2000);
        expect(btn.innerHTML).toBe('Refresh');

        vi.useRealTimers();
    });

    it('restores buttons after timeout (30 attempts exhausted)', async () => {
        vi.useFakeTimers();

        vi.spyOn(globalThis, 'fetch').mockImplementation(async (url) => {
            if (url === '/api/feeds/4/status') {
                return {
                    ok: true,
                    json: async () => ({ lastFetched: 'same-time', lastError: null }),
                    text: async () => '',
                };
            }
            if (url === '/api/feeds/4/refresh') {
                return { ok: true, json: async () => ({}), text: async () => '' };
            }
            if (url === '/api/counts') {
                return {
                    ok: true,
                    json: async () => ({ unread: 0, starred: 0, queue: 0, categories: {}, feeds: {}, feedErrors: {} }),
                    text: async () => '',
                };
            }
            return { ok: true, json: async () => ({}), text: async () => '' };
        });

        document.body.innerHTML = `<button data-feed-id="4">Refresh</button>`;

        const promise = refreshFeed(4);
        await Promise.resolve();

        // Advance through all 30 polling attempts (1s initial + 30 x 1s)
        for (let i = 0; i < 31; i++) {
            await vi.advanceTimersByTimeAsync(1000);
            await Promise.resolve();
        }

        await promise;

        const btn = document.querySelector('button[data-feed-id="4"]');
        // Button should be restored to original content
        expect(btn.disabled).toBe(false);
        expect(btn.innerHTML).toBe('Refresh');

        vi.useRealTimers();
    });

    it('shows error on catch and restores button after 2s', async () => {
        vi.useFakeTimers();

        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network down'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        document.body.innerHTML = `<button data-feed-id="5">Refresh</button>`;

        await refreshFeed(5);

        const btn = document.querySelector('button[data-feed-id="5"]');
        // Button should show error
        expect(btn.innerHTML).toContain('✗');
        expect(btn.innerHTML).toContain('Failed');
        expect(btn.disabled).toBe(false);
        expect(console.error).toHaveBeenCalledWith('Failed to refresh feed:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to refresh feed');

        // After 2s, button should restore
        await vi.advanceTimersByTimeAsync(2000);
        expect(btn.innerHTML).toBe('Refresh');

        vi.useRealTimers();
    });
});

describe('initFeedActionListeners', () => {
    beforeEach(() => {
        document.body.innerHTML = '';
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        }));
    });

    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('delegates refresh-feed clicks', async () => {
        document.body.innerHTML = `
            <button data-action="refresh-feed" data-feed-id="42">Refresh</button>
            <span data-feed-status="42"></span>
        `;
        initFeedActionListeners();

        document.querySelector('[data-action="refresh-feed"]').click();
        await new Promise(r => setTimeout(r, 10));

        expect(fetch).toHaveBeenCalledWith('/api/feeds/42/refresh', expect.any(Object));
    });

    it('delegates edit-feed clicks', async () => {
        document.body.innerHTML = `
            <button data-action="edit-feed" data-feed-id="7">Edit</button>
        `;
        initFeedActionListeners();

        // editFeed calls api('GET', '/api/feeds/7') then creates a modal
        fetch.mockResolvedValueOnce({
            ok: true,
            json: () => Promise.resolve({ id: 7, name: 'Test', url: 'http://example.com', interval: 60, content_filters: '' }),
        });

        document.querySelector('[data-action="edit-feed"]').click();
        await new Promise(r => setTimeout(r, 10));

        expect(fetch).toHaveBeenCalledWith('/api/feeds/7', expect.any(Object));
    });

    it('ignores clicks without feed id', () => {
        document.body.innerHTML = `
            <button data-action="refresh-feed">Refresh</button>
        `;
        initFeedActionListeners();

        document.querySelector('[data-action="refresh-feed"]').click();
        expect(fetch).not.toHaveBeenCalled();
    });

    it('delegates delete-feed click', () => {
        document.body.innerHTML = `
            <button data-action="delete-feed" data-feed-id="9" data-feed-name="Old Blog">Delete</button>
        `;
        initFeedActionListeners();
        vi.spyOn(window, 'confirm').mockReturnValue(false);

        document.querySelector('[data-action="delete-feed"]').click();

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Old Blog'));
    });

    it('delegates filter-feeds checkbox change', () => {
        document.body.innerHTML = `
            <input type="checkbox" data-action="filter-feeds" id="filter-errors">
            <input type="search" id="feeds-search" value="">
            <table class="feeds-table"><tbody>
                <tr data-has-error="true"><td>Feed</td><td class="url-cell">url</td></tr>
                <tr><td>Good Feed</td><td class="url-cell">url2</td></tr>
            </tbody></table>
        `;
        initFeedActionListeners();

        const checkbox = document.getElementById('filter-errors');
        checkbox.checked = true;
        checkbox.dispatchEvent(new Event('change', { bubbles: true }));

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        expect(rows[0].style.display).toBe(''); // error row visible
        expect(rows[1].style.display).toBe('none'); // non-error row hidden
    });

    it('delegates filter-feeds search input', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="filter-errors">
            <input type="search" data-action="filter-feeds" id="feeds-search" value="tech">
            <table class="feeds-table"><tbody>
                <tr><td>Tech Blog</td><td class="url-cell">url</td></tr>
                <tr><td>News</td><td class="url-cell">url2</td></tr>
            </tbody></table>
        `;
        initFeedActionListeners();

        const search = document.getElementById('feeds-search');
        search.dispatchEvent(new Event('input', { bubbles: true }));

        const rows = document.querySelectorAll('.feeds-table tbody tr');
        expect(rows[0].style.display).toBe('');
        expect(rows[1].style.display).toBe('none');
    });

    it('delegates set-feed-category change', async () => {
        document.body.innerHTML = `
            <select data-action="set-feed-category" data-feed-id="5">
                <option value="0">None</option>
                <option value="3">Tech</option>
            </select>
        `;
        initFeedActionListeners();

        const select = document.querySelector('[data-action="set-feed-category"]');
        select.value = '3';
        select.dispatchEvent(new Event('change', { bubbles: true }));
        await new Promise(r => setTimeout(r, 10));

        expect(fetch).toHaveBeenCalledWith('/api/feeds/5/category', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ categoryId: 3 }),
        }));
    });
});

describe('initAddFeedForm', () => {
    it('is a no-op when form is absent', () => {
        initAddFeedForm();
        // No error thrown
    });

    it('submits a basic RSS feed', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="https://example.com/feed.xml">
                <input id="feed-name" value="My Feed">
                <select id="feed-type"><option value="rss" selected>RSS</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: async () => ({ id: 1 }),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await new Promise(r => setTimeout(r, 50));

        expect(fetch).toHaveBeenCalledWith('/api/feeds', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({
                url: 'https://example.com/feed.xml',
                name: 'My Feed',
                feedType: 'rss',
                scraperModule: '',
                scraperConfig: '',
                interval: 60,
            }),
        }));
        expect(reloadMock).toHaveBeenCalled();
    });

    it('builds Reddit RSS URL from subreddit', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="">
                <input id="feed-name" value="">
                <select id="feed-type"><option value="reddit" selected>Reddit</option></select>
                <input id="reddit-subreddit" value="r/javascript">
                <select id="reddit-sort"><option value="hot" selected>Hot</option></select>
                <select id="reddit-top-period"><option value="day" selected>Day</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: async () => ({ id: 2 }),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await new Promise(r => setTimeout(r, 50));

        expect(fetch).toHaveBeenCalledWith('/api/feeds', expect.objectContaining({
            method: 'POST',
            body: expect.stringContaining('"url":"https://www.reddit.com/r/javascript/hot/.rss"'),
        }));
    });

    it('rejects empty subreddit for Reddit feed', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="">
                <input id="feed-name" value="">
                <select id="feed-type"><option value="reddit" selected>Reddit</option></select>
                <input id="reddit-subreddit" value="">
                <select id="reddit-sort"><option value="hot" selected>Hot</option></select>
                <select id="reddit-top-period"><option value="day" selected>Day</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        const fetchSpy = vi.spyOn(globalThis, 'fetch');

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await new Promise(r => setTimeout(r, 50));

        expect(showToast).toHaveBeenCalledWith('Please enter a subreddit name', 'info');
        expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('builds HuggingFace config with daily_papers', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="">
                <input id="feed-name" value="">
                <select id="feed-type"><option value="huggingface" selected>HF</option></select>
                <select id="hf-type"><option value="daily_papers" selected>Daily Papers</option></select>
                <input id="hf-identifier" value="">
                <input id="feed-interval" value="60">
            </form>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: async () => ({ id: 3 }),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await new Promise(r => setTimeout(r, 50));

        expect(fetch).toHaveBeenCalledWith('/api/feeds', expect.objectContaining({
            method: 'POST',
            body: expect.stringContaining('"url":"https://huggingface.co/papers"'),
        }));
    });

    it('sets category after creating feed', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="https://example.com/feed.xml">
                <input id="feed-name" value="My Feed">
                <select id="feed-type"><option value="rss" selected>RSS</option></select>
                <select id="feed-category"><option value="5" selected>Tech</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        vi.spyOn(globalThis, 'fetch')
            .mockResolvedValueOnce({ ok: true, json: async () => ({ id: 10 }) })
            .mockResolvedValueOnce({ ok: true, json: async () => ({}) });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await new Promise(r => setTimeout(r, 50));

        // First call: create feed, second call: set category
        expect(fetch).toHaveBeenCalledTimes(2);
        expect(fetch).toHaveBeenCalledWith('/api/feeds/10/category', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ categoryId: 5 }),
        }));
    });

    it('shows toast on API failure', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="https://example.com/feed.xml">
                <input id="feed-name" value="My Feed">
                <select id="feed-type"><option value="rss" selected>RSS</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            json: async () => ({ error: 'Bad URL' }),
        });

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await new Promise(r => setTimeout(r, 50));

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to add feed'));
    });
});

describe('initFeedItemClickListeners', () => {
    it('does SPA navigation when articles-view is present', () => {
        document.body.innerHTML = `
            <div class="articles-view">
                <div class="view-header"><h1>Title</h1></div>
            </div>
            <div id="articles-list"></div>
            <a class="feed-item" data-feed-id="42" data-feed-name="Test Feed" href="/feed/42">
                <span class="feed-name">Test Feed</span>
            </a>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: async () => ({ feed: { id: 42 }, articles: [] }),
        });
        vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
        initFeedItemClickListeners();

        const link = document.querySelector('.feed-item');
        const event = new Event('click', { cancelable: true, bubbles: true });
        event.preventDefault = vi.fn();
        link.dispatchEvent(event);

        expect(event.preventDefault).toHaveBeenCalled();
    });

    it('falls through to normal navigation on non-article pages', () => {
        document.body.innerHTML = `
            <a class="feed-item" data-feed-id="42" data-feed-name="Test Feed" href="/feed/42">
                <span class="feed-name">Test Feed</span>
            </a>
        `;
        initFeedItemClickListeners();

        const link = document.querySelector('.feed-item');
        const event = new Event('click', { cancelable: true, bubbles: true });
        event.preventDefault = vi.fn();
        link.dispatchEvent(event);

        expect(event.preventDefault).not.toHaveBeenCalled();
    });

    it('reads feed name from data attribute and calls loadFeedArticles', () => {
        document.body.innerHTML = `
            <div class="articles-view">
                <div class="view-header"><h1>Title</h1></div>
            </div>
            <div id="articles-list"></div>
            <a class="feed-item" data-feed-id="7" data-feed-name="Custom Name" href="/feed/7"></a>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: async () => ({ feed: { id: 7 }, articles: [] }),
        });
        vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
        initFeedItemClickListeners();

        const link = document.querySelector('.feed-item');
        link.dispatchEvent(new Event('click', { cancelable: true, bubbles: true }));

        // loadFeedArticles is called with feedId="7" and feedName="Custom Name"
        // Verified indirectly: the fetch call targets the correct feed
        expect(fetch).toHaveBeenCalledWith('/api/feeds/7/articles', expect.any(Object));
    });
});
