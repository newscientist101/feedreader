// OPML import/export functions.

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
            body: formData
        });
        const result = await res.json();
        if (!res.ok) {
            throw new Error(result.error || 'Import failed');
        }
        alert(`Imported ${result.imported} feeds (${result.skipped} skipped, already exist)`);
        location.reload();
    } catch (e) {
        alert('Failed to import OPML: ' + e.message);
    }

    // Clear the input
    input.value = '';
}

/**
 * Initialize delegated listeners for OPML actions on the feeds page.
 */
export function initOpmlListeners() {
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action="export-opml"]');
        if (btn) exportOPML();
    });

    document.addEventListener('change', (e) => {
        const input = e.target.closest('[data-action="import-opml"]');
        if (input) importOPML(input);
    });
}
