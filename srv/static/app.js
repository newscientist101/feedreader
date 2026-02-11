// Mobile sidebar toggle
function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const overlay = document.querySelector('.sidebar-overlay');
    sidebar.classList.toggle('open');
    overlay.classList.toggle('active');
    document.body.style.overflow = sidebar.classList.contains('open') ? 'hidden' : '';
}

// Close sidebar when clicking a link on mobile
document.addEventListener('DOMContentLoaded', () => {
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
    try {
        await api('POST', `/api/feeds/${id}/refresh`);
        alert('Feed refresh started!');
    } catch (e) {
        console.error('Failed to refresh feed:', e);
        alert('Failed to refresh feed');
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

async function markFeedRead(id) {
    try {
        await api('POST', `/api/feeds/${id}/read-all`);
        location.reload();
    } catch (e) {
        console.error('Failed to mark feed read:', e);
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

async function markCategoryRead(id) {
    try {
        await api('POST', `/api/categories/${id}/read-all`);
        location.reload();
    } catch (e) {
        console.error('Failed to mark category read:', e);
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

function updateCounts() {
    // Reload page to get fresh counts
    // Could optimize with API call later
}

// Form handlers
document.addEventListener('DOMContentLoaded', () => {
    // Add feed form
    const addFeedForm = document.getElementById('add-feed-form');
    if (addFeedForm) {
        addFeedForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const url = document.getElementById('feed-url').value;
            const name = document.getElementById('feed-name').value;
            const feedType = document.getElementById('feed-type').value;
            const scraperModule = document.getElementById('scraper-module')?.value || '';
            const categoryId = parseInt(document.getElementById('feed-category')?.value) || 0;
            const interval = parseInt(document.getElementById('feed-interval').value) || 60;

            try {
                const feed = await api('POST', '/api/feeds', {
                    url,
                    name: name || url,
                    feedType,
                    scraperModule,
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
