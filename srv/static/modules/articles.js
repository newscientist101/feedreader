// Article rendering: card HTML, article list, embeds, user preferences.

import { api } from './api.js';
import { showToast } from './toast.js';
import { formatTimeAgo, stripHtml, truncateText, escapeHtml, PREVIEW_TEXT_LIMIT } from './utils.js';
import {
    SVG_MARK_READ, SVG_MARK_UNREAD, SVG_STAR_FILLED, SVG_STAR_EMPTY,
    SVG_EXTERNAL, SVG_QUEUE_ADD, SVG_QUEUE_REMOVE
} from './icons.js';
import { getSetting, applyHideReadArticles, applyHideEmptyFeeds } from './settings.js';
import { queuedArticleIds, queuedIdsReady, initAutoMarkRead } from './article-actions.js';
// Direct import from pagination — this is a circular import (pagination also
// imports from this module) but works fine with ES modules because all exports
// are function declarations (hoisted and available before module body runs).
import {
    updatePaginationCursor, updateEndOfArticlesIndicator, setPaginationState
} from './pagination.js';

// Temporary flag: when true, the current view includes read/hidden articles.
// Reset on the next navigation.
let showingHiddenArticles = false;
let _articleListListenerAC = null;

export function getShowingHiddenArticles() {
    return showingHiddenArticles;
}

export function setShowingHiddenArticles(val) {
    showingHiddenArticles = val;
}

export function _resetArticlesState() {
    showingHiddenArticles = false;
    if (_articleListListenerAC) { _articleListListenerAC.abort(); _articleListListenerAC = null; }
}

// Render the standard set of action buttons for an article.
// `a` must have: id, is_read, is_starred, url (optional), is_queued (optional).
export function renderArticleActions(a) {
    const readBtn = `<button data-action="toggle-read" data-article-id="${a.id}" data-is-read="${a.is_read ? '1' : '0'}" class="btn-icon btn-read-toggle" title="${a.is_read ? 'Mark unread' : 'Mark read'}" aria-label="${a.is_read ? 'Mark unread' : 'Mark read'}">
        ${a.is_read ? SVG_MARK_UNREAD : SVG_MARK_READ}
    </button>`;
    const starBtn = `<button data-action="toggle-star" data-article-id="${a.id}" class="btn-icon ${a.is_starred ? 'starred' : ''}" title="${a.is_starred ? 'Unstar' : 'Star'}" aria-label="${a.is_starred ? 'Unstar' : 'Star'}">
        ${a.is_starred ? SVG_STAR_FILLED : SVG_STAR_EMPTY}
    </button>`;
    const queueBtn = `<button data-action="toggle-queue" data-article-id="${a.id}" class="btn-icon btn-queue-toggle ${a.is_queued ? 'queued' : ''}" title="${a.is_queued ? 'Remove from queue' : 'Add to queue'}" aria-label="${a.is_queued ? 'Remove from queue' : 'Add to queue'}">
        ${a.is_queued ? SVG_QUEUE_REMOVE : SVG_QUEUE_ADD}
    </button>`;
    const extBtn = a.url ? `<a href="${escapeHtml(a.url)}" target="_blank" class="btn-icon" title="Open original" aria-label="Open original">${SVG_EXTERNAL}</a>` : '';
    return `<div class="article-actions">${readBtn}${starBtn}${queueBtn}${extBtn}</div>`;
}

// Re-export updateReadButton from the shared leaf module so existing
// consumers (app.js, tests) can still import from articles.js.
export { updateReadButton } from './read-button.js';

// Apply user preferences from settings
export function applyUserPreferences() {
    // When explicitly showing hidden/read articles (or during search),
    // skip hiding read articles — the user asked to see them.
    if (!showingHiddenArticles) {
        applyHideReadArticles(getSetting('hideReadArticles'));
    }
    updateAllReadMessage();
    applyHideEmptyFeeds(getSetting('hideEmptyFeeds'));
}

// Show message when all articles are read and hidden
export function updateAllReadMessage() {
    const articlesList = document.getElementById('articles-list');
    if (!articlesList) return;

    // Remove existing message if any
    const existingMsg = document.getElementById('all-read-message');
    if (existingMsg) existingMsg.remove();

    const hideRead = getSetting('hideReadArticles') === 'hide';
    if (!hideRead || showingHiddenArticles) return;

    // Check if there are articles but all are hidden
    const allCards = articlesList.querySelectorAll('.article-card');
    const visibleCards = articlesList.querySelectorAll('.article-card:not([style*="display: none"])');

    if (allCards.length > 0 && visibleCards.length === 0) {
        const msg = document.createElement('div');
        msg.id = 'all-read-message';
        msg.className = 'empty-state';
        msg.innerHTML = `
            <svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor" opacity="0.3">
                <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
            </svg>
            <p>All caught up!</p>
            <p class="hint">All ${allCards.length} article${allCards.length === 1 ? '' : 's'} in this view have been read.</p>
            <button data-action="show-read-articles" class="btn btn-secondary" style="margin-top: 10px;">Show read articles</button>
        `;
        articlesList.appendChild(msg);
    }
}

// Temporarily show read articles
export function showReadArticles() {
    document.querySelectorAll('.article-card.read').forEach(card => {
        card.style.display = '';
    });
    const msg = document.getElementById('all-read-message');
    if (msg) msg.remove();
}

// Temporarily show hidden articles by re-fetching with include_read=1
export async function showHiddenArticles() {
    showingHiddenArticles = true;
    const url = getIncludeReadUrl();
    if (!url) return;
    try {
        const data = await api('GET', url);
        renderArticles(data.articles || []);
    } catch (e) {
        console.error('Failed to load hidden articles:', e);
        showToast('Failed to load hidden articles');
    }
}

// Build the API URL for the current view with include_read=1
export function getIncludeReadUrl() {
    const path = window.location.pathname;
    const feedMatch = path.match(/^\/feed\/(\d+)/);
    if (feedMatch) return `/api/feeds/${feedMatch[1]}/articles?include_read=1`;
    const catMatch = path.match(/^\/category\/(\d+)/);
    if (catMatch) return `/api/categories/${catMatch[1]}/articles?include_read=1`;
    if (path === '/') return '/api/articles/unread?include_read=1';
    return null;
}

// Show loading spinner in the articles list
export function showArticlesLoading() {
    showingHiddenArticles = false;
    const list = document.getElementById('articles-list');
    if (list) {
        list.setAttribute('aria-busy', 'true');
        list.innerHTML = `
            <div class="loading-state">
                <div class="spinner"></div>
                <p>Loading articles...</p>
            </div>
        `;
    }
}

export function buildArticleCardHtml(a) {
    a.is_queued = queuedArticleIds.has(a.id);
    const safeTitle = escapeHtml(a.title);
    const safeFeedName = escapeHtml(a.feed_name || '');
    const safeAuthor = escapeHtml(a.author);
    const safeUrl = escapeHtml(a.url);
    const safeImageUrl = escapeHtml(a.image_url);
    const safeSummary = escapeHtml(truncateText(stripHtml(a.summary), PREVIEW_TEXT_LIMIT));
    const safeContent = escapeHtml(truncateText(stripHtml(a.content), PREVIEW_TEXT_LIMIT));
    return `
        <article class="article-card ${a.is_read ? 'read' : ''}${a.image_url ? ' has-image' : ''}" data-id="${a.id}" data-sort-time="${escapeHtml(a.published_at || a.fetched_at)}">
            ${a.image_url ? `<div class="article-image magazine-expanded-only"><img src="${safeImageUrl}" alt="" loading="lazy"></div>` : `<div class="article-image-placeholder magazine-only">
                <svg viewBox="0 0 24 24" width="32" height="32" fill="currentColor">
                    <path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z"/>
                </svg>
            </div>`}
            <div class="article-body clickable">
                <div class="article-meta">
                    <a class="feed-name" href="/feed/${a.feed_id}">${safeFeedName}</a>
                    ${a.author ? `<span class="article-author">${safeAuthor}</span>` : ''}
                    <span class="article-date">${formatTimeAgo(a.published_at)}</span>
                </div>
                <h2 class="article-title">
                    ${a.url ? `<a href="${safeUrl}" target="_blank" data-action="open-external">${safeTitle}</a>` : `<a href="/article/${a.id}" data-action="mark-read-silent">${safeTitle}</a>`}
                </h2>
                ${a.summary ? `<p class="article-summary">${safeSummary}</p>` : (a.content ? `<p class="article-summary">${safeContent}</p>` : '')}
                ${a.content ? `<div class="article-content-preview expanded-only">${safeContent}</div>` : (a.summary ? `<div class="article-content-preview expanded-only">${safeSummary}</div>` : '')}
                ${renderArticleActions(a)}
            </div>
        </article>
    `;
}

export async function renderArticles(articles) {
    await queuedIdsReady;
    const list = document.getElementById('articles-list');
    if (!list) return;

    // Scroll to top when loading a new set of articles
    window.scrollTo(0, 0);

    // Reset pagination for fresh render (cursor-based)
    setPaginationState({ cursorTime: null, cursorId: null, done: false, loading: false });

    const PAGE_SIZE = 50;

    if (!articles || articles.length === 0) {
        const showBtn = !showingHiddenArticles
            ? `<button data-action="show-hidden-articles" class="btn btn-secondary" style="margin-top: 10px;">Show hidden articles</button>`
            : '';
        list.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor" opacity="0.3">
                    <path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
                </svg>
                <p>No articles to show</p>
                ${showBtn}
            </div>
        `;
        setPaginationState({ done: true });
        updateEndOfArticlesIndicator();
        list.setAttribute('aria-busy', 'false');
        return;
    }

    if (articles.length < PAGE_SIZE) {
        setPaginationState({ done: true });
    }

    // Set cursor from last article for next pagination request
    updatePaginationCursor(articles);

    list.innerHTML = articles.map(buildArticleCardHtml).join('');

    // Process embeds in expanded view content previews
    list.querySelectorAll('.article-content-preview').forEach(el => processEmbeds(el));

    // Re-apply user preferences (hide read, etc.)
    applyUserPreferences();
    updateEndOfArticlesIndicator();

    // Re-initialize auto-mark-read observer for new article set
    initAutoMarkRead();

    list.setAttribute('aria-busy', 'false');
}

export function processEmbeds(container) {
    if (!container) return;

    // Process video embeds: <video data-embed-type="video" data-src="...">
    container.querySelectorAll('video[data-embed-type="video"]').forEach(el => {
        const src = el.getAttribute('data-src') || '';
        const videoId = extractYouTubeId(src);
        if (videoId) {
            const wrapper = document.createElement('div');
            wrapper.className = 'embed-video';
            wrapper.innerHTML = `<iframe src="https://www.youtube.com/embed/${videoId}" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`;
            el.replaceWith(wrapper);
        }
    });

    // Process tweet embeds: <div data-embed-type="tweet">
    const tweetEls = container.querySelectorAll('[data-embed-type="tweet"]');
    if (tweetEls.length > 0) {
        tweetEls.forEach(el => {
            // The blockquote is already inside; Twitter's widget JS will pick it up
            const blockquote = el.querySelector('blockquote.twitter-tweet');
            if (blockquote) {
                el.replaceWith(blockquote);
            }
        });
        // Load Twitter widget JS if not already loaded
        if (!document.querySelector('script[src*="platform.twitter.com"]')) {
            const s = document.createElement('script');
            s.src = 'https://platform.twitter.com/widgets.js';
            s.async = true;
            document.body.appendChild(s);
        } else if (window.twttr && window.twttr.widgets) {
            window.twttr.widgets.load(container);
        }
    }

    // Process existing iframes (e.g. YouTube) - ensure they're responsive
    container.querySelectorAll('iframe').forEach(el => {
        if (el.closest('.embed-video')) return; // already wrapped
        const src = el.getAttribute('src') || '';
        if (src.includes('youtube.com') || src.includes('youtu.be')) {
            const wrapper = document.createElement('div');
            wrapper.className = 'embed-video';
            el.parentNode.insertBefore(wrapper, el);
            wrapper.appendChild(el);
            el.removeAttribute('width');
            el.removeAttribute('height');
        }
    });
}

export function extractYouTubeId(url) {
    if (!url) return null;
    // youtube.com/watch?v=ID, youtube.com/shorts/ID, youtube.com/embed/ID, youtu.be/ID
    const m = url.match(/(?:youtube\.com\/(?:watch\?v=|shorts\/|embed\/)|youtu\.be\/)([\w-]{11})/);
    return m ? m[1] : null;
}

// Delegated listeners for article list buttons (replaces inline onclick
// in index.html and JS-built empty-state HTML).
export function initArticleListListeners() {
    if (_articleListListenerAC) _articleListListenerAC.abort();
    _articleListListenerAC = new AbortController();
    const signal = _articleListListenerAC.signal;

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="show-hidden-articles"]');
        if (btn) {
            showHiddenArticles();
        }
    }, { signal });

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="show-read-articles"]');
        if (btn) {
            showReadArticles();
        }
    }, { signal });
}
