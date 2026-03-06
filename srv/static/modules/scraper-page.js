import { api } from './api.js';
import { showToast } from './toast.js';
import { openModal, closeModal } from './modal.js';

let _scraperSubmitting = false;

/**
 * Initialize the scraper page — wires up all event listeners.
 * No-op if not on the scrapers page.
 */
export function initScraperPage() {
    if (!document.querySelector('.scrapers-view')) return;

    initManualForm();
}

let _scraperPageListenerAC = null;

/**
 * Set up delegated click/change listeners for the scrapers page.
 */
export function initScraperPageListeners() {
    if (_scraperPageListenerAC) _scraperPageListenerAC.abort();
    _scraperPageListenerAC = new AbortController();
    const signal = _scraperPageListenerAC.signal;

    document.addEventListener('click', (e) => {
        const action = e.target.closest('[data-action]');
        if (!action) return;

        switch (action.dataset.action) {
            case 'toggle-schema-panel':
                toggleSchemaPanel();
                break;
            case 'insert-field': {
                const key = action.dataset.fieldKey;
                let val = action.dataset.fieldDefault;
                if (val === 'true') val = true;
                else if (val === 'false') val = false;
                else if (val === undefined) val = undefined;
                insertField(key, val);
                break;
            }
            case 'edit-scraper':
                editScraper(Number(action.dataset.scraperId));
                break;
            case 'delete-scraper':
                deleteScraper(Number(action.dataset.scraperId));
                break;
            case 'save-scraper-config':
                saveScraperConfig();
                break;
            case 'close-config-modal':
                closeConfigModal();
                break;
        }
    }, { signal });

    document.addEventListener('change', (e) => {
        const el = e.target.closest('#script-type');
        if (el) updateConfigTemplate();
    }, { signal });

    // Close modal on backdrop click
    const modal = document.getElementById('config-modal');
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target.id === 'config-modal') closeConfigModal();
        }, { signal });
    }
}

function initManualForm() {
    const form = document.getElementById('add-scraper-form');
    if (!form) return;
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (_scraperSubmitting) return;
        _scraperSubmitting = true;
        const btn = e.target.querySelector('button[type="submit"]');
        btn.disabled = true;

        const name = document.getElementById('scraper-name').value;
        const description = document.getElementById('scraper-description').value;
        const script = document.getElementById('scraper-script').value;
        const scriptType = document.getElementById('script-type').value;
        if (!validateJSON(script)) {
            btn.disabled = false;
            _scraperSubmitting = false;
            return;
        }

        try {
            await api('POST', '/api/scrapers', { name, description, script, script_type: scriptType });
            window.location.href = '/scrapers';
        } catch (err) {
            showToast('Failed to create: ' + err.message);
            btn.disabled = false;
            _scraperSubmitting = false;
        }
    });
}

export async function editScraper(id) {
    try {
        const scraper = await api('GET', `/api/scrapers/${id}`);
        document.getElementById('modal-scraper-id').value = scraper.id;
        document.getElementById('modal-scraper-name').value = scraper.name;
        document.getElementById('modal-scraper-description').value = scraper.description || '';
        try {
            document.getElementById('modal-scraper-script').value = JSON.stringify(JSON.parse(scraper.script), null, 2);
        } catch {
            document.getElementById('modal-scraper-script').value = scraper.script;
        }
        const modal = document.getElementById('config-modal');
        modal.style.display = 'flex';
        openModal(modal, closeConfigModal);
    } catch (err) {
        showToast('Failed to load scraper: ' + err.message);
    }
}

export function validateJSON(str) {
    try {
        JSON.parse(str);
        return true;
    } catch (err) {
        showToast('Invalid JSON: ' + err.message);
        return false;
    }
}

export async function saveScraperConfig() {
    const id = document.getElementById('modal-scraper-id').value;
    const name = document.getElementById('modal-scraper-name').value;
    const description = document.getElementById('modal-scraper-description').value;
    const script = document.getElementById('modal-scraper-script').value;
    if (!validateJSON(script)) return;

    try {
        await api('PUT', `/api/scrapers/${id}`, { name, description, script });
        window.location.href = '/scrapers';
    } catch (err) {
        showToast('Failed to save: ' + err.message);
    }
}

export function closeConfigModal() {
    const modal = document.getElementById('config-modal');
    if (modal) modal.style.display = 'none';
    closeModal();
}

export async function deleteScraper(id) {
    if (!confirm('Delete this scraper module?')) return;
    try {
        await api('DELETE', `/api/scrapers/${id}`);
        location.reload();
    } catch (err) {
        showToast('Failed to delete: ' + err.message);
    }
}

export function insertField(key, defaultValue) {
    const ta = document.getElementById('scraper-script');
    if (!ta) return;
    const val = ta.value;

    const valueStr = typeof defaultValue === 'boolean' ? String(defaultValue) : '"' + (defaultValue || '') + '"';
    const newLine = '  "' + key + '": ' + valueStr;

    if (val.trim() === '') {
        ta.value = '{\n' + newLine + '\n}';
        ta.selectionStart = ta.selectionEnd = 2 + newLine.length - (typeof defaultValue === 'boolean' ? 0 : 1);
        ta.focus();
        return;
    }

    const trimmed = val.trimEnd();
    if (trimmed.endsWith('}')) {
        const bracePos = val.lastIndexOf('}');
        const beforeBrace = val.substring(0, bracePos).trimEnd();
        if (beforeBrace.length > 0 && !beforeBrace.endsWith(',') && !beforeBrace.endsWith('{')) {
            const lastCharPos = val.substring(0, bracePos).search(/\S\s*$/);
            if (lastCharPos >= 0) {
                ta.value = val.substring(0, lastCharPos + 1) + ',' + val.substring(lastCharPos + 1, bracePos) + newLine + '\n}';
            } else {
                ta.value = val.substring(0, bracePos) + newLine + '\n}';
            }
        } else {
            ta.value = val.substring(0, bracePos) + newLine + '\n}';
        }
    } else {
        ta.value = val + ',\n' + newLine;
    }

    const inserted = ta.value.lastIndexOf(newLine);
    if (inserted >= 0) {
        ta.selectionStart = ta.selectionEnd = inserted + newLine.length - (typeof defaultValue === 'boolean' ? 0 : 1);
    }
    ta.focus();
}

export function toggleSchemaPanel() {
    const panel = document.getElementById('schema-panel');
    if (!panel) return;
    panel.style.display = panel.style.display === 'none' ? '' : 'none';
    updateSchemaContent();
}

export function updateSchemaContent() {
    const type = document.getElementById('script-type');
    if (!type) return;
    const isJson = type.value === 'json-api';
    const htmlSchema = document.getElementById('schema-html');
    const jsonSchema = document.getElementById('schema-json');
    if (htmlSchema) htmlSchema.style.display = isJson ? 'none' : '';
    if (jsonSchema) jsonSchema.style.display = isJson ? '' : 'none';
}

export function updateConfigTemplate() {
    updateSchemaContent();
    const type = document.getElementById('script-type');
    const textarea = document.getElementById('scraper-script');
    if (!type || !textarea) return;
    if (type.value === 'json-api') {
        textarea.placeholder = '{\n  "type": "json",\n  "itemsPath": "data.items",\n  "titlePath": "title",\n  "urlPath": "url",\n  "summaryPath": "description",\n  "datePath": "timestamp",\n  "baseUrl": "https://example.com",\n  "consolidateDuplicates": false\n}';
    } else {
        textarea.placeholder = '{\n  "type": "html",\n  "itemSelector": "article.post",\n  "titleSelector": "h2.title",\n  "urlSelector": "a.permalink",\n  "urlAttr": "href",\n  "summarySelector": "p.summary",\n  "imageSelector": "img.thumb",\n  "imageAttr": "src",\n  "dateSelector": "time",\n  "dateAttr": "datetime",\n  "baseUrl": "https://example.com"\n}';
    }
}

/** Reset module state for testing. */
export function _resetScraperPageState() {
    _scraperSubmitting = false;
    if (_scraperPageListenerAC) { _scraperPageListenerAC.abort(); _scraperPageListenerAC = null; }
}
