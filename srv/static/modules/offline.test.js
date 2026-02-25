import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    _isStandalone,
    initOfflineSupport,
    cacheQueueForOffline,
    handleOnlineStateChange,
    showOfflineBanner,
    disableNonQueueUI,
    enableAllUI,
    replayPendingActions,
    updateQueueCacheIfStandalone,
    setOfflineDeps,
} from './offline.js';

// Mock the api module
vi.mock('./api.js', () => ({
    api: vi.fn(),
}));

import { api } from './api.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.useFakeTimers();
    vi.restoreAllMocks();
    vi.clearAllMocks();
    setOfflineDeps({ updateCounts: vi.fn() });
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
});

describe('_isStandalone', () => {
    it('is false in jsdom (no matchMedia match)', () => {
        expect(_isStandalone).toBe(false);
    });
});

describe('showOfflineBanner', () => {
    it('creates a banner element with id offline-banner', () => {
        showOfflineBanner();
        const banner = document.getElementById('offline-banner');
        expect(banner).not.toBeNull();
        expect(banner.className).toBe('offline-banner');
    });

    it('does not duplicate the banner if already present', () => {
        showOfflineBanner();
        showOfflineBanner();
        const banners = document.querySelectorAll('#offline-banner');
        expect(banners.length).toBe(1);
    });

    it('shows queue link on non-queue pages', () => {
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        showOfflineBanner();
        const banner = document.getElementById('offline-banner');
        expect(banner.innerHTML).toContain('Go to Queue');
        expect(banner.querySelector('a[href="/queue"]')).not.toBeNull();
    });

    it('omits queue link on the queue page', () => {
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/queue' },
            writable: true,
            configurable: true,
        });
        showOfflineBanner();
        const banner = document.getElementById('offline-banner');
        expect(banner.innerHTML).not.toContain('Go to Queue');
    });

    it('prepends banner to document.body', () => {
        showOfflineBanner();
        expect(document.body.firstElementChild.id).toBe('offline-banner');
    });
});

describe('disableNonQueueUI', () => {
    function setupSidebar() {
        const sidebar = document.createElement('div');
        sidebar.className = 'sidebar';
        sidebar.innerHTML = `
            <a class="nav-item" href="/">Home</a>
            <a class="nav-item" href="/queue">Queue</a>
            <a class="nav-item" href="/starred">Starred</a>
            <a class="feed-item" href="/feeds/1">Feed 1</a>
            <a class="folder-item" href="/folders/1">Folder 1</a>
            <div class="feeds-section">Feeds</div>
            <div class="feeds-header">Header</div>
        `;
        document.body.appendChild(sidebar);
        return sidebar;
    }

    it('disables non-queue nav items', () => {
        setupSidebar();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        const home = document.querySelector('.nav-item[href="/"]');
        expect(home.classList.contains('offline-disabled')).toBe(true);
        expect(home.getAttribute('data-offline-disabled')).toBe('true');

        const starred = document.querySelector('.nav-item[href="/starred"]');
        expect(starred.classList.contains('offline-disabled')).toBe(true);
    });

    it('preserves queue nav item', () => {
        setupSidebar();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        const queue = document.querySelector('.nav-item[href="/queue"]');
        expect(queue.classList.contains('offline-disabled')).toBe(false);
        expect(queue.hasAttribute('data-offline-disabled')).toBe(false);
    });

    it('disables feed and folder items', () => {
        setupSidebar();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        expect(document.querySelector('.feed-item').classList.contains('offline-disabled')).toBe(true);
        expect(document.querySelector('.folder-item').classList.contains('offline-disabled')).toBe(true);
    });

    it('disables feeds-section and feeds-header', () => {
        setupSidebar();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        expect(document.querySelector('.feeds-section').classList.contains('offline-disabled')).toBe(true);
        expect(document.querySelector('.feeds-header').classList.contains('offline-disabled')).toBe(true);
    });

    it('shows overlay on non-queue page', () => {
        setupSidebar();
        const content = document.createElement('div');
        content.className = 'content';
        document.body.appendChild(content);

        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        const overlay = document.getElementById('offline-overlay');
        expect(overlay).not.toBeNull();
        expect(overlay.className).toBe('offline-overlay');
        expect(overlay.innerHTML).toContain("You're offline");
        expect(overlay.querySelector('a[href="/queue"]')).not.toBeNull();
    });

    it('does not show overlay on queue page', () => {
        setupSidebar();
        const content = document.createElement('div');
        content.className = 'content';
        document.body.appendChild(content);

        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/queue' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        expect(document.getElementById('offline-overlay')).toBeNull();
    });

    it('does not create duplicate overlay', () => {
        setupSidebar();
        const content = document.createElement('div');
        content.className = 'content';
        document.body.appendChild(content);

        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();
        disableNonQueueUI();

        expect(document.querySelectorAll('#offline-overlay').length).toBe(1);
    });
});

describe('enableAllUI', () => {
    it('removes offline-disabled class and attribute from elements', () => {
        const el1 = document.createElement('div');
        el1.classList.add('offline-disabled');
        el1.setAttribute('data-offline-disabled', 'true');
        document.body.appendChild(el1);

        const el2 = document.createElement('div');
        el2.classList.add('offline-disabled');
        el2.setAttribute('data-offline-disabled', 'true');
        document.body.appendChild(el2);

        enableAllUI();

        expect(el1.classList.contains('offline-disabled')).toBe(false);
        expect(el1.hasAttribute('data-offline-disabled')).toBe(false);
        expect(el2.classList.contains('offline-disabled')).toBe(false);
        expect(el2.hasAttribute('data-offline-disabled')).toBe(false);
    });

    it('removes the offline-overlay element', () => {
        const overlay = document.createElement('div');
        overlay.id = 'offline-overlay';
        document.body.appendChild(overlay);

        enableAllUI();

        expect(document.getElementById('offline-overlay')).toBeNull();
    });

    it('does nothing if no offline-disabled elements or overlay exist', () => {
        expect(() => enableAllUI()).not.toThrow();
    });
});

describe('replayPendingActions', () => {
    it('calls callback via setTimeout when no serviceWorker', () => {
        delete navigator.serviceWorker;
        const cb = vi.fn();

        replayPendingActions(cb);
        expect(cb).not.toHaveBeenCalled();

        vi.advanceTimersByTime(1);
        expect(cb).toHaveBeenCalledOnce();
    });

    it('calls callback via setTimeout when serviceWorker exists but no controller', () => {
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { controller: null, addEventListener: vi.fn(), removeEventListener: vi.fn() },
            configurable: true,
        });
        const cb = vi.fn();

        replayPendingActions(cb);
        expect(cb).not.toHaveBeenCalled();

        vi.advanceTimersByTime(1);
        expect(cb).toHaveBeenCalledOnce();
    });

    it('does not throw when called without callback and no SW', () => {
        delete navigator.serviceWorker;
        expect(() => replayPendingActions()).not.toThrow();
    });

    it('sends GET_PENDING_ACTIONS to SW controller when present', () => {
        const postMessage = vi.fn();
        const addEventListener = vi.fn();
        const removeEventListener = vi.fn();
        Object.defineProperty(navigator, 'serviceWorker', {
            value: {
                controller: { postMessage },
                addEventListener,
                removeEventListener,
            },
            configurable: true,
        });

        replayPendingActions(vi.fn());

        expect(addEventListener).toHaveBeenCalledWith('message', expect.any(Function));
        expect(postMessage).toHaveBeenCalledWith({ type: 'GET_PENDING_ACTIONS' });
    });

    it('replays dequeue actions and calls callback when SW responds', async () => {
        const postMessage = vi.fn();
        let messageHandler;
        const addEventListener = vi.fn((_event, handler) => { messageHandler = handler; });
        const removeEventListener = vi.fn();
        Object.defineProperty(navigator, 'serviceWorker', {
            value: {
                controller: { postMessage },
                addEventListener,
                removeEventListener,
            },
            configurable: true,
        });

        api.mockResolvedValue({});

        const mockUpdateCounts = vi.fn();
        setOfflineDeps({ updateCounts: mockUpdateCounts });

        const cb = vi.fn();
        replayPendingActions(cb);

        // Simulate SW response with pending actions
        messageHandler({
            data: {
                type: 'PENDING_ACTIONS',
                actions: [
                    { type: 'dequeue', articleId: '123' },
                    { type: 'dequeue', articleId: '456' },
                ],
            },
        });

        // Flush microtasks
        await vi.advanceTimersByTimeAsync(0);

        expect(api).toHaveBeenCalledWith('DELETE', '/api/articles/123/queue');
        expect(api).toHaveBeenCalledWith('DELETE', '/api/articles/456/queue');
        expect(mockUpdateCounts).toHaveBeenCalled();
        expect(cb).toHaveBeenCalledOnce();
        expect(removeEventListener).toHaveBeenCalledWith('message', messageHandler);
    });

    it('calls callback after safety timeout if SW does not respond', () => {
        const postMessage = vi.fn();
        const addEventListener = vi.fn();
        const removeEventListener = vi.fn();
        Object.defineProperty(navigator, 'serviceWorker', {
            value: {
                controller: { postMessage },
                addEventListener,
                removeEventListener,
            },
            configurable: true,
        });

        const cb = vi.fn();
        replayPendingActions(cb);

        expect(cb).not.toHaveBeenCalled();

        vi.advanceTimersByTime(3000);
        expect(cb).toHaveBeenCalledOnce();
        expect(removeEventListener).toHaveBeenCalled();
    });
});

describe('cacheQueueForOffline', () => {
    it('fetches queue articles and posts to SW', async () => {
        const articles = [{ id: 1, title: 'Test' }];
        api.mockResolvedValue(articles);

        const sw = { postMessage: vi.fn() };
        cacheQueueForOffline(sw);

        await vi.runAllTimersAsync();

        expect(api).toHaveBeenCalledWith('GET', '/api/queue');
        expect(sw.postMessage).toHaveBeenCalledWith({
            type: 'CACHE_QUEUE',
            data: { articles },
        });
    });

    it('sends empty array when api returns null', async () => {
        api.mockResolvedValue(null);

        const sw = { postMessage: vi.fn() };
        cacheQueueForOffline(sw);

        await vi.runAllTimersAsync();

        expect(sw.postMessage).toHaveBeenCalledWith({
            type: 'CACHE_QUEUE',
            data: { articles: [] },
        });
    });

    it('does not throw when fetch fails', async () => {
        api.mockRejectedValue(new Error('network error'));

        const sw = { postMessage: vi.fn() };
        cacheQueueForOffline(sw);

        await vi.runAllTimersAsync();

        expect(sw.postMessage).not.toHaveBeenCalled();
    });

    it('does not call postMessage when sw is null', async () => {
        api.mockResolvedValue([]);

        cacheQueueForOffline(null);

        await vi.runAllTimersAsync();
        // Should not throw
    });
});

describe('handleOnlineStateChange', () => {
    it('is a no-op when not in standalone mode', () => {
        expect(_isStandalone).toBe(false);
        handleOnlineStateChange();
        expect(document.getElementById('offline-banner')).toBeNull();
    });
});

describe('initOfflineSupport', () => {
    it('is a no-op when not in standalone mode', () => {
        expect(_isStandalone).toBe(false);
        initOfflineSupport();
        expect(document.getElementById('offline-banner')).toBeNull();
    });
});

describe('updateQueueCacheIfStandalone', () => {
    it('is a no-op when not in standalone mode', () => {
        expect(_isStandalone).toBe(false);
        updateQueueCacheIfStandalone();
    });
});

describe('setOfflineDeps', () => {
    it('accepts and stores dependencies', () => {
        const fn = vi.fn();
        setOfflineDeps({ updateCounts: fn });
        // Verified via replayPendingActions calling it (tested above)
    });
});
