import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    renderArticleActions, buildArticleCardHtml, updateReadButton,
    showArticlesLoading, updateAllReadMessage, showReadArticles,
    processEmbeds, extractYouTubeId, applyUserPreferences,
    getIncludeReadUrl, setShowingHiddenArticles, getShowingHiddenArticles,
    initArticleListListeners, showHiddenArticles, renderArticles,
    _resetArticlesState,
} from './articles.js';
import {
    _resetArticleActionsState, setQueuedArticleIds,
    initAutoMarkRead, _getAutoMarkReadObserver,
} from './article-actions.js';

// Mock pagination module (now directly imported by articles via circular import)
vi.mock('./pagination.js');

vi.mock('./toast.js');

import { showToast } from './toast.js';
import { setPaginationState, updateEndOfArticlesIndicator } from './pagination.js';

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

    it('shows queued state when is_queued is true', () => {
        const html = renderArticleActions({ id: 1, is_read: false, is_starred: false, is_queued: true });
        expect(html).toContain('queued');
        expect(html).toContain('Remove from queue');
    });

    it('shows add-to-queue state when is_queued is false', () => {
        const html = renderArticleActions({ id: 1, is_read: false, is_starred: false, is_queued: false });
        expect(html).toContain('Add to queue');
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
        const list = document.getElementById('articles-list');
        expect(list.innerHTML).toContain('Loading articles');
        expect(list.getAttribute('aria-busy')).toBe('true');
    });

    it('resets showingHiddenArticles flag', () => {
        setShowingHiddenArticles(true);
        showArticlesLoading();
        expect(getShowingHiddenArticles()).toBe(false);
    });

    it('does nothing when articles-list is absent', () => {
        document.body.innerHTML = '';
        expect(() => showArticlesLoading()).not.toThrow();
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

    it('returns early when articles-list is absent', () => {
        document.body.innerHTML = '';
        expect(() => updateAllReadMessage()).not.toThrow();
        expect(document.getElementById('all-read-message')).toBeNull();
    });

    it('removes existing message before re-evaluating', () => {
        window.__settings = { hideReadArticles: 'hide' };
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card" style="display: none"></div>';
        updateAllReadMessage();
        expect(document.getElementById('all-read-message')).not.toBeNull();
        // Call again — should remove old and add new, not duplicate
        updateAllReadMessage();
        expect(document.querySelectorAll('#all-read-message').length).toBe(1);
    });

    it('does not show message when showingHiddenArticles is true', () => {
        window.__settings = { hideReadArticles: 'hide' };
        setShowingHiddenArticles(true);
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card" style="display: none"></div>';
        updateAllReadMessage();
        expect(document.getElementById('all-read-message')).toBeNull();
    });

    it('shows singular article count', () => {
        window.__settings = { hideReadArticles: 'hide' };
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card" style="display: none"></div>';
        updateAllReadMessage();
        const msg = document.getElementById('all-read-message');
        expect(msg.textContent).toContain('1 article');
        expect(msg.textContent).not.toContain('1 articles');
    });

    it('shows plural article count for multiple articles', () => {
        window.__settings = { hideReadArticles: 'hide' };
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card" style="display: none"></div>' +
            '<div class="article-card" style="display: none"></div>';
        updateAllReadMessage();
        const msg = document.getElementById('all-read-message');
        expect(msg.textContent).toContain('2 articles');
    });

    it('does not show message when no articles exist', () => {
        window.__settings = { hideReadArticles: 'hide' };
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

    it('escapes HTML in user-controlled fields to prevent XSS', () => {
        const html = buildArticleCardHtml({
            id: 1,
            title: '<img onerror="alert(1)">',
            feed_id: 1,
            feed_name: '<script>alert(2)</script>',
            author: '" onclick="alert(3)',
            url: 'https://example.com/a?b=1&c=2',
            published_at: '2025-01-01T00:00:00Z',
            is_read: false,
            is_starred: false,
        });
        // Title should be escaped
        expect(html).toContain('&lt;img onerror=&quot;alert(1)&quot;&gt;');
        expect(html).not.toContain('<img onerror=');
        // Feed name should be escaped
        expect(html).toContain('&lt;script&gt;alert(2)&lt;/script&gt;');
        expect(html).not.toContain('<script>alert(2)');
        // Author should be escaped
        expect(html).toContain('&quot; onclick=&quot;alert(3)');
        // URL ampersand should be escaped in attribute
        expect(html).toContain('https://example.com/a?b=1&amp;c=2');
    });

    it('renders mark-read-silent link when article has no url', () => {
        const html = buildArticleCardHtml({
            id: 42, title: 'No URL', feed_id: 1, is_read: false, is_starred: false,
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('href="/article/42"');
        expect(html).toContain('data-action="mark-read-silent"');
        expect(html).not.toContain('target="_blank"');
    });

    it('renders external link when article has url', () => {
        const html = buildArticleCardHtml({
            id: 42, title: 'Has URL', feed_id: 1, is_read: false, is_starred: false,
            url: 'https://example.com/article',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('target="_blank"');
        expect(html).toContain('data-action="open-external"');
    });

    it('renders author when present', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            author: 'John Doe',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-author');
        expect(html).toContain('John Doe');
    });

    it('omits author when not present', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            author: '',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).not.toContain('article-author');
    });

    it('shows summary as preview when summary exists', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            summary: 'This is the summary text',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-summary');
        expect(html).toContain('This is the summary text');
    });

    it('falls back to content for preview when no summary', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            summary: '',
            content: 'This is the content text',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-summary');
        expect(html).toContain('This is the content text');
    });

    it('renders expanded content preview from content', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            summary: 'Summary',
            content: 'Content preview',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-content-preview expanded-only');
        expect(html).toContain('Content preview');
    });

    it('falls back to summary for expanded preview when no content', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            summary: 'Summary for expanded',
            content: '',
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-content-preview expanded-only');
        expect(html).toContain('Summary for expanded');
    });

    it('omits preview sections when neither summary nor content exist', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).not.toContain('article-summary');
        expect(html).not.toContain('article-content-preview');
    });

    it('shows placeholder image when no image_url', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            published_at: '2025-01-01T00:00:00Z',
        });
        expect(html).toContain('article-image-placeholder');
        expect(html).not.toContain('has-image');
    });

    it('uses fetched_at for sort-time when published_at is missing', () => {
        const html = buildArticleCardHtml({
            id: 1, title: 'T', feed_id: 1, is_read: false, is_starred: false,
            fetched_at: '2025-06-15T12:00:00Z',
        });
        expect(html).toContain('data-sort-time="2025-06-15T12:00:00Z"');
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

    it('returns null for empty string', () => {
        expect(extractYouTubeId('')).toBeNull();
    });

    it('extracts ID from URL with extra query parameters', () => {
        expect(extractYouTubeId('https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120')).toBe('dQw4w9WgXcQ');
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

    it('does not replace video element without valid YouTube ID', () => {
        const container = document.createElement('div');
        container.innerHTML = '<video data-embed-type="video" data-src="https://example.com/video.mp4"></video>';
        processEmbeds(container);
        expect(container.querySelector('video')).not.toBeNull();
        expect(container.querySelector('iframe')).toBeNull();
    });

    it('leaves tweet element unchanged when no blockquote child', () => {
        const container = document.createElement('div');
        container.innerHTML = '<div data-embed-type="tweet"><p>Not a blockquote</p></div>';

        // Intercept script load
        const origAppendChild = document.body.appendChild.bind(document.body);
        vi.spyOn(document.body, 'appendChild').mockImplementation((node) => {
            if (node.tagName === 'SCRIPT') return node;
            return origAppendChild(node);
        });

        processEmbeds(container);
        expect(container.querySelector('[data-embed-type="tweet"]')).not.toBeNull();
        document.body.appendChild.mockRestore();
    });

    it('does not double-wrap iframe already in embed-video', () => {
        const container = document.createElement('div');
        container.innerHTML = '<div class="embed-video"><iframe src="https://youtube.com/embed/abc"></iframe></div>';
        processEmbeds(container);
        // Should still be exactly one .embed-video wrapper
        expect(container.querySelectorAll('.embed-video').length).toBe(1);
    });

    it('calls twttr.widgets.load when Twitter script already exists', () => {
        // Stub querySelector to report Twitter script is already present,
        // avoiding a real <script> append that triggers happy-dom's loader.
        const origQS = document.querySelector.bind(document);
        vi.spyOn(document, 'querySelector').mockImplementation((sel) => {
            if (sel === 'script[src*="platform.twitter.com"]') {
                return document.createElement('script'); // truthy sentinel
            }
            return origQS(sel);
        });

        const loadFn = vi.fn();
        window.twttr = { widgets: { load: loadFn } };

        const container = document.createElement('div');
        container.innerHTML = '<div data-embed-type="tweet"><blockquote class="twitter-tweet"></blockquote></div>';
        processEmbeds(container);

        expect(loadFn).toHaveBeenCalledWith(container);
        delete window.twttr;
        document.querySelector.mockRestore();
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

    it('skips hiding read articles when showingHiddenArticles is true', () => {
        window.__settings = { hideReadArticles: 'hide' };
        setShowingHiddenArticles(true);
        const list = document.getElementById('articles-list');
        list.innerHTML = '<div class="article-card read"></div>';
        applyUserPreferences();
        // Read article should NOT be hidden
        expect(document.querySelector('.article-card').style.display).not.toBe('none');
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
        vi.spyOn(console, 'debug').mockImplementation(() => {});
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
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 initial articles');
    });

    it('re-initializes the auto-mark-read observer', async () => {
        initAutoMarkRead();
        const firstObserver = _getAutoMarkReadObserver();

        await renderArticles([
            { id: 10, title: 'New', is_read: 0, is_starred: 0 },
        ]);

        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(_getAutoMarkReadObserver()).not.toBe(firstObserver);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 1 initial articles');
    });

    it('handles empty article list', async () => {
        await renderArticles([]);
        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(0);
        expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
        expect(console.debug).not.toHaveBeenCalled();
    });

    it('handles null articles', async () => {
        await renderArticles(null);
        expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
        expect(console.debug).not.toHaveBeenCalled();
    });

    it('sets aria-busy to false after rendering articles', async () => {
        await renderArticles([{ id: 1, title: 'T', is_read: 0, is_starred: 0 }]);
        expect(document.getElementById('articles-list').getAttribute('aria-busy')).toBe('false');
    });

    it('sets aria-busy to false after rendering empty list', async () => {
        await renderArticles([]);
        expect(document.getElementById('articles-list').getAttribute('aria-busy')).toBe('false');
    });

    it('sets pagination done when articles < PAGE_SIZE', async () => {
        const articles = Array.from({ length: 10 }, (_, i) => ({
            id: i + 1, title: `Art ${i}`, is_read: 0, is_starred: 0,
        }));
        await renderArticles(articles);
        // setPaginationState should be called with done: true (less than 50)
        expect(setPaginationState).toHaveBeenCalledWith(expect.objectContaining({ done: true }));
    });

    it('does not set pagination done for articles at PAGE_SIZE boundary', async () => {
        const articles = Array.from({ length: 50 }, (_, i) => ({
            id: i + 1, title: `Art ${i}`, is_read: 0, is_starred: 0,
        }));
        setPaginationState.mockClear();
        await renderArticles(articles);
        // The first call resets pagination state (cursorTime, cursorId, done: false, loading: false)
        // With exactly 50 articles, the `if (articles.length < PAGE_SIZE)` branch
        // should NOT execute, so the only `done` value set should be false (from reset)
        const calls = setPaginationState.mock.calls;
        // First call is the full reset with done: false
        expect(calls[0][0]).toEqual({ cursorTime: null, cursorId: null, done: false, loading: false });
        // No subsequent call should set done: true as a standalone call
        // (the standalone {done: true} only fires for articles.length < PAGE_SIZE)
        const standalonedoneCalls = calls.slice(1).filter(
            call => Object.keys(call[0]).length === 1 && call[0].done === true
        );
        expect(standalonedoneCalls.length).toBe(0);
    });

    it('shows "Show hidden articles" button only when not showing hidden', async () => {
        setShowingHiddenArticles(false);
        await renderArticles([]);
        expect(document.querySelector('[data-action="show-hidden-articles"]')).not.toBeNull();
    });

    it('hides "Show hidden articles" button when showing hidden', async () => {
        setShowingHiddenArticles(true);
        await renderArticles([]);
        expect(document.querySelector('[data-action="show-hidden-articles"]')).toBeNull();
    });

    it('returns early when articles-list is absent', async () => {
        document.body.innerHTML = '';
        await renderArticles([{ id: 1, title: 'T', is_read: 0, is_starred: 0 }]);
        // Should not throw
    });
});

describe('showHiddenArticles', () => {
    it('sets showingHiddenArticles and re-fetches articles', async () => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
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
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] disabled by setting');

        delete window.scrollTo;
    });

    it('returns early when getIncludeReadUrl returns null', async () => {
        Object.defineProperty(window, 'location', {
            value: { pathname: '/settings' },
            writable: true,
            configurable: true,
        });

        window.fetch = vi.fn();
        await showHiddenArticles();

        expect(window.fetch).not.toHaveBeenCalled();
        expect(getShowingHiddenArticles()).toBe(true);
    });

    it('shows toast on fetch error', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {});
        Object.defineProperty(window, 'location', {
            value: { pathname: '/' },
            writable: true,
            configurable: true,
        });

        window.fetch = vi.fn().mockRejectedValue(new Error('network error'));
        await showHiddenArticles();

        expect(console.error).toHaveBeenCalled();
        expect(showToast).toHaveBeenCalledWith('Failed to load hidden articles');
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
