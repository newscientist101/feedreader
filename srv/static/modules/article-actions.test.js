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

// Mock pagination (articles.js directly imports from pagination.js)
vi.mock('./pagination.js', () => ({
    updatePaginationCursor: vi.fn(),
    updateEndOfArticlesIndicator: vi.fn(),
    setPaginationState: vi.fn(),
}));

// Mock counts and offline modules (now directly imported by article-actions)
vi.mock('./counts.js', () => ({
    updateCounts: vi.fn(),
}));

vi.mock('./offline.js', () => ({
    updateQueueCacheIfStandalone: vi.fn(),
}));

vi.mock('./read-button.js', () => ({
    updateReadButton: vi.fn(),
}));

// Minimal IntersectionObserver mock
class MockIntersectionObserver {
    constructor(callback) {
        this._callback = callback;
        this._entries = [];
    }
    observe(el) { this._entries.push(el); }
    unobserve() {}
    disconnect() { this._entries = []; }
    _fire(entries) { this._callback(entries, this); }
}

beforeEach(() => {
    vi.useFakeTimers();
    vi.spyOn(console, 'debug').mockImplementation(() => {});
    _resetArticleActionsState();
    _resetArticlesState();
    window.IntersectionObserver = MockIntersectionObserver;
    window.__settings = {};
    window.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }));
    document.body.innerHTML = '<div id="articles-list"></div>';
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
});

describe('initAutoMarkRead', () => {
    it('does nothing when autoMarkRead setting is not true', () => {
        window.__settings = { autoMarkRead: 'false' };
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).toBeNull();
    });

    it('creates an IntersectionObserver when autoMarkRead is true', () => {
        window.__settings = { autoMarkRead: 'true' };
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card" data-id="1"></div>';
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBeNull();
    });

    it('disconnects previous observer on re-init', () => {
        window.__settings = { autoMarkRead: 'true' };
        initAutoMarkRead();
        const first = _getAutoMarkReadObserver();
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBe(first);
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
        window.__settings = { autoMarkRead: 'true' };
        initAutoMarkRead();
        const obs = _getAutoMarkReadObserver();
        const spy = vi.spyOn(obs, 'observe');
        const container = document.createElement('div');
        container.innerHTML = '<div class="article-card"></div><div class="article-card"></div>';
        observeNewArticles(container);
        expect(spy).toHaveBeenCalledTimes(2);
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
        window.fetch = vi.fn(() => Promise.resolve({
            ok: true, json: () => Promise.resolve({ status: 'ok' }),
        }));
        markReadSilent(1);
        markReadSilent(2);
        expect(window.fetch).not.toHaveBeenCalled();

        vi.advanceTimersByTime(500);
        expect(window.fetch).toHaveBeenCalledTimes(1);

        const [url, opts] = window.fetch.mock.calls[0];
        expect(url).toBe('/api/articles/batch-read');
        const body = JSON.parse(opts.body);
        expect(body.ids).toEqual([1, 2]);
    });

    it('resets the timer when called rapidly', () => {
        window.fetch = vi.fn(() => Promise.resolve({
            ok: true, json: () => Promise.resolve({ status: 'ok' }),
        }));
        markReadSilent(1);
        vi.advanceTimersByTime(150);
        markReadSilent(2);
        vi.advanceTimersByTime(150);
        // Only 300ms total, but timer was reset at 150ms so another 100ms to go
        expect(window.fetch).not.toHaveBeenCalled();
        vi.advanceTimersByTime(100);
        expect(window.fetch).toHaveBeenCalledTimes(1);
        const body = JSON.parse(window.fetch.mock.calls[0][1].body);
        expect(body.ids).toEqual([1, 2]);
    });
});

describe('flushMarkReadQueue', () => {
    it('posts queued ids as a batch', () => {
        markReadSilent(1);
        markReadSilent(2);
        flushMarkReadQueue();
        expect(window.fetch).toHaveBeenCalledWith(
            '/api/articles/batch-read',
            expect.objectContaining({
                method: 'POST',
                body: expect.stringContaining('"ids"'),
            })
        );
    });

    it('does nothing when queue is empty', () => {
        flushMarkReadQueue();
        expect(window.fetch).not.toHaveBeenCalled();
    });
});

describe('openArticle', () => {
    it('marks the article as read and navigates', () => {
        const assignSpy = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { pathname: '/', hostname: 'localhost' },
            writable: true,
            configurable: true,
        });
        markReadSilent(5);
        openArticle(5);
        // flushMarkReadQueue was called (fetch happened)
        expect(window.fetch).toHaveBeenCalled();
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
            '<div class="article-card" data-id="10"></div>';
        await markRead(null, 10);
        expect(window.fetch).toHaveBeenCalledWith(
            '/api/articles/10/read',
            expect.objectContaining({ method: 'POST' })
        );
    });
});

describe('markUnread', () => {
    it('calls API and removes read class', async () => {
        document.getElementById('articles-list').innerHTML =
            '<div class="article-card read" data-id="10"><button class="btn-read-toggle"></button></div>';
        await markUnread(null, 10);
        expect(window.fetch).toHaveBeenCalledWith(
            '/api/articles/10/unread',
            expect.objectContaining({ method: 'POST' })
        );
        expect(document.querySelector('.article-card').classList.contains('read')).toBe(false);
    });
});

describe('toggleStar', () => {
    it('calls API for star toggle', async () => {
        await toggleStar(null, 10);
        expect(window.fetch).toHaveBeenCalledWith(
            '/api/articles/10/star',
            expect.objectContaining({ method: 'POST' })
        );
    });
});

describe('toggleQueue', () => {
    it('calls API for queue toggle', async () => {
        window.fetch = vi.fn(() => Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ queued: true }),
        }));
        await toggleQueue(null, 10);
        expect(window.fetch).toHaveBeenCalledWith(
            '/api/articles/10/queue',
            expect.objectContaining({ method: 'POST' })
        );
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
        window.__settings = { autoMarkRead: 'true' };
        window.fetch = vi.fn(() => Promise.resolve({
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
    });
});

describe('markAsRead with category URL', () => {
    it('posts to category URL and navigates to next unread folder', async () => {
        const dropdown = document.createElement('div');
        dropdown.className = 'dropdown';
        dropdown.dataset.categoryId = '3';
        const btn = document.createElement('button');
        dropdown.appendChild(btn);
        document.body.appendChild(dropdown);

        window.fetch = vi.fn(async () => ({
            ok: true,
            json: async () => ({}),
            text: async () => '',
        }));

        Object.defineProperty(window, 'location', {
            value: { href: '', reload: vi.fn() }, writable: true, configurable: true,
        });

        // Add folder data so findNextUnreadFolder can work
        document.body.innerHTML += `
            <div class="folder-item" data-category-id="3"></div>
            <span data-count="category-3">0</span>
            <div class="folder-item" data-category-id="9"></div>
            <span data-count="category-9">5</span>
        `;

        await markAsRead(btn, 'week');

        expect(window.fetch).toHaveBeenCalledWith('/api/categories/3/read-all?age=week', expect.any(Object));
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
        const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) });
        vi.stubGlobal('fetch', fetchMock);
        // Stub location to prevent jsdom navigation issues
        delete window.location;
        window.location = { reload: vi.fn(), href: '/' };

        document.querySelector('[data-action="mark-as-read"]').click();
        // markAsRead is async — wait for the promise chain
        await vi.waitFor(() => {
            expect(fetchMock).toHaveBeenCalledWith('/api/feeds/5/read-all?age=day', expect.any(Object));
        });

        vi.unstubAllGlobals();
    });

    it('opens article on article-body click', () => {
        document.body.innerHTML = `
            <article class="article-card" data-id="42">
                <div class="article-body clickable">
                    <p>Some content</p>
                </div>
            </article>
        `;
        initArticleActionListeners();
        // markReadSilent (called by openArticle) needs the fetch mock
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }));
        // openArticle sets window.location, mock it
        const origLocation = window.location;
        delete window.location;
        window.location = '';

        document.querySelector('.article-body.clickable p').click();
        expect(window.location).toBe('/article/42');

        window.location = origLocation;
        vi.unstubAllGlobals();
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
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }));
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
});

describe('initQueueState', () => {
    beforeEach(() => {
        vi.useRealTimers();
    });

    afterEach(() => {
        vi.useFakeTimers();
    });

    it('fetches queue and populates queuedArticleIds', async () => {
        window.fetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve([{ id: 10 }, { id: 20 }, { id: 30 }]),
        });
        const renderFn = vi.fn().mockReturnValue('<span>actions</span>');

        initQueueState(renderFn);
        // Wait for the internal promise chain to settle
        await new Promise(r => setTimeout(r, 50));

        expect(queuedArticleIds.has(10)).toBe(true);
        expect(queuedArticleIds.has(20)).toBe(true);
        expect(queuedArticleIds.has(30)).toBe(true);
    });

    it('hydrates action-button placeholders after queue loads', async () => {
        window.fetch = vi.fn().mockResolvedValue({
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
        await new Promise(r => setTimeout(r, 50));

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
        window.fetch = vi.fn().mockResolvedValue({
            ok: false,
            status: 500,
            text: () => Promise.resolve('Internal Server Error'),
        });
        const renderFn = vi.fn();

        initQueueState(renderFn);
        await new Promise(r => setTimeout(r, 50));

        // Should not throw, renderFn should not be called since the catch swallows the error
        expect(renderFn).not.toHaveBeenCalled();
    });
});
