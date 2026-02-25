import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { exportOPML, importOPML, initOpmlListeners } from './opml.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
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
        const input = { files: [], value: '' };

        await importOPML(input);

        expect(fetch).not.toHaveBeenCalled();
    });

    it('sends file via FormData and shows success alert', async () => {
        const mockFile = new File(['<opml></opml>'], 'feeds.opml', { type: 'text/xml' });
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ imported: 5, skipped: 2 }),
        });
        vi.spyOn(window, 'alert').mockImplementation(() => {});
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
        expect(window.alert).toHaveBeenCalledWith('Imported 5 feeds (2 skipped, already exist)');
        expect(input.value).toBe('');
    });

    it('shows error alert when import fails (server error)', async () => {
        const mockFile = new File(['bad'], 'bad.opml');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            json: () => Promise.resolve({ error: 'Invalid OPML' }),
        });
        vi.spyOn(window, 'alert').mockImplementation(() => {});
        const input = { files: [mockFile], value: 'bad.opml' };

        await importOPML(input);

        expect(window.alert).toHaveBeenCalledWith('Failed to import OPML: Invalid OPML');
        expect(input.value).toBe('');
    });

    it('shows error alert when fetch throws', async () => {
        const mockFile = new File(['data'], 'feeds.opml');
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        vi.spyOn(window, 'alert').mockImplementation(() => {});
        const input = { files: [mockFile], value: 'feeds.opml' };

        await importOPML(input);

        expect(window.alert).toHaveBeenCalledWith('Failed to import OPML: Network error');
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
});
