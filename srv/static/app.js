import { api } from './modules/api.js';
import { getSetting, saveSetting, applyHideReadArticles, applyHideEmptyFeeds } from './modules/settings.js';
import { toggleDropdown, initDropdownCloseListener } from './modules/dropdown.js';
import { initTimestampTooltips } from './modules/timestamps.js';
import { setView, initView } from './modules/views.js';
import { toggleSidebar, navigateFolder, toggleFolderCollapse, setSidebarLoadCategory } from './modules/sidebar.js';
import {
    renderArticleActions, renderArticles, updateReadButton,
    showReadArticles, showHiddenArticles,
    processEmbeds, applyUserPreferences, setArticlesDeps
} from './modules/articles.js';
import {
    markRead, markUnread, toggleStar, toggleQueue, markAsRead,
    markReadSilent, openArticle, openArticleExternal,
    initAutoMarkRead, queuedArticleIds, setQueuedArticleIds, setQueuedIdsReady,
    setArticleActionDeps
} from './modules/article-actions.js';
import {
    updateEndOfArticlesIndicator, updatePaginationCursor,
    checkScrollForMore, setPaginationState, PAGE_SIZE
} from './modules/pagination.js';
import { updateCounts, setCountsDeps } from './modules/counts.js';
import {
    loadFeedArticles, loadCategoryArticles, refreshFeed, deleteFeed,
    editFeed, saveFeed, filterFeeds, closeEditModal, setFeedCategory,
    showFeedErrorBanner, removeFeedErrorBanner,
} from './modules/feeds.js';
import {
    openCreateFolderModal, closeCreateFolderModal, submitCreateFolder,
    renameCategory, unparentCategory, deleteCategory,
} from './modules/folders.js';

// Initialize click-outside listener for dropdowns (was top-level in original code)
initDropdownCloseListener();

// Wire sidebar's late-bound dependency on loadCategoryArticles
setSidebarLoadCategory((...args) => loadCategoryArticles(...args));

// Wire article-actions' late-bound dependencies
setArticleActionDeps({
    updateReadButton,
    updateCounts,
    updateQueueCacheIfStandalone: (...args) => updateQueueCacheIfStandalone(...args),
});

// Wire articles' late-bound dependencies on pagination
setArticlesDeps({
    updatePaginationCursor,
    updateEndOfArticlesIndicator,
    setPaginationState,
});

// Wire counts' late-bound dependencies on feeds and articles
setCountsDeps({
    showFeedErrorBanner,
    removeFeedErrorBanner,
    applyUserPreferences,
});





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
});



// OPML functions
function exportOPML() {
    window.location.href = '/api/opml/export';
}

async function importOPML(input) {
    const file = input.files[0];
    if (!file) return;
    
    const formData = new FormData();
    formData.append('file', file);
    
    try {
        const res = await fetch('/api/opml/import', {
            method: 'POST',
            body: formData
        });
        const result = await res.json();
        if (!res.ok) {
            throw new Error(result.error || 'Import failed');
        }
        alert(`Imported ${result.imported} feeds (${result.skipped} skipped, already exist)`);
        location.reload();
    } catch (e) {
        alert('Failed to import OPML: ' + e.message);
    }
    
    // Clear the input
    input.value = '';
}

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


// Folder drag-and-drop reordering
function initFolderDragDrop() {
    // Sidebar folders
    const foldersContainer = document.querySelector('.folders-list');
    if (foldersContainer) {
        initDragDrop(foldersContainer, '.folder-item', 'data-category-id');
    }
    
    // Feeds page category cards
    const categoriesGrid = document.querySelector('.categories-grid');
    if (categoriesGrid) {
        initDragDrop(categoriesGrid, '.category-card[data-id]', 'data-id');
    }
}

function initDragDrop(container, itemSelector, idAttr) {
    let draggedItem = null;
    let placeholder = null;
    let dropTarget = null; // For nesting
    let draggedParentId = null; // Track parent of dragged item
    
    // Helper to get parent ID from item
    function getParentId(item) {
        const parentAttr = item.dataset.parentId;
        return parentAttr ? parseInt(parentAttr) : null;
    }
    
    // Helper to get siblings (items with same parent)
    function getSiblings(parentId) {
        return Array.from(container.querySelectorAll(itemSelector)).filter(item => {
            return getParentId(item) === parentId;
        });
    }
    
    container.addEventListener('dragstart', (e) => {
        const item = e.target.closest(itemSelector);
        if (!item) return;
        
        draggedItem = item;
        draggedParentId = getParentId(item);
        item.classList.add('dragging');
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', item.getAttribute(idAttr));
        
        // Create placeholder
        placeholder = document.createElement('div');
        placeholder.className = 'drag-placeholder';
        placeholder.style.height = item.offsetHeight + 'px';
        // Copy indentation from dragged item
        if (item.style.paddingLeft) {
            placeholder.style.marginLeft = item.style.paddingLeft;
        }
    });
    
    container.addEventListener('dragend', (e) => {
        if (draggedItem) {
            draggedItem.classList.remove('dragging');
            draggedItem = null;
        }
        if (placeholder && placeholder.parentNode) {
            placeholder.remove();
        }
        placeholder = null;
        draggedParentId = null;
        
        // Remove any remaining drag-over classes
        container.querySelectorAll('.drag-over, .nest-target').forEach(el => {
            el.classList.remove('drag-over', 'nest-target');
        });
        dropTarget = null;
    });
    
    container.addEventListener('dragover', (e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        
        const targetItem = e.target.closest(itemSelector);
        
        // Check if we're hovering over another folder (for nesting)
        // Only if holding Shift key
        if (e.shiftKey && targetItem && targetItem !== draggedItem) {
            // Show nest target indicator
            container.querySelectorAll('.nest-target').forEach(el => el.classList.remove('nest-target'));
            targetItem.classList.add('nest-target');
            dropTarget = targetItem;
            if (placeholder.parentNode) placeholder.remove();
            return;
        } else {
            container.querySelectorAll('.nest-target').forEach(el => el.classList.remove('nest-target'));
            dropTarget = null;
        }
        
        // Get position among siblings only
        const siblings = getSiblings(draggedParentId);
        const afterElement = getDragAfterElementAmongSiblings(siblings, e.clientY);
        
        if (placeholder) {
            if (afterElement) {
                container.insertBefore(placeholder, afterElement);
            } else {
                // Find where to insert at end of siblings
                if (siblings.length > 0) {
                    const lastSibling = siblings[siblings.length - 1];
                    if (lastSibling.nextSibling) {
                        container.insertBefore(placeholder, lastSibling.nextSibling);
                    } else {
                        // Check for add-category card
                        const addCard = container.querySelector('.add-category');
                        if (addCard) {
                            container.insertBefore(placeholder, addCard);
                        } else {
                            container.appendChild(placeholder);
                        }
                    }
                } else {
                    const addCard = container.querySelector('.add-category');
                    if (addCard) {
                        container.insertBefore(placeholder, addCard);
                    } else {
                        container.appendChild(placeholder);
                    }
                }
            }
        }
    });
    
    container.addEventListener('drop', async (e) => {
        e.preventDefault();
        
        if (!draggedItem) return;
        
        const draggedId = parseInt(draggedItem.getAttribute(idAttr));
        
        // Check if nesting (Shift was held and we have a target)
        if (dropTarget && dropTarget !== draggedItem) {
            const parentId = parseInt(dropTarget.getAttribute(idAttr));
            
            // Set parent via API
            try {
                await api('POST', `/api/categories/${draggedId}/parent`, {
                    parent_id: parentId,
                    sort_order: 0
                });
                // Reload to show new hierarchy
                location.reload();
            } catch (err) {
                console.error('Failed to nest folder:', err);
            }
            return;
        }
        
        if (!placeholder) return;
        
        // Insert the dragged item where the placeholder is
        placeholder.replaceWith(draggedItem);
        
        // Get new order - only for siblings with the same parent
        const siblings = getSiblings(draggedParentId);
        const order = siblings
            .map(item => parseInt(item.getAttribute(idAttr)))
            .filter(id => !isNaN(id));
        
        // Save new order to server (include parent_id so server knows context)
        try {
            await api('POST', '/api/categories/reorder', { 
                order,
                parent_id: draggedParentId
            });
            // Sync the other container
            syncFolderOrder(order, container, draggedParentId);
        } catch (err) {
            console.error('Failed to save folder order:', err);
        }
    });
}

function syncFolderOrder(order, sourceContainer, parentId = null) {
    // Sync sidebar folders
    const sidebarFolders = document.querySelector('.folders-list');
    if (sidebarFolders && sidebarFolders !== sourceContainer) {
        reorderElements(sidebarFolders, '.folder-item', 'data-category-id', order, parentId);
    }
    
    // Sync feeds page category cards
    const categoriesGrid = document.querySelector('.categories-grid');
    if (categoriesGrid && categoriesGrid !== sourceContainer) {
        reorderElements(categoriesGrid, '.category-card[data-id]', 'data-id', order, parentId);
    }
}

function reorderElements(container, itemSelector, idAttr, order, parentId = null) {
    // Get only items with the matching parent
    const items = Array.from(container.querySelectorAll(itemSelector)).filter(item => {
        const itemParentId = item.dataset.parentId ? parseInt(item.dataset.parentId) : null;
        return itemParentId === parentId;
    });
    
    const itemMap = new Map();
    items.forEach(item => {
        const id = parseInt(item.getAttribute(idAttr));
        if (!isNaN(id)) {
            itemMap.set(id, item);
        }
    });
    
    if (items.length === 0) return;
    
    // Find the first sibling's position to know where to insert
    const firstSibling = items[0];
    let insertPoint = firstSibling;
    
    // Reorder by inserting in order at the insertion point
    order.forEach((id, index) => {
        const item = itemMap.get(id);
        if (item) {
            if (index === 0) {
                // First item stays at the original position
                insertPoint = item.nextSibling;
            } else {
                container.insertBefore(item, insertPoint);
                insertPoint = item.nextSibling;
            }
        }
    });
}

// Get position among a specific set of sibling elements
function getDragAfterElementAmongSiblings(siblings, y) {
    const nonDragging = siblings.filter(el => !el.classList.contains('dragging'));
    
    return nonDragging.reduce((closest, child) => {
        const box = child.getBoundingClientRect();
        const offset = y - box.top - box.height / 2;
        
        if (offset < 0 && offset > closest.offset) {
            return { offset, element: child };
        } else {
            return closest;
        }
    }, { offset: Number.NEGATIVE_INFINITY }).element;
}

// Process custom embed tags in article content
// Prevent starting a drag when clicking chevrons.
document.addEventListener('dragstart', (event) => {
    if (event.target.closest('.folder-chevron')) {
        event.preventDefault();
    }
}, true);


// --- Helpers moved from inline template scripts ---

// Toggle read articles visibility (used by settings page for instant feedback)
// Settings page: run retention cleanup
async function runCleanup() {
    const status = document.getElementById('cleanup-status');
    status.textContent = 'Cleaning up...';
    status.className = 'cleanup-status';
    try {
        const data = await api('POST', '/api/retention/cleanup');
        status.textContent = `Deleted ${data.deleted} articles`;
        status.className = 'cleanup-status success';
        document.getElementById('articles-to-delete').textContent = '0';
    } catch (err) {
        status.textContent = 'Cleanup failed: ' + err.message;
        status.className = 'cleanup-status error';
    }
}

// Settings page: initialize controls from server settings
function initSettingsPage() {
    const toggle = document.getElementById('auto-mark-read');
    if (!toggle) return; // not on settings page

    toggle.checked = getSetting('autoMarkRead') === 'true';

    const hideReadValue = getSetting('hideReadArticles') || 'show';
    const hideReadRadio = document.querySelector(`input[name="hide-read"][value="${hideReadValue}"]`);
    if (hideReadRadio) hideReadRadio.checked = true;

    const hideEmptyValue = getSetting('hideEmptyFeeds') || 'show';
    const hideEmptyRadio = document.querySelector(`input[name="hide-empty"][value="${hideEmptyValue}"]`);
    if (hideEmptyRadio) hideEmptyRadio.checked = true;

    const folderView = getSetting('defaultFolderView') || 'card';
    const folderRadio = document.querySelector(`input[name="folder-view"][value="${folderView}"]`);
    if (folderRadio) folderRadio.checked = true;

    const feedView = getSetting('defaultFeedView') || 'card';
    const feedRadio = document.querySelector(`input[name="feed-view"][value="${feedView}"]`);
    if (feedRadio) feedRadio.checked = true;

    // Load newsletter address
    loadNewsletterAddress();
}

async function loadNewsletterAddress() {
    const container = document.getElementById('newsletter-container');
    if (!container) return;
    try {
        const data = await api('GET', '/api/newsletter/address');
        if (data.address) {
            showNewsletterAddress(data.address);
        }
    } catch {
        // No address yet, show generate button
    }
}

async function generateNewsletterAddress() {
    try {
        const data = await api('POST', '/api/newsletter/generate-address');
        if (data.address) {
            showNewsletterAddress(data.address);
        }
    } catch (e) {
        alert('Failed to generate address: ' + e.message);
    }
}

function showNewsletterAddress(address) {
    const noAddr = document.getElementById('newsletter-no-address');
    const hasAddr = document.getElementById('newsletter-has-address');
    const addrEl = document.getElementById('newsletter-address');
    if (noAddr) noAddr.style.display = 'none';
    if (hasAddr) hasAddr.style.display = '';
    if (addrEl) addrEl.textContent = address;
}

async function copyNewsletterAddress() {
    const addrEl = document.getElementById('newsletter-address');
    if (!addrEl) return;
    try {
        await navigator.clipboard.writeText(addrEl.textContent);
        const btn = addrEl.nextElementSibling;
        if (btn) {
            const orig = btn.innerHTML;
            btn.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>';
            setTimeout(() => { btn.innerHTML = orig; }, 1500);
        }
    } catch {
        // Fallback: select the text
        const range = document.createRange();
        range.selectNodeContents(addrEl);
        const sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
    }
}

window.addEventListener('scroll', checkScrollForMore);

// ============================================================
// Offline Queue Support (PWA standalone mode only)
// ============================================================

const _isStandalone = (typeof window.matchMedia === 'function' &&
    window.matchMedia('(display-mode: standalone)').matches) ||
    window.navigator.standalone === true;

function initOfflineSupport() {
    if (!_isStandalone) return;
    if (!('serviceWorker' in navigator)) return;

    // Wait for SW to be ready
    navigator.serviceWorker.ready.then((reg) => {
        const sw = reg.active;
        if (!sw) return;

        // Enable offline mode in the SW
        const staticUrls = [];
        document.querySelectorAll('link[rel="stylesheet"]').forEach(l => {
            if (l.href) staticUrls.push(l.href);
        });
        document.querySelectorAll('script[src]').forEach(s => {
            if (s.src && s.src.includes('/static/')) staticUrls.push(s.src);
        });
        sw.postMessage({ type: 'ENABLE_OFFLINE', data: { staticUrls } });

        // Cache queue articles
        cacheQueueForOffline(sw);
    });

    // Listen for SW messages
    navigator.serviceWorker.addEventListener('message', (event) => {
        if (event.data?.type === 'OFFLINE_ENABLED') {
            console.log('Offline mode enabled for PWA');
        }
    });

    // Monitor online/offline state
    window.addEventListener('online', handleOnlineStateChange);
    window.addEventListener('offline', handleOnlineStateChange);
    // Apply initial offline state if needed (without triggering reload)
    if (!navigator.onLine) {
        document.body.classList.add('pwa-offline');
        showOfflineBanner();
        disableNonQueueUI();
    }
}

function cacheQueueForOffline(sw) {
    // Fetch queue articles and send to SW for caching
    api('GET', '/api/queue').then(articles => {
        if (sw) {
            sw.postMessage({ type: 'CACHE_QUEUE', data: { articles: articles || [] } });
        }
    }).catch(() => {});
}

function handleOnlineStateChange() {
    if (!_isStandalone) return;

    const isOffline = !navigator.onLine;
    document.body.classList.toggle('pwa-offline', isOffline);

    if (isOffline) {
        showOfflineBanner();
        disableNonQueueUI();
    } else {
        // This only fires on a real offline→online transition (from the
        // 'online' event), never on initial page load.
        const banner = document.getElementById('offline-banner');
        if (banner) {
            banner.style.background = '#27ae60';
            banner.innerHTML =
                '<svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">' +
                '<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>' +
                '</svg> Back online \u2014 reloading\u2026';
        }
        enableAllUI();
        replayPendingActions(() => {
            window.location.reload();
        });
    }
}

function showOfflineBanner() {
    if (document.getElementById('offline-banner')) return;
    const banner = document.createElement('div');
    banner.id = 'offline-banner';
    banner.className = 'offline-banner';
    banner.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">' +
        '<path d="M19.35 10.04C18.67 6.59 15.64 4 12 4c-1.48 0-2.85.43-4.01 1.17l1.46 1.46C10.21 6.23 11.08 6 12 6c3.04 0 5.5 2.46 5.5 5.5v.5H19c1.66 0 3 1.34 3 3 0 .99-.49 1.87-1.24 2.41l1.46 1.46C23.33 17.98 24 16.58 24 15c0-2.64-2.05-4.78-4.65-4.96zM3 5.27l2.75 2.74C2.56 8.15 0 10.77 0 14c0 3.31 2.69 6 6 6h11.73l2 2 1.27-1.27L4.27 4 3 5.27zM7.73 10l8 8H6c-2.21 0-4-1.79-4-4s1.79-4 4-4h1.73z"/>' +
        '</svg> ' +
        'You\'re offline' +
        (window.location.pathname !== '/queue' ? ' \u2014 <a href="/queue" style="color:#fff;text-decoration:underline">Go to Queue</a>' : '');
    document.body.prepend(banner);
}

function disableNonQueueUI() {
    const isQueuePage = window.location.pathname === '/queue';

    // Disable sidebar links except queue
    document.querySelectorAll('.sidebar .nav-item, .sidebar .feed-item, .sidebar .folder-item').forEach(el => {
        const href = el.getAttribute('href');
        if (href === '/queue') return;
        el.classList.add('offline-disabled');
        el.setAttribute('data-offline-disabled', 'true');
    });

    // Disable sidebar footer sections (scrapers, settings, user info)
    document.querySelectorAll('.sidebar .feeds-section, .sidebar .feeds-header').forEach(el => {
        el.classList.add('offline-disabled');
        el.setAttribute('data-offline-disabled', 'true');
    });

    // On non-queue pages, show overlay
    if (!isQueuePage) {
        const content = document.querySelector('.content');
        if (content && !document.getElementById('offline-overlay')) {
            const overlay = document.createElement('div');
            overlay.id = 'offline-overlay';
            overlay.className = 'offline-overlay';
            overlay.innerHTML = '<div class="offline-overlay-content">' +
                '<svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor">' +
                '<path d="M19.35 10.04C18.67 6.59 15.64 4 12 4c-1.48 0-2.85.43-4.01 1.17l1.46 1.46C10.21 6.23 11.08 6 12 6c3.04 0 5.5 2.46 5.5 5.5v.5H19c1.66 0 3 1.34 3 3 0 .99-.49 1.87-1.24 2.41l1.46 1.46C23.33 17.98 24 16.58 24 15c0-2.64-2.05-4.78-4.65-4.96zM3 5.27l2.75 2.74C2.56 8.15 0 10.77 0 14c0 3.31 2.69 6 6 6h11.73l2 2 1.27-1.27L4.27 4 3 5.27zM7.73 10l8 8H6c-2.21 0-4-1.79-4-4s1.79-4 4-4h1.73z"/>' +
                '</svg>' +
                '<h2>You\'re offline</h2>' +
                '<p>This section is not available offline.</p>' +
                '<a href="/queue" class="btn btn-primary">Go to Reading Queue</a>' +
                '</div>';
            content.style.position = 'relative';
            content.appendChild(overlay);
        }
    }
}

function enableAllUI() {
    document.querySelectorAll('[data-offline-disabled]').forEach(el => {
        el.classList.remove('offline-disabled');
        el.removeAttribute('data-offline-disabled');
    });
    const overlay = document.getElementById('offline-overlay');
    if (overlay) overlay.remove();
}

function replayPendingActions(callback) {
    if (!('serviceWorker' in navigator) || !navigator.serviceWorker.controller) {
        if (callback) setTimeout(callback, 0);
        return;
    }

    const handler = (event) => {
        if (event.data?.type !== 'PENDING_ACTIONS') return;
        navigator.serviceWorker.removeEventListener('message', handler);

        const actions = event.data.actions || [];
        const promises = actions.map((action) => {
            if (action.type === 'dequeue') {
                return api('DELETE', `/api/articles/${action.articleId}/queue`).catch(() => {});
            }
            return Promise.resolve();
        });
        Promise.all(promises).then(() => {
            if (actions.length > 0) updateCounts();
            if (callback) callback();
        });
    };
    navigator.serviceWorker.addEventListener('message', handler);
    navigator.serviceWorker.controller.postMessage({ type: 'GET_PENDING_ACTIONS' });
    // Safety timeout: if SW doesn't respond within 3s, proceed anyway
    setTimeout(() => {
        navigator.serviceWorker.removeEventListener('message', handler);
        if (callback) callback();
    }, 3000);
}

// Update queue cache when articles change
function updateQueueCacheIfStandalone() {
    if (!_isStandalone) return;
    if (!('serviceWorker' in navigator)) return;
    navigator.serviceWorker.ready.then((reg) => {
        if (reg.active) cacheQueueForOffline(reg.active);
    });
}

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', () => {
    initOfflineSupport();
});

// --- Transitional window exports (Phase 2) ---
// Functions called from inline onclick/onchange handlers in templates
// or from <script> blocks in template files need to be global.
// These will be removed in Phase 3 when inline handlers are eliminated.
window.api = api;
window.getSetting = getSetting;
window.saveSetting = saveSetting;
window.saveFeed = saveFeed;
window.applyUserPreferences = applyUserPreferences;
window.applyHideReadArticles = applyHideReadArticles;
window.applyHideEmptyFeeds = applyHideEmptyFeeds;
window.toggleDropdown = toggleDropdown;
window.toggleSidebar = toggleSidebar;
window.toggleFolderCollapse = toggleFolderCollapse;
window.navigateFolder = navigateFolder;
window.openCreateFolderModal = openCreateFolderModal;
window.closeCreateFolderModal = closeCreateFolderModal;
window.closeEditModal = closeEditModal;
window.submitCreateFolder = submitCreateFolder;
window.deleteCategory = deleteCategory;
window.deleteFeed = deleteFeed;
window.editFeed = editFeed;
window.exportOPML = exportOPML;
window.importOPML = importOPML;
window.filterFeeds = filterFeeds;
window.markAsRead = markAsRead;
window.markRead = markRead;
window.markUnread = markUnread;
window.markReadSilent = markReadSilent;
window.openArticle = openArticle;
window.openArticleExternal = openArticleExternal;
window.refreshFeed = refreshFeed;
window.renameCategory = renameCategory;
window.runCleanup = runCleanup;
window.setFeedCategory = setFeedCategory;
window.setView = setView;
window.showHiddenArticles = showHiddenArticles;
window.showReadArticles = showReadArticles;
window.toggleStar = toggleStar;
window.toggleQueue = toggleQueue;
window.unparentCategory = unparentCategory;
window.copyNewsletterAddress = copyNewsletterAddress;
window.generateNewsletterAddress = generateNewsletterAddress;
window.updateQueueCacheIfStandalone = updateQueueCacheIfStandalone;
