import { describe, it, expect, vi, beforeEach } from 'vitest';
import { flushPromises } from './test-helpers.js';

// Mock the api module
vi.mock('./api.js');

vi.mock('./toast.js');

vi.mock('./modal.js');

import { api } from './api.js';
import { showToast } from './toast.js';
import { openModal, closeModal } from './modal.js';
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
    showToast.mockClear();
    openModal.mockClear();
    closeModal.mockClear();
    _resetScraperPageState();
    // Ensure dialog functions exist for happy-dom compatibility
    window.confirm ??= () => false;
    window.prompt ??= () => null;
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

    it('inserts boolean true value without quotes', () => {
        document.body.innerHTML = '<textarea id="scraper-script"></textarea>';
        insertField('consolidateDuplicates', true);
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toBe('{\n  "consolidateDuplicates": true\n}');
    });

    it('inserts boolean false value without quotes', () => {
        document.body.innerHTML = '<textarea id="scraper-script"></textarea>';
        insertField('consolidateDuplicates', false);
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toBe('{\n  "consolidateDuplicates": false\n}');
    });

    it('does nothing if textarea not found', () => {
        document.body.innerHTML = '';
        // Should not throw
        insertField('type', 'html');
    });

    it('appends with comma when text does not end with closing brace', () => {
        document.body.innerHTML = '<textarea id="scraper-script">"someText"</textarea>';
        insertField('type', 'html');
        const ta = document.getElementById('scraper-script');
        expect(ta.value).toBe('"someText",\n  "type": "html"');
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

    it('is a no-op when schema-panel element is missing', () => {
        document.body.innerHTML = '';
        // Should not throw
        toggleSchemaPanel();
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

    it('is a no-op when script-type element is missing', () => {
        document.body.innerHTML = '';
        // Should not throw
        updateSchemaContent();
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

    it('is a no-op when type or textarea elements are missing', () => {
        document.body.innerHTML = '';
        // Should not throw
        updateConfigTemplate();
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
    it('populates modal fields, shows the modal, and calls openModal', async () => {
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
        expect(openModal).toHaveBeenCalledWith(
            document.getElementById('config-modal'),
            expect.any(Function)
        );
    });

    it('shows toast on error', async () => {
        document.body.innerHTML = '';
        api.mockRejectedValue(new Error('not found'));
        await editScraper(999);
        expect(showToast).toHaveBeenCalledWith('Failed to load scraper: not found');
    });

    it('falls back to raw string when script is not valid JSON', async () => {
        document.body.innerHTML = `
            <input type="hidden" id="modal-scraper-id">
            <input type="text" id="modal-scraper-name">
            <input type="text" id="modal-scraper-description">
            <textarea id="modal-scraper-script"></textarea>
            <div id="config-modal" style="display: none;"></div>
        `;
        api.mockResolvedValue({
            id: 10,
            name: 'Bad Script',
            description: 'desc',
            script: 'not valid json {{{',
        });
        await editScraper(10);
        expect(document.getElementById('modal-scraper-script').value).toBe('not valid json {{{');
    });

    it('defaults description to empty string when null', async () => {
        document.body.innerHTML = `
            <input type="hidden" id="modal-scraper-id">
            <input type="text" id="modal-scraper-name">
            <input type="text" id="modal-scraper-description">
            <textarea id="modal-scraper-script"></textarea>
            <div id="config-modal" style="display: none;"></div>
        `;
        api.mockResolvedValue({
            id: 11,
            name: 'No Desc',
            description: null,
            script: '{"a":1}',
        });
        await editScraper(11);
        expect(document.getElementById('modal-scraper-description').value).toBe('');
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

    it('shows toast on API error', async () => {
        document.body.innerHTML = `
            <input type="hidden" id="modal-scraper-id" value="42">
            <input type="text" id="modal-scraper-name" value="Updated">
            <input type="text" id="modal-scraper-description" value="Desc">
            <textarea id="modal-scraper-script">{"type": "html"}</textarea>
        `;
        api.mockRejectedValue(new Error('server error'));
        await saveScraperConfig();
        expect(showToast).toHaveBeenCalledWith('Failed to save: server error');
    });
});

describe('closeConfigModal', () => {
    it('hides the modal and calls closeModal', () => {
        document.body.innerHTML = '<div id="config-modal" style="display: flex;"></div>';
        closeConfigModal();
        expect(document.getElementById('config-modal').style.display).toBe('none');
        expect(closeModal).toHaveBeenCalled();
    });

    it('does not throw if modal is missing, still calls closeModal', () => {
        document.body.innerHTML = '';
        closeConfigModal(); // should not throw
        expect(closeModal).toHaveBeenCalled();
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

    it('shows toast on API error and does not reload', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        api.mockRejectedValue(new Error('delete failed'));
        const reloadSpy = vi.fn();
        delete window.location;
        window.location = { reload: reloadSpy };
        await deleteScraper(7);
        expect(showToast).toHaveBeenCalledWith('Failed to delete: delete failed');
        expect(reloadSpy).not.toHaveBeenCalled();
    });
});

describe('initScraperPage', () => {
    it('is a no-op if not on scrapers page', () => {
        document.body.innerHTML = '<div class="articles-view"></div>';
        initScraperPage(); // should not throw
    });

    it('checks AI status when on scrapers page — available', async () => {
        document.body.innerHTML = `
            <div class="scrapers-view">
                <div id="ai-status" class="ai-status">
                    <span class="status-text">Checking...</span>
                </div>
            </div>
        `;
        api.mockResolvedValue({ available: true });
        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');
    });

    it('checks AI status — unavailable path', async () => {
        document.body.innerHTML = `
            <div class="scrapers-view">
                <div id="ai-status" class="ai-status">
                    <span class="status-text">Checking...</span>
                </div>
                <form id="ai-generate-form">
                    <button id="ai-generate-btn" type="submit">Generate</button>
                </form>
            </div>
        `;
        api.mockResolvedValue({ available: false });
        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status unavailable');
        expect(document.querySelector('#ai-status .status-text').textContent).toBe('Shelley is not running');
        expect(document.getElementById('ai-generate-form').classList.contains('disabled')).toBe(true);
        expect(document.getElementById('ai-generate-btn').disabled).toBe(true);
    });

    it('checks AI status — error/catch path', async () => {
        document.body.innerHTML = `
            <div class="scrapers-view">
                <div id="ai-status" class="ai-status">
                    <span class="status-text">Checking...</span>
                </div>
            </div>
        `;
        api.mockRejectedValue(new Error('network'));
        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status error');
        expect(document.querySelector('#ai-status .status-text').textContent).toBe('Could not check AI status');
    });
});

describe('initAiForm', () => {
    function setupAiFormDOM() {
        document.body.innerHTML = `
            <div class="scrapers-view">
                <div id="ai-status" class="ai-status">
                    <span class="status-text">Checking...</span>
                </div>
                <form id="ai-generate-form">
                    <input id="ai-url" value="https://example.com" />
                    <textarea id="ai-description">Get articles</textarea>
                    <button id="ai-generate-btn" type="submit">Generate</button>
                </form>
                <input id="scraper-name" />
                <textarea id="scraper-script"></textarea>
                <button class="scraper-tab" data-tab="ai"></button>
                <button class="scraper-tab" data-tab="manual"></button>
                <div class="scraper-panel" data-panel="ai"></div>
                <div class="scraper-panel" data-panel="manual"></div>
            </div>
        `;
    }

    it('success: calls API, sets fields, switches to manual tab, restores button', async () => {
        setupAiFormDOM();
        // First call for checkAiStatus, second for form submit
        api.mockResolvedValueOnce({ available: true });
        api.mockResolvedValueOnce({ name: 'Generated Scraper', config: '{"type":"html"}' });

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('ai-generate-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(api).toHaveBeenCalledWith('POST', '/api/ai/generate-scraper', {
            url: 'https://example.com',
            description: 'Get articles',
        });
        expect(document.getElementById('scraper-name').value).toBe('Generated Scraper');
        expect(document.getElementById('scraper-script').value).toBe('{"type":"html"}');
        expect(document.querySelector('[data-tab="manual"]').classList.contains('active')).toBe(true);
        // Button should be restored
        const btn = document.getElementById('ai-generate-btn');
        expect(btn.disabled).toBe(false);
    });

    it('error: shows toast and restores button', async () => {
        setupAiFormDOM();
        api.mockResolvedValueOnce({ available: true });
        api.mockRejectedValueOnce(new Error('AI failed'));

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('ai-generate-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(showToast).toHaveBeenCalledWith('Failed to generate: AI failed');
        const btn = document.getElementById('ai-generate-btn');
        expect(btn.disabled).toBe(false);
    });

    it('success with no name defaults to Custom Scraper', async () => {
        setupAiFormDOM();
        api.mockResolvedValueOnce({ available: true });
        api.mockResolvedValueOnce({ config: '{"type":"json"}' }); // no name

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('ai-generate-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(document.getElementById('scraper-name').value).toBe('Custom Scraper');
    });
});

describe('initManualForm', () => {
    function setupManualFormDOM() {
        document.body.innerHTML = `
            <div class="scrapers-view">
                <div id="ai-status" class="ai-status">
                    <span class="status-text">Checking...</span>
                </div>
                <form id="add-scraper-form">
                    <input id="scraper-name" value="My Scraper" />
                    <textarea id="scraper-description">A description</textarea>
                    <textarea id="scraper-script">{"type": "html"}</textarea>
                    <select id="script-type"><option value="json-config" selected>HTML</option></select>
                    <button type="submit">Create</button>
                </form>
            </div>
        `;
    }

    it('success: validates JSON, calls API POST, redirects', async () => {
        setupManualFormDOM();
        api.mockResolvedValueOnce({ available: true }); // checkAiStatus
        api.mockResolvedValueOnce({}); // POST

        delete window.location;
        window.location = { href: '' };

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('add-scraper-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(api).toHaveBeenCalledWith('POST', '/api/scrapers', {
            name: 'My Scraper',
            description: 'A description',
            script: '{"type": "html"}',
            script_type: 'json-config',
        });
        expect(window.location.href).toBe('/scrapers');
    });

    it('invalid JSON: re-enables button and resets submitting flag', async () => {
        setupManualFormDOM();
        document.getElementById('scraper-script').value = 'NOT JSON';
        api.mockResolvedValueOnce({ available: true }); // checkAiStatus

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('add-scraper-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Invalid JSON'));
        const btn = form.querySelector('button[type="submit"]');
        expect(btn.disabled).toBe(false);
        // Should not have called the POST api
        expect(api).not.toHaveBeenCalledWith('POST', '/api/scrapers', expect.anything());
    });

    it('API error: shows toast, re-enables button, resets submitting flag', async () => {
        setupManualFormDOM();
        api.mockResolvedValueOnce({ available: true }); // checkAiStatus
        api.mockRejectedValueOnce(new Error('create failed')); // POST

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('add-scraper-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(showToast).toHaveBeenCalledWith('Failed to create: create failed');
        const btn = form.querySelector('button[type="submit"]');
        expect(btn.disabled).toBe(false);
    });

    it('double-submit guard: second submit is ignored while first is in-flight', async () => {
        setupManualFormDOM();
        api.mockResolvedValueOnce({ available: true }); // checkAiStatus

        // Make the POST hang until we resolve it
        let resolvePost;
        api.mockImplementationOnce(() => new Promise((r) => { resolvePost = r; }));

        delete window.location;
        window.location = { href: '' };

        initScraperPage();
        await flushPromises();
        expect(document.getElementById('ai-status').className).toBe('ai-status available');

        const form = document.getElementById('add-scraper-form');
        // First submit
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await flushPromises();

        expect(api).toHaveBeenCalledWith('POST', '/api/scrapers', expect.anything());

        // Second submit should be ignored
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

        // API should still only have been called once for POST
        const postCalls = api.mock.calls.filter(c => c[0] === 'POST' && c[1] === '/api/scrapers');
        expect(postCalls.length).toBe(1);

        // Resolve the first POST to clean up
        resolvePost({});
    });
});

describe('initScraperPageListeners', () => {
    // Re-init before each test since _resetScraperPageState aborts the AbortController
    beforeEach(() => {
        initScraperPageListeners();
    });

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

    it('delegates insert-field with boolean true default', () => {
        document.body.innerHTML = `
            <dt data-action="insert-field" data-field-key="consolidateDuplicates" data-field-default="true">consolidateDuplicates</dt>
            <textarea id="scraper-script"></textarea>
        `;
        document.querySelector('[data-action="insert-field"]').click();
        expect(document.getElementById('scraper-script').value).toContain('"consolidateDuplicates": true');
    });

    it('delegates insert-field with boolean false default', () => {
        document.body.innerHTML = `
            <dt data-action="insert-field" data-field-key="consolidateDuplicates" data-field-default="false">consolidateDuplicates</dt>
            <textarea id="scraper-script"></textarea>
        `;
        document.querySelector('[data-action="insert-field"]').click();
        expect(document.getElementById('scraper-script').value).toContain('"consolidateDuplicates": false');
    });

    it('delegates insert-field with no default (undefined)', () => {
        document.body.innerHTML = `
            <dt data-action="insert-field" data-field-key="itemSelector">itemSelector</dt>
            <textarea id="scraper-script"></textarea>
        `;
        document.querySelector('[data-action="insert-field"]').click();
        // undefined default → insertField(key, undefined) which produces ""
        expect(document.getElementById('scraper-script').value).toContain('"itemSelector": ""');
    });

    it('delegates edit-scraper clicks', async () => {
        document.body.innerHTML = `
            <button data-action="edit-scraper" data-scraper-id="42">Edit</button>
            <input type="hidden" id="modal-scraper-id">
            <input type="text" id="modal-scraper-name">
            <input type="text" id="modal-scraper-description">
            <textarea id="modal-scraper-script"></textarea>
            <div id="config-modal" style="display: none;"></div>
        `;
        api.mockResolvedValue({
            id: 42,
            name: 'Clicked Scraper',
            description: 'desc',
            script: '{"type":"html"}',
        });
        document.querySelector('[data-action="edit-scraper"]').click();
        await flushPromises();
        expect(api).toHaveBeenCalledWith('GET', '/api/scrapers/42');
        expect(document.getElementById('modal-scraper-name').value).toBe('Clicked Scraper');
    });

    it('delegates delete-scraper clicks', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        api.mockResolvedValue({});
        const reloadSpy = vi.fn();
        delete window.location;
        window.location = { reload: reloadSpy };

        document.body.innerHTML = `
            <button data-action="delete-scraper" data-scraper-id="7">Delete</button>
        `;
        document.querySelector('[data-action="delete-scraper"]').click();
        await flushPromises();
        expect(api).toHaveBeenCalledWith('DELETE', '/api/scrapers/7');
        expect(reloadSpy).toHaveBeenCalled();
    });

    it('delegates close-config-modal clicks', () => {
        document.body.innerHTML = `
            <div id="config-modal" style="display: flex;">
                <button data-action="close-config-modal">Close</button>
            </div>
        `;
        document.querySelector('[data-action="close-config-modal"]').click();
        expect(document.getElementById('config-modal').style.display).toBe('none');
        expect(closeModal).toHaveBeenCalled();
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

    it('ignores clicks on elements without data-action', () => {
        document.body.innerHTML = `
            <button id="no-action">No action</button>
        `;
        // Should not throw
        document.getElementById('no-action').click();
    });
});

describe('initScraperPageListeners — backdrop click', () => {
    it('closes modal when backdrop (modal element itself) is clicked', () => {
        document.body.innerHTML = `
            <div id="config-modal" style="display: flex;">
                <div class="modal-content">Content</div>
            </div>
        `;
        // Re-init listeners for this describe so modal is found
        initScraperPageListeners();

        const modal = document.getElementById('config-modal');
        // Click on the modal backdrop itself (not content inside)
        const event = new Event('click', { bubbles: true });
        Object.defineProperty(event, 'target', { value: modal });
        modal.dispatchEvent(event);

        expect(modal.style.display).toBe('none');
        expect(closeModal).toHaveBeenCalled();
    });
});
