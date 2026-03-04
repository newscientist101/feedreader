import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { exportOPML, importOPML, initOpmlListeners } from './opml.js';
import { showToast } from './toast.js';

vi.mock('./toast.js');

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('exportOPML', () => {
    it('redirects to the OPML export endpoint', () => {
        const hrefDescriptor = Object.getOwnPropertyDescriptor(window, 'location') ||
            Object.getOwnPropertyDescriptor(Window.prototype, 'location');
        const mockLocation = { ...window.location, href: '' };
        Object.defineProperty(window, 'location', {
            value: mockLocation,
            writable: true,
            configurable: true,
        });

        exportOPML();

        expect(mockLocation.href).toBe('/api/opml/export');

        // Restore
        if (hrefDescriptor) {
            Object.defineProperty(window, 'location', hrefDescriptor);
        }
    });
});

describe('importOPML', () => {
    it('does nothing when no file is selected', async () => {
        vi.spyOn(globalThis, 'fetch');
        const input = { files: [], value: 'some-file.opml' };

        await importOPML(input);

        expect(fetch).not.toHaveBeenCalled();
        expect(showToast).not.toHaveBeenCalled();
        // input.value should NOT be cleared on early return
        expect(input.value).toBe('some-file.opml');
    });

    it('sends file via FormData and shows success toast', async () => {
        const mockFile = new File(['<opml></opml>'], 'feeds.opml', { type: 'text/xml' });
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ imported: 5, skipped: 2 }),
        });
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: vi.fn() },
            writable: true,
            configurable: true,
        });
        const input = { files: [mockFile], value: 'feeds.opml' };

        await importOPML(input);

        expect(fetch).toHaveBeenCalledWith('/api/opml/import', expect.objectContaining({
            method: 'POST',
        }));
        // Verify X-Requested-With header
        const callArgs = fetch.mock.calls[0][1];
        expect(callArgs.headers['X-Requested-With']).toBe('XMLHttpRequest');
        // Verify FormData body contains the file
        expect(callArgs.body).toBeInstanceOf(FormData);
        expect(callArgs.body.get('file')).toBe(mockFile);

        expect(showToast).toHaveBeenCalledWith('Imported 5 feeds (2 skipped)', 'success');
        expect(location.reload).toHaveBeenCalledOnce();
        expect(input.value).toBe('');
    });

    it('shows error toast when import fails (server error)', async () => {
        const mockFile = new File(['bad'], 'bad.opml');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            json: () => Promise.resolve({ error: 'Invalid OPML' }),
        });
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: vi.fn() },
            writable: true,
            configurable: true,
        });
        const input = { files: [mockFile], value: 'bad.opml' };

        await importOPML(input);

        expect(showToast).toHaveBeenCalledWith('Failed to import OPML: Invalid OPML');
        // Error path should not show success or reload
        expect(showToast).not.toHaveBeenCalledWith(expect.anything(), 'success');
        expect(location.reload).not.toHaveBeenCalled();
        expect(input.value).toBe('');
    });

    it('falls back to default error message when server returns no error field', async () => {
        const mockFile = new File(['bad'], 'bad.opml');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            json: () => Promise.resolve({}),
        });
        const input = { files: [mockFile], value: 'bad.opml' };

        await importOPML(input);

        expect(showToast).toHaveBeenCalledWith('Failed to import OPML: Import failed');
        expect(input.value).toBe('');
    });

    it('shows error toast when fetch throws', async () => {
        const mockFile = new File(['data'], 'feeds.opml');
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        const input = { files: [mockFile], value: 'feeds.opml' };

        await importOPML(input);

        expect(showToast).toHaveBeenCalledWith('Failed to import OPML: Network error');
        expect(input.value).toBe('');
    });

    it('shows error toast when response body is not valid JSON', async () => {
        const mockFile = new File(['data'], 'feeds.opml');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            json: () => Promise.reject(new SyntaxError('Unexpected token < in JSON')),
        });
        const input = { files: [mockFile], value: 'feeds.opml' };

        await importOPML(input);

        expect(showToast).toHaveBeenCalledWith(
            expect.stringContaining('Failed to import OPML:')
        );
        expect(input.value).toBe('');
    });
});

describe('initOpmlListeners', () => {
    beforeEach(() => {
        initOpmlListeners();
    });

    it('delegates export-opml click', () => {
        document.body.innerHTML = '<button data-action="export-opml">Export</button>';
        const mockLocation = { ...window.location, href: '' };
        Object.defineProperty(window, 'location', {
            value: mockLocation,
            writable: true,
            configurable: true,
        });

        document.querySelector('[data-action="export-opml"]').click();

        expect(mockLocation.href).toBe('/api/opml/export');
    });

    it('delegates export-opml click from nested child element', () => {
        document.body.innerHTML = '<button data-action="export-opml"><span class="icon">↓</span></button>';
        const mockLocation = { ...window.location, href: '' };
        Object.defineProperty(window, 'location', {
            value: mockLocation,
            writable: true,
            configurable: true,
        });

        document.querySelector('.icon').click();

        expect(mockLocation.href).toBe('/api/opml/export');
    });

    it('does not trigger export for elements without data-action', () => {
        document.body.innerHTML = '<button class="other-btn">Not Export</button>';
        const mockLocation = { ...window.location, href: '' };
        Object.defineProperty(window, 'location', {
            value: mockLocation,
            writable: true,
            configurable: true,
        });

        document.querySelector('.other-btn').click();

        expect(mockLocation.href).toBe('');
    });

    it('delegates import-opml change', async () => {
        document.body.innerHTML = '<input data-action="import-opml" type="file">';
        const input = document.querySelector('[data-action="import-opml"]');
        // Mock the files property
        Object.defineProperty(input, 'files', { value: [], writable: true });
        vi.spyOn(globalThis, 'fetch');

        input.dispatchEvent(new Event('change', { bubbles: true }));
        await new Promise(r => setTimeout(r, 10));

        // importOPML was called, but with no files, so no fetch
        expect(fetch).not.toHaveBeenCalled();
    });

    it('does not trigger import for change on non-import elements', async () => {
        document.body.innerHTML = '<input class="other-input" type="file">';
        vi.spyOn(globalThis, 'fetch');

        document.querySelector('.other-input').dispatchEvent(
            new Event('change', { bubbles: true })
        );
        await new Promise(r => setTimeout(r, 10));

        expect(fetch).not.toHaveBeenCalled();
    });
});
