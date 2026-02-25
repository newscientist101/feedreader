// Search: debounced article search with abort support and context scoping.

import { renderArticles, applyUserPreferences } from './articles.js';

let timeout;
let searchAbort = null;
let originalHTML = null;

// Initialize the search input handler. No-op if #search element is absent.
export function initSearch() {
    const searchInput = document.getElementById('search');
    if (!searchInput) return;

    searchInput.addEventListener('input', (e) => {
        clearTimeout(timeout);
        if (searchAbort) { searchAbort.abort(); searchAbort = null; }
        timeout = setTimeout(async () => {
            const q = e.target.value.trim();
            if (q.length < 2) {
                // Restore original article list if we saved it
                if (originalHTML !== null) {
                    const list = document.getElementById('articles-list');
                    if (list) list.innerHTML = originalHTML;
                    originalHTML = null;
                    applyUserPreferences();
                }
                return;
            }
            // Save original HTML before first search replaces it
            if (originalHTML === null) {
                const list = document.getElementById('articles-list');
                if (list) originalHTML = list.innerHTML;
            }
            try {
                searchAbort = new AbortController();
                let searchUrl = `/api/search?q=${encodeURIComponent(q)}`;
                // Scope search to current feed or category context
                const pathMatch = window.location.pathname.match(/^\/(feed|category)\/(\d+)/);
                if (pathMatch) {
                    const [, type, id] = pathMatch;
                    searchUrl += type === 'feed' ? `&feed_id=${id}` : `&category_id=${id}`;
                }
                const res = await fetch(searchUrl, { signal: searchAbort.signal });
                const articles = await res.json();
                searchAbort = null;
                if (!res.ok) throw new Error(articles.error || 'Search failed');
                renderArticles(articles);
            } catch (err) {
                if (err.name === 'AbortError') return; // cancelled, ignore
                console.error('Search failed:', err);
            }
        }, 300);
    });
}

// Reset module state (for tests).
export function _resetSearchState() {
    clearTimeout(timeout);
    timeout = undefined;
    if (searchAbort) { searchAbort.abort(); }
    searchAbort = null;
    originalHTML = null;
}
