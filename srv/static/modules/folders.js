import { api } from './api.js';

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
        alert('Failed to create folder: ' + e.message);
    }
}

export async function renameCategory(id, currentName) {
    const name = prompt('Enter new name:', currentName);
    if (!name || name === currentName) return;

    try {
        await api('PUT', `/api/categories/${id}`, { name });
        location.reload();
    } catch (e) {
        alert('Failed to rename folder: ' + e.message);
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
        alert('Failed to move folder to top level');
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
        alert('Failed to delete folder: ' + e.message);
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
}
