import { api } from './modules/api.js';
import { initDropdownCloseListener, initDropdownListeners } from './modules/dropdown.js';
import { initTimestampTooltips } from './modules/timestamps.js';
import { initView, initViewListeners } from './modules/views.js';
import { toggleSidebar, setSidebarLoadCategory, initSidebarListeners } from './modules/sidebar.js';
import {
    renderArticleActions, renderArticles,
    processEmbeds, applyUserPreferences,
    initArticleListListeners,
} from './modules/articles.js';
import {
    initAutoMarkRead, queuedArticleIds, setQueuedArticleIds, setQueuedIdsReady,
    initArticleActionListeners,
} from './modules/article-actions.js';
import {
    updateEndOfArticlesIndicator,
    checkScrollForMore, setPaginationState, PAGE_SIZE
} from './modules/pagination.js';
import { updateCounts } from './modules/counts.js';
import {
    loadFeedArticles, loadCategoryArticles, initFeedActionListeners,
} from './modules/feeds.js';
import { initFoldersPageListeners, initCategorySettingsPage } from './modules/folders.js';
import { initFolderDragDrop } from './modules/drag-drop.js';
import { initOpmlListeners } from './modules/opml.js';
import { initSettingsPage, initSettingsPageListeners } from './modules/settings-page.js';
import { initQueuePage } from './modules/queue.js';
import { initScraperPage, initScraperPageListeners } from './modules/scraper-page.js';
import { initOfflineSupport } from './modules/offline.js';

// Initialize click-outside listener for dropdowns (was top-level in original code)
initDropdownCloseListener();

// Initialize delegated sidebar event listeners (replaces inline onclick in base.html)
initSidebarListeners();

// Initialize delegated listeners for index.html elements
initViewListeners();
initDropdownListeners();
initArticleActionListeners();
initFeedActionListeners();
initArticleListListeners();

// Initialize delegated listeners for feeds.html page
initFoldersPageListeners();
initOpmlListeners();

// Initialize delegated listeners for settings.html page
initSettingsPageListeners();

// Initialize delegated listeners for scrapers.html page
initScraperPageListeners();

// Wire sidebar's late-bound dependency on loadCategoryArticles
setSidebarLoadCategory((...args) => loadCategoryArticles(...args));









// Close sidebar when clicking a link on mobile
document.addEventListener('DOMContentLoaded', () => {
    // Expand parent folders of the active folder/feed so it's visible
    const activeItem = document.querySelector('.folder-item.active, .feed-item.active');
    if (activeItem) {
        let el = activeItem.parentElement;
        while (el) {
            const folder = el.closest('.folder-item');
            if (!folder) break;
            folder.classList.add('expanded');
            el = folder.parentElement;
        }
    }

    // Load queued article IDs, then hydrate action-button placeholders
    const _queueReady = api('GET', '/api/queue').then(articles => {
        setQueuedArticleIds(new Set((articles || []).map(a => a.id)));
    }).catch(() => {});
    setQueuedIdsReady(_queueReady);
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

    // Initialize timestamp tooltips with local timezone
    initTimestampTooltips();
    
    // Process embeds in article page content
    processEmbeds(document.querySelector('.article-body'));


    // Initialize auto-mark-read on scroll
    initAutoMarkRead();
    
    // Apply user preferences
    applyUserPreferences();

    // Initialize cursor-based pagination from server-rendered articles
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
    
    // Poll for count updates every 60 seconds (catches new articles from background fetches)
    setInterval(updateCounts, 60000);
    
    const sidebar = document.querySelector('.sidebar');
    if (sidebar) {
        sidebar.querySelectorAll('a').forEach(link => {
            link.addEventListener('click', () => {
                if (window.innerWidth <= 768) {
                    toggleSidebar();
                }
            });
        });
    }

    document.querySelectorAll('.feed-item[data-feed-id]').forEach(link => {
        link.addEventListener('click', (event) => {
            // On non-article pages, use normal navigation
            if (!document.querySelector('.articles-view')) {
                return;
            }
            event.preventDefault();
            const feedId = link.dataset.feedId;
            const feedName = link.dataset.feedName || link.querySelector('.feed-name')?.textContent || 'Feed';
            loadFeedArticles(feedId, feedName);
        });
    });
    
    // Initialize view mode
    initView();
    
    // Initialize folder drag-and-drop
    initFolderDragDrop();

    // Initialize settings page controls (no-op if not on settings page)
    initSettingsPage();

    // Initialize queue page (no-op if not on queue page)
    initQueuePage();

    // Initialize scraper page (no-op if not on scrapers page)
    initScraperPage();

    // Initialize category settings page (no-op if not on that page)
    initCategorySettingsPage();

    // Initialize offline/PWA support (no-op outside standalone mode)
    initOfflineSupport();
});



// Form handlers
document.addEventListener('DOMContentLoaded', () => {
    // Add feed form
    const addFeedForm = document.getElementById('add-feed-form');
    if (addFeedForm) {
        addFeedForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            let url = document.getElementById('feed-url').value;
            const name = document.getElementById('feed-name').value;
            const feedType = document.getElementById('feed-type').value;
            const scraperModule = document.getElementById('scraper-module')?.value || '';
            const categoryId = parseInt(document.getElementById('feed-category')?.value) || 0;
            const interval = parseInt(document.getElementById('feed-interval').value) || 60;
            
            let scraperConfig = '';
            let actualFeedType = feedType;

            // Handle Reddit feed type — it becomes a regular RSS feed
            if (feedType === 'reddit') {
                let subreddit = document.getElementById('reddit-subreddit').value.trim();
                if (!subreddit) {
                    alert('Please enter a subreddit name');
                    return;
                }
                // Strip leading r/ if present
                subreddit = subreddit.replace(/^\/?(r\/)?/, '');
                const sort = document.getElementById('reddit-sort').value;
                const period = document.getElementById('reddit-top-period').value;

                if (sort === 'top') {
                    url = `https://www.reddit.com/r/${subreddit}/top/.rss?t=${period}`;
                } else if (sort === 'best') {
                    url = `https://www.reddit.com/r/${subreddit}/.rss`;
                } else {
                    url = `https://www.reddit.com/r/${subreddit}/${sort}/.rss`;
                }
                actualFeedType = 'rss';
            }

            // Handle HuggingFace feed type
            if (feedType === 'huggingface') {
                const hfType = document.getElementById('hf-type').value;
                const hfIdentifier = document.getElementById('hf-identifier').value;
                
                if (!hfIdentifier && hfType !== 'daily_papers') {
                    alert('Please enter a username, organization, or collection slug');
                    return;
                }
                
                scraperConfig = JSON.stringify({
                    type: hfType,
                    identifier: hfIdentifier,
                    limit: 30
                });
                
                // Generate a URL for display purposes
                if (hfType === 'daily_papers') {
                    url = 'https://huggingface.co/papers';
                } else if (hfType === 'collection') {
                    url = `https://huggingface.co/collections/${hfIdentifier}`;
                } else if (hfType.includes('models')) {
                    url = `https://huggingface.co/${hfIdentifier}`;
                } else if (hfType.includes('datasets')) {
                    url = `https://huggingface.co/datasets?author=${hfIdentifier}`;
                } else if (hfType.includes('spaces')) {
                    url = `https://huggingface.co/spaces?author=${hfIdentifier}`;
                } else if (hfType.includes('posts')) {
                    url = `https://huggingface.co/${hfIdentifier}`;
                }
            }

            try {
                // For HuggingFace/Reddit feeds, let the server auto-generate the name
                const feedName = ((feedType === 'huggingface' || feedType === 'reddit') && !name) ? '' : (name || url);
                const feed = await api('POST', '/api/feeds', {
                    url,
                    name: feedName,
                    feedType: actualFeedType,
                    scraperModule,
                    scraperConfig,
                    interval
                });
                
                // Set category if specified
                if (categoryId > 0 && feed.id) {
                    await api('POST', `/api/feeds/${feed.id}/category`, { categoryId });
                }
                
                location.reload();
            } catch (e) {
                alert('Failed to add feed: ' + e.message);
            }
        });
    }

    // Scraper form is handled in scrapers.html template

    // Search
    const searchInput = document.getElementById('search');
    if (searchInput) {
        let timeout;
        let searchAbort = null;
        let originalHTML = null;
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
                } catch (e) {
                    if (e.name === 'AbortError') return; // cancelled, ignore
                    console.error('Search failed:', e);
                }
            }, 300);
        });
    }
});


// Prevent starting a drag when clicking chevrons.
document.addEventListener('dragstart', (event) => {
    if (event.target.closest('.folder-chevron')) {
        event.preventDefault();
    }
}, true);




window.addEventListener('scroll', checkScrollForMore);

