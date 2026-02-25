// Sidebar: mobile toggle, active highlighting, folder expand/collapse.

// Late-bound reference to loadCategoryArticles (set by app.js during init).
// This avoids a circular dependency with the feeds module.
let _loadCategoryArticles = null;

export function setSidebarLoadCategory(fn) {
    _loadCategoryArticles = fn;
}

// Sidebar highlighting helper — clears all active states in the sidebar,
// then marks the given element (nav-item, feed-item, or folder-item) active.
export function setSidebarActive(el) {
    document.querySelectorAll('.sidebar .active').forEach(a => a.classList.remove('active'));
    if (el) el.classList.add('active');
}

// Mobile sidebar toggle
export function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const overlay = document.querySelector('.sidebar-overlay');
    sidebar.classList.toggle('open');
    overlay.classList.toggle('active');
    document.body.style.overflow = sidebar.classList.contains('open') ? 'hidden' : '';
}

// Toggle folder expand/collapse in sidebar
export function navigateFolder(event, categoryId) {
    // If not on the main articles page, use regular navigation
    if (!document.getElementById('articles-list')) {
        return true;
    }
    event.preventDefault();
    const folderItem = document.querySelector(`.folder-item[data-category-id="${categoryId}"]`);
    if (!folderItem) return false;

    // Load category articles via AJAX (also updates active state)
    if (_loadCategoryArticles) {
        _loadCategoryArticles(categoryId, folderItem.querySelector('.folder-name')?.textContent || 'Category');
    }

    return false;
}

export function toggleFolderCollapse(categoryId) {
    const folderItem = document.querySelector(`.folder-item[data-category-id="${categoryId}"]`);
    if (!folderItem) return;
    if (folderItem.classList.contains('expanded')) {
        collapseFolder(folderItem);
    } else {
        folderItem.classList.add('expanded');
    }
}

export function collapseFolder(folderItem) {
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

// Attach delegated event listeners for sidebar actions (replaces inline onclick handlers).
export function initSidebarListeners() {
    // data-action="toggle-sidebar" — menu toggle button and overlay
    document.addEventListener('click', (e) => {
        if (e.target.closest('[data-action="toggle-sidebar"]')) {
            toggleSidebar();
        }
    });

    // data-action="toggle-folder" — folder chevron expand/collapse
    // Uses capture phase so stopPropagation prevents the click from reaching
    // parent elements (e.g. folder-row click handlers).
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="toggle-folder"]');
        if (btn) {
            e.stopPropagation();
            const catId = Number(btn.dataset.categoryId);
            if (catId) toggleFolderCollapse(catId);
        }
    }, true);

    // data-action="navigate-folder" — folder link SPA navigation
    document.addEventListener('click', (e) => {
        const link = e.target.closest('[data-action="navigate-folder"]');
        if (link) {
            const catId = Number(link.dataset.categoryId);
            // navigateFolder returns true for full-page nav, false/undefined for SPA
            const result = navigateFolder(e, catId);
            if (!result) {
                e.preventDefault();
            }
        }
    });
}
