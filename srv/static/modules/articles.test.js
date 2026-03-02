import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    renderArticleActions, buildArticleCardHtml, updateReadButton,
    showArticlesLoading, updateAllReadMessage, showReadArticles,
    processEmbeds, extractYouTubeId, applyUserPreferences,
    getIncludeReadUrl, setShowingHiddenArticles,
    initArticleListListeners, showHiddenArticles, renderArticles,
    _resetArticlesState,
} from './articles.js';
import {
    _resetArticleActionsState, setQueuedArticleIds,
    initAutoMarkRead, _getAutoMarkReadObserver,
} from './article-actions.js';

// Mock pagination module (now directly imported by articles via circular import)
vi.mock('./pagination.js', () => ({
    updatePaginationCursor: vi.fn(),
    updateEndOfArticlesIndicator: vi.fn(),
    setPaginationState: vi.fn(),
}));

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
    vi.spyOn(console, 'debug').mockImplementation(() => {});
    _resetArticlesState();
    _resetArticleActionsState();
    setQueuedArticleIds(new Set());
    window.IntersectionObserver = MockIntersectionObserver;
    window.__settings = {};
    window.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }));
    document.body.innerHTML = '<div id="articles-list" class="articles-list"></div>';
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('renderArticleActions', () => {
    it('renders read/star/queue buttons for unread article', () => {
        const html = renderArticleActions({ id: 1, is_read: false, is_starred: false, url: 'https://example.com' });
        expect(html).toContain('data-action="toggle-read"');
        expect(html).toContain('data-is-read="0"');
        expect(html).toContain('data-action="toggle-star"');
        expect(html).toContain('data-action="toggle-queue"');
        expect(html).toContain('Open original');
    });

    it('renders markUnread button for read article', () => {
        const html = renderArticleActions({ id: 1, is_read: true, is_starred: false });
        expect(html).toContain('data-is-read="1"');
        expect(html).toContain('Mark unread');
    });

    it('omits external link when no url', () => {
        const html = renderArticleActions({ id: 1, is_read: false, is_starred: false });
        expect(html).not.toContain('Open original');
    });

    it('shows filled star when starred', () => {
        const html = renderArticleActions({ id: 1, is_read: false, is_starred: true });
        expect(html).toContain('starred');
    });
});

describe('updateReadButton', () => {
    it('switches button to markUnread when isRead=true', () => {
        document.body.innerHTML =
            '<div class="article-card" data-id="5"><button class="btn-read-toggle" data-is-read="0"></button></div>';
        const card = document.querySelector('.article-card');
        updateReadButton(card, true);
        expect(card.querySelector('.btn-read-toggle').dataset.isRead).toBe('1');
        expect(card.querySelector('.btn-read-toggle').title).toBe('Mark unread');
    });

    it('switches button to markRead when isRead=false', () => {
        document.body.innerHTML =
            '<div class="article-card" data-id="5"><button class="btn-read-toggle" data-is-read="1"></button></div>';
        const card = document.querySelector('.article-card');
        updateReadButton(card, false);
        expect(card.querySelector('.btn-read-toggle').dataset.isRead).toBe('0');
        expect(card.querySelector('.btn-read-toggle').title).toBe('Mark read');
    });

    it('does nothing for null card', () => {
        updateReadButton(null, true);  // Should not throw
    });
});

describe('showArticlesLoading', () => {
    it('sets loading HTML in articles list', () => {
        showArticlesLoading();
        expect(document.getElementById('articles-list').innerHTML).toContain('Loading articles');
    });
});

describe('updateAllReadMessage', () => {
    it('does nothing when hideReadArticles is not set', () => {
        window.__settings = {};
        updateAllReadMessage();
        expect(document.getElementById('all-read-message')).toBeNull();
    });

    it('shows all-caught-up message when all articles are hidden', () => {
        window.__settings = { hideReadArticles: 'hide' };
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card" style="display: none"></div>';
        updateAllReadMessage();
        expect(document.getElementById('all-read-message')).not.toBeNull();
        expect(document.getElementById('all-read-message').textContent).toContain('All caught up');
    });

    it('does not show message when visible cards exist', () => {
        window.__settings = { hideReadArticles: 'hide' };
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card"></div>';
        updateAllReadMessage();
        expect(document.getElementById('all-read-message')).toBeNull();
    });
});

describe('showReadArticles', () => {
    it('makes read articles visible and removes message', () => {
        const list = document.getElementById('articles-list');
        list.innerHTML =
            '<div class="article-card read" style="display: none"></div>' +
            '<div id="all-read-message"></div>';
        showReadArticles();
        expect(document.querySelector('.article-card').style.display).toBe('');
        expect(document.getElementById('all-read-message')).toBeNull();
    });
});

describe('buildArticleCardHtml', () => {
    it('renders an article card with title and date', () => {
        const html = buildArticleCardHtml({
            id: 1,
            title: 'Test Article',
            feed_id: 10,
            feed_name: 'Test Feed',
            published_at: '2025-01-01T00:00:00Z',
            is_read: false,
            is_starred: false,
        });
        expect(html).toContain('Test Article');
        expect(html).toContain('Test Feed');
        expect(html).toContain('data-id="1"');
    });

    it('adds read class for read articles', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: true, is_starred: false,
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-card read');
    });

    it('includes image when image_url is provided', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            image_url: 'https://example.com/img.jpg',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('img.jpg');
        expect(html).toContain('has-image');
    });
});

describe('extractYouTubeId', () => {
    it('extracts ID from youtube.com/watch URL', () => {
        expect(extractYouTubeId('https://www.youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
    });

    it('extracts ID from youtu.be URL', () => {
        expect(extractYouTubeId('https://youtu.be/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
    });

    it('extracts ID from embed URL', () => {
        expect(extractYouTubeId('https://youtube.com/embed/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
    });

    it('extracts ID from shorts URL', () => {
        expect(extractYouTubeId('https://youtube.com/shorts/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
    });

    it('returns null for non-YouTube URLs', () => {
        expect(extractYouTubeId('https://example.com/video')).toBeNull();
    });

    it('returns null for null/undefined', () => {
        expect(extractYouTubeId(null)).toBeNull();
        expect(extractYouTubeId(undefined)).toBeNull();
    });
});

describe('processEmbeds', () => {
    it('replaces video elements with YouTube iframes', () => {
        const container = document.createElement('div');
        container.innerHTML = '<video data-embed-type="video" data-src="https://youtube.com/watch?v=dQw4w9WgXcQ"></video>';
        processEmbeds(container);
        expect(container.querySelector('iframe')).not.toBeNull();
        expect(container.querySelector('iframe').src).toContain('youtube.com/embed/dQw4w9WgXcQ');
    });

    it('wraps existing YouTube iframes in embed-video div', () => {
        const container = document.createElement('div');
        container.innerHTML = '<iframe src="https://youtube.com/embed/abc" width="560" height="315"></iframe>';
        processEmbeds(container);
        expect(container.querySelector('.embed-video')).not.toBeNull();
    });

    it('does nothing for null container', () => {
        processEmbeds(null); // Should not throw
    });

    it('loads twitter widget script for tweet embeds', () => {
        const container = document.createElement('div');
        container.innerHTML = `
            <div data-embed-type="tweet">
                <blockquote class="twitter-tweet"></blockquote>
            </div>
        `;

        // Intercept appendChild to prevent happy-dom from attempting to
        // load the external Twitter script (which logs a NotSupportedError).
        const origAppendChild = document.body.appendChild.bind(document.body);
        let capturedScript = null;
        vi.spyOn(document.body, 'appendChild').mockImplementation((node) => {
            if (node.tagName === 'SCRIPT' && node.src?.includes('platform.twitter.com')) {
                capturedScript = node;
                return node;
            }
            return origAppendChild(node);
        });

        processEmbeds(container);

        expect(capturedScript).not.toBeNull();
        expect(capturedScript.src).toContain('platform.twitter.com/widgets.js');
        document.body.appendChild.mockRestore();
    });
});

describe('applyUserPreferences', () => {
    it('calls applyHideReadArticles with current setting', () => {
        window.__settings = { hideReadArticles: 'hide' };
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card read"></div>';
        applyUserPreferences();
        // Read article should be hidden
        expect(document.querySelector('.article-card').style.display).toBe('none');
    });
});

describe('getIncludeReadUrl', () => {
    it('returns unread URL for root path', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        expect(getIncludeReadUrl()).toBe('/api/articles/unread?include_read=1');
    });

    it('returns feed URL for feed path', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/feed/42' }, writable: true, configurable: true,
        });
        expect(getIncludeReadUrl()).toBe('/api/feeds/42/articles?include_read=1');
    });

    it('returns category URL for category path', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/category/7' }, writable: true, configurable: true,
        });
        expect(getIncludeReadUrl()).toBe('/api/categories/7/articles?include_read=1');
    });

    it('returns null for unknown paths', () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/settings' }, writable: true, configurable: true,
        });
        expect(getIncludeReadUrl()).toBeNull();
    });
});

describe('renderArticles', () => {
    beforeEach(() => {
        window.__settings = { autoMarkRead: 'true' };
        window.fetch = vi.fn(() => Promise.resolve({
            ok: true, json: () => Promise.resolve({}),
        }));
        window.scrollTo = vi.fn();
    });

    afterEach(() => {
        delete window.scrollTo;
    });

    it('renders articles into the list', async () => {
        const articles = [
            { id: 1, title: 'Article 1', is_read: 0, is_starred: 0 },
            { id: 2, title: 'Article 2', is_read: 0, is_starred: 0 },
        ];
        await renderArticles(articles);

        expect(window.scrollTo).toHaveBeenCalledWith(0, 0);
        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(2);
        expect(cards[0].dataset.id).toBe('1');
        expect(cards[1].dataset.id).toBe('2');
    });

    it('re-initializes the auto-mark-read observer', async () => {
        initAutoMarkRead();
        const firstObserver = _getAutoMarkReadObserver();

        await renderArticles([
            { id: 10, title: 'New', is_read: 0, is_starred: 0 },
        ]);

        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(_getAutoMarkReadObserver()).not.toBe(firstObserver);
    });

    it('handles empty article list', async () => {
        await renderArticles([]);
        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(0);
        expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
    });

    it('handles null articles', async () => {
        await renderArticles(null);
        expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
    });
});

describe('showHiddenArticles', () => {
    it('sets showingHiddenArticles and re-fetches articles', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/', hostname: 'localhost' },
            writable: true,
            configurable: true,
        });
        setShowingHiddenArticles(false);
        window.scrollTo = vi.fn();

        window.fetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                articles: [{
                    id: 1, title: 'Old', is_read: 1, feed_id: 1, guid: 'g1',
                    fetched_at: new Date().toISOString(),
                }],
            }),
        });

        await showHiddenArticles();

        const fetchUrl = window.fetch.mock.calls[0][0];
        expect(fetchUrl).toContain('include_read=1');
        const cards = document.querySelectorAll('.article-card');
        expect(cards.length).toBe(1);

        delete window.scrollTo;
    });
});

describe('initArticleListListeners', () => {
    beforeEach(() => {
        _resetArticlesState();
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ articles: [] }),
        }));
    });

    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('delegates show-hidden-articles clicks', async () => {
        // Set up a path so getIncludeReadUrl returns something
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' }, writable: true, configurable: true,
        });
        window.scrollTo = vi.fn();
        document.body.innerHTML = `
            <div id="articles-list">
                <button data-action="show-hidden-articles">Show hidden articles</button>
            </div>
        `;
        initArticleListListeners();

        document.querySelector('[data-action="show-hidden-articles"]').click();
        await new Promise(r => setTimeout(r, 10));

        expect(fetch).toHaveBeenCalledWith('/api/articles/unread?include_read=1', expect.any(Object));
    });

    it('delegates show-read-articles clicks', () => {
        document.body.innerHTML = `
            <div id="articles-list">
                <article class="article-card read" style="display: none;">Read article</article>
                <button data-action="show-read-articles">Show read articles</button>
            </div>
        `;
        initArticleListListeners();

        document.querySelector('[data-action="show-read-articles"]').click();
        // showReadArticles removes the display:none
        const card = document.querySelector('.article-card.read');
        expect(card.style.display).toBe('');
    });
});
