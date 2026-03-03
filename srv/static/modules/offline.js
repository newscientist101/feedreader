// Offline / PWA standalone-mode support.
// Handles service-worker setup, online/offline banners, and pending-action replay.

import { api } from './api.js';
import { updateCounts } from './counts.js';

// Detect PWA standalone mode
export function isStandalone() {
    return (typeof window.matchMedia === 'function' &&
        window.matchMedia('(display-mode: standalone)').matches) ||
        window.navigator.standalone === true;
}

export function initOfflineSupport() {
    if (!isStandalone()) return;
    if (!('serviceWorker' in navigator)) return;

    // Wait for SW to be ready
    navigator.serviceWorker.ready.then((reg) => {
        const sw = reg.active;
        if (!sw) return;

        // Enable offline mode in the SW
        const staticUrls = [];
        document.querySelectorAll('link[rel="stylesheet"]').forEach(l => {
            if (l.href) staticUrls.push(l.href);
        });
        document.querySelectorAll('script[src]').forEach(s => {
            if (s.src && s.src.includes('/static/')) staticUrls.push(s.src);
        });
        sw.postMessage({ type: 'ENABLE_OFFLINE', data: { staticUrls } });

        // Cache queue articles
        cacheQueueForOffline(sw);
    });

    // Listen for SW messages
    navigator.serviceWorker.addEventListener('message', (event) => {
        if (event.data?.type === 'OFFLINE_ENABLED') {
            console.log('Offline mode enabled for PWA');
        }
    });

    // Monitor online/offline state
    window.addEventListener('online', handleOnlineStateChange);
    window.addEventListener('offline', handleOnlineStateChange);
    // Apply initial offline state if needed (without triggering reload)
    if (!navigator.onLine) {
        document.body.classList.add('pwa-offline');
        showOfflineBanner();
        disableNonQueueUI();
    }
}

export function cacheQueueForOffline(sw) {
    // Fetch queue articles and send to SW for caching
    api('GET', '/api/queue').then(articles => {
        if (sw) {
            sw.postMessage({ type: 'CACHE_QUEUE', data: { articles: articles || [] } });
        }
    }).catch(() => {});
}

export function handleOnlineStateChange() {
    if (!isStandalone()) return;

    const isOffline = !navigator.onLine;
    document.body.classList.toggle('pwa-offline', isOffline);

    if (isOffline) {
        showOfflineBanner();
        disableNonQueueUI();
    } else {
        // This only fires on a real offline\u2192online transition (from the
        // 'online' event), never on initial page load.
        const banner = document.getElementById('offline-banner');
        if (banner) {
            banner.style.background = '#27ae60';
            banner.innerHTML =
                '<svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">' +
                '<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>' +
                '</svg> Back online \u2014 reloading\u2026';
        }
        enableAllUI();
        replayPendingActions(() => {
            window.location.reload();
        });
    }
}

export function showOfflineBanner() {
    if (document.getElementById('offline-banner')) return;
    const banner = document.createElement('div');
    banner.id = 'offline-banner';
    banner.className = 'offline-banner';
    banner.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">' +
        '<path d="M19.35 10.04C18.67 6.59 15.64 4 12 4c-1.48 0-2.85.43-4.01 1.17l1.46 1.46C10.21 6.23 11.08 6 12 6c3.04 0 5.5 2.46 5.5 5.5v.5H19c1.66 0 3 1.34 3 3 0 .99-.49 1.87-1.24 2.41l1.46 1.46C23.33 17.98 24 16.58 24 15c0-2.64-2.05-4.78-4.65-4.96zM3 5.27l2.75 2.74C2.56 8.15 0 10.77 0 14c0 3.31 2.69 6 6 6h11.73l2 2 1.27-1.27L4.27 4 3 5.27zM7.73 10l8 8H6c-2.21 0-4-1.79-4-4s1.79-4 4-4h1.73z"/>' +
        '</svg> ' +
        'You\'re offline' +
        (window.location.pathname !== '/queue' ? ' \u2014 <a href="/queue" style="color:#fff;text-decoration:underline">Go to Queue</a>' : '');
    document.body.prepend(banner);
}

export function disableNonQueueUI() {
    const isQueuePage = window.location.pathname === '/queue';

    // Disable sidebar links except queue
    document.querySelectorAll('.sidebar .nav-item, .sidebar .feed-item, .sidebar .folder-item').forEach(el => {
        const href = el.getAttribute('href');
        if (href === '/queue') return;
        el.classList.add('offline-disabled');
        el.setAttribute('data-offline-disabled', 'true');
    });

    // Disable sidebar footer sections (scrapers, settings, user info)
    document.querySelectorAll('.sidebar .feeds-section, .sidebar .feeds-header').forEach(el => {
        el.classList.add('offline-disabled');
        el.setAttribute('data-offline-disabled', 'true');
    });

    // On non-queue pages, show overlay
    if (!isQueuePage) {
        const content = document.querySelector('.content');
        if (content && !document.getElementById('offline-overlay')) {
            const overlay = document.createElement('div');
            overlay.id = 'offline-overlay';
            overlay.className = 'offline-overlay';
            overlay.innerHTML = '<div class="offline-overlay-content">' +
                '<svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor">' +
                '<path d="M19.35 10.04C18.67 6.59 15.64 4 12 4c-1.48 0-2.85.43-4.01 1.17l1.46 1.46C10.21 6.23 11.08 6 12 6c3.04 0 5.5 2.46 5.5 5.5v.5H19c1.66 0 3 1.34 3 3 0 .99-.49 1.87-1.24 2.41l1.46 1.46C23.33 17.98 24 16.58 24 15c0-2.64-2.05-4.78-4.65-4.96zM3 5.27l2.75 2.74C2.56 8.15 0 10.77 0 14c0 3.31 2.69 6 6 6h11.73l2 2 1.27-1.27L4.27 4 3 5.27zM7.73 10l8 8H6c-2.21 0-4-1.79-4-4s1.79-4 4-4h1.73z"/>' +
                '</svg>' +
                '<h2>You\'re offline</h2>' +
                '<p>This section is not available offline.</p>' +
                '<a href="/queue" class="btn btn-primary">Go to Reading Queue</a>' +
                '</div>';
            content.style.position = 'relative';
            content.appendChild(overlay);
        }
    }
}

export function enableAllUI() {
    document.querySelectorAll('[data-offline-disabled]').forEach(el => {
        el.classList.remove('offline-disabled');
        el.removeAttribute('data-offline-disabled');
    });
    const overlay = document.getElementById('offline-overlay');
    if (overlay) overlay.remove();
}

export function replayPendingActions(callback) {
    if (!('serviceWorker' in navigator) || !navigator.serviceWorker.controller) {
        if (callback) setTimeout(callback, 0);
        return;
    }

    const handler = (event) => {
        if (event.data?.type !== 'PENDING_ACTIONS') return;
        navigator.serviceWorker.removeEventListener('message', handler);

        const actions = event.data.actions || [];
        const promises = actions.map((action) => {
            if (action.type === 'dequeue') {
                return api('DELETE', `/api/articles/${action.articleId}/queue`).catch(() => {});
            }
            return Promise.resolve();
        });
        Promise.all(promises).then(() => {
            if (actions.length > 0) updateCounts();
            if (callback) callback();
        });
    };
    navigator.serviceWorker.addEventListener('message', handler);
    navigator.serviceWorker.controller.postMessage({ type: 'GET_PENDING_ACTIONS' });
    // Safety timeout: if SW doesn't respond within 3s, proceed anyway
    setTimeout(() => {
        navigator.serviceWorker.removeEventListener('message', handler);
        if (callback) callback();
    }, 3000);
}

// Update queue cache when articles change
export function updateQueueCacheIfStandalone() {
    if (!isStandalone()) return;
    if (!('serviceWorker' in navigator)) return;
    navigator.serviceWorker.ready.then((reg) => {
        if (reg.active) cacheQueueForOffline(reg.active);
    });
}
