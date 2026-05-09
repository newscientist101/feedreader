// Usenet newsgroup reader UI — credential management (settings page) and
// newsgroup subscription management (feeds page).

import { api } from './api.js';
import { showToast } from './toast.js';

// --- Module state (declared at top per STYLE.md) ---

let _settingsAC = null;
let _feedsAC = null;

// --- Settings page: Usenet credentials ---

/**
 * Load credential status from the API and populate the settings-page section.
 * No-op if the usenet section is absent from the DOM.
 */
export async function initUsenetCredentialsSection() {
    const section = document.getElementById('usenet-section');
    if (!section) return;

    const statusEl = document.getElementById('usenet-credentials-status');
    const formEl = document.getElementById('usenet-credentials-form');

    try {
        const data = await api('GET', '/api/usenet/credentials');
        _renderCredentialStatus(data, statusEl, formEl);
    } catch (err) {
        if (statusEl) {
            statusEl.innerHTML = `<span class="form-error">Failed to load credential status: ${err.message}</span>`;
        }
    }
}

/**
 * Render the credential status block on the settings page.
 *
 * @param {object} data  - API response: { enabled, configured, username, key_version }
 * @param {Element} statusEl - element to show status summary
 * @param {Element} formEl - element containing the save/delete form
 */
function _renderCredentialStatus(data, statusEl, formEl) {
    if (!statusEl || !formEl) return;

    if (data.configured) {
        statusEl.innerHTML =
            `<span class="status-ok">Configured</span> — ` +
            `username: <strong>${_escHtml(data.username)}</strong>, ` +
            `key version: <code>${_escHtml(data.key_version)}</code>`;
    } else {
        statusEl.innerHTML = '<span class="status-warn">Not configured</span>';
    }

    formEl.style.display = '';

    // Show / hide the delete button depending on configured state.
    const deleteBtn = document.getElementById('usenet-delete-btn');
    if (deleteBtn) {
        deleteBtn.style.display = data.configured ? '' : 'none';
    }

    // Pre-fill username when already configured.
    const usernameInput = document.getElementById('usenet-username');
    if (usernameInput && data.configured) {
        usernameInput.value = data.username;
    }
}

/**
 * Save Usenet credentials from the settings form.
 */
async function _saveUsenetCredentials(form) {
    const username = form.querySelector('#usenet-username')?.value.trim() ?? '';
    const password = form.querySelector('#usenet-password')?.value ?? '';
    const statusEl = document.getElementById('usenet-cred-status');

    if (!username || !password) {
        if (statusEl) statusEl.textContent = 'Username and password are required.';
        return;
    }

    if (statusEl) statusEl.textContent = 'Saving…';

    try {
        const data = await api('PUT', '/api/usenet/credentials', { username, password });
        const statusDiv = document.getElementById('usenet-credentials-status');
        const formDiv = document.getElementById('usenet-credentials-form');
        _renderCredentialStatus(data, statusDiv, formDiv);
        // Clear the password field after a successful save.
        const pwEl = form.querySelector('#usenet-password');
        if (pwEl) pwEl.value = '';
        if (statusEl) statusEl.textContent = '';
        showToast('Usenet credentials saved', 'success');
    } catch (err) {
        if (statusEl) statusEl.textContent = 'Error: ' + err.message;
    }
}

/**
 * Delete Usenet credentials and update the settings section.
 */
async function _deleteUsenetCredentials() {
    if (!confirm('Remove your stored Usenet credentials?')) return;

    const statusEl = document.getElementById('usenet-cred-status');
    if (statusEl) statusEl.textContent = 'Removing…';

    try {
        await api('DELETE', '/api/usenet/credentials');
        const statusDiv = document.getElementById('usenet-credentials-status');
        const formDiv = document.getElementById('usenet-credentials-form');
        _renderCredentialStatus({ configured: false, username: '', key_version: '' }, statusDiv, formDiv);
        const usernameInput = document.getElementById('usenet-username');
        if (usernameInput) usernameInput.value = '';
        if (statusEl) statusEl.textContent = '';
        showToast('Usenet credentials removed');
    } catch (err) {
        if (statusEl) statusEl.textContent = 'Error: ' + err.message;
    }
}

/**
 * Initialize delegated listeners for the settings-page Usenet section.
 * No-op when not on the settings page.
 */
export function initUsenetSettingsListeners() {
    const section = document.getElementById('usenet-section');
    if (!section) return;

    if (_settingsAC) _settingsAC.abort();
    _settingsAC = new AbortController();
    const signal = _settingsAC.signal;

    document.addEventListener('submit', (e) => {
        const form = e.target.closest('#usenet-cred-form');
        if (!form) return;
        e.preventDefault();
        _saveUsenetCredentials(form);
    }, { signal });

    document.addEventListener('click', (e) => {
        if (e.target.closest('[data-action="usenet-delete-credentials"]')) {
            _deleteUsenetCredentials();
        }
    }, { signal });
}

// --- Feeds page: Usenet newsgroups ---

/**
 * Load credential status and newsgroup list, then render the feeds-page
 * Usenet section. No-op when not on the feeds page.
 */
export async function initUsenetGroupsSection() {
    const section = document.getElementById('usenet-groups-section');
    if (!section) return;

    try {
        const creds = await api('GET', '/api/usenet/credentials');
        if (!creds.configured) {
            _showUsenetNoCredentials();
            return;
        }
        _showUsenetGroupsContent();
        await _loadUsenetGroups();
    } catch (err) {
        const noCredsEl = document.getElementById('usenet-no-credentials');
        if (noCredsEl) {
            noCredsEl.style.display = '';
            noCredsEl.innerHTML =
                `<p class="empty-state">Failed to load Usenet status: ${_escHtml(err.message)}</p>`;
        }
    }
}

/** Show the missing-credentials prompt on the feeds page. */
function _showUsenetNoCredentials() {
    const noCredsEl = document.getElementById('usenet-no-credentials');
    const contentEl = document.getElementById('usenet-groups-content');
    if (noCredsEl) noCredsEl.style.display = '';
    if (contentEl) contentEl.style.display = 'none';
}

/** Show the add-group form and list on the feeds page. */
function _showUsenetGroupsContent() {
    const noCredsEl = document.getElementById('usenet-no-credentials');
    const contentEl = document.getElementById('usenet-groups-content');
    if (noCredsEl) noCredsEl.style.display = 'none';
    if (contentEl) contentEl.style.display = '';
}

/**
 * Fetch the subscribed newsgroups list and render it.
 */
async function _loadUsenetGroups() {
    const listEl = document.getElementById('usenet-groups-list');
    if (!listEl) return;

    listEl.innerHTML = '<span class="spinner" aria-label="Loading newsgroups"></span>';

    try {
        const groups = await api('GET', '/api/usenet/groups');
        _renderUsenetGroups(groups, listEl);
    } catch (err) {
        listEl.innerHTML = `<p class="form-error">Failed to load newsgroups: ${_escHtml(err.message)}</p>`;
    }
}

/**
 * Render the list of subscribed newsgroups.
 *
 * @param {Array} groups - Array of { feed_id, name, group_name, provider, high_water_article_number }
 * @param {Element} listEl - container element
 */
function _renderUsenetGroups(groups, listEl) {
    if (!groups || groups.length === 0) {
        listEl.innerHTML = '<p class="empty-state">No newsgroups subscribed yet. Enter a group name above to subscribe.</p>';
        return;
    }

    const rows = groups.map(g => `
        <div class="usenet-group-row" data-feed-id="${g.feed_id}">
            <span class="usenet-group-name">${_escHtml(g.group_name)}</span>
            <span class="usenet-group-hwm" title="Highest article number fetched so far">
                ${g.high_water_article_number > 0 ? `#${g.high_water_article_number}` : 'Not fetched yet'}
            </span>
            <button class="btn-icon danger" data-action="usenet-remove-group"
                    data-feed-id="${g.feed_id}" data-group-name="${_escHtml(g.group_name)}"
                    title="Unsubscribe from ${_escHtml(g.group_name)}" aria-label="Remove">
                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                    <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
                </svg>
            </button>
        </div>
    `).join('');

    listEl.innerHTML = rows;
}

/**
 * Handle adding a newsgroup from the feeds-page form.
 */
async function _addUsenetGroup(form) {
    const groupNameEl = form.querySelector('#usenet-group-name');
    const categoryEl = form.querySelector('#usenet-group-category');
    const statusEl = document.getElementById('usenet-add-status');

    const groupName = groupNameEl?.value.trim() ?? '';
    if (!groupName) {
        if (statusEl) statusEl.textContent = 'Newsgroup name is required.';
        return;
    }

    const categoryID = parseInt(categoryEl?.value ?? '0', 10) || 0;

    if (statusEl) statusEl.textContent = 'Subscribing…';

    try {
        await api('POST', '/api/usenet/groups', {
            group_name: groupName,
            category_id: categoryID,
        });
        if (groupNameEl) groupNameEl.value = '';
        if (statusEl) statusEl.textContent = '';
        showToast(`Subscribed to ${groupName}`, 'success');
        await _loadUsenetGroups();
    } catch (err) {
        if (statusEl) statusEl.textContent = 'Error: ' + err.message;
    }
}

/**
 * Handle removing a newsgroup via the remove button.
 */
async function _removeUsenetGroup(feedID, groupName) {
    if (!confirm(`Unsubscribe from ${groupName}?`)) return;

    try {
        await api('DELETE', `/api/usenet/groups/${feedID}`);
        showToast(`Unsubscribed from ${groupName}`);
        await _loadUsenetGroups();
    } catch (err) {
        showToast('Failed to remove newsgroup: ' + err.message);
    }
}

/**
 * Initialize delegated listeners for the feeds-page Usenet section.
 * No-op when not on the feeds page.
 */
export function initUsenetFeedsListeners() {
    const section = document.getElementById('usenet-groups-section');
    if (!section) return;

    if (_feedsAC) _feedsAC.abort();
    _feedsAC = new AbortController();
    const signal = _feedsAC.signal;

    document.addEventListener('submit', (e) => {
        const form = e.target.closest('#usenet-add-group-form');
        if (!form) return;
        e.preventDefault();
        _addUsenetGroup(form);
    }, { signal });

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="usenet-remove-group"]');
        if (!btn) return;
        const feedID = btn.dataset.feedId;
        const groupName = btn.dataset.groupName;
        if (feedID && groupName) {
            _removeUsenetGroup(feedID, groupName);
        }
    }, { signal });
}

// --- Shared helpers ---

/**
 * Escape a string for safe insertion into HTML.
 *
 * @param {string} s
 * @returns {string}
 */
function _escHtml(s) {
    return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}
