// Article action handlers: mark read/unread, star, queue, open, auto-mark-read.

import { api } from './api.js';
import { getSetting } from './settings.js';
import {
    SVG_STAR_FILLED, SVG_STAR_EMPTY, SVG_QUEUE_ADD, SVG_QUEUE_REMOVE
} from './icons.js';

// --- Late-bound dependencies (set by app.js during init) ---
let _updateReadButton = null;
let _updateCounts = null;
let _updateQueueCacheIfStandalone = null;

export function setArticleActionDeps({ updateReadButton, updateCounts, updateQueueCacheIfStandalone }) {
    if (updateReadButton) _updateReadButton = updateReadButton;
    if (updateCounts) _updateCounts = updateCounts;
    if (updateQueueCacheIfStandalone) _updateQueueCacheIfStandalone = updateQueueCacheIfStandalone;
}

// --- Queue state ---
export let queuedArticleIds = new Set();
export let queuedIdsReady = Promise.resolve();

export function setQueuedArticleIds(ids) {
    queuedArticleIds = ids;
}

export function setQueuedIdsReady(promise) {
    queuedIdsReady = promise;
}

// --- Auto-mark-read state ---
let autoMarkReadObserver = null;
let _markReadQueue = [];
let _markReadTimer = null;

// Test-only accessors for internal state (used by app.test.js during migration)
export function _getAutoMarkReadObserver() { return autoMarkReadObserver; }
export function _setAutoMarkReadObserver(v) { autoMarkReadObserver = v; }
export function _getMarkReadQueue() { return _markReadQueue; }
export function _resetArticleActionsState() {
    if (autoMarkReadObserver) { autoMarkReadObserver.disconnect(); autoMarkReadObserver = null; }
    _markReadQueue = [];
    if (_markReadTimer) { clearTimeout(_markReadTimer); _markReadTimer = null; }
    queuedArticleIds = new Set();
    queuedIdsReady = Promise.resolve();
}

export function initAutoMarkRead() {
    // Disconnect any previous observer
    if (autoMarkReadObserver) {
        autoMarkReadObserver.disconnect();
        autoMarkReadObserver = null;
    }

    if (getSetting('autoMarkRead') !== 'true') {
        console.debug('[auto-mark-read] disabled by setting');
        return;
    }

    // Use IntersectionObserver to detect when articles scroll out of view
    autoMarkReadObserver = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            // Article has scrolled up and out of view
            if (!entry.isIntersecting && entry.boundingClientRect.top < 0) {
                const article = entry.target;
                const articleId = article.dataset.id;

                // Only mark unread articles
                if (articleId && !article.classList.contains('read')) {
                    console.debug(`[auto-mark-read] marking article ${articleId} as read (scrolled out of view)`);
                    markReadSilent(articleId);
                    article.classList.add('read');
                }
            }
        });
    }, {
        root: null,
        rootMargin: '0px',
        threshold: 0
    });

    // Observe all article cards
    const cards = document.querySelectorAll('.article-card');
    console.debug(`[auto-mark-read] observing ${cards.length} initial articles`);
    cards.forEach(article => {
        autoMarkReadObserver.observe(article);
    });
}

// Observe newly added article cards (e.g. from pagination)
export function observeNewArticles(container) {
    if (!autoMarkReadObserver) return;
    const cards = container.querySelectorAll('.article-card');
    if (cards.length > 0) {
        console.debug(`[auto-mark-read] observing ${cards.length} new articles`);
        cards.forEach(article => autoMarkReadObserver.observe(article));
    }
}

export function flushMarkReadQueue() {
    _markReadTimer = null;
    if (_markReadQueue.length === 0) return;
    const ids = _markReadQueue.slice();
    _markReadQueue = [];
    console.debug(`[auto-mark-read] flushing batch of ${ids.length} article(s):`, ids);
    api('POST', '/api/articles/batch-read', { ids })
        .then(() => { if (_updateCounts) _updateCounts(); })
        .catch(e => console.error('Failed to batch mark read:', e));
}

// Mark as read in the DOM without page reload (for auto-mark feature)
export function markCardAsRead(id) {
    const card = document.querySelector(`.article-card[data-id="${id}"]`);
    if (card) {
        card.classList.add('read');
        if (_updateReadButton) _updateReadButton(card, true);
    }
}

export function markReadSilent(id) {
    markCardAsRead(id);
    _markReadQueue.push(Number(id));
    if (_markReadTimer) clearTimeout(_markReadTimer);
    _markReadTimer = setTimeout(flushMarkReadQueue, 250);
}

export function openArticle(id) {
    markReadSilent(id);
    flushMarkReadQueue();
    window.location = `/article/${id}`;
}

export function openArticleExternal(event, id, url) {
    event.stopPropagation();
    markReadSilent(id);
    window.open(url, '_blank');
}

// --- Standard article actions (API calls) ---

export async function markRead(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/read`);
        markCardAsRead(id);
        if (_updateCounts) _updateCounts();
    } catch (e) {
        console.error('Failed to mark read:', e);
    }
}

export async function markUnread(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/unread`);
        const card = document.querySelector(`.article-card[data-id="${id}"]`);
        if (card) {
            card.classList.remove('read');
            if (_updateReadButton) _updateReadButton(card, false);
        }
        if (_updateCounts) _updateCounts();
    } catch (e) {
        console.error('Failed to mark unread:', e);
    }
}

export async function toggleStar(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/star`);
        // Toggle star button appearance
        const btns = document.querySelectorAll(`[onclick="toggleStar(event, ${id})"]`);
        btns.forEach(btn => {
            const isNowStarred = !btn.classList.contains('starred');
            btn.classList.toggle('starred', isNowStarred);
            btn.title = isNowStarred ? 'Unstar' : 'Star';
            btn.innerHTML = isNowStarred ? SVG_STAR_FILLED : SVG_STAR_EMPTY;
        });
        if (_updateCounts) _updateCounts();
    } catch (e) {
        console.error('Failed to toggle star:', e);
    }
}

export async function toggleQueue(event, id) {
    if (event) event.stopPropagation();
    try {
        const resp = await api('POST', `/api/articles/${id}/queue`);
        const isNowQueued = resp.queued;
        if (isNowQueued) {
            queuedArticleIds.add(id);
        } else {
            queuedArticleIds.delete(id);
        }
        // Toggle queue button appearance
        const btns = document.querySelectorAll(`[onclick="toggleQueue(event, ${id})"]`);
        btns.forEach(btn => {
            btn.classList.toggle('queued', isNowQueued);
            btn.title = isNowQueued ? 'Remove from queue' : 'Add to queue';
            btn.innerHTML = isNowQueued ? SVG_QUEUE_REMOVE : SVG_QUEUE_ADD;
        });
        if (_updateCounts) _updateCounts();
        if (_updateQueueCacheIfStandalone) _updateQueueCacheIfStandalone();
    } catch (e) {
        console.error('Failed to toggle queue:', e);
    }
}

// Mark all articles as read (with optional age filter)
export async function markAsRead(btn, age = 'all') {
    const dropdown = btn.closest('.dropdown');
    const feedId = dropdown.dataset.feedId;
    const categoryId = dropdown.dataset.categoryId;

    try {
        let url;
        if (feedId) {
            url = `/api/feeds/${feedId}/read-all?age=${age}`;
        } else if (categoryId) {
            url = `/api/categories/${categoryId}/read-all?age=${age}`;
        } else {
            url = `/api/articles/read-all?age=${age}`;
        }

        await api('POST', url);
        document.querySelectorAll('.dropdown.open').forEach(d => d.classList.remove('open'));

        // After marking a folder as read, navigate to the next folder with unread articles
        if (categoryId) {
            const nextUrl = findNextUnreadFolder(categoryId);
            if (nextUrl) {
                window.location.href = nextUrl;
                return;
            }
        }
        location.reload();
    } catch (e) {
        console.error('Failed to mark as read:', e);
    }
}

// Find the next folder in sidebar order that has unread articles
export function findNextUnreadFolder(currentCategoryId) {
    const allFolders = Array.from(document.querySelectorAll('.folder-item[data-category-id]'));
    const currentIdx = allFolders.findIndex(f => f.dataset.categoryId === String(currentCategoryId));
    if (currentIdx === -1) return null;

    // Search from current+1 to end, then wrap from start to current
    const ordered = [...allFolders.slice(currentIdx + 1), ...allFolders.slice(0, currentIdx)];
    for (const folder of ordered) {
        const catId = folder.dataset.categoryId;
        const badge = document.querySelector(`[data-count="category-${catId}"]`);
        const count = badge ? parseInt(badge.textContent.trim(), 10) : 0;
        if (count > 0) {
            return `/category/${catId}`;
        }
    }
    return null;
}
