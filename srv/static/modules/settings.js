// User settings: read/write settings object, apply UI preferences.

import { showToast } from './toast.js';

// Read a setting value from the server-injected settings object.
export function getSetting(key, defaultValue) {
    const val = (window.__settings || {})[key];
    return val !== undefined ? val : (defaultValue || '');
}

// Save a setting both locally and to the server.
export function saveSetting(key, value) {
    if (!window.__settings) window.__settings = {};
    window.__settings[key] = value;
    fetch('/api/settings', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest',
        },
        body: JSON.stringify({ [key]: value }),
    }).catch(e => { console.error('Failed to save setting:', e); showToast('Failed to save setting'); });
}

// Hide/show read article cards based on setting value.
export function applyHideReadArticles(value) {
    document.querySelectorAll('.article-card.read').forEach(card => {
        card.style.display = value === 'hide' ? 'none' : '';
    });
}

// Hide/show empty feeds in sidebar based on setting value.
export function applyHideEmptyFeeds(value) {
    document.querySelectorAll('.feed-item').forEach(item => {
        // Don't hide feeds that have never been fetched
        if (item.dataset.neverFetched === 'true') return;
        const badge = item.querySelector('.badge');
        const count = badge ? parseInt(badge.textContent || '0', 10) : 0;
        if (!count) {
            item.style.display = value === 'hide' ? 'none' : '';
        } else {
            item.style.display = '';
        }
    });
}
