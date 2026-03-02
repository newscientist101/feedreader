import { api } from './api.js';
import { showToast } from './toast.js';

export function openCreateFolderModal() {
    const modal = document.getElementById('create-folder-modal');
    if (!modal) return;
    document.getElementById('new-folder-name').value = '';
    document.getElementById('new-folder-parent').value = '0';
    modal.style.display = 'flex';
    document.getElementById('new-folder-name').focus();
}

export function closeCreateFolderModal() {
    const modal = document.getElementById('create-folder-modal');
    if (modal) modal.style.display = 'none';
}

export async function submitCreateFolder(e) {
    e.preventDefault();
    const name = document.getElementById('new-folder-name').value.trim();
    if (!name) return;
    const parentId = parseInt(document.getElementById('new-folder-parent').value) || 0;

    try {
        const cat = await api('POST', '/api/categories', { name });
        if (parentId > 0 && cat.id) {
            await api('POST', `/api/categories/${cat.id}/parent`, {
                parent_id: parentId,
                sort_order: 0
            });
        }
        location.reload();
    } catch (e) {
        showToast('Failed to create folder: ' + e.message);
    }
}

export async function renameCategory(id, currentName) {
    const name = prompt('Enter new name:', currentName);
    if (!name || name === currentName) return;

    try {
        await api('PUT', `/api/categories/${id}`, { name });
        location.reload();
    } catch (e) {
        showToast('Failed to rename folder: ' + e.message);
    }
}

export async function unparentCategory(id) {
    try {
        await api('POST', `/api/categories/${id}/parent`, {
            parent_id: null,
            sort_order: 999 // Put at end
        });
        location.reload();
    } catch (e) {
        console.error('Failed to unparent category:', e);
        showToast('Failed to move folder to top level');
    }
}

export async function deleteCategory(id, name) {
    if (!confirm(`Delete folder "${name}"? Feeds will be moved to uncategorized.`)) {
        return;
    }
    try {
        await api('DELETE', `/api/categories/${id}`);
        location.reload();
    } catch (e) {
        showToast('Failed to delete folder: ' + e.message);
    }
}

/**
 * Initialize delegated listeners for folder actions on the feeds page.
 * Handles: rename-category, delete-category, open/close/submit create-folder.
 */
export function initFoldersPageListeners() {
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="rename-category"]');
        if (btn) {
            const id = Number(btn.dataset.categoryId);
            const name = btn.dataset.categoryName || '';
            if (id) renameCategory(id, name);
        }
    });

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="delete-category"]');
        if (btn) {
            const id = Number(btn.dataset.categoryId);
            const name = btn.dataset.categoryName || '';
            if (id) deleteCategory(id, name);
        }
    });

    document.addEventListener('click', (e) => {
        const el = e.target.closest('[data-action="open-create-folder"]');
        if (el) openCreateFolderModal();
    });

    document.addEventListener('click', (e) => {
        const el = e.target.closest('[data-action="close-create-folder"]');
        if (el) closeCreateFolderModal();
    });

    // Handle create folder form submission
    document.addEventListener('submit', (e) => {
        const form = e.target.closest('#create-folder-form');
        if (form) submitCreateFolder(e);
    });

    // Unparent category button (category_settings.html)
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="unparent-category"]');
        if (btn) {
            const id = Number(btn.dataset.categoryId);
            if (id) unparentCategory(id);
        }
    });

    // Delete exclusion rule button (category_settings.html)
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="delete-exclusion"]');
        if (btn) {
            const id = Number(btn.dataset.exclusionId);
            if (!id) return;
            if (!confirm('Delete this exclusion rule?')) return;
            api('DELETE', `/api/exclusions/${id}`)
                .then(() => location.reload())
                .catch(err => showToast('Failed to delete rule: ' + err.message));
        }
    });
}

/**
 * Initialize category settings page — wires up the exclusion form.
 * No-op if not on the category settings page.
 */
export function initCategorySettingsPage() {
    const view = document.querySelector('.settings-view[data-category-id]');
    if (!view) return;
    const categoryId = Number(view.dataset.categoryId);

    const form = document.getElementById('add-exclusion-form');
    if (form) {
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            const type = document.getElementById('exclusion-type').value;
            const pattern = document.getElementById('exclusion-pattern').value;
            const isRegex = document.getElementById('exclusion-regex').checked;
            try {
                await api('POST', `/api/categories/${categoryId}/exclusions`, { type, pattern, isRegex });
                location.reload();
            } catch (err) {
                showToast('Failed to add exclusion: ' + err.message);
            }
        });
    }
}
