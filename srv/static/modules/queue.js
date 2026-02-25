// Queue page — "next" button logic.

import { api } from './api.js';
import { updateQueueCacheIfStandalone } from './offline.js';

/**
 * Initialise the queue page. Call from DOMContentLoaded.
 * Reads the server-rendered article-ID list from the embedded JSON array
 * and wires up the "Next" button.
 */
export function initQueuePage() {
    const dataEl = document.getElementById('queue-data');
    if (!dataEl) return;                // not on queue page

    let ids;
    try {
        ids = JSON.parse(dataEl.textContent);
    } catch {
        return;
    }
    if (!Array.isArray(ids) || ids.length === 0) return;

    const btn = document.querySelector('.queue-next-btn');
    if (btn) {
        btn.addEventListener('click', () => queueNext(ids));
    }
}

/**
 * Remove the current (first) article from the queue and reload.
 * @param {number[]} queueArticleIds  ordered list of article IDs
 */
export async function queueNext(queueArticleIds) {
    if (!queueArticleIds || queueArticleIds.length === 0) return;
    const currentId = queueArticleIds[0];
    await api('DELETE', `/api/articles/${currentId}/queue`);
    updateQueueCacheIfStandalone();
    window.location.href = '/queue';
}
