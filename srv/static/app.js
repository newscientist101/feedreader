// Apply user preferences from localStorage
function applyUserPreferences() {
    // Hide read articles
    const hideRead = localStorage.getItem('hideReadArticles');
    if (hideRead === 'hide') {
        document.querySelectorAll('.article-card.read').forEach(card => {
            card.style.display = 'none';
        });
    }
    
    // Show/hide "all read" message
    updateAllReadMessage();
    
    // Hide empty feeds (but never hide folders - they should always be visible)
    const hideEmpty = localStorage.getItem('hideEmptyFeeds');
    if (hideEmpty === 'hide') {
        // Hide feeds with no unread (but not never-fetched feeds)
        document.querySelectorAll('.feed-item').forEach(item => {
            // Don't hide feeds that have never been fetched
            if (item.dataset.neverFetched === 'true') return;
            const badge = item.querySelector('.badge');
            const count = badge ? parseInt(badge.textContent) : 0;
            if (count === 0) {
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
    
    const hideRead = localStorage.getItem('hideReadArticles') === 'hide';
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
    if (localStorage.getItem('autoMarkRead') !== 'true') {
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
    } catch (e) {
        console.error('Failed to mark article read:', e);
    }
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

// View mode switching
function setView(view) {
    const list = document.getElementById('articles-list');
    if (!list) return;
    
    // Remove all view classes
    list.classList.remove('view-card', 'view-list', 'view-compact', 'view-magazine', 'view-expanded');
    // Add the selected view class
    list.classList.add('view-' + view);
    
    // Update toggle buttons
    document.querySelectorAll('.view-toggle button').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.view === view);
    });
    
    // Save preference
    localStorage.setItem('feedreader-view', view);
}

// Initialize view on page load
function initView() {
    const savedView = localStorage.getItem('feedreader-view') || 'card';
    setView(savedView);
}

// Close sidebar when clicking a link on mobile
document.addEventListener('DOMContentLoaded', () => {
    // Initialize timestamp tooltips with local timezone
    initTimestampTooltips();
    
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
    
    // Initialize view mode
    initView();
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
async function markRead(id) {
    try {
        await api('POST', `/api/articles/${id}/read`);
        const card = document.querySelector(`.article-card[data-id="${id}"]`);
        if (card) card.classList.add('read');
        updateCounts();
    } catch (e) {
        console.error('Failed to mark read:', e);
    }
}

async function markUnread(id) {
    try {
        await api('POST', `/api/articles/${id}/unread`);
        const card = document.querySelector(`.article-card[data-id="${id}"]`);
        if (card) card.classList.remove('read');
        updateCounts();
    } catch (e) {
        console.error('Failed to mark unread:', e);
    }
}

async function toggleStar(id) {
    try {
        await api('POST', `/api/articles/${id}/star`);
        // Toggle star button appearance
        const btns = document.querySelectorAll(`[onclick="toggleStar(${id})"]`);
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
function filterFeeds(query) {
    const rows = document.querySelectorAll('.feeds-table tbody tr');
    const lowerQuery = query.toLowerCase().trim();
    
    rows.forEach(row => {
        if (!lowerQuery) {
            row.style.display = '';
            return;
        }
        
        const name = row.querySelector('td:first-child')?.textContent?.toLowerCase() || '';
        const url = row.querySelector('.url-cell')?.textContent?.toLowerCase() || '';
        
        if (name.includes(lowerQuery) || url.includes(lowerQuery)) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
}

// Edit feed modal
async function editFeed(id) {
    try {
        const feed = await api('GET', `/api/feeds/${id}`);
        document.getElementById('edit-feed-id').value = feed.id;
        document.getElementById('edit-feed-name').value = feed.name;
        document.getElementById('edit-feed-url').value = feed.url;
        document.getElementById('edit-feed-interval').value = feed.fetch_interval_minutes || 60;
        document.getElementById('edit-feed-modal').style.display = 'flex';
    } catch (e) {
        console.error('Failed to load feed:', e);
        alert('Failed to load feed details');
    }
}

function closeEditModal() {
    document.getElementById('edit-feed-modal').style.display = 'none';
}

async function saveFeed(event) {
    event.preventDefault();
    const id = document.getElementById('edit-feed-id').value;
    const name = document.getElementById('edit-feed-name').value;
    const url = document.getElementById('edit-feed-url').value;
    const interval = parseInt(document.getElementById('edit-feed-interval').value) || 60;
    
    try {
        await api('PUT', `/api/feeds/${id}`, {
            name: name,
            url: url,
            fetch_interval_minutes: interval
        });
        closeEditModal();
        location.reload();
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
async function deleteScraper(id, name) {
    if (!confirm(`Delete scraper "${name}"?`)) {
        return;
    }
    try {
        await api('DELETE', `/api/scrapers/${id}`);
        location.reload();
    } catch (e) {
        console.error('Failed to delete scraper:', e);
        alert('Failed to delete scraper');
    }
}

function editScraper(id) {
    // For now, just alert - could load data via API
    alert('Edit functionality coming soon. Delete and recreate for now.');
}

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
                const feed = await api('POST', '/api/feeds', {
                    url,
                    name: name || url,
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

    // Add scraper form
    const addScraperForm = document.getElementById('add-scraper-form');
    if (addScraperForm) {
        addScraperForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const name = document.getElementById('scraper-name').value;
            const description = document.getElementById('scraper-description').value;
            const script = document.getElementById('scraper-script').value;

            // Validate JSON
            try {
                JSON.parse(script);
            } catch (e) {
                alert('Invalid JSON in script field');
                return;
            }

            try {
                await api('POST', '/api/scrapers', {
                    name,
                    description,
                    script,
                    scriptType: 'json'
                });
                location.reload();
            } catch (e) {
                alert('Failed to create scraper: ' + e.message);
            }
        });
    }

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
                <a href="/article/${a.id}">${escapeHtml(a.title)}</a>
            </h2>
            ${a.summary ? `<p class="article-summary">${escapeHtml(truncate(stripHtml(a.summary), 200))}</p>` : ''}
            <div class="article-actions">
                <button onclick="${a.is_read ? 'markUnread' : 'markRead'}(${a.id})" class="btn-icon" title="${a.is_read ? 'Mark unread' : 'Mark read'}">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                        <path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
                    </svg>
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
