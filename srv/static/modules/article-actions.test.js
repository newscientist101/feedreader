import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    initAutoMarkRead, observeNewArticles, flushMarkReadQueue,
    markCardAsRead, markReadSilent, openArticle, openArticleExternal,
    markRead, markUnread, toggleStar, toggleQueue, markAsRead,
    findNextUnreadFolder, initArticleActionListeners, initQueueState,
    setQueuedArticleIds, setQueuedIdsReady, queuedArticleIds,
    _resetArticleActionsState,
    _getAutoMarkReadObserver, _getMarkReadQueue,
} from './article-actions.js';
import { renderArticles, _resetArticlesState } from './articles.js';
import { updateCounts } from './counts.js';
import { updateQueueCacheIfStandalone } from './offline.js';
import { updateReadButton } from './read-button.js';
import { showToast } from './toast.js';
import {
    SVG_STAR_FILLED, SVG_STAR_EMPTY, SVG_QUEUE_ADD, SVG_QUEUE_REMOVE
} from './icons.js';
import { MockIntersectionObserver } from './test-helpers.js';

vi.mock('./pagination.js');
vi.mock('./counts.js');
vi.mock('./offline.js');
vi.mock('./read-button.js');
vi.mock('./toast.js');

beforeEach(() => {
    vi.useFakeTimers();
    _resetArticleActionsState();
    _resetArticlesState();
    window.IntersectionObserver = MockIntersectionObserver;
    window.__settings = {};
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }));
    document.body.innerHTML = '<div id="articles-list"></div>';
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
});

describe('initAutoMarkRead', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
    });

    it('does nothing when autoMarkRead setting is not true', () => {
        window.__settings = { autoMarkRead: 'false' };
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).toBeNull();
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] disabled by setting');
    });

    it('creates an IntersectionObserver when autoMarkRead is true', () => {
        window.__settings = { autoMarkRead: 'true' };
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="1"></div>';
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 1 initial articles');
    });

    it('disconnects previous observer on re-init', () => {
        window.__settings = { autoMarkRead: 'true' };
        initAutoMarkRead();
        const first = _getAutoMarkReadObserver();
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBe(first);
        expect(console.debug).toHaveBeenCalledTimes(2);
    });
});

describe('observeNewArticles', () => {
    it('is a no-op when observer is null', () => {
        const container = document.createElement('div');
        container.innerHTML = '<div class="article-card"></div>';
        observeNewArticles(container);
        // Should not throw
    });

    it('observes new cards in the container', () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        window.__settings = { autoMarkRead: 'true' };
        initAutoMarkRead();
        const obs = _getAutoMarkReadObserver();
        const spy = vi.spyOn(obs, 'observe');
        const container = document.createElement('div');
        container.innerHTML = '<div class="article-card"></div><div class="article-card"></div>';
        observeNewArticles(container);
        expect(spy).toHaveBeenCalledTimes(2);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 new articles');
    });
});

describe('markCardAsRead', () => {
    it('adds read class to the matching card', () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="42"></div>';
        markCardAsRead(42);
        expect(document.querySelector('.article-card').classList.contains('read')).toBe(true);
    });

    it('does not throw for non-existent card', () => {
        markCardAsRead(999);
        // No error expected
    });
});

describe('markReadSilent', () => {
    it('queues article id and sets a timer', () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="42"></div>';
        markReadSilent(42);
        expect(_getMarkReadQueue()).toContain(42);
    });

    it('flushes the queue after a timeout', () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: true, json: () => Promise.resolve({ status: 'ok' }),
        }));
        markReadSilent(1);
        markReadSilent(2);
        expect(globalThis.fetch).not.toHaveBeenCalled();

        vi.advanceTimersByTime(500);
        expect(globalThis.fetch).toHaveBeenCalledTimes(1);

        const [url, opts] = globalThis.fetch.mock.calls[0];
        expect(url).toBe('/api/articles/batch-read');
        const body = JSON.parse(opts.body);
        expect(body.ids).toEqual([1, 2]);
        expect(console.debug).toHaveBeenCalledWith(
            expect.stringContaining('flushing batch of 2'), expect.arrayContaining([1, 2])
        );
    });

    it('resets the timer when called rapidly', () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: true, json: () => Promise.resolve({ status: 'ok' }),
        }));
        markReadSilent(1);
        vi.advanceTimersByTime(150);
        markReadSilent(2);
        vi.advanceTimersByTime(150);
        // Only 300ms total, but timer was reset at 150ms so another 100ms to go
        expect(globalThis.fetch).not.toHaveBeenCalled();
        vi.advanceTimersByTime(100);
        expect(globalThis.fetch).toHaveBeenCalledTimes(1);
        const body = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
        expect(body.ids).toEqual([1, 2]);
        expect(console.debug).toHaveBeenCalledWith(
            expect.stringContaining('flushing batch of 2'), expect.arrayContaining([1, 2])
        );
    });
});

describe('flushMarkReadQueue', () => {
    it('posts queued ids as a batch', () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        markReadSilent(1);
        markReadSilent(2);
        flushMarkReadQueue();
        expect(globalThis.fetch).toHaveBeenCalledWith(
            '/api/articles/batch-read',
            expect.objectContaining({
                method: 'POST',
                body: expect.stringContaining('"ids"'),
            })
        );
        expect(console.debug).toHaveBeenCalledWith(
            expect.stringContaining('flushing batch of 2'), expect.arrayContaining([1, 2])
        );
    });

    it('does nothing when queue is empty', () => {
        flushMarkReadQueue();
        expect(globalThis.fetch).not.toHaveBeenCalled();
    });
});

describe('openArticle', () => {
    it('marks the article as read, flushes queue, and navigates to /article/{id}', () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        Object.defineProperty(window, 'location', {
            value: { pathname: '/', hostname: 'localhost' },
            writable: true,
            configurable: true,
        });
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="5"></div>';
        openArticle(5);
        // flushMarkReadQueue was called — article 5 should be in the batch POST
        expect(globalThis.fetch).toHaveBeenCalledWith(
            '/api/articles/batch-read',
            expect.objectContaining({
                method: 'POST',
                body: expect.stringContaining('"ids":[5]'),
            })
        );
        expect(console.debug).toHaveBeenCalledWith(
            expect.stringContaining('flushing batch of 1'), expect.arrayContaining([5])
        );
        // Assert navigation target
        expect(window.location).toBe('/article/5');
    });
});

describe('openArticleExternal', () => {
    it('stops propagation and opens new tab', () => {
        const event = { stopPropagation: vi.fn() };
        const openSpy = vi.fn();
        window.open = openSpy;
        openArticleExternal(event, 5, 'https://example.com');
        expect(event.stopPropagation).toHaveBeenCalled();
        expect(openSpy).toHaveBeenCalledWith('https://example.com', '_blank');
    });
});

describe('markRead', () => {
    it('calls API and updates DOM', async () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="10"><button class="btn-read-toggle"></button></div>';
        await markRead(null, 10);
        expect(globalThis.fetch).toHaveBeenCalledWith(
            '/api/articles/10/read',
            expect.objectContaining({ method: 'POST' })
        );
        // Verify DOM mutation: card gets .read class
        const card = document.querySelector('.article-card[data-id="10"]');
        expect(card.classList.contains('read')).toBe(true);
        // Verify updateReadButton called with (card, true)
        expect(updateReadButton).toHaveBeenCalledWith(card, true);
        // Verify updateCounts called
        expect(updateCounts).toHaveBeenCalled();
    });

    it('handles API failure with console.error and showToast', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: false, status: 500,
            text: () => Promise.resolve('Internal Server Error'),
        }));
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="10"></div>';
        await markRead(null, 10);
        expect(console.error).toHaveBeenCalledWith('Failed to mark read:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to mark as read');
    });
});

describe('markUnread', () => {
    it('calls API, removes read class, and calls updateReadButton', async () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card read" data-id="10"><button class="btn-read-toggle"></button></div>';
        await markUnread(null, 10);
        expect(globalThis.fetch).toHaveBeenCalledWith(
            '/api/articles/10/unread',
            expect.objectContaining({ method: 'POST' })
        );
        const card = document.querySelector('.article-card');
        expect(card.classList.contains('read')).toBe(false);
        expect(updateReadButton).toHaveBeenCalledWith(card, false);
        expect(updateCounts).toHaveBeenCalled();
    });

    it('handles API failure: card retains .read class, shows error', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: false, status: 500,
            text: () => Promise.resolve('Internal Server Error'),
        }));
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card read" data-id="10"><button class="btn-read-toggle"></button></div>';
        await markUnread(null, 10);
        // Card should still have .read class (API failed, catch block ran)
        expect(document.querySelector('.article-card').classList.contains('read')).toBe(true);
        expect(console.error).toHaveBeenCalledWith('Failed to mark unread:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to mark as unread');
    });
});

describe('toggleStar', () => {
    it('calls API and toggles star button to starred state', async () => {
        // Set up an unstarred button
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-star" data-article-id="10"
                    title="Star" aria-label="Star">${SVG_STAR_EMPTY}</button>
            </div>
        `;
        await toggleStar(null, 10);
        expect(globalThis.fetch).toHaveBeenCalledWith(
            '/api/articles/10/star',
            expect.objectContaining({ method: 'POST' })
        );
        const btn = document.querySelector('[data-action="toggle-star"]');
        expect(btn.classList.contains('starred')).toBe(true);
        expect(btn.title).toBe('Unstar');
        expect(btn.getAttribute('aria-label')).toBe('Unstar');
        // innerHTML may normalize self-closing tags; check key path data
        expect(btn.innerHTML).toContain('M12 17.27L18.18 21l-1.64-7.03');
        expect(btn.innerHTML).not.toContain('M22 9.24l-7.19-.62L12 2');
        expect(updateCounts).toHaveBeenCalled();
    });

    it('toggles star button back to unstarred state', async () => {
        // Set up an already-starred button
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-star" data-article-id="10"
                    class="starred" title="Unstar" aria-label="Unstar">${SVG_STAR_FILLED}</button>
            </div>
        `;
        await toggleStar(null, 10);
        const btn = document.querySelector('[data-action="toggle-star"]');
        expect(btn.classList.contains('starred')).toBe(false);
        expect(btn.title).toBe('Star');
        expect(btn.getAttribute('aria-label')).toBe('Star');
        // SVG_STAR_EMPTY has the outline path with zM subpath
        expect(btn.innerHTML).toContain('M22 9.24l-7.19-.62L12 2');
    });

    it('handles API failure with console.error and showToast', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: false, status: 500,
            text: () => Promise.resolve('Internal Server Error'),
        }));
        await toggleStar(null, 10);
        expect(console.error).toHaveBeenCalledWith('Failed to toggle star:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to toggle star');
    });
});

describe('toggleQueue', () => {
    it('calls API and toggles queue button to queued state', async () => {
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ queued: true }),
        }));
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-queue" data-article-id="10"
                    title="Add to queue" aria-label="Add to queue">${SVG_QUEUE_ADD}</button>
            </div>
        `;
        await toggleQueue(null, 10);
        expect(globalThis.fetch).toHaveBeenCalledWith(
            '/api/articles/10/queue',
            expect.objectContaining({ method: 'POST' })
        );
        const btn = document.querySelector('[data-action="toggle-queue"]');
        expect(btn.classList.contains('queued')).toBe(true);
        expect(btn.title).toBe('Remove from queue');
        expect(btn.getAttribute('aria-label')).toBe('Remove from queue');
        // SVG_QUEUE_REMOVE has the minus-sign path (H10V9h8v2z)
        expect(btn.innerHTML).toContain('zm-2-5H10V9h8v2z');
        // queuedArticleIds should contain the id
        expect(queuedArticleIds.has(10)).toBe(true);
        expect(updateCounts).toHaveBeenCalled();
        expect(updateQueueCacheIfStandalone).toHaveBeenCalled();
    });

    it('toggles queue button to unqueued state and removes from queuedArticleIds', async () => {
        // Pre-populate queuedArticleIds
        queuedArticleIds.add(10);
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ queued: false }),
        }));
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-queue" data-article-id="10"
                    class="queued" title="Remove from queue" aria-label="Remove from queue">${SVG_QUEUE_REMOVE}</button>
            </div>
        `;
        await toggleQueue(null, 10);
        const btn = document.querySelector('[data-action="toggle-queue"]');
        expect(btn.classList.contains('queued')).toBe(false);
        expect(btn.title).toBe('Add to queue');
        expect(btn.getAttribute('aria-label')).toBe('Add to queue');
        // SVG_QUEUE_ADD has the plus-sign path (h2v-3h3V9h-3V6)
        expect(btn.innerHTML).toContain('zm-7-2h2v-3h3V9h-3V6');
        expect(queuedArticleIds.has(10)).toBe(false);
    });

    it('handles API failure: queuedArticleIds unchanged, shows error', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: false, status: 500,
            text: () => Promise.resolve('Internal Server Error'),
        }));
        // Pre-populate to verify it stays unchanged
        queuedArticleIds.add(5);
        await toggleQueue(null, 10);
        expect(queuedArticleIds.has(10)).toBe(false);
        expect(queuedArticleIds.has(5)).toBe(true);
        expect(console.error).toHaveBeenCalledWith('Failed to toggle queue:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to update queue');
    });
});

describe('findNextUnreadFolder', () => {
    it('returns URL of next folder with unread count', () => {
        document.body.innerHTML = `
            <div class="folder-item" data-category-id="1"></div>
            <span data-count="category-1">0</span>
            <div class="folder-item" data-category-id="2"></div>
            <span data-count="category-2">5</span>
            <div class="folder-item" data-category-id="3"></div>
            <span data-count="category-3">0</span>
        `;
        expect(findNextUnreadFolder('1')).toBe('/category/2');
    });

    it('returns null when no unread folders exist', () => {
        document.body.innerHTML = `
            <div class="folder-item" data-category-id="1"></div>
            <span data-count="category-1">0</span>
        `;
        expect(findNextUnreadFolder('1')).toBeNull();
    });

    it('wraps around to find folders before current', () => {
        document.body.innerHTML = `
            <div class="folder-item" data-category-id="1"></div>
            <span data-count="category-1">3</span>
            <div class="folder-item" data-category-id="2"></div>
            <span data-count="category-2">0</span>
        `;
        expect(findNextUnreadFolder('2')).toBe('/category/1');
    });
});

describe('auto-mark-read after client-side navigation (integration)', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        window.__settings = { autoMarkRead: 'true' };
        vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.resolve({
            ok: true, json: () => Promise.resolve({ status: 'ok' }),
        }));
        window.scrollTo = vi.fn();
    });

    afterEach(() => {
        delete window.scrollTo;
    });

    it('observer works on initial page load articles', () => {
        document.getElementById('articles-list').innerHTML = `
            <article class="article-card" data-id="1"></article>
            <article class="article-card" data-id="2"></article>
        `;

        const observeSpy = vi.spyOn(MockIntersectionObserver.prototype, 'observe');
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(observeSpy).toHaveBeenCalledTimes(2);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 initial articles');
        observeSpy.mockRestore();
    });

    it('observer is re-created after renderArticles (client-side nav)', async () => {
        document.getElementById('articles-list').innerHTML =
            '<article class="article-card" data-id="1"></article>';
        initAutoMarkRead();
        const initialObserver = _getAutoMarkReadObserver();

        await renderArticles([
            { id: 100, title: 'VR Article', is_read: 0, is_starred: 0, feed_name: 'VR Feed' },
            { id: 101, title: 'VR News', is_read: 0, is_starred: 0, feed_name: 'VR Feed' },
        ]);

        expect(_getAutoMarkReadObserver()).not.toBe(initialObserver);
        expect(_getAutoMarkReadObserver()).not.toBeNull();

        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(2);
        expect(cards[0].dataset.id).toBe('100');
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 initial articles');
    });

    it('new paginated articles are observed', () => {
        initAutoMarkRead();
        const spy = vi.spyOn(_getAutoMarkReadObserver(), 'observe');

        const temp = document.createElement('div');
        temp.innerHTML = `
            <article class="article-card" data-id="50"></article>
            <article class="article-card" data-id="51"></article>
            <article class="article-card" data-id="52"></article>
        `;
        observeNewArticles(temp);

        expect(spy).toHaveBeenCalledTimes(3);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 3 new articles');
    });

    it('multiple navigations each get a fresh observer', async () => {
        const observers = [];

        for (let i = 0; i < 3; i++) {
            await renderArticles([
                { id: i * 10 + 1, title: `Article ${i}`, is_read: 0, is_starred: 0 },
            ]);
            observers.push(_getAutoMarkReadObserver());
        }

        expect(observers[0]).not.toBe(observers[1]);
        expect(observers[1]).not.toBe(observers[2]);
        observers.forEach(obs => expect(obs).not.toBeNull());
        expect(console.debug).toHaveBeenCalledTimes(3);
    });
});

describe('markAsRead', () => {
    let reloadFn;

    beforeEach(() => {
        reloadFn = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { href: '', reload: reloadFn },
            writable: true,
            configurable: true,
        });
        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: true,
            json: async () => ({}),
            text: async () => '',
        }));
    });

    function makeDropdown(attrs = {}) {
        const dropdown = document.createElement('div');
        dropdown.className = 'dropdown';
        Object.entries(attrs).forEach(([k, v]) => { dropdown.dataset[k] = v; });
        const btn = document.createElement('button');
        dropdown.appendChild(btn);
        document.body.appendChild(dropdown);
        return btn;
    }

    it('posts to category URL and navigates to next unread folder', async () => {
        const btn = makeDropdown({ categoryId: '3' });

        // Add folder data so findNextUnreadFolder can work
        document.body.innerHTML += `
            <div class="folder-item" data-category-id="3"></div>
            <span data-count="category-3">0</span>
            <div class="folder-item" data-category-id="9"></div>
            <span data-count="category-9">5</span>
        `;

        await markAsRead(btn, 'week');

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/categories/3/read-all?age=week', expect.any(Object));
        expect(window.location.href).toBe('/category/9');
    });

    it('posts to feed URL and reloads when dropdown has data-feed-id', async () => {
        const btn = makeDropdown({ feedId: '7' });

        await markAsRead(btn, 'day');

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/feeds/7/read-all?age=day', expect.any(Object));
        expect(reloadFn).toHaveBeenCalled();
    });

    it('posts to global URL and reloads when dropdown has neither feed nor category', async () => {
        const btn = makeDropdown({});

        await markAsRead(btn, 'all');

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/articles/read-all?age=all', expect.any(Object));
        expect(reloadFn).toHaveBeenCalled();
    });

    it('reloads when category path but findNextUnreadFolder returns null', async () => {
        const btn = makeDropdown({ categoryId: '1' });

        // Only one folder, no unread others
        document.body.innerHTML += `
            <div class="folder-item" data-category-id="1"></div>
            <span data-count="category-1">0</span>
        `;

        await markAsRead(btn, 'all');

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/categories/1/read-all?age=all', expect.any(Object));
        // Should fall through to reload since no next unread folder
        expect(reloadFn).toHaveBeenCalled();
    });

    it('handles API failure with console.error and showToast', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {});
        vi.spyOn(globalThis, 'fetch').mockImplementation(async () => ({
            ok: false,
            status: 500,
            text: async () => 'Internal Server Error',
        }));
        const btn = makeDropdown({ feedId: '2' });

        await markAsRead(btn, 'all');

        expect(console.error).toHaveBeenCalledWith('Failed to mark as read:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to mark as read');
        // Should NOT reload on error
        expect(reloadFn).not.toHaveBeenCalled();
    });

    it('removes .open class from open dropdowns after success', async () => {
        const btn = makeDropdown({ feedId: '5' });

        // Create some open dropdowns
        const openDd1 = document.createElement('div');
        openDd1.className = 'dropdown open';
        const openDd2 = document.createElement('div');
        openDd2.className = 'dropdown open';
        document.body.appendChild(openDd1);
        document.body.appendChild(openDd2);

        await markAsRead(btn, 'all');

        expect(openDd1.classList.contains('open')).toBe(false);
        expect(openDd2.classList.contains('open')).toBe(false);
    });
});

describe('initArticleActionListeners', () => {
    beforeEach(() => {
        _resetArticleActionsState();
    });

    it('delegates mark-as-read clicks to markAsRead', async () => {
        document.body.innerHTML = `
            <div class="dropdown" data-feed-id="5">
                <button data-action="mark-as-read" data-scope="day">Older than one day</button>
            </div>
        `;
        initArticleActionListeners();
        // Mock fetch — markAsRead calls api() then location.reload()
        const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true, json: () => Promise.resolve({}) });
        // Stub location to prevent jsdom navigation issues
        delete window.location;
        window.location = { reload: vi.fn(), href: '/' };

        document.querySelector('[data-action="mark-as-read"]').click();
        // markAsRead is async — wait for the promise chain
        await vi.waitFor(() => {
            expect(fetchMock).toHaveBeenCalledWith('/api/feeds/5/read-all?age=day', expect.any(Object));
        });
    });

    it('opens article on article-body click', () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        document.body.innerHTML = `
            <article class="article-card" data-id="42">
                <div class="article-body clickable">
                    <p>Some content</p>
                </div>
            </article>
        `;
        initArticleActionListeners();
        // markReadSilent (called by openArticle) needs the fetch mock
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true, json: () => Promise.resolve({}) });
        // openArticle sets window.location, mock it
        const origLocation = window.location;
        delete window.location;
        window.location = '';

        document.querySelector('.article-body.clickable p').click();
        expect(window.location).toBe('/article/42');
        expect(console.debug).toHaveBeenCalledWith(
            expect.stringContaining('flushing batch'), expect.any(Array)
        );

        window.location = origLocation;
    });

    it('does not open article when clicking a link inside article-body', () => {
        document.body.innerHTML = `
            <article class="article-card" data-id="42">
                <div class="article-body clickable">
                    <a href="/feed/1" class="feed-name">Feed</a>
                </div>
            </article>
        `;
        initArticleActionListeners();
        const origLocation = window.location.href;

        // Click the link — should NOT trigger openArticle
        const link = document.querySelector('.feed-name');
        link.click();
        // location should not have changed to /article/42
        // (it might navigate the link itself, but we check it didn't call openArticle)
        expect(window.location.href).not.toBe('/article/42');
    });

    it('handles open-external action on title links', () => {
        document.body.innerHTML = `
            <article class="article-card" data-id="7">
                <div class="article-body clickable">
                    <h2 class="article-title">
                        <a href="https://example.com/post" data-action="open-external">Title</a>
                    </h2>
                </div>
            </article>
        `;
        initArticleActionListeners();
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true, json: () => Promise.resolve({}) });
        const openSpy = vi.fn();
        vi.stubGlobal('open', openSpy);

        document.querySelector('[data-action="open-external"]').click();
        expect(openSpy).toHaveBeenCalledWith('https://example.com/post', '_blank');
        vi.unstubAllGlobals();
    });

    it('handles mark-read-silent action on title links', () => {
        document.body.innerHTML = `
            <article class="article-card" data-id="9">
                <div class="article-body clickable">
                    <h2 class="article-title">
                        <a href="/article/9" data-action="mark-read-silent">Title</a>
                    </h2>
                </div>
            </article>
        `;
        initArticleActionListeners();

        document.querySelector('[data-action="mark-read-silent"]').click();
        // markReadSilent queues the id for batch flushing and marks the card as read in the DOM
        const card = document.querySelector('.article-card[data-id="9"]');
        expect(card.classList.contains('read')).toBe(true);
        // The id should be in the mark-read queue
        expect(_getMarkReadQueue()).toContain(9);
    });

    it('stops propagation and marks read on content-preview click', () => {
        document.body.innerHTML = `
            <article class="article-card" data-id="15">
                <div class="article-body clickable">
                    <div class="article-content-preview expanded-only">Preview text</div>
                </div>
            </article>
        `;
        initArticleActionListeners();

        document.querySelector('.article-content-preview').click();
        // markReadSilent queues the id and marks card as read
        const card = document.querySelector('.article-card[data-id="15"]');
        expect(card.classList.contains('read')).toBe(true);
        expect(_getMarkReadQueue()).toContain(15);
    });

    it('toggle-read with data-is-read="0" dispatches markRead', async () => {
        document.body.innerHTML = `
            <div id="articles-list">
                <div class="article-card" data-id="20">
                    <button data-action="toggle-read" data-article-id="20" data-is-read="0">Read</button>
                </div>
            </div>
        `;
        initArticleActionListeners();

        document.querySelector('[data-action="toggle-read"]').click();
        await vi.advanceTimersByTimeAsync(0);

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/articles/20/read', expect.objectContaining({ method: 'POST' }));
        const card = document.querySelector('.article-card[data-id="20"]');
        expect(card.classList.contains('read')).toBe(true);
        expect(updateReadButton).toHaveBeenCalledWith(card, true);
        expect(updateCounts).toHaveBeenCalled();
    });

    it('toggle-read with data-is-read="1" dispatches markUnread', async () => {
        document.body.innerHTML = `
            <div id="articles-list">
                <div class="article-card read" data-id="21">
                    <button data-action="toggle-read" data-article-id="21" data-is-read="1">Unread</button>
                </div>
            </div>
        `;
        initArticleActionListeners();

        document.querySelector('[data-action="toggle-read"]').click();
        await vi.advanceTimersByTimeAsync(0);

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/articles/21/unread', expect.objectContaining({ method: 'POST' }));
        const card = document.querySelector('.article-card[data-id="21"]');
        expect(card.classList.contains('read')).toBe(false);
        expect(updateReadButton).toHaveBeenCalledWith(card, false);
        expect(updateCounts).toHaveBeenCalled();
    });

    it('toggle-star dispatches toggleStar via delegation', async () => {
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-star" data-article-id="22"
                    title="Star" aria-label="Star">${SVG_STAR_EMPTY}</button>
            </div>
        `;
        initArticleActionListeners();

        document.querySelector('[data-action="toggle-star"]').click();
        await vi.advanceTimersByTimeAsync(0);

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/articles/22/star', expect.objectContaining({ method: 'POST' }));
        const btn = document.querySelector('[data-action="toggle-star"]');
        expect(btn.classList.contains('starred')).toBe(true);
        expect(btn.getAttribute('aria-label')).toBe('Unstar');
        expect(updateCounts).toHaveBeenCalled();
    });

    it('toggle-queue dispatches toggleQueue via delegation', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ queued: true }),
        });
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-queue" data-article-id="23"
                    title="Add to queue" aria-label="Add to queue">${SVG_QUEUE_ADD}</button>
            </div>
        `;
        initArticleActionListeners();

        document.querySelector('[data-action="toggle-queue"]').click();
        await vi.advanceTimersByTimeAsync(0);

        expect(globalThis.fetch).toHaveBeenCalledWith('/api/articles/23/queue', expect.objectContaining({ method: 'POST' }));
        const btn = document.querySelector('[data-action="toggle-queue"]');
        expect(btn.classList.contains('queued')).toBe(true);
        expect(btn.getAttribute('aria-label')).toBe('Remove from queue');
        expect(queuedArticleIds.has(23)).toBe(true);
        expect(updateCounts).toHaveBeenCalled();
        expect(updateQueueCacheIfStandalone).toHaveBeenCalled();
    });

    it('toggle-read ignores button without data-article-id', async () => {
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="toggle-read" data-is-read="0">Read</button>
            </div>
        `;
        initArticleActionListeners();

        document.querySelector('[data-action="toggle-read"]').click();
        await vi.advanceTimersByTimeAsync(0);

        expect(globalThis.fetch).not.toHaveBeenCalled();
    });
});

describe('initQueueState', () => {
    it('fetches queue and populates queuedArticleIds', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve([{ id: 10 }, { id: 20 }, { id: 30 }]),
        });
        const renderFn = vi.fn().mockReturnValue('<span>actions</span>');

        initQueueState(renderFn);
        // Flush the microtask queue so the promise chain settles
        await vi.advanceTimersByTimeAsync(0);

        expect(queuedArticleIds.has(10)).toBe(true);
        expect(queuedArticleIds.has(20)).toBe(true);
        expect(queuedArticleIds.has(30)).toBe(true);
    });

    it('hydrates action-button placeholders after queue loads', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve([{ id: 5 }]),
        });
        document.body.innerHTML = `
            <div class="article-actions-placeholder"
                 data-article-id="5"
                 data-is-read="1"
                 data-is-starred="0"
                 data-is-queued="0"
                 data-url="https://example.com"></div>
        `;
        const renderFn = vi.fn().mockReturnValue('<span class="rendered">actions</span>');

        initQueueState(renderFn);
        await vi.advanceTimersByTimeAsync(0);

        expect(renderFn).toHaveBeenCalledWith(expect.objectContaining({
            id: 5,
            is_read: true,
            is_starred: false,
            is_queued: true, // in queuedArticleIds
            url: 'https://example.com',
        }));
        expect(document.querySelector('.rendered')).not.toBeNull();
    });

    it('handles API failure gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            status: 500,
            text: () => Promise.resolve('Internal Server Error'),
        });
        const renderFn = vi.fn();

        initQueueState(renderFn);
        await vi.advanceTimersByTimeAsync(0);

        // Should not throw, renderFn should not be called since the catch swallows the error
        expect(renderFn).not.toHaveBeenCalled();
    });
});
