// Article action handlers: mark read/unread, star, queue, open, auto-mark-read.

import { api } from './api.js';
import { showToast } from './toast.js';
import { getSetting } from './settings.js';
import {
    SVG_STAR_FILLED, SVG_STAR_EMPTY, SVG_QUEUE_ADD, SVG_QUEUE_REMOVE
} from './icons.js';
import { updateCounts } from './counts.js';
import { updateQueueCacheIfStandalone } from './offline.js';
import { updateReadButton } from './read-button.js';

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
let _autoMarkReadAC = null;
let _markReadQueue = [];
let _markReadTimer = null;
let _actionListenerAC = null;

// Test-only accessors for internal state (used by app.test.js during migration)
export function _getAutoMarkReadObserver() { return autoMarkReadObserver; }
export function _setAutoMarkReadObserver(v) { autoMarkReadObserver = v; }
export function _getMarkReadQueue() { return _markReadQueue; }
export function _resetArticleActionsState() {
    if (autoMarkReadObserver) { autoMarkReadObserver.disconnect(); autoMarkReadObserver = null; }
    if (_autoMarkReadAC) { _autoMarkReadAC.abort(); _autoMarkReadAC = null; }
    _markReadQueue = [];
    if (_markReadTimer) { clearTimeout(_markReadTimer); _markReadTimer = null; }
    queuedArticleIds = new Set();
    queuedIdsReady = Promise.resolve();
    if (_actionListenerAC) { _actionListenerAC.abort(); _actionListenerAC = null; }
}

// Load queued article IDs from the API, then hydrate action-button placeholders
// in server-rendered article cards.
export function initQueueState(renderArticleActions) {
    const _queueReady = api('GET', '/api/queue').then(articles => {
        queuedArticleIds = new Set((articles || []).map(a => a.id));
    }).catch(() => {});
    queuedIdsReady = _queueReady;
    _queueReady.then(() => {
        document.querySelectorAll('.article-actions-placeholder').forEach(el => {
            const a = {
                id: Number(el.dataset.articleId),
                is_read: el.dataset.isRead === '1',
                is_starred: el.dataset.isStarred === '1',
                is_queued: el.dataset.isQueued === '1' || queuedArticleIds.has(Number(el.dataset.articleId)),
                url: el.dataset.url || null,
            };
            el.outerHTML = renderArticleActions(a);
        });
    });
}

export function initAutoMarkRead() {
    // Disconnect any previous observer and abort any previous scroll listener
    if (autoMarkReadObserver) {
        autoMarkReadObserver.disconnect();
        autoMarkReadObserver = null;
    }
    if (_autoMarkReadAC) {
        _autoMarkReadAC.abort();
        _autoMarkReadAC = null;
    }

    if (getSetting('autoMarkRead') !== 'true') {
        console.debug('[auto-mark-read] disabled by setting');
        return;
    }

    _autoMarkReadAC = new AbortController();
    const signal = _autoMarkReadAC.signal;

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

    // Mark the last article when user scrolls to the bottom of the page.
    // The IntersectionObserver only fires when articles scroll OUT of view,
    // so the last article (which stays visible at the bottom) is never caught.
    const onScroll = () => {
        const scrollBottom = window.innerHeight + window.scrollY;
        if (scrollBottom < document.body.offsetHeight - 50) return;

        // User is at the bottom — mark the last unread article card as read.
        const unreadCards = document.querySelectorAll('#articles-list .article-card:not(.read)');
        if (unreadCards.length === 0) return;
        const last = unreadCards[unreadCards.length - 1];
        const rect = last.getBoundingClientRect();
        // Only mark if the card is at least partially visible in the viewport.
        if (rect.top < window.innerHeight && rect.bottom > 0) {
            const articleId = last.dataset.id;
            if (articleId) {
                console.debug(`[auto-mark-read] marking last article ${articleId} as read (scrolled to bottom)`);
                markReadSilent(articleId);
                last.classList.add('read');
            }
        }
    };
    window.addEventListener('scroll', onScroll, { passive: true, signal });
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
        .then(() => { updateCounts(); })
        .catch(e => { console.error('Failed to batch mark read:', e); showToast('Failed to sync read status'); });
}

// Mark as read in the DOM without page reload (for auto-mark feature)
export function markCardAsRead(id) {
    const card = document.querySelector(`.article-card[data-id="${id}"]`);
    if (card) {
        card.classList.add('read');
        updateReadButton(card, true);
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
    window.location = `/article/${id}`;
    flushMarkReadQueue();
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
        updateCounts();
    } catch (e) {
        console.error('Failed to mark read:', e);
        showToast('Failed to mark as read');
    }
}

export async function markUnread(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/unread`);
        const card = document.querySelector(`.article-card[data-id="${id}"]`);
        if (card) {
            card.classList.remove('read');
            updateReadButton(card, false);
        }
        updateCounts();
    } catch (e) {
        console.error('Failed to mark unread:', e);
        showToast('Failed to mark as unread');
    }
}

export async function toggleStar(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/star`);
        // Toggle star button appearance
        const btns = document.querySelectorAll(`[data-action="toggle-star"][data-article-id="${id}"]`);
        btns.forEach(btn => {
            const isNowStarred = !btn.classList.contains('starred');
            btn.classList.toggle('starred', isNowStarred);
            btn.title = isNowStarred ? 'Unstar' : 'Star';
            btn.setAttribute('aria-label', isNowStarred ? 'Unstar' : 'Star');
            btn.innerHTML = isNowStarred ? SVG_STAR_FILLED : SVG_STAR_EMPTY;
        });
        updateCounts();
    } catch (e) {
        console.error('Failed to toggle star:', e);
        showToast('Failed to toggle star');
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
        const btns = document.querySelectorAll(`[data-action="toggle-queue"][data-article-id="${id}"]`);
        btns.forEach(btn => {
            btn.classList.toggle('queued', isNowQueued);
            btn.title = isNowQueued ? 'Remove from queue' : 'Add to queue';
            btn.setAttribute('aria-label', isNowQueued ? 'Remove from queue' : 'Add to queue');
            btn.innerHTML = isNowQueued ? SVG_QUEUE_REMOVE : SVG_QUEUE_ADD;
        });
        updateCounts();
        updateQueueCacheIfStandalone();
    } catch (e) {
        console.error('Failed to toggle queue:', e);
        showToast('Failed to update queue');
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
        showToast('Failed to mark as read');
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

// Delegated listeners for article actions (replaces inline onclick handlers in
// index.html and dynamically-built article HTML).
export function initArticleActionListeners() {
    // Abort previous listeners to prevent duplicates across re-inits
    if (_actionListenerAC) _actionListenerAC.abort();
    _actionListenerAC = new AbortController();
    const signal = _actionListenerAC.signal;
    // data-action="mark-as-read" with data-scope on dropdown menu buttons
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="mark-as-read"]');
        if (btn) {
            markAsRead(btn, btn.dataset.scope || 'all');
        }
    }, { signal });

    // Article action buttons: toggle-read, toggle-star, toggle-queue
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="toggle-read"], [data-action="toggle-star"], [data-action="toggle-queue"]');
        if (!btn) return;
        const articleId = Number(btn.dataset.articleId);
        if (!articleId) return;
        switch (btn.dataset.action) {
            case 'toggle-read':
                if (btn.dataset.isRead === '1') {
                    markUnread(e, articleId);
                } else {
                    markRead(e, articleId);
                }
                break;
            case 'toggle-star':
                toggleStar(e, articleId);
                break;
            case 'toggle-queue':
                toggleQueue(e, articleId);
                break;
        }
    }, { signal });

    // Article body click — open article (delegated on .article-body.clickable)
    document.addEventListener('click', (e) => {
        const body = e.target.closest('.article-body.clickable');
        if (!body) return;
        // Don't handle if the click target is a link, button, or inside article-actions
        if (e.target.closest('a, button, .article-actions')) return;
        const card = body.closest('.article-card');
        if (card) {
            openArticle(Number(card.dataset.id));
        }
    }, { signal });

    // Article title link with external URL — open externally
    document.addEventListener('click', (e) => {
        const link = e.target.closest('.article-title a[data-action="open-external"]');
        if (link) {
            const card = link.closest('.article-card');
            if (card) {
                openArticleExternal(e, Number(card.dataset.id), link.href);
            }
        }
    }, { signal });

    // Article title link without URL — mark read silently
    document.addEventListener('click', (e) => {
        const link = e.target.closest('.article-title a[data-action="mark-read-silent"]');
        if (link) {
            const card = link.closest('.article-card');
            if (card) {
                markReadSilent(Number(card.dataset.id));
            }
        }
    }, { signal });

    // Feed name links inside article cards — just stop propagation
    // so the article-body click handler doesn't fire
    document.addEventListener('click', (e) => {
        const feedLink = e.target.closest('.article-card .article-meta .feed-name');
        if (feedLink) {
            e.stopPropagation();
        }
    }, { capture: true, signal });

    // Expanded content preview — open the article page (which also marks
    // read silently). Stop propagation so we don't double-fire from the
    // article-body click handler.
    document.addEventListener('click', (e) => {
        const preview = e.target.closest('.article-content-preview');
        if (preview) {
            // Don't navigate if the user clicked a link inside the preview
            if (e.target.closest('a, button')) return;
            e.stopPropagation();
            const card = preview.closest('.article-card');
            if (card) {
                openArticle(Number(card.dataset.id));
            }
        }
    }, { capture: true, signal });
}
