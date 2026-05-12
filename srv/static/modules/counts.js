import { api } from './api.js';
import { showToast } from './toast.js';
import { escapeHtml } from './utils.js';
import { applyUserPreferences } from './articles.js';
import { showFeedErrorBanner, removeFeedErrorBanner } from './feed-errors.js';

// Module-scope state for the visibility-aware poll loop.
let _countsPollAC = null;
let _countsPollTimer = null;

export async function updateCounts() {
    try {
        const counts = await api('GET', '/api/counts');

        // Update unread count
        const unreadBadge = document.querySelector('[data-count="unread"]');
        if (unreadBadge && Number.isFinite(counts.unread)) {
            unreadBadge.textContent = counts.unread || '';
            unreadBadge.setAttribute('aria-label', counts.unread ? `${counts.unread} unread articles` : '');
        }

        // Update starred count
        const starredBadge = document.querySelector('[data-count="starred"]');
        if (starredBadge && Number.isFinite(counts.starred)) {
            starredBadge.textContent = counts.starred || '';
        }

        // Update queue count
        const queueBadge = document.querySelector('[data-count="queue"]');
        if (queueBadge && Number.isFinite(counts.queue)) {
            queueBadge.textContent = counts.queue || '';
            queueBadge.setAttribute('aria-label', counts.queue ? `${counts.queue} queued articles` : '');
        }

        // Update alerts count
        const alertsBadge = document.querySelector('[data-count="alerts"]');
        if (alertsBadge && Number.isFinite(counts.alerts)) {
            alertsBadge.textContent = counts.alerts || '';
            alertsBadge.setAttribute('aria-label', counts.alerts ? `${counts.alerts} alerts` : '');
        }

        // Update category counts — zero all first, then set from response
        // (categories with 0 unread are omitted from the response)
        document.querySelectorAll('[data-count^="category-"]').forEach(badge => {
            badge.textContent = '';
            badge.setAttribute('aria-label', '');
        });
        if (counts.categories && typeof counts.categories === 'object' && !Array.isArray(counts.categories)) {
            for (const [catId, count] of Object.entries(counts.categories)) {
                const badge = document.querySelector(`[data-count="category-${catId}"]`);
                if (badge) {
                    badge.textContent = count || '';
                    badge.setAttribute('aria-label', count ? `${count} unread` : '');
                }
            }
        }

        // Update feed counts — zero all first, then set from response
        // (feeds with 0 unread are omitted from the response)
        document.querySelectorAll('[data-count^="feed-"]').forEach(badge => {
            if (!badge.classList.contains('pending')) {
                badge.textContent = '';
            }
        });
        if (counts.feeds && typeof counts.feeds === 'object' && !Array.isArray(counts.feeds)) {
            for (const [feedId, count] of Object.entries(counts.feeds)) {
                const badges = document.querySelectorAll(`[data-count="feed-${feedId}"]`);
                badges.forEach(badge => {
                    if (!badge.classList.contains('pending') || count > 0) {
                        badge.textContent = count || '';
                        badge.classList.remove('pending');
                        badge.setAttribute('aria-label', count ? `${count} unread` : '');
                    }
                });
            }
        }

        // Update feed errors
        updateFeedErrors(counts.feedErrors || {});

        // Re-apply hide empty preference if enabled
        applyUserPreferences();
    } catch (e) {
        console.error('Failed to update counts:', e.message);
        showToast('Failed to update counts');
    }
}

// Update status cell in the Manage Feeds table
export function updateFeedStatusCell(feedId, lastError) {
    const row = document.querySelector(`tr[data-feed-id="${feedId}"]`);
    if (!row) return;

    // Find the status cell (second to last column)
    const cells = row.querySelectorAll('td');
    const statusCell = cells[cells.length - 2]; // Status is before Actions
    if (!statusCell) return;

    if (lastError) {
        statusCell.innerHTML = `<span class="status-error" title="${escapeHtml(lastError)}">Error</span>`;
        row.dataset.hasError = 'true';
    } else {
        statusCell.innerHTML = '<span class="status-ok">OK</span>';
        delete row.dataset.hasError;
    }
}

export function updateFeedErrors(feedErrors) {
    // Get all feed items in sidebar
    const feedItems = document.querySelectorAll('.feed-item[data-feed-id]');

    feedItems.forEach(item => {
        const feedId = item.dataset.feedId;
        const errorIcon = item.querySelector('[data-error]');
        const hasError = feedErrors[feedId];

        if (hasError) {
            item.classList.add('has-error');
            item.title = 'Error: ' + feedErrors[feedId];
            if (errorIcon) {
                errorIcon.textContent = '⚠';
                errorIcon.title = 'Fetch error';
            }
        } else {
            item.classList.remove('has-error');
            item.title = '';
            if (errorIcon) {
                errorIcon.textContent = '';
                errorIcon.title = '';
            }
        }
    });

    // Also update the error banner on the current feed page if present
    const currentFeedId = document.querySelector('button[data-feed-id]')?.dataset.feedId;
    if (currentFeedId) {
        if (feedErrors[currentFeedId]) {
            showFeedErrorBanner(currentFeedId, feedErrors[currentFeedId]);
        } else {
            removeFeedErrorBanner();
        }
    }
}

// Initialize a visibility-aware background poll that calls updateCounts()
// every 60 seconds while the tab is visible and online. The interval is
// paused when the tab is hidden and resumed (with an immediate call) when
// the tab becomes visible again. The navigator.onLine guard prevents toast
// spam when the connection is flaky — only the poll path applies this guard;
// other callers of updateCounts are not changed.
export function initCountsPolling() {
    if (_countsPollAC) {
        _countsPollAC.abort();
        if (_countsPollTimer) {
            clearInterval(_countsPollTimer);
            _countsPollTimer = null;
        }
    }
    _countsPollAC = new AbortController();
    const signal = _countsPollAC.signal;

    const tick = () => {
        if (document.visibilityState !== 'visible') return;
        if (!navigator.onLine) return;
        updateCounts();
    };

    const startInterval = () => {
        _countsPollTimer = setInterval(tick, 60000);
    };

    const stopInterval = () => {
        if (_countsPollTimer) {
            clearInterval(_countsPollTimer);
            _countsPollTimer = null;
        }
    };

    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'hidden') {
            stopInterval();
        } else {
            tick();
            stopInterval();
            startInterval();
        }
    }, { signal });

    signal.addEventListener('abort', stopInterval);

    if (document.visibilityState === 'visible') {
        startInterval();
    }
}
