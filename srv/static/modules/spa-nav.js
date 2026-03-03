// SPA navigation for article-list pages.
// Intercepts sidebar nav clicks and browser back/forward to swap content
// without a full page reload.

import { api } from './api.js';
import { showArticlesLoading, renderArticles } from './articles.js';
import { setSidebarActive } from './sidebar.js';
import { applyDefaultViewForScope } from './views.js';
import { removeFeedErrorBanner } from './feed-errors.js';
import { updateCounts } from './counts.js';

// Pages that use the article-list layout (index.html template).
// SPA navigation works between these pages without a full reload.
const ARTICLE_LIST_ROUTES = [
    { pattern: /^\/$/, view: 'unread', title: 'All Unread', apiUrl: '/api/articles/unread', scope: 'all' },
    { pattern: /^\/starred$/, view: 'starred', title: 'Starred', apiUrl: '/api/articles/starred', scope: 'all' },
    { pattern: /^\/feed\/(\d+)$/, view: 'feed', scope: 'feed' },
    { pattern: /^\/category\/(\d+)$/, view: 'folder', scope: 'folder' },
];

// Check if the current page has the article-list layout.
function isArticleListPage() {
    return !!document.querySelector('.articles-view');
}

// Match a URL path to an article-list route config.
function matchRoute(path) {
    for (const route of ARTICLE_LIST_ROUTES) {
        const m = path.match(route.pattern);
        if (m) return { ...route, match: m };
    }
    return null;
}

// Navigate to a top-level nav route (/ or /starred) via SPA.
async function navigateToNavRoute(route) {
    if (!isArticleListPage()) return false;

    showArticlesLoading();

    // Update page title
    document.querySelector('.view-header h1').textContent = route.title;
    document.title = `${route.title} - FeedReader`;

    // Update view scope
    const articlesView = document.querySelector('.articles-view');
    if (articlesView) {
        articlesView.dataset.viewScope = route.scope;
    }

    // Update sidebar active state
    const navItem = document.querySelector(`.nav-item[href="${route.match.input}"]`);
    setSidebarActive(navItem);

    try {
        const data = await api('GET', route.apiUrl);

        // Push state (only if not triggered by popstate)
        history.pushState(
            { spaNav: true, path: route.match.input, view: route.view },
            route.title,
            route.match.input
        );

        renderArticles(data.articles);

        // Update the Mark as Read dropdown
        const dropdown = document.querySelector('.dropdown');
        if (dropdown) {
            dropdown.dataset.feedId = '';
            dropdown.dataset.categoryId = '';
        }

        // Hide feed-specific buttons
        document.querySelectorAll('[data-feed-action]').forEach(btn => {
            btn.style.display = 'none';
        });

        removeFeedErrorBanner();
        applyDefaultViewForScope(route.scope);
        updateCounts();
    } catch (e) {
        console.error('SPA navigation failed:', e);
    }

    return true;
}

// Restore content from a popstate event (browser back/forward).
async function restoreFromState(state, path) {
    if (!isArticleListPage()) {
        // Current page is not an article-list page, do full reload.
        window.location.reload();
        return;
    }

    const route = matchRoute(path);
    if (!route) {
        // Not an article-list route — full reload.
        window.location.reload();
        return;
    }

    // For feed/category routes, use state to get the ID and re-fetch.
    if (route.view === 'feed') {
        const feedId = route.match[1];
        // Dynamically import to avoid circular dependency
        const { loadFeedArticles } = await import('./feeds.js');
        const feedItem = document.querySelector(`.feed-item[data-feed-id="${feedId}"]`);
        const feedName = feedItem?.dataset.feedName || feedItem?.querySelector('.feed-name')?.textContent || 'Feed';
        await loadFeedArticles(feedId, feedName, { pushState: false });
        return;
    }

    if (route.view === 'folder') {
        const catId = route.match[1];
        const { loadCategoryArticles } = await import('./feeds.js');
        const folderItem = document.querySelector(`.folder-item[data-category-id="${catId}"]`);
        const catName = folderItem?.querySelector('.folder-name')?.textContent || 'Category';
        await loadCategoryArticles(catId, catName, { pushState: false });
        return;
    }

    // For unread/starred, fetch and render directly.
    showArticlesLoading();
    document.querySelector('.view-header h1').textContent = route.title;
    document.title = `${route.title} - FeedReader`;

    const articlesView = document.querySelector('.articles-view');
    if (articlesView) {
        articlesView.dataset.viewScope = route.scope;
    }

    const navItem = document.querySelector(`.nav-item[href="${path}"]`);
    setSidebarActive(navItem);

    try {
        const data = await api('GET', route.apiUrl);
        renderArticles(data.articles);

        const dropdown = document.querySelector('.dropdown');
        if (dropdown) {
            dropdown.dataset.feedId = '';
            dropdown.dataset.categoryId = '';
        }

        document.querySelectorAll('[data-feed-action]').forEach(btn => {
            btn.style.display = 'none';
        });

        removeFeedErrorBanner();
        applyDefaultViewForScope(route.scope);
    } catch (e) {
        console.error('SPA popstate navigation failed:', e);
    }
}

// Initialize SPA navigation listeners.
export function initSpaNav() {
    // Intercept clicks on sidebar nav-items that point to article-list pages.
    document.addEventListener('click', (e) => {
        const link = e.target.closest('.nav-item[href]');
        if (!link) return;

        const href = link.getAttribute('href');
        const route = matchRoute(href);

        // Only SPA-navigate for top-level article-list routes (/ and /starred).
        // Feed and category routes are handled by their own click listeners.
        if (!route || route.view === 'feed' || route.view === 'folder') return;

        // Only if we're currently on an article-list page.
        if (!isArticleListPage()) return;

        e.preventDefault();
        navigateToNavRoute(route);
    });

    // Handle browser back/forward.
    window.addEventListener('popstate', (e) => {
        const path = window.location.pathname;
        restoreFromState(e.state, path);
    });

    // Replace the current history entry with SPA state so popstate works
    // when navigating back to the initial page.
    const route = matchRoute(window.location.pathname);
    if (route && isArticleListPage()) {
        history.replaceState(
            { spaNav: true, path: window.location.pathname, view: route.view },
            document.title,
            window.location.pathname
        );
    }
}
