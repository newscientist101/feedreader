import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { loadApp, extractTopLevelNames } from './test-helper.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

beforeEach(() => {
  vi.useFakeTimers();
  loadApp();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Pure utility functions
// ---------------------------------------------------------------------------

describe('formatTimeAgo', () => {
  it('returns empty string for falsy input', () => {
    expect(window.formatTimeAgo('')).toBe('');
    expect(window.formatTimeAgo(null)).toBe('');
    expect(window.formatTimeAgo(undefined)).toBe('');
  });

  it('returns "just now" for < 1 minute ago', () => {
    const now = new Date();
    expect(window.formatTimeAgo(now.toISOString())).toBe('just now');
  });

  it('returns minutes ago', () => {
    const d = new Date(Date.now() - 5 * 60000);
    expect(window.formatTimeAgo(d.toISOString())).toBe('5m ago');
  });

  it('returns hours ago', () => {
    const d = new Date(Date.now() - 3 * 3600000);
    expect(window.formatTimeAgo(d.toISOString())).toBe('3h ago');
  });

  it('returns days ago', () => {
    const d = new Date(Date.now() - 2 * 86400000);
    expect(window.formatTimeAgo(d.toISOString())).toBe('2d ago');
  });

  it('returns a date string for > 7 days', () => {
    const d = new Date(Date.now() - 10 * 86400000);
    const result = window.formatTimeAgo(d.toISOString());
    // Should be a locale date string, not "Xd ago"
    expect(result).not.toMatch(/d ago/);
  });
});

describe('stripHtml', () => {
  it('strips HTML tags', () => {
    expect(window.stripHtml('<p>Hello <b>world</b></p>')).toBe('Hello world');
  });

  it('returns empty string for empty input', () => {
    expect(window.stripHtml('')).toBe('');
  });
});

describe('truncateText', () => {
  it('returns text unchanged if under limit', () => {
    expect(window.truncateText('hello', 10)).toBe('hello');
  });

  it('truncates with ellipsis', () => {
    expect(window.truncateText('hello world', 5)).toBe('hello...');
  });

  it('handles null/undefined', () => {
    expect(window.truncateText(null, 5)).toBeNull();
    expect(window.truncateText(undefined, 5)).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

describe('getSetting / saveSetting', () => {
  it('returns default when no setting exists', () => {
    expect(window.getSetting('nonexistent', 'fallback')).toBe('fallback');
  });

  it('returns saved value', () => {
    window.__settings = { autoMarkRead: 'true' };
    expect(window.getSetting('autoMarkRead')).toBe('true');
  });

  it('saveSetting updates window.__settings', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    window.saveSetting('autoMarkRead', 'true');
    expect(window.__settings.autoMarkRead).toBe('true');
  });

  it('saveSetting calls fetch', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    window.saveSetting('hideReadArticles', 'hide');
    expect(window.fetch).toHaveBeenCalledWith('/api/settings', expect.objectContaining({
      method: 'PUT',
    }));
  });
});

// ---------------------------------------------------------------------------
// getPaginationUrl
// ---------------------------------------------------------------------------

describe('getPaginationUrl', () => {
  it('returns unread URL for root path', () => {
    Object.defineProperty(window, 'location', {
      value: { pathname: '/' }, writable: true, configurable: true,
    });
    expect(window.getPaginationUrl()).toBe('/api/articles/unread');
  });

  it('returns feed URL for feed path', () => {
    Object.defineProperty(window, 'location', {
      value: { pathname: '/feed/42' }, writable: true, configurable: true,
    });
    expect(window.getPaginationUrl()).toBe('/api/feeds/42/articles');
  });

  it('returns category URL for category path', () => {
    Object.defineProperty(window, 'location', {
      value: { pathname: '/category/54' }, writable: true, configurable: true,
    });
    expect(window.getPaginationUrl()).toBe('/api/categories/54/articles');
  });

  it('returns null for unknown paths', () => {
    Object.defineProperty(window, 'location', {
      value: { pathname: '/settings' }, writable: true, configurable: true,
    });
    expect(window.getPaginationUrl()).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Auto-mark-read: IntersectionObserver lifecycle
// ---------------------------------------------------------------------------

describe('initAutoMarkRead', () => {
  it('does nothing when setting is disabled', () => {
    window.__settings = { autoMarkRead: 'false' };
    window.initAutoMarkRead();
    expect(window.autoMarkReadObserver).toBeNull();
  });

  it('creates an IntersectionObserver when enabled', () => {
    window.__settings = { autoMarkRead: 'true' };
    window.initAutoMarkRead();
    expect(window.autoMarkReadObserver).not.toBeNull();
  });

  it('disconnects previous observer on re-init', () => {
    window.__settings = { autoMarkRead: 'true' };
    window.initAutoMarkRead();
    const first = window.autoMarkReadObserver;
    const disconnectSpy = vi.spyOn(first, 'disconnect');

    // Re-initialize (simulates client-side navigation calling renderArticles)
    window.initAutoMarkRead();
    expect(disconnectSpy).toHaveBeenCalled();
    expect(window.autoMarkReadObserver).not.toBe(first);
  });

  it('observes all article cards in the DOM', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card" data-id="1"></article>
      <article class="article-card" data-id="2"></article>
      <article class="article-card" data-id="3"></article>
    `;
    window.__settings = { autoMarkRead: 'true' };

    // Spy on the mock's prototype so it catches all new instances
    const observeSpy = vi.spyOn(window.IntersectionObserver.prototype, 'observe');
    window.initAutoMarkRead();
    expect(observeSpy).toHaveBeenCalledTimes(3);
    observeSpy.mockRestore();
  });

  it('cleans up observer when setting toggled off', () => {
    window.__settings = { autoMarkRead: 'true' };
    window.initAutoMarkRead();
    const obs = window.autoMarkReadObserver;
    const disconnectSpy = vi.spyOn(obs, 'disconnect');

    window.__settings = { autoMarkRead: 'false' };
    window.initAutoMarkRead();
    expect(disconnectSpy).toHaveBeenCalled();
    expect(window.autoMarkReadObserver).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// observeNewArticles — called after pagination loads more
// ---------------------------------------------------------------------------

describe('observeNewArticles', () => {
  it('does nothing when observer is null (feature disabled)', () => {
    window.autoMarkReadObserver = null;
    const container = document.createElement('div');
    container.innerHTML = '<article class="article-card" data-id="1"></article>';
    // Should not throw
    window.observeNewArticles(container);
  });

  it('observes new article cards in the provided container', () => {
    window.__settings = { autoMarkRead: 'true' };
    window.initAutoMarkRead();
    const spy = vi.spyOn(window.autoMarkReadObserver, 'observe');

    const container = document.createElement('div');
    container.innerHTML = `
      <article class="article-card" data-id="10"></article>
      <article class="article-card" data-id="11"></article>
    `;
    window.observeNewArticles(container);
    expect(spy).toHaveBeenCalledTimes(2);
  });
});

// ---------------------------------------------------------------------------
// markReadSilent / flushMarkReadQueue — batched mark-read
// ---------------------------------------------------------------------------

describe('markReadSilent', () => {
  it('adds the "read" class to the article card', () => {
    document.getElementById('articles-list').innerHTML =
      '<article class="article-card" data-id="42"></article>';
    window.markReadSilent(42);
    const card = document.querySelector('.article-card[data-id="42"]');
    expect(card.classList.contains('read')).toBe(true);
  });

  it('queues the article id for batch flush', () => {
    window.markReadSilent(42);
    expect(window._markReadQueue).toContain(42);
  });

  it('flushes the queue after a timeout', () => {
    window.fetch = vi.fn(() => Promise.resolve({
      ok: true, json: () => Promise.resolve({ status: 'ok' }),
    }));
    window.markReadSilent(1);
    window.markReadSilent(2);
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
    window.markReadSilent(1);
    vi.advanceTimersByTime(300);
    window.markReadSilent(2);
    vi.advanceTimersByTime(300);
    // Only 600ms total, but timer was reset at 300ms so another 200ms to go
    expect(window.fetch).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(window.fetch).toHaveBeenCalledTimes(1);
    const body = JSON.parse(window.fetch.mock.calls[0][1].body);
    expect(body.ids).toEqual([1, 2]);
  });
});

describe('flushMarkReadQueue', () => {
  it('does nothing when queue is empty', () => {
    window.fetch = vi.fn();
    window.flushMarkReadQueue();
    expect(window.fetch).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// renderArticles — re-initializes observer after client-side navigation
// ---------------------------------------------------------------------------

describe('renderArticles', () => {
  beforeEach(() => {
    window.__settings = { autoMarkRead: 'true' };
    window.fetch = vi.fn(() => Promise.resolve({
      ok: true, json: () => Promise.resolve({}),
    }));
  });

  it('renders articles into the list', async () => {
    const articles = [
      { id: 1, title: 'Article 1', is_read: 0, is_starred: 0 },
      { id: 2, title: 'Article 2', is_read: 0, is_starred: 0 },
    ];
    await window.renderArticles(articles);

    const cards = document.querySelectorAll('#articles-list .article-card');
    expect(cards.length).toBe(2);
    expect(cards[0].dataset.id).toBe('1');
    expect(cards[1].dataset.id).toBe('2');
  });

  it('re-initializes the auto-mark-read observer', async () => {
    // First init
    window.initAutoMarkRead();
    const firstObserver = window.autoMarkReadObserver;

    // Simulate navigation: renderArticles replaces content
    await window.renderArticles([
      { id: 10, title: 'New', is_read: 0, is_starred: 0 },
    ]);

    // Observer should be a new instance (old one disconnected)
    expect(window.autoMarkReadObserver).not.toBeNull();
    expect(window.autoMarkReadObserver).not.toBe(firstObserver);
  });

  it('handles empty article list', async () => {
    await window.renderArticles([]);
    const cards = document.querySelectorAll('#articles-list .article-card');
    expect(cards.length).toBe(0);
    // Should show empty state
    expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
  });

  it('handles null articles', async () => {
    await window.renderArticles(null);
    expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
  });
});

// ---------------------------------------------------------------------------
// buildArticleCardHtml
// ---------------------------------------------------------------------------

describe('buildArticleCardHtml', () => {
  it('includes data-id attribute', () => {
    const html = window.buildArticleCardHtml({
      id: 99, title: 'Test', is_read: 0, is_starred: 0,
    });
    expect(html).toContain('data-id="99"');
  });

  it('adds read class for read articles', () => {
    const html = window.buildArticleCardHtml({
      id: 1, title: 'Test', is_read: 1, is_starred: 0,
    });
    expect(html).toContain('read');
  });

  it('includes title text', () => {
    const html = window.buildArticleCardHtml({
      id: 1, title: 'My Great Article', is_read: 0, is_starred: 0,
    });
    expect(html).toContain('My Great Article');
  });

  it('includes feed name when provided', () => {
    const html = window.buildArticleCardHtml({
      id: 1, title: 'Test', feed_name: 'Tech Blog', is_read: 0, is_starred: 0,
    });
    expect(html).toContain('Tech Blog');
  });

  it('includes summary preview', () => {
    const html = window.buildArticleCardHtml({
      id: 1, title: 'Test', summary: 'A brief summary', is_read: 0, is_starred: 0,
    });
    expect(html).toContain('A brief summary');
  });
});

// ---------------------------------------------------------------------------
// User preferences
// ---------------------------------------------------------------------------

describe('applyUserPreferences', () => {
  it('hides read articles when hideReadArticles is "hide"', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card read" data-id="1"></article>
      <article class="article-card" data-id="2"></article>
    `;
    window.__settings = { hideReadArticles: 'hide' };
    window.applyUserPreferences();

    const readCard = document.querySelector('.article-card[data-id="1"]');
    expect(readCard.style.display).toBe('none');
    const unreadCard = document.querySelector('.article-card[data-id="2"]');
    expect(unreadCard.style.display).not.toBe('none');
  });

  it('does not hide read articles when setting is not "hide"', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card read" data-id="1"></article>
    `;
    window.__settings = {};
    window.applyUserPreferences();

    const card = document.querySelector('.article-card[data-id="1"]');
    expect(card.style.display).not.toBe('none');
  });
});

describe('updateAllReadMessage', () => {
  it('shows all-read message when all articles are hidden', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card read" data-id="1" style="display: none"></article>
    `;
    window.__settings = { hideReadArticles: 'hide' };
    window.updateAllReadMessage();

    expect(document.getElementById('all-read-message')).not.toBeNull();
  });

  it('does not show message when visible articles exist', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card" data-id="1"></article>
    `;
    window.__settings = { hideReadArticles: 'hide' };
    window.updateAllReadMessage();

    expect(document.getElementById('all-read-message')).toBeNull();
  });
});

describe('showReadArticles', () => {
  it('un-hides read article cards', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card read" data-id="1" style="display: none"></article>
    `;
    window.showReadArticles();
    const card = document.querySelector('.article-card[data-id="1"]');
    expect(card.style.display).toBe('');
  });
});

// ---------------------------------------------------------------------------
// updateEndOfArticlesIndicator
// ---------------------------------------------------------------------------

describe('updateEndOfArticlesIndicator', () => {
  it('shows indicator when pagination is done and articles exist', () => {
    document.getElementById('articles-list').innerHTML =
      '<article class="article-card" data-id="1"></article>';
    window.paginationDone = true;
    window.updateEndOfArticlesIndicator();
    expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(true);
  });

  it('hides indicator when pagination is not done', () => {
    document.getElementById('articles-list').innerHTML =
      '<article class="article-card" data-id="1"></article>';
    window.paginationDone = false;
    window.updateEndOfArticlesIndicator();
    expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(false);
  });

  it('hides indicator when no articles', () => {
    window.paginationDone = true;
    window.updateEndOfArticlesIndicator();
    expect(document.getElementById('end-of-articles').classList.contains('visible')).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Full auto-mark-read flow: simulates the bug scenario
// ---------------------------------------------------------------------------

describe('auto-mark-read after client-side navigation (bug fix)', () => {
  beforeEach(() => {
    window.__settings = { autoMarkRead: 'true' };
    window.fetch = vi.fn(() => Promise.resolve({
      ok: true, json: () => Promise.resolve({ status: 'ok' }),
    }));
  });

  it('observer works on initial page load articles', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card" data-id="1"></article>
      <article class="article-card" data-id="2"></article>
    `;

    const observeSpy = vi.spyOn(window.IntersectionObserver.prototype, 'observe');
    window.initAutoMarkRead();
    expect(window.autoMarkReadObserver).not.toBeNull();
    expect(observeSpy).toHaveBeenCalledTimes(2);
    observeSpy.mockRestore();
  });

  it('observer is re-created after renderArticles (client-side nav)', async () => {
    // Initial page
    document.getElementById('articles-list').innerHTML =
      '<article class="article-card" data-id="1"></article>';
    window.initAutoMarkRead();
    const initialObserver = window.autoMarkReadObserver;

    // Simulate navigating to category/54 (subfolder)
    await window.renderArticles([
      { id: 100, title: 'VR Article', is_read: 0, is_starred: 0, feed_name: 'VR Feed' },
      { id: 101, title: 'VR News', is_read: 0, is_starred: 0, feed_name: 'VR Feed' },
    ]);

    // Observer should have been replaced
    expect(window.autoMarkReadObserver).not.toBe(initialObserver);
    expect(window.autoMarkReadObserver).not.toBeNull();

    // New articles should be in the DOM
    const cards = document.querySelectorAll('#articles-list .article-card');
    expect(cards.length).toBe(2);
    expect(cards[0].dataset.id).toBe('100');
  });

  it('new paginated articles are observed', async () => {
    // Set up initial observer
    window.initAutoMarkRead();
    const spy = vi.spyOn(window.autoMarkReadObserver, 'observe');

    // Simulate what loadMoreArticles does: create cards and call observeNewArticles
    const temp = document.createElement('div');
    temp.innerHTML = `
      <article class="article-card" data-id="50"></article>
      <article class="article-card" data-id="51"></article>
      <article class="article-card" data-id="52"></article>
    `;
    window.observeNewArticles(temp);

    expect(spy).toHaveBeenCalledTimes(3);
  });

  it('multiple navigations each get a fresh observer', async () => {
    const observers = [];

    // Navigate 3 times
    for (let i = 0; i < 3; i++) {
      await window.renderArticles([
        { id: i * 10 + 1, title: `Article ${i}`, is_read: 0, is_starred: 0 },
      ]);
      observers.push(window.autoMarkReadObserver);
    }

    // Each navigation should produce a distinct observer
    expect(observers[0]).not.toBe(observers[1]);
    expect(observers[1]).not.toBe(observers[2]);
    // All should be non-null
    observers.forEach(obs => expect(obs).not.toBeNull());
  });
});

// ---------------------------------------------------------------------------
// renderArticleActions
// ---------------------------------------------------------------------------

describe('renderArticleActions', () => {
  it('renders read button for unread article', () => {
    const html = window.renderArticleActions({ id: 1, is_read: 0, is_starred: 0, url: 'http://ex.com' });
    expect(html).toContain('markRead');
    expect(html).toContain('btn-read-toggle');
  });

  it('renders unread button for read article', () => {
    const html = window.renderArticleActions({ id: 1, is_read: 1, is_starred: 0 });
    expect(html).toContain('markUnread');
  });

  it('renders external link when url provided', () => {
    const html = window.renderArticleActions({ id: 1, is_read: 0, is_starred: 0, url: 'http://ex.com' });
    expect(html).toContain('http://ex.com');
  });

  it('omits external link when no url', () => {
    const html = window.renderArticleActions({ id: 1, is_read: 0, is_starred: 0, url: '' });
    expect(html).not.toContain('Open original');
  });
});

// ---------------------------------------------------------------------------
// openArticle / openArticleExternal
// ---------------------------------------------------------------------------

describe('openArticle', () => {
  it('marks article read and flushes immediately', () => {
    window.fetch = vi.fn(() => Promise.resolve({
      ok: true, json: () => Promise.resolve({ status: 'ok' }),
    }));

    window.openArticle(42);

    // openArticle calls markReadSilent then flushMarkReadQueue immediately,
    // so the batch-read API should have been called with the article id.
    const batchCall = window.fetch.mock.calls.find(c => c[0] === '/api/articles/batch-read');
    expect(batchCall).toBeDefined();
    const body = JSON.parse(batchCall[1].body);
    expect(body.ids).toContain(42);
  });
});

describe('openArticleExternal', () => {
  it('marks article read and opens in new tab', () => {
    window.fetch = vi.fn(() => Promise.resolve({
      ok: true, json: () => Promise.resolve({ status: 'ok' }),
    }));
    window.open = vi.fn();
    const event = { stopPropagation: vi.fn() };

    window.openArticleExternal(event, 99, 'http://example.com');
    expect(event.stopPropagation).toHaveBeenCalled();
    expect(window.open).toHaveBeenCalledWith('http://example.com', '_blank');
    expect(window._markReadQueue).toContain(99);
  });
});

// ---------------------------------------------------------------------------
// formatLocalDate
// ---------------------------------------------------------------------------

describe('formatLocalDate', () => {
  it('returns a formatted date string', () => {
    const result = window.formatLocalDate('2024-06-15T10:30:00Z');
    // Should contain the year and some date components
    expect(result).toContain('2024');
    expect(result.length).toBeGreaterThan(5);
  });
});

// ---------------------------------------------------------------------------
// View scope / view switching
// ---------------------------------------------------------------------------

describe('getViewScope', () => {
  it('returns "all" when no articles-view element', () => {
    document.querySelector('.articles-view')?.removeAttribute('data-view-scope');
    expect(window.getViewScope()).toBe('all');
  });

  it('returns the data-view-scope attribute', () => {
    const view = document.querySelector('.articles-view');
    view.dataset.viewScope = 'folder';
    expect(window.getViewScope()).toBe('folder');
  });
});

describe('setView', () => {
  it('applies the view class to articles-list', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    window.setView('magazine');
    const list = document.getElementById('articles-list');
    expect(list.classList.contains('view-magazine')).toBe(true);
    expect(list.classList.contains('view-card')).toBe(false);
  });

  it('falls back compact to list', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    window.setView('compact');
    const list = document.getElementById('articles-list');
    expect(list.classList.contains('view-list')).toBe(true);
  });
});

describe('getDefaultViewForScope', () => {
  it('returns card as default', () => {
    expect(window.getDefaultViewForScope('all')).toBe('card');
  });

  it('returns saved folder view', () => {
    window.__settings = { defaultFolderView: 'list' };
    expect(window.getDefaultViewForScope('folder')).toBe('list');
  });

  it('returns saved feed view', () => {
    window.__settings = { defaultFeedView: 'expanded' };
    expect(window.getDefaultViewForScope('feed')).toBe('expanded');
  });
});

describe('applyDefaultViewForScope', () => {
  it('applies saved view without saving', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    window.__settings = { defaultFolderView: 'magazine' };
    window.applyDefaultViewForScope('folder');
    const list = document.getElementById('articles-list');
    expect(list.classList.contains('view-magazine')).toBe(true);
    // Should not have saved (save: false)
    expect(window.fetch).not.toHaveBeenCalled();
  });
});

describe('migrateLegacyViewDefaults', () => {
  it('migrates localStorage keys to settings', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    localStorage.setItem('feedreader-view-folder-default', 'list');
    window.__settings = {};

    window.migrateLegacyViewDefaults();

    expect(window.__settings.defaultFolderView).toBe('list');
    expect(localStorage.getItem('feedreader-view-folder-default')).toBeNull();
  });

  it('does not overwrite existing settings', () => {
    window.fetch = vi.fn(() => Promise.resolve({ ok: true }));
    localStorage.setItem('feedreader-view-folder-default', 'list');
    window.__settings = { defaultFolderView: 'card' };

    window.migrateLegacyViewDefaults();

    expect(window.__settings.defaultFolderView).toBe('card');
  });
});

// ---------------------------------------------------------------------------
// api wrapper
// ---------------------------------------------------------------------------

describe('api', () => {
  it('sends JSON request and returns parsed response', async () => {
    window.fetch = vi.fn(() => Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    }));
    const result = await window.api('POST', '/api/test', { foo: 'bar' });
    expect(result).toEqual({ status: 'ok' });
    expect(window.fetch).toHaveBeenCalledWith('/api/test', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ foo: 'bar' }),
    }));
  });

  it('throws on non-ok response', async () => {
    window.fetch = vi.fn(() => Promise.resolve({
      ok: false,
      text: () => Promise.resolve('{"error":"bad request"}'),
    }));
    await expect(window.api('GET', '/api/fail')).rejects.toThrow('bad request');
  });
});

// ---------------------------------------------------------------------------
// findNextUnreadFolder
// ---------------------------------------------------------------------------

describe('findNextUnreadFolder', () => {
  beforeEach(() => {
    document.body.innerHTML += `
      <div class="folder-item" data-category-id="1"></div>
      <div class="folder-item" data-category-id="2"></div>
      <div class="folder-item" data-category-id="3"></div>
      <span data-count="category-3">5</span>
    `;
  });

  it('returns the next folder with unread count', () => {
    expect(window.findNextUnreadFolder(1)).toBe('/category/3');
  });

  it('returns null when no unread folders', () => {
    document.querySelector('[data-count="category-3"]').textContent = '0';
    expect(window.findNextUnreadFolder(1)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// extractYouTubeId
// ---------------------------------------------------------------------------

describe('extractYouTubeId', () => {
  it('extracts from watch URL', () => {
    expect(window.extractYouTubeId('https://youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
  });

  it('extracts from shorts URL', () => {
    expect(window.extractYouTubeId('https://youtube.com/shorts/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
  });

  it('extracts from youtu.be URL', () => {
    expect(window.extractYouTubeId('https://youtu.be/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ');
  });

  it('returns null for non-YouTube URL', () => {
    expect(window.extractYouTubeId('https://example.com')).toBeNull();
  });

  it('returns null for empty input', () => {
    expect(window.extractYouTubeId(null)).toBeNull();
    expect(window.extractYouTubeId('')).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// applyHideReadArticles / applyHideEmptyFeeds
// ---------------------------------------------------------------------------

describe('applyHideReadArticles', () => {
  it('hides read articles when value is "hide"', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card read" data-id="1"></article>
      <article class="article-card" data-id="2"></article>
    `;
    window.applyHideReadArticles('hide');
    expect(document.querySelector('[data-id="1"]').style.display).toBe('none');
    expect(document.querySelector('[data-id="2"]').style.display).toBe('');
  });

  it('shows read articles when value is not "hide"', () => {
    document.getElementById('articles-list').innerHTML = `
      <article class="article-card read" data-id="1" style="display:none"></article>
    `;
    window.applyHideReadArticles('show');
    expect(document.querySelector('[data-id="1"]').style.display).toBe('');
  });
});

describe('applyHideEmptyFeeds', () => {
  it('hides feeds with zero count', () => {
    document.body.innerHTML += `
      <div class="feed-item" data-feed-id="1"><span class="badge">0</span></div>
      <div class="feed-item" data-feed-id="2"><span class="badge">5</span></div>
    `;
    window.applyHideEmptyFeeds('hide');
    expect(document.querySelector('[data-feed-id="1"]').style.display).toBe('none');
    expect(document.querySelector('[data-feed-id="2"]').style.display).toBe('');
  });
});

// ---------------------------------------------------------------------------
// Meta-test: ensure every app.js function has a test or is explicitly skipped.
// When adding a new function to app.js, either write a test for it or add it
// to the skip list below with a reason.
// ---------------------------------------------------------------------------

// Functions that are intentionally untested (with reasons).
// Keep this list as small as possible — prefer writing tests.
const UNTESTED_FUNCTIONS = {
  // -- UI modals / prompts (require complex DOM + user interaction) --
  closeCreateFolderModal:    'trivial modal close',
  closeEditModal:            'trivial modal close',
  createEditFeedModal:       'builds modal DOM, no logic to test',
  openCreateFolderModal:     'trivial modal open',
  confirmDeleteAndReload:    'thin wrapper around confirm() + fetch',

  // -- DOM event wiring / init functions --
  initDragDrop:              'drag-and-drop event wiring',
  initFolderDragDrop:        'drag-and-drop event wiring',
  initSettingsPage:          'settings page DOM wiring',
  initTimestampTooltips:     'tooltip DOM wiring',
  initView:                  'calls migrateLegacyViewDefaults + applyDefaultViewForScope (both tested)',

  // -- Thin API wrappers (single fetch call + DOM update) --
  copyNewsletterAddress:     'clipboard API wrapper',
  deleteCategory:            'confirm + API call',
  deleteFeed:                'confirm + API call',
  editFeed:                  'modal + API call',
  exportOPML:                'window.location redirect',
  generateNewsletterAddress: 'API call + DOM update',
  importOPML:                'file upload + API call',
  loadNewsletterAddress:     'API call + DOM update',
  markAsRead:                'API call + optional navigation',
  markRead:                  'API call + DOM update',
  markUnread:                'API call + DOM update',
  refreshFeed:               'complex multi-step API call with polling',
  renameCategory:            'prompt + API call',
  runCleanup:                'API call + DOM update',
  saveFeed:                  'form submit + API call',
  setFeedCategory:           'API call + DOM update',
  submitCreateFolder:        'form submit + API call',
  toggleQueue:               'API call + DOM update',
  toggleStar:                'API call + DOM update',
  unparentCategory:          'API call + reload',
  updateCounts:              'API call + sidebar DOM update',

  // -- Side-effect-heavy functions needing full page context --
  collapseFolder:            'CSS class toggle + sessionStorage',
  filterFeeds:               'sidebar filter with full DOM',
  loadCategoryArticles:      'full page navigation (tested via renderArticles)',
  loadFeedArticles:          'full page navigation (tested via renderArticles)',
  loadMoreArticles:          'pagination (integration; observeNewArticles tested separately)',
  navigateFolder:            'event handler dispatching to loadCategoryArticles',
  processEmbeds:             'YouTube embed processing',
  reorderElements:           'drag-and-drop reorder',
  syncFolderOrder:           'API call for drag-and-drop reorder',
  toggleDropdown:            'dropdown menu toggle',
  toggleFolderCollapse:      'folder expand/collapse',
  toggleSidebar:             'sidebar toggle',

  // -- Functions with minimal logic --
  getDragAfterElementAmongSiblings: 'drag-and-drop geometry helper',
  removeFeedErrorBanner:     'trivial DOM removal',
  setSidebarActive:          'CSS class toggle',
  showArticlesLoading:       'loading spinner HTML',
  showFeedErrorBanner:       'error banner HTML',
  showNewsletterAddress:     'DOM update',
  updateFeedErrors:          'iterates feed errors + calls updateFeedStatusCell',
  updateFeedStatusCell:      'DOM update for feed status',
  updateReadButton:          'button HTML swap',
};

describe('test coverage check', () => {
  it('every app.js function is either tested or explicitly skipped', () => {
    const src = readFileSync(resolve(__dirname, 'app.js'), 'utf-8');
    const { functions } = extractTopLevelNames(src);
    const testSrc = readFileSync(resolve(__dirname, 'app.test.js'), 'utf-8');

    const missing = [];
    const staleSkips = [];

    for (const fn of functions) {
      // Match window.fn( or window.fn, or window.fn) etc. but not window.fnFoo
      const pattern = new RegExp(`window\\.${fn}(?![A-Za-z0-9_])`);
      const isTested = pattern.test(testSrc);
      const isSkipped = fn in UNTESTED_FUNCTIONS;

      if (!isTested && !isSkipped) {
        missing.push(fn);
      }
      if (isTested && isSkipped) {
        staleSkips.push(fn);
      }
    }

    // Check for skip-list entries that don't correspond to real functions
    const fnSet = new Set(functions);
    const phantom = Object.keys(UNTESTED_FUNCTIONS).filter(k => !fnSet.has(k));

    const errors = [];
    if (missing.length > 0) {
      errors.push(
        `Functions missing tests (add tests or add to UNTESTED_FUNCTIONS with a reason):\n` +
        missing.map(f => `  - ${f}`).join('\n')
      );
    }
    if (staleSkips.length > 0) {
      errors.push(
        `Functions in UNTESTED_FUNCTIONS that now have tests (remove from skip list):\n` +
        staleSkips.map(f => `  - ${f}`).join('\n')
      );
    }
    if (phantom.length > 0) {
      errors.push(
        `UNTESTED_FUNCTIONS entries that don't match any app.js function (remove them):\n` +
        phantom.map(f => `  - ${f}`).join('\n')
      );
    }

    if (errors.length > 0) {
      throw new Error('\n' + errors.join('\n\n'));
    }
  });
});
