// Pagination: cursor-based infinite scroll for article lists.

import { api } from './api.js';
import { showToast } from './toast.js';
import { getArticleSortTime } from './utils.js';
import { queuedIdsReady, observeNewArticles } from './article-actions.js';
import {
    buildArticleCardHtml, processEmbeds, applyUserPreferences,
    getShowingHiddenArticles
} from './articles.js';

// --- Pagination state ---
export const PAGE_SIZE = 50;
let paginationCursorTime = null;
let paginationCursorId = null;
let paginationLoading = false;
let paginationDone = false;
let _paginationAC = null;

export function getPaginationState() {
    return {
        cursorTime: paginationCursorTime,
        cursorId: paginationCursorId,
        loading: paginationLoading,
        done: paginationDone,
    };
}

// Test-only accessors for individual state fields (used by app.test.js during migration)
export function _getPaginationCursorTime() { return paginationCursorTime; }
export function _getPaginationCursorId() { return paginationCursorId; }
export function _getPaginationDone() { return paginationDone; }
export function _getPaginationLoading() { return paginationLoading; }
export function _resetPaginationState() {
    if (_paginationAC) _paginationAC.abort();
    paginationCursorTime = null;
    paginationCursorId = null;
    paginationLoading = false;
    paginationDone = false;
}

export function setPaginationState({ cursorTime, cursorId, loading, done }) {
    if (cursorTime !== undefined) paginationCursorTime = cursorTime;
    if (cursorId !== undefined) paginationCursorId = cursorId;
    if (loading !== undefined) paginationLoading = loading;
    if (done !== undefined) paginationDone = done;
}

export function updateEndOfArticlesIndicator() {
    const el = document.getElementById('end-of-articles');
    if (!el) return;
    const hasArticles = document.querySelectorAll('#articles-list .article-card').length > 0;
    el.classList.toggle('visible', paginationDone && hasArticles);
}

export function updatePaginationCursor(articles) {
    if (!articles || articles.length === 0) return;
    const last = articles[articles.length - 1];
    paginationCursorTime = getArticleSortTime(last);
    paginationCursorId = last.id;
}

export function getPaginationUrl() {
    const path = window.location.pathname;
    const feedMatch = path.match(/^\/feed\/(\d+)/);
    if (feedMatch) return `/api/feeds/${feedMatch[1]}/articles`;
    const catMatch = path.match(/^\/category\/(\d+)/);
    if (catMatch) return `/api/categories/${catMatch[1]}/articles`;
    if (path === '/') return '/api/articles/unread';
    if (path === '/starred') return '/api/articles/starred';
    return null;
}

export async function loadMoreArticles() {
    if (paginationLoading || paginationDone) return;
    const url = getPaginationUrl();
    if (!url) return;

    if (!paginationCursorTime || !paginationCursorId) return;

    paginationLoading = true;
    try {
        const includeRead = getShowingHiddenArticles() ? '&include_read=1' : '';
        const params = `before_time=${encodeURIComponent(paginationCursorTime)}&before_id=${paginationCursorId}${includeRead}`;
        const data = await api('GET', `${url}?${params}`);
        const articles = data.articles || [];
        if (articles.length === 0) {
            paginationDone = true;
            return;
        }
        if (articles.length < PAGE_SIZE) {
            paginationDone = true;
        }
        updatePaginationCursor(articles);

        await queuedIdsReady;
        const list = document.getElementById('articles-list');
        if (!list) return;

        const fragment = document.createDocumentFragment();
        const temp = document.createElement('div');
        temp.innerHTML = articles.map(buildArticleCardHtml).join('');
        temp.querySelectorAll('.article-content-preview').forEach(el => processEmbeds(el));
        while (temp.firstChild) {
            fragment.appendChild(temp.firstChild);
        }
        list.appendChild(fragment);
        observeNewArticles(list);
        applyUserPreferences();
    } catch (e) {
        console.error('Failed to load more articles:', e);
        showToast('Failed to load more articles');
    } finally {
        paginationLoading = false;
        updateEndOfArticlesIndicator();
        // Re-check scroll position: if still near the bottom after appending,
        // trigger the next load immediately (the scroll event won't re-fire
        // if the user has stopped scrolling).
        checkScrollForMore();
    }
}

export function checkScrollForMore() {
    if (paginationDone || paginationLoading) return;
    const scrollBottom = window.innerHeight + window.scrollY;
    if (scrollBottom >= document.body.offsetHeight - 600) {
        loadMoreArticles();
    }
}

// Bootstrap pagination state from server-rendered articles and register
// the scroll listener for infinite scrolling.
export function initPagination() {
    if (_paginationAC) _paginationAC.abort();
    _paginationAC = new AbortController();
    const signal = _paginationAC.signal;
    const initialArticles = document.querySelectorAll('#articles-list .article-card');
    if (initialArticles.length > 0) {
        const lastCard = initialArticles[initialArticles.length - 1];
        setPaginationState({
            cursorTime: lastCard.dataset.sortTime || null,
            cursorId: lastCard.dataset.id || null,
            done: initialArticles.length < PAGE_SIZE,
        });
    } else {
        setPaginationState({ done: true });
    }
    updateEndOfArticlesIndicator();
    window.addEventListener('scroll', checkScrollForMore, { signal });
}
