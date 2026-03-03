import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    openCreateFolderModal, closeCreateFolderModal, submitCreateFolder,
    renameCategory, unparentCategory, deleteCategory,
    initFoldersPageListeners,
} from './folders.js';
import { showToast } from './toast.js';

vi.mock('./toast.js', () => ({
    showToast: vi.fn(),
}));

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    // Ensure dialog functions exist for happy-dom compatibility
    window.confirm ??= () => false;
    window.prompt ??= () => null;
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('openCreateFolderModal', () => {
    it('shows the modal and focuses input', () => {
        document.body.innerHTML = `
            <div id="create-folder-modal" style="display: none">
                <input id="new-folder-name" value="Old">
                <select id="new-folder-parent">
                    <option value="0">None</option>
                    <option value="5">Parent</option>
                </select>
            </div>
        `;
        // Pre-select a non-default option
        document.getElementById('new-folder-parent').value = '5';

        openCreateFolderModal();

        expect(document.getElementById('create-folder-modal').style.display).toBe('flex');
        expect(document.getElementById('new-folder-name').value).toBe('');
        expect(document.getElementById('new-folder-parent').value).toBe('0');
    });

    it('does nothing when modal not found', () => {
        document.body.innerHTML = '<div>Content</div>';
        openCreateFolderModal(); // should not throw
    });
});

describe('closeCreateFolderModal', () => {
    it('hides the modal', () => {
        document.body.innerHTML = '<div id="create-folder-modal" style="display: flex"></div>';

        closeCreateFolderModal();

        expect(document.getElementById('create-folder-modal').style.display).toBe('none');
    });

    it('does nothing when modal not found', () => {
        document.body.innerHTML = '<div>Content</div>';
        closeCreateFolderModal(); // should not throw
    });
});

describe('submitCreateFolder', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div id="create-folder-modal">
                <input id="new-folder-name" value="New Folder">
                <select id="new-folder-parent">
                    <option value="0">None</option>
                    <option value="5">Parent</option>
                </select>
            </div>
        `;
    });

    it('creates a folder with no parent', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ id: 10 }),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(event.preventDefault).toHaveBeenCalled();
        expect(fetch).toHaveBeenCalledWith('/api/categories', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ name: 'New Folder' }),
        }));
    });

    it('creates a folder with parent', async () => {
        document.getElementById('new-folder-parent').value = '5';
        const fetchMock = vi.spyOn(globalThis, 'fetch')
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ id: 10 }),
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({}),
            });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(fetchMock).toHaveBeenCalledTimes(2);
        // Second call sets the parent
        expect(fetchMock.mock.calls[1][0]).toBe('/api/categories/10/parent');
    });

    it('does nothing when name is empty', async () => {
        document.getElementById('new-folder-name').value = '  ';
        vi.spyOn(globalThis, 'fetch');
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(fetch).not.toHaveBeenCalled();
    });

    it('shows toast on error', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Duplicate' })),
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to create folder'));
    });
});

describe('renameCategory', () => {
    it('renames on user input', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue('Renamed Folder');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await renameCategory(5, 'Old Name');

        expect(window.prompt).toHaveBeenCalledWith('Enter new name:', 'Old Name');
        expect(fetch).toHaveBeenCalledWith('/api/categories/5', expect.objectContaining({
            method: 'PUT',
            body: JSON.stringify({ name: 'Renamed Folder' }),
        }));
    });

    it('does nothing when user cancels prompt', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue(null);
        vi.spyOn(globalThis, 'fetch');

        await renameCategory(5, 'Old Name');

        expect(fetch).not.toHaveBeenCalled();
    });

    it('does nothing when name is unchanged', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue('Old Name');
        vi.spyOn(globalThis, 'fetch');

        await renameCategory(5, 'Old Name');

        expect(fetch).not.toHaveBeenCalled();
    });
});

describe('unparentCategory', () => {
    it('moves category to top level', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await unparentCategory(5);

        expect(fetch).toHaveBeenCalledWith('/api/categories/5/parent', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ parent_id: null, sort_order: 999 }),
        }));
    });

    it('handles errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'fail' })),
        });
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await unparentCategory(5);

        expect(console.error).toHaveBeenCalledWith('Failed to unparent category:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to move folder to top level');
    });
});

describe('deleteCategory', () => {
    it('deletes on confirm', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await deleteCategory(5, 'Tech');

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Tech'));
        expect(fetch).toHaveBeenCalledWith('/api/categories/5', expect.objectContaining({ method: 'DELETE' }));
    });

    it('does nothing when user cancels', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(false);
        vi.spyOn(globalThis, 'fetch');

        await deleteCategory(5, 'Tech');

        expect(fetch).not.toHaveBeenCalled();
    });
});

describe('initFoldersPageListeners', () => {
    beforeEach(() => {
        initFoldersPageListeners();
    });

    it('delegates rename-category click', () => {
        document.body.innerHTML = `
            <button data-action="rename-category" data-category-id="5" data-category-name="Tech">Rename</button>
        `;
        vi.spyOn(window, 'prompt').mockReturnValue(null); // user cancels

        document.querySelector('[data-action="rename-category"]').click();

        expect(window.prompt).toHaveBeenCalledWith('Enter new name:', 'Tech');
    });

    it('delegates delete-category click', () => {
        document.body.innerHTML = `
            <button data-action="delete-category" data-category-id="5" data-category-name="Tech">Delete</button>
        `;
        vi.spyOn(window, 'confirm').mockReturnValue(false);

        document.querySelector('[data-action="delete-category"]').click();

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Tech'));
    });

    it('delegates open-create-folder click', () => {
        document.body.innerHTML = `
            <div data-action="open-create-folder">New Folder</div>
            <div id="create-folder-modal" style="display: none">
                <input id="new-folder-name" value="">
                <select id="new-folder-parent"><option value="0">None</option></select>
            </div>
        `;

        document.querySelector('[data-action="open-create-folder"]').click();

        expect(document.getElementById('create-folder-modal').style.display).toBe('flex');
    });

    it('delegates close-create-folder click', () => {
        document.body.innerHTML = `
            <div id="create-folder-modal" style="display: flex">
                <button data-action="close-create-folder">Close</button>
            </div>
        `;

        document.querySelector('[data-action="close-create-folder"]').click();

        expect(document.getElementById('create-folder-modal').style.display).toBe('none');
    });

    it('delegates create-folder-form submission', async () => {
        document.body.innerHTML = `
            <form id="create-folder-form">
                <input id="new-folder-name" value="Test">
                <select id="new-folder-parent"><option value="0">None</option></select>
                <button type="submit">Create</button>
            </form>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ id: 10 }),
        });
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: vi.fn() },
            writable: true,
            configurable: true,
        });

        const form = document.getElementById('create-folder-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await new Promise(r => setTimeout(r, 10));

        expect(fetch).toHaveBeenCalledWith('/api/categories', expect.objectContaining({
            method: 'POST',
        }));
    });

    it('ignores rename-category without id', () => {
        document.body.innerHTML = `
            <button data-action="rename-category" data-category-name="Tech">Rename</button>
        `;
        vi.spyOn(window, 'prompt');

        document.querySelector('[data-action="rename-category"]').click();

        expect(window.prompt).not.toHaveBeenCalled();
    });
});
