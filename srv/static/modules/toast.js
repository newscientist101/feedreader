// Toast notification system for user-facing feedback.
// Replaces alert() and silent console.error() with non-blocking toasts.

import { escapeHtml } from './utils.js';

let container = null;

function getContainer() {
    if (container && document.body.contains(container)) return container;
    container = document.createElement('div');
    container.className = 'toast-container';
    container.setAttribute('aria-live', 'polite');
    document.body.appendChild(container);
    return container;
}

const errorIcon = '<svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>';
const successIcon = '<svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg>';
const infoIcon = '<svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>';

/**
 * Show a toast notification.
 * @param {string} message - Text to display
 * @param {'error'|'success'|'info'} [type='error'] - Toast type
 * @param {number} [duration=4000] - Auto-dismiss ms (0 = manual only)
 */
export function showToast(message, type = 'error', duration = 4000) {
    const c = getContainer();
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.setAttribute('role', 'status');

    const icon = type === 'error' ? errorIcon : type === 'success' ? successIcon : infoIcon;

    toast.innerHTML = `
        <span class="toast-icon">${icon}</span>
        <span class="toast-message">${escapeHtml(message)}</span>
        <button class="toast-close" aria-label="Dismiss">&times;</button>
    `;

    toast.querySelector('.toast-close').addEventListener('click', () => dismiss(toast));

    c.appendChild(toast);
    // Trigger reflow then animate in
    void toast.offsetHeight;
    toast.classList.add('toast-visible');

    if (duration > 0) {
        setTimeout(() => dismiss(toast), duration);
    }

    return toast;
}

function dismiss(toast) {
    if (!toast.parentNode) return;
    toast.classList.remove('toast-visible');
    toast.addEventListener('transitionend', () => toast.remove(), { once: true });
    // Fallback if transitionend doesn't fire
    setTimeout(() => toast.remove(), 300);
}



