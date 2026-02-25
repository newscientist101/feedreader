// Read button: update the read/unread toggle button inside an article card.
// Extracted as a shared leaf module to break the article-actions ↔ articles circular dependency.

import { SVG_MARK_READ, SVG_MARK_UNREAD } from './icons.js';

export function updateReadButton(card, isRead) {
    if (!card) return;
    const btn = card.querySelector('.btn-read-toggle');
    if (!btn) return;
    btn.dataset.isRead = isRead ? '1' : '0';
    if (isRead) {
        btn.setAttribute('title', 'Mark unread');
        btn.innerHTML = SVG_MARK_UNREAD;
    } else {
        btn.setAttribute('title', 'Mark read');
        btn.innerHTML = SVG_MARK_READ;
    }
}
