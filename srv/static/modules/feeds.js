import { api } from './api.js';
import { showArticlesLoading, renderArticles } from './articles.js';
import { setSidebarActive } from './sidebar.js';
import { applyDefaultViewForScope } from './views.js';
import { updateCounts, updateFeedStatusCell } from './counts.js';
import { showFeedErrorBanner, removeFeedErrorBanner } from './feed-errors.js';

export async function loadCategoryArticles(categoryId, categoryName) {
    showArticlesLoading();

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
        removeFeedErrorBanner();

        applyDefaultViewForScope('folder');
    } catch (e) {
        console.error('Failed to load category articles:', e);
    }
}

export async function loadFeedArticles(feedId, feedName) {
    showArticlesLoading();

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
            editBtn.dataset.action = 'edit-feed';
            editBtn.dataset.feedId = feedId;

            let refreshBtn = headerActions.querySelector('[data-feed-action="refresh"]');
            if (!refreshBtn) {
                refreshBtn = document.createElement('button');
                refreshBtn.className = 'btn btn-warning';
                refreshBtn.dataset.feedAction = 'refresh';
                headerActions.appendChild(refreshBtn);
            }
            refreshBtn.style.display = '';
            refreshBtn.dataset.action = 'refresh-feed';
            refreshBtn.dataset.feedId = feedId;
            refreshBtn.textContent = 'Refresh';
        }

        if (feed && feed.last_error) {
            showFeedErrorBanner(feedId, feed.last_error);
        } else {
            removeFeedErrorBanner();
        }

        applyDefaultViewForScope('feed');
    } catch (e) {
        console.error('Failed to load feed articles:', e);
    }
}

// Feed actions
export async function refreshFeed(id) {
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

export async function deleteFeed(id, name) {
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
export function filterFeeds() {
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
export function createEditFeedModal() {
    // Check if modal already exists
    let modal = document.getElementById('edit-feed-modal');
    if (modal) return modal;

    modal = document.createElement('div');
    modal.id = 'edit-feed-modal';
    modal.className = 'modal';
    modal.style.display = 'none';
    modal.innerHTML = `
        <div class="modal-backdrop" data-action="close-edit-modal"></div>
        <div class="modal-content">
            <div class="modal-header">
                <h3>Edit Feed</h3>
                <button data-action="close-edit-modal" class="btn-icon">
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                        <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                    </svg>
                </button>
            </div>
            <form id="edit-feed-form">
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
                    <button type="button" data-action="close-edit-modal" class="btn btn-secondary">Cancel</button>
                    <button type="submit" class="btn btn-primary">Save</button>
                </div>
            </form>
        </div>
    `;
    document.body.appendChild(modal);
    return modal;
}

export async function editFeed(id) {
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

export function closeEditModal() {
    const modal = document.getElementById('edit-feed-modal');
    if (modal) modal.style.display = 'none';
}

export async function saveFeed(event) {
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

export async function setFeedCategory(feedId, categoryId) {
    try {
        await api('POST', `/api/feeds/${feedId}/category`, { categoryId: parseInt(categoryId) });
    } catch (e) {
        console.error('Failed to set category:', e);
        alert('Failed to move feed');
    }
}

// Delegated listeners for feed action buttons (replaces inline onclick in
// index.html and feeds.html: edit, refresh, retry, delete, filter, category).
export function initFeedActionListeners() {
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="edit-feed"]');
        if (btn) {
            const feedId = Number(btn.dataset.feedId);
            if (feedId) editFeed(feedId);
        }
    });

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="refresh-feed"]');
        if (btn) {
            const feedId = Number(btn.dataset.feedId);
            if (feedId) refreshFeed(feedId);
        }
    });

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="delete-feed"]');
        if (btn) {
            const feedId = Number(btn.dataset.feedId);
            const feedName = btn.dataset.feedName || '';
            if (feedId) deleteFeed(feedId, feedName);
        }
    });

    // Filter feeds: checkbox change and search input
    document.addEventListener('change', (e) => {
        if (e.target.closest('[data-action="filter-feeds"]')) filterFeeds();
    });
    document.addEventListener('input', (e) => {
        if (e.target.closest('[data-action="filter-feeds"]')) filterFeeds();
    });

    // Set feed category from dropdown
    document.addEventListener('change', (e) => {
        const select = e.target.closest('[data-action="set-feed-category"]');
        if (select) {
            const feedId = Number(select.dataset.feedId);
            if (feedId) setFeedCategory(feedId, select.value);
        }
    });

    // Close edit modal (backdrop, close button, cancel button)
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="close-edit-modal"]');
        if (btn) closeEditModal();
    });

    // Edit feed form submit
    document.addEventListener('submit', (e) => {
        if (e.target.id === 'edit-feed-form') {
            saveFeed(e);
        }
    });
}
