// Feed error banner: create/update/remove the error banner shown when a feed has fetch errors.
// Extracted as a shared leaf module to break the counts ↔ feeds circular dependency.

import { escapeHtml } from './utils.js';

// Create or update the feed error banner
export function showFeedErrorBanner(feedId, errorMessage) {
    let banner = document.querySelector('.feed-error-banner');
    if (!banner) {
        banner = document.createElement('div');
        banner.className = 'feed-error-banner';
        const view = document.querySelector('.articles-view');
        if (view) view.insertBefore(banner, view.firstChild);
    }
    banner.innerHTML = `
        <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
        </svg>
        <span>Last fetch failed: ${escapeHtml(errorMessage)}</span>
        <button class="btn btn-small" data-action="refresh-feed" data-feed-id="${feedId}">Retry</button>
    `;
}

// Remove the feed error banner if present
export function removeFeedErrorBanner() {
    const banner = document.querySelector('.feed-error-banner');
    if (banner) banner.remove();
}
