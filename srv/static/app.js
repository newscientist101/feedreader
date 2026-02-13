// SVG icons for read/unread toggle
const SVG_MARK_READ = '<svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/></svg>';
const SVG_MARK_UNREAD = '<svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 14H4V8l8 5 8-5v10zm-8-7L4 6h16l-8 5z"/></svg>';

// Update the read/unread toggle button inside an article card
function updateReadButton(card, isRead) {
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

// Sidebar highlighting helper — clears all active states in the sidebar,
// then marks the given element (nav-item, feed-item, or folder-item) active.
function setSidebarActive(el) {
    document.querySelectorAll('.sidebar .active').forEach(a => a.classList.remove('active'));
    if (el) el.classList.add('active');
}

// User settings (injected from server, saved via API)
function getSetting(key, defaultValue) {
    const val = (window.__settings || {})[key];
    return val !== undefined ? val : (defaultValue || '');
}

function saveSetting(key, value) {
    if (!window.__settings) window.__settings = {};
    window.__settings[key] = value;
    fetch('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [key]: value }),
    }).catch(e => console.error('Failed to save setting:', e));
}

// Apply user preferences from settings
function applyUserPreferences() {
    // Hide read articles
    const hideRead = getSetting('hideReadArticles');
    if (hideRead === 'hide') {
        document.querySelectorAll('.article-card.read').forEach(card => {
            card.style.display = 'none';
        });
    }
    
    // Show/hide "all read" message
    updateAllReadMessage();
    
    // Hide empty feeds (but never hide folders - they should always be visible)
    const hideEmpty = getSetting('hideEmptyFeeds');
    if (hideEmpty === 'hide') {
        // Hide feeds with no unread (but not never-fetched feeds)
        document.querySelectorAll('.feed-item').forEach(item => {
            // Don't hide feeds that have never been fetched
            if (item.dataset.neverFetched === 'true') return;
            const badge = item.querySelector('.badge');
            const count = badge ? parseInt(badge.textContent || '0', 10) : 0;
            if (!count) {
                item.style.display = 'none';
            }
        });
        // Folders are always visible, even when empty
    }
}

// Show message when all articles are read and hidden
function updateAllReadMessage() {
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
function showReadArticles() {
    document.querySelectorAll('.article-card.read').forEach(card => {
        card.style.display = '';
    });
    const msg = document.getElementById('all-read-message');
    if (msg) msg.remove();
}

// Auto-mark-read on scroll feature
let autoMarkReadObserver = null;

function initAutoMarkRead() {
    if (getSetting('autoMarkRead') !== 'true') {
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
                    markReadSilent(articleId);
                    article.classList.add('read');
                    updateUnreadCounts(-1);
                }
            }
        });
    }, {
        root: null,
        rootMargin: '0px',
        threshold: 0
    });
    
    // Observe all article cards
    document.querySelectorAll('.article-card').forEach(article => {
        autoMarkReadObserver.observe(article);
    });
}

// Mark as read without page reload (for auto-mark feature)
async function markReadSilent(id) {
    try {
        await api('POST', `/api/articles/${id}/read`);
        const card = document.querySelector(`.article-card[data-id="${id}"]`);
        if (card) {
            card.classList.add('read');
            updateReadButton(card, true);
        }
        updateCounts();
    } catch (e) {
        console.error('Failed to mark article read:', e);
    }
}

function openArticle(id) {
    markReadSilent(id);
    window.location = `/article/${id}`;
}

function openArticleExternal(event, id, url) {
    event.stopPropagation();
    markReadSilent(id);
    window.open(url, '_blank');
}

// Update unread counts in sidebar
function updateUnreadCounts(delta) {
    // Update main unread count
    const unreadBadge = document.querySelector('.nav-item .badge');
    if (unreadBadge) {
        const current = parseInt(unreadBadge.textContent) || 0;
        const newCount = Math.max(0, current + delta);
        unreadBadge.textContent = newCount;
    }
    
    // Update feed counts would require knowing which feed, so we skip for simplicity
}

// Format timestamps in user's local timezone
function formatLocalDate(isoString) {
    const date = new Date(isoString);
    return date.toLocaleDateString(undefined, {
        weekday: 'long',
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit'
    });
}

function initTimestampTooltips() {
    document.querySelectorAll('[data-timestamp]').forEach(el => {
        const timestamp = el.dataset.timestamp;
        if (timestamp) {
            el.title = formatLocalDate(timestamp);
        }
    });
}

// Dropdown toggle
function toggleDropdown(btn) {
    const dropdown = btn.closest('.dropdown');
    const wasOpen = dropdown.classList.contains('open');
    
    // Close all dropdowns
    document.querySelectorAll('.dropdown.open').forEach(d => d.classList.remove('open'));
    
    // Toggle this one
    if (!wasOpen) {
        dropdown.classList.add('open');
    }
}

// Close dropdowns when clicking outside
document.addEventListener('click', (e) => {
    if (!e.target.closest('.dropdown')) {
        document.querySelectorAll('.dropdown.open').forEach(d => d.classList.remove('open'));
    }
});

// Mobile sidebar toggle
function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const overlay = document.querySelector('.sidebar-overlay');
    sidebar.classList.toggle('open');
    overlay.classList.toggle('active');
    document.body.style.overflow = sidebar.classList.contains('open') ? 'hidden' : '';
}

// Toggle folder expand/collapse in sidebar
function navigateFolder(event, categoryId) {
    // If not on the main articles page, use regular navigation
    if (!document.getElementById('articles-list')) {
        return true;
    }
    event.preventDefault();
    const folderItem = document.querySelector(`.folder-item[data-category-id="${categoryId}"]`);
    if (!folderItem) return false;
    
    // Load category articles via AJAX (also updates active state)
    loadCategoryArticles(categoryId, folderItem.querySelector('.folder-name')?.textContent || 'Category');
    
    return false;
}

function toggleFolderCollapse(categoryId) {
    const folderItem = document.querySelector(`.folder-item[data-category-id="${categoryId}"]`);
    if (!folderItem) return;
    if (folderItem.classList.contains('expanded')) {
        collapseFolder(folderItem);
    } else {
        folderItem.classList.add('expanded');
    }
}

function collapseFolder(folderItem) {
    folderItem.classList.remove('expanded');
    // Collapse nested subfolders and clear their active/expanded state
    folderItem.querySelectorAll('.folder-item.expanded').forEach(child => {
        child.classList.remove('expanded', 'active');
    });
    // Clear active from any nested feeds/folders that are now hidden
    folderItem.querySelectorAll('.feed-item.active, .folder-item.active').forEach(child => {
        child.classList.remove('active');
    });
}

async function loadCategoryArticles(categoryId, categoryName) {
    // Show loading state immediately
    const list = document.getElementById('articles-list');
    if (list) {
        list.innerHTML = `
            <div class="loading-state">
                <div class="spinner"></div>
                <p>Loading articles...</p>
            </div>
        `;
    }
    
    // Update page title immediately for responsiveness
    document.querySelector('.view-header h1').textContent = categoryName;
    document.title = `${categoryName} - FeedReader`;

    const articlesView = document.querySelector('.articles-view');
    if (articlesView) {
        articlesView.dataset.viewScope = 'folder';
    }
    
    // Update active states in sidebar
    const folderItem = document.querySelector(`.folder-item[data-category-id="${categoryId}"]`);
    setSidebarActive(folderItem);
    
    try {
        const data = await api('GET', `/api/categories/${categoryId}/articles`);
        
        // Update URL without reload
        history.pushState({ categoryId }, categoryName, `/category/${categoryId}`);
        
        // Render articles
        renderArticles(data.articles);
        
        // Update the Mark as Read dropdown
        const dropdown = document.querySelector('.dropdown');
        if (dropdown) {
            dropdown.dataset.feedId = '';
            dropdown.dataset.categoryId = categoryId;
        }
        
        // Hide the Refresh/Edit buttons (they're only for feeds)
        document.querySelectorAll('[data-feed-action]').forEach(btn => {
            btn.style.display = 'none';
        });
        
        // Remove any feed error banner
        const errorBanner = document.querySelector('.feed-error-banner');
        if (errorBanner) errorBanner.remove();

        applyDefaultViewForScope('folder');
    } catch (e) {
        console.error('Failed to load category articles:', e);
    }
}

async function loadFeedArticles(feedId, feedName) {
    const list = document.getElementById('articles-list');
    if (list) {
        list.innerHTML = `
            <div class="loading-state">
                <div class="spinner"></div>
                <p>Loading articles...</p>
            </div>
        `;
    }

    document.querySelector('.view-header h1').textContent = feedName;
    document.title = `${feedName} - FeedReader`;

    const articlesView = document.querySelector('.articles-view');
    if (articlesView) {
        articlesView.dataset.viewScope = 'feed';
    }

    // Clear all sidebar active states, then activate matching feed items
    setSidebarActive(null);
    document.querySelectorAll(`.feed-item[data-feed-id="${feedId}"]`).forEach(item => item.classList.add('active'));

    try {
        const data = await api('GET', `/api/feeds/${feedId}/articles`);
        const feed = data.feed;

        history.pushState({ feedId }, feedName, `/feed/${feedId}`);

        renderArticles(data.articles);

        const dropdown = document.querySelector('.dropdown');
        if (dropdown) {
            dropdown.dataset.feedId = feedId;
            dropdown.dataset.categoryId = '';
        }

        const headerActions = document.querySelector('.header-actions');
        if (headerActions) {
            let editBtn = headerActions.querySelector('[data-feed-action="edit"]');
            if (!editBtn) {
                editBtn = document.createElement('button');
                editBtn.className = 'btn btn-secondary';
                editBtn.dataset.feedAction = 'edit';
                editBtn.textContent = 'Edit';
                headerActions.appendChild(editBtn);
            }
            editBtn.style.display = '';
            editBtn.onclick = () => editFeed(feedId);

            let refreshBtn = headerActions.querySelector('[data-feed-action="refresh"]');
            if (!refreshBtn) {
                refreshBtn = document.createElement('button');
                refreshBtn.className = 'btn btn-warning';
                refreshBtn.dataset.feedAction = 'refresh';
                headerActions.appendChild(refreshBtn);
            }
            refreshBtn.style.display = '';
            refreshBtn.dataset.feedId = feedId;
            refreshBtn.textContent = 'Refresh';
            refreshBtn.onclick = () => refreshFeed(feedId);
        }

        const errorBanner = document.querySelector('.feed-error-banner');
        if (feed && feed.last_error) {
            if (!errorBanner) {
                const banner = document.createElement('div');
                banner.className = 'feed-error-banner';
                banner.innerHTML = `
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
                    </svg>
                    <span>Last fetch failed: ${feed.last_error}</span>
                    <button class="btn btn-small" data-feed-id="${feedId}">Retry</button>
                `;
                const retryBtn = banner.querySelector('button');
                retryBtn.onclick = () => refreshFeed(feedId);
                const view = document.querySelector('.articles-view');
                if (view) {
                    view.insertBefore(banner, view.firstChild);
                }
            } else {
                const span = errorBanner.querySelector('span');
                if (span) span.textContent = 'Last fetch failed: ' + feed.last_error;
            }
        } else if (errorBanner) {
            errorBanner.remove();
        }

        applyDefaultViewForScope('feed');
    } catch (e) {
        console.error('Failed to load feed articles:', e);
    }
}

function renderArticles(articles) {
    const list = document.getElementById('articles-list');
    if (!list) return;
    
    if (!articles || articles.length === 0) {
        list.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor" opacity="0.3">
                    <path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
                </svg>
                <p>No articles to show</p>
            </div>
        `;
        return;
    }
    
    // Build HTML in chunks to avoid UI blocking
    const html = articles.map(a => `
        <article class="article-card ${a.is_read ? 'read' : ''}${a.image_url ? ' has-image' : ''}" data-id="${a.id}">
            ${a.image_url ? `<div class="article-image magazine-expanded-only"><img src="${a.image_url}" alt="" loading="lazy"></div>` : `<div class="article-image-placeholder magazine-only">
                <svg viewBox="0 0 24 24" width="32" height="32" fill="currentColor">
                    <path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z"/>
                </svg>
            </div>`}
            <div class="article-body" onclick="openArticle(${a.id})" style="cursor: pointer;">
                <div class="article-meta">
                    <a class="feed-name" href="/feed/${a.feed_id}" onclick="event.stopPropagation();">${a.feed_name || ''}</a>
                    ${a.author ? `<span class="article-author">${a.author}</span>` : ''}
                    <span class="article-date">${formatTimeAgo(a.published_at)}</span>
                </div>
                <h2 class="article-title">
                    ${a.url ? `<a href="${a.url}" target="_blank" onclick="openArticleExternal(event, ${a.id}, '${a.url.replace(/'/g, "\\'")}'">${a.title}</a>` : `<a href="/article/${a.id}" onclick="markReadSilent(${a.id})">${a.title}</a>`}
                </h2>
                ${a.summary ? `<p class="article-summary">${truncateText(stripHtml(a.summary), 200)}</p>` : ''}
                ${a.content ? `<div class="article-content-preview expanded-only" onclick="event.stopPropagation(); markReadSilent(${a.id})">${truncateText(stripHtml(a.content), 800)}</div>` : (a.summary ? `<div class="article-content-preview expanded-only">${truncateText(stripHtml(a.summary), 800)}</div>` : '')}
                <div class="article-actions">
                    <button onclick="${a.is_read ? 'markUnread' : 'markRead'}(event, ${a.id})" class="btn-icon btn-read-toggle" title="${a.is_read ? 'Mark unread' : 'Mark read'}">
                        ${a.is_read ? SVG_MARK_UNREAD : SVG_MARK_READ}
                    </button>
                    <button onclick="toggleStar(${a.id})" class="btn-icon ${a.is_starred ? 'starred' : ''}" title="Star">
                        <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                            ${a.is_starred
                                ? '<path d="M12 17.27L18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z"/>'
                                : '<path d="M22 9.24l-7.19-.62L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21 12 17.27 18.18 21l-1.63-7.03L22 9.24zM12 15.4l-3.76 2.27 1-4.28-3.32-2.88 4.38-.38L12 6.1l1.71 4.04 4.38.38-3.32 2.88 1 4.28L12 15.4z"/>'
                            }
                        </svg>
                    </button>
                    ${a.url ? `<a href="${a.url}" target="_blank" class="btn-icon" title="Open original">
                        <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                            <path d="M19 19H5V5h7V3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2v-7h-2v7zM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7z"/>
                        </svg>
                    </a>` : ''}
                </div>
            </div>
        </article>
    `).join('');
    
    list.innerHTML = html;
    
    // Process embeds in expanded view content previews
    list.querySelectorAll('.article-content-preview').forEach(el => processEmbeds(el));
    
    // Re-apply user preferences (hide read, etc.)
    applyUserPreferences();
}

function formatTimeAgo(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);
    
    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
}

function stripHtml(html) {
    const tmp = document.createElement('div');
    tmp.innerHTML = html;
    return tmp.textContent || tmp.innerText || '';
}

function truncateText(text, maxLen) {
    if (!text || text.length <= maxLen) return text;
    return text.substring(0, maxLen) + '...';
}

// View mode switching
function getViewScope() {
    const view = document.querySelector('.articles-view');
    if (!view) return 'all';
    return view.dataset.viewScope || 'all';
}

function setView(view) {
    // Compact view was removed; fall back to list
    if (view === 'compact') view = 'list';

    const list = document.getElementById('articles-list');
    if (!list) return;

    // Remove all view classes
    list.classList.remove('view-card', 'view-list', 'view-magazine', 'view-expanded');
    // Add the selected view class
    list.classList.add('view-' + view);

    // Update toggle buttons
    document.querySelectorAll('.view-toggle button').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.view === view);
    });

    // Save preference (scope-aware)
    const scope = getViewScope();
    if (scope === 'folder') {
        saveSetting('defaultFolderView', view);
    } else if (scope === 'feed') {
        saveSetting('defaultFeedView', view);
    } else {
        saveSetting('defaultView', view);
    }
}

function migrateLegacyViewDefaults() {
    // Migrate any localStorage settings to the server
    const keys = ['autoMarkRead', 'hideReadArticles', 'hideEmptyFeeds', 'defaultFolderView', 'defaultFeedView'];
    const localMap = {
        'feedreader-view-folder-default': 'defaultFolderView',
        'feedreader-view-feed-default': 'defaultFeedView',
        'feedreader-view': 'defaultView',
    };
    let migrated = false;
    for (const key of keys) {
        if (!getSetting(key) && localStorage.getItem(key)) {
            saveSetting(key, localStorage.getItem(key));
            localStorage.removeItem(key);
            migrated = true;
        }
    }
    for (const [oldKey, newKey] of Object.entries(localMap)) {
        if (!getSetting(newKey) && localStorage.getItem(oldKey)) {
            saveSetting(newKey, localStorage.getItem(oldKey));
            localStorage.removeItem(oldKey);
            migrated = true;
        }
    }
}

function getDefaultViewForScope(scope) {
    if (scope === 'folder') {
        return getSetting('defaultFolderView') || 'card';
    }
    if (scope === 'feed') {
        return getSetting('defaultFeedView') || 'card';
    }
    return getSetting('defaultView') || 'card';
}

function applyDefaultViewForScope(scope) {
    const savedView = getDefaultViewForScope(scope);
    setView(savedView);
}

// Initialize view on page load
function initView() {
    migrateLegacyViewDefaults();
    applyDefaultViewForScope(getViewScope());
}

// Close sidebar when clicking a link on mobile
document.addEventListener('DOMContentLoaded', () => {
    // Initialize timestamp tooltips with local timezone
    initTimestampTooltips();
    
    // Process embeds in article page content
    processEmbeds(document.querySelector('.article-body'));
    
    // Initialize auto-mark-read on scroll
    initAutoMarkRead();
    
    // Apply user preferences
    applyUserPreferences();
    
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
            if (document.querySelector('.settings-view')) {
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
});

// API helpers
async function api(method, url, data = null) {
    const options = {
        method,
        headers: { 'Content-Type': 'application/json' },
    };
    if (data) {
        options.body = JSON.stringify(data);
    }
    const res = await fetch(url, options);
    const json = await res.json();
    if (!res.ok) {
        throw new Error(json.error || 'Request failed');
    }
    return json;
}

// Article actions
async function markRead(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/read`);
        const card = document.querySelector(`.article-card[data-id="${id}"]`);
        if (card) {
            card.classList.add('read');
            updateReadButton(card, true);
        }
        updateCounts();
    } catch (e) {
        console.error('Failed to mark read:', e);
    }
}

async function markUnread(event, id) {
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
    }
}

async function toggleStar(event, id) {
    if (event) event.stopPropagation();
    try {
        await api('POST', `/api/articles/${id}/star`);
        // Toggle star button appearance
        const btns = document.querySelectorAll(`[onclick="toggleStar(${id})"], [onclick="toggleStar(event, ${id})"]`);
        btns.forEach(btn => btn.classList.toggle('starred'));
        updateCounts();
    } catch (e) {
        console.error('Failed to toggle star:', e);
    }
}

// Feed actions
async function refreshFeed(id) {
    // Find and update all refresh buttons for this feed
    const buttons = document.querySelectorAll(`button[data-feed-id="${id}"]`);
    const originalContents = [];
    
    buttons.forEach((btn, i) => {
        originalContents[i] = btn.innerHTML;
        btn.disabled = true;
        btn.innerHTML = '<span class="spinner"></span> Fetching...';
    });
    
    try {
        // Get current status to compare later
        const beforeStatus = await api('GET', `/api/feeds/${id}/status`);
        const beforeFetched = beforeStatus.lastFetched;
        
        // Start the refresh
        await api('POST', `/api/feeds/${id}/refresh`);
        
        // Poll for completion
        let attempts = 0;
        const maxAttempts = 30; // 30 seconds max
        
        const checkStatus = async () => {
            attempts++;
            const status = await api('GET', `/api/feeds/${id}/status`);
            
            // Check if fetch completed (timestamp changed)
            if (status.lastFetched !== beforeFetched) {
                // Fetch completed
                buttons.forEach((btn, i) => {
                    btn.disabled = false;
                    if (status.lastError) {
                        btn.innerHTML = '<span class="error-icon">✗</span> Error';
                        btn.title = status.lastError;
                    } else {
                        btn.innerHTML = '<span class="success-icon">✓</span> Done';
                    }
                    // Restore original after 2 seconds
                    setTimeout(() => {
                        btn.innerHTML = originalContents[i];
                        btn.title = '';
                    }, 2000);
                });
                
                // Update status cell in the feeds table if present
                updateFeedStatusCell(id, status.lastError);
                
                updateCounts();
                return;
            }
            
            if (attempts < maxAttempts) {
                setTimeout(checkStatus, 1000);
            } else {
                // Timeout - restore buttons
                buttons.forEach((btn, i) => {
                    btn.disabled = false;
                    btn.innerHTML = originalContents[i];
                });
                updateCounts();
            }
        };
        
        // Start polling after 1 second
        setTimeout(checkStatus, 1000);
        
    } catch (e) {
        console.error('Failed to refresh feed:', e);
        buttons.forEach((btn, i) => {
            btn.disabled = false;
            btn.innerHTML = '<span class="error-icon">✗</span> Failed';
            setTimeout(() => {
                btn.innerHTML = originalContents[i];
            }, 2000);
        });
    }
}

async function deleteFeed(id, name) {
    if (!confirm(`Delete feed "${name}"? This will also delete all its articles.`)) {
        return;
    }
    try {
        await api('DELETE', `/api/feeds/${id}`);
        location.reload();
    } catch (e) {
        console.error('Failed to delete feed:', e);
        alert('Failed to delete feed');
    }
}

// Filter feeds table by search query
function filterFeeds() {
    const rows = document.querySelectorAll('.feeds-table tbody tr');
    const searchInput = document.getElementById('feeds-search');
    const errorsCheckbox = document.getElementById('filter-errors');
    
    const query = (searchInput?.value || '').toLowerCase().trim();
    const showOnlyErrors = errorsCheckbox?.checked || false;
    
    rows.forEach(row => {
        let show = true;
        
        // Filter by errors
        if (showOnlyErrors && !row.dataset.hasError) {
            show = false;
        }
        
        // Filter by search query
        if (show && query) {
            const name = row.querySelector('td:first-child')?.textContent?.toLowerCase() || '';
            const url = row.querySelector('.url-cell')?.textContent?.toLowerCase() || '';
            if (!name.includes(query) && !url.includes(query)) {
                show = false;
            }
        }
        
        row.style.display = show ? '' : 'none';
    });
}

// Edit feed modal
// Create and show the edit feed modal
function createEditFeedModal() {
    // Check if modal already exists
    let modal = document.getElementById('edit-feed-modal');
    if (modal) return modal;
    
    modal = document.createElement('div');
    modal.id = 'edit-feed-modal';
    modal.className = 'modal';
    modal.style.display = 'none';
    modal.innerHTML = `
        <div class="modal-backdrop" onclick="closeEditModal()"></div>
        <div class="modal-content">
            <div class="modal-header">
                <h3>Edit Feed</h3>
                <button onclick="closeEditModal()" class="btn-icon">
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                        <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                    </svg>
                </button>
            </div>
            <form id="edit-feed-form" onsubmit="saveFeed(event)">
                <input type="hidden" id="edit-feed-id">
                <div class="form-group">
                    <label for="edit-feed-name">Name</label>
                    <input type="text" id="edit-feed-name" required>
                </div>
                <div class="form-group">
                    <label for="edit-feed-url">URL</label>
                    <input type="url" id="edit-feed-url" required>
                </div>
                <div class="form-group">
                    <label for="edit-feed-interval">Refresh Interval (minutes)</label>
                    <input type="number" id="edit-feed-interval" min="1" value="60">
                </div>
                <div class="form-group">
                    <label for="edit-feed-filters">Content Filters (CSS Selectors)</label>
                    <p class="form-hint">Remove elements from article content. One CSS selector per line.</p>
                    <textarea id="edit-feed-filters" rows="4" placeholder="div.ref-ar&#10;.advertisement&#10;#sidebar"></textarea>
                </div>
                <div class="modal-actions">
                    <button type="button" onclick="closeEditModal()" class="btn btn-secondary">Cancel</button>
                    <button type="submit" class="btn btn-primary">Save</button>
                </div>
            </form>
        </div>
    `;
    document.body.appendChild(modal);
    return modal;
}

async function editFeed(id) {
    try {
        const feed = await api('GET', `/api/feeds/${id}`);
        const modal = createEditFeedModal();
        document.getElementById('edit-feed-id').value = feed.id;
        document.getElementById('edit-feed-name').value = feed.name;
        document.getElementById('edit-feed-url').value = feed.url;
        document.getElementById('edit-feed-interval').value = feed.fetch_interval_minutes || 60;
        
        // Load content filters (CSS selectors)
        const filtersTextarea = document.getElementById('edit-feed-filters');
        if (feed.content_filters) {
            try {
                const filters = JSON.parse(feed.content_filters);
                filtersTextarea.value = filters.map(f => f.selector).join('\n');
            } catch (e) {
                filtersTextarea.value = '';
            }
        } else {
            filtersTextarea.value = '';
        }
        
        modal.style.display = 'flex';
    } catch (e) {
        console.error('Failed to load feed:', e);
        alert('Failed to load feed details');
    }
}

function closeEditModal() {
    const modal = document.getElementById('edit-feed-modal');
    if (modal) modal.style.display = 'none';
}

async function saveFeed(event) {
    event.preventDefault();
    const id = document.getElementById('edit-feed-id').value;
    const name = document.getElementById('edit-feed-name').value;
    const url = document.getElementById('edit-feed-url').value;
    const interval = parseInt(document.getElementById('edit-feed-interval').value) || 60;
    
    // Parse content filters (CSS selectors) from textarea
    const filtersText = document.getElementById('edit-feed-filters').value.trim();
    let contentFilters = null;
    if (filtersText) {
        const selectors = filtersText.split('\n').map(line => line.trim()).filter(line => line);
        const filters = selectors.map(selector => ({ selector }));
        contentFilters = JSON.stringify(filters);
    }
    
    try {
        await api('PUT', `/api/feeds/${id}`, {
            name: name,
            url: url,
            fetch_interval_minutes: interval,
            content_filters: contentFilters
        });
        closeEditModal();
        
        // Update the row in place instead of reloading
        const row = document.querySelector(`tr[data-feed-id="${id}"]`);
        if (row) {
            const cells = row.querySelectorAll('td');
            if (cells[0]) {
                const link = cells[0].querySelector('a');
                if (link) link.textContent = name;
            }
            if (cells[1]) {
                const link = cells[1].querySelector('a');
                if (link) {
                    link.href = url;
                    link.textContent = url.length > 40 ? url.substring(0, 40) + '...' : url;
                }
            }
        }
        
        // Also update sidebar if visible
        const sidebarItem = document.querySelector(`.feed-item[href="/feed/${id}"] .feed-name`);
        if (sidebarItem) {
            sidebarItem.textContent = name;
        }
        
        // Update page title and header if on feed page
        const pageHeader = document.querySelector('.view-header h1');
        if (pageHeader && window.location.pathname === `/feed/${id}`) {
            pageHeader.textContent = name;
            document.title = `${name} - FeedReader`;
        }
    } catch (e) {
        console.error('Failed to save feed:', e);
        alert('Failed to save feed');
    }
}

async function markAsRead(btn, age = 'all') {
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
        location.reload();
    } catch (e) {
        console.error('Failed to mark as read:', e);
    }
}

// Scraper actions
// deleteScraper is defined in scrapers.html template

// editScraper is defined in scrapers.html template

function closeModal() {
    document.getElementById('edit-modal').style.display = 'none';
}

// Category functions
async function createCategory() {
    const name = prompt('Enter folder name:');
    if (!name) return;
    
    try {
        await api('POST', '/api/categories', { name });
        location.reload();
    } catch (e) {
        alert('Failed to create folder: ' + e.message);
    }
}

async function renameCategory(id, currentName) {
    const name = prompt('Enter new name:', currentName);
    if (!name || name === currentName) return;
    
    try {
        await api('PUT', `/api/categories/${id}`, { name });
        location.reload();
    } catch (e) {
        alert('Failed to rename folder: ' + e.message);
    }
}

async function unparentCategory(id) {
    try {
        await api('POST', `/api/categories/${id}/parent`, {
            parent_id: null,
            sort_order: 999 // Put at end
        });
        location.reload();
    } catch (e) {
        console.error('Failed to unparent category:', e);
        alert('Failed to move folder to top level');
    }
}

async function deleteCategory(id, name) {
    if (!confirm(`Delete folder "${name}"? Feeds will be moved to uncategorized.`)) {
        return;
    }
    try {
        await api('DELETE', `/api/categories/${id}`);
        location.reload();
    } catch (e) {
        alert('Failed to delete folder: ' + e.message);
    }
}

async function setFeedCategory(feedId, categoryId) {
    try {
        await api('POST', `/api/feeds/${feedId}/category`, { categoryId: parseInt(categoryId) });
    } catch (e) {
        console.error('Failed to set category:', e);
        alert('Failed to move feed');
    }
}

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

async function updateCounts() {
    try {
        const counts = await api('GET', '/api/counts');
        
        // Update unread count
        const unreadBadge = document.querySelector('[data-count="unread"]');
        if (unreadBadge) {
            unreadBadge.textContent = counts.unread || '';
        }
        
        // Update starred count
        const starredBadge = document.querySelector('[data-count="starred"]');
        if (starredBadge) {
            starredBadge.textContent = counts.starred || '';
        }
        
        // Update category counts
        if (counts.categories) {
            for (const [catId, count] of Object.entries(counts.categories)) {
                const badge = document.querySelector(`[data-count="category-${catId}"]`);
                if (badge) {
                    badge.textContent = count || '';
                }
            }
        }
        
        // Update feed counts and errors
        if (counts.feeds) {
            for (const [feedId, count] of Object.entries(counts.feeds)) {
                const badges = document.querySelectorAll(`[data-count="feed-${feedId}"]`);
                badges.forEach(badge => {
                    // Don't overwrite pending indicator for never-fetched feeds
                    if (!badge.classList.contains('pending') || count > 0) {
                        badge.textContent = count || '';
                        badge.classList.remove('pending');
                    }
                });
            }
        }
        
        // Update feed errors
        updateFeedErrors(counts.feedErrors || {});
        
        // Re-apply hide empty preference if enabled
        applyUserPreferences();
    } catch (e) {
        console.error('Failed to update counts:', e);
    }
}

// Update feed error indicators in sidebar
// Update status cell in the Manage Feeds table
function updateFeedStatusCell(feedId, lastError) {
    const row = document.querySelector(`tr[data-feed-id="${feedId}"]`);
    if (!row) return;
    
    // Find the status cell (second to last column)
    const cells = row.querySelectorAll('td');
    const statusCell = cells[cells.length - 2]; // Status is before Actions
    if (!statusCell) return;
    
    if (lastError) {
        statusCell.innerHTML = `<span class="status-error" title="${lastError}">Error</span>`;
        row.dataset.hasError = 'true';
    } else {
        statusCell.innerHTML = '<span class="status-ok">OK</span>';
        delete row.dataset.hasError;
    }
}

function updateFeedErrors(feedErrors) {
    // Get all feed items in sidebar
    const feedItems = document.querySelectorAll('.feed-item[data-feed-id]');
    
    feedItems.forEach(item => {
        const feedId = item.dataset.feedId;
        const errorIcon = item.querySelector('[data-error]');
        const hasError = feedErrors[feedId];
        
        if (hasError) {
            item.classList.add('has-error');
            item.title = 'Error: ' + feedErrors[feedId];
            if (errorIcon) {
                errorIcon.textContent = '⚠';
                errorIcon.title = 'Fetch error';
            }
        } else {
            item.classList.remove('has-error');
            item.title = '';
            if (errorIcon) {
                errorIcon.textContent = '';
                errorIcon.title = '';
            }
        }
    });
    
    // Also update the error banner on the current feed page if present
    const errorBanner = document.querySelector('.feed-error-banner');
    const currentFeedId = document.querySelector('button[data-feed-id]')?.dataset.feedId;
    if (currentFeedId) {
        if (feedErrors[currentFeedId]) {
            if (!errorBanner) {
                // Create error banner if it doesn't exist
                const banner = document.createElement('div');
                banner.className = 'feed-error-banner';
                banner.innerHTML = `
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
                    </svg>
                    <span>Last fetch failed: ${feedErrors[currentFeedId]}</span>
                    <button onclick="refreshFeed(${currentFeedId})" class="btn btn-small" data-feed-id="${currentFeedId}">Retry</button>
                `;
                const articlesView = document.querySelector('.articles-view');
                if (articlesView) {
                    articlesView.insertBefore(banner, articlesView.firstChild);
                }
            } else {
                // Update existing banner
                const span = errorBanner.querySelector('span');
                if (span) span.textContent = 'Last fetch failed: ' + feedErrors[currentFeedId];
            }
        } else if (errorBanner) {
            // Remove error banner if error is cleared
            errorBanner.remove();
        }
    }
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
                // For HuggingFace feeds, let the server auto-generate the name
                const feedName = (feedType === 'huggingface' && !name) ? '' : (name || url);
                const feed = await api('POST', '/api/feeds', {
                    url,
                    name: feedName,
                    feedType,
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
        searchInput.addEventListener('input', (e) => {
            clearTimeout(timeout);
            timeout = setTimeout(async () => {
                const q = e.target.value.trim();
                if (q.length < 2) {
                    location.reload();
                    return;
                }
                try {
                    const articles = await api('GET', `/api/search?q=${encodeURIComponent(q)}`);
                    renderSearchResults(articles);
                } catch (e) {
                    console.error('Search failed:', e);
                }
            }, 300);
        });
    }
});

function renderSearchResults(articles) {
    const list = document.getElementById('articles-list');
    if (!list) return;

    if (!articles || articles.length === 0) {
        list.innerHTML = '<div class="empty-state"><p>No results found</p></div>';
        return;
    }

    list.innerHTML = articles.map(a => `
        <article class="article-card ${a.is_read ? 'read' : ''}" data-id="${a.id}">
            <div class="article-meta">
                <span class="feed-name">${a.feed_name || 'Unknown'}</span>
                <span class="article-date">${formatDate(a.published_at)}</span>
            </div>
            <h2 class="article-title">
                <a href="/article/${a.id}" onclick="markReadSilent(${a.id})">${escapeHtml(a.title)}</a>
            </h2>
            ${a.summary ? `<p class="article-summary">${escapeHtml(truncate(stripHtml(a.summary), 200))}</p>` : ''}
            <div class="article-actions">
                <button onclick="${a.is_read ? 'markUnread' : 'markRead'}(event, ${a.id})" class="btn-icon btn-read-toggle" title="${a.is_read ? 'Mark unread' : 'Mark read'}">
                    ${a.is_read ? SVG_MARK_UNREAD : SVG_MARK_READ}
                </button>
                <button onclick="toggleStar(${a.id})" class="btn-icon ${a.is_starred ? 'starred' : ''}" title="Star">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                        <path d="M12 17.27L18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z"/>
                    </svg>
                </button>
                ${a.url ? `
                <a href="${a.url}" target="_blank" class="btn-icon" title="Open original">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                        <path d="M19 19H5V5h7V3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2v-7h-2v7zM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7z"/>
                    </svg>
                </a>
                ` : ''}
            </div>
        </article>
    `).join('');
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function stripHtml(str) {
    if (!str) return '';
    return str.replace(/<[^>]*>/g, '');
}

function truncate(str, n) {
    if (!str || str.length <= n) return str;
    return str.slice(0, n) + '...';
}

function formatDate(dateStr) {
    if (!dateStr) return '';
    const d = new Date(dateStr);
    const now = new Date();
    const diff = now - d;
    
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return Math.floor(diff / 60000) + ' min ago';
    if (diff < 86400000) return Math.floor(diff / 3600000) + ' hours ago';
    if (diff < 604800000) return Math.floor(diff / 86400000) + ' days ago';
    return d.toLocaleDateString();
}

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

function getDragAfterElement(container, y, itemSelector) {
    const draggableElements = [...container.querySelectorAll(itemSelector + ':not(.dragging)')];
    
    return draggableElements.reduce((closest, child) => {
        const box = child.getBoundingClientRect();
        const offset = y - box.top - box.height / 2;
        
        if (offset < 0 && offset > closest.offset) {
            return { offset, element: child };
        } else {
            return closest;
        }
    }, { offset: Number.NEGATIVE_INFINITY }).element;
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
function processEmbeds(container) {
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

function extractYouTubeId(url) {
    if (!url) return null;
    // youtube.com/watch?v=ID, youtube.com/shorts/ID, youtube.com/embed/ID, youtu.be/ID
    const m = url.match(/(?:youtube\.com\/(?:watch\?v=|shorts\/|embed\/)|youtu\.be\/)([\w-]{11})/);
    return m ? m[1] : null;
}


// Prevent starting a drag when clicking chevrons.
document.addEventListener('dragstart', (event) => {
    if (event.target.closest('.folder-chevron')) {
        event.preventDefault();
    }
}, true);

