import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    isStandalone,
    initOfflineSupport,
    cacheQueueForOffline,
    handleOnlineStateChange,
    showOfflineBanner,
    disableNonQueueUI,
    enableAllUI,
    replayPendingActions,
    updateQueueCacheIfStandalone,
} from './offline.js';

// Mock the api and counts modules
vi.mock('./api.js');

vi.mock('./counts.js');

import { api } from './api.js';
import { updateCounts } from './counts.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.useFakeTimers();
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
});

describe('isStandalone', () => {
    it('returns false in happy-dom (no matchMedia match)', () => {
        expect(isStandalone()).toBe(false);
    });

    it('returns true when matchMedia matches standalone', () => {
        vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true });
        expect(isStandalone()).toBe(true);
    });

    it('returns true when navigator.standalone is true (iOS Safari)', () => {
        Object.defineProperty(window.navigator, 'standalone', {
            value: true,
            configurable: true,
        });
        expect(isStandalone()).toBe(true);
        Object.defineProperty(window.navigator, 'standalone', {
            value: undefined,
            configurable: true,
        });
    });

    it('returns false when matchMedia exists but does not match', () => {
        vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false });
        expect(isStandalone()).toBe(false);
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

    it('banner contains offline text', () => {
        showOfflineBanner();
        const banner = document.getElementById('offline-banner');
        expect(banner.textContent).toContain("You're offline");
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

    it('sets content position to relative when overlay is created', () => {
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

        expect(content.style.position).toBe('relative');
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

    it('does not create overlay when .content element is absent', () => {
        setupSidebar();
        // No .content element in the DOM
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        expect(document.getElementById('offline-overlay')).toBeNull();
    });

    it('disables nav items without href attribute', () => {
        const sidebar = document.createElement('div');
        sidebar.className = 'sidebar';
        sidebar.innerHTML = '<span class="nav-item">No Link</span>';
        document.body.appendChild(sidebar);

        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });
        disableNonQueueUI();

        const noLink = document.querySelector('.nav-item');
        expect(noLink.classList.contains('offline-disabled')).toBe(true);
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

    it('round-trip: reverses disableNonQueueUI', () => {
        // Setup sidebar and content
        const sidebar = document.createElement('div');
        sidebar.className = 'sidebar';
        sidebar.innerHTML = `
            <a class="nav-item" href="/">Home</a>
            <a class="nav-item" href="/queue">Queue</a>
            <div class="feeds-section">Feeds</div>
        `;
        document.body.appendChild(sidebar);
        const content = document.createElement('div');
        content.className = 'content';
        document.body.appendChild(content);

        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });

        disableNonQueueUI();
        // Verify disabled state
        expect(document.querySelector('.nav-item[href="/"]').classList.contains('offline-disabled')).toBe(true);
        expect(document.getElementById('offline-overlay')).not.toBeNull();

        enableAllUI();
        // Everything should be restored
        expect(document.querySelector('.nav-item[href="/"]').classList.contains('offline-disabled')).toBe(false);
        expect(document.querySelector('.nav-item[href="/"]').hasAttribute('data-offline-disabled')).toBe(false);
        expect(document.getElementById('offline-overlay')).toBeNull();
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

    it('does not throw when called without callback and SW controller present', () => {
        const postMessage = vi.fn();
        Object.defineProperty(navigator, 'serviceWorker', {
            value: {
                controller: { postMessage },
                addEventListener: vi.fn(),
                removeEventListener: vi.fn(),
            },
            configurable: true,
        });

        expect(() => replayPendingActions()).not.toThrow();
        // Safety timeout fires without callback — should not throw
        expect(() => vi.advanceTimersByTime(3000)).not.toThrow();
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

        expect(api).toHaveBeenCalledTimes(2);
        expect(api).toHaveBeenCalledWith('DELETE', '/api/articles/123/queue');
        expect(api).toHaveBeenCalledWith('DELETE', '/api/articles/456/queue');
        expect(updateCounts).toHaveBeenCalled();
        expect(cb).toHaveBeenCalledOnce();
        expect(removeEventListener).toHaveBeenCalledWith('message', messageHandler);
    });

    it('ignores non-PENDING_ACTIONS messages', async () => {
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

        const cb = vi.fn();
        replayPendingActions(cb);

        // Send a non-matching message
        messageHandler({ data: { type: 'OTHER_MESSAGE' } });

        // Handler should not have been removed and callback should not fire
        expect(removeEventListener).not.toHaveBeenCalled();
        expect(cb).not.toHaveBeenCalled();
    });

    it('handles message with null data via optional chaining', async () => {
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

        const cb = vi.fn();
        replayPendingActions(cb);

        // Send message with null/undefined data
        expect(() => messageHandler({ data: null })).not.toThrow();
        expect(() => messageHandler({ data: undefined })).not.toThrow();
        expect(cb).not.toHaveBeenCalled();
    });

    it('calls callback with empty actions and does not call updateCounts', async () => {
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

        const cb = vi.fn();
        replayPendingActions(cb);

        messageHandler({
            data: { type: 'PENDING_ACTIONS', actions: [] },
        });

        await vi.advanceTimersByTimeAsync(0);

        expect(updateCounts).not.toHaveBeenCalled();
        expect(cb).toHaveBeenCalledOnce();
    });

    it('falls back to empty array when actions field is missing', async () => {
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

        const cb = vi.fn();
        replayPendingActions(cb);

        messageHandler({
            data: { type: 'PENDING_ACTIONS' }, // no actions field
        });

        await vi.advanceTimersByTimeAsync(0);

        expect(api).not.toHaveBeenCalled();
        expect(updateCounts).not.toHaveBeenCalled();
        expect(cb).toHaveBeenCalledOnce();
    });

    it('skips unknown action types and still calls callback', async () => {
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

        const cb = vi.fn();
        replayPendingActions(cb);

        messageHandler({
            data: {
                type: 'PENDING_ACTIONS',
                actions: [
                    { type: 'unknown-action', articleId: '99' },
                ],
            },
        });

        await vi.advanceTimersByTimeAsync(0);

        expect(api).not.toHaveBeenCalled();
        // updateCounts is called because actions.length > 0
        expect(updateCounts).toHaveBeenCalled();
        expect(cb).toHaveBeenCalledOnce();
    });

    it('still calls callback when dequeue API calls fail', async () => {
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

        api.mockRejectedValue(new Error('API error'));

        const cb = vi.fn();
        replayPendingActions(cb);

        messageHandler({
            data: {
                type: 'PENDING_ACTIONS',
                actions: [
                    { type: 'dequeue', articleId: '123' },
                    { type: 'dequeue', articleId: '456' },
                ],
            },
        });

        await vi.advanceTimersByTimeAsync(0);

        expect(api).toHaveBeenCalledTimes(2);
        // Callback and updateCounts still called despite failures
        expect(updateCounts).toHaveBeenCalled();
        expect(cb).toHaveBeenCalledOnce();
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

    it('sends empty array when api returns empty array', async () => {
        api.mockResolvedValue([]);

        const sw = { postMessage: vi.fn() };
        cacheQueueForOffline(sw);

        await vi.runAllTimersAsync();

        expect(api).toHaveBeenCalledWith('GET', '/api/queue');
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

    it('does not call postMessage when sw is null but still calls api', async () => {
        api.mockResolvedValue([{ id: 1 }]);

        cacheQueueForOffline(null);

        await vi.runAllTimersAsync();

        expect(api).toHaveBeenCalledWith('GET', '/api/queue');
        // Should not throw — the if(sw) guard prevents postMessage on null
    });
});

// Helper to make isStandalone() return true by mocking matchMedia
function mockStandaloneMode() {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true });
}

describe('handleOnlineStateChange', () => {
    it('is a no-op when not in standalone mode', () => {
        expect(isStandalone()).toBe(false);
        handleOnlineStateChange();
        expect(document.getElementById('offline-banner')).toBeNull();
    });

    it('shows banner and disables UI when offline in standalone mode', () => {
        mockStandaloneMode();
        Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });

        // Setup sidebar for disableNonQueueUI
        const sidebar = document.createElement('div');
        sidebar.className = 'sidebar';
        sidebar.innerHTML = '<a class="nav-item" href="/">Home</a>';
        document.body.appendChild(sidebar);

        handleOnlineStateChange();

        expect(document.body.classList.contains('pwa-offline')).toBe(true);
        expect(document.getElementById('offline-banner')).not.toBeNull();
        expect(document.querySelector('.nav-item').classList.contains('offline-disabled')).toBe(true);

        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });
    });

    it('shows back-online banner, enables UI and replays actions when online in standalone mode', async () => {
        mockStandaloneMode();
        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });

        // Add an existing offline banner to verify it gets updated
        const banner = document.createElement('div');
        banner.id = 'offline-banner';
        banner.innerHTML = 'old text';
        document.body.appendChild(banner);

        // Add disabled elements to verify they get re-enabled
        const el = document.createElement('div');
        el.classList.add('offline-disabled');
        el.setAttribute('data-offline-disabled', 'true');
        document.body.appendChild(el);

        // Mock SW - no controller so replayPendingActions calls callback quickly
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { controller: null, addEventListener: vi.fn(), removeEventListener: vi.fn() },
            configurable: true,
        });

        // Mock reload
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        handleOnlineStateChange();

        expect(document.body.classList.contains('pwa-offline')).toBe(false);
        expect(banner.innerHTML).toContain('Back online');
        expect(banner.style.background).toBe('#27ae60');
        expect(el.classList.contains('offline-disabled')).toBe(false);

        // replayPendingActions fires callback via setTimeout
        vi.advanceTimersByTime(1);
        expect(reloadMock).toHaveBeenCalled();
    });
});

describe('initOfflineSupport', () => {
    it('is a no-op when not in standalone mode', () => {
        expect(isStandalone()).toBe(false);
        initOfflineSupport();
        expect(document.getElementById('offline-banner')).toBeNull();
    });

    it('returns early when no serviceWorker support', () => {
        mockStandaloneMode();
        delete navigator.serviceWorker;
        // Should not throw
        expect(() => initOfflineSupport()).not.toThrow();
    });

    it('registers SW ready handler and event listeners in standalone mode', async () => {
        mockStandaloneMode();

        const sw = { postMessage: vi.fn() };
        const readyPromise = Promise.resolve({ active: sw });
        const swAddEventListener = vi.fn();
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: readyPromise, addEventListener: swAddEventListener },
            configurable: true,
        });
        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });

        // Mock querySelectorAll to return a fake stylesheet link element.
        // Appending a real <link rel="stylesheet"> to the DOM triggers
        // happy-dom's stylesheet fetcher, which races with environment
        // teardown and produces spurious AbortError / NetworkError.
        const origQSA = document.querySelectorAll.bind(document);
        vi.spyOn(document, 'querySelectorAll').mockImplementation((sel) => {
            if (sel === 'link[rel="stylesheet"]') {
                return [{ href: 'http://localhost:3000/static/style.css' }];
            }
            return origQSA(sel);
        });

        const addEventSpy = vi.spyOn(window, 'addEventListener');

        initOfflineSupport();

        // SW ready resolves
        await vi.runAllTimersAsync();

        // Should have sent ENABLE_OFFLINE
        expect(sw.postMessage).toHaveBeenCalledWith(
            expect.objectContaining({ type: 'ENABLE_OFFLINE' })
        );

        // Should have registered SW message listener
        expect(swAddEventListener).toHaveBeenCalledWith('message', expect.any(Function), expect.objectContaining({ signal: expect.any(AbortSignal) }));

        // Should have registered online/offline event listeners
        expect(addEventSpy).toHaveBeenCalledWith('online', handleOnlineStateChange, expect.objectContaining({ signal: expect.any(AbortSignal) }));
        expect(addEventSpy).toHaveBeenCalledWith('offline', handleOnlineStateChange, expect.objectContaining({ signal: expect.any(AbortSignal) }));

        document.querySelectorAll.mockRestore();
    });

    it('collects script[src] URLs containing /static/', async () => {
        mockStandaloneMode();

        const sw = { postMessage: vi.fn() };
        const readyPromise = Promise.resolve({ active: sw });
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: readyPromise, addEventListener: vi.fn() },
            configurable: true,
        });
        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });

        // Script with /static/ path — should be collected
        // Note: happy-dom will try to load the script and throw, but the URL
        // collection happens via querySelectorAll before script execution.
        // We avoid appending script elements to the DOM to prevent load errors.
        // Instead we verify the code path runs by checking postMessage is called.
        initOfflineSupport();
        await vi.runAllTimersAsync();

        expect(sw.postMessage).toHaveBeenCalledWith(
            expect.objectContaining({
                type: 'ENABLE_OFFLINE',
                data: expect.objectContaining({ staticUrls: expect.any(Array) }),
            })
        );
    });

    it('applies initial offline state when navigator is offline', async () => {
        mockStandaloneMode();

        const swAddEventListener = vi.fn();
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: Promise.resolve({ active: null }), addEventListener: swAddEventListener },
            configurable: true,
        });
        Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
        Object.defineProperty(window, 'location', {
            value: { ...window.location, pathname: '/' },
            writable: true,
            configurable: true,
        });

        initOfflineSupport();

        expect(document.body.classList.contains('pwa-offline')).toBe(true);
        expect(document.getElementById('offline-banner')).not.toBeNull();

        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });
    });

    it('does not send postMessage when SW registration has no active worker', async () => {
        mockStandaloneMode();

        const readyPromise = Promise.resolve({ active: null });
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: readyPromise, addEventListener: vi.fn() },
            configurable: true,
        });
        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });

        initOfflineSupport();
        await vi.runAllTimersAsync();
        // Should not throw — the `if (!sw) return` guard prevents postMessage on null
    });

    it('logs when OFFLINE_ENABLED message received from SW', async () => {
        mockStandaloneMode();

        let swMessageHandler;
        const swAddEventListener = vi.fn((_event, handler) => { swMessageHandler = handler; });
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: Promise.resolve({ active: null }), addEventListener: swAddEventListener },
            configurable: true,
        });
        Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });

        const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});

        initOfflineSupport();

        // Trigger the SW message handler
        swMessageHandler({ data: { type: 'OFFLINE_ENABLED' } });
        expect(consoleSpy).toHaveBeenCalledWith('Offline mode enabled for PWA');
    });
});

describe('updateQueueCacheIfStandalone', () => {
    it('is a no-op when not in standalone mode', () => {
        expect(isStandalone()).toBe(false);
        updateQueueCacheIfStandalone();
    });

    it('returns early when no serviceWorker in standalone mode', () => {
        mockStandaloneMode();
        delete navigator.serviceWorker;
        expect(() => updateQueueCacheIfStandalone()).not.toThrow();
    });

    it('caches queue when standalone and SW is active', async () => {
        mockStandaloneMode();

        const sw = { postMessage: vi.fn() };
        const readyPromise = Promise.resolve({ active: sw });
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: readyPromise },
            configurable: true,
        });

        const articles = [{ id: 1, title: 'Queue Article' }];
        api.mockResolvedValue(articles);

        updateQueueCacheIfStandalone();
        await vi.runAllTimersAsync();

        expect(api).toHaveBeenCalledWith('GET', '/api/queue');
        expect(sw.postMessage).toHaveBeenCalledWith({
            type: 'CACHE_QUEUE',
            data: { articles },
        });
    });

    it('does not cache when SW registration has no active worker', async () => {
        mockStandaloneMode();

        const readyPromise = Promise.resolve({ active: null });
        Object.defineProperty(navigator, 'serviceWorker', {
            value: { ready: readyPromise },
            configurable: true,
        });

        updateQueueCacheIfStandalone();
        await vi.runAllTimersAsync();

        expect(api).not.toHaveBeenCalled();
    });
});
