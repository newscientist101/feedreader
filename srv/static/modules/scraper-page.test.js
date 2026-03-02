import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the api module
vi.mock('./api.js', () => ({
    api: vi.fn(),
}));

vi.mock('./toast.js', () => ({
    showToast: vi.fn(),
}));

import { api } from './api.js';
import { showToast } from './toast.js';
import {
    switchScraperTab,
    insertField,
    toggleSchemaPanel,
    updateSchemaContent,
    updateConfigTemplate,
    validateJSON,
    editScraper,
    saveScraperConfig,
    closeConfigModal,
    deleteScraper,
    initScraperPage,
    initScraperPageListeners,
    _resetScraperPageState,
} from './scraper-page.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    api.mockReset();
    _resetScraperPageState();
});

describe('switchScraperTab', () => {
    it('activates the selected tab and panel', () => {
        document.body.innerHTML = `
            <button class="scraper-tab active" data-tab="ai"></button>
            <button class="scraper-tab" data-tab="manual"></button>
            <div class="scraper-panel active" data-panel="ai"></div>
            <div class="scraper-panel" data-panel="manual"></div>
        `;
        switchScraperTab('manual');
        expect(document.querySelector('[data-tab="ai"]').classList.contains('active')).toBe(false);
        expect(document.querySelector('[data-tab="manual"]').classList.contains('active')).toBe(true);
        expect(document.querySelector('[data-panel="ai"]').classList.contains('active')).toBe(false);
        expect(document.querySelector('[data-panel="manual"]').classList.contains('active')).toBe(true);
    });
});

describe('insertField', () => {
    it('inserts a field into an empty textarea', () => {
        document.body.innerHTML = '<textarea id="scraper-script"></textarea>';
        insertField('type', 'html');
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toBe('{\n  "type": "html"\n}');
    });

    it('inserts a field with no default value', () => {
        document.body.innerHTML = '<textarea id="scraper-script"></textarea>';
        insertField('itemSelector');
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toBe('{\n  "itemSelector": ""\n}');
    });

    it('appends a field with comma to existing JSON', () => {
        document.body.innerHTML = '<textarea id="scraper-script">{\n  "type": "html"\n}</textarea>';
        insertField('itemSelector');
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toContain('"type": "html"');
        expect(ta.value).toContain('"itemSelector": ""');
    });

    it('inserts boolean value without quotes', () => {
        document.body.innerHTML = '<textarea id="scraper-script"></textarea>';
        insertField('consolidateDuplicates', true);
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toBe('{\n  "consolidateDuplicates": true\n}');
    });

    it('does nothing if textarea not found', () => {
        document.body.innerHTML = '';
        // Should not throw
        insertField('type', 'html');
    });
});

describe('toggleSchemaPanel', () => {
    it('shows the panel when hidden', () => {
        document.body.innerHTML = `
            <div id="schema-panel" style="display: none;"></div>
            <select id="script-type"><option value="json-config">HTML</option></select>
            <div id="schema-html"></div>
            <div id="schema-json"></div>
        `;
        toggleSchemaPanel();
        expect(document.getElementById('schema-panel').style.display).toBe('');
    });

    it('hides the panel when visible', () => {
        document.body.innerHTML = `
            <div id="schema-panel" style="display: block;"></div>
            <select id="script-type"><option value="json-config">HTML</option></select>
            <div id="schema-html"></div>
            <div id="schema-json"></div>
        `;
        toggleSchemaPanel();
        // 'block' !== 'none', so it should toggle to 'none'
        expect(document.getElementById('schema-panel').style.display).toBe('none');
    });
});

describe('updateSchemaContent', () => {
    it('shows HTML schema for html config type', () => {
        document.body.innerHTML = `
            <select id="script-type"><option value="json-config" selected>HTML</option></select>
            <div id="schema-html" style="display: none;"></div>
            <div id="schema-json"></div>
        `;
        updateSchemaContent();
        expect(document.getElementById('schema-html').style.display).toBe('');
        expect(document.getElementById('schema-json').style.display).toBe('none');
    });

    it('shows JSON schema for json-api type', () => {
        document.body.innerHTML = `
            <select id="script-type"><option value="json-api" selected>JSON</option></select>
            <div id="schema-html"></div>
            <div id="schema-json" style="display: none;"></div>
        `;
        updateSchemaContent();
        expect(document.getElementById('schema-html').style.display).toBe('none');
        expect(document.getElementById('schema-json').style.display).toBe('');
    });
});

describe('updateConfigTemplate', () => {
    it('sets JSON API placeholder when json-api selected', () => {
        document.body.innerHTML = `
            <select id="script-type"><option value="json-api" selected>JSON</option></select>
            <textarea id="scraper-script"></textarea>
            <div id="schema-html"></div>
            <div id="schema-json"></div>
        `;
        updateConfigTemplate();
        expect(document.getElementById('scraper-script').placeholder).toContain('"type": "json"');
    });

    it('sets HTML placeholder when json-config selected', () => {
        document.body.innerHTML = `
            <select id="script-type"><option value="json-config" selected>HTML</option></select>
            <textarea id="scraper-script"></textarea>
            <div id="schema-html"></div>
            <div id="schema-json"></div>
        `;
        updateConfigTemplate();
        expect(document.getElementById('scraper-script').placeholder).toContain('"type": "html"');
    });
});

describe('validateJSON', () => {
    it('returns true for valid JSON', () => {
        expect(validateJSON('{"a": 1}')).toBe(true);
    });

    it('returns false and shows toast for invalid JSON', () => {
        expect(validateJSON('not json')).toBe(false);
        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Invalid JSON'));
    });
});

describe('editScraper', () => {
    it('populates modal fields and shows the modal', async () => {
        document.body.innerHTML = `
            <input type="hidden" id="modal-scraper-id">
            <input type="text" id="modal-scraper-name">
            <input type="text" id="modal-scraper-description">
            <textarea id="modal-scraper-script"></textarea>
            <div id="config-modal" style="display: none;"></div>
        `;
        api.mockResolvedValue({
            id: 42,
            name: 'Test Scraper',
            description: 'A test',
            script: '{"type":"html"}',
        });
        await editScraper(42);
        expect(api).toHaveBeenCalledWith('GET', '/api/scrapers/42');
        expect(document.getElementById('modal-scraper-id').value).toBe('42');
        expect(document.getElementById('modal-scraper-name').value).toBe('Test Scraper');
        expect(document.getElementById('modal-scraper-description').value).toBe('A test');
        expect(document.getElementById('modal-scraper-script').value).toContain('"type": "html"');
        expect(document.getElementById('config-modal').style.display).toBe('flex');
    });

    it('shows toast on error', async () => {
        document.body.innerHTML = '';
        api.mockRejectedValue(new Error('not found'));
        await editScraper(999);
        expect(showToast).toHaveBeenCalledWith('Failed to load scraper: not found');
    });
});

describe('saveScraperConfig', () => {
    it('calls API and redirects on success', async () => {
        document.body.innerHTML = `
            <input type="hidden" id="modal-scraper-id" value="42">
            <input type="text" id="modal-scraper-name" value="Updated">
            <input type="text" id="modal-scraper-description" value="Desc">
            <textarea id="modal-scraper-script">{"type": "html"}</textarea>
        `;
        api.mockResolvedValue({});
        // Mock window.location
        delete window.location;
        window.location = { href: '' };
        await saveScraperConfig();
        expect(api).toHaveBeenCalledWith('PUT', '/api/scrapers/42', {
            name: 'Updated',
            description: 'Desc',
            script: '{"type": "html"}',
        });
        expect(window.location.href).toBe('/scrapers');
    });

    it('does not call API for invalid JSON', async () => {
        document.body.innerHTML = `
            <input type="hidden" id="modal-scraper-id" value="42">
            <input type="text" id="modal-scraper-name" value="X">
            <input type="text" id="modal-scraper-description" value="">
            <textarea id="modal-scraper-script">NOT JSON</textarea>
        `;
        await saveScraperConfig();
        expect(api).not.toHaveBeenCalled();
    });
});

describe('closeConfigModal', () => {
    it('hides the modal', () => {
        document.body.innerHTML = '<div id="config-modal" style="display: flex;"></div>';
        closeConfigModal();
        expect(document.getElementById('config-modal').style.display).toBe('none');
    });

    it('does not throw if modal is missing', () => {
        document.body.innerHTML = '';
        closeConfigModal(); // should not throw
    });
});

describe('deleteScraper', () => {
    it('calls API and reloads on confirm', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        api.mockResolvedValue({});
        // Mock location.reload
        const reloadSpy = vi.fn();
        delete window.location;
        window.location = { reload: reloadSpy };
        await deleteScraper(7);
        expect(api).toHaveBeenCalledWith('DELETE', '/api/scrapers/7');
        expect(reloadSpy).toHaveBeenCalled();
    });

    it('does nothing if user cancels', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(false);
        await deleteScraper(7);
        expect(api).not.toHaveBeenCalled();
    });
});

describe('initScraperPage', () => {
    it('is a no-op if not on scrapers page', () => {
        document.body.innerHTML = '<div class="articles-view"></div>';
        initScraperPage(); // should not throw
    });

    it('checks AI status when on scrapers page', async () => {
        document.body.innerHTML = `
            <div class="scrapers-view">
                <div id="ai-status" class="ai-status">
                    <span class="status-text">Checking...</span>
                </div>
            </div>
        `;
        api.mockResolvedValue({ available: true });
        initScraperPage();
        await vi.waitFor(() => {
            expect(document.getElementById('ai-status').className).toBe('ai-status available');
        });
    });
});

describe('initScraperPageListeners', () => {
    // Only call once since listeners accumulate on document
    initScraperPageListeners();

    it('delegates switch-scraper-tab clicks', () => {
        document.body.innerHTML = `
            <button class="scraper-tab active" data-tab="ai" data-action="switch-scraper-tab"></button>
            <button class="scraper-tab" data-tab="manual" data-action="switch-scraper-tab"></button>
            <div class="scraper-panel active" data-panel="ai"></div>
            <div class="scraper-panel" data-panel="manual"></div>
        `;
        document.querySelector('[data-tab="manual"]').click();
        expect(document.querySelector('[data-tab="manual"]').classList.contains('active')).toBe(true);
        expect(document.querySelector('[data-panel="manual"]').classList.contains('active')).toBe(true);
    });

    it('delegates toggle-schema-panel clicks', () => {
        document.body.innerHTML = `
            <button data-action="toggle-schema-panel">Schema</button>
            <div id="schema-panel" style="display: none;"></div>
            <select id="script-type"><option value="json-config">HTML</option></select>
            <div id="schema-html"></div>
            <div id="schema-json"></div>
        `;
        document.querySelector('[data-action="toggle-schema-panel"]').click();
        expect(document.getElementById('schema-panel').style.display).toBe('');
    });

    it('delegates insert-field clicks', () => {
        document.body.innerHTML = `
            <dt data-action="insert-field" data-field-key="type" data-field-default="html">type</dt>
            <textarea id="scraper-script"></textarea>
        `;
        document.querySelector('[data-action="insert-field"]').click();
        expect(document.getElementById('scraper-script').value).toContain('"type": "html"');
    });

    it('delegates insert-field with boolean default', () => {
        document.body.innerHTML = `
            <dt data-action="insert-field" data-field-key="consolidateDuplicates" data-field-default="true">consolidateDuplicates</dt>
            <textarea id="scraper-script"></textarea>
        `;
        document.querySelector('[data-action="insert-field"]').click();
        expect(document.getElementById('scraper-script').value).toContain('"consolidateDuplicates": true');
    });

    it('delegates close-config-modal clicks', () => {
        document.body.innerHTML = `
            <div id="config-modal" style="display: flex;">
                <button data-action="close-config-modal">Close</button>
            </div>
        `;
        document.querySelector('[data-action="close-config-modal"]').click();
        expect(document.getElementById('config-modal').style.display).toBe('none');
    });

    it('delegates change event on script-type select', () => {
        document.body.innerHTML = `
            <select id="script-type">
                <option value="json-config">HTML</option>
                <option value="json-api">JSON</option>
            </select>
            <textarea id="scraper-script"></textarea>
            <div id="schema-html"></div>
            <div id="schema-json" style="display: none;"></div>
        `;
        const select = document.getElementById('script-type');
        select.value = 'json-api';
        select.dispatchEvent(new Event('change', { bubbles: true }));
        expect(document.getElementById('schema-json').style.display).toBe('');
        expect(document.getElementById('schema-html').style.display).toBe('none');
    });
});
