// Alerts pages — list and detail views.

import { api } from './api.js';
import { showToast } from './toast.js';
import { openModal, closeModal } from './modal.js';

// ---------------------------------------------------------------------------
// Alerts list page (/alerts)
// ---------------------------------------------------------------------------

/**
 * Initialise the alerts list page. Call from DOMContentLoaded.
 * Detected by the presence of a `.alerts-view` element.
 */
export function initAlertsPage() {
    const view = document.querySelector('.alerts-view');
    if (!view) return;

    // "Create Alert" button opens the modal form.
    const createBtn = view.querySelector('[data-action="create-alert"]');
    if (createBtn) {
        createBtn.addEventListener('click', () => openCreateAlertModal());
    }

    // Delegated click handlers inside the alerts view.
    view.addEventListener('click', (e) => {
        const dismissAll = e.target.closest('[data-action="dismiss-all-alert"]');
        if (dismissAll) {
            const alertId = dismissAll.dataset.alertId;
            if (alertId) dismissAllMatches(alertId);
            return;
        }

        const dismissOne = e.target.closest('[data-action="dismiss-article-alert"]');
        if (dismissOne) {
            const articleAlertId = dismissOne.dataset.articleAlertId;
            if (articleAlertId) dismissArticleAlert(articleAlertId);
            return;
        }

        const closeBtn = e.target.closest('[data-action="close-create-alert-modal"]');
        if (closeBtn) {
            closeCreateAlertModal();
            return;
        }
    });

    // Form submission (delegated).
    view.addEventListener('submit', (e) => {
        if (e.target.id === 'create-alert-form') {
            e.preventDefault();
            submitCreateAlert();
        }
    });
}

// -- Create alert modal -----------------------------------------------------

export function createAlertModal() {
    let modal = document.getElementById('create-alert-modal');
    if (modal) return modal;

    modal = document.createElement('div');
    modal.id = 'create-alert-modal';
    modal.className = 'modal';
    modal.style.display = 'none';
    modal.innerHTML = `
        <div class="modal-backdrop" data-action="close-create-alert-modal"></div>
        <div class="modal-content">
            <div class="modal-header">
                <h3>Create Alert</h3>
                <button data-action="close-create-alert-modal" class="btn-icon" aria-label="Close">
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                        <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                    </svg>
                </button>
            </div>
            <form id="create-alert-form">
                <div class="form-group">
                    <label for="alert-name">Name</label>
                    <input type="text" id="alert-name" required>
                </div>
                <div class="form-group">
                    <label for="alert-pattern">Pattern</label>
                    <input type="text" id="alert-pattern" required>
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="alert-is-regex"> Regular expression
                    </label>
                </div>
                <div class="form-group">
                    <label for="alert-match-field">Match field</label>
                    <select id="alert-match-field">
                        <option value="title">Title</option>
                        <option value="content">Content</option>
                        <option value="title_and_content">Title &amp; Content</option>
                    </select>
                </div>
                <div class="modal-actions">
                    <button type="button" data-action="close-create-alert-modal" class="btn btn-secondary">Cancel</button>
                    <button type="submit" class="btn btn-primary">Create</button>
                </div>
            </form>
        </div>
    `;

    // Append to the .alerts-view so delegated listeners work.
    const view = document.querySelector('.alerts-view');
    if (view) {
        view.appendChild(modal);
    } else {
        document.body.appendChild(modal);
    }
    return modal;
}

export function openCreateAlertModal() {
    const modal = createAlertModal();
    modal.style.display = 'flex';
    openModal(modal, closeCreateAlertModal);
}

export function closeCreateAlertModal() {
    const modal = document.getElementById('create-alert-modal');
    if (modal) modal.style.display = 'none';
    closeModal();
}

export async function submitCreateAlert() {
    const name = document.getElementById('alert-name').value.trim();
    const pattern = document.getElementById('alert-pattern').value.trim();
    const isRegex = document.getElementById('alert-is-regex').checked;
    const matchField = document.getElementById('alert-match-field').value;

    if (!name || !pattern) return;

    try {
        await api('POST', '/api/alerts', {
            name,
            pattern,
            is_regex: isRegex,
            match_field: matchField,
        });
        closeCreateAlertModal();
        location.reload();
    } catch (e) {
        console.error('Failed to create alert:', e);
        showToast('Failed to create alert');
    }
}

// -- Dismiss helpers --------------------------------------------------------

export async function dismissAllMatches(alertId) {
    try {
        await api('POST', `/api/alerts/${alertId}/dismiss`);
        // Remove the alert group section from the DOM.
        const section = document.querySelector(`.alert-group[data-alert-id="${alertId}"]`);
        if (section) section.remove();
        updateAlertsBadge();
    } catch (e) {
        console.error('Failed to dismiss alert matches:', e);
        showToast('Failed to dismiss matches');
    }
}

export async function dismissArticleAlert(articleAlertId) {
    try {
        await api('POST', `/api/article-alerts/${articleAlertId}/dismiss`);
        // Remove the individual match element from the DOM.
        const el = document.querySelector(`.article-alert-item[data-article-alert-id="${articleAlertId}"]`);
        if (el) {
            const group = el.closest('.alert-group');
            el.remove();
            // If group is now empty, remove it too.
            if (group && group.querySelectorAll('.article-alert-item').length === 0) {
                group.remove();
            }
        }
        updateAlertsBadge();
    } catch (e) {
        console.error('Failed to dismiss article alert:', e);
        showToast('Failed to dismiss alert');
    }
}

/** Recount visible article-alert items and update the nav badge. */
export function updateAlertsBadge() {
    const remaining = document.querySelectorAll('.article-alert-item').length;
    const badge = document.querySelector('[data-count="alerts"]');
    if (badge) {
        badge.textContent = remaining > 0 ? String(remaining) : '';
    }
}

// ---------------------------------------------------------------------------
// Alert detail page (/alerts/{id})
// ---------------------------------------------------------------------------

/**
 * Initialise the alert detail page. Call from DOMContentLoaded.
 * Detected by the presence of a `.alert-detail-view` element.
 */
export function initAlertDetailPage() {
    const view = document.querySelector('.alert-detail-view');
    if (!view) return;

    const alertId = view.dataset.alertId;
    if (!alertId) return;

    // Edit form submission.
    const editForm = view.querySelector('#edit-alert-form');
    if (editForm) {
        editForm.addEventListener('submit', (e) => {
            e.preventDefault();
            saveAlert(alertId);
        });
    }

    // Delegated clicks inside detail view.
    view.addEventListener('click', (e) => {
        const deleteBtn = e.target.closest('[data-action="delete-alert"]');
        if (deleteBtn) {
            deleteAlert(alertId);
            return;
        }

        const dismissBtn = e.target.closest('[data-action="dismiss-article-alert"]');
        if (dismissBtn) {
            const articleAlertId = dismissBtn.dataset.articleAlertId;
            if (articleAlertId) dismissArticleAlertDetail(articleAlertId);
            return;
        }

        const undismissBtn = e.target.closest('[data-action="undismiss-article-alert"]');
        if (undismissBtn) {
            const articleAlertId = undismissBtn.dataset.articleAlertId;
            if (articleAlertId) undismissArticleAlertDetail(articleAlertId);
            return;
        }
    });
}

export async function saveAlert(alertId) {
    const name = document.getElementById('edit-alert-name').value.trim();
    const pattern = document.getElementById('edit-alert-pattern').value.trim();
    const isRegex = document.getElementById('edit-alert-is-regex').checked;
    const matchField = document.getElementById('edit-alert-match-field').value;

    if (!name || !pattern) return;

    try {
        await api('PUT', `/api/alerts/${alertId}`, {
            name,
            pattern,
            is_regex: isRegex,
            match_field: matchField,
        });
        showToast('Alert updated', 'success');
    } catch (e) {
        console.error('Failed to update alert:', e);
        showToast('Failed to update alert');
    }
}

export async function deleteAlert(alertId) {
    if (!confirm('Delete this alert? This cannot be undone.')) return;
    try {
        await api('DELETE', `/api/alerts/${alertId}`);
        window.location.href = '/alerts';
    } catch (e) {
        console.error('Failed to delete alert:', e);
        showToast('Failed to delete alert');
    }
}

export async function dismissArticleAlertDetail(articleAlertId) {
    try {
        await api('POST', `/api/article-alerts/${articleAlertId}/dismiss`);
        toggleDismissState(articleAlertId, true);
    } catch (e) {
        console.error('Failed to dismiss article alert:', e);
        showToast('Failed to dismiss alert');
    }
}

export async function undismissArticleAlertDetail(articleAlertId) {
    try {
        await api('POST', `/api/article-alerts/${articleAlertId}/undismiss`);
        toggleDismissState(articleAlertId, false);
    } catch (e) {
        console.error('Failed to undismiss article alert:', e);
        showToast('Failed to undismiss alert');
    }
}

/**
 * Toggle the visual dismissed/undismissed state of an article-alert row
 * on the detail page.
 */
export function toggleDismissState(articleAlertId, dismissed) {
    const item = document.querySelector(`.article-alert-item[data-article-alert-id="${articleAlertId}"]`);
    if (!item) return;
    item.classList.toggle('dismissed', dismissed);

    const btn = item.querySelector('[data-action="dismiss-article-alert"], [data-action="undismiss-article-alert"]');
    if (btn) {
        btn.dataset.action = dismissed ? 'undismiss-article-alert' : 'dismiss-article-alert';
        btn.textContent = dismissed ? 'Undismiss' : 'Dismiss';
    }
}
