import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    renderArticleActions, buildArticleCardHtml, updateReadButton,
    showArticlesLoading, updateAllReadMessage, showReadArticles,
    processEmbeds, extractYouTubeId, applyUserPreferences,
    getIncludeReadUrl, setArticlesDeps, setShowingHiddenArticles,
    _resetArticlesState,
} from './articles.js';
import { _resetArticleActionsState, setQueuedArticleIds } from './article-actions.js';

beforeEach(() => {
    vi.spyOn(console, 'debug').mockImplementation(() => {});
    _resetArticlesState();
    _resetArticleActionsState();
    setQueuedArticleIds(new Set());
    window.__settings = {};
    window.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }));
    document.body.innerHTML = '<div id="articles-list" class="articles-list"></div>';
    setArticlesDeps({
        updatePaginationCursor: vi.fn(),
        updateEndOfArticlesIndicator: vi.fn(),
        setPaginationState: vi.fn(),
    });
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('renderArticleActions', () => {
    it('renders read/star/queue buttons for unread article', () => {
        const html = renderArticleActions({ id: 1, is_read: false, is_starred: false, url: 'https://example.com' });
        expect(html).toContain('markRead');
        expect(html).toContain('toggleStar');
        expect(html).toContain('toggleQueue');
        expect(html).toContain('Open original');
    });

    it('renders markUnread button for read article', () => {
        const html = renderArticleActions({ id: 1, is_read: true, is_starred: false });
        expect(html).toContain('markUnread');
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
            '<div class="article-card" data-id="5"><button class="btn-read-toggle"></button></div>';
        const card = document.querySelector('.article-card');
        updateReadButton(card, true);
        expect(card.querySelector('.btn-read-toggle').getAttribute('onclick')).toContain('markUnread');
    });

    it('switches button to markRead when isRead=false', () => {
        document.body.innerHTML =
            '<div class="article-card" data-id="5"><button class="btn-read-toggle"></button></div>';
        const card = document.querySelector('.article-card');
        updateReadButton(card, false);
        expect(card.querySelector('.btn-read-toggle').getAttribute('onclick')).toContain('markRead');
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
