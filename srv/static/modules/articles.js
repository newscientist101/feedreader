// Article rendering: card HTML, article list, embeds, user preferences.

import { api } from './api.js';
import { formatTimeAgo, stripHtml, truncateText, PREVIEW_TEXT_LIMIT } from './utils.js';
import {
    SVG_MARK_READ, SVG_MARK_UNREAD, SVG_STAR_FILLED, SVG_STAR_EMPTY,
    SVG_EXTERNAL, SVG_QUEUE_ADD, SVG_QUEUE_REMOVE
} from './icons.js';
import { getSetting, applyHideReadArticles, applyHideEmptyFeeds } from './settings.js';
import { queuedArticleIds, queuedIdsReady, initAutoMarkRead } from './article-actions.js';

// --- Late-bound dependencies (set by app.js during init) ---
let _updatePaginationCursor = null;
let _updateEndOfArticlesIndicator = null;
let _setPaginationState = null;

export function setArticlesDeps({ updatePaginationCursor, updateEndOfArticlesIndicator, setPaginationState }) {
    if (updatePaginationCursor) _updatePaginationCursor = updatePaginationCursor;
    if (updateEndOfArticlesIndicator) _updateEndOfArticlesIndicator = updateEndOfArticlesIndicator;
    if (setPaginationState) _setPaginationState = setPaginationState;
}

// Temporary flag: when true, the current view includes read/hidden articles.
// Reset on the next navigation.
let showingHiddenArticles = false;

export function getShowingHiddenArticles() {
    return showingHiddenArticles;
}

export function setShowingHiddenArticles(val) {
    showingHiddenArticles = val;
}

export function _resetArticlesState() {
    showingHiddenArticles = false;
}

// Render the standard set of action buttons for an article.
// `a` must have: id, is_read, is_starred, url (optional), is_queued (optional).
export function renderArticleActions(a) {
    const readBtn = `<button onclick="${a.is_read ? 'markUnread' : 'markRead'}(event, ${a.id})" class="btn-icon btn-read-toggle" title="${a.is_read ? 'Mark unread' : 'Mark read'}">
        ${a.is_read ? SVG_MARK_UNREAD : SVG_MARK_READ}
    </button>`;
    const starBtn = `<button onclick="toggleStar(event, ${a.id})" class="btn-icon ${a.is_starred ? 'starred' : ''}" title="Star">
        ${a.is_starred ? SVG_STAR_FILLED : SVG_STAR_EMPTY}
    </button>`;
    const queueBtn = `<button onclick="toggleQueue(event, ${a.id})" class="btn-icon btn-queue-toggle ${a.is_queued ? 'queued' : ''}" title="${a.is_queued ? 'Remove from queue' : 'Add to queue'}">
        ${a.is_queued ? SVG_QUEUE_REMOVE : SVG_QUEUE_ADD}
    </button>`;
    const extBtn = a.url ? `<a href="${a.url}" target="_blank" class="btn-icon" title="Open original">${SVG_EXTERNAL}</a>` : '';
    return `<div class="article-actions">${readBtn}${starBtn}${queueBtn}${extBtn}</div>`;
}

// Update the read/unread toggle button inside an article card
export function updateReadButton(card, isRead) {
    if (!card) return;
    const btn = card.querySelector('.btn-read-toggle');
    if (!btn) return;
    const id = card.dataset.id;
    if (isRead) {
        btn.setAttribute('onclick', `markUnread(event, ${id})`);
        btn.setAttribute('title', 'Mark unread');
        btn.innerHTML = SVG_MARK_UNREAD;
    } else {
        btn.setAttribute('onclick', `markRead(event, ${id})`);
        btn.setAttribute('title', 'Mark read');
        btn.innerHTML = SVG_MARK_READ;
    }
}

// Apply user preferences from settings
export function applyUserPreferences() {
    applyHideReadArticles(getSetting('hideReadArticles'));
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
    if (!hideRead) return;

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
            <button onclick="showReadArticles()" class="btn btn-secondary" style="margin-top: 10px;">Show read articles</button>
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
    return `
        <article class="article-card ${a.is_read ? 'read' : ''}${a.image_url ? ' has-image' : ''}" data-id="${a.id}" data-sort-time="${a.published_at || a.fetched_at}">
            ${a.image_url ? `<div class="article-image magazine-expanded-only"><img src="${a.image_url}" alt="" loading="lazy"></div>` : `<div class="article-image-placeholder magazine-only">
                <svg viewBox="0 0 24 24" width="32" height="32" fill="currentColor">
                    <path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z"/>
                </svg>
            </div>`}
            <div class="article-body clickable" onclick="openArticle(${a.id})">
                <div class="article-meta">
                    <a class="feed-name" href="/feed/${a.feed_id}" onclick="event.stopPropagation();">${a.feed_name || ''}</a>
                    ${a.author ? `<span class="article-author">${a.author}</span>` : ''}
                    <span class="article-date">${formatTimeAgo(a.published_at)}</span>
                </div>
                <h2 class="article-title">
                    ${a.url ? `<a href="${a.url}" target="_blank" onclick="openArticleExternal(event, ${a.id}, '${a.url.replace(/'/g, "\\\'")}')">${a.title}</a>` : `<a href="/article/${a.id}" onclick="markReadSilent(${a.id})">${a.title}</a>`}
                </h2>
                ${a.summary ? `<p class="article-summary">${truncateText(stripHtml(a.summary), PREVIEW_TEXT_LIMIT)}</p>` : (a.content ? `<p class="article-summary">${truncateText(stripHtml(a.content), PREVIEW_TEXT_LIMIT)}</p>` : '')}
                ${a.content ? `<div class="article-content-preview expanded-only" onclick="event.stopPropagation(); markReadSilent(${a.id})">${truncateText(stripHtml(a.content), PREVIEW_TEXT_LIMIT)}</div>` : (a.summary ? `<div class="article-content-preview expanded-only" onclick="event.stopPropagation(); markReadSilent(${a.id})">${truncateText(stripHtml(a.summary), PREVIEW_TEXT_LIMIT)}</div>` : '')}
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
    if (_setPaginationState) {
        _setPaginationState({ cursorTime: null, cursorId: null, done: false, loading: false });
    }

    const PAGE_SIZE = 50;

    if (!articles || articles.length === 0) {
        const showBtn = !showingHiddenArticles
            ? `<button onclick="showHiddenArticles()" class="btn btn-secondary" style="margin-top: 10px;">Show hidden articles</button>`
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
        if (_setPaginationState) _setPaginationState({ done: true });
        if (_updateEndOfArticlesIndicator) _updateEndOfArticlesIndicator();
        return;
    }

    if (articles.length < PAGE_SIZE) {
        if (_setPaginationState) _setPaginationState({ done: true });
    }

    // Set cursor from last article for next pagination request
    if (_updatePaginationCursor) _updatePaginationCursor(articles);

    list.innerHTML = articles.map(buildArticleCardHtml).join('');

    // Process embeds in expanded view content previews
    list.querySelectorAll('.article-content-preview').forEach(el => processEmbeds(el));

    // Re-apply user preferences (hide read, etc.)
    applyUserPreferences();
    if (_updateEndOfArticlesIndicator) _updateEndOfArticlesIndicator();

    // Re-initialize auto-mark-read observer for new article set
    initAutoMarkRead();
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
