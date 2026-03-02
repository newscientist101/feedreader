// Settings page initialization and controls.

import { api } from './api.js';
import { showToast } from './toast.js';
import { getSetting, saveSetting, applyHideReadArticles, applyHideEmptyFeeds } from './settings.js';

/**
 * Initialize settings page controls from server settings.
 * No-op if not on the settings page.
 */
export function initSettingsPage() {
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

/**
 * Run retention cleanup from the settings page.
 */
export async function runCleanup() {
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

/**
 * Load existing newsletter address from the API.
 */
export async function loadNewsletterAddress() {
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

/**
 * Generate a new newsletter email address.
 */
export async function generateNewsletterAddress() {
    try {
        const data = await api('POST', '/api/newsletter/generate-address');
        if (data.address) {
            showNewsletterAddress(data.address);
        }
    } catch (e) {
        showToast('Failed to generate address: ' + e.message);
    }
}

/**
 * Show the newsletter address in the UI.
 */
export function showNewsletterAddress(address) {
    const noAddr = document.getElementById('newsletter-no-address');
    const hasAddr = document.getElementById('newsletter-has-address');
    const addrEl = document.getElementById('newsletter-address');
    if (noAddr) noAddr.style.display = 'none';
    if (hasAddr) hasAddr.style.display = '';
    if (addrEl) addrEl.textContent = address;
}

/**
 * Copy the newsletter address to clipboard.
 */
export async function copyNewsletterAddress() {
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

/**
 * Initialize delegated listeners for settings page controls.
 * Handles: data-setting radios/checkboxes, run-cleanup, generate-newsletter,
 * copy-newsletter.
 */
export function initSettingsPageListeners() {
    // Settings inputs: radios and checkboxes with data-setting attribute
    document.addEventListener('change', (e) => {
        const input = e.target.closest('[data-setting]');
        if (!input) return;
        const key = input.dataset.setting;
        const value = input.dataset.settingType === 'checkbox' ? String(input.checked) : input.value;
        saveSetting(key, value);
        // Apply immediate UI effects if configured
        const applyKey = input.dataset.apply;
        if (applyKey === 'hideReadArticles') applyHideReadArticles(value);
        if (applyKey === 'hideEmptyFeeds') applyHideEmptyFeeds(value);
    });

    document.addEventListener('click', (e) => {
        if (e.target.closest('[data-action="run-cleanup"]')) runCleanup();
    });

    document.addEventListener('click', (e) => {
        if (e.target.closest('[data-action="generate-newsletter"]')) generateNewsletterAddress();
    });

    document.addEventListener('click', (e) => {
        if (e.target.closest('[data-action="copy-newsletter"]')) copyNewsletterAddress();
    });
}
