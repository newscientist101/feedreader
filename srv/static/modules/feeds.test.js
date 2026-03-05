import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    loadCategoryArticles, loadFeedArticles,
    filterFeeds, closeEditModal, saveFeed,
    deleteFeed, setFeedCategory, initFeedActionListeners,
    initAddFeedForm, initFeedItemClickListeners,
    refreshFeed, editFeed, createEditFeedModal,
    _resetFeedsState,
} from './feeds.js';
vi.mock('./articles.js');
vi.mock('./api.js');
vi.mock('./modal.js');

import { api } from './api.js';
import { _resetArticlesState, renderArticles } from './articles.js';
import { _resetArticleActionsState, setQueuedArticleIds } from './article-actions.js';
import { showToast } from './toast.js';
import { applyDefaultViewForScope } from './views.js';
import { showFeedErrorBanner, removeFeedErrorBanner } from './feed-errors.js';
import { makeCountsResponse, flushPromises } from './test-helpers.js';

vi.mock('./toast.js');

// Mock pagination (articles.js directly imports from pagination.js)
vi.mock('./pagination.js');

vi.mock('./views.js');

vi.mock('./feed-errors.js');


beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
    _resetArticlesState();
    _resetArticleActionsState();
    _resetFeedsState();
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
        api.mockResolvedValue({ articles: [{ id: 1, title: 'Test', feed_name: 'F' }] });

        await loadCategoryArticles(3, 'Tech');

        expect(document.querySelector('.view-header h1').textContent).toBe('Tech');
        expect(document.title).toBe('Tech - FeedReader');
        expect(history.pushState).toHaveBeenCalledWith({ spaNav: true, categoryId: 3 }, 'Tech', '/category/3');
        const dropdown = document.querySelector('.dropdown');
        expect(dropdown.dataset.feedId).toBe('');
        expect(dropdown.dataset.categoryId).toBe('3');
        expect(removeFeedErrorBanner).toHaveBeenCalled();
        expect(applyDefaultViewForScope).toHaveBeenCalledWith('folder');
        expect(renderArticles).toHaveBeenCalledWith([{ id: 1, title: 'Test', feed_name: 'F' }]);
    });

    it('hides feed action buttons', async () => {
        api.mockResolvedValue({ articles: [] });

        await loadCategoryArticles(3, 'Tech');

        const editBtn = document.querySelector('[data-feed-action="edit"]');
        expect(editBtn.style.display).toBe('none');
    });

    it('handles API errors gracefully', async () => {
        api.mockRejectedValue(new Error('Network error'));
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
        api.mockResolvedValue({
            articles: [{ id: 1, title: 'Test', feed_name: 'F' }],
            feed: { id: 7, name: 'Feed', last_error: null },
            });
        await loadFeedArticles(7, 'Feed');

        expect(document.querySelector('.view-header h1').textContent).toBe('Feed');
        expect(document.title).toBe('Feed - FeedReader');
        expect(history.pushState).toHaveBeenCalledWith({ spaNav: true, feedId: 7 }, 'Feed', '/feed/7');
        const dropdown = document.querySelector('.dropdown');
        expect(dropdown.dataset.feedId).toBe('7');
        expect(dropdown.dataset.categoryId).toBe('');
        expect(removeFeedErrorBanner).toHaveBeenCalled();
        expect(applyDefaultViewForScope).toHaveBeenCalledWith('feed');
        expect(renderArticles).toHaveBeenCalledWith([{ id: 1, title: 'Test', feed_name: 'F' }]);
    });

    it('sets active state on matching feed items', async () => {
        api.mockResolvedValue({
            articles: [],
            feed: { id: 7, name: 'Feed', last_error: null },
            });

        await loadFeedArticles(7, 'Feed');

        expect(document.querySelector('.feed-item[data-feed-id="7"]').classList.contains('active')).toBe(true);
    });

    it('shows error banner when feed has last_error', async () => {
        api.mockResolvedValue({
            articles: [],
            feed: { id: 7, name: 'Feed', last_error: 'Timeout' },
            });

        await loadFeedArticles(7, 'Feed');

        expect(showFeedErrorBanner).toHaveBeenCalledWith(7, 'Timeout');
    });

    it('creates edit and refresh buttons in header-actions', async () => {
        api.mockResolvedValue({
            articles: [],
            feed: { id: 7, name: 'Feed', last_error: null },
            });

        await loadFeedArticles(7, 'Feed');

        const editBtn = document.querySelector('[data-feed-action="edit"]');
        expect(editBtn).not.toBeNull();
        const refreshBtn = document.querySelector('[data-feed-action="refresh"]');
        expect(refreshBtn).not.toBeNull();
        expect(refreshBtn.dataset.feedId).toBe('7');
    });

    it('handles API errors gracefully', async () => {
        api.mockRejectedValue(new Error('Network error'));
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
            <input type="checkbox" id="edit-feed-skip-retention">
            <table><tbody>
                <tr data-feed-id="5">
                    <td><a>Old Name</a></td>
                    <td><a href="https://old.com" class="url-cell">https://old.com</a></td>
                </tr>
            </tbody></table>
        `;
    });

    it('saves feed and updates row in place', async () => {
        api.mockResolvedValue({});
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        expect(event.preventDefault).toHaveBeenCalled();
        expect(api).toHaveBeenCalledWith('PUT', '/api/feeds/5', expect.objectContaining({
            name: 'Updated Feed',
        }));
        // Modal should be hidden
        expect(document.getElementById('edit-feed-modal').style.display).toBe('none');
        // Row should be updated
        const row = document.querySelector('tr[data-feed-id="5"]');
        expect(row.querySelector('td a').textContent).toBe('Updated Feed');
    });

    it('sends content filters as JSON', async () => {
        document.getElementById('edit-feed-filters').value = '.ad\n.sidebar';
        api.mockResolvedValue({});
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        const body = api.mock.calls[0][2];
        const filters = JSON.parse(body.content_filters);
        expect(filters).toEqual([{ selector: '.ad' }, { selector: '.sidebar' }]);
    });

    it('updates sidebar feed name when matching element exists', async () => {
        // Add a sidebar item matching feed id 5
        const sidebar = document.createElement('div');
        sidebar.innerHTML = '<a class="feed-item" href="/feed/5"><span class="feed-name">Old Name</span></a>';
        document.body.appendChild(sidebar);

        api.mockResolvedValue({});
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        expect(document.querySelector('.feed-item[href="/feed/5"] .feed-name').textContent).toBe('Updated Feed');
    });

    it('updates header and document.title when on matching feed page', async () => {
        // Add view header
        const header = document.createElement('div');
        header.className = 'view-header';
        header.innerHTML = '<h1>Old Name</h1>';
        document.body.appendChild(header);

        // Set location to /feed/5
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/feed/5' },
            writable: true,
            configurable: true,
        });

        api.mockResolvedValue({});
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        expect(document.querySelector('.view-header h1').textContent).toBe('Updated Feed');
        expect(document.title).toBe('Updated Feed - FeedReader');
    });

    it('does not update header when on a different page', async () => {
        // Add view header
        const header = document.createElement('div');
        header.className = 'view-header';
        header.innerHTML = '<h1>Other Page</h1>';
        document.body.appendChild(header);

        // Set location to a different feed page
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/feed/99' },
            writable: true,
            configurable: true,
        });

        api.mockResolvedValue({});
        const event = { preventDefault: vi.fn() };

        await saveFeed(event);

        // Header should NOT be updated since we're on /feed/99, not /feed/5
        expect(document.querySelector('.view-header h1').textContent).toBe('Other Page');
    });

    it('handles save errors', async () => {
        api.mockRejectedValue(new Error('Not found'));
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
        api.mockResolvedValue({});
        // Mock location.reload
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await deleteFeed(5, 'Test Feed');

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Test Feed'));
        expect(api).toHaveBeenCalledWith('DELETE', '/api/feeds/5');
    });

    it('does nothing when user cancels', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(false);

        await deleteFeed(5, 'Test Feed');

        expect(api).not.toHaveBeenCalled();
    });
});

describe('setFeedCategory', () => {
    it('calls the category API', async () => {
        api.mockResolvedValue({});

        await setFeedCategory(5, 3);

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds/5/category', { categoryId: 3 });
    });

    it('handles errors gracefully', async () => {
        api.mockRejectedValue(new Error('fail'));
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
        expect(document.getElementById('edit-feed-skip-retention')).not.toBeNull();
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
        api.mockResolvedValue({
            id: 42,
            name: 'Test Feed',
            url: 'https://example.com/rss',
            fetch_interval_minutes: 120,
            content_filters: null,
            skip_retention: 0,
        });

        await editFeed(42);

        expect(document.getElementById('edit-feed-id').value).toBe('42');
        expect(document.getElementById('edit-feed-name').value).toBe('Test Feed');
        expect(document.getElementById('edit-feed-url').value).toBe('https://example.com/rss');
        expect(document.getElementById('edit-feed-interval').value).toBe('120');
        expect(document.getElementById('edit-feed-filters').value).toBe('');
        expect(document.getElementById('edit-feed-skip-retention').checked).toBe(false);
        // Modal should be visible
        expect(document.getElementById('edit-feed-modal').style.display).toBe('flex');
    });

    it('parses content_filters JSON into newline-separated textarea', async () => {
        api.mockResolvedValue({
            id: 5,
            name: 'Feed',
            url: 'https://example.com',
            fetch_interval_minutes: 60,
            content_filters: JSON.stringify([{ selector: '.ad' }, { selector: '#sidebar' }]),
        });

        await editFeed(5);

        expect(document.getElementById('edit-feed-filters').value).toBe('.ad\n#sidebar');
    });

    it('handles empty content_filters', async () => {
        api.mockResolvedValue({
            id: 5,
            name: 'Feed',
            url: 'https://example.com',
            fetch_interval_minutes: 60,
            content_filters: '',
        });

        await editFeed(5);

        expect(document.getElementById('edit-feed-filters').value).toBe('');
    });

    it('handles invalid JSON in content_filters gracefully', async () => {
        api.mockResolvedValue({
            id: 5,
            name: 'Feed',
            url: 'https://example.com',
            fetch_interval_minutes: 60,
            content_filters: 'not-valid-json',
        });

        await editFeed(5);

        // Should fall back to empty string on parse error
        expect(document.getElementById('edit-feed-filters').value).toBe('');
    });

    it('uses default interval of 60 when fetch_interval_minutes is null', async () => {
        api.mockResolvedValue({
            id: 5,
            name: 'Feed',
            url: 'https://example.com',
            fetch_interval_minutes: null,
            content_filters: null,
        });

        await editFeed(5);

        expect(document.getElementById('edit-feed-interval').value).toBe('60');
    });

    it('shows toast on API failure', async () => {
        api.mockRejectedValue(new Error('Not found'));
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
        api.mockImplementation(async (method, url) => {
            if (url === '/api/feeds/9/status') {
                statusCall += 1;
                return {
                    lastFetched: statusCall === 1 ? 't1' : 't2',
                    lastError: statusCall === 1 ? null : 'boom',
                };
            }
            if (url === '/api/feeds/9/refresh') {
                return {};
            }
            if (url === '/api/counts') {
                return makeCountsResponse();
            }
            return {};
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
        api.mockImplementation(async (method, url) => {
            if (url === '/api/feeds/3/status') {
                statusCall += 1;
                return {
                    lastFetched: statusCall === 1 ? 't1' : 't2',
                    lastError: null,
                };
            }
            if (url === '/api/feeds/3/refresh') {
                return {};
            }
            if (url === '/api/counts') {
                return makeCountsResponse();
            }
            return {};
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

        api.mockImplementation(async (method, url) => {
            if (url === '/api/feeds/4/status') {
                return { lastFetched: 'same-time', lastError: null };
            }
            if (url === '/api/feeds/4/refresh') {
                return {};
            }
            if (url === '/api/counts') {
                return makeCountsResponse();
            }
            return {};
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

        api.mockRejectedValue(new Error('Network down'));
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
        api.mockResolvedValue({});
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it('delegates refresh-feed clicks', async () => {
        document.body.innerHTML = `
            <button data-action="refresh-feed" data-feed-id="42">Refresh</button>
            <span data-feed-status="42"></span>
        `;
        initFeedActionListeners();

        document.querySelector('[data-action="refresh-feed"]').click();

        await flushPromises();
        expect(api).toHaveBeenCalledWith('POST', '/api/feeds/42/refresh');
    });

    it('delegates edit-feed clicks', async () => {
        document.body.innerHTML = `
            <button data-action="edit-feed" data-feed-id="7">Edit</button>
        `;
        initFeedActionListeners();

        // editFeed calls api('GET', '/api/feeds/7') then creates a modal
        api.mockResolvedValueOnce({ id: 7, name: 'Test', url: 'http://example.com', interval: 60, content_filters: '' });

        document.querySelector('[data-action="edit-feed"]').click();

        await flushPromises();
        expect(api).toHaveBeenCalledWith('GET', '/api/feeds/7');
    });

    it('ignores clicks without feed id', () => {
        document.body.innerHTML = `
            <button data-action="refresh-feed">Refresh</button>
        `;
        initFeedActionListeners();

        document.querySelector('[data-action="refresh-feed"]').click();
        expect(api).not.toHaveBeenCalled();
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

    it('delegates close-edit-modal click to hide the modal', () => {
        document.body.innerHTML = `
            <div id="edit-feed-modal" class="modal" style="display: flex">
                <button data-action="close-edit-modal">Close</button>
            </div>
        `;
        initFeedActionListeners();

        document.querySelector('[data-action="close-edit-modal"]').click();

        expect(document.getElementById('edit-feed-modal').style.display).toBe('none');
    });

    it('delegates edit-feed-form submit to saveFeed', async () => {
        document.body.innerHTML = `
            <div id="edit-feed-modal" class="modal" style="display: flex"></div>
            <form id="edit-feed-form">
                <input id="edit-feed-id" value="11">
                <input id="edit-feed-name" value="Saved Feed">
                <input id="edit-feed-url" value="https://saved.com/rss">
                <input id="edit-feed-interval" value="45">
                <textarea id="edit-feed-filters"></textarea>
                <input type="checkbox" id="edit-feed-skip-retention">
            </form>
        `;
        initFeedActionListeners();

        document.getElementById('edit-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true, bubbles: true })
        );

        await flushPromises();
        expect(api).toHaveBeenCalledWith('PUT', '/api/feeds/11', expect.objectContaining({
            name: 'Saved Feed',
        }));
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

        await flushPromises();
        expect(api).toHaveBeenCalledWith('POST', '/api/feeds/5/category', { categoryId: 3 });
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
        api.mockResolvedValue({ id: 1 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', {
            url: 'https://example.com/feed.xml',
            name: 'My Feed',
            feedType: 'rss',
            scraperModule: '',
            scraperConfig: '',
            interval: 60,
        });
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
        api.mockResolvedValue({ id: 2 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', expect.objectContaining({
            url: 'https://www.reddit.com/r/javascript/hot/.rss',
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

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await flushPromises();
        expect(showToast).toHaveBeenCalledWith('Please enter a subreddit name', 'info');
        expect(api).not.toHaveBeenCalled();
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
        api.mockResolvedValue({ id: 3 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', expect.objectContaining({
            url: 'https://huggingface.co/papers',
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
        api
            .mockResolvedValueOnce({ id: 10 })
            .mockResolvedValueOnce({});
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        // First call: create feed, second call: set category
        expect(api).toHaveBeenCalledTimes(2);
        expect(api).toHaveBeenCalledWith('POST', '/api/feeds/10/category', { categoryId: 5 });
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
        api.mockRejectedValue(new Error('Bad URL'));

        initAddFeedForm();
        document.getElementById('add-feed-form').dispatchEvent(
            new Event('submit', { cancelable: true })
        );

        await flushPromises();
        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to add feed'));
    });

    it('sends empty name for RSS feed when no name provided', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="https://store.steampowered.com/news/app/123">
                <input id="feed-name" value="">
                <select id="feed-type"><option value="rss" selected>RSS</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        api.mockResolvedValue({ id: 1 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', {
            url: 'https://store.steampowered.com/news/app/123',
            name: '',
            feedType: 'rss',
            scraperModule: '',
            scraperConfig: '',
            interval: 60,
        });
    });

    it('sends empty name for Reddit feed when no name provided', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="">
                <input id="feed-name" value="">
                <select id="feed-type"><option value="reddit" selected>Reddit</option></select>
                <input id="reddit-subreddit" value="r/gaming">
                <select id="reddit-sort"><option value="hot" selected>Hot</option></select>
                <select id="reddit-top-period"><option value="day" selected>Day</option></select>
                <input id="feed-interval" value="60">
            </form>
        `;
        api.mockResolvedValue({ id: 2 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', {
            url: 'https://www.reddit.com/r/gaming/hot/.rss',
            name: '',
            feedType: 'rss',
            scraperModule: '',
            scraperConfig: '',
            interval: 60,
        });
    });

    it('sends empty name for HuggingFace feed when no name provided', async () => {
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
        api.mockResolvedValue({ id: 3 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', expect.objectContaining({
            name: '',
        }));
    });

    it('includes scraper module when selected', async () => {
        document.body.innerHTML = `
            <form id="add-feed-form">
                <input id="feed-url" value="https://example.com/news">
                <input id="feed-name" value="News Scraper">
                <select id="feed-type"><option value="rss" selected>RSS</option></select>
                <select id="scraper-module">
                    <option value="">None</option>
                    <option value="my-scraper" selected>My Scraper</option>
                </select>
                <input id="feed-interval" value="30">
            </form>
        `;
        api.mockResolvedValue({ id: 7 });
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

        await flushPromises();
        expect(reloadMock).toHaveBeenCalled();

        expect(api).toHaveBeenCalledWith('POST', '/api/feeds', {
            url: 'https://example.com/news',
            name: 'News Scraper',
            feedType: 'rss',
            scraperModule: 'my-scraper',
            scraperConfig: '',
            interval: 30,
        });
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
        api.mockResolvedValue({ feed: { id: 42 }, articles: [] });
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
        api.mockResolvedValue({ feed: { id: 7 }, articles: [] });
        vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
        initFeedItemClickListeners();

        const link = document.querySelector('.feed-item');
        link.dispatchEvent(new Event('click', { cancelable: true, bubbles: true }));

        // loadFeedArticles is called with feedId="7" and feedName="Custom Name"
        // Verified indirectly: the fetch call targets the correct feed
        expect(api).toHaveBeenCalledWith('GET', '/api/feeds/7/articles');
    });
});
