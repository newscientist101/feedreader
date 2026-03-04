// OPML import/export functions.

import { showToast } from './toast.js';

/**
 * Export feeds as OPML by redirecting to the export endpoint.
 */
export function exportOPML() {
    window.location.href = '/api/opml/export';
}

/**
 * Import feeds from an OPML file.
 * @param {HTMLInputElement} input - The file input element.
 */
export async function importOPML(input) {
    const file = input.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('file', file);

    try {
        const res = await fetch('/api/opml/import', {
            method: 'POST',
            headers: { 'X-Requested-With': 'XMLHttpRequest' },
            body: formData
        });
        const result = await res.json();
        if (!res.ok) {
            throw new Error(result.error || 'Import failed');
        }
        showToast(`Imported ${result.imported} feeds (${result.skipped} skipped)`, 'success');
        location.reload();
    } catch (e) {
        showToast('Failed to import OPML: ' + e.message);
    }

    // Clear the input
    input.value = '';
}

let _opmlListenerAC = null;

/**
 * Initialize delegated listeners for OPML actions on the feeds page.
 */
export function initOpmlListeners() {
    if (_opmlListenerAC) _opmlListenerAC.abort();
    _opmlListenerAC = new AbortController();
    const signal = _opmlListenerAC.signal;

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="export-opml"]');
        if (btn) exportOPML();
    }, { signal });

    document.addEventListener('change', (e) => {
        const input = e.target.closest('[data-action="import-opml"]');
        if (input) importOPML(input);
    }, { signal });
}
