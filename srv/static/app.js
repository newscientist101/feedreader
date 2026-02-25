import { initDropdownCloseListener, initDropdownListeners } from './modules/dropdown.js';
import { initTimestampTooltips } from './modules/timestamps.js';
import { initView, initViewListeners } from './modules/views.js';
import { setSidebarLoadCategory, initSidebarListeners, initSidebarMobileClose } from './modules/sidebar.js';
import {
    renderArticleActions,
    processEmbeds, applyUserPreferences,
    initArticleListListeners,
} from './modules/articles.js';
import {
    initAutoMarkRead, initQueueState,
    initArticleActionListeners,
} from './modules/article-actions.js';
import { initPagination } from './modules/pagination.js';
import { updateCounts } from './modules/counts.js';
import {
    loadCategoryArticles, initFeedActionListeners,
    initAddFeedForm, initFeedItemClickListeners,
} from './modules/feeds.js';
import { initSearch } from './modules/search.js';
import { initFoldersPageListeners, initCategorySettingsPage } from './modules/folders.js';
import { initFolderDragDrop, initDragPrevention } from './modules/drag-drop.js';
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

// Prevent drag from chevrons (must be top-level, runs before DOMContentLoaded)
initDragPrevention();

// Wire sidebar's late-bound dependency on loadCategoryArticles
setSidebarLoadCategory((...args) => loadCategoryArticles(...args));


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

    // Load queued article IDs and hydrate action-button placeholders
    initQueueState(renderArticleActions);

    // Initialize timestamp tooltips with local timezone
    initTimestampTooltips();
    
    // Process embeds in article page content
    processEmbeds(document.querySelector('.article-body'));

    // Initialize auto-mark-read on scroll
    initAutoMarkRead();
    
    // Apply user preferences
    applyUserPreferences();

    // Initialize cursor-based pagination from server-rendered articles
    initPagination();
    
    // Poll for count updates every 60 seconds (catches new articles from background fetches)
    setInterval(updateCounts, 60000);
    
    // Close sidebar on mobile when a link is clicked
    initSidebarMobileClose();

    // Initialize SPA feed-item click handlers in sidebar
    initFeedItemClickListeners();
    
    // Initialize view mode
    initView();
    
    // Initialize folder drag-and-drop
    initFolderDragDrop();

    // Initialize add feed form (no-op if not on feeds page)
    initAddFeedForm();

    // Initialize search (no-op if #search element absent)
    initSearch();

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

